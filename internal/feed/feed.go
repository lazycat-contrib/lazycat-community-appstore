package feed

import (
	"strings"
	"time"

	"lazycat.community/appstore/internal/catalogmeta"
	mirrorutil "lazycat.community/appstore/internal/mirror"
)

type Input struct {
	BaseURL       string             `json:"baseUrl"`
	GitHubMirrors []mirrorutil.Entry `json:"githubMirrors,omitempty"`
	GeneratedAt   time.Time          `json:"generatedAt"`
	Site          SiteMeta           `json:"site"`
	Announcement  AnnouncementMeta   `json:"announcement"`
	Apps          []AppInput         `json:"apps"`
}

type SiteMeta struct {
	Title     string `json:"title"`
	IconURL   string `json:"iconUrl,omitempty"`
	PublicURL string `json:"publicUrl"`
	SourceURL string `json:"sourceUrl"`
}

type AnnouncementMeta struct {
	Enabled   bool   `json:"enabled"`
	Level     string `json:"level"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type AppInput struct {
	ID               int                      `json:"id"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	Description      string                   `json:"description"`
	IconURL          string                   `json:"iconUrl,omitempty"`
	Category         string                   `json:"category,omitempty"`
	CategoryI18n     map[string]string        `json:"categoryI18n,omitempty"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	Submitter        string                   `json:"submitter,omitempty"`
	InstallProtected bool                     `json:"installProtected"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	Versions         []VersionInput           `json:"versions"`
}

type VersionInput struct {
	Version             string    `json:"version"`
	Status              string    `json:"status"`
	Changelog           string    `json:"changelog,omitempty"`
	SourceType          string    `json:"sourceType,omitempty"`
	DownloadURL         string    `json:"downloadUrl"`
	UpstreamDownloadURL string    `json:"upstreamDownloadUrl,omitempty"`
	SHA256              string    `json:"sha256"`
	Size                int64     `json:"size"`
	PublishedAt         time.Time `json:"publishedAt"`
}

type Index struct {
	Schema        string             `json:"schema"`
	BaseURL       string             `json:"baseUrl"`
	GitHubMirrors []mirrorutil.Entry `json:"githubMirrors,omitempty"`
	GeneratedAt   time.Time          `json:"generatedAt"`
	Site          SiteMeta           `json:"site"`
	Announcement  AnnouncementMeta   `json:"announcement"`
	Apps          []App              `json:"apps"`
}

type App struct {
	ID               int                      `json:"id"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	Description      string                   `json:"description"`
	IconURL          string                   `json:"iconUrl,omitempty"`
	Category         string                   `json:"category,omitempty"`
	CategoryI18n     map[string]string        `json:"categoryI18n,omitempty"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	Submitter        string                   `json:"submitter,omitempty"`
	InstallProtected bool                     `json:"installProtected"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	LatestVersion    Version                  `json:"latestVersion"`
	Versions         []Version                `json:"versions"`
}

type Version struct {
	Version             string    `json:"version"`
	Changelog           string    `json:"changelog,omitempty"`
	SourceType          string    `json:"sourceType,omitempty"`
	DownloadURL         string    `json:"downloadUrl"`
	UpstreamDownloadURL string    `json:"upstreamDownloadUrl,omitempty"`
	SHA256              string    `json:"sha256"`
	Size                int64     `json:"size"`
	PublishedAt         time.Time `json:"publishedAt"`
}

func BuildIndex(input Input) Index {
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	index := Index{
		Schema:        "lazycat.appstore.source.v1",
		BaseURL:       strings.TrimRight(input.BaseURL, "/"),
		GitHubMirrors: input.GitHubMirrors,
		GeneratedAt:   generatedAt,
		Site:          input.Site,
		Announcement:  input.Announcement,
		Apps:          make([]App, 0, len(input.Apps)),
	}
	if index.Site.PublicURL == "" {
		index.Site.PublicURL = index.BaseURL
	}
	if index.Site.SourceURL == "" && index.Site.PublicURL != "" {
		index.Site.SourceURL = strings.TrimRight(index.Site.PublicURL, "/") + "/source/v1/index.json"
	}

	for _, inApp := range input.Apps {
		versions := approvedVersions(inApp.Versions, inApp.InstallProtected)
		if len(versions) == 0 {
			continue
		}
		index.Apps = append(index.Apps, App{
			ID:               inApp.ID,
			PackageID:        inApp.PackageID,
			Name:             inApp.Name,
			Slug:             inApp.Slug,
			Summary:          inApp.Summary,
			Description:      inApp.Description,
			IconURL:          inApp.IconURL,
			Category:         inApp.Category,
			CategoryI18n:     inApp.CategoryI18n,
			Screenshots:      inApp.Screenshots,
			Tags:             inApp.Tags,
			Submitter:        inApp.Submitter,
			InstallProtected: inApp.InstallProtected,
			UpdatedAt:        inApp.UpdatedAt,
			LatestVersion:    versions[0],
			Versions:         versions,
		})
	}
	return index
}

func approvedVersions(inputs []VersionInput, installProtected bool) []Version {
	versions := make([]Version, 0, len(inputs))
	for _, input := range inputs {
		if input.Status != "APPROVED" {
			continue
		}
		downloadURL := input.DownloadURL
		upstreamDownloadURL := input.UpstreamDownloadURL
		if upstreamDownloadURL == "" {
			upstreamDownloadURL = input.DownloadURL
		}
		if installProtected {
			upstreamDownloadURL = ""
		}
		versions = append(versions, Version{
			Version:             input.Version,
			Changelog:           input.Changelog,
			SourceType:          input.SourceType,
			DownloadURL:         downloadURL,
			UpstreamDownloadURL: upstreamDownloadURL,
			SHA256:              input.SHA256,
			Size:                input.Size,
			PublishedAt:         input.PublishedAt,
		})
	}
	return versions
}
