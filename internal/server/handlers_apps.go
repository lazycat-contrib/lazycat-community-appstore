package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/appvote"
	"lazycat.community/appstore/ent/collaborator"
	"lazycat.community/appstore/ent/collaboratorinvite"
	"lazycat.community/appstore/ent/collaboratorrequest"
	"lazycat.community/appstore/ent/collectionapp"
	commentpkg "lazycat.community/appstore/ent/comment"
	favoritepkg "lazycat.community/appstore/ent/favorite"
	outdatedpkg "lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/tag"
	userpkg "lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/catalogmeta"
	"lazycat.community/appstore/internal/lpkmeta"
	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/pagination"
	"lazycat.community/appstore/internal/storage"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	u := s.optionalUser(r)
	managedList := r.URL.Query().Get("managed") == "1" || r.URL.Query().Get("managed") == "true"
	if !managedList && u == nil {
		value, err := s.sharedFirstLoad(r.Context(), firstLoadKey(r, "public-apps"), func(ctx context.Context) (any, error) {
			return s.publicListAppsResponse(ctx, r)
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
			return
		}
		writeJSON(w, http.StatusOK, value)
		return
	}
	q := s.db.App.Query()
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(r.Context(), pagination.DefaultPageSize, 100), 100)
	if managedList {
		if u == nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}
		if !isAdmin(u) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Only administrators can list managed apps", nil)
			return
		}
	}
	var collaboratorAppIDs []int
	if !isAdmin(u) {
		if u == nil {
			q.Where(app.StatusEQ(app.StatusAPPROVED))
		} else {
			collaboratorAppIDs = s.collaboratorAppIDs(r.Context(), u.ID)
			if len(collaboratorAppIDs) > 0 {
				q.Where(app.Or(app.StatusEQ(app.StatusAPPROVED), app.OwnerIDEQ(u.ID), app.IDIn(collaboratorAppIDs...)))
			} else {
				q.Where(app.Or(app.StatusEQ(app.StatusAPPROVED), app.OwnerIDEQ(u.ID)))
			}
		}
	}
	if err := s.applyAppListVisibility(r.Context(), q, u, collaboratorAppIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
		return
	}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		q.Where(app.Or(app.PackageIDContainsFold(search), app.NameContainsFold(search), app.SummaryContainsFold(search), app.DescriptionContainsFold(search)))
	}
	if ids := parsePositiveIDList(r.URL.Query().Get("ids")); len(ids) > 0 {
		q.Where(app.IDIn(ids...))
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && isAdmin(u) {
		q.Where(app.StatusEQ(app.Status(status)))
	}
	if categoryID, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("categoryId"))); err == nil && categoryID > 0 {
		q.Where(app.CategoryIDEQ(categoryID))
	}
	if owner := strings.TrimSpace(r.URL.Query().Get("submitter")); owner != "" && owner != "all" {
		s.applyAppListSubmitterFilter(r.Context(), q, owner)
	}
	total, err := q.Clone().Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
		return
	}
	s.applyAppListSort(r.Context(), q, strings.TrimSpace(r.URL.Query().Get("sort")))
	apps, err := q.Offset(page.Offset()).Limit(page.PageSize).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
		return
	}
	preload, err := s.preloadAppSummaries(r.Context(), apps, u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
		return
	}
	out := make([]appSummary, 0, len(apps))
	for _, record := range apps {
		dto := s.appSummaryDTOFromPreload(r.Context(), record, u, preload)
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, pagination.NewAppsPage(out, page, total))
}

func (s *Server) publicListAppsResponse(ctx context.Context, r *http.Request) (any, error) {
	q := s.db.App.Query().Where(app.StatusEQ(app.StatusAPPROVED))
	q.Where(app.Not(appHasAnyVisibility()))
	page := pagination.FromRequest(r, s.effectiveDefaultPageSize(ctx, pagination.DefaultPageSize, 100), 100)
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		q.Where(app.Or(app.PackageIDContainsFold(search), app.NameContainsFold(search), app.SummaryContainsFold(search), app.DescriptionContainsFold(search)))
	}
	if ids := parsePositiveIDList(r.URL.Query().Get("ids")); len(ids) > 0 {
		q.Where(app.IDIn(ids...))
	}
	if categoryID, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("categoryId"))); err == nil && categoryID > 0 {
		q.Where(app.CategoryIDEQ(categoryID))
	}
	if owner := strings.TrimSpace(r.URL.Query().Get("submitter")); owner != "" && owner != "all" {
		s.applyAppListSubmitterFilter(ctx, q, owner)
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	s.applyAppListSort(ctx, q, strings.TrimSpace(r.URL.Query().Get("sort")))
	records, err := q.Offset(page.Offset()).Limit(page.PageSize).All(ctx)
	if err != nil {
		return nil, err
	}
	preload, err := s.preloadAppSummaries(ctx, records, nil)
	if err != nil {
		return nil, err
	}
	out := make([]appSummary, 0, len(records))
	for _, record := range records {
		out = append(out, s.appSummaryDTOFromPreload(ctx, record, nil, preload))
	}
	return pagination.NewAppsPage(out, page, total), nil
}

