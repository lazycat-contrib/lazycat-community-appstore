package server

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdminMCPAppManagement(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	adminReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	record := testApp.server.db.App.Create().SetOwnerID(admin.ID).SetPackageID("test.mcp.manage").
		SetName("Original").SetSummary("Preserved").SetDescription("Original description").SetSlug("mcp-manage").SetStatus(app.StatusAPPROVED).SaveX(ctx)
	target := mcpAppTargetInput{AppID: record.ID}
	if _, _, err := testApp.server.mcpAdminGetApp(ctx, userReq, target); err == nil {
		t.Fatal("USER token accessed admin app details")
	}
	if _, _, err := testApp.server.mcpAdminUpdateApp(ctx, userReq, mcpAdminUpdateAppInput{AppID: record.ID, Name: new("Denied")}); err == nil {
		t.Fatal("USER token updated app")
	}
	_, updated, err := testApp.server.mcpAdminUpdateApp(ctx, adminReq, mcpAdminUpdateAppInput{
		PackageID: record.PackageID, Name: new("Renamed"), Description: new(""), CommentsEnabled: new(false),
	})
	if err != nil || updated.App.Name != "Renamed" || updated.App.Summary != "Preserved" || updated.App.Description != "" {
		t.Fatalf("update = %+v, err = %v", updated, err)
	}
	if testApp.server.db.App.GetX(ctx, record.ID).CommentsEnabled {
		t.Fatal("explicit false was ignored")
	}
	if _, _, err := testApp.server.mcpAdminUpdateApp(ctx, adminReq, mcpAdminUpdateAppInput{AppID: record.ID, Name: new(" ")}); err == nil {
		t.Fatal("accepted blank name")
	}
	if _, _, err := testApp.server.mcpAdminGetApp(ctx, adminReq, mcpAppTargetInput{AppID: record.ID, PackageID: record.PackageID}); err == nil {
		t.Fatal("accepted ambiguous app target")
	}
	_, unlisted, err := testApp.server.mcpAdminUnlistApp(ctx, adminReq, target)
	if err != nil || unlisted.App.Status != "UNLISTED" {
		t.Fatalf("unlist = %+v, err = %v", unlisted, err)
	}
	if _, _, err := testApp.server.mcpAdminDeleteApp(ctx, adminReq, mcpAdminDeleteAppInput{AppID: record.ID}); err == nil {
		t.Fatal("deleted app without confirmation")
	}
	if _, _, err := testApp.server.mcpAdminDeleteApp(ctx, userReq, mcpAdminDeleteAppInput{AppID: record.ID, Confirm: true}); err == nil {
		t.Fatal("USER token deleted app")
	}
	testApp.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("1.0.0").SaveX(ctx)
	if _, result, err := testApp.server.mcpAdminDeleteApp(ctx, adminReq, mcpAdminDeleteAppInput{PackageID: record.PackageID, Confirm: true}); err != nil || !result.OK {
		t.Fatalf("delete = %+v, err = %v", result, err)
	}
	if testApp.server.db.App.Query().Where(app.IDEQ(record.ID)).ExistX(ctx) || testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).ExistX(ctx) {
		t.Fatal("app or its versions remain after deletion")
	}
}

