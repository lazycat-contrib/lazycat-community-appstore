package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lazycat.community/appstore/ent/downstreamclientuser"
	"lazycat.community/appstore/ent/user"
)

func TestDownstreamClientObservationMergesSources(t *testing.T) {
	app := newTestApp(t)
	first := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	later := first.Add(time.Hour)
	app.server.setNow(func() time.Time { return first })
	if err := app.server.observeDownstreamClient(t.Context(), "lc_abc123", "Alice", downstreamSeenComment); err != nil {
		t.Fatal(err)
	}
	app.server.setNow(func() time.Time { return later })
	if err := app.server.observeDownstreamClient(t.Context(), "lc_abc123", "Alice L.", downstreamSeenWish); err != nil {
		t.Fatal(err)
	}
	records := app.server.db.DownstreamClientUser.Query().AllX(t.Context())
	if len(records) != 1 || records[0].DisplayName != "Alice L." || !records[0].SeenInComments || !records[0].SeenInWishes || !records[0].FirstSeenAt.Equal(first) || !records[0].LastSeenAt.Equal(later) {
		t.Fatalf("observation not merged: %#v", records)
	}
}

func TestDownstreamClientBlockPermissionsAndEnforcement(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	app.server.cfg.TrustLazyCatClientChat = true
	if err := app.server.observeDownstreamClient(t.Context(), "lc_blocked", "Spammer", downstreamSeenWish); err != nil {
		t.Fatal(err)
	}

	softwareAdmin := app.server.db.User.Create().SetUsername("software-admin").SetPasswordHash("unused").SetRole(user.RoleSOFTWARE_ADMIN).SetEmailVerified(true).SaveX(t.Context())
	app.cookies = []*http.Cookie{app.serverCookieFor(softwareAdmin.ID)}
	rec := app.do(http.MethodGet, "/api/v1/admin/downstream-clients", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lc_blocked") {
		t.Fatalf("software admin list status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = app.do(http.MethodPost, "/api/v1/admin/downstream-clients/lc_blocked/block", map[string]string{"reason": "spam"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("software admin block status=%d body=%s", rec.Code, rec.Body.String())
	}

	app.cookies = nil
	app.login("admin", "changeme")
	rec = app.do(http.MethodPost, "/api/v1/admin/downstream-clients/lc_blocked/block", map[string]string{"reason": "Repeated spam"})
	if rec.Code != http.StatusOK {
		t.Fatalf("site admin block status=%d body=%s", rec.Code, rec.Body.String())
	}
	record := app.server.db.DownstreamClientUser.Query().Where(downstreamclientuser.ClientUserIDEQ("lc_blocked")).OnlyX(t.Context())
	if !record.Blocked || record.BlockReason != "Repeated spam" || record.BlockedBy == nil || record.BlockedAt == nil {
		t.Fatalf("missing block audit: %#v", record)
	}

	req := httptest.NewRequest(http.MethodGet, "/source/v2/index.json", nil)
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
	req.Header.Set("X-LazyCat-Client-User-ID", "lc_blocked")
	feedRec := httptest.NewRecorder()
	app.handler.ServeHTTP(feedRec, req)
	if feedRec.Code != http.StatusForbidden || !strings.Contains(feedRec.Body.String(), `"code":"CLIENT_BLOCKED"`) {
		t.Fatalf("blocked feed status=%d body=%s", feedRec.Code, feedRec.Body.String())
	}

	blockedWish := wishClientRequest(t, app, http.MethodGet, "/api/v1/wishes", nil, "lc_blocked")
	if blockedWish.Code != http.StatusForbidden || !strings.Contains(blockedWish.Body.String(), `"code":"CLIENT_BLOCKED"`) {
		t.Fatalf("blocked wish status=%d body=%s", blockedWish.Code, blockedWish.Body.String())
	}

	for _, endpoint := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/apps/999/comments", ""},
		{http.MethodPost, "/api/v1/apps/999/comments", `{"body":"blocked"}`},
		{http.MethodDelete, "/api/v1/comments/999", ""},
		{http.MethodPost, "/api/v1/apps/999/outdated-marks", `{"note":"blocked"}`},
		{http.MethodGet, "/api/v1/chat/conversations", ""},
	} {
		req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
		req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
		req.Header.Set("X-LazyCat-Client-User-ID", "lc_blocked")
		req.Header.Set("X-LazyCat-Client-Device-ID", "device-1")
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"code":"CLIENT_BLOCKED"`) {
			t.Fatalf("blocked %s %s status=%d body=%s", endpoint.method, endpoint.path, rec.Code, rec.Body.String())
		}
	}

	rec = app.do(http.MethodPost, "/api/v1/admin/downstream-clients/lc_blocked/unblock", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("unblock status=%d body=%s", rec.Code, rec.Body.String())
	}
	record = app.server.db.DownstreamClientUser.Query().Where(downstreamclientuser.ClientUserIDEQ("lc_blocked")).OnlyX(t.Context())
	if record.Blocked || record.BlockReason != "" || record.BlockedBy != nil || record.BlockedAt != nil || !record.SeenInWishes {
		t.Fatalf("bad unblock state: %#v", record)
	}
}
