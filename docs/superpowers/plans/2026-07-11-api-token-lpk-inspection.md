# API-Token LPK Inspection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create API-token first submissions without waiting for a newly published LPK, then safely enrich their details through a bounded asynchronous inspection job.

**Architecture:** Persist an app-scoped inspection job and run it through a small in-process scheduler. Reuse `internal/lpkinspect` for URL policy, bounded download, checksum, and metadata parsing; automatic jobs fill empty fields only, while an explicit manual job may overwrite metadata.

**Tech Stack:** Go 1.26, Ent, net/http, existing server settings and React management UI, OpenAPI 3.0.

## Global Constraints

- Trigger only after a successful API-token `POST /api/v1/apps` that creates a new app with an external LPK URL.
- Automatic inspection waits at most the configured total window: `0..300` seconds, default `30`; `0` disables future jobs.
- Automatic work fills only empty fields and never replaces explicit CI/CD metadata.
- Use existing LPK URL/size/SSRF constraints; retry temporary availability failures only.
- Owners, collaborators, and administrators may manually inspect; overwrite needs explicit `true`.
- Do not add a queue dependency or run local-device installation on the store server.

---

## File Structure

- `ent/schema/lpk_inspection_job.go`: durable state, deadline, retry, ownership, trigger, and URL snapshot.
- `internal/server/lpk_inspection.go`: scheduler, retry classifier, idempotent state transitions, and parsed-metadata application.
- `internal/server/lpk_inspection_test.go`: lifecycle, authorization, metadata, and restart tests.
- `internal/server/handlers_apps.go`, `auth.go`, `server.go`: trigger, manual route, API-token context marker, and lifecycle.
- `internal/server/settings.go`, `handlers_admin.go`, `types.go`: site setting/DTO support.
- management React files, locales, and `docs/openapi.yaml`: status/action/settings contracts.

### Task 1: Persist inspection jobs and wire scheduler lifecycle

**Files:**
- Create: `ent/schema/lpk_inspection_job.go`
- Modify: generated `ent/` files via `go generate ./ent`
- Modify: `internal/server/server.go`
- Create: `internal/server/lpk_inspection_test.go`

**Interfaces:**
- Produces `LPKInspectionJob` with `pending`, `running`, `succeeded`, `failed`, `timed_out`, `cancelled` states.
- Produces `newLPKInspectionScheduler(*Server) (*lpkInspectionScheduler, error)` and `CloseContext(context.Context) error`.

- [ ] **Step 1: Add the failing model/lifecycle test**

```go
func TestLPKInspectionJobLifecycle(t *testing.T) {

	store := newTestApp(t)
	job := store.server.db.LPKInspectionJob.Create().
		SetAppID(1).SetUserID(1).SetDownloadURL("https://example.test/app.lpk").
		SetTrigger(lpkinspectionjob.TriggerAPI_TOKEN_FIRST_SUBMISSION).
		SetState(lpkinspectionjob.StatePENDING).
		SetDeadlineAt(time.Now().Add(30 * time.Second)).SaveX(t.Context())
	if job.Attempts != 0 || job.State != lpkinspectionjob.StatePENDING {
		t.Fatalf("job = %#v", job)
	}
}
```

- [ ] **Step 2: Prove it fails before generation**

Run: `go test ./internal/server -run '^TestLPKInspectionJobLifecycle$' -count=1`

Expected: FAIL because `LPKInspectionJob` is undefined.

- [ ] **Step 3: Define and generate the Ent model**

Include `app_id`, optional `version_id`, `user_id`, `download_url`, enum `trigger`, enum `state`, `overwrite_existing_metadata`, `attempts`, optional `last_error`, optional `next_attempt_at`, optional `deadline_at`, optional `completed_at`, and timestamps. Add indexes on `(state, next_attempt_at)` and `(app_id, state)`. Run `go generate ./ent`. Add a scheduler field to `Server`, start it after routes are registered, and close it alongside other server resources.

- [ ] **Step 4: Verify the durable foundation**

Run: `go test ./internal/server -run '^TestLPKInspectionJobLifecycle$' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ent internal/server/server.go internal/server/lpk_inspection_test.go
git commit -m "feat: add LPK inspection jobs"
```

### Task 2: Implement bounded retry and metadata semantics

**Files:**
- Create: `internal/server/lpk_inspection.go`
- Modify: `internal/server/lpk_fetch.go`
- Modify: `internal/server/handlers_apps.go`
- Test: `internal/server/lpk_inspection_test.go`

