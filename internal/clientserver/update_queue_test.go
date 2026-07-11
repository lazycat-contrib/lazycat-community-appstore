package clientserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsyncsetting"
)

func TestEligibleUpdatesSkipsProtectedCurrentAndUnknownApps(t *testing.T) {
	apps := sourceAppsForUpdateTest(t,
		updateTestSourceApp{PackageID: "eligible", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "protected", Version: "2.0.0", InstallProtected: true},
		updateTestSourceApp{PackageID: "current", Version: "2.0.0"},
	)
	candidates := eligibleUpdates([]InstalledApplicationDTO{
		{AppID: "eligible", Version: "1.0.0"},
		{AppID: "protected", Version: "1.0.0"},
		{AppID: "current", Version: "2.0.0"},
		{AppID: "unknown", Version: "1.0.0"},
	}, apps)
	if len(candidates) != 1 || candidates[0].PackageID != "eligible" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestUpdateQueueContinuesAfterFailure(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db,
		updateTestSourceApp{PackageID: "first", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "second", Version: "2.0.0"},
	)
	app.server.pkg = &updateQueuePackageManager{
		installed:   []InstalledApplicationDTO{{AppID: "first", Version: "1.0.0"}, {AppID: "second", Version: "1.0.0"}},
		installErrs: []error{errors.New("first failed"), nil},
		install:     []InstallResultDTO{{TaskID: "second-task", Status: "CREATING"}},
		tasks:       map[string]InstallTaskDTO{"second-task": {TaskID: "second-task", Status: "INSTALL_OK"}},
	}
	result := app.server.RunUpdateQueue(t.Context(), "alice")
	if len(result.Items) != 2 || result.Items[0].Status != "failed" || result.Items[1].Status != "success" {
		t.Fatalf("items = %#v", result.Items)
	}
}

func TestUpdateQueueRejectsConcurrentUserRun(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "notes", Version: "2.0.0"})
	started := make(chan struct{})
	release := make(chan struct{})
	app.server.pkg = &updateQueuePackageManager{
		installed:      []InstalledApplicationDTO{{AppID: "notes", Version: "1.0.0"}},
		install:        []InstallResultDTO{{TaskID: "task-1", Status: "INSTALL_OK"}},
		blockInstallAt: 1,
		installBlocked: started,
		releaseInstall: release,
	}
	done := make(chan UpdateQueueResultDTO, 1)
	go func() { done <- app.server.RunUpdateQueue(context.Background(), "alice") }()
	<-started
	if snapshot, ok := app.server.installCoordinator.queueSnapshot("alice"); !ok || len(snapshot.Items) != 1 || snapshot.Items[0].Status != "running" {
		t.Fatalf("snapshot = %#v, ok = %v", snapshot, ok)
	}
	if result := app.server.RunUpdateQueue(t.Context(), "alice"); result.Status != "already_running" {
		t.Fatalf("status = %q", result.Status)
	}
	close(release)
	<-done
}

