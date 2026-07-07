package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/apitoken"
	"lazycat.community/appstore/ent/user"
)

const sessionCookie = "lazycat_store_session"

type userContextKey struct{}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, *ent.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.authenticate(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}
		if s.emailVerificationRequiredForUser(r.Context(), u) {
			writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Email verification is required before using this account", nil)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, u)), u)
	}
}

func (s *Server) withRole(roles ...user.Role) func(func(http.ResponseWriter, *http.Request, *ent.User)) http.HandlerFunc {
	allowed := make(map[user.Role]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next func(http.ResponseWriter, *http.Request, *ent.User)) http.HandlerFunc {
		return s.withAuth(func(w http.ResponseWriter, r *http.Request, u *ent.User) {
			if !allowed[u.Role] {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission for this action", nil)
				return
			}
			next(w, r, u)
		})
	}
}

func (s *Server) authenticate(r *http.Request) (*ent.User, bool) {
	if token := bearerToken(r); token != "" {
		if u, ok := s.authenticateToken(r.Context(), token); ok {
			return u, true
		}
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, false
	}
	userID, ok := s.verifySession(cookie.Value)
	if !ok {
		return nil, false
	}
	u, err := s.db.User.Get(r.Context(), userID)
	if err != nil || u.Disabled {
		return nil, false
	}
	return u, true
}

func (s *Server) optionalUser(r *http.Request) *ent.User {
	u, ok := s.authenticate(r)
	if !ok {
		return nil
	}
	if s.emailVerificationRequiredForUser(r.Context(), u) {
		return nil
	}
	return u
}

func (s *Server) authenticateToken(ctx context.Context, tokenValue string) (*ent.User, bool) {
	hash := tokenHash(tokenValue)
	record, err := s.db.APIToken.Query().Where(apitoken.TokenHashEQ(hash)).Only(ctx)
	if err != nil {
		return nil, false
	}
	_, _ = s.db.APIToken.UpdateOneID(record.ID).SetLastUsedAt(time.Now()).Save(ctx)
	u, err := s.db.User.Get(ctx, record.UserID)
	if err != nil || u.Disabled {
		return nil, false
	}
	return u, true
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func (s *Server) setSession(w http.ResponseWriter, userID int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.signSession(userID),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((14 * 24 * time.Hour).Seconds()),
	})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) secureCookies() bool {
	return strings.HasPrefix(strings.ToLower(s.cfg.BaseURL), "https://") || strings.HasPrefix(strings.ToLower(s.cfg.SitePublicURL), "https://")
}

func (s *Server) signSession(userID int) string {
	payload := strconv.Itoa(userID)
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) verifySession(value string) (int, bool) {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return 0, false
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	_, _ = mac.Write([]byte(payload))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(want, got) {
		return 0, false
	}
	id, err := strconv.Atoi(payload)
	return id, err == nil
}

func randomToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "lcst_" + base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func tokenPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isAdmin(u *ent.User) bool {
	return u != nil && (u.Role == user.RoleSOFTWARE_ADMIN || u.Role == user.RoleSITE_ADMIN)
}

func isSiteAdmin(u *ent.User) bool {
	return u != nil && u.Role == user.RoleSITE_ADMIN
}

func (s *Server) emailVerificationRequiredForUser(ctx context.Context, u *ent.User) bool {
	if u == nil || u.EmailVerified || u.Role != user.RoleUSER {
		return false
	}
	return s.effectiveRequireEmailVerify(ctx)
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return fmt.Sprintf("app-%d", time.Now().Unix())
	}
	return result
}