func (s *Server) applyAppListSubmitterFilter(ctx context.Context, q *entgo.AppQuery, value string) {
	if ownerID, err := strconv.Atoi(value); err == nil {
		q.Where(app.OwnerIDEQ(ownerID))
		return
	}
	records, err := s.db.User.Query().
		Where(userpkg.Or(userpkg.UsernameEQ(value), userpkg.NicknameEQ(value))).
		All(ctx)
	if err != nil || len(records) == 0 {
		q.Where(app.OwnerIDEQ(-1))
		return
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	q.Where(app.OwnerIDIn(ids...))
}

func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	u := s.optionalUser(r)
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if record.Status != app.StatusAPPROVED && (u == nil || (!isAdmin(u) && record.OwnerID != u.ID)) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, u) {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	preload, err := s.preloadAppSummaries(r.Context(), []*entgo.App{record}, u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LOAD_FAILED", "Could not load app", nil)
		return
	}
	detail := appDetail{appSummary: s.appSummaryDTOFromPreload(r.Context(), record, u, preload)}
	canManageReleases := u != nil && s.canUploadVersion(r, record, u)
	versionRecords, _ := s.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).Order(entgo.Desc(appversion.FieldCreatedAt)).All(r.Context())
	for _, v := range versionRecords {
		if v.Status != appversion.StatusAPPROVED && !canManageReleases {
			continue
		}
		detail.Versions = append(detail.Versions, toVersionDTO(v))
	}
	if canManageReleases {
		policy := s.versionRetentionPolicyForApp(r.Context(), record)
		detail.VersionRetention = &policy
	}
	detail.Screenshots, _ = s.loadScreenshots(r, record.ID)
	comments, _ := s.loadComments(r, record.ID)
	detail.Comments = comments
	detail.Favorites, _ = s.db.Favorite.Query().Where(favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDEQ(record.ID)).Count(r.Context())
	detail.OutdatedMarks, _ = s.db.OutdatedMark.Query().Where(outdatedpkg.AppIDEQ(record.ID)).Count(r.Context())
	if u != nil {
		detail.CanManageApp = isAdmin(u) || record.OwnerID == u.ID
		detail.CanUploadVersion = detail.CanManageApp || s.isCollaborator(r, record.ID, u.ID)
		detail.CanClearOutdatedMarks = detail.CanManageApp && s.manualOutdatedClearAllowed(r.Context())
		detail.OutdatedMarked, _ = s.db.OutdatedMark.Query().Where(outdatedpkg.AppIDEQ(record.ID), outdatedpkg.UserIDEQ(u.ID)).Exist(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": detail})
}

func (s *Server) handleGetWritableAppByName(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "name is required", nil)
		return
	}
	records, err := s.db.App.Query().Where(app.NameEQ(name)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LOOKUP_FAILED", "Could not resolve app name", nil)
		return
	}
	writable := make([]*entgo.App, 0, 1)
	for _, record := range records {
		if s.canUploadVersion(r, record, u) {
			writable = append(writable, record)
		}
	}
	switch len(writable) {
	case 0:
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
	case 1:
		writeJSON(w, http.StatusOK, map[string]any{"app": s.appSummaryDTO(r, writable[0], u)})
	default:
		writeError(w, http.StatusConflict, "APP_NAME_AMBIGUOUS", "Multiple writable apps have this name", nil)
	}
}

func (s *Server) handleGetPackageLatestVersion(w http.ResponseWriter, r *http.Request) {
	packageID := strings.TrimSpace(r.PathValue("packageId"))
	record, err := s.db.App.Query().Where(app.PackageIDEQ(packageID)).Only(r.Context())
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, s.optionalUser(r)) {
		allowed, err := s.requestHasGroupCodeForApp(r, record.ID)
		if err != nil || !allowed {
			writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
			return
		}
	}
	latest, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		First(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packageId": record.PackageID, "latestVersion": toVersionDTO(latest)})
}

type createAppJSON struct {
	Name                      string            `json:"name"`
	NameI18n                  map[string]string `json:"nameI18n"`
	PackageID                 string            `json:"packageId"`
	Slug                      string            `json:"slug"`
	Summary                   string            `json:"summary"`
	SummaryI18n               map[string]string `json:"summaryI18n"`
	Description               string            `json:"description"`
	DescriptionI18n           map[string]string `json:"descriptionI18n"`
	IconURL                   string            `json:"iconUrl"`
	CategoryID                *int              `json:"categoryId"`
	Tags                      []string          `json:"tags"`
	AllowUnreviewedUpdates    bool              `json:"allowUnreviewedUpdates"`
	CommentsEnabled           *bool             `json:"commentsEnabled"`
	EmailNotificationsEnabled *bool             `json:"emailNotificationsEnabled"`
	InstallPassword           string            `json:"installPassword"`
	Version                   string            `json:"version"`
	Changelog                 string            `json:"changelog"`
	DownloadURL               string            `json:"downloadUrl"`
	SourceType                string            `json:"sourceType"`
	SHA256                    string            `json:"sha256"`
	UseMirrorDownload         bool              `json:"useMirrorDownload"`
	iconAssetID               int
}

type updateAppJSON struct {
	Name                      *string  `json:"name,omitempty"`
	Summary                   *string  `json:"summary,omitempty"`
	Description               *string  `json:"description,omitempty"`
	CategoryID                *int     `json:"categoryId,omitempty"`
	Tags                      []string `json:"tags,omitempty"`
	TagsSet                   bool     `json:"tagsSet,omitempty"`
	AllowUnreviewedUpdates    *bool    `json:"allowUnreviewedUpdates,omitempty"`
	CommentsEnabled           *bool    `json:"commentsEnabled,omitempty"`
	EmailNotificationsEnabled *bool    `json:"emailNotificationsEnabled,omitempty"`
	InstallPassword           *string  `json:"installPassword,omitempty"`
	SubmitForReview           *bool    `json:"submitForReview,omitempty"`
}

func (u *updateAppJSON) UnmarshalJSON(data []byte) error {
	type alias updateAppJSON
	var raw alias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	*u = updateAppJSON(raw)
	if !u.TagsSet {
		_, u.TagsSet = keys["tags"]
	}
	return nil
}

func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		s.createAppMultipart(w, r, u)
		return
	}
	var input createAppJSON
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.DownloadURL = normalizeGitHubRawURL(input.DownloadURL)
	var inspected lpkInspection
	if input.DownloadURL != "" && appInputNeedsLPKInspection(input) {
		var err error
		inspected, err = s.inspectLPKURL(r.Context(), input.DownloadURL, s.effectiveMaxLPKSize(r.Context()), input.UseMirrorDownload)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "LPK_METADATA_FAILED", err.Error(), nil)
			return
		}
		if err := s.applyAppMetadata(r.Context(), &input, inspected.Metadata); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "LPK_METADATA_FAILED", err.Error(), nil)
			return
		}
		if input.SHA256 == "" {
			input.SHA256 = inspected.SHA256
		}
	}
	record, err := s.createAppRecord(r, u, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
		return
	}
	if input.DownloadURL != "" {
		_, err = s.createExternalVersion(r, u, record, input.Version, input.Changelog, input.DownloadURL, input.SourceType, input.SHA256, inspected.Size)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"app": s.appSummaryDTO(r, record, u)})
}

