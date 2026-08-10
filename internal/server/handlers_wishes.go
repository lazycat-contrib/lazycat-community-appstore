package server

import (
	"context"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/wish"
	"lazycat.community/appstore/ent/wishreply"
	"lazycat.community/appstore/ent/wishstatusevent"
	"lazycat.community/appstore/internal/pagination"
)

type createWishInput struct {
	Kind         wish.Kind `json:"kind"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	ReferenceURL string    `json:"referenceUrl,omitempty"`
	ContactEmail string    `json:"contactEmail,omitempty"`
	ContactOther string    `json:"contactOther,omitempty"`
	StatusText   string    `json:"statusText"`
}

type wishDTO struct {
	ID             int                  `json:"id"`
	Kind           wish.Kind            `json:"kind"`
	Status         wish.Status          `json:"status"`
	Title          string               `json:"title"`
	Body           string               `json:"body"`
	ReferenceURL   string               `json:"referenceUrl,omitempty"`
	ContactEmail   string               `json:"contactEmail,omitempty"`
	ContactOther   string               `json:"contactOther,omitempty"`
	ClientUserID   string               `json:"clientUserId,omitempty"`
	CanManage      bool                 `json:"canManage"`
	AuthorName     string               `json:"authorName"`
	Replies        []wishReplyDTO       `json:"replies"`
	StatusHistory  []wishStatusEventDTO `json:"statusHistory"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	LastActivityAt time.Time            `json:"lastActivityAt"`
}