func TestManualInstallRejectsActiveUpdateQueue(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "notes", Version: "2.0.0"})
	started := make(chan struct{})
	release := make(chan struct{})
	app.server.pkg = &updateQueuePackageManager{
		installed:      []InstalledApplicationDTO{{AppID: "notes", Version: "1.0.0"}},
		install:        []InstallResultDTO{{TaskID: "task-1", Status: "INSTALL_OK"}},
		blockInstallAt: 1,
		installBlocked: started,
		releaseInstall: release,
	}
	done := make(chan UpdateQueueResultDTO, 1)
	go func() { done <- app.server.RunUpdateQueue(context.Background(), "alice") }()
	<-started
	rec := app.request(http.MethodPost, "/api/client/v1/install", `{"appId":1}`, "alice")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"INSTALL_IN_PROGRESS"`) {
		t.Fatalf("manual install = %d %s", rec.Code, rec.Body.String())
	}
	close(release)
	<-done
}

func TestClientSettingsPersistAutoUpdate(t *testing.T) {
	app := testServer(t)
	rec := app.request(http.MethodPatch, "/api/client/v1/settings", `{"autoUpdateEnabled":true,"autoUpdateIntervalMinutes":1}`, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"autoUpdateEnabled":true`) || !strings.Contains(rec.Body.String(), `"autoUpdateIntervalMinutes":5`) {
		t.Fatalf("settings = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAutoUpdateDueUsesLastRunAndInterval(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	oldRun := now.Add(-time.Hour)
	recentRun := now.Add(-time.Minute)
	if !autoUpdateDue(&ent.ClientSyncSetting{AutoUpdateEnabled: true, AutoUpdateIntervalMinutes: 60, LastAutoUpdateAt: &oldRun}, now) {
		t.Fatal("old update was not due")
	}
	if autoUpdateDue(&ent.ClientSyncSetting{AutoUpdateEnabled: true, AutoUpdateIntervalMinutes: 60, LastAutoUpdateAt: &recentRun}, now) {
		t.Fatal("recent update was due")
	}
}

func TestRunUpdateQueueSyncsSourcesBeforeInstallation(t *testing.T) {
	feed := updateQueueFeed(t, "notes", "2.0.0")
	defer feed.Close()
	app := testServer(t)
	app.server.db.ClientSource.Create().SetUserID("alice").SetName("Feed").SetURL(feed.URL).SaveX(t.Context())
	pm := &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "notes", Version: "1.0.0"}},
		install:   []InstallResultDTO{{TaskID: "task-1", Status: "CREATING"}},
		tasks:     map[string]InstallTaskDTO{"task-1": {TaskID: "task-1", Status: "INSTALL_OK"}},
	}
	app.server.pkg = pm
	rec := app.request(http.MethodPost, "/api/client/v1/updates/run", "", "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"success"`) {
		t.Fatalf("run updates = %d %s", rec.Code, rec.Body.String())
	}
	if len(pm.install) != 0 {
		t.Fatalf("source sync did not make the update installable")
	}
}

func TestAutoUpdateSchedulerSyncsSourcesBeforeQueue(t *testing.T) {
	feed := updateQueueFeed(t, "notes", "2.0.0")
	defer feed.Close()
	app := testServer(t)
	app.server.db.ClientSource.Create().SetUserID("alice").SetName("Feed").SetURL(feed.URL).SaveX(t.Context())
	app.server.db.ClientSyncSetting.Create().
		SetUserID("alice").
		SetAutoUpdateEnabled(true).
		SetAutoUpdateIntervalMinutes(5).
		SaveX(t.Context())
	pm := &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "notes", Version: "1.0.0"}},
		install:   []InstallResultDTO{{TaskID: "task-1", Status: "CREATING"}},
		tasks:     map[string]InstallTaskDTO{"task-1": {TaskID: "task-1", Status: "INSTALL_OK"}},
	}
	app.server.pkg = pm
	scheduler := &sourceSyncScheduler{server: app.server, running: make(map[string]struct{})}
	if err := scheduler.runDueAutoUpdates(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	setting := app.server.db.ClientSyncSetting.Query().Where(clientsyncsetting.UserIDEQ("alice")).OnlyX(t.Context())
	if setting.LastAutoUpdateAt == nil || setting.LastAutoUpdateStatus == nil || setting.LastAutoUpdateStatus.String() != "success" {
		t.Fatalf("auto update result = %#v", setting)
	}
	if len(pm.install) != 0 {
		t.Fatalf("scheduled update did not install the synced app")
	}
}

func updateQueueFeed(t *testing.T, packageID, version string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"schema": "lazycat.appstore.source.v2",
			"apps": []map[string]any{{
				"id":        1,
				"packageId": packageID,
				"name":      packageID,
				"slug":      packageID,
				"latestVersion": map[string]any{
					"version":     version,
					"downloadUrl": "https://download.example/" + packageID + ".lpk",
					"sha256":      "checksum",
				},
			}},
		})
	}))
}

