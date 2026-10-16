package server

import (
	"slices"
	"strings"
	"testing"

	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/ent/wish"
	"lazycat.community/appstore/ent/wishstatusevent"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPWishOwnershipAndPrivacy(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	owner := app.server.db.User.Create().SetUsername("mcp-wish-owner").SetPasswordHash("test").SetEmailVerified(true).SaveX(ctx)
	other := app.server.db.User.Create().SetUsername("mcp-wish-other").SetPasswordHash("test").SetEmailVerified(true).SaveX(ctx)
	ownerReq := mcpCategoryTestRequest(owner.ID, mcptoken.PrincipalTypeUSER)
	otherReq := mcpCategoryTestRequest(other.ID, mcptoken.PrincipalTypeUSER)

	_, created, err := app.server.mcpCreateWish(ctx, ownerReq, createWishInput{
		Kind: wish.KindSUGGESTION, Title: "Private idea", Body: "Improve search", StatusText: "Please review",
	})
	if err != nil || created.Wish.ID <= 0 || !created.Wish.CanManage || created.Wish.Kind != wish.KindSUGGESTION || created.Wish.ClientUserID != "" {
		t.Fatalf("create private wish = %+v, err = %v", created, err)
	}
	stored := app.server.db.Wish.GetX(ctx, created.Wish.ID)
	if stored.OwnerID != mcpWishOwnerID(owner.ID) || stored.ClientUserID != mcpWishOwnerID(owner.ID) {
		t.Fatalf("wrong MCP ownership: %+v", stored)
	}
	events := app.server.db.WishStatusEvent.Query().AllX(ctx)
	if len(events) != 1 || events[0].ActorType != wishstatusevent.ActorTypeUSER || events[0].ActorUserID == nil || *events[0].ActorUserID != owner.ID {
		t.Fatalf("initial status actor = %+v", events)
	}

	_, public, err := app.server.mcpCreateWish(ctx, otherReq, createWishInput{
		Kind: wish.KindCUSTOMIZATION, Title: "Public request", Body: "Build a tool",
		ContactEmail: "other@example.com", ContactOther: "Private handle", StatusText: "Initial request",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, listed, err := app.server.mcpListWishes(ctx, ownerReq, mcpListWishesInput{Limit: 10})
	if err != nil || len(listed.Items) != 2 {
		t.Fatalf("owner list = %+v, err = %v", listed, err)
	}
	for _, item := range listed.Items {
		if item.ID == public.Wish.ID && (item.ContactEmail != "" || item.ContactOther != "" || item.ClientUserID != "") {
			t.Fatalf("other user's contact data leaked: %+v", item)
		}
	}
	_, firstPage, err := app.server.mcpListWishes(ctx, ownerReq, mcpListWishesInput{Limit: 1})
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextPage != 2 || firstPage.Pagination.TotalItems != 2 {
		t.Fatalf("first wish page = %+v, err = %v", firstPage, err)
	}
	_, secondPage, err := app.server.mcpListWishes(ctx, ownerReq, mcpListWishesInput{Limit: 1, Page: 2})
	if err != nil || len(secondPage.Items) != 1 || secondPage.NextPage != 0 || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second wish page = %+v, err = %v", secondPage, err)
	}
	_, otherList, err := app.server.mcpListWishes(ctx, otherReq, mcpListWishesInput{Kind: wish.KindSUGGESTION})
	if err != nil || len(otherList.Items) != 0 {
		t.Fatalf("other user saw private suggestion: %+v, err = %v", otherList, err)
	}
	if _, _, err := app.server.mcpGetWish(ctx, otherReq, mcpWishIDInput{ID: created.Wish.ID}); err == nil {
		t.Fatal("other user read private suggestion")
	}
	if _, _, err := app.server.mcpUpdateWish(ctx, otherReq, mcpUpdateWishInput{ID: created.Wish.ID, Title: new("Hijacked")}); err == nil {
		t.Fatal("other user edited private suggestion")
	}
	if _, _, err := app.server.mcpDeleteWish(ctx, otherReq, mcpWishIDInput{ID: created.Wish.ID}); err == nil {
		t.Fatal("other user deleted private suggestion")
	}
	if _, _, err := app.server.mcpUpdateWish(ctx, ownerReq, mcpUpdateWishInput{ID: public.Wish.ID, Title: new("Hijacked")}); err == nil {
		t.Fatal("user edited another user's public wish")
	}
	if _, _, err := app.server.mcpDeleteWish(ctx, ownerReq, mcpWishIDInput{ID: public.Wish.ID}); err == nil {
		t.Fatal("user deleted another user's public wish")
	}
	_, updated, err := app.server.mcpUpdateWish(ctx, ownerReq, mcpUpdateWishInput{ID: created.Wish.ID, Title: new("Updated idea")})
	if err != nil || updated.Wish.Title != "Updated idea" || updated.Wish.Body != "Improve search" {
		t.Fatalf("owner update = %+v, err = %v", updated, err)
	}
	if _, _, err := app.server.mcpCreateWish(ctx, ownerReq, createWishInput{Kind: wish.KindAPP_REQUEST, Title: "Missing history", Body: "Body"}); err == nil {
		t.Fatal("created wish without initial status text")
	}
	if _, result, err := app.server.mcpDeleteWish(ctx, ownerReq, mcpWishIDInput{ID: created.Wish.ID}); err != nil || !result.OK {
		t.Fatalf("owner delete = %+v, err = %v", result, err)
	}
	if got := app.server.db.WishStatusEvent.Query().CountX(ctx); got != 1 {
		t.Fatalf("deleted wish history remains; count=%d", got)
	}
}

func TestMCPAdminCanMaintainClientWishes(t *testing.T) {
	app := newTestApp(t)
	ctx := t.Context()
	app.server.cfg.TrustLazyCatClientComments = true
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	adminReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeADMIN)
	userReq := mcpCategoryTestRequest(admin.ID, mcptoken.PrincipalTypeUSER)
	id := createWishForTest(t, app, "SUGGESTION", "lc_wish_owner", nil)

	_, listed, err := app.server.mcpListWishes(ctx, adminReq, mcpListWishesInput{Kind: wish.KindSUGGESTION})
	if err != nil || len(listed.Items) != 1 || listed.Items[0].ID != id || !listed.Items[0].CanManage {
		t.Fatalf("admin list = %+v, err = %v", listed, err)
	}
	if _, _, err := app.server.mcpGetWish(ctx, userReq, mcpWishIDInput{ID: id}); err == nil {
		t.Fatal("admin account with USER token read client suggestion")
	}
	if _, _, err := app.server.mcpAdminReplyToWish(ctx, userReq, mcpWishReplyInput{ID: id, Body: "Denied"}); err == nil {
		t.Fatal("USER token replied as admin")
	}
	if _, _, err := app.server.mcpAdminSetWishStatus(ctx, userReq, mcpWishStatusInput{ID: id, Status: wish.StatusPLANNED, StatusText: "Denied"}); err == nil {
		t.Fatal("USER token changed status")
	}
	_, reply, err := app.server.mcpAdminReplyToWish(ctx, adminReq, mcpWishReplyInput{ID: id, Body: "We will review this"})
	if err != nil || reply.Reply.Body != "We will review this" {
		t.Fatalf("admin reply = %+v, err = %v", reply, err)
	}
	_, status, err := app.server.mcpAdminSetWishStatus(ctx, adminReq, mcpWishStatusInput{ID: id, Status: wish.StatusPLANNED, StatusText: "Roadmap"})
	if err != nil || status.StatusEvent.To != wishstatusevent.ToStatusPLANNED {
		t.Fatalf("admin status = %+v, err = %v", status, err)
	}
	if _, _, err := app.server.mcpAdminSetWishStatus(ctx, adminReq, mcpWishStatusInput{ID: id, Status: wish.StatusCOMPLETED, StatusText: " "}); err == nil {
		t.Fatal("accepted status update without explanation")
	}
	_, updated, err := app.server.mcpUpdateWish(ctx, adminReq, mcpUpdateWishInput{ID: id, Title: new("Moderated title")})
	if err != nil || updated.Wish.Title != "Moderated title" || len(updated.Wish.Replies) != 1 || len(updated.Wish.StatusHistory) != 2 {
		t.Fatalf("admin update = %+v, err = %v", updated, err)
	}
	if _, _, err := app.server.mcpDeleteWish(ctx, userReq, mcpWishIDInput{ID: id}); err == nil {
		t.Fatal("USER token deleted another person's wish")
	}
	if _, result, err := app.server.mcpDeleteWish(ctx, adminReq, mcpWishIDInput{ID: id}); err != nil || !result.OK {
		t.Fatalf("admin delete = %+v, err = %v", result, err)
	}
	if got := app.server.db.WishReply.Query().CountX(ctx); got != 0 {
		t.Fatalf("deleted wish still has replies: %d", got)
	}
}

func TestMCPWishToolRegistration(t *testing.T) {
	app := newTestApp(t)
	for _, tc := range []struct {
		principal mcptoken.PrincipalType
		admin     bool
	}{{mcptoken.PrincipalTypeUSER, false}, {mcptoken.PrincipalTypeADMIN, true}} {
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
		listed, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(listed.Tools))
		for _, tool := range listed.Tools {
			names = append(names, tool.Name)
		}
		for _, name := range []string{"appstore_list_wishes", "appstore_get_wish", "appstore_create_wish", "appstore_update_wish", "appstore_delete_wish"} {
			if !slices.Contains(names, name) {
				t.Errorf("%s missing from %s tools", name, tc.principal)
			}
		}
		for _, name := range []string{"appstore_admin_reply_to_wish", "appstore_admin_set_wish_status"} {
			if slices.Contains(names, name) != tc.admin {
				t.Errorf("%s presence for %s = %v, want %v; tools=%s", name, tc.principal, slices.Contains(names, name), tc.admin, strings.Join(names, ", "))
			}
		}
	}
}