type wishReplyDTO struct {
	ID         int       `json:"id"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type wishStatusEventDTO struct {
	ID        int                       `json:"id"`
	From      string                    `json:"fromStatus,omitempty"`
	To        wishstatusevent.ToStatus  `json:"toStatus"`
	ActorType wishstatusevent.ActorType `json:"actorType"`
	ActorName string                    `json:"actorName"`
	Text      string                    `json:"text"`
	CreatedAt time.Time                 `json:"createdAt"`
}

func (s *Server) handleCreateWish(w http.ResponseWriter, r *http.Request) {
	actor, status, code, message := s.resolveWishClientActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	if s.rejectBlockedClient(w, r, actor.ClientUserID) {
		return
	}
	var input createWishInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	if message := validateWishInput(&input); message != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, nil)
		return
	}
	if err := s.observeDownstreamClient(r.Context(), actor.ClientUserID, actor.DisplayName, downstreamSeenWish); err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_CREATE_FAILED", "Could not record wish author", nil)
		return
	}
	record, err := s.createWish(r.Context(), actor, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_CREATE_FAILED", "Could not create wish", nil)
		return
	}
	dto, err := s.wishToDTO(r.Context(), record, wishViewer{ClientUserID: actor.ClientUserID, OwnerID: actor.OwnerID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_LOAD_FAILED", "Could not load wish", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"wish": dto})
}

func validateWishInput(input *createWishInput) string {
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.ReferenceURL = strings.TrimSpace(input.ReferenceURL)
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	input.ContactOther = strings.TrimSpace(input.ContactOther)
	input.StatusText = strings.TrimSpace(input.StatusText)
	if wish.KindValidator(input.Kind) != nil {
		return "Wish kind is invalid"
	}
	if utf8.RuneCountInString(input.Title) < 1 || utf8.RuneCountInString(input.Title) > 160 {
		return "Title must be between 1 and 160 characters"
	}
	if utf8.RuneCountInString(input.Body) < 1 || utf8.RuneCountInString(input.Body) > 5000 {
		return "Description must be between 1 and 5000 characters"
	}
	if utf8.RuneCountInString(input.StatusText) < 1 || utf8.RuneCountInString(input.StatusText) > 1000 {
		return "Status text must be between 1 and 1000 characters"
	}
	if utf8.RuneCountInString(input.ReferenceURL) > 500 || utf8.RuneCountInString(input.ContactOther) > 500 {
		return "Reference or contact details are too long"
	}
	if input.ReferenceURL != "" {
		parsed, err := url.ParseRequestURI(input.ReferenceURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "Reference URL must be a valid HTTP or HTTPS URL"
		}
	}
	if input.Kind != wish.KindAPP_REQUEST {
		input.ReferenceURL = ""
	}
	if utf8.RuneCountInString(input.ContactEmail) > 254 {
		return "Email address is too long"
	}
	if input.Kind == wish.KindCUSTOMIZATION {
		address, err := mail.ParseAddress(input.ContactEmail)
		if err != nil || address.Address != input.ContactEmail || !strings.Contains(input.ContactEmail, "@") {
			return "A valid contact email is required for customization requests"
		}
	} else {
		input.ContactEmail = ""
		input.ContactOther = ""
	}
	return ""
}

func (s *Server) createWish(ctx context.Context, actor wishActor, input createWishInput) (*entgo.Wish, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	now := s.currentTime().UTC()
	record, err := tx.Wish.Create().
		SetKind(input.Kind).SetStatus(wish.StatusOPEN).SetTitle(input.Title).SetBody(input.Body).
		SetReferenceURL(input.ReferenceURL).SetContactEmail(input.ContactEmail).SetContactOther(input.ContactOther).
		SetClientUserID(actor.ClientUserID).SetOwnerID(actor.OwnerID).SetAuthorName(actor.DisplayName).
		SetCreatedAt(now).SetUpdatedAt(now).SetLastActivityAt(now).Save(ctx)
	if err != nil {
		return nil, err
	}
	_, err = tx.WishStatusEvent.Create().SetWishID(record.ID).SetToStatus(wishstatusevent.ToStatusOPEN).
		SetActorType(wishstatusevent.ActorTypeCLIENT).SetActorClientUserID(actor.ClientUserID).
		SetActorName(actor.DisplayName).SetText(input.StatusText).SetCreatedAt(now).Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.db.Wish.Get(ctx, record.ID)
}

func (s *Server) handleListWishes(w http.ResponseWriter, r *http.Request) {
	viewer := s.wishViewerForRequest(r)
	if viewer.ClientUserID != "" && s.rejectBlockedClient(w, r, viewer.ClientUserID) {
		return
	}
	page := pagination.FromRequest(r, 24, 100)
	query := s.db.Wish.Query()
	if !viewer.isAdmin() {
		if viewer.OwnerID == "" || viewer.ClientUserID == "" {
			query.Where(wish.KindIn(wish.KindAPP_REQUEST, wish.KindCUSTOMIZATION))
		} else {
			query.Where(wish.Or(
				wish.KindIn(wish.KindAPP_REQUEST, wish.KindCUSTOMIZATION),
				wish.And(wish.KindEQ(wish.KindSUGGESTION), wish.Or(
					wish.OwnerIDEQ(viewer.OwnerID),
					wish.And(wish.OwnerIDEQ(""), wish.ClientUserIDEQ(viewer.ClientUserID)),
				)),
			))
		}
	}
	if rawKind := strings.TrimSpace(r.URL.Query().Get("kind")); rawKind != "" {
		kind := wish.Kind(rawKind)
		if wish.KindValidator(kind) != nil || (kind == wish.KindSUGGESTION && !viewer.isAdmin() && viewer.OwnerID == "") {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Wish kind is invalid", nil)
			return
		}
		query.Where(wish.KindEQ(kind))
	}
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" {
		status := wish.Status(rawStatus)
		if wish.StatusValidator(status) != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Wish status is invalid", nil)
			return
		}
		query.Where(wish.StatusEQ(status))
	}
	total, err := query.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_LIST_FAILED", "Could not list wishes", nil)
		return
	}
	records, err := query.Order(entgo.Desc(wish.FieldLastActivityAt), entgo.Desc(wish.FieldID)).
		Offset(page.Offset()).Limit(page.PageSize).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_LIST_FAILED", "Could not list wishes", nil)
		return
	}
	items := make([]wishDTO, 0, len(records))
	for _, record := range records {
		dto, err := s.wishToDTO(r.Context(), record, viewer)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "WISH_LIST_FAILED", "Could not list wishes", nil)
			return
		}
		items = append(items, dto)
	}
	response := map[string]any{"items": items, "pagination": page.Response(total)}
	if viewer.isAdmin() {
		pendingCount, err := s.db.Wish.Query().Where(wish.StatusIn(wish.StatusOPEN, wish.StatusPLANNED, wish.StatusIN_PROGRESS)).Count(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "WISH_LIST_FAILED", "Could not count pending wishes", nil)
			return
		}
		response["pendingCount"] = pendingCount
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleUpdateOwnWish(w http.ResponseWriter, r *http.Request) {
	actor, status, code, message := s.resolveWishClientActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	if s.rejectBlockedClient(w, r, actor.ClientUserID) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	record, err := s.db.Wish.Get(r.Context(), id)
	if err != nil || record.OwnerID == "" || record.OwnerID != actor.OwnerID {
		writeError(w, http.StatusNotFound, "WISH_NOT_FOUND", "Wish not found", nil)
		return
	}
	var input struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		ReferenceURL string `json:"referenceUrl,omitempty"`
		ContactEmail string `json:"contactEmail,omitempty"`
		ContactOther string `json:"contactOther,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	validated := createWishInput{Kind: record.Kind, Title: input.Title, Body: input.Body, ReferenceURL: input.ReferenceURL,
		ContactEmail: input.ContactEmail, ContactOther: input.ContactOther, StatusText: "unchanged"}
	if message := validateWishInput(&validated); message != "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, nil)
		return
	}
	now := s.currentTime().UTC()
	record, err = s.db.Wish.UpdateOne(record).SetTitle(validated.Title).SetBody(validated.Body).
		SetReferenceURL(validated.ReferenceURL).SetContactEmail(validated.ContactEmail).SetContactOther(validated.ContactOther).
		SetUpdatedAt(now).SetLastActivityAt(now).Save(r.Context())
	if err != nil {
		writeError(w, 500, "WISH_UPDATE_FAILED", "Could not update wish", nil)
		return
	}
	dto, err := s.wishToDTO(r.Context(), record, wishViewer{ClientUserID: actor.ClientUserID, OwnerID: actor.OwnerID})
	if err != nil {
		writeError(w, 500, "WISH_LOAD_FAILED", "Could not load wish", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wish": dto})
}

func (s *Server) handleDeleteOwnWish(w http.ResponseWriter, r *http.Request) {
	actor, status, code, message := s.resolveWishClientActor(r)
	if status != 0 {
		writeError(w, status, code, message, nil)
		return
	}
	if s.rejectBlockedClient(w, r, actor.ClientUserID) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	record, err := s.db.Wish.Get(r.Context(), id)
	if err != nil || record.OwnerID == "" || record.OwnerID != actor.OwnerID {
		writeError(w, http.StatusNotFound, "WISH_NOT_FOUND", "Wish not found", nil)
		return
	}
	tx, err := s.db.Tx(r.Context())
	if err != nil {
		writeError(w, 500, "WISH_DELETE_FAILED", "Could not delete wish", nil)
		return
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.WishReply.Delete().Where(wishreply.WishIDEQ(id)).Exec(r.Context())
	if err == nil {
		_, err = tx.WishStatusEvent.Delete().Where(wishstatusevent.WishIDEQ(id)).Exec(r.Context())
	}
	if err == nil {
		err = tx.Wish.DeleteOneID(id).Exec(r.Context())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeError(w, 500, "WISH_DELETE_FAILED", "Could not delete wish", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetWish(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	viewer := s.wishViewerForRequest(r)
	if viewer.ClientUserID != "" && s.rejectBlockedClient(w, r, viewer.ClientUserID) {
		return
	}
	record, err := s.db.Wish.Get(r.Context(), id)
	if err != nil || (record.Kind == wish.KindSUGGESTION && !viewerCanReadSuggestion(record, viewer)) {
		writeError(w, http.StatusNotFound, "WISH_NOT_FOUND", "Wish not found", nil)
		return
	}
	dto, err := s.wishToDTO(r.Context(), record, viewer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WISH_LOAD_FAILED", "Could not load wish", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wish": dto})
}

func viewerCanReadSuggestion(record *entgo.Wish, viewer wishViewer) bool {
	if viewer.isAdmin() {
		return true
	}
	if viewer.OwnerID == "" || viewer.ClientUserID == "" {
		return false
	}
	if record.OwnerID != "" {
		return record.OwnerID == viewer.OwnerID
	}
	// Historical wishes predate device-scoped ownership. Keep them visible to
	// the original account, but read-only because the creating device cannot be
	// reconstructed safely.
	return record.ClientUserID == viewer.ClientUserID
}

func (s *Server) wishToDTO(ctx context.Context, record *entgo.Wish, viewer wishViewer) (wishDTO, error) {
	replies, err := s.db.WishReply.Query().Where(wishreply.WishIDEQ(record.ID)).
		Order(entgo.Asc(wishreply.FieldCreatedAt), entgo.Asc(wishreply.FieldID)).All(ctx)
	if err != nil {
		return wishDTO{}, err
	}
	events, err := s.db.WishStatusEvent.Query().Where(wishstatusevent.WishIDEQ(record.ID)).
		Order(entgo.Asc(wishstatusevent.FieldCreatedAt), entgo.Asc(wishstatusevent.FieldID)).All(ctx)
	if err != nil {
		return wishDTO{}, err
	}
	canManage := viewer.OwnerID != "" && record.OwnerID != "" && viewer.OwnerID == record.OwnerID
	dto := wishDTO{ID: record.ID, Kind: record.Kind, Status: record.Status, Title: record.Title, Body: record.Body,
		ReferenceURL: record.ReferenceURL, AuthorName: record.AuthorName,
		Replies: make([]wishReplyDTO, 0, len(replies)), StatusHistory: make([]wishStatusEventDTO, 0, len(events)),
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, LastActivityAt: record.LastActivityAt, CanManage: canManage}
	legacyOwner := record.OwnerID == "" && viewer.ClientUserID != "" && viewer.ClientUserID == record.ClientUserID
	private := viewer.isAdmin() || canManage || legacyOwner
	if private {
		dto.ClientUserID, dto.ContactEmail, dto.ContactOther = record.ClientUserID, record.ContactEmail, record.ContactOther
	}
	for _, reply := range replies {
		dto.Replies = append(dto.Replies, wishReplyDTO{ID: reply.ID, AuthorName: reply.AuthorName, Body: reply.Body, CreatedAt: reply.CreatedAt, UpdatedAt: reply.UpdatedAt})
	}
	for _, event := range events {
		dto.StatusHistory = append(dto.StatusHistory, wishStatusEventDTO{ID: event.ID, From: event.FromStatus, To: event.ToStatus,
			ActorType: event.ActorType, ActorName: event.ActorName, Text: event.Text, CreatedAt: event.CreatedAt})
	}
	return dto, nil
}

func (s *Server) handleCreateWishReply(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	var input struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || utf8.RuneCountInString(body) > 5000 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Reply must be between 1 and 5000 characters", nil)
		return
	}
	if _, err := s.db.Wish.Get(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "WISH_NOT_FOUND", "Wish not found", nil)
		return
	}
	tx, err := s.db.Tx(r.Context())
	if err != nil {
		writeError(w, 500, "WISH_REPLY_FAILED", "Could not create reply", nil)
		return
	}
	defer func() { _ = tx.Rollback() }()
	now := s.currentTime().UTC()
	reply, err := tx.WishReply.Create().SetWishID(id).SetAuthorUserID(u.ID).SetAuthorName(userDisplayName(u)).SetBody(body).SetCreatedAt(now).SetUpdatedAt(now).Save(r.Context())
	if err == nil {
		_, err = tx.Wish.UpdateOneID(id).SetLastActivityAt(now).SetUpdatedAt(now).Save(r.Context())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeError(w, 500, "WISH_REPLY_FAILED", "Could not create reply", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"reply": wishReplyDTO{ID: reply.ID, AuthorName: reply.AuthorName, Body: reply.Body, CreatedAt: reply.CreatedAt, UpdatedAt: reply.UpdatedAt}})
}

func (s *Server) handleUpdateWishStatus(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	var input struct {
		Status     wish.Status `json:"status"`
		StatusText string      `json:"statusText"`
	}
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.StatusText = strings.TrimSpace(input.StatusText)
	if wish.StatusValidator(input.Status) != nil || input.StatusText == "" || utf8.RuneCountInString(input.StatusText) > 1000 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid status and status text are required", nil)
		return
	}
	record, err := s.db.Wish.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "WISH_NOT_FOUND", "Wish not found", nil)
		return
	}
	tx, err := s.db.Tx(r.Context())
	if err != nil {
		writeError(w, 500, "WISH_STATUS_FAILED", "Could not update status", nil)
		return
	}
	defer func() { _ = tx.Rollback() }()
	now := s.currentTime().UTC()
	event, err := tx.WishStatusEvent.Create().SetWishID(id).SetFromStatus(record.Status.String()).
		SetToStatus(wishstatusevent.ToStatus(input.Status)).SetActorType(wishstatusevent.ActorTypeUSER).
		SetActorUserID(u.ID).SetActorName(userDisplayName(u)).SetText(input.StatusText).SetCreatedAt(now).Save(r.Context())
	if err == nil {
		_, err = tx.Wish.UpdateOneID(id).SetStatus(input.Status).SetLastActivityAt(now).SetUpdatedAt(now).Save(r.Context())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeError(w, 500, "WISH_STATUS_FAILED", "Could not update status", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"statusEvent": wishStatusEventDTO{ID: event.ID, From: event.FromStatus, To: event.ToStatus, ActorType: event.ActorType, ActorName: event.ActorName, Text: event.Text, CreatedAt: event.CreatedAt}})
}
