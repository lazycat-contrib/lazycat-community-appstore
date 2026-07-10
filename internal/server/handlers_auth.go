package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"math"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/apitoken"
	"lazycat.community/appstore/ent/registrationinvite"
	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/storage"
)

var errInvalidRegistrationInvite = errors.New("invalid registration invite")

const emailVerificationSettingPrefix = "email_verify:"
const passwordResetSettingPrefix = "password_reset:"
const passwordResetTTL = 30 * time.Minute
const adminCaptchaFailedAttempts = 3

const (
	adminLoginFailureTTL     = 15 * time.Minute
	adminLoginBlockDuration  = 30 * time.Second
	adminLoginBlockThreshold = 6
	maxAdminLoginFailureKeys = 4096
)

type adminLoginFailure struct {
	Attempts     int
	ExpiresAt    time.Time
	BlockedUntil time.Time
}

type registerRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"inviteCode"`
}

type userResponse struct {
	User publicUser `json:"user"`
}

type avatarResponse struct {
	User publicUser `json:"user"`
	URL  string     `json:"url"`
}

type apiTokensResponse struct {
	Tokens []*ent.APIToken `json:"tokens"`
}

type apiTokenCreateResponse struct {
	Token  string        `json:"token"`
	Record *ent.APIToken `json:"record"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.InviteCode = strings.TrimSpace(input.InviteCode)
	registrationMode := s.registrationMode(r.Context())
	if registrationMode == registrationModeClosed {
		writeError(w, http.StatusForbidden, "REGISTRATION_CLOSED", "Registration is closed", nil)
		return
	}
	if input.Username == "" || len(input.Password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Username and a password of at least 8 characters are required", nil)
		return
	}
	if registrationMode == registrationModeInvite && input.InviteCode == "" {
		writeError(w, http.StatusUnprocessableEntity, "INVITE_REQUIRED", "Invite code is required", nil)
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
	if registrationMode == registrationModeInvite {
		u, token, err := s.createUserWithInvite(r, input, hash, requireEmailVerify)
		if errors.Is(err, errInvalidRegistrationInvite) {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_INVITE", "Invite code is invalid or has no remaining uses", nil)
			return
		}
		if ent.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "USER_EXISTS", "Username or email is already registered", nil)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "REGISTER_FAILED", "Could not create user", nil)
			return
		}
		if requireEmailVerify && input.Email != "" {
			if err := s.sendVerificationEmail(r.Context(), input.Email, u.Username, token, r.Header.Get("Accept-Language")); err != nil {
				writeError(w, http.StatusInternalServerError, "EMAIL_SEND_FAILED", "Could not send verification email", nil)
				return
			}
		}
		s.setSession(w, u.ID)
		writeJSON(w, http.StatusCreated, userResponse{User: toPublicUser(u)})
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
			_ = s.setSetting(r.Context(), emailVerificationSettingPrefix+u.Username, token)
			if input.Email != "" {
				if err := s.sendVerificationEmail(r.Context(), input.Email, u.Username, token, r.Header.Get("Accept-Language")); err != nil {
					writeError(w, http.StatusInternalServerError, "EMAIL_SEND_FAILED", "Could not send verification email", nil)
					return
				}
			}
		}
	}
	s.setSession(w, u.ID)
	writeJSON(w, http.StatusCreated, userResponse{User: toPublicUser(u)})
}