func (s *Server) createAppMultipart(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	maxLPKSize := s.effectiveMaxLPKSize(r.Context())
	if err := r.ParseMultipartForm(maxLPKSize + 32<<20); err != nil {
		badRequest(w, err)
		return
	}
	commentsEnabled := true
	emailNotificationsEnabled := true
	input := createAppJSON{
		Name:                      r.FormValue("name"),
		PackageID:                 r.FormValue("packageId"),
		Slug:                      r.FormValue("slug"),
		Summary:                   r.FormValue("summary"),
		Description:               r.FormValue("description"),
		Version:                   r.FormValue("version"),
		Changelog:                 r.FormValue("changelog"),
		InstallPassword:           r.FormValue("installPassword"),
		UseMirrorDownload:         formBool(r, "useMirrorDownload"),
		AllowUnreviewedUpdates:    formBool(r, "allowUnreviewedUpdates"),
		CommentsEnabled:           &commentsEnabled,
		EmailNotificationsEnabled: &emailNotificationsEnabled,
	}
	if r.Form.Has("emailNotificationsEnabled") {
		emailNotificationsEnabled = formBool(r, "emailNotificationsEnabled")
	}
	if categoryID, err := strconv.Atoi(r.FormValue("categoryId")); err == nil && categoryID > 0 {
		input.CategoryID = &categoryID
	}
	if tags := strings.TrimSpace(r.FormValue("tags")); tags != "" {
		input.Tags = splitCSV(tags)
	}
	file, header, fileErr := r.FormFile("file")
	if fileErr == nil {
		defer func() { _ = file.Close() }()
		meta, err := parseUploadedLPKMetadata(file, header, maxLPKSize)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
			return
		}
		if err := s.applyAppMetadata(r.Context(), &input, meta); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
			return
		}
	}
	record, err := s.createAppRecord(r, u, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
		return
	}
	if fileErr == nil {
		if _, err := s.createUploadedVersion(r, u, record, input.Version, input.Changelog, r.FormValue("storageKey"), file, header); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"app": s.appSummaryDTO(r, record, u)})
}

func (s *Server) createAppRecord(r *http.Request, u *entgo.User, input createAppJSON) (*entgo.App, error) {
	if err := s.materializeAppIconURL(r.Context(), &input); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("app name is required")
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	packageID := strings.TrimSpace(input.PackageID)
	if packageID == "" {
		return nil, errors.New("packageId is required")
	}
	status := app.StatusPENDING
	if isAdmin(u) {
		status = app.StatusAPPROVED
	}
	commentsEnabled := true
	if input.CommentsEnabled != nil {
		commentsEnabled = *input.CommentsEnabled
	}
	emailNotificationsEnabled := true
	if input.EmailNotificationsEnabled != nil {
		emailNotificationsEnabled = *input.EmailNotificationsEnabled
	}
	create := s.db.App.Create().
		SetOwnerID(u.ID).
		SetPackageID(packageID).
		SetName(name).
		SetNameI18nJSON(catalogmeta.EncodeLocalizedText(input.NameI18n)).
		SetSlug(slug).
		SetSummary(input.Summary).
		SetSummaryI18nJSON(catalogmeta.EncodeLocalizedText(input.SummaryI18n)).
		SetDescription(input.Description).
		SetDescriptionI18nJSON(catalogmeta.EncodeLocalizedText(input.DescriptionI18n)).
		SetStatus(status).
		SetAllowUnreviewedUpdates(input.AllowUnreviewedUpdates).
		SetCommentsEnabled(commentsEnabled).
		SetEmailNotificationsEnabled(emailNotificationsEnabled)
	if iconURL := strings.TrimSpace(input.IconURL); iconURL != "" {
		create.SetIconURL(iconURL)
	}
	if hash, err := hashInstallPassword(input.InstallPassword); err != nil {
		return nil, err
	} else if hash != "" {
		create.SetInstallPasswordHash(hash)
	}
	if input.CategoryID != nil {
		create.SetCategoryID(*input.CategoryID)
	}
	record, err := create.Save(r.Context())
	if err != nil {
		if input.iconAssetID > 0 {
			_ = s.cleanupAssetIDs(r.Context(), input.iconAssetID)
		}
		return nil, err
	}
	if input.iconAssetID > 0 {
		if err := s.replaceAssetLinks(r.Context(), assetOwnerApp, record.ID, assetRoleIcon, input.iconAssetID); err != nil {
			return nil, err
		}
	} else if assetID, ok := s.assetIDFromURL(input.IconURL); ok {
		if err := s.replaceAssetLinks(r.Context(), assetOwnerApp, record.ID, assetRoleIcon, assetID); err != nil {
			return nil, err
		}
	}
	_ = s.syncTags(r, record.ID, input.Tags)
	if status == app.StatusPENDING {
		_, _ = s.db.ReviewRequest.Create().
			SetKind(reviewrequest.KindAPP_SUBMISSION).
			SetStatus(reviewrequest.StatusPENDING).
			SetAppID(record.ID).
			SetRequesterID(u.ID).
			Save(r.Context())
	}
	return record, nil
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the owner or an admin can update app info", nil)
		return
	}
	var input updateAppJSON
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}

	resubmittingRejectedApp := record.Status == app.StatusREJECTED && (input.SubmitForReview == nil || *input.SubmitForReview)
	if !isAdmin(u) && (resubmittingRejectedApp || !record.AllowUnreviewedUpdates) {
		if input.InstallPassword != nil {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Install password changes require direct app update permission", nil)
			return
		}
		raw, err := json.Marshal(input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_UPDATE_REVIEW_FAILED", "Could not prepare app update review", nil)
			return
		}
		kind := reviewrequest.KindAPP_INFO_UPDATE
		if resubmittingRejectedApp {
			kind = reviewrequest.KindAPP_RESUBMISSION
		}
		review, err := s.db.ReviewRequest.Create().
			SetKind(kind).
			SetStatus(reviewrequest.StatusPENDING).
			SetAppID(record.ID).
			SetRequesterID(u.ID).
			SetNote(string(raw)).
			Save(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_UPDATE_REVIEW_FAILED", "Could not create app update review", nil)
			return
		}
		if resubmittingRejectedApp {
			_, _ = s.db.App.UpdateOneID(record.ID).SetStatus(app.StatusPENDING).Save(r.Context())
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"review": review})
		return
	}

	updated, err := s.applyAppInfoUpdate(r, id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_UPDATE_FAILED", "Could not update app", nil)
		return
	}
	if isAdmin(u) && resubmittingRejectedApp {
		updated, err = s.db.App.UpdateOneID(id).SetStatus(app.StatusAPPROVED).Save(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_UPDATE_FAILED", "Could not update app status", nil)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": s.appSummaryDTO(r, updated, u)})
}

