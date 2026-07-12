package clientserver

import (
	"net/http"
	"strings"
	"testing"
)

func TestClientAppUpdatePolicyIsUserScoped(t *testing.T) {
	app := testServer(t)
	ctx := t.Context()
	app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("alice").
		SetPackageID("community.lazycat.app.lark").
		SetAutoUpdateEnabled(false).
		SaveX(ctx)

	if _, err := app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("alice").
		SetPackageID("community.lazycat.app.lark").
		SetAutoUpdateEnabled(true).
		Save(ctx); err == nil {
		t.Fatal("duplicate user/package policy was accepted")
	}

	app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("bob").
		SetPackageID("community.lazycat.app.lark").
		SetAutoUpdateEnabled(true).
		SaveX(ctx)
}

func TestInstalledAppsIncludeAutoUpdatePolicy(t *testing.T) {
	app := testServer(t)
	app.server.pkg = &updateQueuePackageManager{installed: []InstalledApplicationDTO{
		{AppID: "community.lazycat.app.lark", Version: "1.0.0"},
		{AppID: "community.lazycat.app.notes", Version: "1.0.0"},
	}}
	app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("alice").
		SetPackageID("community.lazycat.app.lark").
		SetAutoUpdateEnabled(false).
		SaveX(t.Context())

	rec := app.request(http.MethodGet, "/api/client/v1/installed", "", "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("installed apps = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"appid":"community.lazycat.app.lark","version":"1.0.0","autoUpdateEnabled":false`) {
		t.Fatalf("disabled policy missing: %s", body)
	}
	if !strings.Contains(body, `"appid":"community.lazycat.app.notes","version":"1.0.0","autoUpdateEnabled":true`) {
		t.Fatalf("default-enabled policy missing: %s", body)
	}
}

func TestUpdatePolicyEndpointIsUserScopedAndIdempotent(t *testing.T) {
	app := testServer(t)
	path := "/api/client/v1/installed-apps/community.lazycat.app.lark/update-policy"
	for _, body := range []string{`{"autoUpdateEnabled":false}`, `{"autoUpdateEnabled":true}`} {
		rec := app.request(http.MethodPatch, path, body, "alice")
		if rec.Code != http.StatusOK {
			t.Fatalf("patch policy = %d %s", rec.Code, rec.Body.String())
		}
	}
	policy := app.server.db.ClientAppUpdatePolicy.Query().OnlyX(t.Context())
	if policy.UserID != "alice" || policy.PackageID != "community.lazycat.app.lark" || !policy.AutoUpdateEnabled {
		t.Fatalf("policy = %#v", policy)
	}

	rec := app.request(http.MethodPatch, path, `{"autoUpdateEnabled":false}`, "bob")
	if rec.Code != http.StatusOK {
		t.Fatalf("bob patch = %d %s", rec.Code, rec.Body.String())
	}
	if count := app.server.db.ClientAppUpdatePolicy.Query().CountX(t.Context()); count != 2 {
		t.Fatalf("policy count = %d", count)
	}
}

func TestUpdatePolicyEndpointRejectsInvalidInput(t *testing.T) {
	app := testServer(t)
	if rec := app.request(http.MethodPatch, "/api/client/v1/installed-apps/%20/update-policy", `{"autoUpdateEnabled":false}`, "alice"); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty package ID = %d %s", rec.Code, rec.Body.String())
	}
	if rec := app.request(http.MethodPatch, "/api/client/v1/installed-apps/community.lazycat.app.lark/update-policy", `{}`, "alice"); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing value = %d %s", rec.Code, rec.Body.String())
	}
}

func TestScheduledUpdateSkipsDisabledApps(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db,
		updateTestSourceApp{PackageID: "enabled", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "disabled", Version: "2.0.0"},
	)
	app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("alice").
		SetPackageID("disabled").
		SetAutoUpdateEnabled(false).
		SaveX(t.Context())
	pm := &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "enabled", Version: "1.0.0"}, {AppID: "disabled", Version: "1.0.0"}},
		install:   []InstallResultDTO{{Status: "INSTALL_OK"}},
	}
	app.server.pkg = pm

	result := app.server.RunUpdateQueueWithOptions(t.Context(), "alice", UpdateQueueRequestDTO{RespectAutoUpdatePolicy: true})
	if len(result.Items) != 1 || result.Items[0].PackageID != "enabled" || result.Status != "success" {
		t.Fatalf("scheduled result = %#v", result)
	}
}

func TestManualUpdateIncludesDisabledApps(t *testing.T) {
	app := testServer(t)
	sourceAppsForUpdateTestOnClient(t, app.server.db,
		updateTestSourceApp{PackageID: "enabled", Version: "2.0.0"},
		updateTestSourceApp{PackageID: "disabled", Version: "2.0.0"},
	)
	app.server.db.ClientAppUpdatePolicy.Create().
		SetUserID("alice").
		SetPackageID("disabled").
		SetAutoUpdateEnabled(false).
		SaveX(t.Context())
	app.server.pkg = &updateQueuePackageManager{
		installed: []InstalledApplicationDTO{{AppID: "enabled", Version: "1.0.0"}, {AppID: "disabled", Version: "1.0.0"}},
		install:   []InstallResultDTO{{Status: "INSTALL_OK"}, {Status: "INSTALL_OK"}},
	}

	result := app.server.RunUpdateQueue(t.Context(), "alice")
	if len(result.Items) != 2 || result.Status != "success" {
		t.Fatalf("manual result = %#v", result)
	}
}
