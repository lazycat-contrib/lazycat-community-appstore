package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/ad"
	"lazycat.community/appstore/ent/announcement"
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
	feedv1 "lazycat.community/appstore/internal/feed/v1"
	feedv2 "lazycat.community/appstore/internal/feed/v2"
)

func (s *Server) handleSourceIndexV1(w http.ResponseWriter, r *http.Request) {
	if !s.sourceV1Enabled(r.Context()) {
		writeError(w, http.StatusGone, "SOURCE_V1_DISABLED", "The v1 source feed is disabled by the site administrator", nil)
		return
	}
	s.handleSourceIndex(w, r, 1)
}

func (s *Server) handleSourceIndexV2(w http.ResponseWriter, r *http.Request) {
	s.handleSourceIndex(w, r, 2)
}

func (s *Server) handleSourceIndex(w http.ResponseWriter, r *http.Request, version int) {
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

	groupCodes := groupCodesFromRequest(r)
	if len(groupCodes) > maxSourceGroupCodes {
		writeError(w, http.StatusBadRequest, "TOO_MANY_GROUP_CODES", "At most 64 group codes may be requested", nil)
		return
	}
	for {
		groupAccess, err := s.resolveGroupCodeAccess(r.Context(), groupCodes)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SOURCE_BUILD_FAILED", "Could not validate group codes", nil)
			return
		}
		scope := sourceFeedAccessScope{GroupIDs: groupAccess.validGroupIDs, Groups: groupAccess.validGroups}.canonical()
		var snapshot sourceFeedSnapshot
		if s.sourceFeedCache != nil {
			if len(groupAccess.invalidGroupCodes) == 0 {
				snapshot, err = s.sourceFeedCache.GetOrBuild(r.Context(), version, scope)
			} else {
				snapshot, err = s.sourceFeedCache.BuildUncached(r.Context(), version, scope, groupAccess.invalidGroupCodes)
			}
		} else {
			var built sourceFeedBuild
			built, err = s.buildSourceFeed(r.Context(), version, scope, groupAccess.invalidGroupCodes)
			if err == nil {
				snapshot, err = newSourceFeedSnapshotUntil(built.Identity, built.ValidUntil)
			}
		}
		if errors.Is(err, errSourceFeedGenerationChanged) || (err == nil && snapshot.expired(time.Now().UTC())) {
			continue
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SOURCE_BUILD_FAILED", "Could not build source index", nil)
			return
		}
		serveSourceFeedSnapshot(w, r, snapshot)
		return
	}
}

