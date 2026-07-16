package clientserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	candidates, passwordRequired := eligibleUpdates([]InstalledApplicationDTO{
		{AppID: "eligible", Version: "1.0.0"},
		{AppID: "protected", Version: "1.0.0"},
		{AppID: "current", Version: "2.0.0"},
		{AppID: "unknown", Version: "1.0.0"},
	}, apps, nil)
	if len(candidates) != 1 || candidates[0].PackageID != "eligible" {
		t.Fatalf("candidates = %#v", candidates)
	}
	if passwordRequired != 1 {
		t.Fatalf("password required = %d, want 1", passwordRequired)
	}
}

func TestUpdateQueueReportsPasswordRequiredWhenOnlyProtectedUpdatesExist(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "protected", Version: "2.0.0", InstallProtected: true})
	pm := &updateQueuePackageManager{installed: []InstalledApplicationDTO{{AppID: "protected", Version: "1.0.0"}}}
	app.server.pkg = pm

	result := app.server.RunUpdateQueue(t.Context(), "alice")
	if result.Status != "requires_password" || result.PasswordRequired != 1 {
		t.Fatalf("result = %#v", result)
	}
	if pm.installCalls != 0 {
		t.Fatalf("install calls = %d, want 0", pm.installCalls)
	}
}

func TestUpdateQueueReportsPartialWhenProtectedUpdatesRemain(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db,
		updateTestSourceApp{PackageID: "eligible", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "protected", Version: "2.0.0", InstallProtected: true},
	)
	pm := &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "eligible", Version: "1.0.0"}, {AppID: "protected", Version: "1.0.0"}},
		install:   []InstallResultDTO{{Status: "INSTALL_OK"}},
	}
	app.server.pkg = pm

	result := app.server.RunUpdateQueue(t.Context(), "alice")
	if result.Status != "partial" || result.PasswordRequired != 1 || len(result.Items) != 1 || result.Items[0].Status != "success" {
		t.Fatalf("result = %#v", result)
	}
	if pm.installCalls != 1 {
		t.Fatalf("install calls = %d, want 1", pm.installCalls)
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"autoUpdateEnabled":true`) || !strings.Contains(rec.Body.String(), `"autoSyncEnabled":true`) || !strings.Contains(rec.Body.String(), `"autoSyncIntervalMinutes":5`) || !strings.Contains(rec.Body.String(), `"autoUpdateIntervalMinutes":5`) {
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

func TestAutoUpdateSchedulerNormalizesSourceSyncDependency(t *testing.T) {
	app := testServer(t)
	setting := app.server.db.ClientSyncSetting.Create().
		SetUserID("alice").
		SetAutoSyncEnabled(false).
		SetAutoSyncIntervalMinutes(60).
		SetAutoUpdateEnabled(true).
		SetAutoUpdateIntervalMinutes(15).
		SaveX(t.Context())
	scheduler := &sourceSyncScheduler{server: app.server, running: make(map[string]struct{})}
	if err := scheduler.normalizeAutoUpdateSyncDependencies(t.Context()); err != nil {
		t.Fatal(err)
	}
	setting = app.server.db.ClientSyncSetting.GetX(t.Context(), setting.ID)
	if !setting.AutoSyncEnabled || setting.AutoSyncIntervalMinutes != 15 {
		t.Fatalf("setting = %#v", setting)
	}
}

func TestRunUpdateQueueUsesCachedApplicationsWithoutSourceSync(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "notes", Version: "2.0.0"})
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
	if pm.installCalls != 1 {
		t.Fatalf("install calls = %d", pm.installCalls)
	}
}

func TestAutoUpdateSchedulerUsesCachedApplications(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "notes", Version: "2.0.0"})
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
	if pm.installCalls != 1 {
		t.Fatalf("install calls = %d", pm.installCalls)
	}
}

func TestAutoUpdateSchedulerRecordsPasswordRequiredHint(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "protected", Version: "2.0.0", InstallProtected: true})
	app.server.db.ClientSyncSetting.Create().
		SetUserID("alice").
		SetAutoUpdateEnabled(true).
		SetAutoUpdateIntervalMinutes(5).
		SaveX(t.Context())
	app.server.pkg = &updateQueuePackageManager{installed: []InstalledApplicationDTO{{AppID: "protected", Version: "1.0.0"}}}
	scheduler := &sourceSyncScheduler{server: app.server, running: make(map[string]struct{})}

	if err := scheduler.runDueAutoUpdates(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	setting := app.server.db.ClientSyncSetting.Query().Where(clientsyncsetting.UserIDEQ("alice")).OnlyX(t.Context())
	if setting.LastAutoUpdateStatus == nil || setting.LastAutoUpdateStatus.String() != "partial" {
		t.Fatalf("auto update status = %#v", setting.LastAutoUpdateStatus)
	}
	if setting.LastAutoUpdateError == nil || !strings.Contains(*setting.LastAutoUpdateError, "install password") {
		t.Fatalf("auto update error = %#v", setting.LastAutoUpdateError)
	}
}

func TestAutoUpdateResultStatusKeepsFailureAndPasswordHints(t *testing.T) {
	status, message := autoUpdateResultStatus(UpdateQueueResultDTO{
		Status:           "partial",
		PasswordRequired: 1,
		Items:            []UpdateQueueItemDTO{{Status: "failed"}},
	})
	if status != clientsyncsetting.LastAutoUpdateStatusPartial {
		t.Fatalf("status = %q", status)
	}
	if !strings.Contains(message, "could not be updated") || !strings.Contains(message, "install password") {
		t.Fatalf("message = %q", message)
	}

	_, passwordOnly := autoUpdateResultStatus(UpdateQueueResultDTO{Status: "partial", PasswordRequired: 1})
	if strings.Contains(passwordOnly, "could not be updated") || !strings.Contains(passwordOnly, "install password") {
		t.Fatalf("password-only message = %q", passwordOnly)
	}
}

func TestRequestedUpdateCandidatesInstallOnlySubmittedSnapshot(t *testing.T) {
	app := testServer(t)
	apps := sourceAppsForUpdateTestOnClient(t, app.server.db,
		updateTestSourceApp{PackageID: "first", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "second", Version: "2.0.0"},
	)
	pm := &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "first", Version: "1.0.0"}, {AppID: "second", Version: "1.0.0"}},
		install:   []InstallResultDTO{{TaskID: "task-1", Status: "CREATING"}},
		tasks:     map[string]InstallTaskDTO{"task-1": {TaskID: "task-1", Status: "INSTALL_OK"}},
	}
	app.server.pkg = pm
	result := app.server.RunUpdateQueueWithOptions(t.Context(), "alice", UpdateQueueRequestDTO{Candidates: []UpdateQueueCandidateDTO{{
		AppID: apps[0].ID, SourceID: apps[0].SourceID, PackageID: "first", InstalledVersion: "1.0.0", TargetVersion: "2.0.0",
	}}})
	if result.Status != "success" || len(result.Items) != 1 || result.Items[0].PackageID != "first" || pm.installCalls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, pm.installCalls)
	}
}

func TestRequestedUpdateCandidatesSkipChangedOrMismatchedSnapshot(t *testing.T) {
	app := testServer(t)
	apps := sourceAppsForUpdateTestOnClient(t, app.server.db, updateTestSourceApp{PackageID: "notes", Version: "2.0.0"})
	app.server.pkg = &updateQueuePackageManager{installed: []InstalledApplicationDTO{{AppID: "notes", Version: "1.1.0"}}}
	result := app.server.RunUpdateQueueWithOptions(t.Context(), "alice", UpdateQueueRequestDTO{Candidates: []UpdateQueueCandidateDTO{{
		AppID: apps[0].ID, SourceID: apps[0].SourceID, PackageID: "other", InstalledVersion: "1.0.0", TargetVersion: "2.0.0",
	}}})
	if result.Status != "no_updates" {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunUpdateQueueRejectsIncompleteCandidate(t *testing.T) {
	app := testServer(t)
	rec := app.request(http.MethodPost, "/api/client/v1/updates/run", `{"candidates":[{"appId":1}]}`, "alice")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"INVALID_UPDATE_CANDIDATE"`) {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
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
