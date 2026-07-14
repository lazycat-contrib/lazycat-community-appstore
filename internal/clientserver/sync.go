package clientserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/catalogmeta"
	"lazycat.community/appstore/internal/mirror"

	"golang.org/x/sync/errgroup"
)

type sourceSyncError struct {
	code    string
	status  int
	message string
}

func (e sourceSyncError) Error() string {
	return e.message
}

type feedApp struct {
	ID               int                      `json:"id"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	NameI18n         map[string]string        `json:"nameI18n"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	SummaryI18n      map[string]string        `json:"summaryI18n"`
	DescriptionI18n  map[string]string        `json:"descriptionI18n"`
	Author           string                   `json:"author"`
	Homepage         string                   `json:"homepage"`
	License          string                   `json:"license"`
	MinOSVersion     string                   `json:"minOSVersion"`
	CategoryID       *int                     `json:"categoryId"`
	Category         string                   `json:"category"`
	CategoryI18n     map[string]string        `json:"categoryI18n"`
	IconURL          string                   `json:"iconUrl"`
	IconOriginURL    string                   `json:"-"`
	InstallProtected bool                     `json:"installProtected"`
	CommentsEnabled  *bool                    `json:"commentsEnabled"`
	OutdatedMarks    int                      `json:"outdatedMarks"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots"`
	LatestVersion    *VersionDTO              `json:"latestVersion"`
	Versions         []VersionDTO             `json:"versions"`
}

type feedIndex struct {
	Schema            string                  `json:"schema"`
	GitHubMirrors     []mirror.Entry          `json:"githubMirrors"`
	Site              feedSiteMeta            `json:"site"`
	Announcement      SourceAnnouncementDTO   `json:"announcement"`
	Announcements     []SourceAnnouncementDTO `json:"announcements"`
	Ads               []SourceAdDTO           `json:"ads"`
	Categories        []SourceCategoryDTO     `json:"categories"`
	Groups            []SourceGroupDTO        `json:"groups"`
	InvalidGroupCodes []string                `json:"invalidGroupCodes"`
	Apps              json.RawMessage         `json:"apps"`
}

type feedSiteMeta struct {
	ClientPolicy SourceClientPolicyDTO `json:"clientPolicy"`
	Chat         feedChatMeta          `json:"chat"`
}

type feedChatMeta struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSyncSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	source, err := s.syncSource(r.Context(), id, currentUserID(r))
	if err != nil {
		var syncErr sourceSyncError
		if errors.As(err, &syncErr) {
			writeError(w, syncErr.status, "SOURCE_SYNC_FAILED", syncErr.message)
			return
		}
		writeError(w, http.StatusInternalServerError, "SOURCE_SYNC_FAILED", "Could not sync source")
		return
	}
	writeJSON(w, http.StatusOK, sourceSyncResponse{Source: source})
}

func (s *Server) handleSyncAllSources(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	result, err := s.syncAllSources(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_LIST_FAILED", "Could not list sources")
		return
	}
	writeJSON(w, http.StatusOK, syncAllResponse{Result: result})
}

type sourceSyncResponse struct {
	Source SourceDTO `json:"source"`
}

type syncAllResponse struct {
	Result SyncAllResult `json:"result"`
}

func (s *Server) syncAllSources(ctx context.Context, userID string) (SyncAllResult, error) {
	sources, err := s.db.ClientSource.Query().Where(clientsource.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return SyncAllResult{}, err
	}
	results := make([]fetchedSource, len(sources))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for index, source := range sources {
		index := index
		source := source
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
			fetch, err := s.fetchSourceApps(groupCtx, source)
			if err != nil && groupCtx.Err() != nil {
				return groupCtx.Err()
			}
			results[index] = fetchedSource{source: source, sourceFeedFetch: fetch, err: err}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return SyncAllResult{}, err
	}

	result := SyncAllResult{}
	for _, item := range results {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if item.err != nil {
			_ = s.recordSourceSyncFailure(ctx, item.source.ID, item.err)
			result.Failed++
			continue
		}
		if item.notModified {
			if _, err := s.recordSourceNotModified(ctx, item.source, item.etag); err != nil {
				result.Failed++
			} else {
				result.Success++
			}
			continue
		}
		if _, err := s.saveSourceApps(ctx, item.source, item.apps, item.mirrors, item.categories, item.announcements, item.ads, item.clientPolicy, item.groups, item.invalidCodes, item.chatAvailable, item.etag); err != nil {
			result.Failed++
		} else {
			result.Success++
		}
	}
	return result, nil
}