func (s *Server) applyAppInfoUpdate(r *http.Request, id int, input updateAppJSON) (*entgo.App, error) {
	update := s.db.App.UpdateOneID(id)
	if input.Name != nil {
		if name := strings.TrimSpace(*input.Name); name != "" {
			update.SetName(name)
		}
	}
	if input.Summary != nil {
		update.SetSummary(*input.Summary)
	}
	if input.Description != nil {
		update.SetDescription(*input.Description)
	}
	if input.CommentsEnabled != nil {
		update.SetCommentsEnabled(*input.CommentsEnabled)
	}
	if input.EmailNotificationsEnabled != nil {
		update.SetEmailNotificationsEnabled(*input.EmailNotificationsEnabled)
	}
	if input.InstallPassword != nil {
		hash, err := hashInstallPassword(*input.InstallPassword)
		if err != nil {
			return nil, err
		}
		update.SetInstallPasswordHash(hash)
	}
	if input.AllowUnreviewedUpdates != nil {
		update.SetAllowUnreviewedUpdates(*input.AllowUnreviewedUpdates)
	}
	if input.CategoryID != nil {
		update.SetCategoryID(*input.CategoryID)
	}
	updated, err := update.Save(r.Context())
	if err != nil {
		return nil, err
	}
	if input.TagsSet {
		if err := s.syncTags(r, updated.ID, input.Tags); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (s *Server) handleDeleteApp(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the owner or an admin can delete this app", nil)
		return
	}
	versions, _ := s.db.AppVersion.Query().Where(appversion.AppIDEQ(id)).All(r.Context())
	for _, version := range versions {
		if version.StoragePath != "" {
			s.deleteStoredObject(r.Context(), version.StorageKey, version.StoragePath)
		}
	}
	screenshots, _ := s.db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(id)).All(r.Context())
	for _, shot := range screenshots {
		if shot.StoragePath != "" {
			s.deleteStoredObject(r.Context(), shot.StorageKey, shot.StoragePath)
		}
	}
	_, _ = s.db.AppVersion.Delete().Where(appversion.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppScreenshot.Delete().Where(appscreenshot.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppVisibility.Delete().Where(appvisibility.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppTag.Delete().Where(apptag.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Collaborator.Delete().Where(collaborator.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.CollaboratorInvite.Delete().Where(collaboratorinvite.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.CollaboratorRequest.Delete().Where(collaboratorrequest.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.OutdatedMark.Delete().Where(outdatedpkg.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.ReviewRequest.Delete().Where(reviewrequest.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.CollectionApp.Delete().Where(collectionapp.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Comment.Delete().Where(commentpkg.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Favorite.Delete().Where(favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppDownload.Delete().Where(appdownload.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppVote.Delete().Where(appvote.AppIDEQ(id)).Exec(r.Context())
	if err := s.db.App.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "APP_DELETE_FAILED", "Could not delete app", nil)
		return
	}
	_ = s.deleteAssetLinksForOwner(r.Context(), assetOwnerApp, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUnlistApp(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the owner or an admin can unlist this app", nil)
		return
	}
	updated, err := s.db.App.UpdateOneID(id).SetStatus(app.StatusUNLISTED).Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_UNLIST_FAILED", "Could not unlist app", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": s.appSummaryDTO(r, updated, u)})
}

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.canUploadVersion(r, record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You cannot upload versions for this app", nil)
		return
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		maxLPKSize := s.effectiveMaxLPKSize(r.Context())
		if err := r.ParseMultipartForm(maxLPKSize + 32<<20); err != nil {
			badRequest(w, err)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			badRequest(w, err)
			return
		}
		defer func() { _ = file.Close() }()
		created, err := s.createUploadedVersion(r, u, record, r.FormValue("version"), r.FormValue("changelog"), r.FormValue("storageKey"), file, header)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"version": toVersionDTO(created)})
		return
	}

	var input createAppJSON
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	input.DownloadURL = normalizeGitHubRawURL(input.DownloadURL)
	var inspected lpkInspection
	if input.DownloadURL != "" && versionInputNeedsLPKInspection(input) {
		inspected, err = s.inspectLPKURL(r.Context(), input.DownloadURL, s.effectiveMaxLPKSize(r.Context()), input.UseMirrorDownload)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "LPK_METADATA_FAILED", err.Error(), nil)
			return
		}
		if inspected.Metadata.PackageID != "" && inspected.Metadata.PackageID != record.PackageID {
			writeError(w, http.StatusUnprocessableEntity, "LPK_METADATA_FAILED", fmt.Sprintf("LPK package %q does not match app packageId %q", inspected.Metadata.PackageID, record.PackageID), nil)
			return
		}
		if input.Version == "" {
			input.Version = inspected.Metadata.Version
		}
		if input.SHA256 == "" {
			input.SHA256 = inspected.SHA256
		}
	}
	created, err := s.createExternalVersion(r, u, record, input.Version, input.Changelog, input.DownloadURL, input.SourceType, input.SHA256, inspected.Size)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
		return
	}
	if created.Status == appversion.StatusAPPROVED && inspected.Metadata.PackageID != "" {
		_ = s.updateAppFromApprovedLPKMetadata(r, record.ID, inspected.Metadata)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": toVersionDTO(created)})
}

func (s *Server) createUploadedVersion(r *http.Request, u *entgo.User, record *entgo.App, versionName, changelog, storageKeyInput string, file multipart.File, header *multipart.FileHeader) (*entgo.AppVersion, error) {
	meta, err := parseUploadedLPKMetadata(file, header, s.effectiveMaxLPKSize(r.Context()))
	if err != nil {
		return nil, err
	}
	if meta.PackageID != "" && record.PackageID != "" && meta.PackageID != record.PackageID {
		return nil, fmt.Errorf("LPK package %q does not match app packageId %q", meta.PackageID, record.PackageID)
	}
	versionName = strings.TrimSpace(versionName)
	if versionName == "" {
		versionName = meta.Version
	}
	if versionName == "" {
		return nil, errors.New("version is required")
	}
	storageKey, err := s.uploadStorageKey(r.Context(), storageKeyInput)
	if err != nil {
		return nil, err
	}
	backend, err := s.storageBackendForKey(r.Context(), storageKey)
	if err != nil {
		return nil, err
	}
	obj, err := storage.SaveLPK(r.Context(), backend, file, header.Filename, s.effectiveMaxLPKSize(r.Context()))
	if err != nil {
		return nil, err
	}
	status := appversion.StatusPENDING
	published := false
	if isAdmin(u) || record.AllowUnreviewedUpdates {
		status = appversion.StatusAPPROVED
		published = true
	}
	create := s.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(u.ID).
		SetVersion(versionName).
		SetChangelog(changelog).
		SetStatus(status).
		SetSourceType(appversion.SourceTypeLOCAL).
		SetDownloadURL(s.absoluteURL(r.Context(), obj.DownloadURL)).
		SetStorageKey(storageKey).
		SetStoragePath(obj.Path).
		SetFileSize(obj.Size).
		SetSha256(obj.SHA256)
	if published {
		create.SetPublishedAt(time.Now())
	}
	created, err := create.Save(r.Context())
	if err != nil {
		_ = backend.Delete(r.Context(), obj.Path)
		return nil, err
	}
	if status == appversion.StatusPENDING {
		_, _ = s.db.ReviewRequest.Create().
			SetKind(reviewrequest.KindVERSION_UPLOAD).
			SetStatus(reviewrequest.StatusPENDING).
			SetAppID(record.ID).
			SetVersionID(created.ID).
			SetRequesterID(u.ID).
			Save(r.Context())
	} else {
		_ = s.updateAppFromApprovedLPKMetadata(r, record.ID, meta)
		s.clearAppOutdatedMarks(r, record.ID)
		if _, _, err := s.enforceVersionRetention(r.Context(), record.ID); err != nil {
			slog.Warn("Could not enforce version retention", "app_id", record.ID, "error", err)
		}
	}
	return created, nil
}

func (s *Server) createExternalVersion(r *http.Request, u *entgo.User, record *entgo.App, versionName, changelog, downloadURL, sourceType, sha256 string, fileSize int64) (*entgo.AppVersion, error) {
	versionName = strings.TrimSpace(versionName)
	downloadURL = normalizeGitHubRawURL(downloadURL)
	sha256 = strings.TrimSpace(strings.ToLower(sha256))
	if versionName == "" || downloadURL == "" {
		return nil, errors.New("version and downloadUrl are required")
	}
	if sha256 == "" {
		return nil, errors.New("sha256 is required for external versions")
	}
	if !isSHA256Hex(sha256) {
		return nil, errors.New("sha256 must be a 64-character hexadecimal string")
	}
	var source appversion.SourceType
	switch strings.ToUpper(strings.TrimSpace(sourceType)) {
	case "", "GITHUB":
		source = appversion.SourceTypeGITHUB
	case "WEBDAV":
		source = appversion.SourceTypeWEBDAV
	case "S3":
		source = appversion.SourceTypeS3
	default:
		return nil, fmt.Errorf("unsupported sourceType %q", sourceType)
	}
	status := appversion.StatusPENDING
	if isAdmin(u) || record.AllowUnreviewedUpdates {
		status = appversion.StatusAPPROVED
	}
	create := s.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(u.ID).
		SetVersion(versionName).
		SetChangelog(changelog).
		SetStatus(status).
		SetSourceType(source).
		SetDownloadURL(downloadURL).
		SetSha256(sha256)
	if fileSize > 0 {
		create.SetFileSize(fileSize)
	}
	if status == appversion.StatusAPPROVED {
		create.SetPublishedAt(time.Now())
	}
	created, err := create.Save(r.Context())
	if err != nil {
		return nil, err
	}
	if status == appversion.StatusPENDING {
		_, _ = s.db.ReviewRequest.Create().
			SetKind(reviewrequest.KindVERSION_UPLOAD).
			SetStatus(reviewrequest.StatusPENDING).
			SetAppID(record.ID).
			SetVersionID(created.ID).
			SetRequesterID(u.ID).
			Save(r.Context())
	} else {
		_, _ = s.db.App.UpdateOneID(record.ID).SetStatus(app.StatusAPPROVED).Save(r.Context())
		s.clearAppOutdatedMarks(r, record.ID)
		if _, _, err := s.enforceVersionRetention(r.Context(), record.ID); err != nil {
			slog.Warn("Could not enforce version retention", "app_id", record.ID, "error", err)
		}
	}
	return created, nil
}

func (s *Server) updateAppFromApprovedLPKMetadata(r *http.Request, appID int, meta lpkmeta.Metadata) error {
	update := s.db.App.UpdateOneID(appID).SetStatus(app.StatusAPPROVED)
	iconAssetID := 0
	if !meta.NameI18n.IsZero() {
		update.SetNameI18nJSON(catalogmeta.EncodeLocalizedText(meta.NameI18n))
	}
	if !meta.DescriptionI18n.IsZero() {
		update.SetDescriptionI18nJSON(catalogmeta.EncodeLocalizedText(meta.DescriptionI18n))
		summaryI18n := packageSummaryI18n(meta.DescriptionI18n)
		if !summaryI18n.IsZero() {
			update.SetSummaryI18nJSON(catalogmeta.EncodeLocalizedText(summaryI18n))
		}
	}
	if strings.TrimSpace(meta.Name) != "" {
		update.SetName(meta.Name)
	}
	if strings.TrimSpace(meta.Description) != "" {
		update.SetDescription(meta.Description)
		if summary := packageSummary(meta.Description); summary != "" {
			update.SetSummary(summary)
		}
	}
	if len(meta.IconData) > 0 {
		iconURL, assetID, err := s.saveLPKIconAsset(r.Context(), meta)
		if err != nil {
			return err
		}
		if iconURL != "" {
			update.SetIconURL(iconURL)
			iconAssetID = assetID
		}
	}
	_, err := update.Save(r.Context())
	if err != nil {
		if iconAssetID > 0 {
			_ = s.cleanupAssetIDs(r.Context(), iconAssetID)
		}
		return err
	}
	if iconAssetID > 0 {
		return s.replaceAssetLinks(r.Context(), assetOwnerApp, appID, assetRoleIcon, iconAssetID)
	}
	return nil
}

func appInputNeedsLPKInspection(input createAppJSON) bool {
	return strings.TrimSpace(input.PackageID) == "" ||
		strings.TrimSpace(input.Name) == "" ||
		strings.TrimSpace(input.Version) == "" ||
		strings.TrimSpace(input.SHA256) == ""
}

func versionInputNeedsLPKInspection(input createAppJSON) bool {
	return strings.TrimSpace(input.Version) == "" || strings.TrimSpace(input.SHA256) == ""
}

func (s *Server) applyAppMetadata(ctx context.Context, input *createAppJSON, meta lpkmeta.Metadata) error {
	if input.PackageID != "" && meta.PackageID != "" && input.PackageID != meta.PackageID {
		return fmt.Errorf("LPK package %q does not match packageId %q", meta.PackageID, input.PackageID)
	}
	if input.PackageID == "" {
		input.PackageID = meta.PackageID
	}
	if input.Name == "" {
		input.Name = meta.Name
		if input.Name == "" {
			input.Name = packageDisplayName(meta.PackageID)
		}
	}
	if catalogmeta.LocalizedText(input.NameI18n).IsZero() {
		input.NameI18n = meta.NameI18n
	}
	if input.Summary == "" {
		input.Summary = packageSummary(meta.Description)
	}
	if catalogmeta.LocalizedText(input.SummaryI18n).IsZero() {
		input.SummaryI18n = packageSummaryI18n(meta.DescriptionI18n)
	}
	if input.Description == "" {
		input.Description = meta.Description
	}
	if catalogmeta.LocalizedText(input.DescriptionI18n).IsZero() {
		input.DescriptionI18n = meta.DescriptionI18n
	}
	if input.Version == "" {
		input.Version = meta.Version
	}
	if strings.TrimSpace(input.IconURL) == "" {
		iconURL, assetID, err := s.saveLPKIconAsset(ctx, meta)
		if err != nil {
			return err
		}
		input.IconURL = iconURL
		input.iconAssetID = assetID
	}
	return nil
}

func packageDisplayName(packageID string) string {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return ""
	}
	parts := strings.Split(packageID, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}

func packageSummary(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	line := strings.TrimSpace(strings.Split(description, "\n")[0])
	runes := []rune(line)
	if len(runes) <= 120 {
		return line
	}
	return strings.TrimSpace(string(runes[:117])) + "..."
}

func packageSummaryI18n(values catalogmeta.LocalizedText) catalogmeta.LocalizedText {
	out := catalogmeta.LocalizedText{}
	for key, value := range catalogmeta.CleanLocalizedText(values) {
		summary := packageSummary(value)
		if summary != "" {
			out[key] = summary
		}
	}
	return out
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func (s *Server) canUploadVersion(r *http.Request, record *entgo.App, u *entgo.User) bool {
	if isAdmin(u) || record.OwnerID == u.ID {
		return true
	}
	return s.isCollaborator(r, record.ID, u.ID)
}

func (s *Server) isCollaborator(r *http.Request, appID, userID int) bool {
	ok, _ := s.db.Collaborator.Query().Where(collaborator.AppIDEQ(appID), collaborator.UserIDEQ(userID)).Exist(r.Context())
	return ok
}

func (s *Server) collaboratorAppIDs(ctx context.Context, userID int) []int {
	records, err := s.db.Collaborator.Query().Where(collaborator.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return nil
	}
	ids := make([]int, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.AppID)
	}
	return ids
}

func (s *Server) appSummaryDTO(r *http.Request, record *entgo.App, u *entgo.User) appSummary {
	dto := appSummary{
		ID:                        record.ID,
		OwnerID:                   record.OwnerID,
		CategoryID:                record.CategoryID,
		PackageID:                 record.PackageID,
		Name:                      record.Name,
		NameI18n:                  catalogmeta.DecodeLocalizedText(record.NameI18nJSON),
		Slug:                      record.Slug,
		Summary:                   record.Summary,
		SummaryI18n:               catalogmeta.DecodeLocalizedText(record.SummaryI18nJSON),
		Description:               record.Description,
		DescriptionI18n:           catalogmeta.DecodeLocalizedText(record.DescriptionI18nJSON),
		IconURL:                   record.IconURL,
		Status:                    string(record.Status),
		AllowUnreviewedUpdates:    record.AllowUnreviewedUpdates,
		CommentsEnabled:           record.CommentsEnabled,
		CommentsAllowed:           s.commentsAllowed(r.Context(), record.CommentsEnabled),
		EmailNotificationsEnabled: record.EmailNotificationsEnabled,
		InstallProtected:          record.InstallPasswordHash != "",
		DownloadCount:             record.DownloadCount,
		DownloadStats:             s.downloadStatsForApp(r.Context(), record.ID, record.DownloadCount),
		Rating:                    s.ratingForApp(r.Context(), record.ID, u),
		CreatedAt:                 record.CreatedAt,
		UpdatedAt:                 record.UpdatedAt,
		Tags:                      []string{},
		VisibleGroupIDs:           s.visibleGroupIDs(r.Context(), record.ID),
	}
	if owner, err := s.db.User.Get(r.Context(), record.OwnerID); err == nil {
		dto.Owner = userDisplayName(owner)
	}
	if u != nil {
		dto.CanManageApp = isAdmin(u) || record.OwnerID == u.ID
		dto.CanUploadVersion = dto.CanManageApp || s.isCollaborator(r, record.ID, u.ID)
		dto.AppFavorited, _ = s.db.Favorite.Query().
			Where(favoritepkg.UserIDEQ(u.ID), favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDEQ(record.ID)).
			Exist(r.Context())
		dto.SubmitterFavorited, _ = s.db.Favorite.Query().
			Where(favoritepkg.UserIDEQ(u.ID), favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeSUBMITTER), favoritepkg.TargetIDEQ(record.OwnerID)).
			Exist(r.Context())
	}
	if record.CategoryID != nil {
		if cat, err := s.db.Category.Get(r.Context(), *record.CategoryID); err == nil {
			dto.Category = cat.Name
			dto.CategoryI18n = catalogmeta.DecodeLocalizedText(cat.NameI18n)
		}
	}
	dto.Tags = s.tagNames(r, record.ID)
	if latest, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		First(r.Context()); err == nil {
		v := toVersionDTO(latest)
		dto.LatestVersion = &v
	}
	return dto
}

func toVersionDTO(v *entgo.AppVersion) version {
	return version{
		ID:          v.ID,
		AppID:       v.AppID,
		UploaderID:  v.UploaderID,
		Version:     v.Version,
		Changelog:   v.Changelog,
		Status:      string(v.Status),
		SourceType:  string(v.SourceType),
		DownloadURL: v.DownloadURL,
		StorageKey:  v.StorageKey,
		StoragePath: v.StoragePath,
		FileSize:    v.FileSize,
		SHA256:      v.Sha256,
		PublishedAt: v.PublishedAt,
		CreatedAt:   v.CreatedAt,
	}
}

func (s *Server) handleDownloadVersion(w http.ResponseWriter, r *http.Request) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	versionID, err := strconv.Atoi(r.PathValue("versionId"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil || record.Status != app.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.userCanSeeApp(r, record, s.optionalUser(r)) {
		allowed, err := s.requestHasGroupCodeForApp(r, record.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "GROUP_CODE_CHECK_FAILED", "Could not validate group code", nil)
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "GROUP_CODE_REQUIRED", "A valid group code is required for this app", nil)
			return
		}
	}
	versionRecord, err := s.db.AppVersion.Get(r.Context(), versionID)
	if err != nil || versionRecord.AppID != appID || versionRecord.Status != appversion.StatusAPPROVED {
		writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", "Version not found", nil)
		return
	}
	if record.InstallPasswordHash != "" {
		password := r.Header.Get("X-Install-Password")
		if password == "" {
			password = r.URL.Query().Get("installPassword")
		}
		if !auth.CheckPassword(record.InstallPasswordHash, password) {
			writeError(w, http.StatusUnauthorized, "INSTALL_PASSWORD_REQUIRED", "A valid install password is required", nil)
			return
		}
	}
	downloadURL := versionRecord.DownloadURL
	if mirrorID := strings.TrimSpace(r.URL.Query().Get("mirrorId")); mirrorID != "" {
		if !mirror.IsGitHubURL(downloadURL) {
			writeError(w, http.StatusUnprocessableEntity, "MIRROR_NOT_APPLICABLE", "Mirror can only be used with GitHub downloads", nil)
			return
		}
		entry, ok := mirror.FindApplicable(s.effectiveGitHubMirrors(r.Context()), mirrorID, downloadURL)
		if !ok {
			writeError(w, http.StatusUnprocessableEntity, "MIRROR_NOT_FOUND", "Mirror not found", nil)
			return
		}
		downloadURL = mirror.RewriteGitHub(downloadURL, entry)
	}
	if err := s.recordAppDownload(r.Context(), appID, versionRecord.Version); err != nil {
		writeError(w, http.StatusInternalServerError, "DOWNLOAD_RECORD_FAILED", "Could not record download", nil)
		return
	}
	http.Redirect(w, r, downloadURL, http.StatusFound)
}

func (s *Server) requestHasGroupCodeForApp(r *http.Request, appID int) (bool, error) {
	appGroupIDs := s.visibleGroupIDs(r.Context(), appID)
	if len(appGroupIDs) == 0 {
		return true, nil
	}
	access, err := s.resolveGroupCodeAccess(r.Context(), groupCodesFromRequest(r))
	if err != nil {
		return false, err
	}
	valid := map[int]struct{}{}
	for _, groupID := range access.validGroupIDs {
		valid[groupID] = struct{}{}
	}
	for _, groupID := range appGroupIDs {
		if _, ok := valid[groupID]; ok {
			return true, nil
		}
	}
	return false, nil
}

func hashInstallPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}
	if len([]rune(password)) < 4 {
		return "", errors.New("install password must be at least 4 characters")
	}
	if len(password) > 256 {
		return "", errors.New("install password must be at most 256 bytes")
	}
	return auth.HashPassword(password)
}

func (s *Server) handleUploadScreenshot(w http.ResponseWriter, r *http.Request, u *entgo.User) {
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
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app maintainers can upload screenshots", nil)
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		badRequest(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Screenshots must be png, jpg, or webp", nil)
		return
	}
	storageKey, err := s.uploadStorageKey(r.Context(), r.FormValue("storageKey"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SCREENSHOT_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	backend, err := s.storageBackendForKey(r.Context(), storageKey)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SCREENSHOT_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), backend, file, header.Filename, 10<<20)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SCREENSHOT_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	deviceType := appscreenshot.DeviceType(catalogmeta.CleanDeviceType(r.FormValue("deviceType")))
	count, _ := s.db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(appID)).Count(r.Context())
	created, err := s.db.AppScreenshot.Create().
		SetAppID(appID).
		SetUploaderID(u.ID).
		SetImageURL(s.absoluteURL(r.Context(), obj.DownloadURL)).
		SetStorageKey(storageKey).
		SetStoragePath(obj.Path).
		SetCaption(r.FormValue("caption")).
		SetDeviceType(deviceType).
		SetSortOrder(count).
		Save(r.Context())
	if err != nil {
		_ = backend.Delete(r.Context(), obj.Path)
		writeError(w, http.StatusInternalServerError, "SCREENSHOT_CREATE_FAILED", "Could not save screenshot", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"screenshot": toScreenshotDTO(created)})
}

