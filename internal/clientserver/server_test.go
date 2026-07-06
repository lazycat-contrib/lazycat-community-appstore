package clientserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsyncsetting"
	"lazycat.community/appstore/internal/mirror"
)

func TestSQLiteDSNAddsPragmas(t *testing.T) {
	dsn := sqliteDSN("./tmp/client.db")
	for _, part := range []string{
		"cache=shared",
		"_pragma=foreign_keys(1)",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=busy_timeout(10000)",
	} {
		if !strings.Contains(dsn, part) {
			t.Fatalf("sqliteDSN missing %s in %q", part, dsn)
		}
	}
}

func TestClientSourceSchemaUserScopedUniqueness(t *testing.T) {
	ctx := context.Background()
	client := testClient(t)
	defer client.Close()

	_, err := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Community").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ClientSource.Create().
		SetUserID("alice").
		SetName("Duplicate").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx); err == nil {
		t.Fatal("expected duplicate url for same user to fail")
	}
	if _, err := client.ClientSource.Create().
		SetUserID("bob").
		SetName("Community").
		SetURL("https://store.example/source/v1/index.json").
		Save(ctx); err != nil {
		t.Fatalf("same url for different user failed: %v", err)
	}
}

func TestSourceCRUDIsUserScoped(t *testing.T) {
	app := testServer(t)

	alice := app.request("POST", "/api/client/v1/sources", `{"name":"A","url":"https://a.example/source/v1/index.json","password":"secret","mirror":"https://mirror.example"}`, "alice")
	if alice.Code != http.StatusCreated {
		t.Fatalf("create alice = %d %s", alice.Code, alice.Body.String())
	}
	bobList := app.request("GET", "/api/client/v1/sources", ``, "bob")
	if strings.Contains(bobList.Body.String(), "a.example") {
		t.Fatalf("bob saw alice source: %s", bobList.Body.String())
	}
	aliceList := app.request("GET", "/api/client/v1/sources", ``, "alice")
	if !strings.Contains(aliceList.Body.String(), "a.example") {
		t.Fatalf("alice source missing: %s", aliceList.Body.String())
	}
}

func TestSourceDuplicateURLForUserFails(t *testing.T) {
	app := testServer(t)
	body := `{"name":"A","url":"https://a.example/source/v1/index.json"}`
	if rec := app.request("POST", "/api/client/v1/sources", body, "alice"); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d", rec.Code)
	}
	rec := app.request("POST", "/api/client/v1/sources", body, "alice")
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate = %d %s", rec.Code, rec.Body.String())
	}
}

