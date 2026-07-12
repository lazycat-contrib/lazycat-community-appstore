package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/reviewrequest"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/storage"
)

type recordingDeleteBackend struct {
	storage.Backend
	paths []string
	err   error
}

func TestVersionManagementAuthorizationAndCrossAppIsolation(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("version-auth-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	maintainer := testApp.server.db.User.Create().
		SetUsername("version-auth-maintainer").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	firstApp := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.version-auth-first").
		SetName("Version Auth First").
		SetSlug("version-auth-first").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	secondApp := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.version-auth-second").
		SetName("Version Auth Second").
		SetSlug("version-auth-second").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	testApp.server.db.Collaborator.Create().
		SetAppID(firstApp.ID).
		SetUserID(maintainer.ID).
		SaveX(ctx)
	firstVersion := testApp.server.db.AppVersion.Create().
		SetAppID(firstApp.ID).
		SetUploaderID(owner.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Now()).
		SaveX(ctx)
	secondVersion := testApp.server.db.AppVersion.Create().
		SetAppID(secondApp.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Now()).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(maintainer.ID)}
	rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(firstApp.ID)+"/version-retention", map[string]any{
		"mode":        "CUSTOM",
		"maxVersions": 2,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("collaborator retention status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = testApp.do(
		http.MethodDelete,
		"/api/v1/apps/"+strconv.Itoa(firstApp.ID)+"/versions/"+strconv.Itoa(secondVersion.ID),
		nil,
	)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "VERSION_NOT_FOUND") {
		t.Fatalf("cross-app delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = testApp.do(
		http.MethodDelete,
		"/api/v1/apps/"+strconv.Itoa(firstApp.ID)+"/versions/"+strconv.Itoa(firstVersion.ID),
		nil,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("collaborator delete status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAppDetailShowsRetentionAndPendingVersionsToCollaborator(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("detail-retention-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	maintainer := testApp.server.db.User.Create().
		SetUsername("detail-retention-maintainer").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.detail-retention").
		SetName("Detail Retention").
		SetSlug("detail-retention").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	testApp.server.db.Collaborator.Create().SetAppID(record.ID).SetUserID(maintainer.ID).SaveX(ctx)
	testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.0-rc1").
		SetStatus(appversion.StatusPENDING).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(maintainer.ID)}
	rec := testApp.do(http.MethodGet, "/api/v1/apps/"+strconv.Itoa(record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("collaborator detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		App struct {
			Versions         []version               `json:"versions"`
			VersionRetention *versionRetentionPolicy `json:"versionRetention"`
		} `json:"app"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode collaborator detail: %v", err)
	}
	if len(response.App.Versions) != 1 || response.App.Versions[0].Status != string(appversion.StatusPENDING) {
		t.Fatalf("collaborator versions = %+v", response.App.Versions)
	}
	if response.App.VersionRetention == nil || response.App.VersionRetention.Mode != "INHERIT" {
		t.Fatalf("collaborator retention = %+v", response.App.VersionRetention)
	}

	testApp.cookies = nil
	rec = testApp.do(http.MethodGet, "/api/v1/apps/"+strconv.Itoa(record.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "versionRetention") || strings.Contains(rec.Body.String(), "2.0.0-rc1") {
		t.Fatalf("public detail leaked release management data: %s", rec.Body.String())
	}
}

func TestApprovedExternalVersionsRespectAppRetentionOverride(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.app-retention-override").
		SetName("App Retention Override").
		SetSlug("app-retention-override").
		SetStatus(app.StatusAPPROVED).
		SetVersionRetentionCount(1).
		SaveX(ctx)
	testApp.login("admin", "changeme")
	for index, versionName := range []string{"1.0.0", "2.0.0"} {
		rec := testApp.do(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions", map[string]any{
			"version":     versionName,
			"sourceType":  "GITHUB",
			"downloadUrl": "https://github.com/acme/app/releases/download/v" + strconv.Itoa(index+1) + "/app.lpk",
			"sha256":      strings.Repeat(strconv.Itoa(index+1), 64),
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create version %s status = %d, body = %s", versionName, rec.Code, rec.Body.String())
		}
	}
	approved := testApp.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		AllX(ctx)
	if len(approved) != 1 || approved[0].Version != "2.0.0" {
		t.Fatalf("approved versions = %+v", approved)
	}
}

func TestExternalVersionWithCICDMetadataDoesNotFetchPackage(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.cicd-checksum").
		SetName("CI CD Checksum").
		SetSlug("cicd-checksum").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	testApp.login("admin", "changeme")
	rec := testApp.do(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions", map[string]any{
		"version":     "1.0.0",
		"sourceType":  "WEBDAV",
		"downloadUrl": "http://127.0.0.1:1/releases/app.lpk",
		"sha256":      strings.Repeat("b", 64),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create CI/CD version status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := testApp.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("1.0.0")).
		OnlyX(ctx)
	if created.Sha256 != strings.Repeat("b", 64) {
		t.Fatalf("stored sha256 = %q", created.Sha256)
	}
}

func TestAPITokenRepublishSameVersionUpdatesRebuiltArtifact(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("api-token-rebuild-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.api-token-rebuild").
		SetName("API token rebuild").
		SetSlug("api-token-rebuild").
		SetStatus(app.StatusAPPROVED).
		SetAllowUnreviewedUpdates(true).
		SaveX(ctx)
	token := "lcst_api_token_rebuild"
	testApp.server.db.APIToken.Create().
		SetUserID(owner.ID).
		SetName("rebuild publisher").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)
	publish := func(sha string) *httptest.ResponseRecorder {
		body := `{"version":"1.0.0","sourceType":"GITHUB","downloadUrl":"https://github.com/acme/app/releases/latest/download/app.lpk","sha256":"` + sha + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		testApp.handler.ServeHTTP(rec, req)
		return rec
	}

	firstSHA := strings.Repeat("a", 64)
	if rec := publish(firstSHA); rec.Code != http.StatusCreated {
		t.Fatalf("first publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	first := testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("1.0.0")).OnlyX(ctx)
	firstPublishedAt := *first.PublishedAt

	secondSHA := strings.Repeat("b", 64)
	if rec := publish(secondSHA); rec.Code != http.StatusCreated {
		t.Fatalf("rebuilt publish status = %d, body = %s", rec.Code, rec.Body.String())
	}
	versions := testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("1.0.0")).AllX(ctx)
	if len(versions) != 1 {
		t.Fatalf("same-version records = %d, want 1", len(versions))
	}
	updated := versions[0]
	if updated.ID != first.ID || updated.Sha256 != secondSHA || updated.DownloadURL != "https://github.com/acme/app/releases/latest/download/app.lpk" {
		t.Fatalf("rebuilt version = %+v", updated)
	}
	if updated.PublishedAt == nil || !updated.PublishedAt.After(firstPublishedAt) {
		t.Fatalf("published_at = %v, want after %v", updated.PublishedAt, firstPublishedAt)
	}
}

func TestConcurrentPublicationRetentionAndManualDeletion(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("community.lazycat.concurrent-retention").
		SetName("Concurrent Retention").
		SetSlug("concurrent-retention").
		SetStatus(app.StatusAPPROVED).
		SetVersionRetentionCount(1).
		SaveX(ctx)
	old := testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Now().Add(-time.Hour)).
		SaveX(ctx)
	testApp.login("admin", "changeme")

	type result struct {
		operation string
		status    int
		body      string
	}
	results := make(chan result, 3)
	var waitGroup sync.WaitGroup
	waitGroup.Go(func() {
		rec := testApp.do(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions", map[string]any{
			"version":     "2.0.0",
			"sourceType":  "GITHUB",
			"downloadUrl": "https://github.com/acme/app/releases/download/v2/app.lpk",
			"sha256":      strings.Repeat("a", 64),
		})
		results <- result{operation: "publish", status: rec.Code, body: rec.Body.String()}
	})
	waitGroup.Go(func() {
		rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
			"mode":        "CUSTOM",
			"maxVersions": 1,
		})
		results <- result{operation: "retention", status: rec.Code, body: rec.Body.String()}
	})
	waitGroup.Go(func() {
		rec := testApp.do(
			http.MethodDelete,
			"/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions/"+strconv.Itoa(old.ID),
			nil,
		)
		results <- result{operation: "delete", status: rec.Code, body: rec.Body.String()}
	})
	waitGroup.Wait()
	close(results)
	for item := range results {
		switch item.operation {
		case "publish":
			if item.status != http.StatusCreated {
				t.Fatalf("publish status = %d, body = %s", item.status, item.body)
			}
		case "retention":
			if item.status != http.StatusOK {
				t.Fatalf("retention status = %d, body = %s", item.status, item.body)
			}
		case "delete":
			if item.status != http.StatusOK && item.status != http.StatusNotFound {
				t.Fatalf("delete status = %d, body = %s", item.status, item.body)
			}
		}
	}
	approvedCount := testApp.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		CountX(ctx)
	if approvedCount > 1 {
		t.Fatalf("approved versions after concurrent operations = %d, want at most 1", approvedCount)
	}
}

func (backend *recordingDeleteBackend) Delete(_ context.Context, objectPath string) error {
	backend.paths = append(backend.paths, objectPath)
	return backend.err
}

func TestUpdateVersionRetentionOwnerSetsCustomPolicy(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("retention-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.retention-owner").
		SetName("Retention Owner").
		SetSlug("retention-owner").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}
	rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
		"mode":        "CUSTOM",
		"maxVersions": 3,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update retention status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		VersionRetention versionRetentionPolicy `json:"versionRetention"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.VersionRetention.Mode != "CUSTOM" || response.VersionRetention.EffectiveMaxVersions != 3 {
		t.Fatalf("policy = %+v", response.VersionRetention)
	}
	updated := testApp.server.db.App.GetX(ctx, record.ID)
	if updated.VersionRetentionCount == nil || *updated.VersionRetentionCount != 3 {
		t.Fatalf("version retention = %v, want 3", updated.VersionRetentionCount)
	}
}

func TestVersionRetentionValidationRejectsAmbiguousInputs(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("retention-validation-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.retention-validation").
		SetName("Retention Validation").
		SetSlug("retention-validation").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}

	tests := []struct {
		name  string
		input map[string]any
	}{
		{name: "inherit with count", input: map[string]any{"mode": "INHERIT", "maxVersions": 2}},
		{name: "custom without count", input: map[string]any{"mode": "CUSTOM"}},
		{name: "custom negative", input: map[string]any{"mode": "CUSTOM", "maxVersions": -1}},
		{name: "unknown mode", input: map[string]any{"mode": "DEFAULT"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", test.input)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestVersionRetentionUnlimitedThenInheritCurrentSitePolicy(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("retention-modes-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.retention-modes").
		SetName("Retention Modes").
		SetSlug("retention-modes").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	for index, versionName := range []string{"1.0.0", "2.0.0"} {
		publishedAt := time.Date(2026, 7, 10, index+1, 0, 0, 0, time.UTC)
		testApp.server.db.AppVersion.Create().
			SetAppID(record.ID).
			SetUploaderID(owner.ID).
			SetVersion(versionName).
			SetStatus(appversion.StatusAPPROVED).
			SetPublishedAt(publishedAt).
			SetCreatedAt(publishedAt).
			SaveX(ctx)
	}
	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}
	rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
		"mode":        "CUSTOM",
		"maxVersions": 0,
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"effectiveMaxVersions":0`) {
		t.Fatalf("unlimited status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if count := testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(ctx); count != 2 {
		t.Fatalf("versions after unlimited policy = %d, want 2", count)
	}
	if err := testApp.server.setSetting(ctx, settingMaxVersions, "1"); err != nil {
		t.Fatal(err)
	}
	rec = testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
		"mode": "INHERIT",
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"effectiveMaxVersions":1`) {
		t.Fatalf("inherit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := testApp.server.db.App.GetX(ctx, record.ID)
	if updated.VersionRetentionCount != nil {
		t.Fatalf("inherited retention stored override = %v", updated.VersionRetentionCount)
	}
	if count := testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).CountX(ctx); count != 1 {
		t.Fatalf("versions after inherit policy = %d, want 1", count)
	}
}

func TestUpdateVersionRetentionCleanupFailureDoesNotRollbackPrune(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	backend := &recordingDeleteBackend{err: errors.New("storage unavailable")}
	testApp.server.storage = backend
	owner := testApp.server.db.User.Create().
		SetUsername("retention-cleanup-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.retention-cleanup").
		SetName("Retention Cleanup").
		SetSlug("retention-cleanup").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	base := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	old := testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(base).
		SetCreatedAt(base).
		SetStorageKey("missing-storage").
		SetStoragePath("apps/retention-cleanup/1.0.0.lpk").
		SaveX(ctx)
	testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("2.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(base.Add(time.Hour)).
		SetCreatedAt(base.Add(time.Hour)).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}
	rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
		"mode":        "CUSTOM",
		"maxVersions": 1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update retention status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		PrunedVersions []deletedVersionResult `json:"prunedVersions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.PrunedVersions) != 1 || response.PrunedVersions[0].Warning == nil {
		t.Fatalf("pruned versions = %+v", response.PrunedVersions)
	}
	if testApp.server.db.AppVersion.Query().Where(appversion.IDEQ(old.ID)).ExistX(ctx) {
		t.Fatal("pruned version was rolled back after cleanup failure")
	}
	if len(backend.paths) != 1 || backend.paths[0] != old.StoragePath {
		t.Fatalf("deleted paths = %v", backend.paths)
	}
}