func TestAdminMCPVersionManagement(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	adminReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	record := testApp.server.db.App.Create().SetOwnerID(admin.ID).SetPackageID("test.mcp.versions").SetName("Versions").SetSlug("mcp-versions").SaveX(ctx)
	other := testApp.server.db.App.Create().SetOwnerID(admin.ID).SetPackageID("test.mcp.other").SetName("Other").SetSlug("mcp-other").SaveX(ctx)
	first := testApp.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("1.0.0").SetStatus(appversion.StatusAPPROVED).SaveX(ctx)
	second := testApp.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("2.0.0").SetStatus(appversion.StatusAPPROVED).SaveX(ctx)
	if _, _, err := testApp.server.mcpAdminListVersions(ctx, userReq, mcpAdminListVersionsInput{AppID: record.ID}); err == nil {
		t.Fatal("USER token listed private versions")
	}
	_, listed, err := testApp.server.mcpAdminListVersions(ctx, adminReq, mcpAdminListVersionsInput{PackageID: record.PackageID, Limit: 1})
	if err != nil || len(listed.Versions) != 1 || listed.Pagination.TotalItems != 2 || listed.NextPage != 2 {
		t.Fatalf("list = %+v, err = %v", listed, err)
	}
	_, updated, err := testApp.server.mcpAdminUpdateVersion(ctx, adminReq, mcpAdminUpdateVersionInput{AppID: record.ID, VersionID: first.ID, Changelog: "Corrected notes"})
	if err != nil || updated.Version.Changelog != "Corrected notes" || updated.Version.Version != "1.0.0" {
		t.Fatalf("version update = %+v, err = %v", updated, err)
	}
	if _, _, err := testApp.server.mcpAdminUpdateVersion(ctx, userReq, mcpAdminUpdateVersionInput{AppID: record.ID, VersionID: first.ID}); err == nil {
		t.Fatal("USER token edited version")
	}
	if _, _, err := testApp.server.mcpAdminUpdateVersion(ctx, adminReq, mcpAdminUpdateVersionInput{AppID: other.ID, VersionID: first.ID, Changelog: "Wrong app"}); err == nil {
		t.Fatal("edited a version belonging to another app")
	}
	if _, _, err := testApp.server.mcpAdminDeleteVersion(ctx, adminReq, mcpAdminDeleteVersionInput{AppID: other.ID, VersionID: first.ID, Confirm: true}); err == nil {
		t.Fatal("deleted version belonging to another app")
	}
	if _, _, err := testApp.server.mcpAdminDeleteVersion(ctx, userReq, mcpAdminDeleteVersionInput{AppID: record.ID, VersionID: first.ID, Confirm: true}); err == nil {
		t.Fatal("USER token deleted version")
	}
	if _, _, err := testApp.server.mcpAdminDeleteVersion(ctx, adminReq, mcpAdminDeleteVersionInput{AppID: record.ID, VersionID: first.ID}); err == nil {
		t.Fatal("deleted version without confirmation")
	}
	_, deleted, err := testApp.server.mcpAdminDeleteVersion(ctx, adminReq, mcpAdminDeleteVersionInput{AppID: record.ID, VersionID: first.ID, Confirm: true})
	if err != nil || deleted.DeletedVersion.Version.ID != first.ID {
		t.Fatalf("delete version = %+v, err = %v", deleted, err)
	}
	if _, _, err := testApp.server.mcpAdminSetVersionRetention(ctx, userReq, mcpAdminSetVersionRetentionInput{AppID: record.ID, Mode: "CUSTOM", MaxVersions: new(0), Confirm: true}); err == nil {
		t.Fatal("USER token changed retention")
	}
	for _, input := range []mcpAdminSetVersionRetentionInput{
		{AppID: record.ID, Mode: "CUSTOM", MaxVersions: new(-1), Confirm: true},
		{AppID: record.ID, Mode: "INHERIT", MaxVersions: new(1), Confirm: true},
		{AppID: record.ID, Mode: "CUSTOM", Confirm: true},
		{AppID: record.ID, Mode: "CUSTOM", MaxVersions: new(1)},
	} {
		if _, _, err := testApp.server.mcpAdminSetVersionRetention(ctx, adminReq, input); err == nil {
			t.Fatalf("accepted invalid retention %+v", input)
		}
	}
	_, policy, err := testApp.server.mcpAdminSetVersionRetention(ctx, adminReq, mcpAdminSetVersionRetentionInput{AppID: record.ID, Mode: "CUSTOM", MaxVersions: new(0), Confirm: true})
	if err != nil || policy.VersionRetention.EffectiveMaxVersions != 0 || !testApp.server.db.AppVersion.Query().Where(appversion.IDEQ(second.ID)).ExistX(ctx) {
		t.Fatalf("retention = %+v, err = %v", policy, err)
	}
	old := testApp.server.db.AppVersion.Create().SetAppID(record.ID).SetUploaderID(admin.ID).SetVersion("0.9.0").
		SetStatus(appversion.StatusAPPROVED).SetPublishedAt(time.Now().Add(-time.Hour)).SaveX(ctx)
	_, policy, err = testApp.server.mcpAdminSetVersionRetention(ctx, adminReq, mcpAdminSetVersionRetentionInput{AppID: record.ID, Mode: "CUSTOM", MaxVersions: new(1), Confirm: true})
	if err != nil || len(policy.PrunedVersions) != 1 || policy.PrunedVersions[0].Version.ID != old.ID {
		t.Fatalf("prune = %+v, err = %v", policy, err)
	}
	if testApp.server.db.AppVersion.Query().Where(appversion.IDEQ(old.ID)).ExistX(ctx) {
		t.Fatal("pruned version still exists")
	}
}

func TestMCPAppManagementToolRegistration(t *testing.T) {
	testApp := newTestApp(t)
	for _, principal := range []mcptoken.PrincipalType{mcptoken.PrincipalTypeUSER, mcptoken.PrincipalTypeADMIN} {
		server := testApp.server.newMCPServer(t.Context(), principal)
		clientTransport, serverTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(t.Context(), serverTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = serverSession.Close() })
		session, err := mcp.NewClient(testApp.server.mcpImplementation(t.Context()), nil).Connect(t.Context(), clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		listed, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"appstore_admin_get_app", "appstore_admin_update_app", "appstore_admin_delete_app", "appstore_admin_unlist_app", "appstore_admin_list_versions", "appstore_admin_update_version", "appstore_admin_delete_version", "appstore_admin_set_version_retention"} {
			present := slices.ContainsFunc(listed.Tools, func(tool *mcp.Tool) bool { return tool.Name == name })
			if present != (principal == mcptoken.PrincipalTypeADMIN) {
				t.Errorf("%s exposed to %s = %v", name, principal, present)
			}
		}
		for _, tool := range listed.Tools {
			if !slices.Contains([]string{"appstore_admin_get_app", "appstore_admin_delete_app", "appstore_admin_delete_version", "appstore_admin_set_version_retention"}, tool.Name) {
				continue
			}
			raw, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatal(err)
			}
			if slices.Contains(schema.Required, "appId") || slices.Contains(schema.Required, "packageId") {
				t.Errorf("%s schema prevents selecting an app by either identity: %s", tool.Name, raw)
			}
			if tool.Name != "appstore_admin_get_app" && !slices.Contains(schema.Required, "confirm") {
				t.Errorf("%s schema does not require confirmation: %s", tool.Name, raw)
			}
		}
	}
}
