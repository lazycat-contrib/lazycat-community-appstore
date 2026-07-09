package clientserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	clientSessionCookie = "lazycat_store_client_session"
	clientOIDCCookie    = "lazycat_store_client_oidc"
)

type clientAuth struct {
	secret []byte
	oidc   *clientOIDCRuntime
}

type clientOIDCRuntime struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config
	issuer   string
}

type clientSessionClaims struct {
	UserID      string `json:"uid"`
	Subject     string `json:"sub,omitempty"`
	Issuer      string `json:"iss,omitempty"`
	DisplayName string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Expiry      int64  `json:"exp"`
}

type clientOIDCState struct {
	State  string `json:"state"`
	Nonce  string `json:"nonce"`
	Next   string `json:"next"`
	Expiry int64  `json:"exp"`
}

type clientOIDCClaims struct {
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Subject           string `json:"sub"`
	Name              string `json:"name"`
	Nickname          string `json:"nickname"`
	PreferredUsername string `json:"preferred_username"`
	Picture           string `json:"picture"`
}

func newClientAuth(ctx context.Context, cfg Config) (*clientAuth, error) {
	auth := &clientAuth{secret: []byte(strings.TrimSpace(cfg.SessionSecret))}
	if len(auth.secret) == 0 {
		auth.secret = []byte("dev-client-session-secret-change-me")
	}
	if !cfg.OIDCEnabled {
		return auth, nil
	}
	if strings.TrimSpace(cfg.OIDCIssuerURL) == "" || strings.TrimSpace(cfg.OIDCClientID) == "" || strings.TrimSpace(cfg.OIDCClientSecret) == "" {
		return nil, fmt.Errorf("client OIDC is enabled but issuer, client id, or client secret is missing")
	}
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("initialize client OIDC provider: %w", err)
	}
	auth.oidc = &clientOIDCRuntime{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.OIDCClientID,
		}),
		oauth2: oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       cfg.OIDCScopes,
		},
		issuer: cfg.OIDCIssuerURL,
	}
	return auth, nil
}

func (a *clientAuth) OIDCEnabled() bool {
	return a != nil && a.oidc != nil
}

func (s *Server) handleClientAuthMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.clientIdentity(r)
	var user *ClientIdentityDTO
	authenticated := ok && identity.Source != clientIdentityLocal
	if authenticated {
		user = &ClientIdentityDTO{
			ID:          identity.UserID,
			DisplayName: identity.DisplayName,
			Email:       identity.Email,
			AvatarURL:   identity.AvatarURL,
			Source:      identity.Source,
		}
	}
	writeJSON(w, http.StatusOK, ClientAuthStatusDTO{
		Authenticated: authenticated,
		OIDCEnabled:   s.auth != nil && s.auth.OIDCEnabled(),
		User:          user,
	})
}

func (s *Server) handleClientAuthLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     clientSessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClientOIDCStart(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || s.auth.oidc == nil {
		http.NotFound(w, r)
		return
	}
	state := randomClientToken()
	nonce := randomClientToken()
	encoded, err := s.auth.signJSON(clientOIDCState{
		State:  state,
		Nonce:  nonce,
		Next:   sanitizeClientNext(r.URL.Query().Get("next")),
		Expiry: time.Now().Add(10 * time.Minute).Unix(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OIDC_STATE_FAILED", "Could not start LazyCat OIDC login")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     clientOIDCCookie,
		Value:    encoded,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.auth.oidc.oauth2.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func (s *Server) handleClientOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil || s.auth.oidc == nil {
		http.NotFound(w, r)
		return
	}
	if errMessage := strings.TrimSpace(r.URL.Query().Get("error")); errMessage != "" {
		writeError(w, http.StatusUnauthorized, "OIDC_DENIED", errMessage)
		return
	}
	stateCookie, err := r.Cookie(clientOIDCCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "OIDC_STATE_MISSING", "Missing LazyCat OIDC state")
		return
	}
	var state clientOIDCState
	if err := s.auth.verifyJSON(stateCookie.Value, &state); err != nil || state.Expiry < time.Now().Unix() {
		writeError(w, http.StatusBadRequest, "OIDC_STATE_INVALID", "Invalid LazyCat OIDC state")
		return
	}
	if state.State != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "OIDC_STATE_MISMATCH", "LazyCat OIDC state mismatch")
		return
	}
	oauth2Token, err := s.auth.oidc.oauth2.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "OIDC_EXCHANGE_FAILED", "LazyCat OIDC code exchange failed")
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		writeError(w, http.StatusUnauthorized, "OIDC_TOKEN_MISSING", "LazyCat OIDC token is missing")
		return
	}
	idToken, err := s.auth.oidc.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "OIDC_TOKEN_INVALID", "LazyCat OIDC token is invalid")
		return
	}
	if idToken.Nonce != state.Nonce {
		writeError(w, http.StatusUnauthorized, "OIDC_NONCE_MISMATCH", "LazyCat OIDC nonce mismatch")
		return
	}
	var claims clientOIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		writeError(w, http.StatusUnauthorized, "OIDC_CLAIMS_INVALID", "LazyCat OIDC claims are invalid")
		return
	}
	claims = s.auth.mergeUserInfoClaims(r.Context(), oauth2Token, claims)
	identity := identityFromOIDCClaims(s.auth.oidc.issuer, claims)
	if identity.UserID == "" {
		writeError(w, http.StatusUnauthorized, "OIDC_ID_MISSING", "LazyCat OIDC user id is missing")
		return
	}
	s.auth.setSession(w, clientSessionClaims{
		UserID:      identity.UserID,
		Subject:     claims.Subject,
		Issuer:      s.auth.oidc.issuer,
		DisplayName: identity.DisplayName,
		Email:       identity.Email,
		AvatarURL:   identity.AvatarURL,
		Expiry:      time.Now().Add(14 * 24 * time.Hour).Unix(),
	})
	s.auth.clearOIDCCookie(w)
	http.Redirect(w, r, sanitizeClientNext(state.Next), http.StatusFound)
}