func (s *Server) createUserWithInvite(r *http.Request, input registerRequest, passwordHash string, requireEmailVerify bool) (*ent.User, string, error) {
	ctx := r.Context()
	verificationToken := ""
	if requireEmailVerify {
		token, err := emailVerificationToken()
		if err != nil {
			return nil, "", err
		}
		verificationToken = token
	}

	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	codeHash := tokenHash(input.InviteCode)
	affected, err := tx.RegistrationInvite.Update().
		Where(registrationinvite.CodeHashEQ(codeHash), registrationinvite.RemainingUsesGT(0)).
		AddRemainingUses(-1).
		Save(ctx)
	if err != nil {
		return nil, "", err
	}
	if affected != 1 {
		return nil, "", errInvalidRegistrationInvite
	}

	create := tx.User.Create().
		SetUsername(input.Username).
		SetPasswordHash(passwordHash).
		SetEmailVerified(!requireEmailVerify)
	if input.Email != "" {
		create.SetEmail(input.Email)
	}
	u, err := create.Save(ctx)
	if err != nil {
		return nil, "", err
	}
	if requireEmailVerify {
		if err := setSettingTx(ctx, tx, emailVerificationSettingPrefix+u.Username, verificationToken); err != nil {
			return nil, "", err
		}
	}
	if _, err := tx.RegistrationInvite.Delete().
		Where(registrationinvite.CodeHashEQ(codeHash), registrationinvite.RemainingUsesLTE(0)).
		Exec(ctx); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	committed = true
	return u, verificationToken, nil
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
	token := strings.TrimSpace(input.Token)
	record, err := s.db.SiteSetting.Query().
		Where(sitesetting.KeyHasPrefix(emailVerificationSettingPrefix), sitesetting.ValueEQ(token)).
		First(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Verification token not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "VERIFY_FAILED", "Could not verify email", nil)
		return
	}
	username := strings.TrimPrefix(record.Key, emailVerificationSettingPrefix)
	u, err := s.db.User.Query().Where(user.UsernameEQ(username)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Verification token not found", nil)
		return
	}
	updated, err := s.db.User.UpdateOneID(u.ID).SetEmailVerified(true).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VERIFY_FAILED", "Could not verify email", nil)
		return
	}
	_ = s.setSetting(r.Context(), record.Key, "")
	s.setSession(w, updated.ID)
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(updated)})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totpCode"`
}

type loginFailureDetails struct {
	FailedAttempts  int  `json:"failedAttempts"`
	CaptchaRequired bool `json:"captchaRequired"`
}

type twoFactorRequiredDetails struct {
	TwoFactorRequired bool `json:"twoFactorRequired"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	username := strings.TrimSpace(input.Username)
	u, err := s.db.User.Query().Where(user.UsernameEQ(username)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", nil)
		return
	}
	if isAdmin(u) {
		now := s.adminLoginNow()
		if state, ok := s.adminLoginFailureForRequest(r, username); ok && state.BlockedUntil.After(now) {
			writeAdminLoginRateLimit(w, state.BlockedUntil.Sub(now), state)
			return
		}
	}
	if !auth.CheckPassword(u.PasswordHash, input.Password) {
		var details any
		if isAdmin(u) {
			state := s.recordAdminLoginFailure(r, username)
			now := s.adminLoginNow()
			if state.BlockedUntil.After(now) {
				writeAdminLoginRateLimit(w, state.BlockedUntil.Sub(now), state)
				return
			}
			details = loginFailureDetails{
				FailedAttempts:  state.Attempts,
				CaptchaRequired: state.Attempts >= adminCaptchaFailedAttempts,
			}
		}
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password", details)
		return
	}
	if u.Disabled {
		writeError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "This account is disabled", nil)
		return
	}
	if s.emailVerificationRequiredForUser(r.Context(), u) {
		writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Email verification is required before login", nil)
		return
	}
	if s.userRequiresTOTP(r.Context(), u) {
		code := strings.TrimSpace(input.TOTPCode)
		if code == "" {
			writeError(w, http.StatusUnauthorized, "TWO_FACTOR_REQUIRED", "Two-factor authentication code is required", twoFactorRequiredDetails{TwoFactorRequired: true})
			return
		}
		if !totp.Validate(code, *u.TotpSecret) {
			writeError(w, http.StatusUnauthorized, "INVALID_TOTP", "Two-factor authentication code is invalid", twoFactorRequiredDetails{TwoFactorRequired: true})
			return
		}
	}
	if isAdmin(u) {
		s.clearAdminLoginFailures(r, username)
	}
	s.setSession(w, u.ID)
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(u)})
}

func (s *Server) userRequiresTOTP(ctx context.Context, u *ent.User) bool {
	return u != nil && s.twoFactorAuthEnabled(ctx) && u.TotpEnabled && u.TotpSecret != nil && strings.TrimSpace(*u.TotpSecret) != ""
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type passwordResetRecord struct {
	UserID    int       `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) handleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input passwordResetRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	email := strings.TrimSpace(input.Email)
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid email is required", nil)
		return
	}
	email = strings.TrimSpace(parsedEmail.Address)
	u, err := s.db.User.Query().Where(user.EmailEQ(email)).Only(r.Context())
	if err == nil && !u.Disabled {
		_ = s.createAndSendPasswordReset(r, u)
	}
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func (s *Server) createAndSendPasswordReset(r *http.Request, u *ent.User) error {
	if u.Email == nil || strings.TrimSpace(*u.Email) == "" {
		return nil
	}
	token, err := emailVerificationToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(passwordResetRecord{UserID: u.ID, ExpiresAt: time.Now().UTC().Add(passwordResetTTL)})
	if err != nil {
		return err
	}
	if err := s.setSetting(r.Context(), passwordResetSettingPrefix+tokenHash(token), string(payload)); err != nil {
		return err
	}
	resetURL := strings.TrimRight(s.sitePublicURL(r.Context()), "/") + "/login?mode=reset&token=" + token
	subject, textBody, htmlBody, err := s.renderMail(r.Context(), mailKindPasswordReset, mailRenderData{
		RecipientName: userDisplayName(u),
		ActionURL:     resetURL,
		Language:      r.Header.Get("Accept-Language"),
	})
	if err != nil {
		return err
	}
	return s.sendRenderedEmail(r.Context(), *u.Email, subject, textBody, htmlBody)
}

