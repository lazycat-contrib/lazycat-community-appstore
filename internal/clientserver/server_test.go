package clientserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/ent/clientsyncsetting"
	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/pagination"
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
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close client: %v", err)
		}
	}()

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

	alice := app.request("POST", "/api/client/v1/sources", `{"name":"A","url":"https://a.example/source/v1/index.json","password":"secret","groupCodes":[]}`, "alice")
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

func TestClientOIDCSessionScopesSources(t *testing.T) {
	app := testServer(t)
	aliceCookie := app.sessionCookie(t, clientSessionClaims{
		UserID:      "alice",
		DisplayName: "Alice",
		Expiry:      time.Now().Add(time.Hour).Unix(),
	})

	create := app.requestWithCookies("POST", "/api/client/v1/sources", `{"name":"A","url":"https://a.example/source/v1/index.json"}`, "", []*http.Cookie{aliceCookie})
	if create.Code != http.StatusCreated {
		t.Fatalf("create with OIDC session = %d %s", create.Code, create.Body.String())
	}
	list := app.requestWithCookies("GET", "/api/client/v1/sources", ``, "", []*http.Cookie{aliceCookie})
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "a.example") {
		t.Fatalf("alice source missing with OIDC session: status=%d body=%s", list.Code, list.Body.String())
	}
	localList := app.request("GET", "/api/client/v1/sources", ``, "")
	if strings.Contains(localList.Body.String(), "a.example") {
		t.Fatalf("local user saw OIDC source: %s", localList.Body.String())
	}
}

func TestClientHeaderIdentityOverridesOIDCSession(t *testing.T) {
	app := testServer(t)
	aliceCookie := app.sessionCookie(t, clientSessionClaims{
		UserID: "alice",
		Expiry: time.Now().Add(time.Hour).Unix(),
	})
	create := app.requestWithCookies("POST", "/api/client/v1/sources", `{"name":"B","url":"https://b.example/source/v1/index.json"}`, "bob", []*http.Cookie{aliceCookie})
	if create.Code != http.StatusCreated {
		t.Fatalf("create with header and session = %d %s", create.Code, create.Body.String())
	}
	bobList := app.request("GET", "/api/client/v1/sources", ``, "bob")
	if !strings.Contains(bobList.Body.String(), "b.example") {
		t.Fatalf("header user did not own source: %s", bobList.Body.String())
	}
	aliceList := app.requestWithCookies("GET", "/api/client/v1/sources", ``, "", []*http.Cookie{aliceCookie})
	if strings.Contains(aliceList.Body.String(), "b.example") {
		t.Fatalf("OIDC session user saw header-owned source: %s", aliceList.Body.String())
	}
}

func TestClientOIDCEnabledRequiresIdentity(t *testing.T) {
	app := testServer(t)
	app.server.auth.oidc = &clientOIDCRuntime{}

	rec := app.request("GET", "/api/client/v1/sources", ``, "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "CLIENT_AUTH_REQUIRED") {
		t.Fatalf("expected auth requirement, got %d %s", rec.Code, rec.Body.String())
	}
	me := app.request("GET", "/api/client/v1/auth/me", ``, "")
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"oidcEnabled":true`) || !strings.Contains(me.Body.String(), `"authenticated":false`) {
		t.Fatalf("auth status mismatch: %d %s", me.Code, me.Body.String())
	}
	header := app.request("GET", "/api/client/v1/sources", ``, "alice")
	if header.Code != http.StatusOK {
		t.Fatalf("header identity should bypass explicit OIDC login: %d %s", header.Code, header.Body.String())
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

func TestClientSourceStoresDecodedGroupCodes(t *testing.T) {
	app := testServer(t)
	rec := app.request("POST", "/api/client/v1/sources", `{"name":"Private","url":"https://store.example/source/v1/index.json","groupCodes":["abc123","ABC123","old999"]}`, "alice")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create source = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"groupCodes":["ABC123","OLD999"]`) {
		t.Fatalf("group codes not normalized/deduped: %s", rec.Body.String())
	}
}

