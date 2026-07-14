package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/ad"
	"lazycat.community/appstore/internal/pagination"
)

type adRequest struct {
	Enabled   *bool   `json:"enabled,omitempty"`
	Title     *string `json:"title,omitempty"`
	Body      *string `json:"body,omitempty"`
	ImageURL  *string `json:"imageUrl,omitempty"`
	LinkLabel *string `json:"linkLabel,omitempty"`
	LinkURL   *string `json:"linkUrl,omitempty"`
	StartsAt  *string `json:"startsAt,omitempty"`
	EndsAt    *string `json:"endsAt,omitempty"`
	SortOrder *int    `json:"sortOrder,omitempty"`
}

type adDraft struct {
	Enabled   bool
	Title     string
	Body      string
	ImageURL  string
	LinkLabel string
	LinkURL   string
	StartsAt  *time.Time
	EndsAt    *time.Time
	SortOrder int
}

func (s *Server) handleListAds(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(r.Context(), 50, 200), 200)
	q := s.db.Ad.Query()
	total, err := q.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AD_LIST_FAILED", "Could not list ads", nil)
		return
	}
	records, err := q.
		Order(entgo.Asc(ad.FieldSortOrder), entgo.Desc(ad.FieldUpdatedAt)).
		Offset(page.Offset()).
		Limit(page.PageSize).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AD_LIST_FAILED", "Could not list ads", nil)
		return
	}
	out := make([]siteAd, 0, len(records))
	for _, record := range records {
		out = append(out, toSiteAd(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ads":        out,
		"pagination": page.Response(total),
	})
}

func (s *Server) handleCreateAd(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input adRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	draft := adDraft{Enabled: true}
	if err := applyAdRequest(&draft, input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validateAdDraft(draft); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	record, err := s.db.Ad.Create().
		SetEnabled(draft.Enabled).
		SetTitle(draft.Title).
		SetBody(draft.Body).
		SetImageURL(draft.ImageURL).
		SetLinkLabel(draft.LinkLabel).
		SetLinkURL(draft.LinkURL).
		SetNillableStartsAt(draft.StartsAt).
		SetNillableEndsAt(draft.EndsAt).
		SetSortOrder(draft.SortOrder).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AD_CREATE_FAILED", "Could not create ad", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ad": toSiteAd(record), "site": s.siteProfile(r.Context())})
}

func (s *Server) handleUpdateAd(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	record, err := s.db.Ad.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "AD_NOT_FOUND", "Ad not found", nil)
		return
	}
	var input adRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	draft := adDraft{
		Enabled:   record.Enabled,
		Title:     record.Title,
		Body:      record.Body,
		ImageURL:  record.ImageURL,
		LinkLabel: record.LinkLabel,
		LinkURL:   record.LinkURL,
		StartsAt:  record.StartsAt,
		EndsAt:    record.EndsAt,
		SortOrder: record.SortOrder,
	}
	if err := applyAdRequest(&draft, input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := validateAdDraft(draft); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	update := s.db.Ad.UpdateOneID(record.ID).
		SetEnabled(draft.Enabled).
		SetTitle(draft.Title).
		SetBody(draft.Body).
		SetImageURL(draft.ImageURL).
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
		writeError(w, http.StatusInternalServerError, "AD_UPDATE_FAILED", "Could not update ad", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ad": toSiteAd(updated), "site": s.siteProfile(r.Context())})
}

func (s *Server) handleDeleteAd(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		badRequest(w, err)
		return
	}
	if err := s.db.Ad.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusNotFound, "AD_NOT_FOUND", "Ad not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "site": s.siteProfile(r.Context())})
}

func applyAdRequest(draft *adDraft, input adRequest) error {
	if input.Enabled != nil {
		draft.Enabled = *input.Enabled
	}
	if input.Title != nil {
		draft.Title = strings.TrimSpace(*input.Title)
	}
	if input.Body != nil {
		draft.Body = strings.TrimSpace(*input.Body)
	}
	if input.ImageURL != nil {
		draft.ImageURL = cleanURLSetting(*input.ImageURL)
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

func validateAdDraft(draft adDraft) error {
	if draft.Title == "" && draft.Body == "" && draft.ImageURL == "" {
		return fmt.Errorf("ad title, body, or image URL is required")
	}
	if len([]rune(draft.Title)) > 120 {
		return fmt.Errorf("ad title must be 120 characters or fewer")
	}
	if len([]rune(draft.Body)) > 600 {
		return fmt.Errorf("ad body must be 600 characters or fewer")
	}
	if len([]rune(draft.LinkLabel)) > 60 {
		return fmt.Errorf("ad link label must be 60 characters or fewer")
	}
	if !isHTTPURLOrEmpty(draft.ImageURL) {
		return fmt.Errorf("ad image URL must be an http or https URL")
	}
	if !isHTTPURLOrEmpty(draft.LinkURL) {
		return fmt.Errorf("ad link URL must be an http or https URL")
	}
	if draft.StartsAt != nil && draft.EndsAt != nil && !draft.EndsAt.After(*draft.StartsAt) {
		return fmt.Errorf("ad end time must be after the start time")
	}
	return nil
}

func (s *Server) activeSiteAdsAt(ctx context.Context, now time.Time) []siteAd {
	records, err := s.db.Ad.Query().
		Where(
			ad.EnabledEQ(true),
			ad.Or(ad.StartsAtIsNil(), ad.StartsAtLTE(now)),
			ad.Or(ad.EndsAtIsNil(), ad.EndsAtGT(now)),
		).
		Order(entgo.Asc(ad.FieldSortOrder), entgo.Desc(ad.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return nil
	}
	out := make([]siteAd, 0, len(records))
	for _, record := range records {
		item := toSiteAd(record)
		if item.Title == "" && item.Body == "" && item.ImageURL == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func toSiteAd(record *entgo.Ad) siteAd {
	item := siteAd{
		ID:        record.ID,
		Enabled:   record.Enabled,
		Title:     strings.TrimSpace(record.Title),
		Body:      strings.TrimSpace(record.Body),
		ImageURL:  cleanURLSetting(record.ImageURL),
		LinkLabel: strings.TrimSpace(record.LinkLabel),
		LinkURL:   cleanURLSetting(record.LinkURL),
		SortOrder: record.SortOrder,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: record.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if record.StartsAt != nil {
		item.StartsAt = record.StartsAt.UTC().Format(time.RFC3339)
	}
	if record.EndsAt != nil {
		item.EndsAt = record.EndsAt.UTC().Format(time.RFC3339)
	}
	return item
}
