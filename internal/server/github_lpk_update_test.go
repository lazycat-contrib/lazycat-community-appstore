package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/githublpkupdatepolicy"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/user"
)

type fakeGitHubLatestReleaseClient struct {
	release      *github.RepositoryRelease
	err          error
	beforeReturn func()
	owner        string
	repo         string
	calls        int
}

func (f *fakeGitHubLatestReleaseClient) GetLatestRelease(_ context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	f.calls++
	f.owner = owner
	f.repo = repo
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	return f.release, nil, f.err
}

func TestParseGitHubReleaseLPKURL(t *testing.T) {
	t.Parallel()
	valid := "https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.2/com.lxy.app.clash-v2.0.2.lpk"
	parsed, err := parseGitHubReleaseLPKURL(valid)
	if err != nil {
		t.Fatalf("parse valid URL: %v", err)
	}
	if parsed.Owner != "wlabbyflower" || parsed.Repo != "peppapigconfigurationguide" || parsed.Tag != "v2.0.2" || parsed.AssetName != "com.lxy.app.clash-v2.0.2.lpk" {
		t.Fatalf("parsed URL = %+v", parsed)
	}

	for _, rawURL := range []string{
		"http://github.com/acme/app/releases/download/v1/app.lpk",
		"https://github.com/acme/app/releases/latest/download/app.lpk",
		"https://github.com/acme/app/releases/tag/v1",
		"https://github.com/acme/app/releases/download/v1/app.zip",
		"https://example.com/acme/app/releases/download/v1/app.lpk",
		"https://github.com/acme/app/releases/download/v1/app.lpk?download=1",
	} {
		if _, err := parseGitHubReleaseLPKURL(rawURL); err == nil {
			t.Fatalf("parseGitHubReleaseLPKURL(%q) succeeded, want error", rawURL)
		}
	}
}

