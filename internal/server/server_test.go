package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/config"
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
		"githubMirror":          "https://mirror.example.com",
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
	if got := app.server.setting(t.Context(), "github_mirror", ""); got != "https://mirror.example.com" {
		t.Fatalf("github_mirror = %q, want https://mirror.example.com", got)
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

func TestAdminCanCreateCollectionAndPublicCanListIt(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
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
	rec := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"github_mirror": "https://mirror.example.com"})
	if rec.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	rec = app.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, version.ID), nil)
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

func TestUserCanListFavorites(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	submitter := app.server.db.User.Create().SetUsername("favorite-submitter").SetPasswordHash("x").SaveX(ctx)
	viewer := app.server.db.User.Create().SetUsername("favorite-viewer").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(submitter.ID).
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

func TestRejectingVersionUploadDoesNotRejectApprovedApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	owner := app.server.db.User.Create().SetUsername("version-owner").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
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

func TestOwnerCanUnlistApp(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("unlister").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
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
		SetName("First App").
		SetSlug("first-app").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	second := app.server.db.App.Create().
		SetOwnerID(admin.ID).
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

func TestApprovedExternalVersionsRespectRetention(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
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
		{"github_mirror": "ftp://mirror.example.com"},
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

	rec := app.do(http.MethodGet, "/source/v1/index.json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("source status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"sourceType":"GITHUB"`) || !strings.Contains(body, `"upstreamDownloadUrl":"`+upstream+`"`) {
		t.Fatalf("source feed missing upstream mirror fields: %s", body)
	}
	if !strings.Contains(body, fmt.Sprintf("/api/v1/apps/%d/versions/", record.ID)) {
		t.Fatalf("source feed missing store download endpoint: %s", body)
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

func TestCollaboratorRequestListIncludesRequesterProfile(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("collab-owner").SetPasswordHash("x").SaveX(ctx)
	requester := app.server.db.User.Create().SetUsername("collab-requester").SetEmail("requester@example.com").SetPasswordHash("x").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(owner.ID).
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
