package feed

import (
	"time"

	"lazycat.community/appstore/internal/catalogmeta"
	mirrorutil "lazycat.community/appstore/internal/mirror"
)

type Input struct {
	BaseURL           string             `json:"baseUrl"`
	GitHubMirrors     []mirrorutil.Entry `json:"githubMirrors,omitempty"`
	GeneratedAt       time.Time          `json:"generatedAt"`
	Site              SiteMeta           `json:"site"`
	Announcement      AnnouncementMeta   `json:"announcement"`
	Announcements     []AnnouncementMeta `json:"announcements,omitempty"`
	Ads               []AdMeta           `json:"ads,omitempty"`
	Categories        []CategoryInput    `json:"categories,omitempty"`
	Groups            []GroupMeta        `json:"groups,omitempty"`
	InvalidGroupCodes []string           `json:"invalidGroupCodes,omitempty"`
	Apps              []AppInput         `json:"apps"`
}

type SiteMeta struct {
	Title        string           `json:"title"`
	IconURL      string           `json:"iconUrl,omitempty"`
	PublicURL    string           `json:"publicUrl"`
	SourceURL    string           `json:"sourceUrl"`
	ClientPolicy ClientPolicyMeta `json:"clientPolicy,omitempty"`
	Chat         ChatMeta         `json:"chat"`
}

type ClientPolicyMeta struct {
	MinVersion string `json:"minVersion,omitempty"`
	Message    string `json:"message,omitempty"`
}

type ChatMeta struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retentionDays,omitempty"`
}

type AnnouncementMeta struct {
	ID        int    `json:"id,omitempty"`
	Enabled   bool   `json:"enabled"`
	Level     string `json:"level"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	StartsAt  string `json:"startsAt,omitempty"`
	EndsAt    string `json:"endsAt,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type AdMeta struct {
	ID        int    `json:"id,omitempty"`
	Enabled   bool   `json:"enabled"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	ImageURL  string `json:"imageUrl,omitempty"`
	LinkLabel string `json:"linkLabel,omitempty"`
	LinkURL   string `json:"linkUrl,omitempty"`
	StartsAt  string `json:"startsAt,omitempty"`
	EndsAt    string `json:"endsAt,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type GroupMeta struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}

type CategoryInput struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	NameI18n  map[string]string `json:"nameI18n,omitempty"`
	Slug      string            `json:"slug"`
	ParentID  *int              `json:"parentId,omitempty"`
	SortOrder int               `json:"sortOrder,omitempty"`
}

type AppInput struct {
	ID               int                      `json:"id"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	NameI18n         map[string]string        `json:"nameI18n,omitempty"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	SummaryI18n      map[string]string        `json:"summaryI18n,omitempty"`
	Description      string                   `json:"description"`
	DescriptionI18n  map[string]string        `json:"descriptionI18n,omitempty"`
	IconURL          string                   `json:"iconUrl,omitempty"`
	CategoryID       *int                     `json:"categoryId,omitempty"`
	Category         string                   `json:"category,omitempty"`
	CategoryI18n     map[string]string        `json:"categoryI18n,omitempty"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	Submitter        string                   `json:"submitter,omitempty"`
	InstallProtected bool                     `json:"installProtected"`
	CommentsEnabled  *bool                    `json:"commentsEnabled,omitempty"`
	OutdatedMarks    int                      `json:"outdatedMarks,omitempty"`
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

type App struct {
	ID               int                      `json:"id"`
	PackageID        string                   `json:"packageId"`
	Name             string                   `json:"name"`
	NameI18n         map[string]string        `json:"nameI18n,omitempty"`
	Slug             string                   `json:"slug"`
	Summary          string                   `json:"summary"`
	SummaryI18n      map[string]string        `json:"summaryI18n,omitempty"`
	Description      string                   `json:"description"`
	DescriptionI18n  map[string]string        `json:"descriptionI18n,omitempty"`
	IconURL          string                   `json:"iconUrl,omitempty"`
	Category         string                   `json:"category,omitempty"`
	CategoryI18n     map[string]string        `json:"categoryI18n,omitempty"`
	Screenshots      []catalogmeta.Screenshot `json:"screenshots,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	Submitter        string                   `json:"submitter,omitempty"`
	InstallProtected bool                     `json:"installProtected"`
	CommentsEnabled  bool                     `json:"commentsEnabled"`
	OutdatedMarks    int                      `json:"outdatedMarks,omitempty"`
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

func BuildApp(inApp AppInput) (App, bool) {
	versions := ApprovedVersions(inApp.Versions, inApp.InstallProtected)
	if len(versions) == 0 {
		return App{}, false
	}
	commentsEnabled := true
	if inApp.CommentsEnabled != nil {
		commentsEnabled = *inApp.CommentsEnabled
	}
	return App{
		ID:               inApp.ID,
		PackageID:        inApp.PackageID,
		Name:             inApp.Name,
		NameI18n:         inApp.NameI18n,
		Slug:             inApp.Slug,
		Summary:          inApp.Summary,
		SummaryI18n:      inApp.SummaryI18n,
		Description:      inApp.Description,
		DescriptionI18n:  inApp.DescriptionI18n,
		IconURL:          inApp.IconURL,
		Category:         inApp.Category,
		CategoryI18n:     inApp.CategoryI18n,
		Screenshots:      inApp.Screenshots,
		Tags:             inApp.Tags,
		Submitter:        inApp.Submitter,
		InstallProtected: inApp.InstallProtected,
		CommentsEnabled:  commentsEnabled,
		OutdatedMarks:    inApp.OutdatedMarks,
		UpdatedAt:        inApp.UpdatedAt,
		LatestVersion:    versions[0],
		Versions:         versions,
	}, true
}

func ApprovedVersions(inputs []VersionInput, installProtected bool) []Version {
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
