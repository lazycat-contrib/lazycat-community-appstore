package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/user"
)

func TestPackageLatestVersion(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.latest-test").
		SetName("Latest Test").
		SetSlug("latest-test").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).
		SaveX(ctx)
	store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("2.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)).
		SaveX(ctx)
	store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.versionless").
		SetName("Versionless").
		SetSlug("versionless").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.pending").
		SetName("Pending").
		SetSlug("pending").
		SetStatus(app.StatusPENDING).
		SaveX(ctx)
	group := store.server.db.UserGroup.Create().
		SetOwnerID(admin.ID).
		SetName("Latest Version Group").
		SetSlug("latest-version-group").
		SetCode("LATE23").
		SaveX(ctx)
	groupApp := store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.group-latest-test").
		SetName("Group Latest Test").
		SetSlug("group-latest-test").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	store.server.db.AppVisibility.Create().SetAppID(groupApp.ID).SetGroupID(group.ID).SaveX(ctx)
	store.server.db.AppVersion.Create().
		SetAppID(groupApp.ID).
		SetUploaderID(admin.ID).
		SetVersion("3.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SaveX(ctx)

	rec := store.do(http.MethodGet, "/api/v1/packages/community.lazycat.latest-test/latest-version", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"packageId":"community.lazycat.latest-test"`) || !strings.Contains(rec.Body.String(), `"version":"2.0.0"`) {
		t.Fatalf("latest version response = %d %s", rec.Code, rec.Body.String())
	}

	for _, packageID := range []string{"community.lazycat.missing", "community.lazycat.versionless", "community.lazycat.pending"} {
		rec := store.do(http.MethodGet, "/api/v1/packages/"+packageID+"/latest-version", nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "APP_NOT_FOUND") {
			t.Fatalf("package %s response = %d %s", packageID, rec.Code, rec.Body.String())
		}
	}

	groupPath := "/api/v1/packages/community.lazycat.group-latest-test/latest-version"
	if rec := store.do(http.MethodGet, groupPath, nil); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "APP_NOT_FOUND") {
		t.Fatalf("group package without code = %d %s", rec.Code, rec.Body.String())
	}
	if rec := store.do(http.MethodGet, groupPath+"?groupCodes=LATE23", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"3.0.0"`) {
		t.Fatalf("group package with code = %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, groupPath, nil)
	req.Header.Set("X-Group-Codes", "LATE23")
	for _, cookie := range store.cookies {
		req.AddCookie(cookie)
	}
	headerRec := httptest.NewRecorder()
	store.handler.ServeHTTP(headerRec, req)
	if headerRec.Code != http.StatusOK || !strings.Contains(headerRec.Body.String(), `"version":"3.0.0"`) {
		t.Fatalf("group package with header code = %d %s", headerRec.Code, headerRec.Body.String())
	}
}