func TestSyncRemovesInvalidGroupCodesAndKeepsSource(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Group-Codes"); got != "ABC123,OLD999" {
			t.Fatalf("group code header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"groups":            []map[string]string{{"name": "Private", "code": "ABC123"}},
			"invalidGroupCodes": []string{"OLD999"},
			"apps":              []map[string]any{},
		})
	}))
	defer feed.Close()

	app := testServer(t)
	create := app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`","groupCodes":["ABC123","OLD999"]}`, "alice")
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"groupCodes":["ABC123","OLD999"]`) || strings.Contains(body, `"groupCodes":["OLD999"]`) {
		t.Fatalf("invalid code still present in groupCodes: %s", body)
	}
	if !strings.Contains(body, `"groupCodes":["ABC123"]`) || !strings.Contains(body, `"groups":[{"name":"Private"`) || !strings.Contains(body, `"lastInvalidGroupCodes":["OLD999"]`) {
		t.Fatalf("group metadata/cleanup missing: %s", body)
	}
}

func TestClientSettingsStoresSyncConfigInDedicatedTable(t *testing.T) {
	app := testServer(t)
	rec := app.request("PATCH", "/api/client/v1/settings", `{"clientTitle":"  Alice Store  ","commentDisplayName":"  Alice  Cat  ","autoSyncEnabled":true,"autoSyncIntervalMinutes":1,"syncOnStartup":true}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("settings save = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"clientTitle":"Alice Store"`) || !strings.Contains(rec.Body.String(), `"commentDisplayName":"Alice Cat"`) || !strings.Contains(rec.Body.String(), `"defaultPageSize":24`) || !strings.Contains(rec.Body.String(), `"autoSyncIntervalMinutes":5`) || !strings.Contains(rec.Body.String(), `"installSuccessDismissSeconds":3`) {
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
	if count, err := app.server.db.ClientSetting.Query().Count(context.Background()); err != nil || count != 4 {
		t.Fatalf("client_settings count = %d, err = %v", count, err)
	}
	rec = app.request("PATCH", "/api/client/v1/settings", `{"installSuccessDismissSeconds":999}`, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"installSuccessDismissSeconds":60`) {
		t.Fatalf("settings max clamp failed = %d %s", rec.Code, rec.Body.String())
	}
	rec = app.request("PATCH", "/api/client/v1/settings", `{"installSuccessDismissSeconds":0}`, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"installSuccessDismissSeconds":0`) {
		t.Fatalf("settings zero disable failed = %d %s", rec.Code, rec.Body.String())
	}
}

func TestInstallHistoryUsesClientDefaultPageSize(t *testing.T) {
	app := testServer(t)
	ctx := context.Background()
	seedCachedApp(t, app.server.db, "alice")
	sourceApp := app.server.db.ClientSourceApp.Query().OnlyX(ctx)
	for i := range 3 {
		app.server.db.ClientInstallHistory.Create().
			SetUserID("alice").
			SetSourceID(sourceApp.SourceID).
			SetSourceAppID(sourceApp.ID).
			SetSourceName("Feed").
			SetPackageID(fmt.Sprintf("cloud.lazycat.app.notes.%d", i)).
			SetAppName(fmt.Sprintf("Notes %d", i)).
			SetVersion("1.0.0").
			SetResult(clientinstallhistory.ResultSUCCESS).
			SetCreatedAt(time.Now().Add(time.Duration(i) * time.Minute)).
			SaveX(ctx)
	}

	rec := app.request("PATCH", "/api/client/v1/settings", `{"defaultPageSize":2}`, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("settings save = %d %s", rec.Code, rec.Body.String())
	}
	rec = app.request("GET", "/api/client/v1/history", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("history = %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		History    []InstallHistoryDTO `json:"history"`
		Pagination pagination.Response `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(response.History) != 2 {
		t.Fatalf("history length = %d, want 2; body = %s", len(response.History), rec.Body.String())
	}
	if response.Pagination.Page != 1 || response.Pagination.PageSize != 2 || response.Pagination.TotalItems != 3 || response.Pagination.TotalPages != 2 {
		t.Fatalf("pagination = %#v", response.Pagination)
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
			"schema": "lazycat.appstore.source.v2",
			"githubMirrors": []map[string]any{
				{"kind": "download", "name": "Fast", "url": "https://ghproxy.example/https://github.com"},
				{"kind": "download", "name": "Backup", "url": "https://backup.example/https://github.com"},
			},
			"categories": []map[string]any{
				{"id": 1, "name": "Tools", "slug": "tools", "sortOrder": 1},
				{"id": 2, "name": "Utilities", "slug": "utilities", "parentId": 1, "sortOrder": 2},
			},
			"apps": []map[string]any{{
				"id":              7,
				"packageId":       "cloud.lazycat.app.notes",
				"name":            "Notes",
				"slug":            "notes",
				"summary":         "Write notes",
				"categoryId":      2,
				"category":        "tools",
				"iconUrl":         feed.URL + "/icons/notes.png",
				"commentsEnabled": false,
				"latestVersion": map[string]any{
					"version":             "1.2.3",
					"changelog":           "Fix sync and polish UI",
					"downloadUrl":         "https://github.com/org/notes/releases/download/a/notes.lpk",
					"upstreamDownloadUrl": "https://github.com/org/notes/releases/download/a/notes.lpk",
					"sha256":              strings.Repeat("a", 64),
					"size":                123,
				},
				"versions": []map[string]any{
					{
						"version":             "1.2.3",
						"changelog":           "Fix sync and polish UI",
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
	if !strings.Contains(body, `"packageId":"cloud.lazycat.app.notes"`) || !strings.Contains(body, `"categoryId":2`) || !strings.Contains(body, `"iconUrl":"`+feed.URL+`/icons/notes.png"`) || !strings.Contains(body, `"commentsEnabled":false`) || strings.Contains(body, "https://ghproxy.example/https://github.com/org/notes") || !strings.Contains(body, `"version":"1.0.0"`) || !strings.Contains(body, `"changelog":"Fix sync and polish UI"`) {
		t.Fatalf("cached app should keep original download URL: %s", body)
	}
	sources := app.request("GET", "/api/client/v1/sources", ``, "alice")
	mirrorID := mirror.ID(mirror.KindDownload, "https://ghproxy.example/https://github.com")
	if !strings.Contains(sources.Body.String(), `"name":"Fast"`) || !strings.Contains(sources.Body.String(), `"id":"`+mirrorID+`"`) || !strings.Contains(sources.Body.String(), `"categories":[`) || !strings.Contains(sources.Body.String(), `"parentId":1`) {
		t.Fatalf("source did not expose mirrors: %s", sources.Body.String())
	}
	install := app.request("POST", "/api/client/v1/install", `{"appId":1,"mirrorId":"`+mirrorID+`"}`, "alice")
	if install.Code != http.StatusAccepted || !strings.Contains(install.Body.String(), `"taskId":"task-mirror"`) {
		t.Fatalf("install with mirror = %d %s", install.Code, install.Body.String())
	}
	wantURL := "https://ghproxy.example/https://github.com/org/notes/releases/download/a/notes.lpk"
	if pm.req.DownloadURL != wantURL {
		t.Fatalf("mirrored install URL = %q, want %q", pm.req.DownloadURL, wantURL)
	}
}

func TestSyncSourceCachesDataURLIconAsClientAsset(t *testing.T) {
	app := testServer(t)
	iconDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(clientTestPNG1x1)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"schema": "lazycat.appstore.source.v2",
			"apps": []map[string]any{{
				"id":        9,
				"packageId": "cloud.lazycat.app.icon-cache",
				"name":      "Icon Cache",
				"slug":      "icon-cache",
				"summary":   "Caches icons",
				"iconUrl":   iconDataURL,
				"latestVersion": map[string]any{
					"version":     "1.0.0",
					"downloadUrl": "https://example.com/icon-cache.lpk",
					"sha256":      strings.Repeat("b", 64),
				},
			}},
		})
	}))
	defer feed.Close()
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Icon Source").
		SetURL(feed.URL).
		SaveX(t.Context())

	rec := app.request(http.MethodPost, fmt.Sprintf("/api/client/v1/sources/%d/sync", source.ID), "", "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync source = %d %s", rec.Code, rec.Body.String())
	}
	list := app.request(http.MethodGet, "/api/client/v1/apps", "", "alice")
	if list.Code != http.StatusOK {
		t.Fatalf("list apps = %d %s", list.Code, list.Body.String())
	}
	body := list.Body.String()
	if strings.Contains(body, "data:image") || !strings.Contains(body, `"/api/client/v1/assets/`) {
		t.Fatalf("app list did not use client asset URL: %s", body)
	}
	sourceApp := app.server.db.ClientSourceApp.Query().Where(clientsourceapp.SourceIDEQ(source.ID)).OnlyX(t.Context())
	if !strings.HasPrefix(sourceApp.IconURL, "/api/client/v1/assets/") {
		t.Fatalf("cached icon URL = %q", sourceApp.IconURL)
	}
	assetRec := app.request(http.MethodGet, sourceApp.IconURL, "", "alice")
	if assetRec.Code != http.StatusOK || !strings.HasPrefix(assetRec.Header().Get("Content-Type"), "image/png") {
		t.Fatalf("asset response = %d content-type=%q", assetRec.Code, assetRec.Header().Get("Content-Type"))
	}
}

func TestOutdatedMarkProxyForwardsClientIdentityAndBody(t *testing.T) {
	var gotPath, gotProxy, gotDevice, gotPassword, gotBody string
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotProxy = r.Header.Get("X-LazyCat-Client-Proxy")
		gotDevice = r.Header.Get("X-LazyCat-Client-Device-ID")
		gotPassword = r.Header.Get("X-Source-Password")
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read source request body: %v", err)
		}
		gotBody = string(raw)
		writeJSON(w, http.StatusCreated, map[string]any{"outdatedMarks": 2})
	}))
	defer sourceServer.Close()

	app := testServer(t)
	ctx := context.Background()
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Feed").
		SetURL(sourceServer.URL + "/source/v1/index.json").
		SetPassword("pw").
		SaveX(ctx)
	sourceApp := app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/client/v1/apps/%d/outdated-marks", sourceApp.ID), strings.NewReader(`{"note":"new upstream","installedVersion":"1.0.0"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hc-user-id", "alice")
	req.Header.Set("x-hc-device-id", "device-1")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("proxy outdated status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/api/v1/apps/7/outdated-marks" {
		t.Fatalf("source path = %q", gotPath)
	}
	if gotProxy != "lazycat-appstore-client" || gotDevice != "device-1" || gotPassword != "pw" {
		t.Fatalf("missing forwarded headers: proxy=%q device=%q password=%q", gotProxy, gotDevice, gotPassword)
	}
	if !strings.Contains(gotBody, `"note":"new upstream"`) || !strings.Contains(gotBody, `"installedVersion":"1.0.0"`) {
		t.Fatalf("source body = %q", gotBody)
	}
	if !strings.Contains(rec.Body.String(), `"outdatedMarks":2`) {
		t.Fatalf("proxy response body = %s", rec.Body.String())
	}
}