type updateTestSourceApp struct {
	PackageID        string
	Version          string
	InstallProtected bool
}

func sourceAppsForUpdateTest(t *testing.T, values ...updateTestSourceApp) []*ent.ClientSourceApp {
	t.Helper()
	client := testClient(t)
	t.Cleanup(func() { _ = client.Close() })
	return sourceAppsForUpdateTestOnClient(t, client, values...)
}

func sourceAppsForUpdateTestOnClient(t *testing.T, client *ent.Client, values ...updateTestSourceApp) []*ent.ClientSourceApp {
	t.Helper()
	source := client.ClientSource.Create().SetUserID("alice").SetName("Feed").SetURL("https://feed.example/index.json").SaveX(t.Context())
	apps := make([]*ent.ClientSourceApp, 0, len(values))
	for index, value := range values {
		latest, err := json.Marshal(VersionDTO{Version: value.Version, DownloadURL: "https://download.example/" + value.PackageID + ".lpk", SHA256: "checksum"})
		if err != nil {
			t.Fatal(err)
		}
		apps = append(apps, client.ClientSourceApp.Create().
			SetSourceID(source.ID).
			SetExternalID(string(rune('1'+index))).
			SetPackageID(value.PackageID).
			SetName(value.PackageID).
			SetSlug(value.PackageID).
			SetInstallProtected(value.InstallProtected).
			SetLatestVersionJSON(string(latest)).
			SaveX(t.Context()))
	}
	return apps
}

type updateQueuePackageManager struct {
	installed []InstalledApplicationDTO

	mu              sync.Mutex
	installErrs     []error
	install         []InstallResultDTO
	tasks           map[string]InstallTaskDTO
	cancelledTaskID string
	started         chan struct{}
	release         chan struct{}
	installCalls    int
	blockInstallAt  int
	installBlocked  chan struct{}
	releaseInstall  chan struct{}
	cancelErrors    map[string]error
}

func (f *updateQueuePackageManager) QueryInstalled(context.Context, string) ([]InstalledApplicationDTO, error) {
	return f.installed, nil
}

func (f *updateQueuePackageManager) InstallLPK(_ context.Context, _ string, _ InstallRequestDTO) (InstallResultDTO, error) {
	f.mu.Lock()
	f.installCalls++
	shouldBlock := f.installCalls == f.blockInstallAt && f.installBlocked != nil && f.releaseInstall != nil
	if shouldBlock {
		close(f.installBlocked)
		f.mu.Unlock()
		<-f.releaseInstall
		f.mu.Lock()
	}
	defer f.mu.Unlock()
	if len(f.installErrs) > 0 {
		err := f.installErrs[0]
		f.installErrs = f.installErrs[1:]
		if err != nil {
			return InstallResultDTO{}, err
		}
	}
	if len(f.install) == 0 {
		return InstallResultDTO{}, errors.New("missing install result")
	}
	result := f.install[0]
	f.install = f.install[1:]
	return result, nil
}

func (f *updateQueuePackageManager) GetInstallTask(ctx context.Context, _ string, taskID string) (InstallTaskDTO, error) {
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return InstallTaskDTO{}, ctx.Err()
		}
	}
	task, ok := f.tasks[taskID]
	if !ok {
		return InstallTaskDTO{}, errors.New("task not found")
	}
	if f.release != nil {
		task.Status = "INSTALL_OK"
	}
	return task, nil
}

func (f *updateQueuePackageManager) CancelInstall(_ context.Context, _ string, taskID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelledTaskID = taskID
	if err := f.cancelErrors[taskID]; err != nil {
		return err
	}
	return nil
}

var _ PackageManager = (*updateQueuePackageManager)(nil)
