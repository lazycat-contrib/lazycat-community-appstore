package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apppkg "lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appscreenshot"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/appvisibility"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/collaborator"
	"lazycat.community/appstore/ent/collaboratorrequest"
	"lazycat.community/appstore/ent/collection"
	"lazycat.community/appstore/ent/collectionapp"
	"lazycat.community/appstore/ent/favorite"
	"lazycat.community/appstore/ent/groupmember"
	"lazycat.community/appstore/ent/outdatedmark"
	"lazycat.community/appstore/ent/registrationinvite"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/config"
	"lazycat.community/appstore/internal/mirror"
)

type testApp struct {
	t       *testing.T
	server  *Server
	handler http.Handler
	cookies []*http.Cookie
}

func TestSetupWizardCreatesFirstSiteAdmin(t *testing.T) {
	app := newTestAppWithAdminBootstrap(t, false)

	rec := app.do(http.MethodGet, "/api/v1/setup/status", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needsSetup":true`) {
		t.Fatalf("setup status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPost, "/api/v1/setup", map[string]any{
		"username":              "owner",
		"email":                 "owner@example.com",
		"password":              "owner-password",
		"sourcePasswordEnabled": true,
		"sourcePassword":        "feed-secret",
		"githubDownloadMirrors": "美国 1=>https://mirror.example.com/https://github.com",
		"requireEmailVerify":    true,
	})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"role":"SITE_ADMIN"`) {
		t.Fatalf("setup create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("setup did not set a session cookie")
	}
	admin := app.server.db.User.Query().Where(user.UsernameEQ("owner")).OnlyX(t.Context())
	if admin.Role != user.RoleSITE_ADMIN || !admin.EmailVerified {
		t.Fatalf("admin role=%s emailVerified=%v", admin.Role, admin.EmailVerified)
	}
	if got := app.server.setting(t.Context(), "source_password", ""); got != "feed-secret" {
		t.Fatalf("source_password = %q, want feed-secret", got)
	}
	if got := app.server.setting(t.Context(), "github_download_mirrors", ""); got != "美国 1=>https://mirror.example.com/https://github.com" {
		t.Fatalf("github_download_mirrors = %q", got)
	}
	if got := app.server.setting(t.Context(), "require_email_verify", "false"); got != "true" {
		t.Fatalf("require_email_verify = %q, want true", got)
	}

	rec = app.do(http.MethodGet, "/api/v1/setup/status", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needsSetup":false`) {
		t.Fatalf("setup status after create = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPost, "/api/v1/setup", map[string]string{
		"username": "second",
		"password": "second-password",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second setup status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestEnvBootstrapStillCreatesDefaultAdmin(t *testing.T) {
	app := newTestApp(t)

	rec := app.do(http.MethodGet, "/api/v1/setup/status", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needsSetup":false`) {
		t.Fatalf("setup status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.login("admin", "changeme")
}

func TestEmbeddedAppConfigUsesSameOriginAPI(t *testing.T) {
	app := newTestApp(t)

	rec := app.do(http.MethodGet, "/app-config.js", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("app config status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"window.LAZYCAT_APPSTORE_CONFIG",
		"apiBaseURL: window.location.origin",
		`defaultSourceURL: window.location.origin + "/source/v1/index.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("app config missing %q, body = %s, headers = %v", want, body, rec.Header())
		}
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestServerSQLiteDSNAddsEntSQLitePragmas(t *testing.T) {
	dsn := sqliteDSN("./tmp/server.db")
	if !strings.HasPrefix(dsn, "file:./tmp/server.db?") {
		t.Fatalf("sqliteDSN = %q, want file URI", dsn)
	}
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

func TestPublicSiteProfileUsesSettings(t *testing.T) {
	app := newTestApp(t)

	rec := app.do(http.MethodGet, "/api/v1/site/profile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("site profile status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"title":"懒猫私有商店服务端"`) || !strings.Contains(rec.Body.String(), `"sourceUrl":"http://store.test/source/v1/index.json"`) {
		t.Fatalf("default site profile body = %s", rec.Body.String())
	}
	var defaultProfile struct {
		Site siteProfile `json:"site"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &defaultProfile); err != nil {
		t.Fatalf("decode default profile: %v", err)
	}
	if defaultProfile.Site.Version != appVersion() {
		t.Fatalf("site profile version = %q, want %q", defaultProfile.Site.Version, appVersion())
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{
		"site_title":              "My NAS Store",
		"site_subtitle":           "发现适合这台 NAS 的应用。",
		"site_icon_url":           "https://cdn.example.com/icon.png",
		"site_public_url":         "https://apps.example.com/",
		"announcement_enabled":    "true",
		"announcement_level":      "warning",
		"announcement_title":      "Maintenance",
		"announcement_body":       "Downloads may be slower tonight.",
		"announcement_link_label": "Details",
		"announcement_link_url":   "https://status.example.com/",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = nil

	rec = app.do(http.MethodGet, "/api/v1/site/profile", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("site profile status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"title":"My NAS Store"`,
		`"subtitle":"发现适合这台 NAS 的应用。"`,
		`"iconUrl":"https://cdn.example.com/icon.png"`,
		`"publicUrl":"https://apps.example.com"`,
		`"sourceUrl":"https://apps.example.com/source/v1/index.json"`,
		`"enabled":true`,
		`"level":"warning"`,
		`"title":"Maintenance"`,
		`"body":"Downloads may be slower tonight."`,
		`"linkUrl":"https://status.example.com"`,
		`"updatedAt":`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("site profile missing %q, body = %s", want, body)
		}
	}

	var profile struct {
		Site siteProfile `json:"site"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	firstAnnouncementUpdate := profile.Site.Announcement.UpdatedAt
	if firstAnnouncementUpdate == "" {
		t.Fatal("announcement updatedAt was not set")
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{
		"site_title":               "My NAS Store",
		"site_subtitle":            "发现适合这台 NAS 的应用。",
		"site_icon_url":            "https://cdn.example.com/icon.png",
		"site_public_url":          "https://apps.example.com",
		"announcement_enabled":     "true",
		"announcement_level":       "warning",
		"announcement_title":       "Maintenance",
		"announcement_body":        "Downloads may be slower tonight.",
		"announcement_link_label":  "Details",
		"announcement_link_url":    "https://status.example.com",
		"announcement_updated_at":  firstAnnouncementUpdate,
		"max_versions":             "8",
		"max_lpk_size":             "1048576",
		"source_password":          "",
		"source_password_rotation": "0",
		"github_download_mirrors":  "",
		"github_raw_mirrors":       "",
		"require_email_verify":     "false",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("policy-only settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = nil
	rec = app.do(http.MethodGet, "/api/v1/site/profile", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile after policy save: %v", err)
	}
	if profile.Site.Announcement.UpdatedAt != firstAnnouncementUpdate {
		t.Fatalf("announcement timestamp changed on policy save: got %q want %q", profile.Site.Announcement.UpdatedAt, firstAnnouncementUpdate)
	}
}

func TestServerDoesNotExposeClientInstalledEndpoint(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/client/v1/installed", nil)
	req.Header.Set("x-hc-user-id", "alice")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("installed endpoint status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"apps"`) || strings.Contains(rec.Body.String(), `"appid"`) {
		t.Fatalf("server leaked installed app payload: %s", rec.Body.String())
	}
}

func TestAdminCanCreateCollectionAndPublicCanListIt(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.featured-app").
		SetName("Featured App").
		SetSlug("featured-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/featured/releases/download/v1/app.lpk").
		SetPublishedAt(time.Now()).
		SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, "/api/v1/admin/collections", map[string]any{
		"name":   "编辑推荐",
		"slug":   "featured",
		"kind":   "MANUAL",
		"appIds": []int{record.ID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create collection status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	rec = app.do(http.MethodGet, "/api/v1/collections", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "编辑推荐") || !strings.Contains(rec.Body.String(), "Featured App") {
		t.Fatalf("list collections status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestScreenshotUploadAppearsOnAppDetail(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.screenshot-app").
		SetName("Screenshot App").
		SetSlug("screenshot-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.login("admin", "changeme")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "screen.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("fake png"))
	_ = writer.WriteField("caption", "首页截图")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/screenshots", record.ID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range app.cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload screenshot status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "首页截图") || !strings.Contains(rec.Body.String(), "screen") {
		t.Fatalf("app detail missing screenshot, status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestMaintainerCanReorderAndDeleteScreenshots(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.managed-screenshots").
		SetName("Managed Screenshots").
		SetSlug("managed-screenshots").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	first := app.server.db.AppScreenshot.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetImageURL("http://store.test/files/first.png").
		SetStoragePath("first.png").
		SetCaption("first").
		SetSortOrder(0).
		SaveX(ctx)
	second := app.server.db.AppScreenshot.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetImageURL("http://store.test/files/second.png").
		SetStoragePath("second.png").
		SetCaption("second").
		SetSortOrder(1).
		SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/screenshots/reorder", record.ID), map[string]any{
		"items": []map[string]int{
			{"id": second.ID, "sortOrder": 0},
			{"id": first.ID, "sortOrder": 1},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder screenshots status = %d, body = %s", rec.Code, rec.Body.String())
	}
	reordered := app.server.db.AppScreenshot.Query().
		Where(appscreenshot.AppIDEQ(record.ID)).
		Order(appscreenshot.BySortOrder()).
		AllX(ctx)
	if reordered[0].ID != second.ID || reordered[1].ID != first.ID {
		t.Fatalf("screenshot order = [%d,%d], want [%d,%d]", reordered[0].ID, reordered[1].ID, second.ID, first.ID)
	}

	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/screenshots/%d", record.ID, second.ID), map[string]any{
		"caption": " updated caption ",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update screenshot status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := app.server.db.AppScreenshot.GetX(ctx, second.ID)
	if updated.Caption != "updated caption" {
		t.Fatalf("caption = %q, want trimmed updated caption", updated.Caption)
	}

	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d/screenshots/%d", record.ID, first.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete screenshot status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if exists, _ := app.server.db.AppScreenshot.Query().Where(appscreenshot.IDEQ(first.ID)).Exist(ctx); exists {
		t.Fatal("deleted screenshot still exists")
	}
}

func TestDownloadEndpointIncrementsDownloadCount(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.download-app").
		SetName("Download App").
		SetSlug("download-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	version := app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/download/releases/download/v1/app.lpk").
		SetPublishedAt(time.Now()).
		SaveX(ctx)

	rec := app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, version.ID), nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("download status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := app.server.db.App.GetX(ctx, record.ID)
	if updated.DownloadCount != 1 {
		t.Fatalf("download_count = %d, want 1", updated.DownloadCount)
	}
}

func TestLocalFileServerDoesNotListDirectories(t *testing.T) {
	app := newTestApp(t)
	if err := os.MkdirAll(filepath.Join(app.server.cfg.LocalStoragePath, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app.server.cfg.LocalStoragePath, "nested", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write storage file: %v", err)
	}

	rec := app.do(http.MethodGet, "/files/", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("directory status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodGet, "/files/nested/", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("nested directory status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodGet, "/files/nested/file.txt", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "content" {
		t.Fatalf("file status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDownloadEndpointUsesGitHubMirror(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.mirrored-app").
		SetName("Mirrored App").
		SetSlug("mirrored-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	version := app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/mirrored/releases/download/v1/app.lpk").
		SetPublishedAt(time.Now()).
		SaveX(ctx)
	app.login("admin", "changeme")
	mirrorURL := "https://mirror.example.com/https://github.com"
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"github_download_mirrors": "Fast=>" + mirrorURL})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, version.ID), nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("download status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if location := rec.Result().Header.Get("Location"); location != "https://github.com/acme/mirrored/releases/download/v1/app.lpk" {
		t.Fatalf("Location without mirror = %q", location)
	}
	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download?mirrorId=%s", record.ID, version.ID, mirror.ID(mirror.KindDownload, mirrorURL)), nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("download status = %d, body = %s", rec.Code, rec.Body.String())
	}
	location := rec.Result().Header.Get("Location")
	want := "https://mirror.example.com/https://github.com/acme/mirrored/releases/download/v1/app.lpk"
	if location != want {
		t.Fatalf("Location = %q, want %q", location, want)
	}
}

func TestUserCanToggleSubmitterFavorite(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	submitter := app.server.db.User.Create().SetUsername("submitter").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("viewer").SetPasswordHash("x").SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/submitters/%d/favorites", submitter.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"favorited":true`) {
		t.Fatalf("favorite submitter status = %d, body = %s", rec.Code, rec.Body.String())
	}
	exists, _ := app.server.db.Favorite.Query().
		Where(favorite.UserIDEQ(viewer.ID), favorite.TargetTypeEQ(favorite.TargetTypeSUBMITTER), favorite.TargetIDEQ(submitter.ID)).
		Exist(ctx)
	if !exists {
		t.Fatal("submitter favorite was not created")
	}

	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/submitters/%d/favorites", submitter.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"favorited":false`) {
		t.Fatalf("unfavorite submitter status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAppDetailIncludesCurrentUserFavoriteState(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	submitter := app.server.db.User.Create().SetUsername("favorite-state-submitter").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("favorite-state-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(submitter.ID).
		SetPackageID("cloud.lazycat.test.favorite-state").
		SetName("Favorite State App").
		SetSlug("favorite-state").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}

	rec := app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"appFavorited":false`) || !strings.Contains(rec.Body.String(), `"submitterFavorited":false`) {
		t.Fatalf("initial favorite state status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/favorites", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"favorited":true`) || !strings.Contains(rec.Body.String(), `"favorites":1`) {
		t.Fatalf("favorite app status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/submitters/%d/favorites", submitter.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"favorited":true`) {
		t.Fatalf("favorite submitter status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"appFavorited":true`) || !strings.Contains(rec.Body.String(), `"submitterFavorited":true`) || !strings.Contains(rec.Body.String(), `"favorites":1`) {
		t.Fatalf("updated favorite state status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUserCanListFavorites(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	submitter := app.server.db.User.Create().SetUsername("favorite-submitter").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("favorite-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(submitter.ID).
		SetPackageID("cloud.lazycat.test.favorite-app").
		SetName("Favorite App").
		SetSlug("favorite-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.Favorite.Create().SetUserID(viewer.ID).SetTargetType(favorite.TargetTypeAPP).SetTargetID(record.ID).SaveX(ctx)
	app.server.db.Favorite.Create().SetUserID(viewer.ID).SetTargetType(favorite.TargetTypeSUBMITTER).SetTargetID(submitter.ID).SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}

	rec := app.do(http.MethodGet, "/api/v1/me/favorites", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Favorite App") || !strings.Contains(rec.Body.String(), "favorite-submitter") {
		t.Fatalf("list favorites status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestOwnerAppInfoUpdateRequiresReviewUnlessUnreviewedUpdatesAllowed(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("publisher").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.stable-app").
		SetName("Stable App").
		SetSlug("stable-app").
		SetSummary("old summary").
		SetStatus(apppkg.StatusAPPROVED).
		SetAllowUnreviewedUpdates(false).
		SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec := app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d", record.ID), map[string]any{
		"name":    "Reviewed App",
		"summary": "new summary",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("reviewed update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	unchanged := app.server.db.App.GetX(ctx, record.ID)
	if unchanged.Name != "Stable App" || unchanged.Summary != "old summary" {
		t.Fatalf("app changed before review: name=%q summary=%q", unchanged.Name, unchanged.Summary)
	}
	review := app.server.db.ReviewRequest.Query().
		Where(reviewrequest.KindEQ(reviewrequest.KindAPP_INFO_UPDATE), reviewrequest.StatusEQ(reviewrequest.StatusPENDING)).
		OnlyX(ctx)

	app.login("admin", "changeme")
	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/reviews/%d/approve", review.ID), map[string]string{"note": "ok"})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve app info review status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := app.server.db.App.GetX(ctx, record.ID)
	if updated.Name != "Reviewed App" || updated.Summary != "new summary" {
		t.Fatalf("approved app info not applied: name=%q summary=%q", updated.Name, updated.Summary)
	}

	fast := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.fast-app").
		SetName("Fast App").
		SetSlug("fast-app").
		SetStatus(apppkg.StatusAPPROVED).
		SetAllowUnreviewedUpdates(true).
		SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d", fast.ID), map[string]any{"summary": "direct"})
	if rec.Code != http.StatusOK {
		t.Fatalf("direct update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := app.server.db.App.GetX(ctx, fast.ID).Summary; got != "direct" {
		t.Fatalf("direct summary = %q, want direct", got)
	}
}

func TestRejectedAppOwnerCanResubmitAppInfo(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("resubmitter").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.rejected-app").
		SetName("Rejected App").
		SetSlug("rejected-app").
		SetSummary("old summary").
		SetStatus(apppkg.StatusREJECTED).
		SetAllowUnreviewedUpdates(false).
		SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec := app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d", record.ID), map[string]any{
		"name":            "Resubmitted App",
		"summary":         "ready now",
		"submitForReview": true,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("resubmit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	pending := app.server.db.App.GetX(ctx, record.ID)
	if pending.Status != apppkg.StatusPENDING || pending.Name != "Rejected App" || pending.Summary != "old summary" {
		t.Fatalf("pending app = status:%s name:%q summary:%q", pending.Status, pending.Name, pending.Summary)
	}
	review := app.server.db.ReviewRequest.Query().
		Where(reviewrequest.KindEQ(reviewrequest.KindAPP_RESUBMISSION), reviewrequest.StatusEQ(reviewrequest.StatusPENDING)).
		OnlyX(ctx)

	app.login("admin", "changeme")
	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/reviews/%d/approve", review.ID), map[string]string{"note": "fixed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve resubmission status = %d, body = %s", rec.Code, rec.Body.String())
	}
	approved := app.server.db.App.GetX(ctx, record.ID)
	if approved.Status != apppkg.StatusAPPROVED || approved.Name != "Resubmitted App" || approved.Summary != "ready now" {
		t.Fatalf("approved app = status:%s name:%q summary:%q", approved.Status, approved.Name, approved.Summary)
	}
}

func TestAdminCanUpdateRejectedAppDirectly(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	owner := app.server.db.User.Create().SetUsername("admin-resubmit-owner").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.admin-resubmit").
		SetName("Rejected Admin App").
		SetSlug("rejected-admin-app").
		SetSummary("old").
		SetStatus(apppkg.StatusREJECTED).
		SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(admin.ID)}
	rec := app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d", record.ID), map[string]any{
		"name":            "Admin Fixed App",
		"summary":         "fixed",
		"submitForReview": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin rejected app update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := app.server.db.App.GetX(ctx, record.ID)
	if updated.Status != apppkg.StatusAPPROVED || updated.Name != "Admin Fixed App" || updated.Summary != "fixed" {
		t.Fatalf("admin updated app = status:%s name:%q summary:%q", updated.Status, updated.Name, updated.Summary)
	}
}

func TestRejectingVersionUploadDoesNotRejectApprovedApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	owner := app.server.db.User.Create().SetUsername("version-owner").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.versioned-app").
		SetName("Versioned App").
		SetSlug("versioned-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	version := app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.0").
		SetStatus(appversion.StatusPENDING).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/versioned/releases/download/v2/app.lpk").
		SaveX(ctx)
	review := app.server.db.ReviewRequest.Create().
		SetKind(reviewrequest.KindVERSION_UPLOAD).
		SetStatus(reviewrequest.StatusPENDING).
		SetAppID(record.ID).
		SetVersionID(version.ID).
		SetRequesterID(owner.ID).
		SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(admin.ID)}

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/reviews/%d/reject", review.ID), map[string]string{"note": "no"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reject version review status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := app.server.db.App.GetX(ctx, record.ID).Status; got != apppkg.StatusAPPROVED {
		t.Fatalf("app status = %s, want APPROVED", got)
	}
}

func TestDynamicSettingsAffectRegistrationAndLPKUpload(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{
		"require_email_verify": "true",
		"max_lpk_size":         "4",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "dynamic-verify",
		"email":    "dynamic@example.com",
		"password": "long-password",
	})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"emailVerified":false`) {
		t.Fatalf("register with dynamic email verify status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.login("admin", "changeme")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "Tiny Limit App")
	_ = writer.WriteField("packageId", "cloud.lazycat.test.tiny-limit-app")
	_ = writer.WriteField("version", "1.0.0")
	part, err := writer.CreateFormFile("file", "too-large.lpk")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = part.Write([]byte("12345"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range app.cookies {
		req.AddCookie(cookie)
	}
	rec = httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "exceeds") {
		t.Fatalf("upload over dynamic max status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRegistrationModeAndInvites(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()

	rec := app.do(http.MethodGet, "/api/v1/site/profile", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"registration":{"mode":"open"}`) {
		t.Fatalf("default registration profile status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"registration_mode": "closed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("close registration status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = nil
	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "closed-user",
		"password": "long-password",
	})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "REGISTRATION_CLOSED") {
		t.Fatalf("closed registration status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"registration_mode": "invite"})
	if rec.Code != http.StatusOK {
		t.Fatalf("invite registration status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPost, "/api/v1/admin/registration-invites", map[string]any{
		"note":    "beta testers",
		"maxUses": 2,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create invite status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Invite registrationInviteDTO `json:"invite"`
		Code   string                `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode invite: %v", err)
	}
	if created.Code == "" || created.Invite.MaxUses != 2 || created.Invite.RemainingUses != 2 {
		t.Fatalf("created invite = %#v code=%q", created.Invite, created.Code)
	}

	app.cookies = nil
	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "missing-invite",
		"password": "long-password",
	})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "INVITE_REQUIRED") {
		t.Fatalf("missing invite status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username":   "invite-one",
		"password":   "long-password",
		"inviteCode": created.Code,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("first invited registration status = %d, body = %s", rec.Code, rec.Body.String())
	}
	record := app.server.db.RegistrationInvite.Query().
		Where(registrationinvite.CodeHashEQ(tokenHash(created.Code))).
		OnlyX(ctx)
	if record.RemainingUses != 1 {
		t.Fatalf("remaining uses after first registration = %d, want 1", record.RemainingUses)
	}

	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username":   "invite-two",
		"password":   "long-password",
		"inviteCode": created.Code,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second invited registration status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if exists := app.server.db.RegistrationInvite.Query().Where(registrationinvite.CodeHashEQ(tokenHash(created.Code))).ExistX(ctx); exists {
		t.Fatal("invite still exists after remaining uses reached zero")
	}

	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username":   "invite-three",
		"password":   "long-password",
		"inviteCode": created.Code,
	})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "INVALID_INVITE") {
		t.Fatalf("exhausted invite status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPost, "/api/v1/admin/registration-invites", map[string]any{
		"note":    "delete me",
		"maxUses": 3,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second invite status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var second struct {
		Invite registrationInviteDTO `json:"invite"`
		Code   string                `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second invite: %v", err)
	}
	rec = app.do(http.MethodGet, "/api/v1/admin/registration-invites", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list invites status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), second.Invite.CodePrefix) || !strings.Contains(rec.Body.String(), second.Code) {
		t.Fatalf("invite list should expose reusable code, body = %s", rec.Body.String())
	}
	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/registration-invites/%d", second.Invite.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete invite status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if exists := app.server.db.RegistrationInvite.Query().Where(registrationinvite.IDEQ(second.Invite.ID)).ExistX(ctx); exists {
		t.Fatal("deleted invite still exists")
	}
}

func TestOwnerCanUnlistApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("unlister").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.public-until-unlisted").
		SetName("Public Until Unlisted").
		SetSlug("public-until-unlisted").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/unlist", record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unlist status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := app.server.db.App.GetX(ctx, record.ID).Status; got != apppkg.StatusUNLISTED {
		t.Fatalf("app status = %s, want UNLISTED", got)
	}

	app.cookies = nil
	rec = app.do(http.MethodGet, "/api/v1/apps", nil)
	if strings.Contains(rec.Body.String(), "Public Until Unlisted") {
		t.Fatalf("unlisted app is publicly visible: %s", rec.Body.String())
	}
}

func TestDeleteAppCleansAssociatedRecords(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	other := app.server.db.User.Create().SetUsername("cleanup-user").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.cleanup-app").
		SetName("Cleanup App").
		SetSlug("cleanup-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	tag := app.server.db.Tag.Create().SetName("cleanup").SetSlug("cleanup").SaveX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(admin.ID).SetName("Cleanup Group").SetSlug("cleanup-group").SaveX(ctx)
	collectionRecord := app.server.db.Collection.Create().SetCreatorID(admin.ID).SetName("Cleanup Collection").SetSlug("cleanup-collection").SaveX(ctx)
	app.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("1.0.0").SetStatus(appversion.StatusAPPROVED).SetSourceType(appversion.SourceTypeLOCAL).SetDownloadURL("http://store.test/app.lpk").SaveX(ctx)
	app.server.db.AppScreenshot.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetImageURL("http://store.test/screen.png").SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(record.ID).SetGroupID(group.ID).SaveX(ctx)
	app.server.db.AppTag.Create().SetAppID(record.ID).SetTagID(tag.ID).SaveX(ctx)
	app.server.db.Collaborator.Create().SetAppID(record.ID).SetUserID(other.ID).SaveX(ctx)
	app.server.db.CollaboratorRequest.Create().SetAppID(record.ID).SetUserID(other.ID).SaveX(ctx)
	app.server.db.OutdatedMark.Create().SetAppID(record.ID).SetUserID(other.ID).SaveX(ctx)
	app.server.db.ReviewRequest.Create().SetKind(reviewrequest.KindAPP_SUBMISSION).SetStatus(reviewrequest.StatusPENDING).SetAppID(record.ID).SetRequesterID(other.ID).SaveX(ctx)
	app.server.db.CollectionApp.Create().SetCollectionID(collectionRecord.ID).SetAppID(record.ID).SaveX(ctx)
	app.server.db.Comment.Create().SetAppID(record.ID).SetUserID(other.ID).SetBody("cleanup").SaveX(ctx)
	app.server.db.Favorite.Create().SetUserID(other.ID).SetTargetType(favorite.TargetTypeAPP).SetTargetID(record.ID).SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete app status = %d, body = %s", rec.Code, rec.Body.String())
	}
	checks := map[string]bool{
		"versions":              app.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).ExistX(ctx),
		"screenshots":           app.server.db.AppScreenshot.Query().Where(appscreenshot.AppIDEQ(record.ID)).ExistX(ctx),
		"visibility":            app.server.db.AppVisibility.Query().Where(appvisibility.AppIDEQ(record.ID)).ExistX(ctx),
		"tags":                  app.server.db.AppTag.Query().Where(apptag.AppIDEQ(record.ID)).ExistX(ctx),
		"collaborators":         app.server.db.Collaborator.Query().Where(collaborator.AppIDEQ(record.ID)).ExistX(ctx),
		"collaborator_requests": app.server.db.CollaboratorRequest.Query().Where(collaboratorrequest.AppIDEQ(record.ID)).ExistX(ctx),
		"outdated":              app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).ExistX(ctx),
		"reviews":               app.server.db.ReviewRequest.Query().Where(reviewrequest.AppIDEQ(record.ID)).ExistX(ctx),
		"collection_apps":       app.server.db.CollectionApp.Query().Where(collectionapp.AppIDEQ(record.ID)).ExistX(ctx),
	}
	for name, exists := range checks {
		if exists {
			t.Fatalf("%s record still exists after app deletion", name)
		}
	}
}

func TestAdminCanUpdateAndDeleteTaxonomy(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, "/api/v1/admin/categories", map[string]string{"name": "工具", "slug": "tools"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category status = %d, body = %s", rec.Code, rec.Body.String())
	}
	createdCategory := app.server.db.Category.Query().Where(category.SlugEQ("tools")).OnlyX(t.Context())
	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/categories/%d", createdCategory.ID), map[string]string{"name": "效率工具", "slug": "efficiency-tools"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "效率工具") {
		t.Fatalf("update category status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/categories/%d", createdCategory.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete category status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodPost, "/api/v1/admin/tags", map[string]string{"name": "NAS", "slug": "nas"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tag status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPatch, "/api/v1/admin/tags/nas", map[string]string{"name": "Home NAS", "slug": "home-nas"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Home NAS") {
		t.Fatalf("update tag status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodDelete, "/api/v1/admin/tags/home-nas", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete tag status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminCanUpdateAndDeleteCollection(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	first := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.first-app").
		SetName("First App").
		SetSlug("first-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	second := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.second-app").
		SetName("Second App").
		SetSlug("second-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, "/api/v1/admin/collections", map[string]any{
		"name":   "初始聚合",
		"slug":   "initial",
		"kind":   "MANUAL",
		"appIds": []int{first.ID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create collection status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := app.server.db.Collection.Query().Where(collection.SlugEQ("initial")).OnlyX(ctx)
	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/collections/%d", created.ID), map[string]any{
		"name":   "编辑精选",
		"slug":   "featured",
		"kind":   "MANUAL",
		"appIds": []int{second.ID, first.ID},
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Second App") {
		t.Fatalf("update collection status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/collections/%d", created.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete collection status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestEmailVerificationFlow(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.RequireEmailVerify = true

	rec := app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "verify-me",
		"email":    "verify@example.com",
		"password": "long-password",
	})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"emailVerified":false`) {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}

	token := app.server.setting(t.Context(), "email_verify:verify-me", "")
	if token == "" {
		t.Fatal("missing email verification token")
	}
	rec = app.do(http.MethodPost, "/api/v1/auth/verify-email", map[string]string{"token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify email status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("verify email did not set a session cookie")
	}
	verified := app.server.db.User.Query().Where(user.UsernameEQ("verify-me")).OnlyX(t.Context())
	if !verified.EmailVerified {
		t.Fatal("user email was not verified")
	}
}

func TestEmailVerificationRequirementBlocksUnverifiedAccounts(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.RequireEmailVerify = true

	rec := app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "missing-mail",
		"password": "long-password",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("register without email status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "blocked-user",
		"email":    "blocked@example.com",
		"password": "long-password",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = rec.Result().Cookies()

	rec = app.do(http.MethodPost, "/api/v1/apps", map[string]string{
		"name": "Blocked App",
	})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "EMAIL_NOT_VERIFIED") {
		t.Fatalf("unverified protected action status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	rec = app.do(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "blocked-user",
		"password": "long-password",
	})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "EMAIL_NOT_VERIFIED") {
		t.Fatalf("unverified login status = %d, body = %s", rec.Code, rec.Body.String())
	}

	token := app.server.setting(t.Context(), "email_verify:blocked-user", "")
	if token == "" {
		t.Fatal("missing email verification token")
	}
	rec = app.do(http.MethodPost, "/api/v1/auth/verify-email", map[string]string{"token": token})
	if rec.Code != http.StatusOK {
		t.Fatalf("verify email status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.login("blocked-user", "long-password")
}

func TestHTTPSBaseURLSetsSecureSessionCookie(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.BaseURL = "https://store.example.com"

	body := strings.NewReader(`{"username":"admin","password":"changeme"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("secure cookie not set: %#v", cookies)
	}
}

func TestRegisterSendsVerificationEmailWhenSMTPConfigured(t *testing.T) {
	app := newTestApp(t)
	mailer := &captureMailer{}
	app.server.cfg.RequireEmailVerify = true
	app.server.cfg.SMTPHost = "smtp.test"
	app.server.cfg.SMTPFrom = "store@example.com"
	app.server.cfg.SitePublicURL = "https://store.example.com"
	app.server.mailer = mailer

	rec := app.do(http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": "mail-me",
		"email":    "mail@example.com",
		"password": "long-password",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}
	token := app.server.setting(t.Context(), "email_verify:mail-me", "")
	if token == "" {
		t.Fatal("missing email verification token")
	}
	if mailer.to != "mail@example.com" || !strings.Contains(mailer.body, token) || !strings.Contains(mailer.body, "https://store.example.com") {
		t.Fatalf("unexpected verification mail: to=%q body=%q", mailer.to, mailer.body)
	}
}

func TestServerStartsWithGitHubStorageBackendForExternalVersions(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{
		Addr:             ":0",
		BaseURL:          "http://store.test",
		ClientOrigins:    []string{"http://client.test"},
		DBDriver:         "sqlite3",
		DBDSN:            filepath.Join(root, "store.db"),
		StorageBackend:   "github",
		LocalStoragePath: filepath.Join(root, "files"),
		MaxLPKSize:       1024 * 1024,
		MaxVersions:      10,
		AdminUsername:    "admin",
		AdminPassword:    "changeme",
		AdminBootstrap:   true,
		SessionSecret:    "test-secret",
		ReadTimeout:      time.Second,
		WriteTimeout:     time.Second,
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New with github storage: %v", err)
	}
	defer srv.Close()
	app := &testApp{t: t, server: srv, handler: srv.Handler()}
	app.login("admin", "changeme")
	rec := app.do(http.MethodPost, "/api/v1/apps", map[string]any{
		"packageId":   "cloud.lazycat.test.github-linked-app",
		"name":        "GitHub Linked App",
		"version":     "1.0.0",
		"sourceType":  "GITHUB",
		"downloadUrl": "https://github.com/acme/linked/releases/download/v1/app.lpk",
		"sha256":      strings.Repeat("a", 64),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create github-linked app status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAPITokenCanPublishExternalApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	publisher := app.server.db.User.Create().SetUsername("ci-publisher").SetPasswordHash("x").SaveX(ctx)
	token := "lcst_ci_publish_token"
	app.server.db.APIToken.Create().
		SetUserID(publisher.ID).
		SetName("CI").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)

	body := strings.NewReader(`{
		"packageId":"cloud.lazycat.test.ci-app",
		"name":"CI App",
		"version":"1.2.3",
		"summary":"published by CI",
		"sourceType":"GITHUB",
		"downloadUrl":"https://github.com/acme/ci/releases/download/v1/app.lpk",
		"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("token publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := app.server.db.AppVersion.Query().OnlyX(ctx)
	if created.Sha256 != strings.Repeat("b", 64) {
		t.Fatalf("sha256 = %q", created.Sha256)
	}
}

func TestExternalVersionRequiresValidSHA256(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")
	rec := app.do(http.MethodPost, "/api/v1/apps", map[string]any{
		"packageId":   "cloud.lazycat.test.bad-checksum-app",
		"name":        "Bad Checksum App",
		"version":     "1.0.0",
		"sourceType":  "GITHUB",
		"downloadUrl": "https://github.com/acme/bad/releases/download/v1/app.lpk",
		"sha256":      "not-a-sha",
	})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "sha256") {
		t.Fatalf("invalid sha status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMultipartCreateAppFillsMetadataFromLPK(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")
	lpk := testLPKArchive(t, "cloud.lazycat.test.upload-meta", "1.2.3", "Upload Meta", "Parsed from package.yml")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "upload-meta.lpk")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(lpk); err != nil {
		t.Fatalf("write lpk: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range app.cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create app from upload status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := app.server.db.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.test.upload-meta")).OnlyX(t.Context())
	if created.Name != "Upload Meta" || created.Slug != "upload-meta" || created.Summary != "Parsed from package.yml" {
		t.Fatalf("metadata not applied: %+v", created)
	}
	if created.IconURL == nil || !strings.HasPrefix(*created.IconURL, "data:image/png;base64,") {
		t.Fatalf("icon metadata not applied: icon=%v", created.IconURL)
	}
	version := app.server.db.AppVersion.Query().Where(appversion.AppIDEQ(created.ID)).OnlyX(t.Context())
	if version.Version != "1.2.3" || version.Sha256 == "" || version.FileSize != int64(len(lpk)) {
		t.Fatalf("version metadata not applied: %+v", version)
	}
}

func TestURLCreateAppFillsMetadataAndSHA256(t *testing.T) {
	app := newTestApp(t)
	app.server.allowPrivateLPKURLHosts = true
	app.login("admin", "changeme")
	lpk := testLPKArchive(t, "cloud.lazycat.test.url-meta", "2.0.0", "URL Meta", "Fetched from URL")
	sum := sha256.Sum256(lpk)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(lpk)
	}))
	defer feed.Close()

	rec := app.do(http.MethodPost, "/api/v1/apps", map[string]any{
		"downloadUrl": feed.URL + "/url-meta.lpk",
		"sourceType":  "GITHUB",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create app from url status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := app.server.db.App.Query().Where(apppkg.PackageIDEQ("cloud.lazycat.test.url-meta")).OnlyX(t.Context())
	version := app.server.db.AppVersion.Query().Where(appversion.AppIDEQ(created.ID)).OnlyX(t.Context())
	if created.Name != "URL Meta" || version.Version != "2.0.0" || version.Sha256 != hex.EncodeToString(sum[:]) || version.FileSize != int64(len(lpk)) {
		t.Fatalf("url metadata not applied: app=%+v version=%+v", created, version)
	}
	if created.IconURL == nil || !strings.HasPrefix(*created.IconURL, "data:image/png;base64,") {
		t.Fatalf("url icon metadata not applied: icon=%v", created.IconURL)
	}
}

func TestNormalizeGitHubRawURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/lazycat-contrib/roon-server-lzcapp/raw/refs/heads/main/community.lazycat.app.roon-server-v2.65.1653.lpk": "https://raw.githubusercontent.com/lazycat-contrib/roon-server-lzcapp/refs/heads/main/community.lazycat.app.roon-server-v2.65.1653.lpk",
		"https://github.com/acme/demo/raw/main/app.lpk?download=1":                                                                   "https://raw.githubusercontent.com/acme/demo/main/app.lpk?download=1",
		"https://github.com/acme/demo/releases/download/v1/app.lpk":                                                                  "https://github.com/acme/demo/releases/download/v1/app.lpk",
	}
	for input, want := range cases {
		if got := normalizeGitHubRawURL(input); got != want {
			t.Fatalf("normalizeGitHubRawURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLPKFetchURLUsesConfiguredGitHubMirrors(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	if err := app.server.setSetting(ctx, settingGitHubDownloadMirrors, "Release=>https://release-mirror.test/https://github.com"); err != nil {
		t.Fatalf("set download mirror: %v", err)
	}
	if err := app.server.setSetting(ctx, settingGitHubRawMirrors, "Raw=>https://raw-mirror.test/https://raw.githubusercontent.com"); err != nil {
		t.Fatalf("set raw mirror: %v", err)
	}

	releaseURL, err := app.server.lpkFetchURL(ctx, "https://github.com/acme/demo/releases/download/v1/app.lpk", true)
	if err != nil {
		t.Fatalf("release fetch url: %v", err)
	}
	if got, want := releaseURL.String(), "https://release-mirror.test/https://github.com/acme/demo/releases/download/v1/app.lpk"; got != want {
		t.Fatalf("release fetch url = %q, want %q", got, want)
	}

	rawURL, err := app.server.lpkFetchURL(ctx, "https://github.com/acme/demo/raw/main/app.lpk", true)
	if err != nil {
		t.Fatalf("raw fetch url: %v", err)
	}
	if got, want := rawURL.String(), "https://raw-mirror.test/https://raw.githubusercontent.com/acme/demo/main/app.lpk"; got != want {
		t.Fatalf("raw fetch url = %q, want %q", got, want)
	}

	directURL, err := app.server.lpkFetchURL(ctx, "https://github.com/acme/demo/releases/download/v1/app.lpk", false)
	if err != nil {
		t.Fatalf("direct fetch url: %v", err)
	}
	if got, want := directURL.String(), "https://github.com/acme/demo/releases/download/v1/app.lpk"; got != want {
		t.Fatalf("direct fetch url = %q, want %q", got, want)
	}
}

func TestInspectLPKURLRetriesConfiguredGitHubMirrors(t *testing.T) {
	app := newTestApp(t)
	app.server.allowPrivateLPKURLHosts = true
	ctx := t.Context()
	lpk := testLPKArchive(t, "cloud.lazycat.test.mirror-retry", "1.0.0", "Mirror Retry", "Fetched from second mirror")
	var firstHits atomic.Int64
	var secondHits atomic.Int64
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits.Add(1)
		http.Error(w, "bad mirror", http.StatusBadGateway)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(lpk)
	}))
	defer second.Close()
	if err := app.server.setSetting(ctx, settingGitHubDownloadMirrors, "Slow=>"+first.URL+"\nFast=>"+second.URL); err != nil {
		t.Fatalf("set mirrors: %v", err)
	}

	inspected, err := app.server.inspectLPKURL(ctx, "https://github.com/acme/retry/releases/download/v1/app.lpk", int64(len(lpk)+1024), true)
	if err != nil {
		t.Fatalf("inspect with retry: %v", err)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 1 {
		t.Fatalf("mirror hits first=%d second=%d, want 1/1", firstHits.Load(), secondHits.Load())
	}
	if inspected.Metadata.PackageID != "cloud.lazycat.test.mirror-retry" || inspected.Metadata.Version != "1.0.0" {
		t.Fatalf("unexpected metadata: %+v", inspected.Metadata)
	}
}

func TestInspectLPKURLGivesEachCandidateSeparateTimeout(t *testing.T) {
	originalCandidateTimeout := lpkFetchCandidateTimeout
	originalTotalTimeout := lpkInspectionTotalTimeout
	lpkFetchCandidateTimeout = 30 * time.Millisecond
	lpkInspectionTotalTimeout = 2 * time.Second
	t.Cleanup(func() {
		lpkFetchCandidateTimeout = originalCandidateTimeout
		lpkInspectionTotalTimeout = originalTotalTimeout
	})

	app := newTestApp(t)
	app.server.allowPrivateLPKURLHosts = true
	ctx := t.Context()
	lpk := testLPKArchive(t, "cloud.lazycat.test.mirror-timeout", "1.0.0", "Mirror Timeout", "Fetched after a timed out mirror")
	var firstHits atomic.Int64
	var secondHits atomic.Int64
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHits.Add(1)
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(lpk)
	}))
	defer second.Close()
	if err := app.server.setSetting(ctx, settingGitHubDownloadMirrors, "Slow=>"+first.URL+"\nFast=>"+second.URL); err != nil {
		t.Fatalf("set mirrors: %v", err)
	}

	inspected, err := app.server.inspectLPKURL(ctx, "https://github.com/acme/retry/releases/download/v1/app.lpk", int64(len(lpk)+1024), true)
	if err != nil {
		t.Fatalf("inspect with per-candidate timeout: %v", err)
	}
	if firstHits.Load() != 1 || secondHits.Load() != 1 {
		t.Fatalf("mirror hits first=%d second=%d, want 1/1", firstHits.Load(), secondHits.Load())
	}
	if inspected.Metadata.PackageID != "cloud.lazycat.test.mirror-timeout" {
		t.Fatalf("unexpected metadata: %+v", inspected.Metadata)
	}
}

func TestVersionUploadRejectsPackageMismatch(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.expected").
		SetName("Expected").
		SetSlug("expected").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.login("admin", "changeme")
	lpk := testLPKArchive(t, "cloud.lazycat.test.other", "1.0.0", "Other", "Mismatch")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "other.lpk")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(lpk); err != nil {
		t.Fatalf("write lpk: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/versions", record.ID), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, cookie := range app.cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "does not match") {
		t.Fatalf("mismatch status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestInspectLPKURLRejectsPrivateHost(t *testing.T) {
	app := newTestApp(t)
	_, err := app.server.inspectLPKURL(t.Context(), "http://127.0.0.1/app.lpk", 1024, false)
	if err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("inspect private host error = %v", err)
	}
}

func TestApprovedExternalVersionsRespectRetention(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.external-retention").
		SetName("External Retention").
		SetSlug("external-retention").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.login("admin", "changeme")
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"max_versions": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for idx, versionName := range []string{"1.0.0", "1.1.0"} {
		rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/versions", record.ID), map[string]any{
			"version":     versionName,
			"sourceType":  "GITHUB",
			"downloadUrl": fmt.Sprintf("https://github.com/acme/external/releases/download/v%d/app.lpk", idx),
			"sha256":      strings.Repeat(fmt.Sprintf("%x", idx+1), 64),
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create external version %s status = %d, body = %s", versionName, rec.Code, rec.Body.String())
		}
	}
	count := app.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		CountX(ctx)
	if count != 1 {
		t.Fatalf("approved external versions = %d, want 1", count)
	}
}

func newTestApp(t *testing.T) *testApp {
	return newTestAppWithAdminBootstrap(t, true)
}

func newTestAppWithAdminBootstrap(t *testing.T, adminBootstrap bool) *testApp {
	t.Helper()
	root := t.TempDir()
	cfg := config.Config{
		Addr:             ":0",
		BaseURL:          "http://store.test",
		ClientOrigins:    []string{"http://client.test"},
		DBDriver:         "sqlite3",
		DBDSN:            filepath.Join(root, "store.db"),
		StorageBackend:   "local",
		LocalStoragePath: filepath.Join(root, "files"),
		MaxLPKSize:       1024 * 1024,
		MaxVersions:      10,
		AdminUsername:    "admin",
		AdminPassword:    "changeme",
		AdminBootstrap:   adminBootstrap,
		SessionSecret:    "test-secret",
		ReadTimeout:      time.Second,
		WriteTimeout:     time.Second,
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return &testApp{t: t, server: srv, handler: srv.Handler()}
}

func (a *testApp) login(username, password string) {
	a.t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"username":%q,"password":%q}`, username, password))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		a.t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	a.cookies = rec.Result().Cookies()
}

func (a *testApp) do(method, path string, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range a.cookies {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func TestAdminSettingSourcePasswordProtectsSourceFeed(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"source_password": "secret"})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("source without password status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, "/source/v1/index.json?password=secret", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source with password status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSettingsRejectInvalidValues(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	tests := []map[string]string{
		{"unknown": "value"},
		{"require_email_verify": "not-bool"},
		{"max_versions": "-1"},
		{"max_lpk_size": "0"},
		{"github_download_mirrors": "Bad=>ftp://mirror.example.com"},
		{"site_public_url": "ftp://apps.example.com"},
		{"site_icon_url": "file:///tmp/icon.png"},
		{"announcement_link_url": "javascript:alert(1)"},
		{"announcement_level": "danger"},
		{"announcement_enabled": "yes"},
		{"registration_mode": "members-only"},
	}
	for _, body := range tests {
		rec := app.do(http.MethodPatch, "/api/v1/admin/settings", body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("settings update %#v status = %d, body = %s", body, rec.Code, rec.Body.String())
		}
	}
	if got := app.server.setting(t.Context(), "unknown", ""); got != "" {
		t.Fatalf("unknown setting was persisted: %q", got)
	}
}

func TestAdminStorageConfigCanBeManagedAndTested(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	rec := app.do(http.MethodGet, "/api/v1/admin/storage", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("storage get status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"provider":"LOCAL"`) || !strings.Contains(rec.Body.String(), `"deliveryMode":"SERVER"`) {
		t.Fatalf("storage defaults body = %s", rec.Body.String())
	}

	root := filepath.Join(t.TempDir(), "objects")
	rec = app.do(http.MethodPatch, "/api/v1/admin/storage", map[string]any{
		"provider":     "LOCAL",
		"deliveryMode": "SERVER",
		"localPath":    root,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("storage update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), root) || !strings.Contains(rec.Body.String(), `/api/v1/files/`) {
		t.Fatalf("storage update body = %s", rec.Body.String())
	}

	rec = app.do(http.MethodPost, "/api/v1/admin/storage/test", map[string]any{
		"provider":     "LOCAL",
		"deliveryMode": "SERVER",
		"localPath":    root,
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("storage test status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminStorageConfigRejectsIncompleteSettings(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	tests := []map[string]any{
		{"provider": "S3", "deliveryMode": "SERVER", "bucketName": "apps"},
		{"provider": "CLOUDFLARE_R2", "deliveryMode": "DIRECT", "endpointUrl": "https://r2.example.com", "bucketName": "apps", "accessKeyId": "ak", "secretAccessKey": "sk"},
		{"provider": "WEBDAV", "deliveryMode": "SERVER", "endpointUrl": "ftp://files.example.com"},
	}
	for _, body := range tests {
		rec := app.do(http.MethodPatch, "/api/v1/admin/storage", body)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("storage update %#v status = %d, body = %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestStorageProxyServesConfiguredLocalObjects(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	root := filepath.Join(t.TempDir(), "objects")
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write storage file: %v", err)
	}
	rec := app.do(http.MethodPatch, "/api/v1/admin/storage", map[string]any{
		"provider":     "LOCAL",
		"deliveryMode": "SERVER",
		"localPath":    root,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("storage update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, "/api/v1/files/primary/nested/file.txt", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "content" {
		t.Fatalf("proxy file status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSettingsDoNotExposeInternalSettings(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	if err := app.server.setSetting(ctx, "email_verify:pending-user", "secret-token"); err != nil {
		t.Fatalf("set internal setting: %v", err)
	}
	if err := app.server.setSetting(ctx, "source_password_rotated_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("set internal rotated_at setting: %v", err)
	}
	app.login("admin", "changeme")

	rec := app.do(http.MethodGet, "/api/v1/admin/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret-token") || strings.Contains(body, "email_verify:") || strings.Contains(body, "source_password_rotated_at") {
		t.Fatalf("internal setting leaked: %s", body)
	}
}

func TestSourceFeedExposesUpstreamDownloadURLForClientMirrors(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.mirrored-source-app").
		SetName("Mirrored Source App").
		SetSlug("mirrored-source-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	upstream := "https://github.com/acme/mirrored-source/releases/download/v1/app.lpk"
	app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL(upstream).
		SetSha256(strings.Repeat("a", 64)).
		SetPublishedAt(time.Now()).
		SaveX(ctx)

	mirrorURL := "https://mirror.example.com/https://github.com"
	app.login("admin", "changeme")
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"github_download_mirrors": "Fast=>" + mirrorURL})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = nil

	rec = app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"sourceType":"GITHUB"`) || !strings.Contains(body, `"upstreamDownloadUrl":"`+upstream+`"`) || !strings.Contains(body, `"githubMirrors"`) || !strings.Contains(body, `"name":"Fast"`) {
		t.Fatalf("source feed missing upstream mirror fields: %s", body)
	}
	if !strings.Contains(body, fmt.Sprintf("/api/v1/apps/%d/versions/", record.ID)) {
		t.Fatalf("source feed missing store download endpoint: %s", body)
	}
}

func TestSourceFeedIncludesSiteProfileMetadata(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{
		"site_title":           "Source Brand",
		"site_public_url":      "https://source.example.com",
		"announcement_enabled": "true",
		"announcement_level":   "info",
		"announcement_title":   "Welcome",
		"announcement_body":    "New apps land every Friday.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	app.cookies = nil

	rec = app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"baseUrl":"https://source.example.com"`,
		`"site":{"title":"Source Brand"`,
		`"publicUrl":"https://source.example.com"`,
		`"sourceUrl":"https://source.example.com/source/v1/index.json"`,
		`"announcement":{"enabled":true`,
		`"title":"Welcome"`,
		`"body":"New apps land every Friday."`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("source feed missing %q, body = %s", want, body)
		}
	}
}

func TestInstallPasswordProtectsDownloadAndSourceFeedMarksApp(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, "/api/v1/apps", map[string]any{
		"packageId":       "cloud.lazycat.test.protected-install-app",
		"name":            "Protected Install App",
		"version":         "1.0.0",
		"summary":         "Requires an install password",
		"sourceType":      "GITHUB",
		"downloadUrl":     "https://github.com/acme/protected/releases/download/v1/app.lpk",
		"sha256":          strings.Repeat("c", 64),
		"installPassword": "install-secret",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create protected app status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		App struct {
			ID               int  `json:"id"`
			InstallProtected bool `json:"installProtected"`
			LatestVersion    struct {
				ID int `json:"id"`
			} `json:"latestVersion"`
		} `json:"app"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created app: %v", err)
	}
	if !created.App.InstallProtected {
		t.Fatalf("created app did not report installProtected: %s", rec.Body.String())
	}

	rec = app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"installProtected":true`) {
		t.Fatalf("source feed did not mark protected app: %s", body)
	}
	if strings.Contains(body, "install-secret") || strings.Contains(body, "installPasswordHash") {
		t.Fatalf("source feed leaked install password data: %s", body)
	}
	if strings.Contains(body, "github.com/acme/protected") {
		t.Fatalf("source feed leaked protected upstream download URL: %s", body)
	}

	downloadPath := fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", created.App.ID, created.App.LatestVersion.ID)
	rec = app.do(http.MethodGet, downloadPath, nil)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "INSTALL_PASSWORD_REQUIRED") {
		t.Fatalf("download without password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	count := app.server.db.App.GetX(t.Context(), created.App.ID).DownloadCount
	if count != 0 {
		t.Fatalf("download count after rejected download = %d, want 0", count)
	}

	rec = app.do(http.MethodGet, downloadPath+"?installPassword=wrong", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("download with wrong password status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodGet, downloadPath+"?installPassword=install-secret", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("download with password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://github.com/acme/protected/releases/download/v1/app.lpk" {
		t.Fatalf("download redirect = %q", got)
	}
	count = app.server.db.App.GetX(t.Context(), created.App.ID).DownloadCount
	if count != 1 {
		t.Fatalf("download count after accepted download = %d, want 1", count)
	}
}

func TestPrivateAppVisibilityUsesGroupsAndSourceFeedStaysPublic(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	alice := app.server.db.User.Create().SetUsername("alice").SetPasswordHash("x").SaveX(ctx)
	bob := app.server.db.User.Create().SetUsername("bob").SetPasswordHash("x").SaveX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(admin.ID).SetName("Team").SetSlug("team").SaveX(ctx)
	app.server.db.GroupMember.Create().SetGroupID(group.ID).SetUserID(alice.ID).SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.private-app").
		SetName("Private App").
		SetSlug("private-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(record.ID).SetGroupID(group.ID).SaveX(ctx)
	app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/private/releases/download/v1/app.lpk").
		SetPublishedAt(time.Now()).
		SaveX(ctx)

	rec := app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Private App") {
		t.Fatalf("private app leaked into source feed: %s", rec.Body.String())
	}

	app.cookies = []*http.Cookie{app.serverCookieFor(alice.ID)}
	rec = app.do(http.MethodGet, "/api/v1/apps", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Private App") {
		t.Fatalf("group member cannot see private app, status = %d body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = []*http.Cookie{app.serverCookieFor(bob.ID)}
	rec = app.do(http.MethodGet, "/api/v1/apps", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("non-member list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Private App") {
		t.Fatalf("non-member can see private app: %s", rec.Body.String())
	}
}

func TestGroupMembershipAndVisibilityRejectInvalidAssignments(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("visibility-owner").SetPasswordHash("x").SaveX(ctx)
	otherOwner := app.server.db.User.Create().SetUsername("other-owner").SetPasswordHash("x").SaveX(ctx)
	ownedGroup := app.server.db.UserGroup.Create().SetOwnerID(owner.ID).SetName("Owned").SetSlug("owned").SaveX(ctx)
	otherGroup := app.server.db.UserGroup.Create().SetOwnerID(otherOwner.ID).SetName("Other").SetSlug("other").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.visibility-guard").
		SetName("Visibility Guard").
		SetSlug("visibility-guard").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(record.ID).SetGroupID(ownedGroup.ID).SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/members/%d", ownedGroup.ID, 999999), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("add nonexistent member status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if exists := app.server.db.GroupMember.Query().Where(groupmember.GroupIDEQ(ownedGroup.ID), groupmember.UserIDEQ(999999)).ExistX(ctx); exists {
		t.Fatal("nonexistent user was added to group")
	}

	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/visibility", record.ID), map[string]any{"groupIds": []int{otherGroup.ID}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("assign foreign group status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ids := app.server.visibleGroupIDs(ctx, record.ID)
	if len(ids) != 1 || ids[0] != ownedGroup.ID {
		t.Fatalf("visibility changed after rejected request: %#v", ids)
	}
}

func TestSocialActionsRequireVisibleApprovedApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("social-owner").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("social-viewer").SetPasswordHash("x").SaveX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(owner.ID).SetName("Private Social").SetSlug("private-social").SaveX(ctx)
	privateApp := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.private-social-app").
		SetName("Private Social App").
		SetSlug("private-social-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(privateApp.ID).SetGroupID(group.ID).SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}

	blocked := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/comments", privateApp.ID), map[string]string{"body": "hidden"}},
		{http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/favorites", privateApp.ID), nil},
		{http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", privateApp.ID), map[string]string{"note": "hidden"}},
		{http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", privateApp.ID), nil},
		{http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/collaborator-requests", privateApp.ID), map[string]string{"message": "hidden"}},
	}
	for _, item := range blocked {
		rec := app.do(item.method, item.path, item.body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, body = %s", item.method, item.path, rec.Code, rec.Body.String())
		}
	}

	if app.server.db.Favorite.Query().Where(favorite.UserIDEQ(viewer.ID), favorite.TargetIDEQ(privateApp.ID)).ExistX(ctx) {
		t.Fatal("favorite was created for hidden app")
	}
	if app.server.db.OutdatedMark.Query().Where(outdatedmark.UserIDEQ(viewer.ID), outdatedmark.AppIDEQ(privateApp.ID)).ExistX(ctx) {
		t.Fatal("outdated mark was created for hidden app")
	}
	if app.server.db.CollaboratorRequest.Query().Where(collaboratorrequest.UserIDEQ(viewer.ID), collaboratorrequest.AppIDEQ(privateApp.ID)).ExistX(ctx) {
		t.Fatal("collaborator request was created for hidden app")
	}
}

func TestSiteCommentsSettingDisablesNewCommentsAndSourceFeed(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	viewer := app.server.db.User.Create().SetUsername("comment-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.site-comments").
		SetName("Site Comments").
		SetSlug("site-comments").
		SetStatus(apppkg.StatusAPPROVED).
		SetCommentsEnabled(true).
		SaveX(ctx)
	app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/site-comments/releases/download/v1/app.lpk").
		SetSha256(strings.Repeat("d", 64)).
		SetPublishedAt(time.Now()).
		SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(admin.ID)}
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"comments_enabled": "false"})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable comments setting status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}
	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/comments", record.ID), map[string]string{"body": "hello"})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "COMMENTS_DISABLED") {
		t.Fatalf("disabled comment status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.Comment.Query().CountX(ctx); count != 0 {
		t.Fatalf("comments after disabled post = %d, want 0", count)
	}

	app.cookies = nil
	rec = app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"commentsEnabled":false`) {
		t.Fatalf("source feed comments flag status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestMarkOutdatedRequiresReasonAndNotifiesOwner(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	mailer := &captureMailer{}
	app.server.mailer = mailer
	if err := app.server.setSetting(ctx, settingSMTPHost, "smtp.test"); err != nil {
		t.Fatal(err)
	}
	if err := app.server.setSetting(ctx, settingSMTPFrom, "store@example.com"); err != nil {
		t.Fatal(err)
	}
	owner := app.server.db.User.Create().SetUsername("outdated-owner").SetEmail("owner@example.com").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("outdated-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.outdated-app").
		SetName("Outdated App").
		SetSlug("outdated-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", record.ID), map[string]string{"note": " "})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty outdated note status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).CountX(ctx); count != 0 {
		t.Fatalf("outdated marks after empty note = %d, want 0", count)
	}

	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", record.ID), map[string]string{
		"note":             "Upstream released a newer build.",
		"installedVersion": "1.0.0",
		"expectedVersion":  "1.2.0",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("mark outdated status = %d, body = %s", rec.Code, rec.Body.String())
	}
	mark := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID), outdatedmark.UserIDEQ(viewer.ID)).OnlyX(ctx)
	for _, want := range []string{
		"Reason:\nUpstream released a newer build.",
		"Current or installed version: 1.0.0",
		"Expected newer version or source: 1.2.0",
	} {
		if !strings.Contains(mark.Note, want) {
			t.Fatalf("outdated note missing %q: %q", want, mark.Note)
		}
		if !strings.Contains(mailer.body, want) {
			t.Fatalf("outdated mail missing %q: %q", want, mailer.body)
		}
	}
	if mailer.to != "owner@example.com" || mailer.subject != "Update requested for Outdated App" {
		t.Fatalf("unexpected outdated mail: to=%q subject=%q body=%q", mailer.to, mailer.subject, mailer.body)
	}

	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"outdatedMarks":1`) || !strings.Contains(body, `"outdatedMarked":true`) {
		t.Fatalf("detail outdated state missing: status=%d body=%s", rec.Code, body)
	}
}

func TestManualOutdatedClearRequiresSettingAndMaintainer(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("manual-clear-owner").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("manual-clear-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.manual-clear").
		SetName("Manual Clear").
		SetSlug("manual-clear").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.OutdatedMark.Create().SetAppID(record.ID).SetUserID(viewer.ID).SetNote("needs update").SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}

	rec := app.do(http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", record.ID), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("default clear status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).CountX(ctx); count != 1 {
		t.Fatalf("outdated marks after default owner clear = %d, want 1", count)
	}

	if err := app.server.setSetting(ctx, settingAllowManualOutdatedClear, "true"); err != nil {
		t.Fatal(err)
	}
	other := app.server.db.User.Create().SetUsername("manual-clear-other").SetPasswordHash("x").SaveX(ctx)
	app.server.db.OutdatedMark.Create().SetAppID(record.ID).SetUserID(other.ID).SetNote("also needs update").SaveX(ctx)
	app.cookies = []*http.Cookie{app.serverCookieFor(viewer.ID)}
	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", record.ID), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer clear status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).CountX(ctx); count != 2 {
		t.Fatalf("outdated marks after enabled viewer clear = %d, want 2", count)
	}

	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec = app.do(http.MethodDelete, fmt.Sprintf("/api/v1/apps/%d/outdated-marks", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"outdatedMarks":0`) {
		t.Fatalf("manual clear status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).CountX(ctx); count != 0 {
		t.Fatalf("outdated marks after enabled owner clear = %d, want 0", count)
	}

	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canClearOutdatedMarks":true`) {
		t.Fatalf("detail manual clear capability status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestApprovedVersionClearsOutdatedMarks(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	viewer := app.server.db.User.Create().SetUsername("outdated-clear-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.outdated-clear").
		SetName("Outdated Clear").
		SetSlug("outdated-clear").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.OutdatedMark.Create().SetAppID(record.ID).SetUserID(viewer.ID).SetNote("needs update").SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/versions", record.ID), map[string]any{
		"version":     "2.0.0",
		"sourceType":  "GITHUB",
		"downloadUrl": "https://github.com/acme/outdated-clear/releases/download/v2/app.lpk",
		"sha256":      strings.Repeat("c", 64),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create clearing version status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.OutdatedMark.Query().Where(outdatedmark.AppIDEQ(record.ID)).CountX(ctx); count != 0 {
		t.Fatalf("outdated marks after approved version = %d, want 0", count)
	}
}

func TestCollaboratorRequestListIncludesRequesterProfile(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("collab-owner").SetPasswordHash("x").SaveX(ctx)
	requester := app.server.db.User.Create().SetUsername("collab-requester").SetEmail("requester@example.com").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.collab-app").
		SetName("Collab App").
		SetSlug("collab-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.CollaboratorRequest.Create().
		SetAppID(record.ID).
		SetUserID(requester.ID).
		SetMessage("can help").
		SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec := app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d/collaborator-requests", record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list collaborator requests status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"username":"collab-requester"`) || !strings.Contains(body, `"email":"requester@example.com"`) || !strings.Contains(body, `"userId":`) {
		t.Fatalf("collaborator request profile missing: %s", body)
	}
}

func TestMyCollaborationHidesOwnedAppsWithoutCollaborationRecords(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("collab-filter-owner").SetPasswordHash("x").SaveX(ctx)
	member := app.server.db.User.Create().SetUsername("collab-filter-member").SetPasswordHash("x").SaveX(ctx)
	app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.empty-collab").
		SetName("Empty Collab").
		SetSlug("empty-collab").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	withCollaborator := app.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.with-collab").
		SetName("With Collab").
		SetSlug("with-collab").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.Collaborator.Create().SetAppID(withCollaborator.ID).SetUserID(member.ID).SaveX(ctx)

	app.cookies = []*http.Cookie{app.serverCookieFor(owner.ID)}
	rec := app.do(http.MethodGet, "/api/v1/me/collaboration", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my collaboration status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Owned []ownedCollaborationDTO `json:"owned"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode my collaboration: %v", err)
	}
	if len(payload.Owned) != 1 {
		t.Fatalf("owned collaboration count = %d, body = %s", len(payload.Owned), rec.Body.String())
	}
	if payload.Owned[0].App.ID != withCollaborator.ID {
		t.Fatalf("owned collaboration app = %q, want %q", payload.Owned[0].App.Name, withCollaborator.Name)
	}
}

func TestSiteAdminCanListAndUpdateUsers(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	created := app.server.db.User.Create().SetUsername("reviewer").SetPasswordHash("x").SaveX(ctx)
	app.login("admin", "changeme")

	rec := app.do(http.MethodGet, "/api/v1/admin/users", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "reviewer") {
		t.Fatalf("list users status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = app.do(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", created.ID), map[string]any{"role": "SOFTWARE_ADMIN"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update user status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := app.server.db.User.GetX(ctx, created.ID)
	if updated.Role != user.RoleSOFTWARE_ADMIN {
		t.Fatalf("role = %s, want SOFTWARE_ADMIN", updated.Role)
	}
}

func testLPKArchive(t *testing.T, packageID, version, name, description string) []byte {
	t.Helper()
	body := fmt.Sprintf("package: %s\nversion: %s\nname: %s\ndescription: %s\nicon: icon.png\n", packageID, version, name, description)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeTestTarFile(t, tw, "package.yml", []byte(body))
	writeTestTarFile(t, tw, "icon.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d})
	if err := tw.Close(); err != nil {
		t.Fatalf("Close tar: %v", err)
	}
	return buf.Bytes()
}

func writeTestTarFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Write %s: %v", name, err)
	}
}

func (a *testApp) serverCookieFor(userID int) *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: a.server.signSession(userID), Path: "/"}
}

type captureMailer struct {
	to      string
	subject string
	body    string
}

func (m *captureMailer) Send(_ context.Context, to, subject, body string) error {
	m.to = to
	m.subject = subject
	m.body = body
	return nil
}