type screenshotOrderRequest struct {
	Items []struct {
		ID        int `json:"id"`
		SortOrder int `json:"sortOrder"`
	} `json:"items"`
}

type screenshotUpdateRequest struct {
	Caption string `json:"caption"`
}

func (s *Server) handleReorderScreenshots(w http.ResponseWriter, r *http.Request, u *entgo.User) {
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
	if !s.canManageAppAssets(record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app maintainers can reorder screenshots", nil)
		return
	}
	var input screenshotOrderRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	for _, item := range input.Items {
		if item.ID <= 0 {
			continue
		}
		shot, err := s.db.AppScreenshot.Get(r.Context(), item.ID)
		if err != nil || shot.AppID != appID {
			writeError(w, http.StatusNotFound, "SCREENSHOT_NOT_FOUND", "Screenshot not found", nil)
			return
		}
		_, _ = s.db.AppScreenshot.UpdateOneID(item.ID).SetSortOrder(item.SortOrder).Save(r.Context())
	}
	shots, err := s.loadScreenshots(r, appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SCREENSHOT_LIST_FAILED", "Could not list screenshots", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"screenshots": shots})
}

func (s *Server) handleUpdateScreenshot(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	screenshotID, err := strconv.Atoi(r.PathValue("screenshotId"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.canManageAppAssets(record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app maintainers can update screenshots", nil)
		return
	}
	var input screenshotUpdateRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	shot, err := s.db.AppScreenshot.Get(r.Context(), screenshotID)
	if err != nil || shot.AppID != appID {
		writeError(w, http.StatusNotFound, "SCREENSHOT_NOT_FOUND", "Screenshot not found", nil)
		return
	}
	updated, err := s.db.AppScreenshot.UpdateOneID(screenshotID).
		SetCaption(strings.TrimSpace(input.Caption)).
		Save(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SCREENSHOT_UPDATE_FAILED", "Could not update screenshot", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"screenshot": toScreenshotDTO(updated)})
}

func (s *Server) handleDeleteScreenshot(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		badRequest(w, err)
		return
	}
	screenshotID, err := strconv.Atoi(r.PathValue("screenshotId"))
	if err != nil {
		badRequest(w, err)
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !s.canManageAppAssets(record, u) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only app maintainers can delete screenshots", nil)
		return
	}
	shot, err := s.db.AppScreenshot.Get(r.Context(), screenshotID)
	if err != nil || shot.AppID != appID {
		writeError(w, http.StatusNotFound, "SCREENSHOT_NOT_FOUND", "Screenshot not found", nil)
		return
	}
	if err := s.db.AppScreenshot.DeleteOneID(screenshotID).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "SCREENSHOT_DELETE_FAILED", "Could not delete screenshot", nil)
		return
	}
	if shot.StoragePath != "" {
		s.deleteStoredObject(r.Context(), shot.StorageKey, shot.StoragePath)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) canManageAppAssets(record *entgo.App, u *entgo.User) bool {
	return isAdmin(u) || record.OwnerID == u.ID
}

func (s *Server) loadScreenshots(r *http.Request, appID int) ([]screenshot, error) {
	records, err := s.db.AppScreenshot.Query().
		Where(appscreenshot.AppIDEQ(appID)).
		Order(entgo.Asc(appscreenshot.FieldSortOrder), entgo.Asc(appscreenshot.FieldCreatedAt)).
		All(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]screenshot, 0, len(records))
	for _, record := range records {
		out = append(out, toScreenshotDTO(record))
	}
	return out, nil
}

func toScreenshotDTO(record *entgo.AppScreenshot) screenshot {
	return screenshot{
		ID:         record.ID,
		AppID:      record.AppID,
		ImageURL:   record.ImageURL,
		StorageKey: record.StorageKey,
		Caption:    record.Caption,
		DeviceType: catalogmeta.CleanDeviceType(record.DeviceType.String()),
		SortOrder:  record.SortOrder,
		CreatedAt:  record.CreatedAt,
	}
}

func (s *Server) syncTags(r *http.Request, appID int, tagNames []string) error {
	_, _ = s.db.AppTag.Delete().Where(apptag.AppIDEQ(appID)).Exec(r.Context())
	if len(tagNames) == 0 {
		return nil
	}
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		slug := slugify(name)
		tagRecord, err := s.db.Tag.Query().Where(tag.SlugEQ(slug)).Only(r.Context())
		if err != nil {
			tagRecord, err = s.db.Tag.Create().
				SetName(name).
				SetNameI18n(catalogmeta.EncodeLocalizedText(catalogmeta.WithLocalizedDefaults(nil, name))).
				SetSlug(slug).
				Save(r.Context())
			if err != nil {
				return err
			}
		} else if catalogmeta.DecodeLocalizedText(tagRecord.NameI18n).IsZero() {
			updated, err := s.db.Tag.UpdateOneID(tagRecord.ID).
				SetNameI18n(catalogmeta.EncodeLocalizedText(catalogmeta.WithLocalizedDefaults(nil, tagRecord.Name))).
				Save(r.Context())
			if err == nil {
				tagRecord = updated
			}
		}
		_, _ = s.db.AppTag.Create().SetAppID(appID).SetTagID(tagRecord.ID).Save(r.Context())
	}
	return nil
}