func TestDeleteLatestVersionKeepsReviewAuditAndRemovesOnlyVersion(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("version-delete-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.version-delete").
		SetName("Version Delete").
		SetSlug("version-delete").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	versionRecord := testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetPublishedAt(time.Now()).
		SaveX(ctx)
	review := testApp.server.db.ReviewRequest.Create().
		SetKind(reviewrequest.KindVERSION_UPLOAD).
		SetStatus(reviewrequest.StatusAPPROVED).
		SetAppID(record.ID).
		SetVersionID(versionRecord.ID).
		SetRequesterID(owner.ID).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}
	rec := testApp.do(
		http.MethodDelete,
		"/api/v1/apps/"+strconv.Itoa(record.ID)+"/versions/"+strconv.Itoa(versionRecord.ID),
		nil,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete latest status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if testApp.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID)).ExistX(ctx) {
		t.Fatal("version still exists")
	}
	keptReview := testApp.server.db.ReviewRequest.GetX(ctx, review.ID)
	if keptReview.VersionID != nil {
		t.Fatalf("review version id = %v, want nil", keptReview.VersionID)
	}
}

func TestUpdateVersionRetentionImmediatelyPrunesOldApprovedVersions(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	owner := testApp.server.db.User.Create().
		SetUsername("retention-prune-owner").
		SetPasswordHash("unused").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("community.lazycat.retention-prune").
		SetName("Retention Prune").
		SetSlug("retention-prune").
		SetStatus(app.StatusAPPROVED).
		SaveX(ctx)
	base := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
	for index, versionName := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		publishedAt := base.Add(time.Duration(index) * time.Hour)
		testApp.server.db.AppVersion.Create().
			SetAppID(record.ID).
			SetUploaderID(owner.ID).
			SetVersion(versionName).
			SetStatus(appversion.StatusAPPROVED).
			SetPublishedAt(publishedAt).
			SetCreatedAt(publishedAt).
			SaveX(ctx)
	}
	testApp.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion("4.0.0-rc1").
		SetStatus(appversion.StatusPENDING).
		SaveX(ctx)

	testApp.cookies = []*http.Cookie{testApp.serverCookieFor(owner.ID)}
	rec := testApp.do(http.MethodPatch, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/version-retention", map[string]any{
		"mode":        "CUSTOM",
		"maxVersions": 1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update retention status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		PrunedVersions []deletedVersionResult `json:"prunedVersions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.PrunedVersions) != 2 {
		t.Fatalf("pruned versions = %d, body = %s", len(response.PrunedVersions), rec.Body.String())
	}
	remaining := testApp.server.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID)).
		Order().
		AllX(ctx)
	if len(remaining) != 2 {
		t.Fatalf("remaining versions = %d, want approved latest plus pending", len(remaining))
	}
	remainingNames := map[string]bool{}
	for _, item := range remaining {
		remainingNames[item.Version] = true
	}
	if !remainingNames["3.0.0"] || !remainingNames["4.0.0-rc1"] {
		t.Fatalf("remaining versions = %v", remainingNames)
	}
}
