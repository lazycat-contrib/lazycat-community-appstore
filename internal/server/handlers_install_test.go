package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/lazycatpkg"
)

type captureLazyCatInstaller struct {
	identity lazycatpkg.Identity
	request  lazycatpkg.InstallRequest
	result   lazycatpkg.InstallResult
	err      error
}

func (f *captureLazyCatInstaller) InstallLPK(_ context.Context, identity lazycatpkg.Identity, req lazycatpkg.InstallRequest) (lazycatpkg.InstallResult, error) {
	f.identity = identity
	f.request = req
	return f.result, f.err
}

func TestLazyCatRuntimeCapabilities(t *testing.T) {
	store := newTestApp(t)

	tests := []struct {
		name     string
		trusted  bool
		userID   string
		deviceID string
		want     bool
	}{
		{name: "disabled rejects spoofed headers", userID: "alice", deviceID: "pc-1", want: false},
		{name: "missing device", trusted: true, userID: "alice", want: false},
		{name: "missing user", trusted: true, deviceID: "pc-1", want: false},
		{name: "oversized identity", trusted: true, userID: strings.Repeat("a", 257), deviceID: "pc-1", want: false},
		{name: "trusted LazyCat request", trusted: true, userID: "alice", deviceID: "pc-1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.server.cfg.TrustLazyCatClientInstall = tt.trusted
			req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/capabilities", nil)
			req.Header.Set("x-hc-user-id", tt.userID)
			req.Header.Set("x-hc-device-id", tt.deviceID)
			rec := httptest.NewRecorder()
			store.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
			}
			wantJSON := fmt.Sprintf(`"lazycatInstall":%t`, tt.want)
			if !strings.Contains(rec.Body.String(), wantJSON) {
				t.Fatalf("body = %s, want %s", rec.Body.String(), wantJSON)
			}
		})
	}
}

func TestLazyCatInstallUsesTrustedServerData(t *testing.T) {
	store := newTestApp(t)
	store.server.cfg.TrustLazyCatClientInstall = true
	installer := &captureLazyCatInstaller{result: lazycatpkg.InstallResult{
		Mode:   "lazycat-go-sdk",
		TaskID: "task-1",
		Status: "INSTALL_OK",
	}}
	store.server.lazycatInstaller = installer
	record, version := createLazyCatInstallFixture(t, store, "")

	rec := lazyCatInstallRequest(store, record.ID, version.ID, "", "alice", "pc-1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"taskId":"task-1"`) {
		t.Fatalf("install status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if installer.identity != (lazycatpkg.Identity{UserID: "alice", DeviceID: "pc-1"}) {
		t.Fatalf("identity = %#v", installer.identity)
	}
	wantRequest := lazycatpkg.InstallRequest{
		DownloadURL: version.DownloadURL,
		SHA256:      version.Sha256,
		PackageID:   record.PackageID,
		Name:        record.Name,
	}
	if installer.request != wantRequest {
		t.Fatalf("request = %#v, want %#v", installer.request, wantRequest)
	}
	if got := store.server.db.App.GetX(t.Context(), record.ID).DownloadCount; got != 1 {
		t.Fatalf("download count = %d, want 1", got)
	}
}

func TestLazyCatInstallRejectsUntrustedAndInvalidPassword(t *testing.T) {
	store := newTestApp(t)
	installer := &captureLazyCatInstaller{}
	store.server.lazycatInstaller = installer
	record, version := createLazyCatInstallFixture(t, store, "install-secret")

	rec := lazyCatInstallRequest(store, record.ID, version.ID, "install-secret", "alice", "pc-1")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "LAZYCAT_INSTALL_UNAVAILABLE") {
		t.Fatalf("untrusted status = %d, body = %s", rec.Code, rec.Body.String())
	}

	store.server.cfg.TrustLazyCatClientInstall = true
	rec = lazyCatInstallRequest(store, record.ID, version.ID, "wrong", "alice", "pc-1")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "INSTALL_PASSWORD_REQUIRED") {
		t.Fatalf("wrong password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if installer.request.DownloadURL != "" {
		t.Fatal("installer was called for a rejected password")
	}
}

func TestLazyCatInstallFailureIsSafeAndDoesNotIncrementDownloads(t *testing.T) {
	store := newTestApp(t)
	store.server.cfg.TrustLazyCatClientInstall = true
	store.server.lazycatInstaller = &captureLazyCatInstaller{err: errors.New("open /lzcapp/run/certs/private.key: permission denied")}
	record, version := createLazyCatInstallFixture(t, store, "")

	rec := lazyCatInstallRequest(store, record.ID, version.ID, "", "alice", "pc-1")
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "LAZYCAT_INSTALL_FAILED") {
		t.Fatalf("failure status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "/lzcapp/") || strings.Contains(rec.Body.String(), "private.key") {
		t.Fatalf("response leaked SDK internals: %s", rec.Body.String())
	}
	if got := store.server.db.App.GetX(t.Context(), record.ID).DownloadCount; got != 0 {
		t.Fatalf("download count = %d, want 0", got)
	}
}

func TestLazyCatInstallRejectsConcurrentRequest(t *testing.T) {
	store := newTestApp(t)
	store.server.cfg.TrustLazyCatClientInstall = true
	installer := &captureLazyCatInstaller{}
	store.server.lazycatInstaller = installer
	record, version := createLazyCatInstallFixture(t, store, "")
	store.server.lazycatInstallSlots <- struct{}{}
	defer func() { <-store.server.lazycatInstallSlots }()

	rec := lazyCatInstallRequest(store, record.ID, version.ID, "", "alice", "pc-1")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "INSTALL_IN_PROGRESS") {
		t.Fatalf("concurrent status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if installer.request.DownloadURL != "" {
		t.Fatal("installer was called while another installation was active")
	}
}

func createLazyCatInstallFixture(t *testing.T, store *testApp, password string) (*entgo.App, *entgo.AppVersion) {
	t.Helper()
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	create := store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.test.server-install").
		SetName("Server Install App").
		SetSlug("server-install-app").
		SetStatus(app.StatusAPPROVED)
	if password != "" {
		hash, err := hashInstallPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		create.SetInstallPasswordHash(hash)
	}
	record := create.SaveX(ctx)
	version := store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.2.3").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://downloads.example/server-install.lpk").
		SetSha256(strings.Repeat("a", 64)).
		SaveX(ctx)
	return record, version
}

func lazyCatInstallRequest(store *testApp, appID, versionID int, password, userID, deviceID string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"installPassword":%q}`, password)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/versions/%d/install", appID, versionID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hc-user-id", userID)
	req.Header.Set("x-hc-device-id", deviceID)
	rec := httptest.NewRecorder()
	store.handler.ServeHTTP(rec, req)
	return rec
}
