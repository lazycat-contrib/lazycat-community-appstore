package clientserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lazycat.community/appstore/ent"
)

func TestSQLiteDSNAddsPragmas(t *testing.T) {
	dsn := sqliteDSN("./tmp/client.db")
	for _, part := range []string{
		"cache=shared",
		"_fk=1",
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

func TestSyncSourceCachesAppsAndUpdatesSource(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source-Password"); got != "pw" {
			t.Fatalf("password header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{"apps": []map[string]any{{
			"id":       7,
			"name":     "Notes",
			"slug":     "notes",
			"summary":  "Write notes",
			"category": "tools",
			"latestVersion": map[string]any{
				"version":             "1.2.3",
				"downloadUrl":         "https://github.com/org/notes/releases/download/a/notes.lpk",
				"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
				"sha256":              strings.Repeat("a", 64),
				"size":                123,
			},
		}}})
	}))
	defer feed.Close()

	app := testServer(t)
	create := app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`","password":"pw","mirror":"https://ghproxy.example"}`, "alice")
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	apps := app.request("GET", "/api/client/v1/apps", ``, "alice")
	body := apps.Body.String()
	if !strings.Contains(body, `"slug":"notes"`) || !strings.Contains(body, "https://ghproxy.example/https://github.com/org/notes") {
		t.Fatalf("cached app missing mirror rewrite: %s", body)
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
	req       InstallRequestDTO
}

func (f *fakePackageManager) QueryInstalled(ctx context.Context, userID string) ([]InstalledApplicationDTO, error) {
	return f.installed, nil
}

func (f *fakePackageManager) InstallLPK(ctx context.Context, userID string, req InstallRequestDTO) (InstallResultDTO, error) {
	f.req = req
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
	if pm.req.DownloadURL == "" || pm.req.SHA256 == "" || pm.req.PackageID != "notes" {
		t.Fatalf("bad install request: %#v", pm.req)
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
	if _, err := client.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetName("Notes").
		SetSlug("notes").
		SetSummary("Write notes").
		SetLatestVersionJSON(string(version)).
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
	client, err := ent.Open("sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	return client
}
