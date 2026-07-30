package clientserver

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWishProxyRoutesIdentityAndOwnerManagement(t *testing.T) {
	type captured struct{ method, path, userID, deviceID, proxy, body string }
	requests := make(chan captured, 5)
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- captured{method: r.Method, path: r.URL.RequestURI(), userID: r.Header.Get("X-LazyCat-Client-User-ID"), deviceID: r.Header.Get("X-LazyCat-Client-Device-ID"), proxy: r.Header.Get("X-LazyCat-Client-Proxy"), body: string(body)}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	t.Cleanup(sourceServer.Close)
	app := testServer(t)
	source := app.server.db.ClientSource.Create().SetUserID("alice").SetName("Wish Feed").
		SetURL(sourceServer.URL + "/source/v2/index.json").SetWishWallAvailable(true).SaveX(t.Context())

	call := func(method, suffix, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, fmt.Sprintf("/api/client/v1/sources/%d/wishes%s", source.ID, suffix), strings.NewReader(body))
		req.Header.Set("x-hc-user-id", "alice")
		req.Header.Set("x-hc-device-id", "device-1")
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		return rec
	}
	checks := []struct{ method, suffix, body, wantPath string }{
		{http.MethodGet, "?kind=APP_REQUEST&page=2", "", "/api/v1/wishes?kind=APP_REQUEST&page=2"},
		{http.MethodPost, "", `{"kind":"APP_REQUEST","title":"Demo"}`, "/api/v1/wishes"},
		{http.MethodGet, "/42", "", "/api/v1/wishes/42"},
		{http.MethodPatch, "/42", `{"title":"Updated"}`, "/api/v1/wishes/42"},
		{http.MethodDelete, "/42", "", "/api/v1/wishes/42"},
	}
	for _, check := range checks {
		if rec := call(check.method, check.suffix, check.body); rec.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", check.method, check.suffix, rec.Code, rec.Body.String())
		}
		got := <-requests
		if got.method != check.method || got.path != check.wantPath || got.userID != pseudonymousClientUserID(source.URL, "alice") || got.deviceID != "device-1" || got.proxy != "lazycat-appstore-client" || got.body != check.body {
			t.Fatalf("unexpected proxy request: %#v", got)
		}
	}
}

func TestWishProxyRequiresSupportedSourceAndLazyCatDevice(t *testing.T) {
	app := testServer(t)
	source := app.server.db.ClientSource.Create().SetUserID("alice").SetName("Old Feed").SetURL("https://store.example/source/v2/index.json").SaveX(t.Context())
	path := fmt.Sprintf("/api/client/v1/sources/%d/wishes", source.ID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("x-hc-user-id", "alice")
	req.Header.Set("x-hc-device-id", "device-1")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "WISH_WALL_UNAVAILABLE") {
		t.Fatalf("unsupported status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("x-hc-user-id", "alice")
	rec = httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "LAZYCAT_CLIENT_REQUIRED") {
		t.Fatalf("missing device status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSourceSyncSendsClientIdentityAndSurfacesBlockedError(t *testing.T) {
	var gotUserID, gotProxy string
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = r.Header.Get("X-LazyCat-Client-User-ID")
		gotProxy = r.Header.Get("X-LazyCat-Client-Proxy")
		writeError(w, http.StatusForbidden, "CLIENT_BLOCKED", "This client user is blocked")
	}))
	t.Cleanup(sourceServer.Close)
	app := testServer(t)
	source := app.server.db.ClientSource.Create().SetUserID("alice").SetName("Blocked Feed").SetURL(sourceServer.URL + "/source/v2/index.json").SaveX(t.Context())
	_, err := app.server.fetchSourceApps(t.Context(), source)
	if err == nil {
		t.Fatal("fetch succeeded, want blocked error")
	}
	syncErr := normalizeSourceSyncError(err)
	if syncErr.code != "blocked" || syncErr.status != http.StatusForbidden || !strings.Contains(syncErr.message, "封禁") {
		t.Fatalf("sync error=%#v", syncErr)
	}
	if gotUserID != pseudonymousClientUserID(source.URL, "alice") || gotProxy != "lazycat-appstore-client" {
		t.Fatalf("identity headers user=%q proxy=%q", gotUserID, gotProxy)
	}
}
