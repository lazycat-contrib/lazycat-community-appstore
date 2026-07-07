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
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	Category         string                   `json:"category"`
	CategoryI18n     map[string]string        `json:"categoryI18n"`
	IconURL          string                   `json:"iconUrl"`
	InstallProtected bool                     `json:"installProtected"`
	CommentsEnabled  *bool                    `json:"commentsEnabled"`
	OutdatedMarks    int                      `json:"outdatedMarks"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots"`
	LatestVersion    *VersionDTO              `json:"latestVersion"`
	Versions         []VersionDTO             `json:"versions"`
}

type feedIndex struct {
	GitHubMirrors []mirror.Entry  `json:"githubMirrors"`
	Apps          json.RawMessage `json:"apps"`
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
	writeJSON(w, http.StatusOK, map[string]any{"source": source})
}

func (s *Server) handleSyncAllSources(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	result, err := s.syncAllSources(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_LIST_FAILED", "Could not list sources")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *Server) syncAllSources(ctx context.Context, userID string) (SyncAllResult, error) {
	sources, err := s.db.ClientSource.Query().Where(clientsource.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return SyncAllResult{}, err
	}
	result := SyncAllResult{}
	for _, source := range sources {
		if _, err := s.syncSource(ctx, source.ID, userID); err != nil {
			result.Failed++
		} else {
			result.Success++
		}
	}
	return result, nil
}

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
	apps, mirrors, err := s.fetchSourceApps(ctx, source)
	if err != nil {
		var syncErr sourceSyncError
		if !errors.As(err, &syncErr) {
			syncErr = sourceSyncError{code: "network", status: http.StatusBadGateway, message: err.Error()}
		}
		_, _ = s.db.ClientSource.UpdateOneID(source.ID).
			SetLastError(syncErr.message).
			SetLastErrorCode(clientsource.LastErrorCode(syncErr.code)).
			Save(ctx)
		return SourceDTO{}, syncErr
	}
	installableCount := 0
	for i := range apps {
		if apps[i].LatestVersion != nil && apps[i].LatestVersion.DownloadURL != "" {
			installableCount++
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
	for _, app := range apps {
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
				_ = tx.Rollback()
				return SourceDTO{}, err
			}
			versionJSON = string(encoded)
		}
		if len(versions) > 0 {
			encoded, err := json.Marshal(versions)
			if err != nil {
				_ = tx.Rollback()
				return SourceDTO{}, err
			}
			versionsJSON = string(encoded)
		}
		commentsEnabled := true
		if app.CommentsEnabled != nil {
			commentsEnabled = *app.CommentsEnabled
		}
		if _, err := tx.ClientSourceApp.Create().
			SetSourceID(source.ID).
			SetExternalID(strconv.Itoa(app.ID)).
			SetPackageID(app.PackageID).
			SetName(app.Name).
			SetSlug(app.Slug).
			SetSummary(app.Summary).
			SetCategory(app.Category).
			SetCategoryI18nJSON(catalogmeta.EncodeLocalizedText(app.CategoryI18n)).
			SetIconURL(app.IconURL).
			SetInstallProtected(app.InstallProtected).
			SetCommentsEnabled(commentsEnabled).
			SetOutdatedMarks(app.OutdatedMarks).
			SetScreenshotsJSON(screenshotsJSON).
			SetLatestVersionJSON(versionJSON).
			SetVersionsJSON(versionsJSON).
			Save(ctx); err != nil {
			_ = tx.Rollback()
			return SourceDTO{}, err
		}
	}
	now := time.Now()
	mirrorsJSON := ""
	if len(mirrors) > 0 {
		encoded, err := json.Marshal(mirrors)
		if err != nil {
			_ = tx.Rollback()
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
	updated, err := tx.ClientSource.UpdateOneID(source.ID).
		SetLastSync(now).
		ClearLastError().
		ClearLastErrorCode().
		SetLastAppCount(len(apps)).
		SetLastInstallableCount(installableCount).
		SetMirrorsJSON(mirrorsJSON).
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
	return sourceDTO(updated), nil
}

func (s *Server) fetchSourceApps(ctx context.Context, source *ent.ClientSource) ([]feedApp, []mirror.Entry, error) {
	timeout := s.cfg.SyncTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	feedURL, err := url.Parse(source.URL)
	if err != nil {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	if source.Password != "" {
		q := feedURL.Query()
		q.Set("password", source.Password)
		feedURL.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL.String(), nil)
	if err != nil {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, sourceSyncError{code: "network", status: http.StatusBadGateway, message: "Could not reach source"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, nil, sourceSyncError{code: "auth", status: http.StatusUnauthorized, message: "Source password is invalid"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, sourceSyncError{code: "http", status: http.StatusBadGateway, message: fmt.Sprintf("Source returned HTTP %d", resp.StatusCode)}
	}
	var root feedIndex
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed is not valid JSON"}
	}
	if len(root.Apps) == 0 || strings.TrimSpace(string(root.Apps)) == "null" {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	mirrors, err := normalizeFeedMirrors(root.GitHubMirrors)
	if err != nil {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: err.Error()}
	}
	var apps []feedApp
	if err := json.Unmarshal(root.Apps, &apps); err != nil {
		return nil, nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	out := make([]feedApp, 0, len(apps))
	for _, app := range apps {
		app.PackageID = strings.TrimSpace(app.PackageID)
		app.Name = strings.TrimSpace(app.Name)
		app.Slug = strings.TrimSpace(app.Slug)
		app.IconURL = strings.TrimSpace(app.IconURL)
		if app.PackageID == "" || app.Name == "" || app.Slug == "" {
			continue
		}
		out = append(out, app)
	}
	return out, mirrors, nil
}

func normalizeFeedMirrors(input []mirror.Entry) ([]mirror.Entry, error) {
	if len(input) == 0 {
		return nil, nil
	}
	linesByKind := map[string][]string{}
	for _, entry := range input {
		kind := mirror.CleanKind(entry.Kind)
		if kind == "" {
			return nil, fmt.Errorf("Source feed mirror %q has an invalid kind", strings.TrimSpace(entry.Name))
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
