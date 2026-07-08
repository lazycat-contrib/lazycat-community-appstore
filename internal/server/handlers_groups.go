package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	groups, err := s.groupDTOs(r, records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_LIST_FAILED", "Could not list groups", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
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
	code, err := s.createGroupCode(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_CREATE_FAILED", "Could not create group", nil)
		return
	}
	now := time.Now()
	record, err := s.db.UserGroup.Create().
		SetOwnerID(u.ID).
		SetName(name).
		SetSlug(slug).
		SetDescription(strings.TrimSpace(input.Description)).
		SetCode(code).
		SetCodeUpdatedAt(now).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "GROUP_CREATE_FAILED", "Could not create group", nil)
		return
	}
	dto, err := s.groupDTO(r, record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_CREATE_FAILED", "Could not create group", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"group": dto})
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	groupID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if !s.canManageGroup(r, groupID, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot manage this group", nil)
		return
	}
	attached, err := s.db.AppVisibility.Query().Where(appvisibility.GroupIDEQ(groupID)).Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_DELETE_FAILED", "Could not delete group", nil)
		return
	}
	if attached > 0 {
		writeError(w, http.StatusConflict, "GROUP_IN_USE", "Groups attached to apps cannot be deleted. Rotate the group code instead.", nil)
		return
	}
	_, _ = s.db.GroupMember.Delete().Where(groupmember.GroupIDEQ(groupID)).Exec(r.Context())
	if err := s.db.UserGroup.DeleteOneID(groupID).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_DELETE_FAILED", "Could not delete group", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRotateGroupCode(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	groupID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if !s.canManageGroup(r, groupID, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot manage this group", nil)
		return
	}
	record, err := s.rotateGroupCode(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_CODE_ROTATE_FAILED", "Could not rotate group code", nil)
		return
	}
	dto, err := s.groupDTO(r, record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_CODE_ROTATE_FAILED", "Could not rotate group code", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group": dto})
}

type groupClientConfigRequest struct {
	SourceURL string `json:"sourceUrl"`
	GroupIDs  []int  `json:"groupIds"`
}

func (s *Server) handleGroupClientConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input groupClientConfigRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" {
		sourceURL = s.siteProfile(r.Context()).SourceURL
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Valid source URL is required", nil)
		return
	}
	records := make([]*entgo.UserGroup, 0, len(input.GroupIDs))
	seen := map[int]struct{}{}
	for _, groupID := range input.GroupIDs {
		if groupID <= 0 {
			continue
		}
		if _, exists := seen[groupID]; exists {
			continue
		}
		seen[groupID] = struct{}{}
		if !s.canManageGroup(r, groupID, u) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot export this group", nil)
			return
		}
		record, err := s.db.UserGroup.Get(r.Context(), groupID)
		if err != nil {
			writeError(w, http.StatusNotFound, "GROUP_NOT_FOUND", "Group not found", nil)
			return
		}
		record, err = s.ensureGroupCode(r.Context(), record)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "GROUP_CONFIG_FAILED", "Could not prepare group code", nil)
			return
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Select at least one group", nil)
		return
	}
	config := groupClientConfig{SourceURL: strings.TrimRight(parsed.String(), "/")}
	for _, record := range records {
		config.GroupCodes = append(config.GroupCodes, record.Code)
		config.Groups = append(config.Groups, sourceGroupDTO{ID: record.ID, Name: record.Name, Code: record.Code})
	}
	encoded, err := encodeGroupClientConfig(config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GROUP_CONFIG_FAILED", "Could not generate group config", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"encoded": encoded, "config": config, "preview": config})
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

func (s *Server) groupDTOs(r *http.Request, records []*entgo.UserGroup) ([]groupDTO, error) {
	out := make([]groupDTO, 0, len(records))
	for _, record := range records {
		dto, err := s.groupDTO(r, record)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Server) groupDTO(r *http.Request, record *entgo.UserGroup) (groupDTO, error) {
	var err error
	record, err = s.ensureGroupCode(r.Context(), record)
	if err != nil {
		return groupDTO{}, err
	}
	members, err := s.db.GroupMember.Query().Where(groupmember.GroupIDEQ(record.ID)).Count(r.Context())
	if err != nil {
		return groupDTO{}, err
	}
	attached, err := s.db.AppVisibility.Query().Where(appvisibility.GroupIDEQ(record.ID)).Count(r.Context())
	if err != nil {
		return groupDTO{}, err
	}
	return groupDTO{
		ID:               record.ID,
		OwnerID:          record.OwnerID,
		Name:             record.Name,
		Slug:             record.Slug,
		Description:      record.Description,
		Code:             record.Code,
		CodeUpdatedAt:    record.CodeUpdatedAt,
		MemberCount:      members,
		AttachedAppCount: attached,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}, nil
}