type fetchedSource struct {
	source *ent.ClientSource
	sourceFeedFetch
	err error
}

type sourceFeedFetch struct {
	apps          []feedApp
	mirrors       []mirror.Entry
	categories    []SourceCategoryDTO
	announcements []SourceAnnouncementDTO
	ads           []SourceAdDTO
	clientPolicy  SourceClientPolicyDTO
	groups        []SourceGroupDTO
	invalidCodes  []string
	chatAvailable bool
	etag          string
	notModified   bool
}

const sourceAppInsertBatchSize = 50

func (s *Server) syncSource(ctx context.Context, sourceID int, userID string) (SourceDTO, error) {
	source, err := s.db.ClientSource.Query().
		Where(clientsource.IDEQ(sourceID), clientsource.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return SourceDTO{}, sourceSyncError{code: "not_found", status: http.StatusNotFound, message: "Source not found"}
		}
		return SourceDTO{}, err
	}
	fetch, err := s.fetchSourceApps(ctx, source)
	if err != nil {
		return SourceDTO{}, s.recordSourceSyncFailure(ctx, source.ID, err)
	}
	if fetch.notModified {
		return s.recordSourceNotModified(ctx, source, fetch.etag)
	}
	return s.saveSourceApps(ctx, source, fetch.apps, fetch.mirrors, fetch.categories, fetch.announcements, fetch.ads, fetch.clientPolicy, fetch.groups, fetch.invalidCodes, fetch.chatAvailable, fetch.etag)
}

func (s *Server) recordSourceSyncFailure(ctx context.Context, sourceID int, err error) sourceSyncError {
	syncErr := normalizeSourceSyncError(err)
	_, _ = s.db.ClientSource.UpdateOneID(sourceID).
		SetLastError(syncErr.message).
		SetLastErrorCode(clientsource.LastErrorCode(syncErr.code)).
		Save(ctx)
	return syncErr
}

func normalizeSourceSyncError(err error) sourceSyncError {
	var syncErr sourceSyncError
	if !errors.As(err, &syncErr) {
		syncErr = sourceSyncError{code: "network", status: http.StatusBadGateway, message: err.Error()}
	}
	return syncErr
}

func (s *Server) recordSourceNotModified(ctx context.Context, source *ent.ClientSource, etag string) (SourceDTO, error) {
	update := s.db.ClientSource.UpdateOneID(source.ID).
		SetLastSync(time.Now()).
		ClearLastError().
		ClearLastErrorCode()
	if strings.TrimSpace(etag) != "" {
		update.SetLastEtag(strings.TrimSpace(etag))
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return SourceDTO{}, err
	}
	return sourceDTO(updated), nil
}