func TestClientSettingsStoresSyncConfigInDedicatedTable(t *testing.T) {
	app := testServer(t)
	rec := app.request("PATCH", "/api/client/v1/settings", `{"commentDisplayName":"  Alice  Cat  ","autoSyncEnabled":true,"autoSyncIntervalMinutes":1,"syncOnStartup":true}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("settings save = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"commentDisplayName":"Alice Cat"`) || !strings.Contains(rec.Body.String(), `"autoSyncIntervalMinutes":5`) {
		t.Fatalf("settings response not normalized: %s", rec.Body.String())
	}
	setting, err := app.server.db.ClientSyncSetting.Query().
		Where(clientsyncsetting.UserIDEQ("alice")).
		Only(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !setting.AutoSyncEnabled || !setting.SyncOnStartup || setting.AutoSyncIntervalMinutes != 5 {
		t.Fatalf("bad sync setting: %#v", setting)
	}
	if count, err := app.server.db.ClientSetting.Query().Count(context.Background()); err != nil || count != 1 {
		t.Fatalf("client_settings count = %d, err = %v", count, err)
	}
}

func TestAutoSyncDueUsesLastRunAndInterval(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	oldRun := now.Add(-61 * time.Minute)
	recentRun := now.Add(-30 * time.Minute)
	if !autoSyncDue(&ent.ClientSyncSetting{AutoSyncEnabled: true, AutoSyncIntervalMinutes: 60, LastAutoSyncAt: &oldRun}, now) {
		t.Fatal("expected old auto sync to be due")
	}
	if autoSyncDue(&ent.ClientSyncSetting{AutoSyncEnabled: true, AutoSyncIntervalMinutes: 60, LastAutoSyncAt: &recentRun}, now) {
		t.Fatal("expected recent auto sync not to be due")
	}
	if autoSyncDue(&ent.ClientSyncSetting{AutoSyncEnabled: false, AutoSyncIntervalMinutes: 60, LastAutoSyncAt: &oldRun}, now) {
		t.Fatal("disabled auto sync should not be due")
	}
}

func TestSyncSourceCachesAppsAndUpdatesSource(t *testing.T) {
	var feed *httptest.Server
	feed = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source-Password"); got != "pw" {
			t.Fatalf("password header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"githubMirrors": []map[string]any{
				{"kind": "download", "name": "Fast", "url": "https://ghproxy.example/https://github.com"},
				{"kind": "download", "name": "Backup", "url": "https://backup.example/https://github.com"},
			},
			"apps": []map[string]any{{
				"id":        7,
				"packageId": "cloud.lazycat.app.notes",
				"name":      "Notes",
				"slug":      "notes",
				"summary":   "Write notes",
				"category":  "tools",
				"iconUrl":   feed.URL + "/icons/notes.png",
				"latestVersion": map[string]any{
					"version":             "1.2.3",
					"downloadUrl":         "https://github.com/org/notes/releases/download/a/notes.lpk",
					"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
					"sha256":              strings.Repeat("a", 64),
					"size":                123,
				},
				"versions": []map[string]any{
					{
						"version":             "1.2.3",
						"downloadUrl":         "https://github.com/org/notes/releases/download/a/notes.lpk",
						"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
						"sha256":              strings.Repeat("a", 64),
						"size":                123,
					},
					{
						"version":             "1.0.0",
						"downloadUrl":         "https://github.com/org/notes/releases/download/old/notes.lpk",
						"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/old/notes.lpk",
						"sha256":              strings.Repeat("b", 64),
						"size":                100,
					},
				},
			}}})
	}))
	defer feed.Close()

	app := testServer(t)
	pm := &fakePackageManager{install: InstallResultDTO{Mode: "lazycat-go-sdk", TaskID: "task-mirror"}}
	app.server.pkg = pm
	create := app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`","password":"pw"}`, "alice")
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	apps := app.request("GET", "/api/client/v1/apps", ``, "alice")
	body := apps.Body.String()
	if !strings.Contains(body, `"packageId":"cloud.lazycat.app.notes"`) || !strings.Contains(body, `"iconUrl":"`+feed.URL+`/icons/notes.png"`) || strings.Contains(body, "https://ghproxy.example/https://github.com/org/notes") || !strings.Contains(body, `"version":"1.0.0"`) {
		t.Fatalf("cached app should keep original download URL: %s", body)
	}
	sources := app.request("GET", "/api/client/v1/sources", ``, "alice")
	mirrorID := mirror.ID(mirror.KindDownload, "https://ghproxy.example/https://github.com")
	if !strings.Contains(sources.Body.String(), `"name":"Fast"`) || !strings.Contains(sources.Body.String(), `"id":"`+mirrorID+`"`) {
		t.Fatalf("source did not expose mirrors: %s", sources.Body.String())
	}
	install := app.request("POST", "/api/client/v1/install", `{"appId":1,"mirrorId":"`+mirrorID+`"}`, "alice")
	if install.Code != http.StatusOK {
		t.Fatalf("install with mirror = %d %s", install.Code, install.Body.String())
	}
	wantURL := "https://ghproxy.example/https://github.com/org/notes/releases/download/a/notes.lpk"
	if pm.req.DownloadURL != wantURL {
		t.Fatalf("mirrored install URL = %q, want %q", pm.req.DownloadURL, wantURL)
	}
}

func TestSyncSourceAuthFailureIsStored(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer feed.Close()
	app := testServer(t)
	_ = app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`"}`, "alice")
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	sources := app.request("GET", "/api/client/v1/sources", ``, "alice")
	if !strings.Contains(sources.Body.String(), `"lastErrorCode":"auth"`) {
		t.Fatalf("auth code not stored: %s", sources.Body.String())
	}
}

type fakePackageManager struct {
	installed []InstalledApplicationDTO
	install   InstallResultDTO
	err       error
	req       InstallRequestDTO
}

func (f *fakePackageManager) QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error) {
	return f.installed, nil
}

func (f *fakePackageManager) InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error) {
	f.req = req
	if f.err != nil {
		return InstallResultDTO{}, f.err
	}
	return f.install, nil
}

