package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/ent/tag"
)

func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	q := s.db.ReviewRequest.Query().Order(entgo.Desc(reviewrequest.FieldCreatedAt)).Limit(200)
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		q.Where(reviewrequest.StatusEQ(reviewrequest.Status(status)))
	}
	records, err := q.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "REVIEW_LIST_FAILED", "Could not list reviews", nil)
		return
	}
	out := make([]reviewDTO, 0, len(records))
	for _, record := range records {
		out = append(out, reviewDTO{
			ID:         record.ID,
			Kind:       string(record.Kind),
			Status:     string(record.Status),
			AppID:      record.AppID,
			VersionID:  record.VersionID,
			Requester:  record.RequesterID,
			ReviewerID: record.ReviewerID,
			Note:       record.Note,
			ReviewNote: record.ReviewNote,
			ReviewedAt: record.ReviewedAt,
			CreatedAt:  record.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": out})
}

type reviewDecisionRequest struct {
	Note string `json:"note"`
}

func (s *Server) handleApproveReview(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.decideReview(w, r, u, true)
}

func (s *Server) handleRejectReview(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.decideReview(w, r, u, false)
}

func (s *Server) decideReview(w http.ResponseWriter, r *http.Request, u *entgo.User, approve bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	var input reviewDecisionRequest
	_ = decodeJSON(r, &input)
	record, err := s.db.ReviewRequest.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "REVIEW_NOT_FOUND", "Review request not found", nil)
		return
	}
	now := time.Now()
	status := reviewrequest.StatusREJECTED
	if approve {
		status = reviewrequest.StatusAPPROVED
	}
	updated, err := s.db.ReviewRequest.UpdateOneID(record.ID).
		SetStatus(status).
		SetReviewerID(u.ID).
		SetReviewNote(input.Note).
		SetReviewedAt(now).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "REVIEW_UPDATE_FAILED", "Could not update review", nil)
		return
	}
	if record.AppID != nil {
		switch record.Kind {
		case reviewrequest.KindAPP_SUBMISSION:
			appStatus := app.StatusREJECTED
			if approve {
				appStatus = app.StatusAPPROVED
			}
			_, _ = s.db.App.UpdateOneID(*record.AppID).SetStatus(appStatus).Save(r.Context())
		case reviewrequest.KindAPP_INFO_UPDATE:
			if approve {
				var payload updateAppJSON
				if err := json.Unmarshal([]byte(record.Note), &payload); err != nil {
					writeError(w, http.StatusInternalServerError, "APP_INFO_REVIEW_FAILED", "Could not read app update payload", nil)
					return
				}
				if _, err := s.applyAppInfoUpdate(r, *record.AppID, payload); err != nil {
					writeError(w, http.StatusInternalServerError, "APP_INFO_REVIEW_FAILED", "Could not apply app update payload", nil)
					return
				}
			}
		}
	}
	if record.VersionID != nil {
		versionStatus := appversion.StatusREJECTED
		update := s.db.AppVersion.UpdateOneID(*record.VersionID)
		if approve {
			versionStatus = appversion.StatusAPPROVED
			update.SetPublishedAt(now)
		}
		updatedVersion, err := update.SetStatus(versionStatus).Save(r.Context())
		if err == nil && approve {
			_, _ = s.db.App.UpdateOneID(updatedVersion.AppID).SetStatus(app.StatusAPPROVED).Save(r.Context())
			s.enforceVersionRetention(r, updatedVersion.AppID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": updated})
}

type taxonomyRequest struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ParentID  *int   `json:"parentId"`
	SortOrder *int   `json:"sortOrder"`
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.handlePublicListCategories(w, r)
}

func (s *Server) handlePublicListCategories(w http.ResponseWriter, r *http.Request) {
	records, err := s.db.Category.Query().Order(entgo.Asc(category.FieldSortOrder), entgo.Asc(category.FieldName)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CATEGORY_LIST_FAILED", "Could not list categories", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": records})
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Category name is required", nil)
		return
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	create := s.db.Category.Create().SetName(name).SetSlug(slug)
	if input.ParentID != nil {
		create.SetParentID(*input.ParentID)
	}
	if input.SortOrder != nil {
		create.SetSortOrder(*input.SortOrder)
	}
	record, err := create.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "CATEGORY_CREATE_FAILED", "Could not create category", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"category": record})
}

func (s *Server) handleUpdateCategory(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	update := s.db.Category.UpdateOneID(id)
	if name := strings.TrimSpace(input.Name); name != "" {
		update.SetName(name)
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		update.SetSlug(slug)
	}
	if input.ParentID != nil {
		update.SetParentID(*input.ParentID)
	}
	if input.SortOrder != nil {
		update.SetSortOrder(*input.SortOrder)
	}
	record, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "CATEGORY_UPDATE_FAILED", "Could not update category", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": record})
}

