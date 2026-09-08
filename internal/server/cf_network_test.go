package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"lazycat.community/appstore/internal/cfnetwork"
)

func TestCFPresetsAdminConfigAndFeed(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")
	read := func() []string {
		t.Helper()
		response := app.do(http.MethodGet, "/source/v2/index.json", nil)
		var body struct {
			Site struct {
				ClientPolicy siteClientPolicy `json:"clientPolicy"`
			} `json:"site"`
		}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &body) != nil {
			t.Fatalf("feed: %d %s", response.Code, response.Body.String())
		}
		return body.Site.ClientPolicy.CFPreferredEndpoints
	}
	if got := read(); len(got) != 1 || got[0] != cfnetwork.DefaultEndpoint {
		t.Fatalf("default=%v", got)
	}
	response := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"cf_preferred_endpoints": "1.1.1.1\nSAAS.SIN.FAN\n1.1.1.1"})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if got := read(); len(got) != 2 || got[0] != "1.1.1.1" || got[1] != cfnetwork.DefaultEndpoint {
		t.Fatalf("updated feed=%v", got)
	}
	response = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"cf_preferred_endpoints": "127.0.0.1"})
	if response.Code != 422 || len(read()) != 2 {
		t.Fatal("invalid preset accepted or changed previous config")
	}
	response = app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"cf_preferred_endpoints": ""})
	if response.Code != 200 || len(read()) != 0 {
		t.Fatal("empty presets did not stop publishing suggestions")
	}
}
