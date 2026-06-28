package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/collaborator"
	"lazycat.community/appstore/ent/collaboratorrequest"
	"lazycat.community/appstore/ent/collectionapp"
	commentpkg "lazycat.community/appstore/ent/comment"
	favoritepkg "lazycat.community/appstore/ent/favorite"
	outdatedpkg "lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/internal/auth"
	"lazycat.community/appstore/internal/storage"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	u := s.optionalUser(r)
	q := s.db.App.Query().Order(entgo.Desc(app.FieldUpdatedAt)).Limit(100)
	if !isAdmin(u) {
		if u == nil {
			q.Where(app.StatusEQ(app.StatusAPPROVED))
		} else {
			q.Where(app.Or(app.StatusEQ(app.StatusAPPROVED), app.OwnerIDEQ(u.ID)))
		}
	}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		q.Where(app.Or(app.NameContainsFold(search), app.SummaryContainsFold(search), app.DescriptionContainsFold(search)))
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && isAdmin(u) {
		q.Where(app.StatusEQ(app.Status(status)))
	}
	if owner := strings.TrimSpace(r.URL.Query().Get("submitter")); owner != "" {
		if ownerID, err := strconv.Atoi(owner); err == nil {
			q.Where(app.OwnerIDEQ(ownerID))
		}
	}
	apps, err := q.All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_LIST_FAILED", "Could not list apps", nil)
		return
	}
	out := make([]appSummary, 0, len(apps))
	for _, record := range apps {
		if !s.userCanSeeApp(r, record, u) {
			continue
		}
		dto := s.appSummaryDTO(r, record, u)
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out})
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
	detail := appDetail{appSummary: s.appSummaryDTO(r, record, u)}
	versionRecords, _ := s.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).Order(entgo.Desc(appversion.FieldCreatedAt)).All(r.Context())
	for _, v := range versionRecords {
		if v.Status != appversion.StatusAPPROVED && (u == nil || (!isAdmin(u) && record.OwnerID != u.ID)) {
			continue
		}
		detail.Versions = append(detail.Versions, toVersionDTO(v))
	}
	detail.Screenshots, _ = s.loadScreenshots(r, record.ID)
	comments, _ := s.loadComments(r, record.ID)
	detail.Comments = comments
	detail.Favorites, _ = s.db.Favorite.Query().Where(favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDEQ(record.ID)).Count(r.Context())
	if u != nil {
		detail.CanManageApp = isAdmin(u) || record.OwnerID == u.ID
		detail.CanUploadVersion = detail.CanManageApp || s.isCollaborator(r, record.ID, u.ID)
	}
	if detail.CanUploadVersion {
		detail.OutdatedMarks, _ = s.db.OutdatedMark.Query().Where(outdatedpkg.AppIDEQ(record.ID)).Count(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": detail})
}

type createAppJSON struct {
	Name                   string   `json:"name"`
	Slug                   string   `json:"slug"`
	Summary                string   `json:"summary"`
	Description            string   `json:"description"`
	CategoryID             *int     `json:"categoryId"`
	Tags                   []string `json:"tags"`
	AllowUnreviewedUpdates bool     `json:"allowUnreviewedUpdates"`
	CommentsEnabled        *bool    `json:"commentsEnabled"`
	InstallPassword        string   `json:"installPassword"`
	Version                string   `json:"version"`
	Changelog              string   `json:"changelog"`
	DownloadURL            string   `json:"downloadUrl"`
	SourceType             string   `json:"sourceType"`
	SHA256                 string   `json:"sha256"`
}

type updateAppJSON struct {
	Name                   *string  `json:"name,omitempty"`
	Summary                *string  `json:"summary,omitempty"`
	Description            *string  `json:"description,omitempty"`
	CategoryID             *int     `json:"categoryId,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
	TagsSet                bool     `json:"tagsSet,omitempty"`
	AllowUnreviewedUpdates *bool    `json:"allowUnreviewedUpdates,omitempty"`
	CommentsEnabled        *bool    `json:"commentsEnabled,omitempty"`
	InstallPassword        *string  `json:"installPassword,omitempty"`
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
	record, err := s.createAppRecord(r, u, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
		return
	}
	if input.DownloadURL != "" && input.Version != "" {
		_, err = s.createExternalVersion(r, u, record, input.Version, input.Changelog, input.DownloadURL, input.SourceType, input.SHA256)
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
	input := createAppJSON{
		Name:                   r.FormValue("name"),
		Slug:                   r.FormValue("slug"),
		Summary:                r.FormValue("summary"),
		Description:            r.FormValue("description"),
		Version:                r.FormValue("version"),
		Changelog:              r.FormValue("changelog"),
		InstallPassword:        r.FormValue("installPassword"),
		AllowUnreviewedUpdates: formBool(r, "allowUnreviewedUpdates"),
		CommentsEnabled:        &commentsEnabled,
	}
	if categoryID, err := strconv.Atoi(r.FormValue("categoryId")); err == nil && categoryID > 0 {
		input.CategoryID = &categoryID
	}
	if tags := strings.TrimSpace(r.FormValue("tags")); tags != "" {
		input.Tags = splitCSV(tags)
	}
	record, err := s.createAppRecord(r, u, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "APP_CREATE_FAILED", err.Error(), nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		if input.Version == "" {
			input.Version = "0.1.0"
		}
		if _, err := s.createUploadedVersion(r, u, record, input.Version, input.Changelog, file, header); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
			return
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"app": s.appSummaryDTO(r, record, u)})
}

func (s *Server) createAppRecord(r *http.Request, u *entgo.User, input createAppJSON) (*entgo.App, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("app name is required")
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	status := app.StatusPENDING
	if isAdmin(u) {
		status = app.StatusAPPROVED
	}
	commentsEnabled := true
	if input.CommentsEnabled != nil {
		commentsEnabled = *input.CommentsEnabled
	}
	create := s.db.App.Create().
		SetOwnerID(u.ID).
		SetName(name).
		SetSlug(slug).
		SetSummary(input.Summary).
		SetDescription(input.Description).
		SetStatus(status).
		SetAllowUnreviewedUpdates(input.AllowUnreviewedUpdates).
		SetCommentsEnabled(commentsEnabled)
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
		return nil, err
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

	if !isAdmin(u) && !record.AllowUnreviewedUpdates {
		if input.InstallPassword != nil {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Install password changes require direct app update permission", nil)
			return
		}
		raw, err := json.Marshal(input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_UPDATE_REVIEW_FAILED", "Could not prepare app update review", nil)
			return
		}
		review, err := s.db.ReviewRequest.Create().
			SetKind(reviewrequest.KindAPP_INFO_UPDATE).
			SetStatus(reviewrequest.StatusPENDING).
			SetAppID(record.ID).
			SetRequesterID(u.ID).
			SetNote(string(raw)).
			Save(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "APP_UPDATE_REVIEW_FAILED", "Could not create app update review", nil)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"review": review})
		return
	}

	updated, err := s.applyAppInfoUpdate(r, id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "APP_UPDATE_FAILED", "Could not update app", nil)
		return
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
			_ = s.storage.Delete(r.Context(), version.StoragePath)
		}
	}
	screenshots, _ := s.db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(id)).All(r.Context())
	for _, shot := range screenshots {
		if shot.StoragePath != "" {
			_ = s.storage.Delete(r.Context(), shot.StoragePath)
		}
	}
	_, _ = s.db.AppVersion.Delete().Where(appversion.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppScreenshot.Delete().Where(appscreenshot.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppVisibility.Delete().Where(appvisibility.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.AppTag.Delete().Where(apptag.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Collaborator.Delete().Where(collaborator.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.CollaboratorRequest.Delete().Where(collaboratorrequest.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.OutdatedMark.Delete().Where(outdatedpkg.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.ReviewRequest.Delete().Where(reviewrequest.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.CollectionApp.Delete().Where(collectionapp.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Comment.Delete().Where(commentpkg.AppIDEQ(id)).Exec(r.Context())
	_, _ = s.db.Favorite.Delete().Where(favoritepkg.TargetTypeEQ(favoritepkg.TargetTypeAPP), favoritepkg.TargetIDEQ(id)).Exec(r.Context())
	if err := s.db.App.DeleteOneID(id).Exec(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "APP_DELETE_FAILED", "Could not delete app", nil)
		return
	}
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
		defer file.Close()
		created, err := s.createUploadedVersion(r, u, record, r.FormValue("version"), r.FormValue("changelog"), file, header)
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
	created, err := s.createExternalVersion(r, u, record, input.Version, input.Changelog, input.DownloadURL, input.SourceType, input.SHA256)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VERSION_CREATE_FAILED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": toVersionDTO(created)})
}

func (s *Server) createUploadedVersion(r *http.Request, u *entgo.User, record *entgo.App, versionName, changelog string, file multipart.File, header *multipart.FileHeader) (*entgo.AppVersion, error) {
	versionName = strings.TrimSpace(versionName)
	if versionName == "" {
		return nil, errors.New("version is required")
	}
	obj, err := storage.SaveLPK(r.Context(), s.storage, file, header.Filename, s.effectiveMaxLPKSize(r.Context()))
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
		SetDownloadURL(s.absoluteURL(obj.DownloadURL)).
		SetStoragePath(obj.Path).
		SetFileSize(obj.Size).
		SetSha256(obj.SHA256)
	if published {
		create.SetPublishedAt(time.Now())
	}
	created, err := create.Save(r.Context())
	if err != nil {
		_ = s.storage.Delete(r.Context(), obj.Path)
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
		_ = s.db.App.UpdateOneID(record.ID).SetStatus(app.StatusAPPROVED).SaveX(r.Context())
		s.enforceVersionRetention(r, record.ID)
	}
	return created, nil
}

func (s *Server) createExternalVersion(r *http.Request, u *entgo.User, record *entgo.App, versionName, changelog, downloadURL, sourceType, sha256 string) (*entgo.AppVersion, error) {
	versionName = strings.TrimSpace(versionName)
	downloadURL = strings.TrimSpace(downloadURL)
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
	source := appversion.SourceTypeGITHUB
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
		s.enforceVersionRetention(r, record.ID)
	}
	return created, nil
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

func (s *Server) appSummaryDTO(r *http.Request, record *entgo.App, u *entgo.User) appSummary {
	dto := appSummary{
		ID:                     record.ID,
		OwnerID:                record.OwnerID,
		CategoryID:             record.CategoryID,
		Name:                   record.Name,
		Slug:                   record.Slug,
		Summary:                record.Summary,
		Description:            record.Description,
		IconURL:                record.IconURL,
		Status:                 string(record.Status),
		AllowUnreviewedUpdates: record.AllowUnreviewedUpdates,
		CommentsEnabled:        record.CommentsEnabled,
		InstallProtected:       record.InstallPasswordHash != "",
		DownloadCount:          record.DownloadCount,
		CreatedAt:              record.CreatedAt,
		UpdatedAt:              record.UpdatedAt,
		Tags:                   []string{},
		VisibleGroupIDs:        s.visibleGroupIDs(r.Context(), record.ID),
	}
	if owner, err := s.db.User.Get(r.Context(), record.OwnerID); err == nil {
		dto.Owner = owner.Username
	}
	if record.CategoryID != nil {
		if cat, err := s.db.Category.Get(r.Context(), *record.CategoryID); err == nil {
			dto.Category = cat.Name
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
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
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
	_, _ = s.db.App.UpdateOneID(appID).AddDownloadCount(1).Save(r.Context())
	http.Redirect(w, r, mirrorDownloadURL(versionRecord.DownloadURL, s.effectiveGitHubMirror(r.Context())), http.StatusFound)
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

func mirrorDownloadURL(rawURL, mirror string) string {
	if mirror == "" {
		return rawURL
	}
	if strings.Contains(rawURL, "github.com/") || strings.Contains(rawURL, "githubusercontent.com/") {
		return strings.TrimRight(mirror, "/") + "/" + rawURL
	}
	return rawURL
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
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Screenshots must be png, jpg, or webp", nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), s.storage, file, header.Filename, 10<<20)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "SCREENSHOT_UPLOAD_FAILED", err.Error(), nil)
		return
	}
	count, _ := s.db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(appID)).Count(r.Context())
	created, err := s.db.AppScreenshot.Create().
		SetAppID(appID).
		SetUploaderID(u.ID).
		SetImageURL(s.absoluteURL(obj.DownloadURL)).
		SetStoragePath(obj.Path).
		SetCaption(r.FormValue("caption")).
		SetSortOrder(count).
		Save(r.Context())
	if err != nil {
		_ = s.storage.Delete(r.Context(), obj.Path)
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
		_ = s.storage.Delete(r.Context(), shot.StoragePath)
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
		ID:        record.ID,
		AppID:     record.AppID,
		ImageURL:  record.ImageURL,
		Caption:   record.Caption,
		SortOrder: record.SortOrder,
		CreatedAt: record.CreatedAt,
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
			tagRecord, err = s.db.Tag.Create().SetName(name).SetSlug(slug).Save(r.Context())
			if err != nil {
				return err
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
	records, err := s.db.Comment.Query().Where(commentpkg.AppIDEQ(appID), commentpkg.DeletedEQ(false)).Order(entgo.Desc(commentpkg.FieldCreatedAt)).Limit(50).All(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]comment, 0, len(records))
	for _, record := range records {
		dto := comment{ID: record.ID, AppID: record.AppID, UserID: record.UserID, Body: record.Body, CreatedAt: record.CreatedAt}
		if u, err := s.db.User.Get(r.Context(), record.UserID); err == nil {
			dto.Username = u.Username
		}
		out = append(out, dto)
	}
	return out, nil
}

func (s *Server) absoluteURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
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

func formBool(r *http.Request, key string) bool {
	value := strings.ToLower(strings.TrimSpace(r.FormValue(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
