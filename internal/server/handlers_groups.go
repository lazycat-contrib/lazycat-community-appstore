package server

import (
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/groupmember"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/ent/usergroup"
)

type groupRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	q := s.db.UserGroup.Query().Order(entgo.Asc(usergroup.FieldName))
	if !isAdmin(u) {
		q.Where(usergroup.OwnerIDEQ(u.ID))
	}
	records, err := q.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_LIST_FAILED", "Could not list groups", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": records})
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input groupRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Group name is required", nil)
		return
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	record, err := s.db.UserGroup.Create().
		SetOwnerID(u.ID).
		SetName(name).
		SetSlug(slug).
		SetDescription(input.Description).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "GROUP_CREATE_FAILED", "Could not create group", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": record})
}

func (s *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	groupID, userID, ok := s.groupMemberPath(w, r)
	if !ok {
		return
	}
	if !s.canManageGroup(r, groupID, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot manage this group", nil)
		return
	}
	if exists, err := s.db.User.Query().Where(user.IDEQ(userID)).Exist(r.Context()); err != nil || !exists {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
		return
	}
	record, err := s.db.GroupMember.Create().SetGroupID(groupID).SetUserID(userID).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "GROUP_MEMBER_CREATE_FAILED", "Could not add group member", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": record})
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	groupID, userID, ok := s.groupMemberPath(w, r)
	if !ok {
		return
	}
	if !s.canManageGroup(r, groupID, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot manage this group", nil)
		return
	}
	_, _ = s.db.GroupMember.Delete().Where(groupmember.GroupIDEQ(groupID), groupmember.UserIDEQ(userID)).Exec(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type appVisibilityRequest struct {
	GroupIDs []int `json:"groupIds"`
}

func (s *Server) handleSetAppVisibility(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app owners can set visibility", nil)
		return
	}
	var input appVisibilityRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	for _, groupID := range input.GroupIDs {
		if groupID <= 0 {
			continue
		}
		if !s.canManageGroup(r, groupID, u) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot assign this group", nil)
			return
		}
	}
	_, _ = s.db.AppVisibility.Delete().Where(appvisibility.AppIDEQ(appID)).Exec(r.Context())
	for _, groupID := range input.GroupIDs {
		if groupID <= 0 {
			continue
		}
		_, _ = s.db.AppVisibility.Create().SetAppID(appID).SetGroupID(groupID).Save(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"groupIds": s.visibleGroupIDs(r.Context(), appID)})
}

func (s *Server) groupMemberPath(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	groupID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return 0, 0, false
	}
	userID, err := strconv.Atoi(r.PathValue("userId"))
	if err != nil {
		badRequest(w, err)
		return 0, 0, false
	}
	return groupID, userID, true
}

func (s *Server) canManageGroup(r *http.Request, groupID int, u *entgo.User) bool {
	if isAdmin(u) {
		return true
	}
	record, err := s.db.UserGroup.Get(r.Context(), groupID)
	return err == nil && record.OwnerID == u.ID
}