func (s *Server) saveSourceApps(ctx context.Context, source *ent.ClientSource, apps []feedApp, mirrors []mirror.Entry, categories []SourceCategoryDTO, announcements []SourceAnnouncementDTO, ads []SourceAdDTO, clientPolicy SourceClientPolicyDTO, groups []SourceGroupDTO, invalidCodes []string, chatAvailable bool, etag string) (SourceDTO, error) {
	oldAppRecords, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.SourceIDEQ(source.ID)).
		All(ctx)
	if err != nil {
		return SourceDTO{}, err
	}
	oldAppIDs := make([]int, 0, len(oldAppRecords))
	for _, record := range oldAppRecords {
		oldAppIDs = append(oldAppIDs, record.ID)
	}
	if err := s.materializeSourceIcons(ctx, source, apps, oldAppRecords); err != nil {
		return SourceDTO{}, err
	}
	installableCount := 0
	rows := make([]sourceAppCacheRow, 0, len(apps))
	categories = normalizeSourceCategories(categories)
	announcements = normalizeSourceAnnouncements(announcements)
	ads = normalizeSourceAds(ads)
	clientPolicy = normalizeSourceClientPolicy(clientPolicy)
	categoryIDs := sourceCategoryIDs(categories)
	for i := range apps {
		if apps[i].LatestVersion != nil && apps[i].LatestVersion.DownloadURL != "" {
			installableCount++
		}
		if apps[i].CategoryID != nil {
			if _, ok := categoryIDs[*apps[i].CategoryID]; !ok {
				apps[i].CategoryID = nil
			}
		}
		row, err := buildSourceAppCacheRow(apps[i])
		if err != nil {
			return SourceDTO{}, err
		}
		rows = append(rows, row)
	}
	mirrorsJSON := ""
	if len(mirrors) > 0 {
		encoded, err := json.Marshal(mirrors)
		if err != nil {
			return SourceDTO{}, err
		}
		mirrorsJSON = string(encoded)
	}
	defaultDownloadMirrorID := source.DefaultDownloadMirrorID
	if defaultDownloadMirrorID != "" {
		if entry, ok := mirror.Find(mirrors, defaultDownloadMirrorID); !ok || entry.Kind != mirror.KindDownload {
			defaultDownloadMirrorID = ""
		}
	}
	defaultRawMirrorID := source.DefaultRawMirrorID
	if defaultRawMirrorID != "" {
		if entry, ok := mirror.Find(mirrors, defaultRawMirrorID); !ok || entry.Kind != mirror.KindRaw {
			defaultRawMirrorID = ""
		}
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return SourceDTO{}, err
	}
	if _, err := tx.ClientSourceApp.Delete().Where(clientsourceapp.SourceIDEQ(source.ID)).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return SourceDTO{}, err
	}
	builders := make([]*ent.ClientSourceAppCreate, 0, min(len(rows), sourceAppInsertBatchSize))
	for i := range rows {
		builders = append(builders, sourceAppCreateBuilder(tx, source.ID, rows[i]))
		if len(builders) < sourceAppInsertBatchSize {
			continue
		}
		if err := tx.ClientSourceApp.CreateBulk(builders...).Exec(ctx); err != nil {
			_ = tx.Rollback()
			return SourceDTO{}, err
		}
		builders = builders[:0]
	}
	if len(builders) > 0 {
		if err := tx.ClientSourceApp.CreateBulk(builders...).Exec(ctx); err != nil {
			_ = tx.Rollback()
			return SourceDTO{}, err
		}
	}
	now := time.Now()
	groupCodes := removeInvalidGroupCodes(decodeStringSlice(source.GroupCodesJSON), invalidCodes)
	updated, err := tx.ClientSource.UpdateOneID(source.ID).
		SetLastSync(now).
		ClearLastError().
		ClearLastErrorCode().
		SetLastAppCount(len(apps)).
		SetLastInstallableCount(installableCount).
		SetLastEtag(strings.TrimSpace(etag)).
		SetGroupCodesJSON(encodeStringSlice(groupCodes)).
		SetGroupNamesJSON(encodeSourceGroups(groups)).
		SetLastInvalidGroupCodesJSON(encodeStringSlice(invalidCodes)).
		SetMirrorsJSON(mirrorsJSON).
		SetCategoriesJSON(encodeSourceCategories(categories)).
		SetAnnouncementsJSON(encodeSourceAnnouncements(announcements)).
		SetAdsJSON(encodeSourceAds(ads)).
		SetMinClientVersion(clientPolicy.MinVersion).
		SetMinClientVersionMessage(clientPolicy.Message).
		SetChatAvailable(chatAvailable).
		SetDefaultDownloadMirrorID(defaultDownloadMirrorID).
		SetDefaultRawMirrorID(defaultRawMirrorID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return SourceDTO{}, err
	}
	if err := tx.Commit(); err != nil {
		return SourceDTO{}, err
	}
	if err := s.linkClientSourceAppIconAssets(ctx, updated.ID); err != nil {
		return SourceDTO{}, err
	}
	if err := s.deleteClientAssetLinksForOwnerIDs(ctx, clientAssetOwnerSourceApp, oldAppIDs); err != nil {
		return SourceDTO{}, err
	}
	return sourceDTO(updated), nil
}

type sourceAppCacheRow struct {
	ExternalID          string
	PackageID           string
	Name                string
	NameI18nJSON        string
	Slug                string
	Summary             string
	SummaryI18nJSON     string
	DescriptionI18nJSON string
	Author              string
	Homepage            string
	License             string
	MinOSVersion        string
	CategoryID          *int
	Category            string
	CategoryI18nJSON    string
	IconURL             string
	IconOriginURL       string
	InstallProtected    bool
	CommentsEnabled     bool
	OutdatedMarks       int
	ScreenshotsJSON     string
	LatestVersionJSON   string
	VersionsJSON        string
}

