package v2

import (
	"strings"
	"time"

	"lazycat.community/appstore/internal/feed"
	mirrorutil "lazycat.community/appstore/internal/mirror"
)

const Schema = "lazycat.appstore.source.v2"

type Category = feed.CategoryInput

type App struct {
	feed.App
	CategoryID *int `json:"categoryId,omitempty"`
}

type Index struct {
	Schema            string                  `json:"schema"`
	BaseURL           string                  `json:"baseUrl"`
	GitHubMirrors     []mirrorutil.Entry      `json:"githubMirrors,omitempty"`
	GeneratedAt       time.Time               `json:"generatedAt"`
	Site              feed.SiteMeta           `json:"site"`
	Announcement      feed.AnnouncementMeta   `json:"announcement"`
	Announcements     []feed.AnnouncementMeta `json:"announcements,omitempty"`
	Ads               []feed.AdMeta           `json:"ads,omitempty"`
	Categories        []Category              `json:"categories,omitempty"`
	Groups            []feed.GroupMeta        `json:"groups,omitempty"`
	InvalidGroupCodes []string                `json:"invalidGroupCodes,omitempty"`
	Apps              []App                   `json:"apps"`
}

func BuildIndex(input feed.Input) Index {
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	index := Index{
		Schema:            Schema,
		BaseURL:           strings.TrimRight(input.BaseURL, "/"),
		GitHubMirrors:     input.GitHubMirrors,
		GeneratedAt:       generatedAt,
		Site:              input.Site,
		Announcement:      input.Announcement,
		Announcements:     input.Announcements,
		Ads:               input.Ads,
		Categories:        make([]Category, 0, len(input.Categories)),
		Groups:            input.Groups,
		InvalidGroupCodes: input.InvalidGroupCodes,
		Apps:              make([]App, 0, len(input.Apps)),
	}
	if index.Site.PublicURL == "" {
		index.Site.PublicURL = index.BaseURL
	}
	if index.Site.SourceURL == "" && index.Site.PublicURL != "" {
		index.Site.SourceURL = strings.TrimRight(index.Site.PublicURL, "/") + "/source/v2/index.json"
	}
	index.Categories = append(index.Categories, input.Categories...)

	for _, inApp := range input.Apps {
		app, ok := feed.BuildApp(inApp)
		if !ok {
			continue
		}
		index.Apps = append(index.Apps, App{App: app, CategoryID: inApp.CategoryID})
	}
	return index
}