func TestInstalledAppsUsesPackageManager(t *testing.T) {
	app := testServer(t)
	app.server.pkg = &fakePackageManager{installed: []InstalledApplicationDTO{{AppID: "notes", Title: "Notes", Version: "1.2.3", Status: "Installed"}}}
	rec := app.request("GET", "/api/client/v1/installed", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"appid":"notes"`) {
		t.Fatalf("installed = %d %s", rec.Code, rec.Body.String())
	}
}

func TestInstallUsesCachedAppVersion(t *testing.T) {
	app := testServer(t)
	pm := &fakePackageManager{install: InstallResultDTO{Mode: "lazycat-go-sdk", TaskID: "task-1"}}
	app.server.pkg = pm
	seedCachedApp(t, app.server.db, "alice")
	rec := app.request("POST", "/api/client/v1/install", `{"appId":1}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("install = %d %s", rec.Code, rec.Body.String())
	}
	if pm.req.DownloadURL == "" || pm.req.SHA256 == "" || pm.req.PackageID != "cloud.lazycat.app.notes" {
		t.Fatalf("bad install request: %#v", pm.req)
	}
}

func TestAppVersionsEndpointReturnsCachedVersions(t *testing.T) {
	app := testServer(t)
	seedCachedApp(t, app.server.db, "alice")

	rec := app.request("GET", "/api/client/v1/apps/1/versions", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"1.0.0"`) || !strings.Contains(rec.Body.String(), `"version":"1.2.3"`) {
		t.Fatalf("versions = %d %s", rec.Code, rec.Body.String())
	}
}

func TestInstallCanSelectOlderVersionAndRecordsHistory(t *testing.T) {
	app := testServer(t)
	pm := &fakePackageManager{install: InstallResultDTO{Mode: "lazycat-go-sdk", TaskID: "task-rollback"}}
	app.server.pkg = pm
	seedCachedApp(t, app.server.db, "alice")

	rec := app.request("POST", "/api/client/v1/install", `{"appId":1,"version":"1.0.0"}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("install old version = %d %s", rec.Code, rec.Body.String())
	}
	if pm.req.Version != "1.0.0" || !strings.Contains(pm.req.DownloadURL, "/old/notes.lpk") || pm.req.SHA256 != strings.Repeat("b", 64) {
		t.Fatalf("bad rollback request: %#v", pm.req)
	}
	history := app.request("GET", "/api/client/v1/history", ``, "alice")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"version":"1.0.0"`) || !strings.Contains(history.Body.String(), `"result":"SUCCESS"`) {
		t.Fatalf("history missing success = %d %s", history.Code, history.Body.String())
	}
}

func TestFailedInstallRecordsHistory(t *testing.T) {
	app := testServer(t)
	app.server.pkg = &fakePackageManager{err: errors.New("sdk offline")}
	seedCachedApp(t, app.server.db, "alice")

	rec := app.request("POST", "/api/client/v1/install", `{"appId":1}`, "alice")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("failed install = %d %s", rec.Code, rec.Body.String())
	}
	history := app.request("GET", "/api/client/v1/history", ``, "alice")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"result":"FAILED"`) || !strings.Contains(history.Body.String(), "sdk offline") {
		t.Fatalf("history missing failure = %d %s", history.Code, history.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	app := testServer(t)
	rec := app.request("GET", "/healthz", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("healthz = %d %s", rec.Code, rec.Body.String())
	}
}

func seedCachedApp(t *testing.T, client *ent.Client, userID string) {
	t.Helper()
	ctx := context.Background()
	source, err := client.ClientSource.Create().
		SetUserID(userID).
		SetName("Feed").
		SetURL("https://feed.example/source/v1/index.json").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	version, err := json.Marshal(VersionDTO{
		Version:     "1.2.3",
		DownloadURL: "https://download.example/notes.lpk",
		SHA256:      strings.Repeat("a", 64),
		Size:        123,
	})
	if err != nil {
		t.Fatal(err)
	}
	versions, err := json.Marshal([]VersionDTO{
		{
			Version:     "1.2.3",
			DownloadURL: "https://download.example/notes.lpk",
			SHA256:      strings.Repeat("a", 64),
			Size:        123,
		},
		{
			Version:     "1.0.0",
			DownloadURL: "https://download.example/old/notes.lpk",
			SHA256:      strings.Repeat("b", 64),
			Size:        100,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SetSummary("Write notes").
		SetLatestVersionJSON(string(version)).
		SetVersionsJSON(string(versions)).
		Save(ctx); err != nil {
		t.Fatal(err)
	}
}

type clientTestServer struct {
	t       *testing.T
	server  *Server
	handler http.Handler
}

func testServer(t *testing.T) *clientTestServer {
	t.Helper()
	client := testClient(t)
	s := newTestServer(client)
	t.Cleanup(func() { _ = s.Close() })
	return &clientTestServer{t: t, server: s, handler: s.Handler()}
}

func (a *clientTestServer) request(method, target, body, userID string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("x-hc-user-id", userID)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func testClient(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client, err := ent.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	return client
}
