package server

import (
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/pagination"
)

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(r.Context(), 50, 200), 200)
	q := s.db.User.Query()
	total, err := q.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "USER_LIST_FAILED", "Could not list users", nil)
		return
	}
	records, err := q.Order(entgo.Asc(user.FieldUsername)).
		Offset(page.Offset()).
		Limit(page.PageSize).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "USER_LIST_FAILED", "Could not list users", nil)
		return
	}
	out := make([]publicUser, 0, len(records))
	for _, record := range records {
		out = append(out, toPublicUser(record))
	}
	writeJSON(w, http.StatusOK, pagination.NewUsersPage(out, page, total))
}

type updateUserRequest struct {
	Username      *string `json:"username"`
	Nickname      *string `json:"nickname"`
	Email         *string `json:"email"`
	Password      *string `json:"password"`
	Role          *string `json:"role"`
	EmailVerified *bool   `json:"emailVerified"`
	Disabled      *bool   `json:"disabled"`
}

type createUserRequest struct {
	Username      string  `json:"username"`
	Nickname      string  `json:"nickname"`
	Email         *string `json:"email"`
	Password      string  `json:"password"`
	Role          string  `json:"role"`
	EmailVerified bool    `json:"emailVerified"`
	Disabled      bool    `json:"disabled"`
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input createUserRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	username := strings.TrimSpace(input.Username)
	nickname := strings.TrimSpace(input.Nickname)
	if err := validateManagedUsername(username); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if len([]rune(nickname)) > 80 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Nickname must be 80 characters or fewer", nil)
		return
	}
	password := strings.TrimSpace(input.Password)
	if len(password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Password must be at least 8 characters", nil)
		return
	}
	role, err := normalizeUserRole(input.Role)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid user role", nil)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not create user", nil)
		return
	}
	create := s.db.User.Create().
		SetUsername(username).
		SetNickname(nickname).
		SetPasswordHash(hash).
		SetRole(role).
		SetEmailVerified(input.EmailVerified).
		SetDisabled(input.Disabled)
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		if email != "" {
			create.SetEmail(email)
		}
	}
	created, err := create.Save(r.Context())
	if err != nil {
		if entgo.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "USER_EXISTS", "Username or email is already registered", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "USER_CREATE_FAILED", "Could not create user", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": toPublicUser(created)})
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	var input updateUserRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	update := s.db.User.UpdateOneID(id)
	if input.Username != nil {
		username := strings.TrimSpace(*input.Username)
		if err := validateManagedUsername(username); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		update.SetUsername(username)
	}
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
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" {
		password := strings.TrimSpace(*input.Password)
		if len(password) < 8 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Password must be at least 8 characters", nil)
			return
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "Could not update user", nil)
			return
		}
		update.SetPasswordHash(hash)
	}
	if input.Role != nil {
		role, err := normalizeUserRole(*input.Role)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid user role", nil)
			return
		}
		update.SetRole(role)
	}
	if input.EmailVerified != nil {
		update.SetEmailVerified(*input.EmailVerified)
	}
	if input.Disabled != nil {
		if id == u.ID && *input.Disabled {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "You cannot disable your own account", nil)
			return
		}
		if err := s.ensureNotLastSiteAdmin(r, id, *input.Disabled, input.Role); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		update.SetDisabled(*input.Disabled)
	} else if input.Role != nil {
		if err := s.ensureNotLastSiteAdmin(r, id, false, input.Role); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		if entgo.IsConstraintError(err) {
			writeError(w, http.StatusConflict, "USER_EXISTS", "Username or email is already registered", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "USER_UPDATE_FAILED", "Could not update user", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toPublicUser(updated)})
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if id == u.ID {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "You cannot delete your own account", nil)
		return
	}
	record, err := s.db.User.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}
	if record.Role == user.RoleSITE_ADMIN {
		count, err := s.db.User.Query().Where(user.RoleEQ(user.RoleSITE_ADMIN), user.DisabledEQ(false), user.IDNEQ(id)).Count(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "USER_DELETE_FAILED", "Could not delete user", nil)
			return
		}
		if count == 0 {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "At least one active site admin is required", nil)
			return
		}
	}
	if err := s.db.User.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "USER_DELETE_FAILED", "Could not delete user", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func validateManagedUsername(username string) error {
	if username == "" {
		return errString("Username is required")
	}
	if len(username) > 80 {
		return errString("Username must be 80 characters or fewer")
	}
	return nil
}

func normalizeUserRole(value string) (user.Role, error) {
	role := user.Role(strings.ToUpper(strings.TrimSpace(value)))
	if role == "" {
		role = user.RoleUSER
	}
	if err := user.RoleValidator(role); err != nil {
		return "", err
	}
	return role, nil
}

func (s *Server) ensureNotLastSiteAdmin(r *http.Request, id int, disabled bool, roleInput *string) error {
	current, err := s.db.User.Get(r.Context(), id)
	if err != nil {
		return errString("User not found")
	}
	nextRole := current.Role
	if roleInput != nil {
		role, err := normalizeUserRole(*roleInput)
		if err != nil {
			return err
		}
		nextRole = role
	}
	if current.Role != user.RoleSITE_ADMIN || (nextRole == user.RoleSITE_ADMIN && !disabled) {
		return nil
	}
	count, err := s.db.User.Query().Where(user.RoleEQ(user.RoleSITE_ADMIN), user.DisabledEQ(false), user.IDNEQ(id)).Count(r.Context())
	if err != nil {
		return err
	}
	if count == 0 {
		return errString("At least one active site admin is required")
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }
