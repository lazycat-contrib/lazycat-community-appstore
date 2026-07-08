package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/announcement"
	"lazycat.community/appstore/internal/pagination"
)

type announcementRequest struct {
	Enabled   *bool   `json:"enabled,omitempty"`
	Level     *string `json:"level,omitempty"`
	Title     *string `json:"title,omitempty"`
	Body      *string `json:"body,omitempty"`
	LinkLabel *string `json:"linkLabel,omitempty"`
	LinkURL   *string `json:"linkUrl,omitempty"`
	StartsAt  *string `json:"startsAt,omitempty"`
	EndsAt    *string `json:"endsAt,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
}

type announcementDraft struct {
	Enabled   bool
	Level     string
	Title     string
	Body      string
	LinkLabel string
	LinkURL   string
	StartsAt  *time.Time
	EndsAt    *time.Time
	SortOrder int
}

func (s *Server) handleListAnnouncements(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(r.Context(), 50, 200), 200)
	q := s.db.Announcement.Query()
	total, err := q.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANNOUNCEMENT_LIST_FAILED", "Could not list announcements", nil)
		return
	}
	records, err := q.
		Order(entgo.Asc(announcement.FieldSortOrder), entgo.Desc(announcement.FieldUpdatedAt)).
		Offset(page.Offset()).
		Limit(page.PageSize).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANNOUNCEMENT_LIST_FAILED", "Could not list announcements", nil)
		return
	}
	out := make([]siteAnnouncement, 0, len(records))
	for _, record := range records {
		out = append(out, toSiteAnnouncement(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"announcements": out,
		"pagination":    page.Response(total),
	})
}

func (s *Server) handleCreateAnnouncement(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input announcementRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	draft := announcementDraft{Enabled: true, Level: "info"}
	if err := applyAnnouncementRequest(&draft, input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validateAnnouncementDraft(draft); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	record, err := s.db.Announcement.Create().
		SetEnabled(draft.Enabled).
		SetLevel(announcement.Level(draft.Level)).
		SetTitle(draft.Title).
		SetBody(draft.Body).
		SetLinkLabel(draft.LinkLabel).
		SetLinkURL(draft.LinkURL).
		SetNillableStartsAt(draft.StartsAt).
		SetNillableEndsAt(draft.EndsAt).
		SetSortOrder(draft.SortOrder).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANNOUNCEMENT_CREATE_FAILED", "Could not create announcement", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"announcement": toSiteAnnouncement(record), "site": s.siteProfile(r.Context())})
}

func (s *Server) handleUpdateAnnouncement(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	record, err := s.db.Announcement.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "ANNOUNCEMENT_NOT_FOUND", "Announcement not found", nil)
		return
	}
	var input announcementRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	draft := announcementDraft{
		Enabled:   record.Enabled,
		Level:     string(record.Level),
		Title:     record.Title,
		Body:      record.Body,
		LinkLabel: record.LinkLabel,
		LinkURL:   record.LinkURL,
		StartsAt:  record.StartsAt,
		EndsAt:    record.EndsAt,
		SortOrder: record.SortOrder,
	}
	if err := applyAnnouncementRequest(&draft, input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validateAnnouncementDraft(draft); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	update := s.db.Announcement.UpdateOneID(record.ID).
		SetEnabled(draft.Enabled).
		SetLevel(announcement.Level(draft.Level)).
		SetTitle(draft.Title).
		SetBody(draft.Body).
		SetLinkLabel(draft.LinkLabel).
		SetLinkURL(draft.LinkURL).
		SetSortOrder(draft.SortOrder)
	if draft.StartsAt != nil {
		update.SetStartsAt(*draft.StartsAt)
	} else {
		update.ClearStartsAt()
	}
	if draft.EndsAt != nil {
		update.SetEndsAt(*draft.EndsAt)
	} else {
		update.ClearEndsAt()
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANNOUNCEMENT_UPDATE_FAILED", "Could not update announcement", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"announcement": toSiteAnnouncement(updated), "site": s.siteProfile(r.Context())})
}

func (s *Server) handleDeleteAnnouncement(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	if err := s.db.Announcement.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusNotFound, "ANNOUNCEMENT_NOT_FOUND", "Announcement not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "site": s.siteProfile(r.Context())})
}

func applyAnnouncementRequest(draft *announcementDraft, input announcementRequest) error {
	if input.Enabled != nil {
		draft.Enabled = *input.Enabled
	}
	if input.Level != nil {
		draft.Level = strings.ToLower(strings.TrimSpace(*input.Level))
	}
	if input.Title != nil {
		draft.Title = strings.TrimSpace(*input.Title)
	}
	if input.Body != nil {
		draft.Body = strings.TrimSpace(*input.Body)
	}
	if input.LinkLabel != nil {
		draft.LinkLabel = strings.TrimSpace(*input.LinkLabel)
	}
	if input.LinkURL != nil {
		draft.LinkURL = cleanURLSetting(*input.LinkURL)
	}
	if input.StartsAt != nil {
		startsAt, err := parseAnnouncementTime(*input.StartsAt)
		if err != nil {
			return fmt.Errorf("startsAt must be an RFC3339 timestamp")
		}
		draft.StartsAt = startsAt
	}
	if input.EndsAt != nil {
		endsAt, err := parseAnnouncementTime(*input.EndsAt)
		if err != nil {
			return fmt.Errorf("endsAt must be an RFC3339 timestamp")
		}
		draft.EndsAt = endsAt
	}
	if input.SortOrder != nil {
		draft.SortOrder = *input.SortOrder
	}
	return nil
}

func validateAnnouncementDraft(draft announcementDraft) error {
	if !validAnnouncementLevel(draft.Level) {
		return fmt.Errorf("announcement level must be info, warning, or success")
	}
	if draft.Title == "" && draft.Body == "" {
		return fmt.Errorf("announcement title or body is required")
	}
	if len([]rune(draft.Title)) > 120 {
		return fmt.Errorf("announcement title must be 120 characters or fewer")
	}
	if len([]rune(draft.Body)) > 600 {
		return fmt.Errorf("announcement body must be 600 characters or fewer")
	}
	if len([]rune(draft.LinkLabel)) > 60 {
		return fmt.Errorf("announcement link label must be 60 characters or fewer")
	}
	if !isHTTPURLOrEmpty(draft.LinkURL) {
		return fmt.Errorf("announcement link URL must be an http or https URL")
	}
	if draft.StartsAt != nil && draft.EndsAt != nil && !draft.EndsAt.After(*draft.StartsAt) {
		return fmt.Errorf("announcement end time must be after the start time")
	}
	return nil
}

func parseAnnouncementTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func (s *Server) activeSiteAnnouncements(ctx context.Context) []siteAnnouncement {
	total, err := s.db.Announcement.Query().Count(ctx)
	if err != nil || total == 0 {
		legacy := s.legacySiteAnnouncement(ctx)
		if legacy.Enabled && (legacy.Title != "" || legacy.Body != "") {
			return []siteAnnouncement{legacy}
		}
		return nil
	}
	now := time.Now().UTC()
	records, err := s.db.Announcement.Query().
		Where(
			announcement.EnabledEQ(true),
			announcement.Or(announcement.StartsAtIsNil(), announcement.StartsAtLTE(now)),
			announcement.Or(announcement.EndsAtIsNil(), announcement.EndsAtGT(now)),
		).
		Order(entgo.Asc(announcement.FieldSortOrder), entgo.Desc(announcement.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil
	}
	out := make([]siteAnnouncement, 0, len(records))
	for _, record := range records {
		item := toSiteAnnouncement(record)
		if item.Title == "" && item.Body == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func toSiteAnnouncement(record *entgo.Announcement) siteAnnouncement {
	item := siteAnnouncement{
		ID:        record.ID,
		Enabled:   record.Enabled,
		Level:     string(record.Level),
		Title:     strings.TrimSpace(record.Title),
		Body:      strings.TrimSpace(record.Body),
		LinkLabel: strings.TrimSpace(record.LinkLabel),
		LinkURL:   cleanURLSetting(record.LinkURL),
		SortOrder: record.SortOrder,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: record.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.Level == "" || !validAnnouncementLevel(item.Level) {
		item.Level = "info"
	}
	if record.StartsAt != nil {
		item.StartsAt = record.StartsAt.UTC().Format(time.RFC3339)
	}
	if record.EndsAt != nil {
		item.EndsAt = record.EndsAt.UTC().Format(time.RFC3339)
	}
	return item
}