func TestGitHubReleaseVersionNormalizesTagForStorage(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"v2.0.3":             "2.0.3",
		"release-2.0.3-rc.1": "2.0.3-rc.1",
		"server-v0.1.38":     "0.1.38",
		"refs/tags/v3.4.5":   "3.4.5",
		"not-a-version":      "",
		"v01.2.3":            "",
		"release-2.0.3foo":   "",
		"release-2.0":        "",
		"release-2.0.3.4":    "",
		"v2.0.3+build.7":     "2.0.3+build.7",
	}
	for input, want := range tests {
		if got := githubReleaseVersion(input); got != want {
			t.Fatalf("githubReleaseVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSelectGitHubLPKReleaseAssetUsesVersionedName(t *testing.T) {
	t.Parallel()
	current := githubReleaseLPK{
		Tag:       "v2.0.2",
		AssetName: "com.lxy.app.clash-v2.0.2.lpk",
	}
	assets := []*github.ReleaseAsset{
		{Name: github.Ptr("checksums.txt")},
		{Name: github.Ptr("com.lxy.app.clash-v2.0.3.lpk")},
		{Name: github.Ptr("another-package-v2.0.3.lpk")},
	}
	asset, err := selectGitHubLPKReleaseAsset(assets, current, "com.lxy.app.clash", "2.0.2", "v2.0.3", "2.0.3")
	if err != nil {
		t.Fatalf("select asset: %v", err)
	}
	if got := asset.GetName(); got != "com.lxy.app.clash-v2.0.3.lpk" {
		t.Fatalf("asset = %q", got)
	}
}

func TestGitHubReleaseAssetSHA256RequiresDigest(t *testing.T) {
	t.Parallel()
	want := strings.Repeat("a", 64)
	if got, err := githubReleaseAssetSHA256("sha256:" + want); err != nil || got != want {
		t.Fatalf("digest = %q, %v", got, err)
	}
	for _, value := range []string{"", want, "sha512:" + want, "sha256:bad"} {
		if _, err := githubReleaseAssetSHA256(value); err == nil {
			t.Fatalf("digest %q succeeded, want error", value)
		}
	}
}

func TestGitHubLPKUpdatePolicyCascadesWithAppDeletion(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	store.server.db.App.DeleteOneID(record.ID).ExecX(t.Context())
	if count := store.server.db.GitHubLPKUpdatePolicy.Query().Where(githublpkupdatepolicy.AppIDEQ(record.ID)).CountX(t.Context()); count != 0 {
		t.Fatalf("policy count after app deletion = %d, want 0", count)
	}
}

func TestGitHubLPKAutoUpdateCreatesApprovedVersionFromAssetDigest(t *testing.T) {
	store := newTestApp(t)
	record, current := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.App.UpdateOneID(record.ID).SetVersionRetentionCount(1).SaveX(t.Context())
	publishedAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	catalogPublishedAt := time.Date(2026, 7, 28, 13, 0, 0, 0, time.UTC)
	store.server.setNow(func() time.Time { return catalogPublishedAt })
	digest := strings.Repeat("b", 64)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName:     "release-v2.0.3",
		Body:        github.Ptr("Upstream release notes"),
		PublishedAt: &github.Timestamp{Time: publishedAt},
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/release-v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	version, err := store.server.runGitHubLPKUpdate(t.Context(), policy)
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	if version != "2.0.3" || fake.calls != 1 || fake.owner != "wlabbyflower" || fake.repo != "peppapigconfigurationguide" {
		t.Fatalf("version=%q calls=%d repo=%s/%s", version, fake.calls, fake.owner, fake.repo)
	}
	created := store.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("2.0.3")).
		OnlyX(t.Context())
	if created.Status != appversion.StatusAPPROVED || created.SourceType != appversion.SourceTypeGITHUB || created.Sha256 != digest || created.FileSize != 123456 || created.Changelog != "Upstream release notes" {
		t.Fatalf("created version = %+v", created)
	}
	if created.PublishedAt == nil || !created.PublishedAt.Equal(catalogPublishedAt) {
		t.Fatalf("publishedAt = %v, want %v", created.PublishedAt, catalogPublishedAt)
	}
	if created.UpstreamPublishedAt == nil || !created.UpstreamPublishedAt.Equal(publishedAt) {
		t.Fatalf("upstreamPublishedAt = %v, want %v", created.UpstreamPublishedAt, publishedAt)
	}
	if current.DownloadURL == created.DownloadURL {
		t.Fatalf("download URL did not advance: %q", created.DownloadURL)
	}
	latest, err := store.server.latestApprovedVersion(t.Context(), record.ID)
	if err != nil || latest.ID != created.ID {
		t.Fatalf("latest approved version = %+v, %v; want created version", latest, err)
	}
	if count := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(t.Context()); count != 1 {
		t.Fatalf("version count after retention = %d, want 1", count)
	}
}

func TestGitHubLPKAutoUpdateWithoutReleaseTimestampIsIdempotent(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	digest := strings.Repeat("4", 64)
	store.server.githubReleases = &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err != nil {
		t.Fatalf("first update: %v", err)
	}
	created := store.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("2.0.3")).
		OnlyX(t.Context())
	if created.UpstreamPublishedAt != nil {
		t.Fatalf("upstreamPublishedAt = %v, want nil", created.UpstreamPublishedAt)
	}
	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err != nil {
		t.Fatalf("second update: %v", err)
	}
	unchanged := store.server.db.AppVersion.GetX(t.Context(), created.ID)
	if !unchanged.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("idempotent update changed updatedAt from %v to %v", created.UpdatedAt, unchanged.UpdatedAt)
	}
}

func TestGitHubLPKAutoUpdateDoesNotChangeVersionWithoutDigest(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("run update error = %v, want digest error", err)
	}
	if count := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(t.Context()); count != 1 {
		t.Fatalf("version count = %d, want existing version only", count)
	}
}

