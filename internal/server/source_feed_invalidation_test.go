package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestSourceFeedInvalidatesAfterSuccessfulMutation(t *testing.T) {
	app := newTestApp(t)
	first := app.do(http.MethodGet, "/source/v2/index.json", nil)
	if first.Code != http.StatusOK || first.Header().Get("ETag") == "" {
		t.Fatalf("initial feed = %d ETag=%q", first.Code, first.Header().Get("ETag"))
	}

	app.login("admin", "changeme")
	updated := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"site_title": "Fresh Feed Title"})
	if updated.Code != http.StatusOK {
		t.Fatalf("settings update = %d body=%s", updated.Code, updated.Body.String())
	}

	second := app.do(http.MethodGet, "/source/v2/index.json", nil)
	if second.Code != http.StatusOK {
		t.Fatalf("updated feed = %d body=%s", second.Code, second.Body.String())
	}
	if first.Header().Get("ETag") == second.Header().Get("ETag") {
		t.Fatalf("ETag did not change: %q", second.Header().Get("ETag"))
	}
	if !strings.Contains(second.Body.String(), `"title":"Fresh Feed Title"`) {
		t.Fatalf("updated feed body = %s", second.Body.String())
	}
}

func TestSourceFeedKeepsCacheAfterRejectedMutation(t *testing.T) {
	app := newTestApp(t)
	first := app.do(http.MethodGet, "/source/v2/index.json", nil)

	rejected := app.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{"site_title": "Rejected Title"})
	if rejected.Code != http.StatusUnauthorized {
		t.Fatalf("rejected update = %d body=%s", rejected.Code, rejected.Body.String())
	}

	second := app.do(http.MethodGet, "/source/v2/index.json", nil)
	if first.Header().Get("ETag") != second.Header().Get("ETag") {
		t.Fatalf("rejected mutation invalidated cache: %q != %q", first.Header().Get("ETag"), second.Header().Get("ETag"))
	}
}