func (s *Server) handleDeleteCategory(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	_, _ = s.db.App.Update().Where(app.CategoryIDEQ(id)).ClearCategoryID().Save(r.Context())
	if err := s.db.Category.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "CATEGORY_DELETE_FAILED", "Could not delete category", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.handlePublicListTags(w, r)
}

func (s *Server) handlePublicListTags(w http.ResponseWriter, r *http.Request) {
	records, err := s.db.Tag.Query().Order(entgo.Asc(tag.FieldName)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TAG_LIST_FAILED", "Could not list tags", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": records})
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Tag name is required", nil)
		return
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	record, err := s.db.Tag.Create().SetName(name).SetSlug(slug).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "TAG_CREATE_FAILED", "Could not create tag", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tag": record})
}

func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	record, ok := s.tagByPath(w, r)
	if !ok {
		return
	}
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	update := s.db.Tag.UpdateOneID(record.ID)
	if name := strings.TrimSpace(input.Name); name != "" {
		update.SetName(name)
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		update.SetSlug(slug)
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "TAG_UPDATE_FAILED", "Could not update tag", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": updated})
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	record, ok := s.tagByPath(w, r)
	if !ok {
		return
	}
	_, _ = s.db.AppTag.Delete().Where(apptag.TagIDEQ(record.ID)).Exec(r.Context())
	if err := s.db.Tag.DeleteOneID(record.ID).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "TAG_DELETE_FAILED", "Could not delete tag", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) tagByPath(w http.ResponseWriter, r *http.Request) (*entgo.Tag, bool) {
	value := strings.TrimSpace(r.PathValue("id"))
	if id, err := strconv.Atoi(value); err == nil {
		record, err := s.db.Tag.Get(r.Context(), id)
		if err == nil {
			return record, true
		}
		writeError(w, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found", nil)
		return nil, false
	}
	record, err := s.db.Tag.Query().Where(tag.SlugEQ(value)).Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "TAG_NOT_FOUND", "Tag not found", nil)
		return nil, false
	}
	return record, true
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	records, err := s.db.SiteSetting.Query().Order(entgo.Asc(sitesetting.FieldKey)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTING_LIST_FAILED", "Could not list settings", nil)
		return
	}
	values := map[string]string{
		"max_lpk_size":             strconv.FormatInt(s.cfg.MaxLPKSize, 10),
		"max_versions":             strconv.Itoa(s.cfg.MaxVersions),
		"require_email_verify":     strconv.FormatBool(s.cfg.RequireEmailVerify),
		"source_password":          s.cfg.SourcePassword,
		"source_password_rotation": strconv.Itoa(s.cfg.SourcePasswordRotation),
		"github_mirror":            s.cfg.GitHubMirror,
	}
	for _, record := range records {
		if isPublicSetting(record.Key) {
			values[record.Key] = record.Value
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": values})
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input map[string]string
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	for key, value := range input {
		if err := validateSetting(key, value); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
	}
	for key, value := range input {
		exists, err := s.db.SiteSetting.Query().Where(sitesetting.KeyEQ(key)).Only(r.Context())
		if err == nil {
			_, _ = s.db.SiteSetting.UpdateOneID(exists.ID).SetValue(value).Save(r.Context())
			continue
		}
		_, _ = s.db.SiteSetting.Create().SetKey(key).SetValue(value).Save(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func validateSetting(key, value string) error {
	if !isPublicSetting(key) {
		return fmt.Errorf("unknown setting %q", key)
	}
	switch key {
	case "max_lpk_size":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
	case "max_versions", "source_password_rotation":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return fmt.Errorf("%s must be a non-negative integer", key)
		}
	case "require_email_verify":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
	case "github_mirror":
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%s must be an http or https URL", key)
		}
	case "source_password":
		return nil
	}
	return nil
}

func isPublicSetting(key string) bool {
	switch key {
	case "max_lpk_size", "max_versions", "require_email_verify", "source_password", "source_password_rotation", "github_mirror":
		return true
	default:
		return false
	}
}

func (s *Server) enforceVersionRetention(r *http.Request, appID int) {
	maxVersions := s.effectiveMaxVersions(r.Context())
	if maxVersions == 0 {
		return
	}
	records, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(appID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		All(r.Context())
	if err != nil || len(records) <= maxVersions {
		return
	}
	for _, old := range records[maxVersions:] {
		if old.StoragePath != "" {
			_ = s.storage.Delete(r.Context(), old.StoragePath)
		}
		_ = s.db.AppVersion.DeleteOneID(old.ID).Exec(r.Context())
	}
}
