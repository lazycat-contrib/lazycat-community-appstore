package server

import (
	"fmt"
	"net/http"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/internal/feed"
)

func (s *Server) handleSourceIndex(w http.ResponseWriter, r *http.Request) {
	sourcePassword := s.sourcePassword(r.Context())
	if sourcePassword != "" {
		password := r.Header.Get("X-Source-Password")
		if password == "" {
			password = r.URL.Query().Get("password")
		}
		if password != sourcePassword {
			writeError(w, http.StatusUnauthorized, "SOURCE_PASSWORD_REQUIRED", "A valid source password is required", nil)
			return
		}
	}

	apps, err := s.db.App.Query().
		Where(app.StatusEQ(app.StatusAPPROVED)).
		Order(entgo.Desc(app.FieldUpdatedAt)).
		All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_BUILD_FAILED", "Could not build source index", nil)
		return
	}

	input := feed.Input{
		BaseURL:      s.cfg.BaseURL,
		GitHubMirror: s.effectiveGitHubMirror(r.Context()),
		Apps:         make([]feed.AppInput, 0, len(apps)),
	}
	for _, record := range apps {
		if !s.appIsPublic(r.Context(), record.ID) {
			continue
		}
		appInput := feed.AppInput{
			ID:               record.ID,
			Name:             record.Name,
			Slug:             record.Slug,
			Summary:          record.Summary,
			Description:      record.Description,
			UpdatedAt:        record.UpdatedAt,
			Tags:             s.tagNames(r, record.ID),
			InstallProtected: record.InstallPasswordHash != "",
		}
		if record.IconURL != nil {
			appInput.IconURL = *record.IconURL
		}
		if owner, err := s.db.User.Get(r.Context(), record.OwnerID); err == nil {
			appInput.Submitter = owner.Username
		}
		if record.CategoryID != nil {
			if category, err := s.db.Category.Get(r.Context(), *record.CategoryID); err == nil {
				appInput.Category = category.Name
			}
		}
		versions, _ := s.db.AppVersion.Query().
			Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
			Order(entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
			All(r.Context())
		for _, versionRecord := range versions {
			appInput.Versions = append(appInput.Versions, feed.VersionInput{
				Version:             versionRecord.Version,
				Status:              string(versionRecord.Status),
				Changelog:           versionRecord.Changelog,
				SourceType:          string(versionRecord.SourceType),
				DownloadURL:         s.absoluteURL(fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, versionRecord.ID)),
				UpstreamDownloadURL: versionRecord.DownloadURL,
				SHA256:              versionRecord.Sha256,
				Size:                versionRecord.FileSize,
				PublishedAt:         valueTime(versionRecord.PublishedAt),
			})
		}
		input.Apps = append(input.Apps, appInput)
	}

	writeJSON(w, http.StatusOK, feed.BuildIndex(input))
}

func valueTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