func (s *Server) buildSourceFeed(ctx context.Context, version int, scope sourceFeedAccessScope, invalidGroupCodes []string) (sourceFeedBuild, error) {
	asOf := time.Now().UTC()
	apps, err := s.db.App.Query().
		Where(app.StatusEQ(app.StatusAPPROVED)).
		Order(entgo.Desc(app.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return sourceFeedBuild{}, err
	}

	profile := s.siteProfileAt(ctx, asOf)
	input := feed.Input{
		BaseURL:       profile.PublicURL,
		GitHubMirrors: s.effectiveGitHubMirrors(ctx),
		Site: feed.SiteMeta{
			Title:     profile.Title,
			IconURL:   s.feedImageURL(ctx, profile.IconURL),
			PublicURL: profile.PublicURL,
			SourceURL: sourceFeedURL(profile.PublicURL, version),
			Chat: feed.ChatMeta{
				Enabled:       profile.Chat.Enabled,
				RetentionDays: profile.Chat.RetentionDays,
			},
		},
		Announcement: siteAnnouncementToFeed(profile.Announcement),
		Apps:         make([]feed.AppInput, 0, len(apps)),
	}
	for _, group := range scope.Groups {
		input.Groups = append(input.Groups, feed.GroupMeta{ID: group.ID, Name: group.Name, Code: group.Code})
	}
	input.InvalidGroupCodes = invalidGroupCodes
	siteCommentsEnabled := s.commentsEnabled(ctx)
	preload, err := s.sourceIndexPreload(ctx, apps, scope.GroupIDs)
	if err != nil {
		return sourceFeedBuild{}, err
	}
	if version >= 2 {
		input.Site.ClientPolicy = feed.ClientPolicyMeta{
			MinVersion: profile.ClientPolicy.MinVersion,
			Message:    profile.ClientPolicy.Message,
		}
		input.Categories = sourceIndexCategoryInputs(preload.categories)
		input.Announcements = siteAnnouncementsToFeed(profile.Announcements)
		input.Ads = siteAdsToFeed(profile.Ads)
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
			Author:           record.Author,
			Homepage:         record.Homepage,
			License:          record.License,
			MinOSVersion:     record.MinOsVersion,
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
			appInput.IconURL = s.feedImageURL(ctx, *record.IconURL)
		}
		if record.CategoryID != nil {
			if categoryRecord := preload.categories[*record.CategoryID]; categoryRecord != nil {
				appInput.CategoryID = record.CategoryID
				appInput.Category = categoryRecord.Name
				appInput.CategoryI18n = catalogmeta.DecodeLocalizedText(categoryRecord.NameI18n)
			}
		}
		input.Apps = append(input.Apps, appInput)
	}

	var output any
	if version >= 2 {
		output = feedv2.BuildIndex(input)
	} else {
		output = feedv1.BuildIndex(input)
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return sourceFeedBuild{}, err
	}
	validUntil, err := s.nextSourceFeedBoundary(ctx, asOf)
	if err != nil {
		return sourceFeedBuild{}, err
	}
	return sourceFeedBuild{Identity: append(raw, '\n'), ValidUntil: validUntil}, nil
}

func (s *Server) nextSourceFeedBoundary(ctx context.Context, after time.Time) (time.Time, error) {
	if err := s.migrateLegacyAnnouncement(ctx); err != nil {
		return time.Time{}, err
	}
	announcements, err := s.db.Announcement.Query().
		Where(announcement.EnabledEQ(true)).
		Select(announcement.FieldStartsAt, announcement.FieldEndsAt).
		All(ctx)
	if err != nil {
		return time.Time{}, err
	}
	ads, err := s.db.Ad.Query().
		Where(ad.EnabledEQ(true)).
		Select(ad.FieldStartsAt, ad.FieldEndsAt).
		All(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var next time.Time
	consider := func(value *time.Time) {
		if value == nil || !value.After(after) {
			return
		}
		if next.IsZero() || value.Before(next) {
			next = value.UTC()
		}
	}
	for _, item := range announcements {
		consider(item.StartsAt)
		consider(item.EndsAt)
	}
	for _, item := range ads {
		consider(item.StartsAt)
		consider(item.EndsAt)
	}
	return next, nil
}

func (s *Server) feedImageURL(ctx context.Context, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "data:") {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		return s.absoluteURL(ctx, raw)
	}
	return raw
}

func siteAnnouncementToFeed(item siteAnnouncement) feed.AnnouncementMeta {
	return feed.AnnouncementMeta{
		ID:        item.ID,
		Enabled:   item.Enabled,
		Level:     item.Level,
		Title:     item.Title,
		Body:      item.Body,
		LinkLabel: item.LinkLabel,
		LinkURL:   item.LinkURL,
		StartsAt:  item.StartsAt,
		EndsAt:    item.EndsAt,
		SortOrder: item.SortOrder,
		UpdatedAt: item.UpdatedAt,
	}
}

func siteAnnouncementsToFeed(items []siteAnnouncement) []feed.AnnouncementMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]feed.AnnouncementMeta, 0, len(items))
	for _, item := range items {
		out = append(out, siteAnnouncementToFeed(item))
	}
	return out
}

func siteAdToFeed(item siteAd) feed.AdMeta {
	return feed.AdMeta{
		ID:        item.ID,
		Enabled:   item.Enabled,
		Title:     item.Title,
		Body:      item.Body,
		ImageURL:  item.ImageURL,
		LinkLabel: item.LinkLabel,
		LinkURL:   item.LinkURL,
		StartsAt:  item.StartsAt,
		EndsAt:    item.EndsAt,
		SortOrder: item.SortOrder,
		UpdatedAt: item.UpdatedAt,
	}
}