func TestOutdatedMarkProxyEnforcesExactBodyLimit(t *testing.T) {
	var hits atomic.Int64
	var gotBytes atomic.Int64
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		n, _ := io.Copy(io.Discard, r.Body)
		gotBytes.Store(n)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	t.Cleanup(sourceServer.Close)
	app := testServer(t)
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Feed").
		SetURL(sourceServer.URL + "/source/v1/index.json").
		SaveX(t.Context())
	sourceApp := app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SaveX(t.Context())
	request := func(size int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/client/v1/apps/%d/outdated-marks", sourceApp.ID), strings.NewReader(strings.Repeat("x", size)))
		req.Header.Set("x-hc-user-id", "alice")
		req.Header.Set("x-hc-device-id", "device-1")
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(4096); rec.Code != http.StatusOK || gotBytes.Load() != 4096 {
		t.Fatalf("exact-limit response=%d bytes=%d body=%s", rec.Code, gotBytes.Load(), rec.Body.String())
	}
	if rec := request(4097); rec.Code != http.StatusRequestEntityTooLarge || hits.Load() != 1 {
		t.Fatalf("overflow response=%d hits=%d body=%s", rec.Code, hits.Load(), rec.Body.String())
	}
}

func TestChatProxyEnforcesExactBodyLimit(t *testing.T) {
	var hits atomic.Int64
	var gotBytes atomic.Int64
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		n, _ := io.Copy(io.Discard, r.Body)
		gotBytes.Store(n)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	t.Cleanup(sourceServer.Close)
	app := testServer(t)
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Chat Feed").
		SetURL(sourceServer.URL + "/source/v1/index.json").
		SetChatAvailable(true).
		SetChatEnabled(true).
		SaveX(t.Context())
	sourceApp := app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetPackageID("cloud.lazycat.app.chat").
		SetName("Chat").
		SetSlug("chat").
		SaveX(t.Context())
	request := func(size int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/client/v1/apps/%d/chat", sourceApp.ID), strings.NewReader(strings.Repeat("x", size)))
		req.Header.Set("x-hc-user-id", "alice")
		req.Header.Set("x-hc-device-id", "device-1")
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(1 << 20); rec.Code != http.StatusOK || gotBytes.Load() != 1<<20 {
		t.Fatalf("exact-limit response=%d bytes=%d body=%s", rec.Code, gotBytes.Load(), rec.Body.String())
	}
	if rec := request(1<<20 + 1); rec.Code != http.StatusRequestEntityTooLarge || hits.Load() != 1 {
		t.Fatalf("overflow response=%d hits=%d body=%s", rec.Code, hits.Load(), rec.Body.String())
	}
}

func TestCommentProxyRejectsCachedDisabledComments(t *testing.T) {
	var hitSource bool
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitSource = true
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
	}))
	defer sourceServer.Close()

	app := testServer(t)
	ctx := context.Background()
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Feed").
		SetURL(sourceServer.URL + "/source/v1/index.json").
		SaveX(ctx)
	sourceApp := app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetExternalID("7").
		SetPackageID("cloud.lazycat.app.notes").
		SetName("Notes").
		SetSlug("notes").
		SetCommentsEnabled(false).
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/client/v1/apps/%d/comments", sourceApp.ID), strings.NewReader(`{"body":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hc-user-id", "alice")
	req.Header.Set("x-hc-device-id", "device-1")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "COMMENTS_DISABLED") {
		t.Fatalf("disabled comment proxy status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if hitSource {
		t.Fatal("disabled comment proxy reached source server")
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
	task      InstallTaskDTO
	err       error
	req       InstallRequestDTO
	cancelled string
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

func (f *fakePackageManager) GetInstallTask(_ context.Context, _ string, taskID string) (InstallTaskDTO, error) {
	if f.err != nil {
		return InstallTaskDTO{}, f.err
	}
	if f.task.TaskID != taskID {
		return InstallTaskDTO{}, errors.New("task not found")
	}
	return f.task, nil
}

func (f *fakePackageManager) CancelInstall(_ context.Context, _ string, taskID string) error {
	f.cancelled = taskID
	return f.err
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
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"taskId":"task-1"`) {
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

func TestInstallCanSelectOlderVersion(t *testing.T) {
	app := testServer(t)
	pm := &fakePackageManager{install: InstallResultDTO{Mode: "lazycat-go-sdk", TaskID: "task-rollback"}}
	app.server.pkg = pm
	seedCachedApp(t, app.server.db, "alice")

	rec := app.request("POST", "/api/client/v1/install", `{"appId":1,"version":"1.0.0"}`, "alice")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"taskId":"task-rollback"`) {
		t.Fatalf("install old version = %d %s", rec.Code, rec.Body.String())
	}
	if pm.req.Version != "1.0.0" || !strings.Contains(pm.req.DownloadURL, "/old/notes.lpk") || pm.req.SHA256 != strings.Repeat("b", 64) {
		t.Fatalf("bad rollback request: %#v", pm.req)
	}
}

func TestInstallTaskStatusAndCancel(t *testing.T) {
	app := testServer(t)
	pm := &fakePackageManager{task: InstallTaskDTO{TaskID: "task-1", Status: "DOWNLOADING", DownloadedSize: 5}}
	app.server.pkg = pm

	rec := app.request("GET", "/api/client/v1/install-tasks/task-1", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"DOWNLOADING"`) {
		t.Fatalf("task = %d %s", rec.Code, rec.Body.String())
	}
	rec = app.request("DELETE", "/api/client/v1/install-tasks/task-1", ``, "alice")
	if rec.Code != http.StatusOK || pm.cancelled != "task-1" {
		t.Fatalf("cancel = %d %s task=%q", rec.Code, rec.Body.String(), pm.cancelled)
	}
	if rec = app.request("GET", "/api/client/v1/install-tasks/missing", ``, "alice"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing = %d %s", rec.Code, rec.Body.String())
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
	return a.requestWithCookies(method, target, body, userID, nil)
}

func (a *clientTestServer) requestWithCookies(method, target, body, userID string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("x-hc-user-id", userID)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func (a *clientTestServer) sessionCookie(t *testing.T, claims clientSessionClaims) *http.Cookie {
	t.Helper()
	value, err := a.server.auth.signJSON(claims)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: clientSessionCookie, Value: value, Path: "/"}
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

var clientTestPNG1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}