func TestGitHubLPKAutoUpdateRejectsMismatchedAssetTag(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	digest := strings.Repeat("7", 64)
	store.server.githubReleases = &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v9.9.9/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "asset tag") {
		t.Fatalf("run update error = %v, want asset tag error", err)
	}
	if count := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(t.Context()); count != 1 {
		t.Fatalf("version count = %d, want existing version only", count)
	}
}

func TestGitHubLPKAutoUpdateFailsClosedForInvalidCurrentVersion(t *testing.T) {
	store := newTestApp(t)
	record, current := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.AppVersion.UpdateOneID(current.ID).SetVersion("2.0.2_hotfix").SaveX(t.Context())
	fake := &fakeGitHubLatestReleaseClient{}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "valid SemVer") {
		t.Fatalf("run update error = %v, want SemVer error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("GitHub API calls = %d, want 0", fake.calls)
	}
}

func TestGitHubLPKAutoUpdateDiscardsResultWhenLatestVersionChanges(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	owner := store.server.db.User.GetX(t.Context(), record.OwnerID)
	digest := strings.Repeat("6", 64)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	fake.beforeReturn = func() {
		store.server.db.AppVersion.Create().
			SetAppID(record.ID).
			SetUploaderID(owner.ID).
			SetVersion("3.0.0").
			SetStatus(appversion.StatusAPPROVED).
			SetSourceType(appversion.SourceTypeGITHUB).
			SetDownloadURL("https://github.com/example/new-upstream/releases/download/v3.0.0/com.lxy.app.clash-v3.0.0.lpk").
			SetSha256(strings.Repeat("5", 64)).
			SetFileSize(222222).
			SetPublishedAt(time.Now().Add(time.Hour)).
			SaveX(t.Context())
		fake.beforeReturn = nil
	}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "changed during") {
		t.Fatalf("run update error = %v, want changed baseline error", err)
	}
	if exists := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("2.0.3")).ExistX(t.Context()); exists {
		t.Fatal("stale upstream version 2.0.3 was created")
	}
}

func TestGitHubLPKAutoUpdateDoesNotRunForUnlistedApp(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.App.UpdateOneID(record.ID).SetStatus(app.StatusUNLISTED).SaveX(t.Context())
	fake := &fakeGitHubLatestReleaseClient{}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "approved app") {
		t.Fatalf("run update error = %v, want approved app error", err)
	}
	if fake.calls != 0 {
		t.Fatalf("GitHub API calls = %d, want 0", fake.calls)
	}
	if status := store.server.db.App.GetX(t.Context(), record.ID).Status; status != app.StatusUNLISTED {
		t.Fatalf("app status = %s, want UNLISTED", status)
	}
}

func TestGitHubLPKAutoUpdateDoesNotOverwriteDifferentSourceAtSameVersion(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	manual := store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(record.OwnerID).
		SetVersion("2.0.3").
		SetStatus(appversion.StatusPENDING).
		SetSourceType(appversion.SourceTypeLOCAL).
		SetDownloadURL("https://downloads.example.com/manual-v2.0.3.lpk").
		SetSha256(strings.Repeat("d", 64)).
		SetFileSize(654321).
		SaveX(t.Context())
	digest := strings.Repeat("e", 64)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "different source") {
		t.Fatalf("run update error = %v, want different source error", err)
	}
	unchanged := store.server.db.AppVersion.GetX(t.Context(), manual.ID)
	if unchanged.SourceType != appversion.SourceTypeLOCAL || unchanged.DownloadURL != manual.DownloadURL || unchanged.Sha256 != manual.Sha256 || unchanged.FileSize != manual.FileSize {
		t.Fatalf("manual version was overwritten: %+v", unchanged)
	}
}

func TestGitHubLPKAutoUpdateDoesNotPublishForDisabledOwner(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.User.UpdateOneID(record.OwnerID).SetDisabled(true).SaveX(t.Context())
	digest := strings.Repeat("f", 64)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "owner is disabled") {
		t.Fatalf("run update error = %v, want disabled owner error", err)
	}
	if count := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(t.Context()); count != 1 {
		t.Fatalf("version count = %d, want existing version only", count)
	}
}

