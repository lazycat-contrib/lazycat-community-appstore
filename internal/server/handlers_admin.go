package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/sitesetting"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/internal/catalogmeta"
	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/storage"
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
		case reviewrequest.KindAPP_INFO_UPDATE, reviewrequest.KindAPP_RESUBMISSION:
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
			if record.Kind == reviewrequest.KindAPP_RESUBMISSION {
				appStatus := app.StatusREJECTED
				if approve {
					appStatus = app.StatusAPPROVED
				}
				_, _ = s.db.App.UpdateOneID(*record.AppID).SetStatus(appStatus).Save(r.Context())
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
			s.clearAppOutdatedMarks(r, updatedVersion.AppID)
			s.enforceVersionRetention(r, updatedVersion.AppID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": updated})
}

type taxonomyRequest struct {
	Name      string                    `json:"name"`
	NameI18n  catalogmeta.LocalizedText `json:"nameI18n"`
	Slug      string                    `json:"slug"`
	ParentID  *int                      `json:"parentId"`
	SortOrder *int                      `json:"sortOrder"`
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
	out := make([]categoryDTO, 0, len(records))
	for _, record := range records {
		out = append(out, toCategoryDTO(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.NameI18n = catalogmeta.CleanLocalizedText(input.NameI18n)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = input.NameI18n.Fallback("")
		if name == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Category name is required", nil)
			return
		}
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	create := s.db.Category.Create().SetName(name).SetNameI18n(catalogmeta.EncodeLocalizedText(input.NameI18n)).SetSlug(slug)
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
	writeJSON(w, http.StatusCreated, map[string]any{"category": toCategoryDTO(record)})
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
	if input.NameI18n != nil {
		update.SetNameI18n(catalogmeta.EncodeLocalizedText(input.NameI18n))
		if strings.TrimSpace(input.Name) == "" {
			if name := input.NameI18n.Fallback(""); name != "" {
				update.SetName(name)
			}
		}
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
	writeJSON(w, http.StatusOK, map[string]any{"category": toCategoryDTO(record)})
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
	out := make([]tagDTO, 0, len(records))
	for _, record := range records {
		out = append(out, toTagDTO(record))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tags": out})
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input taxonomyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.NameI18n = catalogmeta.CleanLocalizedText(input.NameI18n)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = input.NameI18n.Fallback("")
		if name == "" {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Tag name is required", nil)
			return
		}
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	record, err := s.db.Tag.Create().SetName(name).SetNameI18n(catalogmeta.EncodeLocalizedText(input.NameI18n)).SetSlug(slug).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "TAG_CREATE_FAILED", "Could not create tag", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tag": toTagDTO(record)})
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
	if input.NameI18n != nil {
		update.SetNameI18n(catalogmeta.EncodeLocalizedText(input.NameI18n))
		if strings.TrimSpace(input.Name) == "" {
			if name := input.NameI18n.Fallback(""); name != "" {
				update.SetName(name)
			}
		}
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		update.SetSlug(slug)
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "TAG_UPDATE_FAILED", "Could not update tag", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": toTagDTO(updated)})
}

func toCategoryDTO(record *entgo.Category) categoryDTO {
	return categoryDTO{
		ID:        record.ID,
		Name:      record.Name,
		NameI18n:  catalogmeta.DecodeLocalizedText(record.NameI18n),
		Slug:      record.Slug,
		ParentID:  record.ParentID,
		SortOrder: record.SortOrder,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func toTagDTO(record *entgo.Tag) tagDTO {
	return tagDTO{
		ID:        record.ID,
		Name:      record.Name,
		NameI18n:  catalogmeta.DecodeLocalizedText(record.NameI18n),
		Slug:      record.Slug,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
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
		settingMaxLPKSize:               strconv.FormatInt(s.cfg.MaxLPKSize, 10),
		settingMaxVersions:              strconv.Itoa(s.cfg.MaxVersions),
		settingRequireEmailVerify:       strconv.FormatBool(s.cfg.RequireEmailVerify),
		settingSourcePassword:           s.cfg.SourcePassword,
		settingSourcePasswordRotation:   strconv.Itoa(s.cfg.SourcePasswordRotation),
		settingCommentsEnabled:          "true",
		settingAllowManualOutdatedClear: "false",
		settingGitHubDownloadMirrors:    s.cfg.GitHubDownloadMirrors,
		settingGitHubRawMirrors:         s.cfg.GitHubRawMirrors,
		settingSiteTitle:                s.siteProfile(r.Context()).Title,
		settingSiteSubtitle:             s.siteProfile(r.Context()).Subtitle,
		settingSiteIconURL:              "",
		settingSitePublicURL:            s.sitePublicURL(r.Context()),
		settingAnnouncementEnabled:      "false",
		settingAnnouncementLevel:        "info",
		settingAnnouncementTitle:        "",
		settingAnnouncementBody:         "",
		settingAnnouncementLinkLabel:    "",
		settingAnnouncementLinkURL:      "",
		settingAnnouncementUpdatedAt:    "",
		settingRegistrationMode:         registrationModeOpen,
		settingSMTPHost:                 s.cfg.SMTPHost,
		settingSMTPPort:                 strconv.Itoa(s.cfg.SMTPPort),
		settingSMTPUser:                 s.cfg.SMTPUser,
		settingSMTPPass:                 s.cfg.SMTPPass,
		settingSMTPFrom:                 s.cfg.SMTPFrom,
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
		value = strings.TrimSpace(value)
		if err := validateSetting(key, value); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		input[key] = normalizeSettingValue(key, value)
	}
	if s.settingsChangeAnnouncement(r.Context(), input) {
		input[settingAnnouncementUpdatedAt] = time.Now().UTC().Format(time.RFC3339)
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

func (s *Server) handleUploadSiteIcon(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	if err := r.ParseMultipartForm(maxSiteIconImageSize + 1<<20); err != nil {
		badRequest(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, err)
		return
	}
	defer file.Close()
	if err := validateUploadedImage(file, header, maxSiteIconImageSize); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	storageKey, err := s.uploadStorageKey(r.Context(), r.FormValue("storageKey"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SITE_ICON_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	backend, err := s.storageBackendForKey(r.Context(), storageKey)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SITE_ICON_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), backend, file, header.Filename, maxSiteIconImageSize)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SITE_ICON_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	iconURL := s.absoluteURL(r.Context(), obj.DownloadURL)
	if err := s.setSetting(r.Context(), settingSiteIconURL, iconURL); err != nil {
		_ = backend.Delete(r.Context(), obj.Path)
		writeError(w, http.StatusInternalServerError, "SITE_ICON_SAVE_FAILED", "Could not save site icon", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        iconURL,
		"storageKey": storageKey,
		"path":       obj.Path,
		"site":       s.siteProfile(r.Context()),
	})
}

type testEmailRequest struct {
	To       string            `json:"to"`
	Settings map[string]string `json:"settings"`
}

func (s *Server) handleSendTestEmail(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input testEmailRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	to := strings.TrimSpace(input.To)
	if _, err := mail.ParseAddress(to); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "A valid recipient email is required", nil)
		return
	}
	cfg, err := s.smtpConfigFromSettings(r.Context(), input.Settings)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.From) == "" {
		writeError(w, http.StatusUnprocessableEntity, "SMTP_NOT_CONFIGURED", "SMTP host and from address are required", nil)
		return
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "SMTP from address must be valid", nil)
		return
	}
	subject := "LazyCat private store test email"
	body := "This is a test email from LazyCat private store.\n\nIf you received this message, SMTP delivery is configured correctly.\n"
	if mailer, ok := s.mailer.(smtpMailer); ok {
		if err := mailer.SendWithConfig(r.Context(), cfg, to, subject, body); err != nil {
			writeError(w, http.StatusBadGateway, "TEST_EMAIL_FAILED", err.Error(), nil)
			return
		}
	} else if err := s.mailer.Send(r.Context(), to, subject, body); err != nil {
		writeError(w, http.StatusBadGateway, "TEST_EMAIL_FAILED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) smtpConfigFromSettings(ctx context.Context, settings map[string]string) (smtpConfig, error) {
	cfg := s.smtpSettings(ctx)
	if len(settings) == 0 {
		return cfg, nil
	}
	for _, key := range []string{settingSMTPHost, settingSMTPPort, settingSMTPUser, settingSMTPPass, settingSMTPFrom} {
		value, ok := settings[key]
		if !ok {
			continue
		}
		value = normalizeSettingValue(key, strings.TrimSpace(value))
		if err := validateSetting(key, value); err != nil {
			return smtpConfig{}, err
		}
		switch key {
		case settingSMTPHost:
			cfg.Host = value
		case settingSMTPPort:
			port, err := strconv.Atoi(value)
			if err != nil {
				return smtpConfig{}, fmt.Errorf("%s must be a valid port", key)
			}
			cfg.Port = port
		case settingSMTPUser:
			cfg.User = value
		case settingSMTPPass:
			cfg.Pass = value
		case settingSMTPFrom:
			cfg.From = value
		}
	}
	return cfg, nil
}

func validateSetting(key, value string) error {
	if !isPublicSetting(key) {
		return fmt.Errorf("unknown setting %q", key)
	}
	switch key {
	case settingMaxLPKSize:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
	case settingMaxVersions, settingSourcePasswordRotation:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return fmt.Errorf("%s must be a non-negative integer", key)
		}
	case settingRequireEmailVerify, settingAnnouncementEnabled, settingCommentsEnabled, settingAllowManualOutdatedClear:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
	case settingRegistrationMode:
		if !validRegistrationMode(value) {
			return fmt.Errorf("%s must be open, invite, or closed", key)
		}
	case settingSMTPPort:
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return fmt.Errorf("%s must be a valid port", key)
		}
	case settingGitHubDownloadMirrors, settingGitHubRawMirrors:
		if _, err := mirror.Parse(value, settingMirrorKind(key)); err != nil {
			return fmt.Errorf("%s %w", key, err)
		}
	case settingSitePublicURL, settingSiteIconURL, settingAnnouncementLinkURL:
		if !isHTTPURLOrEmpty(value) {
			return fmt.Errorf("%s must be an http or https URL", key)
		}
	case settingAnnouncementLevel:
		if !validAnnouncementLevel(value) {
			return fmt.Errorf("%s must be info, warning, or success", key)
		}
	case settingSiteTitle:
		if len([]rune(value)) > 80 {
			return fmt.Errorf("%s must be 80 characters or fewer", key)
		}
	case settingSiteSubtitle:
		if len([]rune(value)) > 180 {
			return fmt.Errorf("%s must be 180 characters or fewer", key)
		}
	case settingAnnouncementTitle:
		if len([]rune(value)) > 120 {
			return fmt.Errorf("%s must be 120 characters or fewer", key)
		}
	case settingAnnouncementBody:
		if len([]rune(value)) > 600 {
			return fmt.Errorf("%s must be 600 characters or fewer", key)
		}
	case settingAnnouncementLinkLabel:
		if len([]rune(value)) > 60 {
			return fmt.Errorf("%s must be 60 characters or fewer", key)
		}
	case settingAnnouncementUpdatedAt:
		if value == "" {
			return nil
		}
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("%s must be an RFC3339 timestamp", key)
		}
	case settingSourcePassword, settingSMTPHost, settingSMTPUser, settingSMTPPass, settingSMTPFrom:
		return nil
	}
	return nil
}

func isPublicSetting(key string) bool {
	switch key {
	case settingMaxLPKSize,
		settingMaxVersions,
		settingRequireEmailVerify,
		settingSourcePassword,
		settingSourcePasswordRotation,
		settingCommentsEnabled,
		settingAllowManualOutdatedClear,
		settingGitHubDownloadMirrors,
		settingGitHubRawMirrors,
		settingSiteTitle,
		settingSiteSubtitle,
		settingSiteIconURL,
		settingSitePublicURL,
		settingAnnouncementEnabled,
		settingAnnouncementLevel,
		settingAnnouncementTitle,
		settingAnnouncementBody,
		settingAnnouncementLinkLabel,
		settingAnnouncementLinkURL,
		settingAnnouncementUpdatedAt,
		settingRegistrationMode,
		settingSMTPHost,
		settingSMTPPort,
		settingSMTPUser,
		settingSMTPPass,
		settingSMTPFrom:
		return true
	default:
		return false
	}
}

func normalizeSettingValue(key, value string) string {
	switch key {
	case settingGitHubDownloadMirrors, settingGitHubRawMirrors:
		normalized, err := mirror.Normalize(value, settingMirrorKind(key))
		if err != nil {
			return value
		}
		return normalized
	case settingSitePublicURL, settingSiteIconURL, settingAnnouncementLinkURL:
		return cleanURLSetting(value)
	case settingRegistrationMode:
		return strings.ToLower(value)
	default:
		return value
	}
}

func settingMirrorKind(key string) string {
	if key == settingGitHubRawMirrors {
		return mirror.KindRaw
	}
	return mirror.KindDownload
}

func (s *Server) settingsChangeAnnouncement(ctx context.Context, input map[string]string) bool {
	for key, value := range input {
		switch key {
		case settingAnnouncementEnabled,
			settingAnnouncementLevel,
			settingAnnouncementTitle,
			settingAnnouncementBody,
			settingAnnouncementLinkLabel,
			settingAnnouncementLinkURL:
			if value != s.setting(ctx, key, announcementSettingDefault(key)) {
				return true
			}
		}
	}
	return false
}

func announcementSettingDefault(key string) string {
	switch key {
	case settingAnnouncementEnabled:
		return "false"
	case settingAnnouncementLevel:
		return "info"
	default:
		return ""
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
			s.deleteStoredObject(r.Context(), old.StorageKey, old.StoragePath)
		}
		_ = s.db.AppVersion.DeleteOneID(old.ID).Exec(r.Context())
	}
}

func (s *Server) clearAppOutdatedMarks(r *http.Request, appID int) {
	_, _ = s.db.OutdatedMark.Delete().Where(outdatedmark.AppIDEQ(appID)).Exec(r.Context())
}
