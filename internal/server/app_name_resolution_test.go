package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/user"
)

func TestResolveWritableAppByExactName(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	owner := store.server.db.User.Create().SetUsername("publisher").SetPasswordHash("x").SaveX(ctx)
	other := store.server.db.User.Create().SetUsername("other").SetPasswordHash("x").SaveX(ctx)
	token := "lcst_name_resolver"
	store.server.db.APIToken.Create().
		SetUserID(owner.ID).
		SetName("CI").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)

	writable := store.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.current-package").
		SetName("Existing App").
		SetSlug("existing-app-owned").
		SetStatus(app.StatusPENDING).
		SaveX(ctx)
	store.server.db.App.Create().
		SetOwnerID(other.ID).
		SetPackageID("community.lazycat.other-package").
		SetName("Existing App").
		SetSlug("existing-app-other").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)

	rec := resolveAppByName(store, token, "Existing App")
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve response = %d %s", rec.Code, rec.Body.String())
	}
	for _, expected := range []string{
		`"id":` + intString(writable.ID),
		`"packageId":"community.lazycat.current-package"`,
		`"name":"Existing App"`,
		`"canUploadVersion":true`,
	} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Fatalf("resolve response missing %s: %s", expected, rec.Body.String())
		}
	}

	if rec := resolveAppByName(store, token, "existing app"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "APP_NOT_FOUND") {
		t.Fatalf("case-mismatched name = %d %s", rec.Code, rec.Body.String())
	}
	if rec := resolveAppByName(store, token, "Missing"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "APP_NOT_FOUND") {
		t.Fatalf("missing name = %d %s", rec.Code, rec.Body.String())
	}
	if rec := resolveAppByName(store, token, "   "); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "BAD_REQUEST") {
		t.Fatalf("empty name = %d %s", rec.Code, rec.Body.String())
	}
	if rec := resolveAppByName(store, "", "Existing App"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous resolve = %d %s", rec.Code, rec.Body.String())
	}
}

func TestResolveWritableAppByNameRejectsAmbiguousMatches(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	owner := store.server.db.User.Create().SetUsername("publisher").SetPasswordHash("x").SetRole(user.RoleUSER).SaveX(ctx)
	token := "lcst_name_ambiguous"
	store.server.db.APIToken.Create().SetUserID(owner.ID).SetName("CI").SetPrefix(tokenPrefix(token)).SetTokenHash(tokenHash(token)).SaveX(ctx)
	for index, packageID := range []string{"community.lazycat.first", "community.lazycat.second"} {
		store.server.db.App.Create().
			SetOwnerID(owner.ID).
			SetPackageID(packageID).
			SetName("Duplicate App").
			SetSlug("duplicate-app-" + intString(index)).
			SetStatus(app.StatusPENDING).
			SaveX(ctx)
	}

	rec := resolveAppByName(store, token, "Duplicate App")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "APP_NAME_AMBIGUOUS") {
		t.Fatalf("ambiguous name = %d %s", rec.Code, rec.Body.String())
	}
}

func resolveAppByName(store *testApp, token, name string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/apps/by-name?name="+url.QueryEscape(name), nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	store.handler.ServeHTTP(recorder, request)
	return recorder
}

func intString(value int) string {
	return strconv.Itoa(value)
}
