package clientserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lazycat.community/appstore/internal/cfnetwork"
)

func TestCFSettingsOptionalPersistedAndUserScoped(t *testing.T) {
	app := testServer(t)
	read := func(user string) ClientSettingsDTO {
		t.Helper()
		response := app.request("GET", "/api/client/v1/settings", "", user)
		var body struct {
			Settings ClientSettingsDTO `json:"settings"`
		}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &body) != nil {
			t.Fatalf("settings: %s", response.Body.String())
		}
		return body.Settings
	}
	if got := read("alice"); got.CFEnabled || got.CFEndpoint != cfnetwork.DefaultEndpoint || len(got.CFPresets) != 1 {
		t.Fatalf("defaults=%+v", got)
	}
	response := app.request("PATCH", "/api/client/v1/settings", `{"cfEnabled":true,"cfEndpoint":"1.1.1.1"}`, "alice")
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if got := read("alice"); !got.CFEnabled || got.CFEndpoint != "1.1.1.1" {
		t.Fatalf("not saved: %+v", got)
	}
	if got := read("bob"); got.CFEnabled || got.CFEndpoint != cfnetwork.DefaultEndpoint {
		t.Fatalf("cross-user settings: %+v", got)
	}
	response = app.request("PATCH", "/api/client/v1/settings", `{"cfEnabled":false,"cfEndpoint":"https://bad.example"}`, "alice")
	if response.Code != 400 {
		t.Fatalf("invalid status=%d", response.Code)
	}
	if got := read("alice"); !got.CFEnabled || got.CFEndpoint != "1.1.1.1" {
		t.Fatalf("invalid request mutated settings: %+v", got)
	}
	// Old clients omit new fields. They must not erase a user's routing choice.
	response = app.request("PATCH", "/api/client/v1/settings", `{"clientTitle":"hello"}`, "alice")
	if response.Code != 200 || !read("alice").CFEnabled {
		t.Fatal("legacy patch reset network settings")
	}
	response = app.request("PATCH", "/api/client/v1/settings", `{"cfEnabled":false}`, "alice")
	if response.Code != 200 || read("alice").CFEnabled || read("alice").CFEndpoint != "1.1.1.1" {
		t.Fatal("disable failed or lost chosen endpoint")
	}
}

func TestCFFeedPresetsPersistWithoutOverridingUserChoice(t *testing.T) {
	app := testServer(t)
	presets := []string{"1.1.1.1", cfnetwork.DefaultEndpoint}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"site": map[string]any{"clientPolicy": map[string]any{"cfPreferredEndpoints": presets}}, "apps": []any{}})
	}))
	defer upstream.Close()
	source, err := app.server.db.ClientSource.Create().SetUserID("alice").SetName("My source").SetURL(upstream.URL + "/source/v2/index.json").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fetch, err := app.server.fetchSourceApps(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.server.saveSourceApps(t.Context(), source, fetch.apps, fetch.siteIconURL, fetch.mirrors, fetch.categories, fetch.announcements, fetch.ads, fetch.clientPolicy, fetch.groups, fetch.invalidCodes, fetch.chatAvailable, fetch.wishWallAvailable, fetch.etag); err != nil {
		t.Fatal(err)
	}
	// Create another Server against the same database to prove the data is not in memory.
	restarted := &Server{db: app.server.db}
	var settings ClientSettingsDTO
	restarted.applyCFSettings(t.Context(), "alice", &settings)
	if settings.CFEnabled || settings.CFEndpoint != cfnetwork.DefaultEndpoint || len(settings.CFPresets) != 2 || settings.CFPresets[1].SourceName != "My source" {
		t.Fatalf("presets=%+v", settings)
	}
	restarted.applyCFSettings(t.Context(), "bob", &settings)
	if len(settings.CFPresets) != 1 {
		t.Fatalf("presets leaked across users: %+v", settings)
	}
	// Removing a source removes it from suggestions even though old KV data remains.
	if err := app.server.db.ClientSource.DeleteOneID(source.ID).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	restarted.applyCFSettings(t.Context(), "alice", &settings)
	if len(settings.CFPresets) != 1 {
		t.Fatal("deleted source still advertised")
	}
}

func TestCFClientRouteSelectionAndStreamingTimeout(t *testing.T) {
	app := testServer(t)
	source, err := app.server.db.ClientSource.Create().SetUserID("alice").SetName("A").SetURL("https://origin.example/source/v2/index.json").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if client := app.server.sourceHTTPClient(t.Context(), source, app.server.httpClient); client != app.server.httpClient {
		t.Fatal("optional route enabled by default")
	}
	response := app.request("PATCH", "/api/client/v1/settings", `{"cfEnabled":true,"cfEndpoint":"saas.sin.fan"}`, "alice")
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	for _, base := range []*http.Client{app.server.httpClient, app.server.streamClient} {
		client := app.server.sourceHTTPClient(t.Context(), source, base)
		rt, ok := client.Transport.(cfnetwork.OriginTransport)
		if !ok || rt.Origin != "origin.example" || client.Timeout != base.Timeout {
			t.Fatalf("route=%T timeout=%s", client.Transport, client.Timeout)
		}
	}
	source.UserID = "bob"
	if app.server.sourceHTTPClient(t.Context(), source, app.server.httpClient) != app.server.httpClient {
		t.Fatal("alice route applied to bob")
	}
	source.UserID = "alice"
	source.URL = strings.Replace(source.URL, "https:", "http:", 1)
	if app.server.sourceHTTPClient(t.Context(), source, app.server.httpClient) != app.server.httpClient {
		t.Fatal("HTTP was routed through CF")
	}
}
