package server

import (
	"net/http"
	"strconv"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/collaboratorrequest"
	"lazycat.community/appstore/ent/favorite"
	"lazycat.community/appstore/ent/outdatedmark"
)

type commentRequest struct {
	Body string `json:"body"`
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !record.CommentsEnabled {
		writeError(w, http.StatusForbidden, "COMMENTS_DISABLED", "Comments are disabled for this app", nil)
		return
	}
	var input commentRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if input.Body == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Comment body is required", nil)
		return
	}
	created, err := s.db.Comment.Create().SetAppID(id).SetUserID(u.ID).SetBody(input.Body).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_CREATE_FAILED", "Could not create comment", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"comment": created})
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.Comment.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found", nil)
		return
	}
	appRecord, _ := s.db.App.Get(r.Context(), record.AppID)
	if record.UserID != u.ID && !isAdmin(u) && (appRecord == nil || appRecord.OwnerID != u.ID) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot delete this comment", nil)
		return
	}
	_, err = s.db.Comment.UpdateOneID(id).SetDeleted(true).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_DELETE_FAILED", "Could not delete comment", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleToggleFavorite(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	s.toggleFavorite(w, r, u, favorite.TargetTypeAPP, id)
}

func (s *Server) handleToggleSubmitterFavorite(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	if _, err := s.db.User.Get(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "Submitter not found", nil)
		return
	}
	s.toggleFavorite(w, r, u, favorite.TargetTypeSUBMITTER, id)
}

func (s *Server) toggleFavorite(w http.ResponseWriter, r *http.Request, u *entgo.User, targetType favorite.TargetType, targetID int) {
	existing, err := s.db.Favorite.Query().Where(favorite.UserIDEQ(u.ID), favorite.TargetTypeEQ(targetType), favorite.TargetIDEQ(targetID)).Only(r.Context())
	if err == nil {
		_ = s.db.Favorite.DeleteOneID(existing.ID).Exec(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{"favorited": false})
		return
	}
	_, err = s.db.Favorite.Create().SetUserID(u.ID).SetTargetType(targetType).SetTargetID(targetID).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FAVORITE_FAILED", "Could not update favorite", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"favorited": true})
}

func (s *Server) handleListFavorites(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	records, err := s.db.Favorite.Query().Where(favorite.UserIDEQ(u.ID)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FAVORITE_LIST_FAILED", "Could not list favorites", nil)
		return
	}
	apps := []appSummary{}
	submitters := []publicUser{}
	for _, record := range records {
		switch record.TargetType {
		case favorite.TargetTypeAPP:
			appRecord, err := s.db.App.Get(r.Context(), record.TargetID)
			if err == nil && appRecord.Status == app.StatusAPPROVED && s.userCanSeeApp(r, appRecord, u) {
				apps = append(apps, s.appSummaryDTO(r, appRecord, u))
			}
		case favorite.TargetTypeSUBMITTER:
			submitter, err := s.db.User.Get(r.Context(), record.TargetID)
			if err == nil {
				submitters = append(submitters, toPublicUser(submitter))
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps, "submitters": submitters})
}

type outdatedRequest struct {
	Note string `json:"note"`
}

func (s *Server) handleMarkOutdated(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	var input outdatedRequest
	_ = decodeJSON(r, &input)
	existing, err := s.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(id), outdatedmark.UserIDEQ(u.ID)).Only(r.Context())
	if err == nil {
		updated, err := s.db.OutdatedMark.UpdateOneID(existing.ID).SetNote(input.Note).Save(r.Context())
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"outdatedMark": updated})
			return
		}
	}
	created, err := s.db.OutdatedMark.Create().SetAppID(id).SetUserID(u.ID).SetNote(input.Note).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OUTDATED_MARK_FAILED", "Could not mark app as outdated", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"outdatedMark": created})
}

func (s *Server) handleClearOutdated(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	_, _ = s.db.OutdatedMark.Delete().Where(outdatedmark.AppIDEQ(id), outdatedmark.UserIDEQ(u.ID)).Exec(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type collaboratorRequestBody struct {
	Message string `json:"message"`
}

func (s *Server) handleCreateCollaboratorRequest(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if record.OwnerID == u.ID || s.isCollaborator(r, record.ID, u.ID) {
		writeError(w, http.StatusConflict, "COLLABORATOR_REQUEST_FAILED", "You already maintain this app", nil)
		return
	}
	var input collaboratorRequestBody
	_ = decodeJSON(r, &input)
	created, err := s.db.CollaboratorRequest.Create().SetAppID(id).SetUserID(u.ID).SetMessage(input.Message).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "COLLABORATOR_REQUEST_FAILED", "Could not create collaborator request", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"request": s.collaboratorRequestDTO(r, created)})
}

func (s *Server) handleListCollaboratorRequests(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	appRecord, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && appRecord.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app owners can view collaborator requests", nil)
		return
	}
	records, err := s.db.CollaboratorRequest.Query().Where(collaboratorrequest.AppIDEQ(id)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COLLABORATOR_REQUEST_LIST_FAILED", "Could not list collaborator requests", nil)
		return
	}
	out := make([]collaboratorRequestDTO, 0, len(records))
	for _, record := range records {
		out = append(out, s.collaboratorRequestDTO(r, record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": out})
}

func (s *Server) handleApproveCollaboratorRequest(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.decideCollaboratorRequest(w, r, u, true)
}

func (s *Server) handleRejectCollaboratorRequest(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.decideCollaboratorRequest(w, r, u, false)
}

func (s *Server) decideCollaboratorRequest(w http.ResponseWriter, r *http.Request, u *entgo.User, approve bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.CollaboratorRequest.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "COLLABORATOR_REQUEST_NOT_FOUND", "Collaborator request not found", nil)
		return
	}
	appRecord, err := s.db.App.Get(r.Context(), record.AppID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && appRecord.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app owners can decide collaborator requests", nil)
		return
	}
	status := collaboratorrequest.StatusREJECTED
	if approve {
		status = collaboratorrequest.StatusAPPROVED
		_, _ = s.db.Collaborator.Create().SetAppID(record.AppID).SetUserID(record.UserID).Save(r.Context())
	}
	updated, err := s.db.CollaboratorRequest.UpdateOneID(id).SetStatus(status).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COLLABORATOR_REQUEST_UPDATE_FAILED", "Could not update collaborator request", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"request": s.collaboratorRequestDTO(r, updated)})
}

func (s *Server) collaboratorRequestDTO(r *http.Request, record *entgo.CollaboratorRequest) collaboratorRequestDTO {
	dto := collaboratorRequestDTO{
		ID:        record.ID,
		AppID:     record.AppID,
		UserID:    record.UserID,
		UserIDRaw: record.UserID,
		Status:    string(record.Status),
		Message:   record.Message,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
	if requester, err := s.db.User.Get(r.Context(), record.UserID); err == nil {
		dto.Username = requester.Username
		dto.Email = requester.Email
	}
	return dto
}
