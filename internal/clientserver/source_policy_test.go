package clientserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

type rejectingSourceURLPolicy struct{}

func (rejectingSourceURLPolicy) Validate(context.Context, clientIdentity, *url.URL) error {
	return errors.New("source rejected by policy")
}

func TestAllowSourceURLPolicyPermitsPrivateAndPublicHTTP(t *testing.T) {
	policy := allowSourceURLPolicy{}
	for _, raw := range []string{
		"http://127.0.0.1/source/v2/index.json",
		"http://[::1]/source/v2/index.json",
		"http://192.168.1.10/source/v2/index.json",
		"https://store.example/source/v2/index.json",
	} {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := policy.Validate(t.Context(), clientIdentity{}, target); err != nil {
			t.Fatalf("Validate(%q) error = %v", raw, err)
		}
	}
}

func TestRejectingSourceURLPolicyPreventsCreateAndUpdate(t *testing.T) {
	app := testServer(t)
	app.server.sourcePolicy = rejectingSourceURLPolicy{}
	body := `{"name":"Private","url":"http://127.0.0.1/source/v2/index.json","password":"","groupCodes":[]}`
	rec := app.request(http.MethodPost, "/api/client/v1/sources", body, "alice")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "SOURCE_URL_NOT_ALLOWED") {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	if count, err := app.server.db.ClientSource.Query().Count(t.Context()); err != nil || count != 0 {
		t.Fatalf("source count = %d, err = %v", count, err)
	}

	app.server.sourcePolicy = allowSourceURLPolicy{}
	rec = app.request(http.MethodPost, "/api/client/v1/sources", body, "alice")
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed create status=%d body=%s", rec.Code, rec.Body.String())
	}
	record, err := app.server.db.ClientSource.Query().Only(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	app.server.sourcePolicy = rejectingSourceURLPolicy{}
	updateBody := `{"name":"Changed","url":"https://changed.example/source/v2/index.json","password":"","groupCodes":[]}`
	rec = app.request(http.MethodPatch, "/api/client/v1/sources/"+strconv.Itoa(record.ID), updateBody, "alice")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("update status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := app.server.db.ClientSource.Get(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != record.URL {
		t.Fatalf("source URL changed to %q", got.URL)
	}
}

func TestSourceUpdateKeepsOrClearsValidatorByRequestIdentity(t *testing.T) {
	app := testServer(t)
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Private").
		SetURL("https://store.example/source/v2/index.json").
		SetPassword("secret").
		SetGroupCodesJSON(`["ABC123"]`).
		SetGroupNamesJSON(`[{"name":"Private","code":"ABC123"}]`).
		SetLastInvalidGroupCodesJSON(`["OLD999"]`).
		SetLastEtag(`W/"feed-v1"`).
		SaveX(t.Context())

	preferenceOnly := `{"name":"Renamed","url":"https://store.example/source/v2/index.json","password":"secret","groupCodes":["ABC123"],"chatEnabled":false}`
	rec := app.request(http.MethodPatch, "/api/client/v1/sources/"+strconv.Itoa(source.ID), preferenceOnly, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("preference update status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored := app.server.db.ClientSource.GetX(t.Context(), source.ID)
	if stored.LastEtag != `W/"feed-v1"` || stored.GroupNamesJSON == "" || stored.LastInvalidGroupCodesJSON == "" {
		t.Fatalf("preference update cleared feed state: ETag=%q groups=%q invalid=%q", stored.LastEtag, stored.GroupNamesJSON, stored.LastInvalidGroupCodesJSON)
	}

	passwordChange := `{"name":"Renamed","url":"https://store.example/source/v2/index.json","password":"rotated","groupCodes":["ABC123"]}`
	rec = app.request(http.MethodPatch, "/api/client/v1/sources/"+strconv.Itoa(source.ID), passwordChange, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("password update status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored = app.server.db.ClientSource.GetX(t.Context(), source.ID)
	if stored.LastEtag != "" || stored.GroupNamesJSON == "" {
		t.Fatalf("password update validator/groups = %q / %q", stored.LastEtag, stored.GroupNamesJSON)
	}

	app.server.db.ClientSource.UpdateOneID(source.ID).SetLastEtag(`W/"feed-v2"`).SaveX(t.Context())
	groupChange := `{"name":"Renamed","url":"https://store.example/source/v2/index.json","password":"rotated","groupCodes":["NEW111"]}`
	rec = app.request(http.MethodPatch, "/api/client/v1/sources/"+strconv.Itoa(source.ID), groupChange, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("group update status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored = app.server.db.ClientSource.GetX(t.Context(), source.ID)
	if stored.LastEtag != "" || stored.GroupNamesJSON != "" || stored.LastInvalidGroupCodesJSON != "" {
		t.Fatalf("group update retained stale state: ETag=%q groups=%q invalid=%q", stored.LastEtag, stored.GroupNamesJSON, stored.LastInvalidGroupCodesJSON)
	}
}