func TestGitHubLPKAutoUpdateCreatesPendingVersionForReviewedApp(t *testing.T) {
	store := newTestApp(t)
	owner := store.server.db.User.Create().
		SetUsername("github-auto-update-owner").
		SetPasswordHash("x").
		SetRole(user.RoleUSER).
		SaveX(t.Context())
	record, _ := createGitHubLPKUpdateFixtureForOwner(t, store, owner, false)
	digest := strings.Repeat("c", 64)
	publishedAt := time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)
	fake := &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName:     "v2.0.3",
		PublishedAt: &github.Timestamp{Time: publishedAt},
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk"),
		}},
	}}
	store.server.githubReleases = fake
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if version, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err != nil || version != "2.0.3" {
		t.Fatalf("run update = %q, %v", version, err)
	}
	created := store.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("2.0.3")).
		OnlyX(t.Context())
	if created.Status != appversion.StatusPENDING || created.PublishedAt != nil {
		t.Fatalf("created version status = %s, publishedAt = %v", created.Status, created.PublishedAt)
	}
	if created.UpstreamPublishedAt == nil || !created.UpstreamPublishedAt.Equal(publishedAt) {
		t.Fatalf("upstreamPublishedAt = %v, want %v", created.UpstreamPublishedAt, publishedAt)
	}
	review := store.server.db.ReviewRequest.Query().
		Where(
			reviewrequest.KindEQ(reviewrequest.KindVERSION_UPLOAD),
			reviewrequest.StatusEQ(reviewrequest.StatusPENDING),
			reviewrequest.VersionIDEQ(created.ID),
		).
		OnlyX(t.Context())
	if review.AppID == nil || *review.AppID != record.ID || review.RequesterID != owner.ID {
		t.Fatalf("review request = %+v", review)
	}
	fake.release.Assets[0].Digest = github.Ptr("sha256:" + strings.Repeat("9", 64))
	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "pending review") {
		t.Fatalf("changed pending release error = %v, want pending review error", err)
	}
	unchanged := store.server.db.AppVersion.GetX(t.Context(), created.ID)
	if unchanged.Sha256 != digest {
		t.Fatalf("pending version digest changed from %s to %s", digest, unchanged.Sha256)
	}
	if count := store.server.db.ReviewRequest.Query().
		Where(reviewrequest.VersionIDEQ(created.ID), reviewrequest.StatusEQ(reviewrequest.StatusPENDING)).
		CountX(t.Context()); count != 1 {
		t.Fatalf("pending review count = %d, want 1", count)
	}
	store.login("admin", "changeme")
	rec := store.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/reviews/%d/approve", review.ID), map[string]string{"note": "ok"})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve review status = %d, body = %s", rec.Code, rec.Body.String())
	}
	approved := store.server.db.AppVersion.GetX(t.Context(), created.ID)
	if approved.Status != appversion.StatusAPPROVED || approved.PublishedAt == nil {
		t.Fatalf("approved version = %+v", approved)
	}
	if approved.UpstreamPublishedAt == nil || !approved.UpstreamPublishedAt.Equal(publishedAt) {
		t.Fatalf("approved upstreamPublishedAt = %v, want %v", approved.UpstreamPublishedAt, publishedAt)
	}
}

