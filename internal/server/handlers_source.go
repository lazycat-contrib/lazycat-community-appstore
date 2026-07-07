package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/ent/user"
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
	preload, err := s.sourceIndexPreload(r.Context(), apps)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_BUILD_FAILED", "Could not build source index", nil)
		return
	}
	for _, record := range apps {
		if _, ok := preload.publicAppIDs[record.ID]; !ok {
			continue
		}
		commentsEnabled := siteCommentsEnabled && record.CommentsEnabled
		appInput := feed.AppInput{
			ID:               record.ID,
			PackageID:        record.PackageID,
			Name:             record.Name,
			NameI18n:         catalogmeta.DecodeLocalizedText(record.NameI18nJSON),
			Slug:             record.Slug,
			Summary:          record.Summary,
			SummaryI18n:      catalogmeta.DecodeLocalizedText(record.SummaryI18nJSON),
			Description:      record.Description,
			DescriptionI18n:  catalogmeta.DecodeLocalizedText(record.DescriptionI18nJSON),
			UpdatedAt:        record.UpdatedAt,
			Tags:             preload.tags[record.ID],
			InstallProtected: record.InstallPasswordHash != "",
			CommentsEnabled:  &commentsEnabled,
			OutdatedMarks:    preload.outdatedMarks[record.ID],
			Screenshots:      preload.screenshots[record.ID],
			Submitter:        preload.submitters[record.OwnerID],
			Versions:         preload.versions[record.ID],
		}
		if record.IconURL != nil {
			appInput.IconURL = *record.IconURL
		}
		if record.CategoryID != nil {
			if categoryRecord := preload.categories[*record.CategoryID]; categoryRecord != nil {
				appInput.Category = categoryRecord.Name
				appInput.CategoryI18n = catalogmeta.DecodeLocalizedText(categoryRecord.NameI18n)
			}
		}
		input.Apps = append(input.Apps, appInput)
	}

	writeJSON(w, http.StatusOK, feed.BuildIndex(input))
}

type sourceIndexPreload struct {
	publicAppIDs  map[int]struct{}
	submitters    map[int]string
	categories    map[int]*entgo.Category
	tags          map[int][]string
	screenshots   map[int][]catalogmeta.Screenshot
	versions      map[int][]feed.VersionInput
	outdatedMarks map[int]int
}

func (s *Server) sourceIndexPreload(ctx context.Context, apps []*entgo.App) (sourceIndexPreload, error) {
	data := sourceIndexPreload{
		publicAppIDs:  make(map[int]struct{}, len(apps)),
		submitters:    map[int]string{},
		categories:    map[int]*entgo.Category{},
		tags:          map[int][]string{},
		screenshots:   map[int][]catalogmeta.Screenshot{},
		versions:      map[int][]feed.VersionInput{},
		outdatedMarks: map[int]int{},
	}
	if len(apps) == 0 {
		return data, nil
	}

	appIDs := make([]int, 0, len(apps))
	for _, record := range apps {
		appIDs = append(appIDs, record.ID)
	}
	visibilityRecords, err := s.db.AppVisibility.Query().
		Where(appvisibility.AppIDIn(appIDs...)).
		All(ctx)
	if err != nil {
		return data, err
	}
	privateAppIDs := make(map[int]struct{}, len(visibilityRecords))
	for _, record := range visibilityRecords {
		privateAppIDs[record.AppID] = struct{}{}
	}

	publicAppIDs := make([]int, 0, len(apps))
	ownerIDs := map[int]struct{}{}
	categoryIDs := map[int]struct{}{}
	for _, record := range apps {
		if _, private := privateAppIDs[record.ID]; private {
			continue
		}
		data.publicAppIDs[record.ID] = struct{}{}
		publicAppIDs = append(publicAppIDs, record.ID)
		ownerIDs[record.OwnerID] = struct{}{}
		if record.CategoryID != nil {
			categoryIDs[*record.CategoryID] = struct{}{}
		}
	}
	if len(publicAppIDs) == 0 {
		return data, nil
	}

	if err := s.loadSourceIndexSubmitters(ctx, ownerIDs, data.submitters); err != nil {
		return data, err
	}
	if err := s.loadSourceIndexCategories(ctx, categoryIDs, data.categories); err != nil {
		return data, err
	}
	if err := s.loadSourceIndexTags(ctx, publicAppIDs, data.tags); err != nil {
		return data, err
	}
	if err := s.loadSourceIndexScreenshots(ctx, publicAppIDs, data.screenshots); err != nil {
		return data, err
	}
	if err := s.loadSourceIndexVersions(ctx, publicAppIDs, data.versions); err != nil {
		return data, err
	}
	if err := s.loadSourceIndexOutdatedMarks(ctx, publicAppIDs, data.outdatedMarks); err != nil {
		return data, err
	}
	return data, nil
}