func buildSourceAppCacheRow(app feedApp) (sourceAppCacheRow, error) {
	versionJSON := ""
	versions := app.Versions
	if len(versions) == 0 && app.LatestVersion != nil {
		versions = []VersionDTO{*app.LatestVersion}
	}
	versionsJSON := ""
	screenshotsJSON := catalogmeta.EncodeScreenshots(app.Screenshots)
	if app.LatestVersion != nil {
		encoded, err := json.Marshal(app.LatestVersion)
		if err != nil {
			return sourceAppCacheRow{}, err
		}
		versionJSON = string(encoded)
	}
	if len(versions) > 0 {
		encoded, err := json.Marshal(versions)
		if err != nil {
			return sourceAppCacheRow{}, err
		}
		versionsJSON = string(encoded)
	}
	commentsEnabled := true
	if app.CommentsEnabled != nil {
		commentsEnabled = *app.CommentsEnabled
	}
	return sourceAppCacheRow{
		ExternalID:          strconv.Itoa(app.ID),
		PackageID:           app.PackageID,
		Name:                app.Name,
		NameI18nJSON:        catalogmeta.EncodeLocalizedText(app.NameI18n),
		Slug:                app.Slug,
		Summary:             app.Summary,
		SummaryI18nJSON:     catalogmeta.EncodeLocalizedText(app.SummaryI18n),
		DescriptionI18nJSON: catalogmeta.EncodeLocalizedText(app.DescriptionI18n),
		Author:              strings.TrimSpace(app.Author),
		Homepage:            strings.TrimSpace(app.Homepage),
		License:             strings.TrimSpace(app.License),
		MinOSVersion:        strings.TrimSpace(app.MinOSVersion),
		CategoryID:          app.CategoryID,
		Category:            app.Category,
		CategoryI18nJSON:    catalogmeta.EncodeLocalizedText(app.CategoryI18n),
		IconURL:             app.IconURL,
		IconOriginURL:       app.IconOriginURL,
		InstallProtected:    app.InstallProtected,
		CommentsEnabled:     commentsEnabled,
		OutdatedMarks:       app.OutdatedMarks,
		ScreenshotsJSON:     screenshotsJSON,
		LatestVersionJSON:   versionJSON,
		VersionsJSON:        versionsJSON,
	}, nil
}

func sourceAppCreateBuilder(tx *ent.Tx, sourceID int, row sourceAppCacheRow) *ent.ClientSourceAppCreate {
	return tx.ClientSourceApp.Create().
		SetSourceID(sourceID).
		SetExternalID(row.ExternalID).
		SetPackageID(row.PackageID).
		SetName(row.Name).
		SetNameI18nJSON(row.NameI18nJSON).
		SetSlug(row.Slug).
		SetSummary(row.Summary).
		SetSummaryI18nJSON(row.SummaryI18nJSON).
		SetDescriptionI18nJSON(row.DescriptionI18nJSON).
		SetAuthor(row.Author).
		SetHomepage(row.Homepage).
		SetLicense(row.License).
		SetMinOsVersion(row.MinOSVersion).
		SetNillableCategoryID(row.CategoryID).
		SetCategory(row.Category).
		SetCategoryI18nJSON(row.CategoryI18nJSON).
		SetIconURL(row.IconURL).
		SetIconOriginURL(row.IconOriginURL).
		SetInstallProtected(row.InstallProtected).
		SetCommentsEnabled(row.CommentsEnabled).
		SetOutdatedMarks(row.OutdatedMarks).
		SetScreenshotsJSON(row.ScreenshotsJSON).
		SetLatestVersionJSON(row.LatestVersionJSON).
		SetVersionsJSON(row.VersionsJSON)
}

