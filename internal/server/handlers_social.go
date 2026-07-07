package server

import (
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/collaboratorrequest"
	commentpkg "lazycat.community/appstore/ent/comment"
	commentnotificationpkg "lazycat.community/appstore/ent/commentnotification"
	"lazycat.community/appstore/ent/favorite"
	"lazycat.community/appstore/ent/outdatedmark"
)

type commentRequest struct {
	Body        string `json:"body"`
	ParentID    *int   `json:"parentId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type commentActor struct {
	User         *entgo.User
	UserID       int
	ClientUserID string
	DisplayName  string
	IsClient     bool
}

func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	u := s.optionalUser(r)
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	comments, err := s.loadComments(r, record.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_LIST_FAILED", "Could not list comments", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	actor, status, code, message := s.resolveCommentActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, actor.User) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.commentsAllowed(r.Context(), record.CommentsEnabled) {
		writeError(w, http.StatusForbidden, "COMMENTS_DISABLED", "Comments are disabled for this app", nil)
		return
	}
	var input commentRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	body := trimRunes(strings.TrimSpace(input.Body), 4000)
	if body == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Comment body is required", nil)
		return
	}
	var parentID *int
	if input.ParentID != nil && *input.ParentID > 0 {
		parent, err := s.db.Comment.Query().
			Where(commentpkg.IDEQ(*input.ParentID), commentpkg.AppIDEQ(id), commentpkg.DeletedEQ(false)).
			Only(r.Context())
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Parent comment does not exist", nil)
			return
		}
		if parent.ParentID != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Replies can only target top-level comments", nil)
			return
		}
		parentID = input.ParentID
	}
	create := s.db.Comment.Create().
		SetAppID(id).
		SetUserID(actor.UserID).
		SetAuthorName(actor.DisplayName).
		SetBody(body).
		SetNillableParentID(parentID)
	if actor.IsClient {
		create.SetAuthorType(commentpkg.AuthorTypeCLIENT).SetClientUserID(actor.ClientUserID)
	} else {
		create.SetAuthorType(commentpkg.AuthorTypeUSER)
	}
	created, err := create.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_CREATE_FAILED", "Could not create comment", nil)
		return
	}
	s.createCommentNotification(r, record, created, actor)
	writeJSON(w, http.StatusCreated, map[string]any{"comment": s.commentDTO(r, created, actor, s.actorCanMaintainApp(actor, record))})
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	actor, status, code, message := s.resolveCommentActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	record, err := s.db.Comment.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "Comment not found", nil)
		return
	}
	appRecord, _ := s.db.App.Get(r.Context(), record.AppID)
	if appRecord == nil || !s.actorCanDeleteComment(actor, record, appRecord) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot delete this comment", nil)
		return
	}
	_, err = s.db.Comment.Update().
		Where(commentpkg.Or(commentpkg.IDEQ(id), commentpkg.ParentIDEQ(id))).
		SetDeleted(true).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_DELETE_FAILED", "Could not delete comment", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListCommentNotifications(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	query := s.db.CommentNotification.Query().
		Where(commentnotificationpkg.OwnerIDEQ(u.ID)).
		Order(entgo.Desc(commentnotificationpkg.FieldCreatedAt)).
		Limit(80)
	if r.URL.Query().Get("unreadOnly") == "true" {
		query.Where(commentnotificationpkg.ReadEQ(false))
	}
	records, err := query.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_NOTIFICATION_LIST_FAILED", "Could not list comment notifications", nil)
		return
	}
	out := make([]commentNotificationDTO, 0, len(records))
	for _, record := range records {
		out = append(out, commentNotificationDTO{
			ID:        record.ID,
			AppID:     record.AppID,
			CommentID: record.CommentID,
			AppName:   record.AppName,
			ActorName: record.ActorName,
			Body:      record.Body,
			Read:      record.Read,
			CreatedAt: record.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

func (s *Server) handleReadCommentNotification(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	updated, err := s.db.CommentNotification.UpdateOneID(id).
		Where(commentnotificationpkg.OwnerIDEQ(u.ID)).
		SetRead(true).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "COMMENT_NOTIFICATION_NOT_FOUND", "Comment notification not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notification": commentNotificationDTO{
		ID:        updated.ID,
		AppID:     updated.AppID,
		CommentID: updated.CommentID,
		AppName:   updated.AppName,
		ActorName: updated.ActorName,
		Body:      updated.Body,
		Read:      updated.Read,
		CreatedAt: updated.CreatedAt,
	}})
}

func (s *Server) handleReadAllCommentNotifications(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	count, err := s.db.CommentNotification.Update().
		Where(commentnotificationpkg.OwnerIDEQ(u.ID), commentnotificationpkg.ReadEQ(false)).
		SetRead(true).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "COMMENT_NOTIFICATION_UPDATE_FAILED", "Could not update comment notifications", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": count})
}

func (s *Server) resolveCommentActor(r *http.Request) (commentActor, int, string, string) {
	if u, ok := s.authenticate(r); ok && !s.emailVerificationRequiredForUser(r.Context(), u) {
		return commentActor{User: u, UserID: u.ID, DisplayName: userDisplayName(u)}, 0, "", ""
	}
	if !s.cfg.TrustLazyCatClientComments || r.Header.Get("X-LazyCat-Client-Proxy") != "lazycat-appstore-client" || sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID")) == "" {
		return commentActor{}, http.StatusUnauthorized, "LAZYCAT_CLIENT_REQUIRED", "Comments from clients require the LazyCat app store client"
	}
	clientUserID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-User-ID"))
	if clientUserID == "" {
		return commentActor{}, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required"
	}
	if !s.sourcePasswordAllowsClientComment(r) {
		return commentActor{}, http.StatusUnauthorized, "SOURCE_PASSWORD_REQUIRED", "A valid source password is required"
	}
	displayName := sanitizeDisplayName(r.Header.Get("X-LazyCat-Client-Display-Name"))
	if displayName == "" {
		displayName = "LazyCat " + trimRunes(clientUserID, 12)
	}
	return commentActor{UserID: 0, ClientUserID: clientUserID, DisplayName: displayName, IsClient: true}, 0, "", ""
}

func (s *Server) optionalCommentActor(r *http.Request) commentActor {
	actor, status, _, _ := s.resolveCommentActor(r)
	if status != 0 {
		return commentActor{}
	}
	return actor
}

func (s *Server) sourcePasswordAllowsClientComment(r *http.Request) bool {
	sourcePassword := s.sourcePassword(r.Context())
	if sourcePassword == "" {
		return true
	}
	password := r.Header.Get("X-Source-Password")
	if password == "" {
		password = r.URL.Query().Get("password")
	}
	return password == sourcePassword
}

func (s *Server) actorCanMaintainApp(actor commentActor, appRecord *entgo.App) bool {
	return actor.User != nil && (isAdmin(actor.User) || appRecord.OwnerID == actor.User.ID)
}

func (s *Server) actorCanDeleteComment(actor commentActor, record *entgo.Comment, appRecord *entgo.App) bool {
	if s.actorCanMaintainApp(actor, appRecord) {
		return true
	}
	if actor.User != nil && record.AuthorType == commentpkg.AuthorTypeUSER && record.UserID == actor.User.ID {
		return true
	}
	return actor.IsClient && record.AuthorType == commentpkg.AuthorTypeCLIENT && record.ClientUserID != "" && record.ClientUserID == actor.ClientUserID
}

func (s *Server) createCommentNotification(r *http.Request, appRecord *entgo.App, created *entgo.Comment, actor commentActor) {
	if actor.User != nil && actor.User.ID == appRecord.OwnerID {
		return
	}
	_, _ = s.db.CommentNotification.Create().
		SetOwnerID(appRecord.OwnerID).
		SetAppID(appRecord.ID).
		SetCommentID(created.ID).
		SetAppName(appRecord.Name).
		SetActorName(actor.DisplayName).
		SetBody(trimRunes(created.Body, 180)).
		Save(r.Context())
	s.emailAppOwnerNotification(r, appRecord, actor.DisplayName, "New comment on "+appRecord.Name, created.Body)
}

func (s *Server) emailAppOwnerNotification(r *http.Request, appRecord *entgo.App, actorName, subject, body string) {
	if !appRecord.EmailNotificationsEnabled {
		return
	}
	owner, err := s.db.User.Get(r.Context(), appRecord.OwnerID)
	if err != nil || owner.Email == nil || strings.TrimSpace(*owner.Email) == "" {
		return
	}
	appURL := strings.TrimRight(s.sitePublicURL(r.Context()), "/") + "/"
	message := strings.TrimSpace(body)
	if message == "" {
		message = subject
	}
	mailBody := "App: " + appRecord.Name + "\n" +
		"From: " + actorName + "\n\n" +
		message + "\n\n" +
		"Open the store backend to review this app:\n" + appURL + "\n"
	_ = s.sendEmail(r.Context(), *owner.Email, subject, mailBody)
}

func sanitizeIdentity(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	return trimRunes(value, 128)
}

func sanitizeDisplayName(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return trimRunes(value, 64)
}

func trimRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
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
	Note             string `json:"note"`
	InstalledVersion string `json:"installedVersion"`
	ExpectedVersion  string `json:"expectedVersion"`
}

func (s *Server) handleMarkOutdated(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	actor, status, code, message := s.resolveOutdatedActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, actor.User) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	var input outdatedRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	note, ok := validateOutdatedNote(w, input)
	if !ok {
		return
	}
	existing, err := s.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(id), outdatedmark.UserIDEQ(actor.UserID)).Only(r.Context())
	if err == nil {
		updated, err := s.db.OutdatedMark.UpdateOneID(existing.ID).SetNote(note).Save(r.Context())
		if err == nil {
			if actor.User == nil || record.OwnerID != actor.User.ID {
				s.emailAppOwnerNotification(r, record, actor.DisplayName, "Update requested for "+record.Name, note)
			}
			count, _ := s.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(id)).Count(r.Context())
			writeJSON(w, http.StatusOK, map[string]any{"outdatedMark": updated, "outdatedMarked": true, "outdatedMarks": count})
			return
		}
	}
	created, err := s.db.OutdatedMark.Create().SetAppID(id).SetUserID(actor.UserID).SetNote(note).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OUTDATED_MARK_FAILED", "Could not mark app as outdated", nil)
		return
	}
	if actor.User == nil || record.OwnerID != actor.User.ID {
		s.emailAppOwnerNotification(r, record, actor.DisplayName, "Update requested for "+record.Name, note)
	}
	count, _ := s.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(id)).Count(r.Context())
	writeJSON(w, http.StatusCreated, map[string]any{"outdatedMark": created, "outdatedMarked": true, "outdatedMarks": count})
}

func (s *Server) handleClearOutdated(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	actor, status, code, message := s.resolveOutdatedActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, actor.User) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.manualOutdatedClearAllowed(r.Context()) {
		writeError(w, http.StatusForbidden, "OUTDATED_CLEAR_DISABLED", "Manual outdated clearing is disabled", nil)
		return
	}
	if !s.actorCanMaintainApp(actor, record) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the app maintainer or an admin can clear outdated marks", nil)
		return
	}
	_, _ = s.db.OutdatedMark.Delete().Where(outdatedmark.AppIDEQ(id)).Exec(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "outdatedMarked": false, "outdatedMarks": 0})
}

func (s *Server) resolveOutdatedActor(r *http.Request) (commentActor, int, string, string) {
	if u, ok := s.authenticate(r); ok && !s.emailVerificationRequiredForUser(r.Context(), u) {
		return commentActor{User: u, UserID: u.ID, DisplayName: userDisplayName(u)}, 0, "", ""
	}
	if !s.cfg.TrustLazyCatClientComments || r.Header.Get("X-LazyCat-Client-Proxy") != "lazycat-appstore-client" || sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID")) == "" {
		return commentActor{}, http.StatusUnauthorized, "LAZYCAT_CLIENT_REQUIRED", "Outdated marks require the LazyCat app store client"
	}
	clientUserID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-User-ID"))
	if clientUserID == "" {
		return commentActor{}, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required"
	}
	if !s.sourcePasswordAllowsClientComment(r) {
		return commentActor{}, http.StatusUnauthorized, "SOURCE_PASSWORD_REQUIRED", "A valid source password is required"
	}
	displayName := sanitizeDisplayName(r.Header.Get("X-LazyCat-Client-Display-Name"))
	if displayName == "" {
		displayName = "LazyCat " + trimRunes(clientUserID, 12)
	}
	return commentActor{UserID: outdatedClientUserID(clientUserID), ClientUserID: clientUserID, DisplayName: displayName, IsClient: true}, 0, "", ""
}

func validateOutdatedNote(w http.ResponseWriter, input outdatedRequest) (string, bool) {
	reason := strings.TrimSpace(input.Note)
	if reason == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Outdated reason is required", nil)
		return "", false
	}
	reason = trimRunes(reason, 1000)
	installedVersion := trimRunes(strings.TrimSpace(input.InstalledVersion), 80)
	expectedVersion := trimRunes(strings.TrimSpace(input.ExpectedVersion), 80)
	parts := []string{"Reason:\n" + reason}
	if installedVersion != "" {
		parts = append(parts, "Current or installed version: "+installedVersion)
	}
	if expectedVersion != "" {
		parts = append(parts, "Expected newer version or source: "+expectedVersion)
	}
	return strings.Join(parts, "\n\n"), true
}

func outdatedClientUserID(clientUserID string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(clientUserID))
	return -int(hash.Sum32()&0x3fffffff) - 1
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
		dto.Username = userDisplayName(requester)
		dto.Email = requester.Email
	}
	return dto
}