func (s *Server) loadSourceIndexSubmitters(ctx context.Context, ownerIDs map[int]struct{}, out map[int]string) error {
	ids := mapKeys(ownerIDs)
	if len(ids) == 0 {
		return nil
	}
	records, err := s.db.User.Query().Where(user.IDIn(ids...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.ID] = userDisplayName(record)
	}
	return nil
}

func (s *Server) loadSourceIndexCategories(ctx context.Context, categoryIDs map[int]struct{}, out map[int]*entgo.Category) error {
	ids := mapKeys(categoryIDs)
	if len(ids) == 0 {
		return nil
	}
	records, err := s.db.Category.Query().Where(category.IDIn(ids...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.ID] = record
	}
	return nil
}

func (s *Server) loadSourceIndexTags(ctx context.Context, appIDs []int, out map[int][]string) error {
	links, err := s.db.AppTag.Query().Where(apptag.AppIDIn(appIDs...)).All(ctx)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	tagIDs := map[int]struct{}{}
	for _, link := range links {
		tagIDs[link.TagID] = struct{}{}
	}
	tagRecords, err := s.db.Tag.Query().Where(tag.IDIn(mapKeys(tagIDs)...)).All(ctx)
	if err != nil {
		return err
	}
	tagNames := make(map[int]string, len(tagRecords))
	for _, record := range tagRecords {
		tagNames[record.ID] = record.Name
	}
	for _, link := range links {
		if name := tagNames[link.TagID]; name != "" {
			out[link.AppID] = append(out[link.AppID], name)
		}
	}
	return nil
}

func (s *Server) loadSourceIndexScreenshots(ctx context.Context, appIDs []int, out map[int][]catalogmeta.Screenshot) error {
	records, err := s.db.AppScreenshot.Query().
		Where(appscreenshot.AppIDIn(appIDs...)).
		Order(entgo.Asc(appscreenshot.FieldAppID), entgo.Asc(appscreenshot.FieldSortOrder), entgo.Asc(appscreenshot.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.AppID] = append(out[record.AppID], catalogmeta.Screenshot{
			ID:         record.ID,
			AppID:      record.AppID,
			ImageURL:   record.ImageURL,
			Caption:    record.Caption,
			DeviceType: catalogmeta.CleanDeviceType(record.DeviceType.String()),
			SortOrder:  record.SortOrder,
			CreatedAt:  record.CreatedAt,
		})
	}
	return nil
}

func (s *Server) loadSourceIndexVersions(ctx context.Context, appIDs []int, out map[int][]feed.VersionInput) error {
	records, err := s.db.AppVersion.Query().
		Where(appversion.AppIDIn(appIDs...), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Asc(appversion.FieldAppID), entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.AppID] = append(out[record.AppID], feed.VersionInput{
			Version:             record.Version,
			Status:              string(record.Status),
			Changelog:           record.Changelog,
			SourceType:          string(record.SourceType),
			DownloadURL:         s.absoluteURL(ctx, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.AppID, record.ID)),
			UpstreamDownloadURL: record.DownloadURL,
			SHA256:              record.Sha256,
			Size:                record.FileSize,
			PublishedAt:         valueTime(record.PublishedAt),
		})
	}
	return nil
}

func (s *Server) loadSourceIndexOutdatedMarks(ctx context.Context, appIDs []int, out map[int]int) error {
	records, err := s.db.OutdatedMark.Query().Where(outdatedmark.AppIDIn(appIDs...)).All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		out[record.AppID]++
	}
	return nil
}

func mapKeys(values map[int]struct{}) []int {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func valueTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
