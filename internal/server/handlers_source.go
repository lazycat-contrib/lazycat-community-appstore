package server

import (
	"fmt"
	"net/http"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/internal/catalogmeta"
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

	profile := s.siteProfile(r.Context())
	input := feed.Input{
		BaseURL:       profile.PublicURL,
		GitHubMirrors: s.effectiveGitHubMirrors(r.Context()),
		Site: feed.SiteMeta{
			Title:     profile.Title,
			IconURL:   profile.IconURL,
			PublicURL: profile.PublicURL,
			SourceURL: profile.SourceURL,
		},
		Announcement: feed.AnnouncementMeta{
			Enabled:   profile.Announcement.Enabled,
			Level:     profile.Announcement.Level,
			Title:     profile.Announcement.Title,
			Body:      profile.Announcement.Body,
			LinkLabel: profile.Announcement.LinkLabel,
			LinkURL:   profile.Announcement.LinkURL,
			UpdatedAt: profile.Announcement.UpdatedAt,
		},
		Apps: make([]feed.AppInput, 0, len(apps)),
	}
	siteCommentsEnabled := s.commentsEnabled(r.Context())
	for _, record := range apps {
		if !s.appIsPublic(r.Context(), record.ID) {
			continue
		}
		commentsEnabled := siteCommentsEnabled && record.CommentsEnabled
		appInput := feed.AppInput{
			ID:               record.ID,
			PackageID:        record.PackageID,
			Name:             record.Name,
			Slug:             record.Slug,
			Summary:          record.Summary,
			Description:      record.Description,
			UpdatedAt:        record.UpdatedAt,
			Tags:             s.tagNames(r, record.ID),
			InstallProtected: record.InstallPasswordHash != "",
			CommentsEnabled:  &commentsEnabled,
		}
		appInput.OutdatedMarks, _ = s.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).Count(r.Context())
		if record.IconURL != nil {
			appInput.IconURL = *record.IconURL
		}
		if screenshots, err := s.loadScreenshots(r, record.ID); err == nil {
			for _, shot := range screenshots {
				appInput.Screenshots = append(appInput.Screenshots, catalogmeta.Screenshot{
					ID:         shot.ID,
					AppID:      shot.AppID,
					ImageURL:   shot.ImageURL,
					Caption:    shot.Caption,
					DeviceType: shot.DeviceType,
					SortOrder:  shot.SortOrder,
					CreatedAt:  shot.CreatedAt,
				})
			}
		}
		if owner, err := s.db.User.Get(r.Context(), record.OwnerID); err == nil {
			appInput.Submitter = userDisplayName(owner)
		}
		if record.CategoryID != nil {
			if category, err := s.db.Category.Get(r.Context(), *record.CategoryID); err == nil {
				appInput.Category = category.Name
				appInput.CategoryI18n = catalogmeta.DecodeLocalizedText(category.NameI18n)
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
				DownloadURL:         s.absoluteURL(r.Context(), fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, versionRecord.ID)),
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