func (s *Server) handleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var input passwordResetConfirmRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	token := strings.TrimSpace(input.Token)
	password := strings.TrimSpace(input.NewPassword)
	if token == "" || len(password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Token and a password of at least 8 characters are required", nil)
		return
	}
	key := passwordResetSettingPrefix + tokenHash(token)
	record, err := s.db.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Password reset token not found or expired", nil)
		return
	}
	var payload passwordResetRecord
	if err := json.Unmarshal([]byte(record.Value), &payload); err != nil || payload.UserID <= 0 || time.Now().UTC().After(payload.ExpiresAt) {
		_ = s.db.SiteSetting.DeleteOneID(record.ID).Exec(r.Context())
		writeError(w, http.StatusNotFound, "TOKEN_NOT_FOUND", "Password reset token not found or expired", nil)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not reset password", nil)
		return
	}
	if _, err := s.db.User.UpdateOneID(payload.UserID).SetPasswordHash(hash).Save(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_RESET_FAILED", "Could not reset password", nil)
		return
	}
	_ = s.db.SiteSetting.DeleteOneID(record.ID).Exec(r.Context())
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

type twoFactorSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpAuthUrl"`
	QRDataURL  string `json:"qrDataUrl"`
}

type twoFactorEnableRequest struct {
	Secret string `json:"secret"`
	Code   string `json:"code"`
}

type twoFactorDisableRequest struct {
	CurrentPassword string `json:"currentPassword"`
}

func (s *Server) handleTwoFactorSetup(w http.ResponseWriter, r *http.Request, u *ent.User) {
	if !s.twoFactorAuthEnabled(r.Context()) {
		writeError(w, http.StatusForbidden, "TWO_FACTOR_DISABLED", "Two-factor authentication is disabled for this site", nil)
		return
	}
	accountName := u.Username
	if u.Email != nil && strings.TrimSpace(*u.Email) != "" {
		accountName = strings.TrimSpace(*u.Email)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.siteProfile(r.Context()).Title,
		AccountName: accountName,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TWO_FACTOR_SETUP_FAILED", "Could not create two-factor setup", nil)
		return
	}
	qr, err := key.Image(220, 220)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TWO_FACTOR_SETUP_FAILED", "Could not create two-factor QR code", nil)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, qr); err != nil {
		writeError(w, http.StatusInternalServerError, "TWO_FACTOR_SETUP_FAILED", "Could not encode two-factor QR code", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"setup": twoFactorSetupResponse{
		Secret:     key.Secret(),
		OTPAuthURL: key.URL(),
		QRDataURL:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}})
}