func (s *Server) fetchSourceApps(ctx context.Context, source *ent.ClientSource) (sourceFeedFetch, error) {
	s.ensureHTTPClients()
	timeout := s.cfg.SyncTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	feedURL, err := url.Parse(source.URL)
	if err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	if source.Password != "" {
		q := feedURL.Query()
		q.Set("password", source.Password)
		feedURL.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL.String(), nil)
	if err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	req.Header.Set("Accept-Encoding", "br, gzip")
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	if codes := decodeStringSlice(source.GroupCodesJSON); len(codes) > 0 {
		req.Header.Set("X-Group-Codes", strings.Join(codes, ","))
	}
	if strings.TrimSpace(source.LastEtag) != "" {
		if count, countErr := s.db.ClientSourceApp.Query().Where(clientsourceapp.SourceIDEQ(source.ID)).Count(ctx); countErr == nil && count == source.LastAppCount {
			req.Header.Set("If-None-Match", source.LastEtag)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "network", status: http.StatusBadGateway, message: "Could not reach source"}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return sourceFeedFetch{}, sourceSyncError{code: "auth", status: http.StatusUnauthorized, message: "Source password is invalid"}
	}
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	if resp.StatusCode == http.StatusNotModified {
		if etag == "" {
			etag = source.LastEtag
		}
		return sourceFeedFetch{etag: etag, notModified: true}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return sourceFeedFetch{}, sourceSyncError{code: "http", status: http.StatusBadGateway, message: fmt.Sprintf("Source returned HTTP %d", resp.StatusCode)}
	}
	body, err := sourceResponseBody(resp)
	if err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed uses an unsupported content encoding"}
	}
	defer func() { _ = body.Close() }()
	var root feedIndex
	if err := decodeLimitedJSON(body, maxSourceFeedBytes, &root); err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed is not valid JSON"}
	}
	if len(root.Apps) == 0 || strings.TrimSpace(string(root.Apps)) == "null" {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	mirrors, err := normalizeFeedMirrors(root.GitHubMirrors)
	if err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: err.Error()}
	}
	var apps []feedApp
	if err := json.Unmarshal(root.Apps, &apps); err != nil {
		return sourceFeedFetch{}, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	categories := normalizeSourceCategories(root.Categories)
	announcements := normalizeSourceAnnouncements(root.Announcements)
	if len(announcements) == 0 {
		announcements = normalizeSourceAnnouncements([]SourceAnnouncementDTO{root.Announcement})
	}
	ads := normalizeSourceAds(root.Ads)
	categoryIDs := sourceCategoryIDs(categories)
	out := make([]feedApp, 0, len(apps))
	for _, app := range apps {
		app.PackageID = strings.TrimSpace(app.PackageID)
		app.Name = strings.TrimSpace(app.Name)
		app.Slug = strings.TrimSpace(app.Slug)
		app.IconURL = strings.TrimSpace(app.IconURL)
		if app.PackageID == "" || app.Name == "" || app.Slug == "" {
			continue
		}
		if app.CategoryID != nil {
			if _, ok := categoryIDs[*app.CategoryID]; !ok {
				app.CategoryID = nil
			}
		}
		out = append(out, app)
	}
	return sourceFeedFetch{
		apps:          out,
		mirrors:       mirrors,
		categories:    categories,
		announcements: announcements,
		ads:           ads,
		clientPolicy:  normalizeSourceClientPolicy(root.Site.ClientPolicy),
		groups:        root.Groups,
		invalidCodes:  normalizeGroupCodes(root.InvalidGroupCodes),
		chatAvailable: root.Site.Chat.Enabled,
		etag:          etag,
	}, nil
}

func normalizeFeedMirrors(input []mirror.Entry) ([]mirror.Entry, error) {
	if len(input) == 0 {
		return nil, nil
	}
	linesByKind := map[string][]string{}
	for _, entry := range input {
		kind := mirror.CleanKind(entry.Kind)
		if kind == "" {
			return nil, fmt.Errorf("source feed mirror %q has an invalid kind", strings.TrimSpace(entry.Name))
		}
		linesByKind[kind] = append(linesByKind[kind], strings.TrimSpace(entry.Name)+"=>"+strings.TrimSpace(entry.URL))
	}
	out := []mirror.Entry{}
	for _, kind := range []string{mirror.KindDownload, mirror.KindRaw} {
		if len(linesByKind[kind]) == 0 {
			continue
		}
		entries, err := mirror.Parse(strings.Join(linesByKind[kind], "\n"), kind)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	return out, nil
}