func (a *clientAuth) setSession(w http.ResponseWriter, claims clientSessionClaims) {
	value, err := a.signJSON(claims)
	if err != nil {
		return
	}
	maxAge := int(time.Until(time.Unix(claims.Expiry, 0)).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     clientSessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *clientAuth) clearOIDCCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     clientOIDCCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *clientAuth) session(r *http.Request) (clientSessionClaims, bool) {
	if a == nil {
		return clientSessionClaims{}, false
	}
	cookie, err := r.Cookie(clientSessionCookie)
	if err != nil {
		return clientSessionClaims{}, false
	}
	var claims clientSessionClaims
	if err := a.verifyJSON(cookie.Value, &claims); err != nil || claims.Expiry < time.Now().Unix() || strings.TrimSpace(claims.UserID) == "" {
		return clientSessionClaims{}, false
	}
	return claims, true
}

func (a *clientAuth) signJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *clientAuth) verifyJSON(value string, dst any) error {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return fmt.Errorf("invalid token")
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return fmt.Errorf("invalid signature")
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func (a *clientAuth) mergeUserInfoClaims(ctx context.Context, token *oauth2.Token, claims clientOIDCClaims) clientOIDCClaims {
	if a == nil || a.oidc == nil || token == nil {
		return claims
	}
	userInfo, err := a.oidc.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return claims
	}
	var extra clientOIDCClaims
	if err := userInfo.Claims(&extra); err != nil {
		return claims
	}
	claims.Email = defaultClientString(claims.Email, extra.Email)
	claims.Subject = defaultClientString(claims.Subject, extra.Subject)
	claims.Name = defaultClientString(claims.Name, extra.Name)
	claims.Nickname = defaultClientString(claims.Nickname, extra.Nickname)
	claims.PreferredUsername = defaultClientString(claims.PreferredUsername, extra.PreferredUsername)
	claims.Picture = defaultClientString(claims.Picture, extra.Picture)
	if !claims.EmailVerified {
		claims.EmailVerified = extra.EmailVerified
	}
	return claims
}

func identityFromOIDCClaims(issuer string, claims clientOIDCClaims) clientIdentity {
	userID := firstNonEmpty(claims.PreferredUsername, claims.Subject, claims.Email)
	return clientIdentity{
		UserID:      userID,
		DisplayName: firstNonEmpty(claims.Name, claims.Nickname, claims.PreferredUsername, claims.Email, claims.Subject),
		Email:       strings.TrimSpace(claims.Email),
		AvatarURL:   strings.TrimSpace(claims.Picture),
		Source:      clientIdentityOIDC,
		Issuer:      strings.TrimSpace(issuer),
		Subject:     strings.TrimSpace(claims.Subject),
	}
}

func sanitizeClientNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return "/"
	}
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") || strings.HasPrefix(next, "//") {
		return "/"
	}
	if !strings.HasPrefix(next, "/") {
		return "/"
	}
	parsed, err := url.Parse(next)
	if err != nil || parsed.Host != "" {
		return "/"
	}
	if strings.HasPrefix(parsed.Path, "/auth/oidc") {
		return "/"
	}
	return next
}

func randomClientToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func defaultClientString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
