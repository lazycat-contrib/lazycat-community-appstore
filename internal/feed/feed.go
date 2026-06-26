package feed

import (
	"strings"
	"time"
)

type Input struct {
	BaseURL      string     `json:"baseUrl"`
	GitHubMirror string     `json:"githubMirror,omitempty"`
	GeneratedAt  time.Time  `json:"generatedAt"`
	Apps         []AppInput `json:"apps"`
}

type AppInput struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	IconURL     string         `json:"iconUrl,omitempty"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Submitter   string         `json:"submitter,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Versions    []VersionInput `json:"versions"`
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
	Schema      string    `json:"schema"`
	GeneratedAt time.Time `json:"generatedAt"`
	Apps        []App     `json:"apps"`
}

type App struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Summary       string    `json:"summary"`
	Description   string    `json:"description"`
	IconURL       string    `json:"iconUrl,omitempty"`
	Category      string    `json:"category,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	Submitter     string    `json:"submitter,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
	LatestVersion Version   `json:"latestVersion"`
	Versions      []Version `json:"versions"`
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
		Schema:      "lazycat.appstore.source.v1",
		GeneratedAt: generatedAt,
		Apps:        make([]App, 0, len(input.Apps)),
	}

	for _, inApp := range input.Apps {
		versions := approvedVersions(inApp.Versions, input.GitHubMirror)
		if len(versions) == 0 {
			continue
		}
		index.Apps = append(index.Apps, App{
			ID:            inApp.ID,
			Name:          inApp.Name,
			Slug:          inApp.Slug,
			Summary:       inApp.Summary,
			Description:   inApp.Description,
			IconURL:       inApp.IconURL,
			Category:      inApp.Category,
			Tags:          inApp.Tags,
			Submitter:     inApp.Submitter,
			UpdatedAt:     inApp.UpdatedAt,
			LatestVersion: versions[0],
			Versions:      versions,
		})
	}
	return index
}

func approvedVersions(inputs []VersionInput, mirror string) []Version {
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
		if isGitHubSource(input.SourceType, upstreamDownloadURL) && mirror != "" {
			downloadURL = mirrorDownload(upstreamDownloadURL, mirror)
		}
		versions = append(versions, Version{
			Version:             input.Version,
			Changelog:           input.Changelog,
			SourceType:          input.SourceType,
			DownloadURL:         downloadURL,
			UpstreamDownloadURL: input.UpstreamDownloadURL,
			SHA256:              input.SHA256,
			Size:                input.Size,
			PublishedAt:         input.PublishedAt,
		})
	}
	return versions
}

func isGitHubSource(sourceType, rawURL string) bool {
	return strings.EqualFold(sourceType, "GITHUB") && (strings.Contains(rawURL, "github.com/") || strings.Contains(rawURL, "githubusercontent.com/"))
}

func mirrorDownload(rawURL, mirror string) string {
	if mirror == "" {
		return rawURL
	}
	if strings.Contains(rawURL, "github.com/") || strings.Contains(rawURL, "githubusercontent.com/") {
		return strings.TrimRight(mirror, "/") + "/" + rawURL
	}
	return rawURL
}
