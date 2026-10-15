package server

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/catalogmeta"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAdminMCPCategoryToolsManageHierarchy(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	adminRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	_, baseline, err := app.server.mcpAdminListCategories(ctx, adminRequest, struct{}{})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := app.server.mcpAdminListCategories(ctx, userRequest, struct{}{}); err == nil || !strings.Contains(err.Error(), "admin MCP token") {
		t.Fatalf("user token list error = %v", err)
	}
	if _, _, err := app.server.mcpAdminCreateCategory(ctx, userRequest, mcpCreateCategoryInput{Name: "Denied"}); err == nil {
		t.Fatal("user token created a category")
	}

	_, root, err := app.server.mcpAdminCreateCategory(ctx, adminRequest, mcpCreateCategoryInput{
		NameI18n:  catalogmeta.LocalizedText{"zh-CN": "工具", "en": "Tools"},
		SortOrder: new(2),
	})
	if err != nil || root.Category.Name != "工具" || root.Category.Slug == "" || root.Category.SortOrder != 2 {
		t.Fatalf("create root = %+v, err = %v", root, err)
	}
	_, child, err := app.server.mcpAdminCreateCategory(ctx, adminRequest, mcpCreateCategoryInput{
		Name: "下载工具", Slug: "downloads", ParentID: new(root.Category.ID),
	})
	if err != nil || child.Category.ParentID == nil || *child.Category.ParentID != root.Category.ID {
		t.Fatalf("create child = %+v, err = %v", child, err)
	}
	if _, _, err := app.server.mcpAdminCreateCategory(ctx, adminRequest, mcpCreateCategoryInput{
		Name: "Too deep", ParentID: new(child.Category.ID),
	}); err == nil {
		t.Fatal("created a third category level")
	}
	if _, _, err := app.server.mcpAdminDeleteCategory(ctx, adminRequest, mcpDeleteCategoryInput{ID: root.Category.ID}); err == nil {
		t.Fatal("deleted category with a child")
	}

	_, list, err := app.server.mcpAdminListCategories(ctx, adminRequest, struct{}{})
	if err != nil || len(list.Categories) != len(baseline.Categories)+2 {
		t.Fatalf("list categories = %+v, err = %v", list, err)
	}
	_, updated, err := app.server.mcpAdminUpdateCategory(ctx, adminRequest, mcpUpdateCategoryInput{
		ID: child.Category.ID, Name: "下载", ClearParent: true, SortOrder: new(1),
	})
	if err != nil || updated.Category.Name != "下载" || updated.Category.ParentID != nil || updated.Category.SortOrder != 1 {
		t.Fatalf("update category = %+v, err = %v", updated, err)
	}
	if _, _, err := app.server.mcpAdminUpdateCategory(ctx, adminRequest, mcpUpdateCategoryInput{
		ID: root.Category.ID, ParentID: new(child.Category.ID), ClearParent: true,
	}); err == nil {
		t.Fatal("accepted conflicting parent update")
	}
	if _, _, err := app.server.mcpAdminDeleteCategory(ctx, userRequest, mcpDeleteCategoryInput{ID: child.Category.ID}); err == nil {
		t.Fatal("user token deleted a category")
	}
	linkedApp := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetCategoryID(child.Category.ID).
		SetPackageID("community.lazycat.mcp-category-test").
		SetName("MCP category test").
		SetSlug("mcp-category-test").
		SaveX(ctx)
	if _, result, err := app.server.mcpAdminDeleteCategory(ctx, adminRequest, mcpDeleteCategoryInput{ID: child.Category.ID}); err != nil || !result.OK {
		t.Fatalf("delete child = %+v, err = %v", result, err)
	}
	if got := app.server.db.App.GetX(ctx, linkedApp.ID).CategoryID; got != nil {
		t.Fatalf("deleted category still linked to app: %d", *got)
	}
	if _, result, err := app.server.mcpAdminDeleteCategory(ctx, adminRequest, mcpDeleteCategoryInput{ID: root.Category.ID}); err != nil || !result.OK {
		t.Fatalf("delete root = %+v, err = %v", result, err)
	}
	if _, list, err := app.server.mcpAdminListCategories(ctx, adminRequest, struct{}{}); err != nil || len(list.Categories) != len(baseline.Categories) {
		t.Fatalf("categories after delete = %+v, err = %v", list, err)
	}
}

func TestAdminMCPCategoryToolsAreNotListedForUserTokens(t *testing.T) {
	app := newTestApp(t)
	for _, tc := range []struct {
		principal mcptoken.PrincipalType
		want      bool
	}{
		{mcptoken.PrincipalTypeUSER, false},
		{mcptoken.PrincipalTypeADMIN, true},
	} {
		server := app.server.newMCPServer(t.Context(), tc.principal)
		clientTransport, serverTransport := mcp.NewInMemoryTransports()
		if _, err := server.Connect(t.Context(), serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		client := mcp.NewClient(app.server.mcpImplementation(t.Context()), nil)
		session, err := client.Connect(t.Context(), clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		tools, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		for _, name := range []string{
			"appstore_admin_list_categories",
			"appstore_admin_create_category",
			"appstore_admin_update_category",
			"appstore_admin_delete_category",
		} {
			if got := slices.Contains(names, name); got != tc.want {
				t.Errorf("%s listed for %s = %v, want %v", name, tc.principal, got, tc.want)
			}
		}
		_ = session.Close()
	}
}

func mcpCategoryTestRequest(userID int, principalType mcptoken.PrincipalType) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &mcpauth.TokenInfo{
		UserID: strconv.Itoa(userID),
		Extra:  map[string]any{"principalType": string(principalType)},
	}}}
}
