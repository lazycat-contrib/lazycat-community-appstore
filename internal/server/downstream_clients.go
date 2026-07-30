package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/downstreamclientuser"
	"lazycat.community/appstore/internal/pagination"
)

const (
	downstreamSeenComment = "COMMENT"
	downstreamSeenWish    = "WISH"
)

type downstreamClientDTO struct {
	ID             int        `json:"id"`
	ClientUserID   string     `json:"clientUserId"`
	DisplayName    string     `json:"displayName"`
	SeenInComments bool       `json:"seenInComments"`
	SeenInWishes   bool       `json:"seenInWishes"`
	Blocked        bool       `json:"blocked"`
	BlockReason    string     `json:"blockReason,omitempty"`
	BlockedBy      *int       `json:"blockedBy,omitempty"`
	BlockedAt      *time.Time `json:"blockedAt,omitempty"`
	FirstSeenAt    time.Time  `json:"firstSeenAt"`
	LastSeenAt     time.Time  `json:"lastSeenAt"`
}

func (s *Server) observeDownstreamClient(ctx context.Context, clientUserID, displayName, source string) error {
	clientUserID = sanitizeIdentity(clientUserID)
	if !strings.HasPrefix(clientUserID, "lc_") || len(clientUserID) < 6 {
		return nil
	}
	displayName = sanitizeDisplayName(displayName)
	now := s.currentTime().UTC()
	record, err := s.db.DownstreamClientUser.Query().
		Where(downstreamclientuser.ClientUserIDEQ(clientUserID)).
		Only(ctx)
	if entgo.IsNotFound(err) {
		create := s.db.DownstreamClientUser.Create().
			SetClientUserID(clientUserID).
			SetDisplayName(displayName).
			SetFirstSeenAt(now).
			SetLastSeenAt(now).
			SetCreatedAt(now).
			SetUpdatedAt(now)
		if source == downstreamSeenComment {
			create.SetSeenInComments(true)
		}
		if source == downstreamSeenWish {
			create.SetSeenInWishes(true)
		}
		_, err = create.Save(ctx)
		if entgo.IsConstraintError(err) {
			return s.observeDownstreamClient(ctx, clientUserID, displayName, source)
		}
		return err
	}
	if err != nil {
		return err
	}
	update := s.db.DownstreamClientUser.UpdateOne(record).
		SetLastSeenAt(now).
		SetUpdatedAt(now)
	if displayName != "" {
		update.SetDisplayName(displayName)
	}
	if source == downstreamSeenComment {
		update.SetSeenInComments(true)
	}
	if source == downstreamSeenWish {
		update.SetSeenInWishes(true)
	}
	_, err = update.Save(ctx)
	return err
}

func (s *Server) clientUserBlocked(ctx context.Context, clientUserID string) (bool, error) {
	clientUserID = sanitizeIdentity(clientUserID)
	if clientUserID == "" {
		return false, nil
	}
	record, err := s.db.DownstreamClientUser.Query().
		Where(downstreamclientuser.ClientUserIDEQ(clientUserID)).
		Only(ctx)
	if entgo.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return record.Blocked, nil
}

func (s *Server) rejectBlockedClient(w http.ResponseWriter, r *http.Request, clientUserID string) bool {
	blocked, err := s.clientUserBlocked(r.Context(), clientUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CLIENT_ACCESS_CHECK_FAILED", "Could not check client access", nil)
		return true
	}
	if blocked {
		writeError(w, http.StatusForbidden, "CLIENT_BLOCKED", "This client user is blocked", nil)
		return true
	}
	return false
}

func (s *Server) handleListDownstreamClients(w http.ResponseWriter, r *http.Request, _ *entgo.User) {
	page := pagination.FromRequest(r, 40, 100)
	query := s.db.DownstreamClientUser.Query()
	if raw := strings.TrimSpace(r.URL.Query().Get("blocked")); raw != "" {
		blocked, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "blocked must be true or false", nil)
			return
		}
		query.Where(downstreamclientuser.BlockedEQ(blocked))
	}
	if search := strings.TrimSpace(r.URL.Query().Get("search")); search != "" {
		query.Where(downstreamclientuser.Or(
			downstreamclientuser.ClientUserIDContainsFold(search),
			downstreamclientuser.DisplayNameContainsFold(search),
		))
	}
	total, err := query.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DOWNSTREAM_CLIENT_LIST_FAILED", "Could not list downstream clients", nil)
		return
	}
	records, err := query.Order(entgo.Desc(downstreamclientuser.FieldLastSeenAt), entgo.Desc(downstreamclientuser.FieldID)).
		Offset(page.Offset()).Limit(page.PageSize).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DOWNSTREAM_CLIENT_LIST_FAILED", "Could not list downstream clients", nil)
		return
	}
	items := make([]downstreamClientDTO, 0, len(records))
	for _, record := range records {
		items = append(items, downstreamClientToDTO(record))
	}
	writeJSON(w, http.StatusOK, pagination.NewPage(items, page, total))
}

func downstreamClientToDTO(record *entgo.DownstreamClientUser) downstreamClientDTO {
	return downstreamClientDTO{
		ID: record.ID, ClientUserID: record.ClientUserID, DisplayName: record.DisplayName,
		SeenInComments: record.SeenInComments, SeenInWishes: record.SeenInWishes,
		Blocked: record.Blocked, BlockReason: record.BlockReason, BlockedBy: record.BlockedBy,
		BlockedAt: record.BlockedAt, FirstSeenAt: record.FirstSeenAt, LastSeenAt: record.LastSeenAt,
	}
}

func (s *Server) handleBlockDownstreamClient(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	clientUserID := sanitizeIdentity(r.PathValue("clientUserId"))
	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Block reason must be between 1 and 500 characters", nil)
		return
	}
	record, err := s.db.DownstreamClientUser.Query().Where(downstreamclientuser.ClientUserIDEQ(clientUserID)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "DOWNSTREAM_CLIENT_NOT_FOUND", "Downstream client not found", nil)
		return
	}
	now := s.currentTime().UTC()
	record, err = s.db.DownstreamClientUser.UpdateOne(record).
		SetBlocked(true).SetBlockReason(reason).SetBlockedBy(u.ID).SetBlockedAt(now).SetUpdatedAt(now).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DOWNSTREAM_CLIENT_UPDATE_FAILED", "Could not block downstream client", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": downstreamClientToDTO(record)})
}

func (s *Server) handleUnblockDownstreamClient(w http.ResponseWriter, r *http.Request, _ *entgo.User) {
	clientUserID := sanitizeIdentity(r.PathValue("clientUserId"))
	record, err := s.db.DownstreamClientUser.Query().Where(downstreamclientuser.ClientUserIDEQ(clientUserID)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "DOWNSTREAM_CLIENT_NOT_FOUND", "Downstream client not found", nil)
		return
	}
	record, err = s.db.DownstreamClientUser.UpdateOne(record).
		SetBlocked(false).SetBlockReason("").ClearBlockedBy().ClearBlockedAt().SetUpdatedAt(s.currentTime().UTC()).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DOWNSTREAM_CLIENT_UPDATE_FAILED", "Could not unblock downstream client", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client": downstreamClientToDTO(record)})
}