func siteAdsToFeed(items []siteAd) []feed.AdMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]feed.AdMeta, 0, len(items))
	for _, item := range items {
		out = append(out, siteAdToFeed(item))
	}
	return out
}

type sourceIndexPreload struct {
	publicAppIDs  map[int]struct{}
	privateAppIDs map[int]struct{}
	submitters    map[int]string
	categories    map[int]*entgo.Category
	tags          map[int][]string
	screenshots   map[int][]catalogmeta.Screenshot
	versions      map[int][]feed.VersionInput
	outdatedMarks map[int]int
}

func (s *Server) sourceIndexPreload(ctx context.Context, apps []*entgo.App, groupIDs []int) (sourceIndexPreload, error) {
	data := sourceIndexPreload{
		publicAppIDs:  make(map[int]struct{}, len(apps)),
		privateAppIDs: map[int]struct{}{},
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
	validGroupIDs := make(map[int]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		validGroupIDs[id] = struct{}{}
	}
	for _, record := range visibilityRecords {
		privateAppIDs[record.AppID] = struct{}{}
		data.privateAppIDs[record.AppID] = struct{}{}
		if _, ok := validGroupIDs[record.GroupID]; ok {
			data.publicAppIDs[record.AppID] = struct{}{}
		}
	}

	publicAppIDs := make([]int, 0, len(apps))
	ownerIDs := map[int]struct{}{}
	categoryIDs := map[int]struct{}{}
	for _, record := range apps {
		if _, private := privateAppIDs[record.ID]; private {
			if _, allowed := data.publicAppIDs[record.ID]; !allowed {
				continue
			}
		} else {
			data.publicAppIDs[record.ID] = struct{}{}
		}
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
	if err := s.loadSourceIndexVersions(ctx, publicAppIDs, data.privateAppIDs, data.versions); err != nil {
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
	pending := mapKeys(categoryIDs)
	for len(pending) > 0 {
		records, err := s.db.Category.Query().Where(category.IDIn(pending...)).All(ctx)
		if err != nil {
			return err
		}
		next := []int{}
		for _, record := range records {
			out[record.ID] = record
			if record.ParentID != nil {
				if _, exists := out[*record.ParentID]; !exists {
					next = append(next, *record.ParentID)
				}
			}
		}
		pending = next
	}
	return nil
}

func sourceIndexCategoryInputs(records map[int]*entgo.Category) []feed.CategoryInput {
	out := make([]feed.CategoryInput, 0, len(records))
	for _, record := range records {
		out = append(out, feed.CategoryInput{
			ID:        record.ID,
			Name:      record.Name,
			NameI18n:  catalogmeta.DecodeLocalizedText(record.NameI18n),
			Slug:      record.Slug,
			ParentID:  record.ParentID,
			SortOrder: record.SortOrder,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.ID < right.ID
	})
	return out
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

func (s *Server) loadSourceIndexVersions(ctx context.Context, appIDs []int, privateAppIDs map[int]struct{}, out map[int][]feed.VersionInput) error {
	records, err := s.db.AppVersion.Query().
		Where(appversion.AppIDIn(appIDs...), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(entgo.Asc(appversion.FieldAppID), entgo.Desc(appversion.FieldPublishedAt), entgo.Desc(appversion.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		upstreamDownloadURL := record.DownloadURL
		if _, private := privateAppIDs[record.AppID]; private {
			upstreamDownloadURL = ""
		}
		out[record.AppID] = append(out[record.AppID], feed.VersionInput{
			Version:             record.Version,
			Status:              string(record.Status),
			Changelog:           record.Changelog,
			SourceType:          string(record.SourceType),
			DownloadURL:         s.absoluteURL(ctx, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.AppID, record.ID)),
			UpstreamDownloadURL: upstreamDownloadURL,
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
