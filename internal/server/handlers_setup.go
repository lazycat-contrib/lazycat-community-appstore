package server

import (
	"context"
	"net/http"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
)

type setupStatusDTO struct {
	NeedsSetup bool `json:"needsSetup"`
}

type setupRequest struct {
	Username              string `json:"username"`
	Email                 string `json:"email"`
	Password              string `json:"password"`
	SourcePassword        string `json:"sourcePassword"`
	GitHubMirror          string `json:"githubMirror"`
	RequireEmailVerify    bool   `json:"requireEmailVerify"`
	SourcePasswordEnabled bool   `json:"sourcePasswordEnabled"`
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.needsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETUP_STATUS_FAILED", "Could not read setup status", nil)
		return
	}
	writeJSON(w, http.StatusOK, setupStatusDTO{NeedsSetup: needsSetup})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := s.needsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETUP_STATUS_FAILED", "Could not read setup status", nil)
		return
	}
	if !needsSetup {
		writeError(w, http.StatusConflict, "SETUP_ALREADY_DONE", "Setup has already been completed", nil)
		return
	}

	var input setupRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.SourcePassword = strings.TrimSpace(input.SourcePassword)
	input.GitHubMirror = strings.TrimRight(strings.TrimSpace(input.GitHubMirror), "/")
	if input.Username == "" || len(input.Password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Username and a password of at least 8 characters are required", nil)
		return
	}
	if input.Email != "" && !strings.Contains(input.Email, "@") {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Email address is invalid", nil)
		return
	}
	if input.GitHubMirror != "" {
		if err := validateSetting("github_mirror", input.GitHubMirror); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
	}
	if input.SourcePasswordEnabled && input.SourcePassword == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Source password is required when source protection is enabled", nil)
		return
	}

	settings := map[string]string{
		"require_email_verify": boolString(input.RequireEmailVerify),
	}
	if input.SourcePasswordEnabled {
		settings["source_password"] = input.SourcePassword
	} else {
		settings["source_password"] = ""
	}
	if input.GitHubMirror != "" {
		settings["github_mirror"] = input.GitHubMirror
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not create administrator", nil)
		return
	}

	tx, err := s.db.Tx(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETUP_SAVE_FAILED", "Could not start setup", nil)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	exists, err := tx.User.Query().Where(user.RoleEQ(user.RoleSITE_ADMIN)).Exist(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETUP_STATUS_FAILED", "Could not read setup status", nil)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "SETUP_ALREADY_DONE", "Setup has already been completed", nil)
		return
	}

	create := tx.User.Create().
		SetUsername(input.Username).
		SetPasswordHash(hash).
		SetRole(user.RoleSITE_ADMIN).
		SetEmailVerified(true)
	if input.Email != "" {
		create.SetEmail(input.Email)
	}
	admin, err := create.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "USER_EXISTS", "Username or email is already registered", nil)
		return
	}
	for key, value := range settings {
		if err := setSettingTx(r.Context(), tx, key, value); err != nil {
			writeError(w, http.StatusInternalServerError, "SETUP_SAVE_FAILED", "Could not save setup settings", nil)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "SETUP_SAVE_FAILED", "Could not finish setup", nil)
		return
	}
	committed = true

	s.setSession(w, admin.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"user": toPublicUser(admin), "setup": setupStatusDTO{NeedsSetup: false}})
}

func setSettingTx(ctx context.Context, tx *entgo.Tx, key, value string) error {
	record, err := tx.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(ctx)
	if err == nil {
		_, err = tx.SiteSetting.UpdateOneID(record.ID).SetValue(value).Save(ctx)
		return err
	}
	if !entgo.IsNotFound(err) {
		return err
	}
	_, err = tx.SiteSetting.Create().SetKey(key).SetValue(value).Save(ctx)
	return err
}

func (s *Server) needsSetup(ctx context.Context) (bool, error) {
	exists, err := s.db.User.Query().Where(user.RoleEQ(user.RoleSITE_ADMIN)).Exist(ctx)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