func TestGitHubLPKAutoUpdateDoesNotResubmitRejectedVersion(t *testing.T) {
	store := newTestApp(t)
	owner := store.server.db.User.Create().
		SetUsername("github-auto-update-rejected-owner").
		SetPasswordHash("x").
		SetRole(user.RoleUSER).
		SaveX(t.Context())
	record, _ := createGitHubLPKUpdateFixtureForOwner(t, store, owner, false)
	digest := strings.Repeat("8", 64)
	rejected := store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.3").
		SetStatus(appversion.StatusREJECTED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.3/com.lxy.app.clash-v2.0.3.lpk").
		SetSha256(digest).
		SetFileSize(123456).
		SaveX(t.Context())
	store.server.githubReleases = &fakeGitHubLatestReleaseClient{release: &github.RepositoryRelease{
		TagName: "v2.0.3",
		Assets: []*github.ReleaseAsset{{
			Name:               github.Ptr("com.lxy.app.clash-v2.0.3.lpk"),
			State:              github.Ptr("uploaded"),
			Size:               github.Ptr(123456),
			Digest:             github.Ptr("sha256:" + digest),
			BrowserDownloadURL: github.Ptr(rejected.DownloadURL),
		}},
	}}
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(defaultGitHubLPKUpdateIntervalMinutes).
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), policy); err == nil || !strings.Contains(err.Error(), "was rejected") {
		t.Fatalf("run update error = %v, want rejected error", err)
	}
	if count := store.server.db.ReviewRequest.Query().
		Where(reviewrequest.VersionIDEQ(rejected.ID), reviewrequest.StatusEQ(reviewrequest.StatusPENDING)).
		CountX(t.Context()); count != 0 {
		t.Fatalf("pending review count = %d, want 0", count)
	}
}

func TestGitHubLPKUpdatePolicyAPIValidatesSupportAndInterval(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	store.login("admin", "changeme")

	rec := store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         true,
		"intervalMinutes": 30,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short interval status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         false,
		"intervalMinutes": 180,
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"intervalMinutes":180`) {
		t.Fatalf("save disabled policy status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = store.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"githubLPKUpdatePolicy"`) {
		t.Fatalf("app detail policy status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubLPKUpdatePolicyDoesNotInvalidateSourceFeed(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/apps/1/github-lpk-update-policy", nil)
	if isSourceFeedMutationRequest(req) {
		t.Fatal("policy-only update should not invalidate the source feed")
	}
}