func (s *Server) handleTwoFactorEnable(w http.ResponseWriter, r *http.Request, u *ent.User) {
	if !s.twoFactorAuthEnabled(r.Context()) {
		writeError(w, http.StatusForbidden, "TWO_FACTOR_DISABLED", "Two-factor authentication is disabled for this site", nil)
		return
	}
	var input twoFactorEnableRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	secret := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.Secret), " ", ""))
	code := strings.TrimSpace(input.Code)
	if secret == "" || code == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Secret and verification code are required", nil)
		return
	}
	if !totp.Validate(code, secret) {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_TOTP", "Two-factor authentication code is invalid", nil)
		return
	}
	updated, err := s.db.User.UpdateOneID(u.ID).SetTotpSecret(secret).SetTotpEnabled(true).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TWO_FACTOR_ENABLE_FAILED", "Could not enable two-factor authentication", nil)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(updated)})
}

func (s *Server) handleTwoFactorDisable(w http.ResponseWriter, r *http.Request, u *ent.User) {
	var input twoFactorDisableRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, input.CurrentPassword) {
		writeError(w, http.StatusForbidden, "INVALID_PASSWORD", "Current password is incorrect", nil)
		return
	}
	updated, err := s.db.User.UpdateOneID(u.ID).SetTotpEnabled(false).ClearTotpSecret().Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TWO_FACTOR_DISABLE_FAILED", "Could not disable two-factor authentication", nil)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(updated)})
}

func (s *Server) handleAdminResetUserTwoFactor(w http.ResponseWriter, r *http.Request, _ *ent.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	updated, err := s.db.User.UpdateOneID(id).SetTotpEnabled(false).ClearTotpSecret().Save(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(updated)})
}

func (s *Server) adminLoginNow() time.Time {
	if s.authNow != nil {
		return s.authNow().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) recordAdminLoginFailure(r *http.Request, username string) adminLoginFailure {
	key := adminLoginFailureKey(r, username)
	now := s.adminLoginNow()
	s.adminLoginFailuresMu.Lock()
	defer s.adminLoginFailuresMu.Unlock()
	if s.adminLoginFailures == nil {
		s.adminLoginFailures = map[string]adminLoginFailure{}
	}
	s.pruneAdminLoginFailuresLocked(now)
	if _, exists := s.adminLoginFailures[key]; !exists && len(s.adminLoginFailures) >= maxAdminLoginFailureKeys {
		s.evictAdminLoginFailureLocked()
	}
	state := s.adminLoginFailures[key]
	state.Attempts++
	state.ExpiresAt = now.Add(adminLoginFailureTTL)
	if state.Attempts >= adminLoginBlockThreshold {
		state.BlockedUntil = now.Add(adminLoginBlockDuration)
	}
	s.adminLoginFailures[key] = state
	return state
}

func (s *Server) adminLoginFailureForRequest(r *http.Request, username string) (adminLoginFailure, bool) {
	key := adminLoginFailureKey(r, username)
	now := s.adminLoginNow()
	s.adminLoginFailuresMu.Lock()
	defer s.adminLoginFailuresMu.Unlock()
	s.pruneAdminLoginFailuresLocked(now)
	state, ok := s.adminLoginFailures[key]
	return state, ok
}

func (s *Server) pruneAdminLoginFailuresLocked(now time.Time) {
	for key, state := range s.adminLoginFailures {
		if !state.ExpiresAt.After(now) {
			delete(s.adminLoginFailures, key)
		}
	}
}

func (s *Server) evictAdminLoginFailureLocked() {
	oldestKey := ""
	var oldestExpiry time.Time
	for key, state := range s.adminLoginFailures {
		if oldestKey == "" || state.ExpiresAt.Before(oldestExpiry) || (state.ExpiresAt.Equal(oldestExpiry) && key < oldestKey) {
			oldestKey = key
			oldestExpiry = state.ExpiresAt
		}
	}
	if oldestKey != "" {
		delete(s.adminLoginFailures, oldestKey)
	}
}

func (s *Server) clearAdminLoginFailures(r *http.Request, username string) {
	key := adminLoginFailureKey(r, username)
	s.adminLoginFailuresMu.Lock()
	defer s.adminLoginFailuresMu.Unlock()
	delete(s.adminLoginFailures, key)
}

func writeAdminLoginRateLimit(w http.ResponseWriter, remaining time.Duration, state adminLoginFailure) {
	retryAfter := max(1, int(math.Ceil(remaining.Seconds())))
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeError(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "Too many administrator login attempts", loginFailureDetails{
		FailedAttempts:  state.Attempts,
		CaptchaRequired: true,
	})
}

func adminLoginFailureKey(r *http.Request, username string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	return strings.ToLower(strings.TrimSpace(username)) + "|" + host
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}

func emailVerificationToken() (string, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, u *ent.User) {
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(u)})
}

