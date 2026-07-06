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
	ID               int          `json:"id"`
	PackageID        string       `json:"packageId"`
	Name             string       `json:"name"`
	Slug             string       `json:"slug"`
	Summary          string       `json:"summary"`
	Category         string       `json:"category"`
	InstallProtected bool         `json:"installProtected"`
	LatestVersion    *VersionDTO  `json:"latestVersion"`
	Versions         []VersionDTO `json:"versions"`
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
	sources, err := s.db.ClientSource.Query().Where(clientsource.UserIDEQ(userID)).All(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SOURCE_LIST_FAILED", "Could not list sources")
		return
	}
	result := SyncAllResult{}
	for _, source := range sources {
		if _, err := s.syncSource(r.Context(), source.ID, userID); err != nil {
			result.Failed++
		} else {
			result.Success++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
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
	apps, err := s.fetchSourceApps(ctx, source)
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
		if _, err := tx.ClientSourceApp.Create().
			SetSourceID(source.ID).
			SetExternalID(strconv.Itoa(app.ID)).
			SetPackageID(app.PackageID).
			SetName(app.Name).
			SetSlug(app.Slug).
			SetSummary(app.Summary).
			SetCategory(app.Category).
			SetInstallProtected(app.InstallProtected).
			SetLatestVersionJSON(versionJSON).
			SetVersionsJSON(versionsJSON).
			Save(ctx); err != nil {
			_ = tx.Rollback()
			return SourceDTO{}, err
		}
	}
	now := time.Now()
	updated, err := tx.ClientSource.UpdateOneID(source.ID).
		SetLastSync(now).
		ClearLastError().
		ClearLastErrorCode().
		SetLastAppCount(len(apps)).
		SetLastInstallableCount(installableCount).
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

func (s *Server) fetchSourceApps(ctx context.Context, source *ent.ClientSource) ([]feedApp, error) {
	timeout := s.cfg.SyncTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	feedURL, err := url.Parse(source.URL)
	if err != nil {
		return nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	if source.Password != "" {
		q := feedURL.Query()
		q.Set("password", source.Password)
		feedURL.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL.String(), nil)
	if err != nil {
		return nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Invalid source URL"}
	}
	if source.Password != "" {
		req.Header.Set("X-Source-Password", source.Password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, sourceSyncError{code: "network", status: http.StatusBadGateway, message: "Could not reach source"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, sourceSyncError{code: "auth", status: http.StatusUnauthorized, message: "Source password is invalid"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, sourceSyncError{code: "http", status: http.StatusBadGateway, message: fmt.Sprintf("Source returned HTTP %d", resp.StatusCode)}
	}
	var root struct {
		Apps json.RawMessage `json:"apps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed is not valid JSON"}
	}
	if len(root.Apps) == 0 || strings.TrimSpace(string(root.Apps)) == "null" {
		return nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	var apps []feedApp
	if err := json.Unmarshal(root.Apps, &apps); err != nil {
		return nil, sourceSyncError{code: "format", status: http.StatusUnprocessableEntity, message: "Source feed apps must be an array"}
	}
	out := make([]feedApp, 0, len(apps))
	for _, app := range apps {
		app.PackageID = strings.TrimSpace(app.PackageID)
		app.Name = strings.TrimSpace(app.Name)
		app.Slug = strings.TrimSpace(app.Slug)
		if app.PackageID == "" || app.Name == "" || app.Slug == "" {
			continue
		}
		if app.LatestVersion != nil {
			app.LatestVersion.DownloadURL = mirroredDownloadURL(source, app.LatestVersion, app.InstallProtected)
		}
		for i := range app.Versions {
			app.Versions[i].DownloadURL = mirroredDownloadURL(source, &app.Versions[i], app.InstallProtected)
		}
		out = append(out, app)
	}
	return out, nil
}

func mirroredDownloadURL(source *ent.ClientSource, version *VersionDTO, installProtected bool) string {
	upstream := strings.TrimSpace(version.UpstreamDownloadURL)
	direct := strings.TrimSpace(version.DownloadURL)
	githubURL := upstream
	if githubURL == "" {
		githubURL = direct
	}
	if !installProtected && source.Mirror != "" && (strings.Contains(githubURL, "github.com/") || strings.Contains(githubURL, "githubusercontent.com/")) {
		return strings.TrimRight(source.Mirror, "/") + "/" + githubURL
	}
	return direct
}
