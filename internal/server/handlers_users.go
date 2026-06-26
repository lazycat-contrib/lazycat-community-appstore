package server

import (
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/user"
)

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	records, err := s.db.User.Query().Order(entgo.Asc(user.FieldUsername)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "USER_LIST_FAILED", "Could not list users", nil)
		return
	}
	out := make([]publicUser, 0, len(records))
	for _, record := range records {
		out = append(out, toPublicUser(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

type updateUserRequest struct {
	Role          *string `json:"role"`
	EmailVerified *bool   `json:"emailVerified"`
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
	if input.Role != nil {
		role := user.Role(strings.ToUpper(strings.TrimSpace(*input.Role)))
		if err := user.RoleValidator(role); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid user role", nil)
			return
		}
		update.SetRole(role)
	}
	if input.EmailVerified != nil {
		update.SetEmailVerified(*input.EmailVerified)
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "USER_UPDATE_FAILED", "Could not update user", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toPublicUser(updated)})
}
