package v2_test

import (
	"testing"

	"lazycat.community/appstore/internal/feed"
	feedv2 "lazycat.community/appstore/internal/feed/v2"
)

func TestBuildIndexPublishesStructuredCategories(t *testing.T) {
	parentID := 1
	index := feedv2.BuildIndex(feed.Input{
		BaseURL: "https://store.example.com",
		Categories: []feed.CategoryInput{
			{ID: parentID, Name: "Tools", Slug: "tools"},
			{ID: 2, Name: "Download", Slug: "download", ParentID: &parentID, SortOrder: 10},
		},
		Apps: []feed.AppInput{
			{
				ID:         1,
				PackageID:  "cloud.lazycat.app.demo",
				Name:       "Demo",
				Slug:       "demo",
				CategoryID: intPtr(2),
				Category:   "Download",
				Versions: []feed.VersionInput{
					{Version: "1.0.0", Status: "APPROVED", DownloadURL: "https://store.example.com/api/v1/apps/1/versions/1/download"},
				},
			},
		},
	})

	if index.Schema != feedv2.Schema {
		t.Fatalf("schema = %q, want %q", index.Schema, feedv2.Schema)
	}
	if got := index.Site.SourceURL; got != "https://store.example.com/source/v2/index.json" {
		t.Fatalf("source url = %q", got)
	}
	if len(index.Categories) != 2 || index.Categories[1].ParentID == nil || *index.Categories[1].ParentID != parentID {
		t.Fatalf("categories = %#v", index.Categories)
	}
	if len(index.Apps) != 1 || index.Apps[0].CategoryID == nil || *index.Apps[0].CategoryID != 2 {
		t.Fatalf("apps = %#v", index.Apps)
	}
}

func intPtr(value int) *int {
	return &value
}