func (s *Server) tagNames(r *http.Request, appID int) []string {
	links, err := s.db.AppTag.Query().Where(apptag.AppIDEQ(appID)).All(r.Context())
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(links))
	for _, link := range links {
		record, err := s.db.Tag.Get(r.Context(), link.TagID)
		if err == nil {
			names = append(names, record.Name)
		}
	}
	return names
}

func (s *Server) loadComments(r *http.Request, appID int) ([]comment, error) {
	records, err := s.db.Comment.Query().
		Where(commentpkg.AppIDEQ(appID), commentpkg.DeletedEQ(false)).
		Order(entgo.Asc(commentpkg.FieldCreatedAt)).
		Limit(200).
		All(r.Context())
	if err != nil {
		return nil, err
	}
	actor := s.optionalCommentActor(r)
	appRecord, _ := s.db.App.Get(r.Context(), appID)
	canMaintain := appRecord != nil && s.actorCanMaintainApp(actor, appRecord)

	byID := make(map[int]comment, len(records))
	topLevelIDs := make([]int, 0, len(records))
	repliesByParent := make(map[int][]comment)
	for _, record := range records {
		dto := s.commentDTO(r, record, actor, canMaintain)
		byID[record.ID] = dto
		if record.ParentID != nil {
			repliesByParent[*record.ParentID] = append(repliesByParent[*record.ParentID], dto)
			continue
		}
		topLevelIDs = append(topLevelIDs, record.ID)
	}

	out := make([]comment, 0, len(topLevelIDs))
	for _, id := range topLevelIDs {
		dto := byID[id]
		dto.Replies = repliesByParent[id]
		out = append(out, dto)
	}
	return out, nil
}

