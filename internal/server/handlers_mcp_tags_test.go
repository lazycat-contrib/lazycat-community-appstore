package server

import (
	"slices"
	"testing"

	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/catalogmeta"
)

func TestAdminMCPManagesTagsAndAppTaxonomy(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	adminRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	const packageID = "community.lazycat.mcp-taxonomy-test"
	application := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID(packageID).
		SetName("MCP taxonomy test").
		SetSlug("mcp-taxonomy-test").
		SaveX(ctx)

	if _, _, err := app.server.mcpAdminCreateTag(ctx, userRequest, mcpCreateTagInput{Name: "Denied"}); err == nil {
		t.Fatal("user token created a tag")
	}
	_, first, err := app.server.mcpAdminCreateTag(ctx, adminRequest, mcpCreateTagInput{
		NameI18n: catalogmeta.LocalizedText{"zh-CN": "网络", "en": "Network"}, Slug: "mcp-network",
	})
	if err != nil || first.Tag.Name != "网络" || first.Tag.NameI18n["en"] != "Network" {
		t.Fatalf("create first tag = %+v, err = %v", first, err)
	}
	_, second, err := app.server.mcpAdminCreateTag(ctx, adminRequest, mcpCreateTagInput{Name: "Utilities", Slug: "mcp-utilities"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.server.mcpAdminCreateTag(ctx, adminRequest, mcpCreateTagInput{Name: "Duplicate", Slug: "mcp-network"}); err == nil {
		t.Fatal("created duplicate tag slug")
	}
	_, updated, err := app.server.mcpAdminUpdateTag(ctx, adminRequest, mcpUpdateTagInput{ID: first.Tag.ID, Name: "Connectivity", Slug: "mcp-connectivity"})
	if err != nil || updated.Tag.Name != "Connectivity" || updated.Tag.Slug != "mcp-connectivity" {
		t.Fatalf("update tag = %+v, err = %v", updated, err)
	}
	_, tags, err := app.server.mcpAdminListTags(ctx, adminRequest, struct{}{})
	if err != nil || !slices.ContainsFunc(tags.Tags, func(item tagDTO) bool { return item.ID == first.Tag.ID }) {
		t.Fatalf("list tags = %+v, err = %v", tags, err)
	}

	_, category, err := app.server.mcpAdminCreateCategory(ctx, adminRequest, mcpCreateCategoryInput{Name: "MCP category", Slug: "mcp-category"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.server.mcpAdminSetAppCategory(ctx, userRequest, mcpSetAppCategoryInput{AppID: application.ID, CategoryID: category.Category.ID}); err == nil {
		t.Fatal("user token changed app category")
	}
	if _, _, err := app.server.mcpAdminSetAppCategory(ctx, adminRequest, mcpSetAppCategoryInput{AppID: application.ID, CategoryID: 999999}); err == nil {
		t.Fatal("assigned nonexistent category")
	}
	_, categorized, err := app.server.mcpAdminSetAppCategory(ctx, adminRequest, mcpSetAppCategoryInput{PackageID: packageID, CategoryID: category.Category.ID})
	if err != nil || categorized.CategoryID == nil || *categorized.CategoryID != category.Category.ID {
		t.Fatalf("set category = %+v, err = %v", categorized, err)
	}
	if _, _, err := app.server.mcpAdminSetAppCategory(ctx, adminRequest, mcpSetAppCategoryInput{AppID: application.ID, CategoryID: category.Category.ID, ClearCategory: true}); err == nil {
		t.Fatal("accepted conflicting category assignment")
	}

	if _, _, err := app.server.mcpAdminSetAppTags(ctx, userRequest, mcpSetAppTagsInput{AppID: application.ID, TagIDs: []int{first.Tag.ID}}); err == nil {
		t.Fatal("user token changed app tags")
	}
	_, assigned, err := app.server.mcpAdminSetAppTags(ctx, adminRequest, mcpSetAppTagsInput{AppID: application.ID, TagIDs: []int{second.Tag.ID, first.Tag.ID, first.Tag.ID}})
	if err != nil || !slices.Equal(assigned.TagIDs, []int{first.Tag.ID, second.Tag.ID}) {
		t.Fatalf("set tags = %+v, err = %v", assigned, err)
	}
	_, listedApps, err := app.server.mcpAdminListApps(ctx, adminRequest, mcpAdminListAppsInput{Query: packageID})
	if err != nil || len(listedApps.Apps) != 1 || listedApps.Apps[0].CategoryID == nil || *listedApps.Apps[0].CategoryID != category.Category.ID || len(listedApps.Apps[0].Tags) != 2 {
		t.Fatalf("admin app summary taxonomy = %+v, err = %v", listedApps, err)
	}
	if _, _, err := app.server.mcpAdminRemoveAppTag(ctx, userRequest, mcpAppTagInput{AppID: application.ID, TagID: first.Tag.ID}); err == nil {
		t.Fatal("user token removed an app tag")
	}
	_, remaining, err := app.server.mcpAdminRemoveAppTag(ctx, adminRequest, mcpAppTagInput{AppID: application.ID, TagID: first.Tag.ID})
	if err != nil || !slices.Equal(remaining.TagIDs, []int{second.Tag.ID}) {
		t.Fatalf("remove one app tag = %+v, err = %v", remaining, err)
	}
	if _, _, err := app.server.mcpAdminAddAppTag(ctx, userRequest, mcpAppTagInput{AppID: application.ID, TagID: first.Tag.ID}); err == nil {
		t.Fatal("user token added an app tag")
	}
	_, added, err := app.server.mcpAdminAddAppTag(ctx, adminRequest, mcpAppTagInput{PackageID: packageID, TagID: first.Tag.ID})
	if err != nil || !slices.Equal(added.TagIDs, []int{first.Tag.ID, second.Tag.ID}) {
		t.Fatalf("add one app tag = %+v, err = %v", added, err)
	}
	if _, duplicate, err := app.server.mcpAdminAddAppTag(ctx, adminRequest, mcpAppTagInput{AppID: application.ID, TagID: first.Tag.ID}); err != nil || len(duplicate.TagIDs) != 2 {
		t.Fatalf("duplicate add app tag = %+v, err = %v", duplicate, err)
	}
	if _, _, err := app.server.mcpAdminSetAppTags(ctx, adminRequest, mcpSetAppTagsInput{AppID: application.ID, TagIDs: []int{first.Tag.ID, 999999}}); err == nil {
		t.Fatal("accepted nonexistent tag ID")
	}
	if count := app.server.db.AppTag.Query().Where(apptag.AppIDEQ(application.ID)).CountX(ctx); count != 2 {
		t.Fatalf("failed tag assignment removed existing tags: count = %d", count)
	}
	if _, _, err := app.server.mcpAdminSetAppTags(ctx, adminRequest, mcpSetAppTagsInput{AppID: application.ID}); err == nil {
		t.Fatal("missing tagIds cleared tags")
	}
	_, cleared, err := app.server.mcpAdminSetAppCategory(ctx, adminRequest, mcpSetAppCategoryInput{AppID: application.ID, ClearCategory: true})
	if err != nil || cleared.CategoryID != nil {
		t.Fatalf("clear category = %+v, err = %v", cleared, err)
	}
	if _, _, err := app.server.mcpAdminDeleteTag(ctx, userRequest, mcpDeleteTagInput{ID: first.Tag.ID}); err == nil {
		t.Fatal("user token deleted a tag")
	}
	if _, result, err := app.server.mcpAdminDeleteTag(ctx, adminRequest, mcpDeleteTagInput{ID: first.Tag.ID}); err != nil || !result.OK {
		t.Fatalf("delete linked tag = %+v, err = %v", result, err)
	}
	if count := app.server.db.AppTag.Query().Where(apptag.AppIDEQ(application.ID)).CountX(ctx); count != 1 {
		t.Fatalf("tag deletion did not detach app: count = %d", count)
	}
	_, empty, err := app.server.mcpAdminSetAppTags(ctx, adminRequest, mcpSetAppTagsInput{AppID: application.ID, TagIDs: []int{}})
	if err != nil || len(empty.TagIDs) != 0 || len(empty.Tags) != 0 {
		t.Fatalf("clear app tags = %+v, err = %v", empty, err)
	}
}