func TestGitHubLPKUpdatePolicyIsHiddenForNonGitHubLatestVersion(t *testing.T) {
	store := newTestApp(t)
	record, current := createGitHubLPKUpdateFixture(t, store, true)
	store.server.db.AppVersion.UpdateOneID(current.ID).
		SetDownloadURL("https://downloads.example.com/com.lxy.app.clash-v2.0.2.lpk").
		SaveX(t.Context())
	store.login("admin", "changeme")

	rec := store.do(http.MethodGet, fmt.Sprintf("/api/v1/apps/%d", record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("app detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"githubLPKUpdatePolicy"`) {
		t.Fatalf("non-GitHub app detail exposed automatic update policy: %s", rec.Body.String())
	}
	rec = store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         true,
		"intervalMinutes": defaultGitHubLPKUpdateIntervalMinutes,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("enable non-GitHub policy status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubLPKUpdatePolicySaveDoesNotBypassNextCheck(t *testing.T) {
	store := newTestApp(t)
	if err := store.server.githubLPKUpdateScheduler.CloseContext(t.Context()); err != nil {
		t.Fatalf("stop scheduler: %v", err)
	}
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	now := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)
	nextCheckAt := now.Add(6 * time.Hour)
	store.server.setNow(func() time.Time { return now })
	store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(12 * 60).
		SetLastCheckedAt(now).
		SetNextCheckAt(nextCheckAt).
		SaveX(t.Context())
	store.login("admin", "changeme")

	rec := store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         true,
		"intervalMinutes": 6 * 60,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save enabled policy status = %d, body = %s", rec.Code, rec.Body.String())
	}
	policy := store.server.db.GitHubLPKUpdatePolicy.Query().Where(githublpkupdatepolicy.AppIDEQ(record.ID)).OnlyX(t.Context())
	if policy.NextCheckAt == nil || !policy.NextCheckAt.Equal(nextCheckAt) {
		t.Fatalf("next check changed from %v to %v", nextCheckAt, policy.NextCheckAt)
	}
	if policy.IntervalMinutes != 6*60 {
		t.Fatalf("interval = %d, want %d", policy.IntervalMinutes, 6*60)
	}
	rec = store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         false,
		"intervalMinutes": 6 * 60,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable policy status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = store.do(http.MethodPatch, fmt.Sprintf("/api/v1/apps/%d/github-lpk-update-policy", record.ID), map[string]any{
		"enabled":         true,
		"intervalMinutes": 6 * 60,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("re-enable policy status = %d, body = %s", rec.Code, rec.Body.String())
	}
	policy = store.server.db.GitHubLPKUpdatePolicy.Query().Where(githublpkupdatepolicy.AppIDEQ(record.ID)).OnlyX(t.Context())
	if policy.NextCheckAt == nil || !policy.NextCheckAt.Equal(nextCheckAt) {
		t.Fatalf("re-enable bypassed cooldown: next check = %v, want %v", policy.NextCheckAt, nextCheckAt)
	}
}

func TestGitHubLPKUpdateSchedulerRecordsErrorAndNextCheck(t *testing.T) {
	store := newTestApp(t)
	if err := store.server.githubLPKUpdateScheduler.CloseContext(t.Context()); err != nil {
		t.Fatalf("stop scheduler: %v", err)
	}
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	store.server.githubReleases = &fakeGitHubLatestReleaseClient{err: errors.New("rate limited")}
	now := time.Date(2026, 7, 28, 13, 0, 0, 0, time.UTC)
	store.server.setNow(func() time.Time { return now })
	store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(120).
		SetNextCheckAt(now.Add(-time.Minute)).
		SaveX(t.Context())

	scheduler := &githubLPKUpdateScheduler{server: store.server}
	scheduler.runDue(t.Context())
	policy := store.server.db.GitHubLPKUpdatePolicy.Query().Where(githublpkupdatepolicy.AppIDEQ(record.ID)).OnlyX(t.Context())
	if policy.LastCheckedAt == nil || !policy.LastCheckedAt.Equal(now) || policy.NextCheckAt == nil || !policy.NextCheckAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("policy schedule = %+v", policy)
	}
	if !strings.Contains(policy.LastError, "rate limited") || policy.LastSuccessAt != nil {
		t.Fatalf("policy result = %+v", policy)
	}
}

func TestGitHubLPKUpdateClaimHasSingleWinnerAndCanBeReleased(t *testing.T) {
	store := newTestApp(t)
	if err := store.server.githubLPKUpdateScheduler.CloseContext(t.Context()); err != nil {
		t.Fatalf("stop scheduler: %v", err)
	}
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(120).
		SetNextCheckAt(now).
		SaveX(t.Context())

	claimed, ok, err := store.server.claimGitHubLPKUpdatePolicy(t.Context(), policy, now)
	if err != nil || !ok {
		t.Fatalf("first claim = %v, %v", ok, err)
	}
	if _, ok, err := store.server.claimGitHubLPKUpdatePolicy(t.Context(), policy, now); err != nil || ok {
		t.Fatalf("second claim = %v, %v; want no claim", ok, err)
	}
	if err := store.server.releaseGitHubLPKUpdateClaim(t.Context(), claimed, now); err != nil {
		t.Fatalf("release claim: %v", err)
	}
	released := store.server.db.GitHubLPKUpdatePolicy.GetX(t.Context(), policy.ID)
	if released.NextCheckAt == nil || !released.NextCheckAt.Equal(now) {
		t.Fatalf("released next check = %v, want %v", released.NextCheckAt, now)
	}
}

func TestGitHubLPKUpdateDisabledBeforeRunSkipsGitHubAPI(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	fake := &fakeGitHubLatestReleaseClient{}
	store.server.githubReleases = fake
	stale := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(120).
		SaveX(t.Context())
	store.server.db.GitHubLPKUpdatePolicy.UpdateOneID(stale.ID).
		SetEnabled(false).
		ClearNextCheckAt().
		SaveX(t.Context())

	if _, err := store.server.runGitHubLPKUpdate(t.Context(), stale); !errors.Is(err, errGitHubLPKUpdateDisabled) {
		t.Fatalf("run update error = %v, want disabled", err)
	}
	if fake.calls != 0 {
		t.Fatalf("GitHub API calls = %d, want 0", fake.calls)
	}
}

func TestGitHubLPKUpdateUnsupportedErrorKeepsPolicyEnabled(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	now := time.Date(2026, 7, 28, 17, 0, 0, 0, time.UTC)
	policy := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(120).
		SetNextCheckAt(now).
		SaveX(t.Context())

	if err := store.server.finishGitHubLPKUpdate(t.Context(), policy, "", errGitHubLPKUpdateUnsupported, now); err != nil {
		t.Fatalf("finish update: %v", err)
	}
	updated := store.server.db.GitHubLPKUpdatePolicy.GetX(t.Context(), policy.ID)
	if !updated.Enabled || updated.NextCheckAt == nil || !updated.NextCheckAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("unsupported policy schedule = %+v", updated)
	}
	if !strings.Contains(updated.LastError, "not a supported") {
		t.Fatalf("last error = %q", updated.LastError)
	}
}

