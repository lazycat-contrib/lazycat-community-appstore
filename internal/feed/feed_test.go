package feed

import (
	"testing"

	"lazycat.community/appstore/internal/mirror"
)

func TestBuildIndexUsesLatestApprovedVersionAndPublishesMirrors(t *testing.T) {
	index := BuildIndex(Input{
		BaseURL: "https://store.example.com",
		GitHubMirrors: []mirror.Entry{
			{ID: mirror.ID(mirror.KindDownload, "https://mirror.example.com/https://github.com"), Kind: mirror.KindDownload, Name: "Fast", URL: "https://mirror.example.com/https://github.com"},
		},
		Apps: []AppInput{
			{
				ID:          1,
				Name:        "Demo",
				Slug:        "demo",
				Summary:     "A demo app",
				Description: "Demo description",
				Category:    "Tools",
				Tags:        []string{"utility"},
				Versions: []VersionInput{
					{Version: "1.0.0", Status: "APPROVED", SourceType: "GITHUB", DownloadURL: "https://store.example.com/api/v1/apps/1/versions/2/download", UpstreamDownloadURL: "https://github.com/acme/demo/releases/download/v1/demo.lpk", SHA256: "abc", Size: 12},
					{Version: "0.9.0", Status: "APPROVED", SourceType: "GITHUB", DownloadURL: "https://store.example.com/api/v1/apps/1/versions/1/download", UpstreamDownloadURL: "https://github.com/acme/demo/releases/download/v0/demo.lpk", SHA256: "old", Size: 11},
				},
			},
		},
	})

	if len(index.Apps) != 1 {
		t.Fatalf("len(index.Apps) = %d, want 1", len(index.Apps))
	}
	app := index.Apps[0]
	if app.LatestVersion.Version != "1.0.0" {
		t.Fatalf("latest version = %q, want 1.0.0", app.LatestVersion.Version)
	}
	if len(index.GitHubMirrors) != 1 || index.GitHubMirrors[0].Name != "Fast" {
		t.Fatalf("github mirrors = %#v", index.GitHubMirrors)
	}
	if app.LatestVersion.DownloadURL != "https://store.example.com/api/v1/apps/1/versions/2/download" {
		t.Fatalf("download URL = %q", app.LatestVersion.DownloadURL)
	}
	if app.LatestVersion.UpstreamDownloadURL != "https://github.com/acme/demo/releases/download/v1/demo.lpk" {
		t.Fatalf("upstream download URL = %q", app.LatestVersion.UpstreamDownloadURL)
	}
}

func TestBuildIndexKeepsStoreDownloadURLWithoutMirror(t *testing.T) {
	index := BuildIndex(Input{
		Apps: []AppInput{
			{
				ID:   1,
				Name: "Demo",
				Versions: []VersionInput{
					{
						Version:             "1.0.0",
						Status:              "APPROVED",
						SourceType:          "GITHUB",
						DownloadURL:         "https://store.example.com/api/v1/apps/1/versions/2/download",
						UpstreamDownloadURL: "https://github.com/acme/demo/releases/download/v1/demo.lpk",
					},
				},
			},
		},
	})

	if got := index.Apps[0].LatestVersion.DownloadURL; got != "https://store.example.com/api/v1/apps/1/versions/2/download" {
		t.Fatalf("download URL = %q", got)
	}
}

func TestBuildIndexDoesNotMirrorProtectedAppDownloads(t *testing.T) {
	index := BuildIndex(Input{
		Apps: []AppInput{
			{
				ID:               1,
				Name:             "Protected",
				InstallProtected: true,
				Versions: []VersionInput{
					{
						Version:             "1.0.0",
						Status:              "APPROVED",
						SourceType:          "GITHUB",
						DownloadURL:         "https://store.example.com/api/v1/apps/1/versions/2/download",
						UpstreamDownloadURL: "https://github.com/acme/protected/releases/download/v1/app.lpk",
					},
				},
			},
		},
	})

	if got := index.Apps[0].LatestVersion.DownloadURL; got != "https://store.example.com/api/v1/apps/1/versions/2/download" {
		t.Fatalf("protected app download URL = %q", got)
	}
	if got := index.Apps[0].LatestVersion.UpstreamDownloadURL; got != "" {
		t.Fatalf("protected app upstream download URL = %q", got)
	}
	if !index.Apps[0].InstallProtected {
		t.Fatal("protected app did not expose installProtected=true")
	}
}

func TestBuildIndexSkipsAppsWithoutApprovedVersion(t *testing.T) {
	index := BuildIndex(Input{
		Apps: []AppInput{
			{
				ID:   1,
				Name: "Draft",
				Versions: []VersionInput{
					{Version: "1.0.0", Status: "PENDING", DownloadURL: "https://example.com/app.lpk"},
				},
			},
		},
	})

	if len(index.Apps) != 0 {
		t.Fatalf("len(index.Apps) = %d, want 0", len(index.Apps))
	}
}
