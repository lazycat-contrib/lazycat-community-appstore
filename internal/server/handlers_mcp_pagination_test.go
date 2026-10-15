package server

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/catalogmeta"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPAppListsIncludeSoftwareDescriptions(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	const packageID = "community.lazycat.mcp-description-test"
	store.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID(packageID).
		SetName("Description test").
		SetSlug("mcp-description-test").
		SetSummary("Short summary").
		SetSummaryI18nJSON(catalogmeta.EncodeLocalizedText(catalogmeta.LocalizedText{"zh-CN": "简短说明"})).
		SetDescription("Full software description").
		SetDescriptionI18nJSON(catalogmeta.EncodeLocalizedText(catalogmeta.LocalizedText{"zh-CN": "完整软件说明"})).
		SaveX(ctx)

	for _, tc := range []struct {
		name string
		list func() (mcpAppsOutput, error)
	}{
		{"user", func() (mcpAppsOutput, error) {
			_, out, err := store.server.mcpListMyApps(ctx, mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER), mcpListAppsInput{Query: packageID})
			return out, err
		}},
		{"admin", func() (mcpAppsOutput, error) {
			_, out, err := store.server.mcpAdminListApps(ctx, mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN), mcpAdminListAppsInput{Query: packageID})
			return out, err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.list()
			if err != nil || len(out.Apps) != 1 {
				t.Fatalf("list = %+v, err = %v", out, err)
			}
			got := out.Apps[0]
			if got.Summary != "Short summary" || got.Description != "Full software description" || got.SummaryI18n["zh-CN"] != "简短说明" || got.DescriptionI18n["zh-CN"] != "完整软件说明" {
				t.Fatalf("software descriptions = %+v", got)
			}
		})
	}
}

func TestMCPAppListToolsExposePageParameter(t *testing.T) {
	store := newTestApp(t)
	for _, tc := range []struct {
		principal mcptoken.PrincipalType
		toolName  string
	}{
		{mcptoken.PrincipalTypeUSER, "appstore_list_my_apps"},
		{mcptoken.PrincipalTypeADMIN, "appstore_admin_list_apps"},
	} {
		server := store.server.newMCPServer(t.Context(), tc.principal)
		clientTransport, serverTransport := mcp.NewInMemoryTransports()
		if _, err := server.Connect(t.Context(), serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		client := mcp.NewClient(store.server.mcpImplementation(t.Context()), nil)
		session, err := client.Connect(t.Context(), clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		tools, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, tool := range tools.Tools {
			if tool.Name != tc.toolName {
				continue
			}
			found = true
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(schema), `"page"`) || !strings.Contains(string(schema), `"limit"`) {
				t.Errorf("%s input schema omits pagination: %s", tc.toolName, schema)
			}
		}
		if !found {
			t.Errorf("%s is not listed for %s", tc.toolName, tc.principal)
		}
		_ = session.Close()
	}
}

func TestMCPAppListsPageThroughFilteredResults(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	other := store.server.db.User.Create().SetUsername("mcp-page-owner").SetPasswordHash("test").SaveX(ctx)
	adminRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userRequest := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	updatedAt := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	ids := make([]int, 0, 7)
	for index := range 7 {
		ownerID := admin.ID
		if index >= 5 {
			ownerID = other.ID
		}
		record := store.server.db.App.Create().
			SetOwnerID(ownerID).
			SetPackageID(fmt.Sprintf("community.lazycat.mcp-page-%d", index)).
			SetName(fmt.Sprintf("MCP-PAGE-%d", index)).
			SetSlug(fmt.Sprintf("mcp-page-%d", index)).
			SetUpdatedAt(updatedAt).
			SaveX(ctx)
		ids = append(ids, record.ID)
	}
	store.server.db.Collaborator.Create().SetAppID(ids[5]).SetUserID(admin.ID).SaveX(ctx)

	var gotIDs []int
	for page := 1; page <= 4; page++ {
		_, result, err := store.server.mcpAdminListApps(ctx, adminRequest, mcpAdminListAppsInput{Query: "MCP-PAGE-", Page: page, Limit: 3})
		if err != nil {
			t.Fatal(err)
		}
		if result.Pagination.Page != page || result.Pagination.PageSize != 3 || result.Pagination.TotalItems != 7 || result.Pagination.TotalPages != 3 {
			t.Fatalf("page %d metadata = %+v", page, result.Pagination)
		}
		wantNext := 0
		if page < 3 {
			wantNext = page + 1
		}
		if result.NextPage != wantNext {
			t.Fatalf("page %d nextPage = %d, want %d", page, result.NextPage, wantNext)
		}
		for _, record := range result.Apps {
			gotIDs = append(gotIDs, record.ID)
		}
	}
	slices.Reverse(ids)
	if !slices.Equal(gotIDs, ids) {
		t.Fatalf("paged app IDs = %v, want %v", gotIDs, ids)
	}

	_, myFirst, err := store.server.mcpListMyApps(ctx, userRequest, mcpListAppsInput{Query: "MCP-PAGE-", Limit: 2})
	if err != nil || myFirst.Pagination.Page != 1 || myFirst.Pagination.TotalItems != 6 || myFirst.NextPage != 2 || len(myFirst.Apps) != 2 {
		t.Fatalf("first user page = %+v, err = %v", myFirst, err)
	}
	_, mySecond, err := store.server.mcpListMyApps(ctx, userRequest, mcpListAppsInput{Query: "MCP-PAGE-", Page: 2, Limit: 2})
	if err != nil || mySecond.Pagination.Page != 2 || mySecond.NextPage != 3 || len(mySecond.Apps) != 2 {
		t.Fatalf("second user page = %+v, err = %v", mySecond, err)
	}
	if _, _, err := store.server.mcpAdminListApps(ctx, userRequest, mcpAdminListAppsInput{Page: 2}); err == nil {
		t.Fatal("user token accessed admin pagination")
	}
	if _, _, err := store.server.mcpAdminListApps(ctx, adminRequest, mcpAdminListAppsInput{Page: -1}); err == nil {
		t.Fatal("accepted negative page")
	}
	if _, _, err := store.server.mcpAdminListApps(ctx, adminRequest, mcpAdminListAppsInput{Page: math.MaxInt, Limit: 200}); err == nil {
		t.Fatal("accepted overflowing page offset")
	}
}

func TestAdminMCPAppListCanReadPastTwoHundred(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	request := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	builders := make([]*entgo.AppCreate, 0, 201)
	for index := range 201 {
		builders = append(builders, store.server.db.App.Create().
			SetOwnerID(admin.ID).
			SetPackageID(fmt.Sprintf("community.lazycat.mcp-bulk-%d", index)).
			SetName(fmt.Sprintf("MCP-BULK-%d", index)).
			SetSlug(fmt.Sprintf("mcp-bulk-%d", index)))
	}
	store.server.db.App.CreateBulk(builders...).SaveX(ctx)
	_, result, err := store.server.mcpAdminListApps(ctx, request, mcpAdminListAppsInput{Query: "MCP-BULK-", Page: 2, Limit: 999})
	if err != nil || len(result.Apps) != 1 || result.Pagination.PageSize != 200 || result.Pagination.TotalItems != 201 || result.NextPage != 0 {
		t.Fatalf("admin page after first 200 = %+v, err = %v", result, err)
	}
}