func TestFinishGitHubLPKUpdatePreservesConcurrentDisable(t *testing.T) {
	store := newTestApp(t)
	record, _ := createGitHubLPKUpdateFixture(t, store, true)
	now := time.Date(2026, 7, 28, 14, 0, 0, 0, time.UTC)
	stale := store.server.db.GitHubLPKUpdatePolicy.Create().
		SetAppID(record.ID).
		SetEnabled(true).
		SetIntervalMinutes(120).
		SetNextCheckAt(now).
		SaveX(t.Context())
	store.server.db.GitHubLPKUpdatePolicy.UpdateOneID(stale.ID).
		SetEnabled(false).
		ClearNextCheckAt().
		SaveX(t.Context())

	if err := store.server.finishGitHubLPKUpdate(t.Context(), stale, "2.0.3", nil, now); err != nil {
		t.Fatalf("finish update: %v", err)
	}
	policy := store.server.db.GitHubLPKUpdatePolicy.GetX(t.Context(), stale.ID)
	if policy.Enabled || policy.NextCheckAt != nil {
		t.Fatalf("concurrently disabled policy was rescheduled: %+v", policy)
	}
	if policy.LastSuccessAt == nil || !policy.LastSuccessAt.Equal(now) || policy.LastVersion != "2.0.3" {
		t.Fatalf("completed check result was not recorded: %+v", policy)
	}
}

func createGitHubLPKUpdateFixture(t *testing.T, store *testApp, allowUnreviewed bool) (*entgo.App, *entgo.AppVersion) {
	t.Helper()
	ctx := t.Context()
	owner := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	return createGitHubLPKUpdateFixtureForOwner(t, store, owner, allowUnreviewed)
}

func createGitHubLPKUpdateFixtureForOwner(t *testing.T, store *testApp, owner *entgo.User, allowUnreviewed bool) (*entgo.App, *entgo.AppVersion) {
	t.Helper()
	ctx := t.Context()
	record := store.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("com.lxy.app.clash").
		SetName("Clash").
		SetSlug(fmt.Sprintf("clash-%d", time.Now().UnixNano())).
		SetStatus(app.StatusAPPROVED).
		SetAllowUnreviewedUpdates(allowUnreviewed).
		SaveX(ctx)
	current := store.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.2").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/wlabbyflower/peppapigconfigurationguide/releases/download/v2.0.2/com.lxy.app.clash-v2.0.2.lpk").
		SetSha256(strings.Repeat("a", 64)).
		SetFileSize(1000).
		SetPublishedAt(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)).
		SaveX(ctx)
	return record, current
}