func (s *Server) commentDTO(r *http.Request, record *entgo.Comment, actor commentActor, canMaintain bool) comment {
	authorType := string(record.AuthorType)
	if authorType == "" {
		authorType = string(commentpkg.AuthorTypeUSER)
	}
	username := strings.TrimSpace(record.AuthorName)
	if username == "" && record.AuthorType == commentpkg.AuthorTypeUSER && record.UserID > 0 {
		if u, err := s.db.User.Get(r.Context(), record.UserID); err == nil {
			username = userDisplayName(u)
		}
	}
	if username == "" {
		if record.AuthorType == commentpkg.AuthorTypeCLIENT {
			clientID := trimRunes(record.ClientUserID, 12)
			if clientID == "" {
				username = "App Store Client"
			} else {
				username = "MiaoMiao " + clientID
			}
		} else if record.UserID > 0 {
			username = fmt.Sprintf("User #%d", record.UserID)
		} else {
			username = "User"
		}
	}
	canDelete := canMaintain
	if !canDelete && actor.User != nil && record.AuthorType == commentpkg.AuthorTypeUSER && record.UserID == actor.User.ID {
		canDelete = true
	}
	if !canDelete && actor.IsClient && record.AuthorType == commentpkg.AuthorTypeCLIENT && record.ClientUserID != "" && record.ClientUserID == actor.ClientUserID {
		canDelete = true
	}
	return comment{
		ID:           record.ID,
		AppID:        record.AppID,
		UserID:       record.UserID,
		ParentID:     record.ParentID,
		AuthorType:   authorType,
		ClientUserID: record.ClientUserID,
		Username:     username,
		Body:         record.Body,
		CanDelete:    canDelete,
		CreatedAt:    record.CreatedAt,
	}
}

func (s *Server) absoluteURL(ctx context.Context, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(s.sitePublicURL(ctx), "/") + "/" + strings.TrimLeft(path, "/")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parsePositiveIDList(value string) []int {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	seen := map[int]struct{}{}
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func formBool(r *http.Request, key string) bool {
	value := strings.ToLower(strings.TrimSpace(r.FormValue(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
