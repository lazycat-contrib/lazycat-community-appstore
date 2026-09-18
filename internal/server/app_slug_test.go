package server

import (
	"net/http"
	"strings"
	"testing"

	"lazycat.community/appstore/ent/app"
)

func TestCreateAppGeneratesAvailableSlug(t *testing.T) {
	store := newTestApp(t)
	store.login("admin", "changeme")

	tests := []struct {
		packageID string
		name      string
		wantSlug  string
	}{
		{packageID: "cloud.lazycat.slug-first", name: "Duplicate App", wantSlug: "duplicate-app"},
		{packageID: "cloud.lazycat.slug-second", name: "duplicate-app"},
		{packageID: "cloud.lazycat.unicode-only", name: "纯中文应用", wantSlug: "cloud-lazycat-unicode-only"},
	}

	seen := make(map[string]struct{}, len(tests))
	for _, test := range tests {
		response := store.do(http.MethodPost, "/api/v1/apps", map[string]any{
			"packageId": test.packageID,
			"name":      test.name,
		})
		if response.Code != http.StatusCreated {
			t.Fatalf("create %s status = %d, body = %s", test.packageID, response.Code, response.Body.String())
		}

		record := store.server.db.App.Query().Where(app.PackageIDEQ(test.packageID)).OnlyX(t.Context())
		if _, exists := seen[record.Slug]; exists {
			t.Fatalf("create %s reused slug %q", test.packageID, record.Slug)
		}
		seen[record.Slug] = struct{}{}
		if test.wantSlug != "" && record.Slug != test.wantSlug {
			t.Fatalf("create %s slug = %q, want %q", test.packageID, record.Slug, test.wantSlug)
		}
		if test.wantSlug == "" && !strings.HasPrefix(record.Slug, "duplicate-app-") {
			t.Fatalf("create %s slug = %q, want duplicate-app suffix", test.packageID, record.Slug)
		}
	}
}
