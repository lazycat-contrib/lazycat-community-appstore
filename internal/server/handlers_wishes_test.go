package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/wish"
	"lazycat.community/appstore/ent/wishstatusevent"
)

func wishClientRequestOnDevice(t *testing.T, app *testApp, method, path string, body any, clientUserID, deviceID string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
	req.Header.Set("X-LazyCat-Client-Device-ID", deviceID)
	req.Header.Set("X-LazyCat-Client-User-ID", clientUserID)
	req.Header.Set("X-LazyCat-Client-Display-Name", "LazyCat User")
	rec := httptest.NewRecorder()
	app.handler.ServeHTTP(rec, req)
	return rec
}

func wishClientRequest(t *testing.T, app *testApp, method, path string, body any, clientUserID string) *httptest.ResponseRecorder {
	t.Helper()
	return wishClientRequestOnDevice(t, app, method, path, body, clientUserID, "device-1")
}

func createWishForTest(t *testing.T, app *testApp, kind, clientUserID string, extra map[string]any) int {
	t.Helper()
	input := map[string]any{"kind": kind, "title": "A useful wish", "body": "Please consider this request", "statusText": "Initial request details"}
	for key, value := range extra {
		input[key] = value
	}
	rec := wishClientRequest(t, app, http.MethodPost, "/api/v1/wishes", input, clientUserID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create wish status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Wish struct {
			ID int `json:"id"`
		} `json:"wish"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Wish.ID
}

func TestWishCreateRequiresStatusTextAndCreatesHistory(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	rec := wishClientRequest(t, app, http.MethodPost, "/api/v1/wishes", map[string]any{
		"kind": "APP_REQUEST", "title": "Paperless", "body": "Package Paperless", "statusText": " ",
	}, "lc_client_a")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("blank status status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := app.server.db.Wish.Query().CountX(t.Context()); got != 0 {
		t.Fatalf("wishes=%d want 0", got)
	}

	id := createWishForTest(t, app, "APP_REQUEST", "lc_client_a", nil)
	created := app.server.db.Wish.GetX(t.Context(), id)
	if created.ClientUserID != "lc_client_a" || created.OwnerID != scopedWishOwnerID("lc_client_a", "device-1") {
		t.Fatalf("wish identities moderation=%q owner=%q", created.ClientUserID, created.OwnerID)
	}
	record := app.server.db.Wish.GetX(t.Context(), id)
	events := app.server.db.WishStatusEvent.Query().AllX(t.Context())
	if record.Status != wish.StatusOPEN || len(events) != 1 || events[0].ToStatus != wishstatusevent.ToStatusOPEN || strings.TrimSpace(events[0].Text) == "" {
		t.Fatalf("wish/event not initialized record=%#v events=%#v", record, events)
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodPost, "/api/v1/wishes", map[string]any{"kind": "APP_REQUEST", "title": "x", "body": "x", "statusText": "x"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("server session submit status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWishVisibilityPrivacyAndOwnManagement(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	suggestionID := createWishForTest(t, app, "SUGGESTION", "lc_client_a", nil)
	createWishForTest(t, app, "APP_REQUEST", "lc_client_a", map[string]any{"referenceUrl": "https://example.com/app"})
	createWishForTest(t, app, "CUSTOMIZATION", "lc_client_a", map[string]any{"contactEmail": "owner@example.com", "contactOther": "Matrix @owner"})
	createWishForTest(t, app, "SUGGESTION", "lc_client_b", nil)

	rec := app.do(http.MethodGet, "/api/v1/wishes?pageSize=100", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, `"kind":"SUGGESTION"`) || strings.Contains(body, "owner@example.com") || strings.Contains(body, "lc_client_a") || strings.Contains(body, `"canManage":true`) {
		t.Fatalf("public privacy response status=%d body=%s", rec.Code, body)
	}
	rec = wishClientRequest(t, app, http.MethodGet, "/api/v1/wishes?pageSize=100", nil, "lc_client_a")
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"kind":"SUGGESTION"`) != 1 || !strings.Contains(rec.Body.String(), "owner@example.com") || !strings.Contains(rec.Body.String(), `"canManage":true`) {
		t.Fatalf("owner list status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = wishClientRequest(t, app, http.MethodPatch, "/api/v1/wishes/"+strconv.Itoa(suggestionID), map[string]any{
		"title": "Updated title", "body": "Updated details",
	}, "lc_client_a")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Updated title") {
		t.Fatalf("owner update status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = wishClientRequest(t, app, http.MethodDelete, "/api/v1/wishes/"+strconv.Itoa(suggestionID), nil, "lc_client_b")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other delete status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = wishClientRequest(t, app, http.MethodDelete, "/api/v1/wishes/"+strconv.Itoa(suggestionID), nil, "lc_client_a")
	if rec.Code != http.StatusOK || app.server.db.WishStatusEvent.Query().Where(wishstatusevent.WishIDEQ(suggestionID)).CountX(t.Context()) != 0 {
		t.Fatalf("owner delete status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWishOwnershipIsDeviceScopedButBlockingIsUserScoped(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	id := createWishForTest(t, app, "APP_REQUEST", "lc_client_a", nil)
	created := app.server.db.Wish.GetX(t.Context(), id)

	rec := wishClientRequestOnDevice(t, app, http.MethodGet, "/api/v1/wishes?pageSize=100", nil, "lc_client_a", "device-2")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canManage":false`) {
		t.Fatalf("other device list status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = wishClientRequestOnDevice(t, app, http.MethodPatch, "/api/v1/wishes/"+strconv.Itoa(id), map[string]any{
		"title": "Cross-device update", "body": "Must not be accepted",
	}, "lc_client_a", "device-2")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other device update status=%d body=%s", rec.Code, rec.Body.String())
	}

	record := app.server.db.DownstreamClientUser.Query().OnlyX(t.Context())
	if record.ClientUserID != created.ClientUserID {
		t.Fatalf("moderation identity=%q wish identity=%q", record.ClientUserID, created.ClientUserID)
	}
	app.server.db.DownstreamClientUser.UpdateOne(record).SetBlocked(true).SaveX(t.Context())
	rec = wishClientRequestOnDevice(t, app, http.MethodGet, "/api/v1/wishes?pageSize=100", nil, "lc_client_a", "device-2")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CLIENT_BLOCKED") {
		t.Fatalf("blocked other device status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLegacyWishKeepsModerationIdentityWithoutGrantingDeviceOwnership(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	now := app.server.currentTime().UTC()
	legacy := app.server.db.Wish.Create().
		SetKind(wish.KindSUGGESTION).
		SetTitle("Legacy private wish").
		SetBody("Created before device ownership was recorded").
		SetClientUserID("lc_client_a").
		SetAuthorName("Legacy user").
		SetCreatedAt(now).
		SetUpdatedAt(now).
		SetLastActivityAt(now).
		SaveX(t.Context())

	rec := wishClientRequest(t, app, http.MethodGet, "/api/v1/wishes/"+strconv.Itoa(legacy.ID), nil, "lc_client_a")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"canManage":false`) || !strings.Contains(rec.Body.String(), `"clientUserId":"lc_client_a"`) {
		t.Fatalf("legacy client view status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = wishClientRequest(t, app, http.MethodGet, "/api/v1/wishes?pageSize=100", nil, "lc_client_a")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Legacy private wish") || !strings.Contains(rec.Body.String(), `"canManage":false`) {
		t.Fatalf("legacy owner list status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = wishClientRequest(t, app, http.MethodGet, "/api/v1/wishes/"+strconv.Itoa(legacy.ID), nil, "lc_client_b")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy other-user view status=%d body=%s", rec.Code, rec.Body.String())
	}

	app.login("admin", "changeme")
	rec = app.do(http.MethodGet, "/api/v1/wishes/"+strconv.Itoa(legacy.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"clientUserId":"lc_client_a"`) {
		t.Fatalf("legacy admin view status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestScopedWishOwnerID(t *testing.T) {
	first := scopedWishOwnerID("lc_client_a", "device-1")
	sameDevice := scopedWishOwnerID("lc_client_a", "device-1")
	otherDevice := scopedWishOwnerID("lc_client_a", "device-2")
	otherUser := scopedWishOwnerID("lc_client_b", "device-1")
	if first == "" || first != sameDevice || first == otherDevice || first == otherUser {
		t.Fatalf("owner identities first=%q same=%q otherDevice=%q otherUser=%q", first, sameDevice, otherDevice, otherUser)
	}
}

func TestWishAdminReplyAndStatusHistory(t *testing.T) {
	app := newTestApp(t)
	app.server.cfg.TrustLazyCatClientComments = true
	id := createWishForTest(t, app, "APP_REQUEST", "lc_client_a", nil)
	app.login("admin", "changeme")
	rec := app.do(http.MethodPost, "/api/v1/admin/wishes/"+strconv.Itoa(id)+"/replies", map[string]string{"body": "We are reviewing this"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("reply status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, transition := range []struct{ status, text string }{{"PLANNED", "Added to roadmap"}, {"IN_PROGRESS", "Packaging started"}} {
		rec = app.do(http.MethodPost, "/api/v1/admin/wishes/"+strconv.Itoa(id)+"/status", map[string]string{"status": transition.status, "statusText": transition.text})
		if rec.Code != http.StatusOK {
			t.Fatalf("status transition=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	events := app.server.db.WishStatusEvent.Query().Where(wishstatusevent.WishIDEQ(id)).Order(ent.Asc(wishstatusevent.FieldID)).AllX(t.Context())
	if len(events) != 3 || events[0].ToStatus != wishstatusevent.ToStatusOPEN || events[1].ToStatus != wishstatusevent.ToStatusPLANNED || events[2].ToStatus != wishstatusevent.ToStatusIN_PROGRESS {
		t.Fatalf("unexpected status history %#v", events)
	}
	rec = app.do(http.MethodPost, "/api/v1/admin/wishes/"+strconv.Itoa(id)+"/status", map[string]string{"status": "COMPLETED", "statusText": " "})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("blank transition status=%d body=%s", rec.Code, rec.Body.String())
	}
}