**Interfaces:**
- Produces `enqueueAutomaticLPKInspection(ctx context.Context, appID, versionID, userID int, downloadURL string) error`.
- Produces `runLPKInspectionJob(ctx context.Context, jobID int, now time.Time) error`.
- Produces `enqueueManualLPKInspection(ctx context.Context, appID, userID int, overwrite bool) (*ent.LPKInspectionJob, error)`.

- [ ] **Step 1: Write failing worker tests**

```go
func TestAutomaticInspectionTimesOutAfterConfiguredWindow(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	store := newTestApp(t)
	store.server.now = func() time.Time { return now }
	store.server.inspectLPK = func(context.Context, string, int64, bool) (lpkInspection, error) { return lpkInspection{}, errTemporaryLPKUnavailable }
	job := createInspectionJob(t, store, now.Add(30*time.Second), false)
	for _, next := range []time.Time{now, now.Add(time.Second), now.Add(4*time.Second), now.Add(11*time.Second), now.Add(30*time.Second)} {
		now = next; _ = store.server.runLPKInspectionJob(t.Context(), job.ID, now)
	}
	if got := reloadInspectionJob(t, store, job.ID).State; got != lpkinspectionjob.StateTIMED_OUT { t.Fatalf("state = %s", got) }
}
func TestAutomaticInspectionOnlyFillsEmptyMetadata(t *testing.T) {
	app, version := createAppWithVersion(t, "CI name", "1.0.0", "a"+strings.Repeat("0", 63))
	applySuccessfulInspection(t, app, version, false, lpkmeta.Metadata{Name: "LPK name", PackageID: app.PackageID, Version: "2.0.0"})
	if got := reloadApp(t, app.ID).Name; got != "CI name" { t.Fatalf("name = %q", got) }
	if got := reloadVersion(t, version.ID).Version; got != "1.0.0" { t.Fatalf("version = %q", got) }
}
func TestManualInspectionOverwritesOnlyWithConsent(t *testing.T) {
	app, version := createAppWithVersion(t, "CI name", "1.0.0", "b"+strings.Repeat("0", 63))
	applySuccessfulInspection(t, app, version, false, lpkmeta.Metadata{Name: "LPK name", PackageID: app.PackageID, Version: "2.0.0"})
	if got := reloadApp(t, app.ID).Name; got != "CI name" { t.Fatalf("non-overwrite name = %q", got) }
	applySuccessfulInspection(t, app, version, true, lpkmeta.Metadata{Name: "LPK name", PackageID: app.PackageID, Version: "2.0.0"})
	if got := reloadApp(t, app.ID).Name; got != "LPK name" { t.Fatalf("overwrite name = %q", got) }
}
func TestMalformedLPKIsTerminal(t *testing.T) {
	job := createInspectionJob(t, newTestApp(t), time.Now().Add(time.Minute), false)
	finishInspectionWithError(t, job, errors.New("invalid LPK archive"))
	if job.State != lpkinspectionjob.StateFAILED || job.NextAttemptAt != nil { t.Fatalf("job = %#v", job) }
}
func TestInspectionRecoverySkipsExpiredJobs(t *testing.T) {
	store := newTestApp(t); expired := createInspectionJob(t, store, time.Now().Add(-time.Second), false)
	store.server.lpkInspectionScheduler.resumePending(t.Context())
	if got := reloadInspectionJob(t, store, expired.ID).State; got != lpkinspectionjob.StateTIMED_OUT { t.Fatalf("state = %s", got) }
}
```

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/server -run '^(TestAutomaticInspection|TestManualInspection|TestMalformedLPK|TestInspectionRecovery)' -count=1`

Expected: FAIL because the worker does not exist.

- [ ] **Step 3: Implement exact state transitions**

Use a per-app lock plus conditional `pending -> running` update. Try immediately, then use 1s, 3s, 7s, and 15s delays while `now < deadline_at`; do not schedule after the deadline. Network connection errors and transient unavailable responses retry; malformed archives, package mismatch, URL-policy, and size-limit errors become terminal `failed`. A successful automatic job updates only blank app fields and blank linked first-version fields; a manual job overwrites only when its boolean flag is true.

```go
if !now.Before(job.DeadlineAt) {

	return s.finishLPKInspection(ctx, job, lpkinspectionjob.StateTIMED_OUT, "LPK was not reachable before deadline")
}
```

- [ ] **Step 4: Re-run worker tests**

Run: `go test ./internal/server -run '^(TestAutomaticInspection|TestManualInspection|TestMalformedLPK|TestInspectionRecovery)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/lpk_inspection.go internal/server/lpk_inspection_test.go internal/server/lpk_fetch.go internal/server/handlers_apps.go
git commit -m "feat: inspect LPKs asynchronously"
```

### Task 3: Add API-token trigger, settings, and manual management endpoint

**Files:**
- Modify: `internal/server/auth.go`, `internal/server/handlers_apps.go`, `internal/server/server.go`
- Modify: `internal/server/settings.go`, `internal/server/handlers_admin.go`, `internal/server/types.go`
- Modify: `docs/openapi.yaml`
- Modify: app management React component(s) and `client/src/locales/{zh,en}.ts`
- Test: `internal/server/lpk_inspection_test.go`, `internal/server/server_test.go`

**Interfaces:**
- Adds context helper `authenticatedByAPIToken(context.Context) bool`.
- Adds `effectiveAPITokenInitialLPKInspectionTimeout(context.Context) time.Duration`.
- Adds `POST /api/v1/apps/{id}/lpk-inspection` body `{"overwriteExistingMetadata":false}`.
- Adds latest inspection state to app detail DTO.

- [ ] **Step 1: Write failing API tests**

```go
func TestAPITokenFirstAppSubmissionReturns201AndEnqueuesJob(t *testing.T) {
	store, token := newTokenTestApp(t)
	rec := store.doBearer(http.MethodPost, "/api/v1/apps", token, firstExternalAppJSON())
	if rec.Code != http.StatusCreated { t.Fatalf("status = %d: %s", rec.Code, rec.Body.String()) }
	if count := store.server.db.LPKInspectionJob.Query().CountX(t.Context()); count != 1 { t.Fatalf("jobs = %d", count) }
}
func TestCookieFirstAppSubmissionDoesNotEnqueueJob(t *testing.T) {
	store := newTestApp(t); rec := store.do(http.MethodPost, "/api/v1/apps", firstExternalAppJSON())
	if rec.Code != http.StatusCreated { t.Fatalf("status = %d", rec.Code) }
	if count := store.server.db.LPKInspectionJob.Query().CountX(t.Context()); count != 0 { t.Fatalf("jobs = %d", count) }
}
func TestInspectionTimeoutSettingClampsAndDisables(t *testing.T) {
	store := newTestApp(t)
	for raw, want := range map[string]int{"0": 0, "30": 30, "301": 300} { store.server.setSetting(t.Context(), settingAPITokenInitialLPKInspectionTimeoutSeconds, raw); if got := int(store.server.effectiveAPITokenInitialLPKInspectionTimeout(t.Context()).Seconds()); got != want { t.Fatalf("%s => %d", raw, got) } }
}
func TestManualInspectionRequiresUploadPermission(t *testing.T) {
	store, appID, ownerToken, outsiderToken := newInspectionAccessFixture(t)
	if rec := store.doBearer(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/lpk-inspection", appID), ownerToken, map[string]bool{"overwriteExistingMetadata": false}); rec.Code != http.StatusAccepted { t.Fatalf("owner = %d", rec.Code) }
	if rec := store.doBearer(http.MethodPost, fmt.Sprintf("/api/v1/apps/%d/lpk-inspection", appID), outsiderToken, map[string]bool{"overwriteExistingMetadata": false}); rec.Code != http.StatusForbidden { t.Fatalf("outsider = %d", rec.Code) }
}
```

- [ ] **Step 2: Run focused API tests**

Run: `go test ./internal/server -run '^(TestAPITokenFirstAppSubmission|TestCookieFirstAppSubmission|TestInspectionTimeoutSetting|TestManualInspectionRequires)' -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement the API surface**

Mark only successful bearer API-token authentication in context. After a new app plus its first external version persist, enqueue one automatic job when the context marker and positive timeout exist; log enqueue errors but preserve the `201`. Expose the 0..300 setting through existing admin settings. The protected manual endpoint authorizes with `canUploadVersion`, reuses an active job for the same app, and returns its DTO. Add management status/error/action UI, including a separate confirmation for overwrite.

- [ ] **Step 4: Verify contracts**

Run:

```bash
go test ./internal/server -run '^(TestAPITokenFirstAppSubmission|TestCookieFirstAppSubmission|TestInspectionTimeoutSetting|TestManualInspectionRequires)' -count=1
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
cd client && npm run build
```

Expected: all commands pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server client/src docs/openapi.yaml
git commit -m "feat: manage API-token LPK inspection"
```

### Task 4: Run final server verification

- [ ] **Step 1: Run repository checks**

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
git diff --check
```

- [ ] **Step 2: Commit any final test/documentation-only changes**

```bash
git add internal/server docs/openapi.yaml
git commit -m "test: cover LPK inspection recovery"
```