type updateProfileRequest struct {
	Nickname        *string `json:"nickname"`
	Email           *string `json:"email"`
	CurrentPassword string  `json:"currentPassword"`
	NewPassword     string  `json:"newPassword"`
}

func (s *Server) handleUpdateMyProfile(w http.ResponseWriter, r *http.Request, u *ent.User) {
	var input updateProfileRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	update := s.db.User.UpdateOneID(u.ID)
	if input.Nickname != nil {
		nickname := strings.TrimSpace(*input.Nickname)
		if len([]rune(nickname)) > 80 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Nickname must be 80 characters or fewer", nil)
			return
		}
		update.SetNickname(nickname)
	}
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		if email == "" {
			update.ClearEmail()
		} else {
			update.SetEmail(email)
		}
	}
	if strings.TrimSpace(input.NewPassword) != "" {
		if !auth.CheckPassword(u.PasswordHash, input.CurrentPassword) {
			writeError(w, http.StatusForbidden, "INVALID_PASSWORD", "Current password is incorrect", nil)
			return
		}
		password := strings.TrimSpace(input.NewPassword)
		if len(password) < 8 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Password must be at least 8 characters", nil)
			return
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not update password", nil)
			return
		}
		update.SetPasswordHash(hash)
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		if ent.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "USER_EXISTS", "Email is already registered", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "PROFILE_UPDATE_FAILED", "Could not update profile", nil)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{User: toPublicUser(updated)})
}

func (s *Server) handleUploadMyAvatar(w http.ResponseWriter, r *http.Request, u *ent.User) {
	if err := r.ParseMultipartForm(maxAvatarImageSize + 1<<20); err != nil {
		badRequest(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	if err := validateUploadedImage(file, header, maxAvatarImageSize); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	storageKey, err := s.uploadStorageKey(r.Context(), r.FormValue("storageKey"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "AVATAR_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	backend, err := s.storageBackendForKey(r.Context(), storageKey)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "AVATAR_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), backend, file, header.Filename, maxAvatarImageSize)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "AVATAR_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	oldStorageKey, oldStoragePath := u.AvatarStorageKey, u.AvatarStoragePath
	updated, err := s.db.User.UpdateOneID(u.ID).
		SetAvatarURL(s.absoluteURL(r.Context(), obj.DownloadURL)).
		SetAvatarStorageKey(storageKey).
		SetAvatarStoragePath(obj.Path).
		Save(r.Context())
	if err != nil {
		_ = backend.Delete(r.Context(), obj.Path)
		writeError(w, http.StatusInternalServerError, "AVATAR_SAVE_FAILED", "Could not save avatar", nil)
		return
	}
	if oldStoragePath != "" {
		s.deleteStoredObject(r.Context(), oldStorageKey, oldStoragePath)
	}
	writeJSON(w, http.StatusOK, avatarResponse{User: toPublicUser(updated), URL: updated.AvatarURL})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request, u *ent.User) {
	tokens, err := s.db.APIToken.Query().Where(apitoken.UserIDEQ(u.ID)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_LIST_FAILED", "Could not list API tokens", nil)
		return
	}
	writeJSON(w, http.StatusOK, apiTokensResponse{Tokens: tokens})
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
	writeJSON(w, http.StatusCreated, apiTokenCreateResponse{Token: token, Record: record})
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
	writeJSON(w, http.StatusOK, okResponse{OK: true})
}
