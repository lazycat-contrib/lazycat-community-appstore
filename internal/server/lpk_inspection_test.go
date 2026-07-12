package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/lpkinspectionjob"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/lpkinspect"
)

func TestBulkLPKInspectionQueuesEligibleOwnedAppsAndProtectsStatus(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	store.login("admin", "changeme")
	owner := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	eligible := store.server.db.App.Create().SetOwnerID(owner.ID).SetPackageID("cloud.lazycat.test.bulk-eligible").SetName("Eligible").SetSlug("bulk-eligible").SaveX(ctx)
	store.server.db.AppVersion.Create().SetAppID(eligible.ID).SetUploaderID(owner.ID).SetVersion("1.0.0").SetDownloadURL("https://example.test/eligible.lpk").SetSourceType(appversion.SourceTypeGITHUB).SaveX(ctx)
	ineligible := store.server.db.App.Create().SetOwnerID(owner.ID).SetPackageID("cloud.lazycat.test.bulk-ineligible").SetName("Ineligible").SetSlug("bulk-ineligible").SaveX(ctx)

	rec := store.do(http.MethodPost, "/api/v1/me/apps/lpk-inspections", map[string]any{"appIds": []int{eligible.ID, ineligible.ID}, "overwriteExistingMetadata": false})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("bulk enqueue status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response bulkLPKInspectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Inspections) != 1 || response.Inspections[0].AppID != eligible.ID {
		t.Fatalf("inspections = %#v", response.Inspections)
	}
	if len(response.Skipped) != 1 || response.Skipped[0].AppID != ineligible.ID {
		t.Fatalf("skipped = %#v", response.Skipped)
	}

	outsider := store.server.db.User.Create().SetUsername("bulk-outsider").SetPasswordHash("x").SetEmailVerified(true).SaveX(ctx)
	token := "lcst_bulk_outsider"
	store.server.db.APIToken.Create().SetUserID(outsider.ID).SetName("bulk outsider").SetPrefix(tokenPrefix(token)).SetTokenHash(tokenHash(token)).SaveX(ctx)
	statusBody, err := json.Marshal(bulkLPKInspectionStatusRequest{IDs: []int{response.Inspections[0].Inspection.ID}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/apps/lpk-inspections/status", bytes.NewReader(statusBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	statusRec := httptest.NewRecorder()
	store.handler.ServeHTTP(statusRec, req)
	if statusRec.Code != http.StatusOK || strings.Contains(statusRec.Body.String(), `"appId":`+strconv.Itoa(eligible.ID)) {
		t.Fatalf("outsider status = %d, body = %s", statusRec.Code, statusRec.Body.String())
	}
}

func TestApplyLPKInspectionMetadataFillsAndOverwritesApplicationMetadata(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	owner := store.server.db.User.Create().SetUsername("metadata-owner").SetPasswordHash("x").SaveX(ctx)
	record := store.server.db.App.Create().
		SetOwnerID(owner.ID).
		SetPackageID("cloud.lazycat.test.metadata-inspection").
		SetName("Metadata inspection").
		SetSlug("metadata-inspection").
		SetAuthor("Existing author").
		SaveX(ctx)
	job := store.server.db.LPKInspectionJob.Create().
		SetAppID(record.ID).
		SetUserID(owner.ID).
		SetDownloadURL("https://example.test/app.lpk").
		SetTrigger(lpkinspectionjob.TriggerMANUAL).
		SetState(lpkinspectionjob.StateRUNNING).
		SetDeadlineAt(time.Now().Add(time.Minute)).
		SetOverwriteExistingMetadata(false).
		SaveX(ctx)
	metadata := lpkinspect.Metadata{
		PackageID:    record.PackageID,
		Author:       "LPK author",
		Homepage:     "https://example.com/app",
		License:      "MIT",
		MinOSVersion: "1.3.0",
	}
	if err := store.server.applyLPKInspectionMetadata(ctx, job, lpkInspection{Metadata: metadata}); err != nil {
		t.Fatal(err)
	}
	filled := store.server.db.App.GetX(ctx, record.ID)
	if filled.Author != "Existing author" || filled.Homepage != metadata.Homepage || filled.License != metadata.License || filled.MinOsVersion != metadata.MinOSVersion {
		t.Fatalf("fill-missing metadata = %+v", filled)
	}
	job = job.Update().SetOverwriteExistingMetadata(true).SaveX(ctx)
	if err := store.server.applyLPKInspectionMetadata(ctx, job, lpkInspection{Metadata: metadata}); err != nil {
		t.Fatal(err)
	}
	overwritten := store.server.db.App.GetX(ctx, record.ID)
	if overwritten.Author != metadata.Author {
		t.Fatalf("overwrite metadata = %+v", overwritten)
	}
}

func TestLPKInspectionJobLifecycle(t *testing.T) {
	store := newTestApp(t)
	job := store.server.db.LPKInspectionJob.Create().
		SetAppID(1).
		SetUserID(1).
		SetDownloadURL("https://example.test/app.lpk").
		SetTrigger(lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION).
		SetState(lpkinspectionjob.StatePENDING).
		SetDeadlineAt(time.Now().Add(30 * time.Second)).
		SaveX(t.Context())
	if job.Attempts != 0 || job.State != lpkinspectionjob.StatePENDING {
		t.Fatalf("job = %#v", job)
	}
}

func TestTemporaryLPKInspectionErrorRetriesUnavailableHTTPResponse(t *testing.T) {
	if !temporaryLPKInspectionError(errors.New("LPK URL returned HTTP 404")) {
		t.Fatal("an unavailable release asset must be retried within the inspection deadline")
	}
}

func TestLPKInspectionReschedulesUnavailableHTTPResponse(t *testing.T) {
	store := newTestApp(t)
	now := time.Now()
	job := store.server.db.LPKInspectionJob.Create().
		SetAppID(1).
		SetUserID(1).
		SetDownloadURL("https://example.test/app.lpk").
		SetTrigger(lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION).
		SetState(lpkinspectionjob.StatePENDING).
		SetDeadlineAt(now.Add(30 * time.Second)).
		SetNextAttemptAt(now.Add(time.Hour)).
		SaveX(t.Context())
	store.server.inspectLPKForJob = func(context.Context, string, int64, bool, time.Duration) (lpkInspection, error) {
		return lpkInspection{}, errors.New("LPK URL returned HTTP 404")
	}

	if err := store.server.runLPKInspectionJob(t.Context(), job.ID, now); err != nil {
		t.Fatal(err)
	}
	updated := store.server.db.LPKInspectionJob.GetX(t.Context(), job.ID)
	if updated.State != lpkinspectionjob.StatePENDING || updated.Attempts != 1 || updated.NextAttemptAt == nil || !updated.NextAttemptAt.After(now) {
		t.Fatalf("unavailable release job = %#v", updated)
	}
}

func TestAPITokenExternalCreateQueuesLPKInspectionWithoutSynchronousFetch(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	store.server.inspectLPKForJob = blockLPKInspectionUntilCancelled
	publisher := store.server.db.User.Create().
		SetUsername("ci-inspection").
		SetPasswordHash("x").
		SetEmailVerified(true).
		SaveX(ctx)
	token := "lcst_ci_inspection_token"
	store.server.db.APIToken.Create().
		SetUserID(publisher.ID).
		SetName("CI inspection").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", strings.NewReader(`{
  "packageId":"cloud.lazycat.test.queued-inspection",
  "name":"Queued inspection",
  "version":"1.0.0",
  "summary":"CI metadata",
  "sourceType":"GITHUB",
  "downloadUrl":"https://github.com/acme/queued/releases/download/v1/app.lpk",
  "sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	store.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("API token create status = %d, body = %s", rec.Code, rec.Body.String())
	}

	created := store.server.db.LPKInspectionJob.Query().OnlyX(ctx)
	if created.Trigger != lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION || (created.State != lpkinspectionjob.StatePENDING && created.State != lpkinspectionjob.StateRUNNING) {
		t.Fatalf("inspection job = %#v", created)
	}
	if created.DeadlineAt == nil || created.DeadlineAt.Sub(created.CreatedAt) > 30*time.Second+time.Second {
		t.Fatalf("inspection deadline = %v, created = %v", created.DeadlineAt, created.CreatedAt)
	}
}

func TestAPITokenExternalCreateSkipsAutomaticLPKInspectionWhenDisabled(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	if err := store.server.setSetting(ctx, settingAutomaticLPKInspectionWaitSeconds, "0"); err != nil {
		t.Fatal(err)
	}
	publisher := store.server.db.User.Create().
		SetUsername("ci-inspection-disabled").
		SetPasswordHash("x").
		SetEmailVerified(true).
		SaveX(ctx)
	token := "lcst_ci_inspection_disabled_token"
	store.server.db.APIToken.Create().
		SetUserID(publisher.ID).
		SetName("CI disabled").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps", strings.NewReader(`{
  "packageId":"cloud.lazycat.test.disabled-inspection",
  "name":"Disabled inspection",
  "version":"1.0.0",
  "summary":"CI metadata",
  "sourceType":"GITHUB",
  "downloadUrl":"https://github.com/acme/disabled/releases/download/v1/app.lpk",
  "sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	store.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("API token create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := store.server.db.LPKInspectionJob.Query().CountX(ctx); got != 0 {
		t.Fatalf("inspection jobs = %d, want 0", got)
	}
}

func TestManualLPKInspectionRequiresVersionUploadPermission(t *testing.T) {
	store := newTestApp(t)
	ctx := t.Context()
	store.server.inspectLPKForJob = blockLPKInspectionUntilCancelled
	store.login("admin", "changeme")
	create := store.do(http.MethodPost, "/api/v1/apps", map[string]any{
		"packageId":   "cloud.lazycat.test.manual-inspection",
		"name":        "Manual inspection",
		"version":     "1.0.0",
		"sourceType":  "GITHUB",
		"downloadUrl": "https://github.com/acme/manual/releases/download/v1/app.lpk",
		"sha256":      strings.Repeat("c", 64),
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create app status = %d, body = %s", create.Code, create.Body.String())
	}
	record := store.server.db.App.Query().OnlyX(ctx)
	outsider := store.server.db.User.Create().
		SetUsername("inspection-outsider").
		SetPasswordHash("x").
		SetEmailVerified(true).
		SetRole(user.RoleUSER).
		SaveX(ctx)
	token := "lcst_inspection_outsider_token"
	store.server.db.APIToken.Create().
		SetUserID(outsider.ID).
		SetName("outsider").
		SetPrefix(tokenPrefix(token)).
		SetTokenHash(tokenHash(token)).
		SaveX(ctx)

	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/lpk-inspections", bytes.NewBufferString(`{"overwriteExistingMetadata":false}`))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			for _, cookie := range store.cookies {
				req.AddCookie(cookie)
			}
		}
		rec := httptest.NewRecorder()
		store.handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(token); rec.Code != http.StatusForbidden {
		t.Fatalf("outsider manual inspection status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec := request(""); rec.Code != http.StatusAccepted {
		t.Fatalf("owner manual inspection status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := store.server.db.LPKInspectionJob.Query().CountX(ctx); got != 1 {
		t.Fatalf("inspection jobs = %d, want 1", got)
	}
	detail := store.do(http.MethodGet, "/api/v1/apps/"+strconv.Itoa(record.ID), nil)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"lpkInspection"`) {
		t.Fatalf("detail status = %d, body = %s", detail.Code, detail.Body.String())
	}
}

func blockLPKInspectionUntilCancelled(ctx context.Context, _ string, _ int64, _ bool, _ time.Duration) (lpkInspection, error) {
	<-ctx.Done()
	return lpkInspection{}, ctx.Err()
}
