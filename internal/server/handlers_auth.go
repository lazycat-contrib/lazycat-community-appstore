package server

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/apitoken"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	if input.Username == "" || len(input.Password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Username and a password of at least 8 characters are required", nil)
		return
	}
	requireEmailVerify := s.effectiveRequireEmailVerify(r.Context())
	if requireEmailVerify && input.Email == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Email is required when email verification is enabled", nil)
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not create user", nil)
		return
	}
	create := s.db.User.Create().
		SetUsername(input.Username).
		SetPasswordHash(hash).
		SetEmailVerified(!requireEmailVerify)
	if input.Email != "" {
		create.SetEmail(input.Email)
	}
	u, err := create.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "USER_EXISTS", "Username or email is already registered", nil)
		return
	}
	if requireEmailVerify {
		token, err := emailVerificationToken()
		if err == nil {
			_ = s.setSetting(r.Context(), "email_verify:"+u.Username, token)
			if input.Email != "" {
				if err := s.sendVerificationEmail(r.Context(), input.Email, token); err != nil {
					writeError(w, http.StatusInternalServerError, "EMAIL_SEND_FAILED", "Could not send verification email", nil)
					return
				}
			}
		}
	}
	s.setSession(w, u.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"user": toPublicUser(u)})
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var input verifyEmailRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(input.Token) == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Verification token is required", nil)
		return
	}
	users, err := s.db.User.Query().All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VERIFY_FAILED", "Could not verify email", nil)
		return
	}
	for _, u := range users {
		key := "email_verify:" + u.Username
		if s.setting(r.Context(), key, "") == input.Token {
			updated, err := s.db.User.UpdateOneID(u.ID).SetEmailVerified(true).Save(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "VERIFY_FAILED", "Could not verify email", nil)
				return
			}
			_ = s.setSetting(r.Context(), key, "")
			writeJSON(w, http.StatusOK, map[string]any{"user": toPublicUser(updated)})
			return
		}
	}
	writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Verification token not found", nil)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	u, err := s.db.User.Query().Where(user.UsernameEQ(strings.TrimSpace(input.Username))).Only(r.Context())
	if err != nil || !auth.CheckPassword(u.PasswordHash, input.Password) {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
		return
	}
	if s.emailVerificationRequiredForUser(r.Context(), u) {
		writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Email verification is required before login", nil)
		return
	}
	s.setSession(w, u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": toPublicUser(u)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func emailVerificationToken() (string, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *ent.User) {
	writeJSON(w, http.StatusOK, map[string]any{"user": toPublicUser(u)})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request, u *ent.User) {
	tokens, err := s.db.APIToken.Query().Where(apitoken.UserIDEQ(u.ID)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_LIST_FAILED", "Could not list API tokens", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

type createTokenRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request, u *ent.User) {
	var input createTokenRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "CI token"
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_CREATE_FAILED", "Could not create API token", nil)
		return
	}
	record, err := s.db.APIToken.Create().
		SetUserID(u.ID).
		SetName(input.Name).
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_CREATE_FAILED", "Could not create API token", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "record": record})
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request, u *ent.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.APIToken.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "API token not found", nil)
		return
	}
	if record.UserID != u.ID && !isSiteAdmin(u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot delete this token", nil)
		return
	}
	if err := s.db.APIToken.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_DELETE_FAILED", "Could not delete API token", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
