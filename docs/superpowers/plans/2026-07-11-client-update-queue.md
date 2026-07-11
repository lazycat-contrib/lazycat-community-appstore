# Client Update Queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe one-click and scheduled updates for locally installed source applications, and make installation progress visible inside the initiating modal.

**Architecture:** Reuse client source sync, installed-app lookup, and `InstallLPK` to build a user-scoped sequential update queue. Persist automatic-update settings next to the existing sync settings; both manual and timer triggers use the same exclusion lock and result DTO.

**Tech Stack:** Go 1.26, Ent, LazyCat Go SDK package manager, React/TypeScript, Astryx UI, i18next.

## Global Constraints

- Bulk/scheduled work installs only applications with a newer source version and no install password.
- Sync sources then read local installed apps before calculating eligibility.
- Queue is sequential; failures are recorded and later eligible apps continue.
- Cancellation forwards the active LazyCat `taskId` to `CancelPendingTask` and prevents remaining queue items from starting.
- One update/interactive install operation per client user at a time.
- Installed-device data remains in the client server.
- Use only short opacity/transform UI transitions, active press feedback, and reduced-motion fallbacks.

---

## File Structure

- `ent/schema/client_sync_setting.go`: auto-update controls and last-run persistence.
- `internal/clientserver/update_queue.go`: eligibility, queue execution/cancellation, per-user exclusion, and result records.
- `internal/clientserver/update_queue_test.go`: candidates, continuation, concurrency, and scheduler tests.
- `internal/clientserver/install.go`, `scheduler.go`, `settings.go`, `types.go`, `server.go`: shared install guard, due runs, settings, DTOs, and manual route.
- `client/src/modules/client/InstalledAppsView.tsx`: bulk button, confirmation, and sticky summary.
- `client/src/modules/client/InstallOptionsDialog.tsx`, `SourceAppDetailPage.tsx`, `SourceAppGrid.tsx`: modal-owned activity state for detail and install/list entry points.
- `client/src/modules/client/ClientSettingsView.tsx`, `ClientCatalog.tsx`, `clientUxState.ts`: automation controls and API state.
- `client/src/shared/types.ts`, `client/src/locales/{zh,en}.ts`, relevant client CSS: types, copy, and visual behavior.

### Task 1: Build the server-side update queue

**Files:**
- Create: `internal/clientserver/update_queue.go`
- Create: `internal/clientserver/update_queue_test.go`
- Modify: `internal/clientserver/install.go`, `internal/clientserver/types.go`

**Interfaces:**
- Produces `UpdateQueueItemDTO`, `UpdateQueueResultDTO`, and `RunUpdateQueue(ctx context.Context, userID string) UpdateQueueResultDTO`.
- Extends `PackageManager` with `CancelInstall(ctx context.Context, userID, taskID string) error`.
- Extends `PackageManager` with `GetInstallTask(ctx context.Context, userID, taskID string) (InstallTaskDTO, error)`.
- Changes `POST /api/client/v1/install` to return `202` with `InstallTaskDTO`; adds `GET` and `DELETE /api/client/v1/install-tasks/{taskId}`.
- Produces `eligibleUpdates(installed []InstalledApplicationDTO, apps []*ent.ClientSourceApp) []updateCandidate`.

- [ ] **Step 1: Write failing eligibility and queue tests**

```go
func TestEligibleUpdatesSkipsProtectedCurrentAndUnknownApps(t *testing.T) {
	installed := []InstalledApplicationDTO{{AppID: "eligible", Version: "1.0.0"}, {AppID: "protected", Version: "1.0.0"}, {AppID: "current", Version: "2.0.0"}, {AppID: "unknown", Version: "1.0.0"}}
	candidates := eligibleUpdates(installed, sourceAppsForTest(t, "eligible", "2.0.0", false, "protected", "2.0.0", true, "current", "2.0.0", false))
	if len(candidates) != 1 || candidates[0].PackageID != "eligible" { t.Fatalf("candidates = %#v", candidates) }
}
func TestUpdateQueueContinuesAfterFailure(t *testing.T) {
	store := newTestApp(t); store.server.pkg = &fakePackageManager{installErrors: []error{errors.New("first failed"), nil}}
	result := store.server.RunUpdateQueue(t.Context(), "alice")
	if got := []string{result.Items[0].Status, result.Items[1].Status}; !slices.Equal(got, []string{"failed", "success"}) { t.Fatalf("statuses = %#v", got) }
}
func TestUpdateQueueRejectsConcurrentUserRun(t *testing.T) {
	store := newTestApp(t); release := make(chan struct{}); store.server.pkg = blockingPackageManager{release: release}
	go store.server.RunUpdateQueue(t.Context(), "alice")
	if result := store.server.RunUpdateQueue(t.Context(), "alice"); result.Status != "already_running" { t.Fatalf("status = %q", result.Status) }
	close(release)
}
func TestCancelUpdateQueueCancelsActiveTaskAndRemainingItems(t *testing.T) {
	store := newTestApp(t); started := make(chan struct{}); release := make(chan struct{})
	store.server.pkg = blockingPackageManager{started: started, release: release, taskID: "task-1"}
	go store.server.RunUpdateQueue(t.Context(), "alice"); <-started
	if err := store.server.CancelUpdateQueue(t.Context(), "alice"); err != nil { t.Fatal(err) }
	close(release)
	if got := store.server.pkg.(*blockingPackageManager).cancelledTaskID; got != "task-1" { t.Fatalf("cancelled = %q", got) }
}
```

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/clientserver -run '^(TestEligibleUpdates|TestUpdateQueue)' -count=1`

Expected: FAIL because queue functions do not exist.

- [ ] **Step 3: Implement queue and shared exclusion**

Load installed apps through `QueryInstalled`, match cached source apps by stable package ID, compare versions, skip protected/local/unknown/no-version/current apps, and resolve each source's default mirror. Install one candidate at a time using asynchronous `InstallLPK` without a password, poll its `InstallTaskDTO` to a terminal state, and record normal install history for each result. Persist the active task ID in the in-memory user queue state. `CancelUpdateQueue` sets the cancellation flag, calls `PackageManager.CancelInstall`, and marks unstarted items cancelled. Share the per-user running lock with `handleInstall`.

### Task 1a: Expose asynchronous install-task control before queue/UI work

**Files:**
- Modify: `internal/clientserver/types.go`, `lazycat.go`, `install.go`, `server.go`, `server_test.go`
- Modify: `client/src/shared/types.ts`, `client/src/modules/client/ClientCatalog.tsx`, `InstallOptionsDialog.tsx`

- [ ] **Step 1: Write contract tests**

```go
func TestInstallReturnsAcceptedTask(t *testing.T) { /* status 202 and taskId */ }
func TestInstallTaskStatusAndCancelAreUserScoped(t *testing.T) { /* owner 200/204, different user 404 */ }
```

- [ ] **Step 2: Implement SDK and HTTP task methods**

Set `WaitUnitDone` to `false`; map `PendingTaskInfo` to `InstallTaskDTO`. Query with `QueryPendingTask`, match task ID, and cancel with `CancelPendingTaskRequest{TaskId: taskID}`. Register `GET`/`DELETE` routes under `/api/client/v1/install-tasks/{taskId}` and return 202/200/204/404 through the existing JSON error contract.

- [ ] **Step 3: Wire dialog polling and cancellation**

After install creation, retain `taskId`, poll at a bounded interval while pending, and call DELETE from the visible progress state. Stop polling on terminal, cancellation, unmount, and retry replacement.

- [ ] **Step 4: Verify task contracts**

Run: `go test ./internal/clientserver -run '^(TestInstallReturnsAcceptedTask|TestInstallTaskStatusAndCancelAreUserScoped)' -count=1 && cd client && npm run build`

```go
for _, candidate := range candidates {

	result.Items = append(result.Items, s.installUpdateCandidate(ctx, userID, candidate))
}
```

- [ ] **Step 4: Verify queue behavior**

Run: `go test ./internal/clientserver -run '^(TestEligibleUpdates|TestUpdateQueue)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clientserver/update_queue.go internal/clientserver/update_queue_test.go internal/clientserver/install.go internal/clientserver/types.go
git commit -m "feat: add client update queue"
```

### Task 2: Persist and schedule automatic updates

**Files:**
- Modify: `ent/schema/client_sync_setting.go` and generated `ent/` files
- Modify: `internal/clientserver/settings.go`, `scheduler.go`, `server.go`, `types.go`
- Modify: `internal/clientserver/server_test.go`
- Test: `internal/clientserver/update_queue_test.go`

**Interfaces:**
- Adds `auto_update_enabled`, `auto_update_interval_minutes`, `last_auto_update_at`, `last_auto_update_status`, `last_auto_update_error`.
- Adds `POST /api/client/v1/updates/run` returning `UpdateQueueResultDTO`.

- [ ] **Step 1: Write failing persistence/due tests**

```go
func TestClientSettingsPersistAutoUpdate(t *testing.T) {
	store := newTestApp(t)
	rec := store.request(http.MethodPatch, "/api/client/v1/settings", `{"autoUpdateEnabled":true,"autoUpdateIntervalMinutes":5}`, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"autoUpdateEnabled":true`) || !strings.Contains(rec.Body.String(), `"autoUpdateIntervalMinutes":5`) { t.Fatalf("settings = %d %s", rec.Code, rec.Body.String()) }
}
func TestAutoUpdateDue(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	if !autoUpdateDue(&ent.ClientSyncSetting{AutoUpdateEnabled: true, AutoUpdateIntervalMinutes: 60, LastAutoUpdateAt: ptr(now.Add(-time.Hour))}, now) { t.Fatal("old update was not due") }
	if autoUpdateDue(&ent.ClientSyncSetting{AutoUpdateEnabled: true, AutoUpdateIntervalMinutes: 60, LastAutoUpdateAt: ptr(now.Add(-time.Minute))}, now) { t.Fatal("recent update was due") }
}
func TestScheduledUpdateSyncsBeforeQueue(t *testing.T) {
	store := newTestApp(t); events := []string{}
	store.server.syncAllSources = func(context.Context, string) (SyncAllResult, error) { events = append(events, "sync"); return SyncAllResult{}, nil }
	store.server.runUpdateQueue = func(context.Context, string) UpdateQueueResultDTO { events = append(events, "update"); return UpdateQueueResultDTO{} }
	store.server.syncScheduler.runDueAutoUpdates(t.Context(), "")
	if !slices.Equal(events, []string{"sync", "update"}) { t.Fatalf("events = %#v", events) }
}
```

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/clientserver -run '^(TestClientSettingsPersistAutoUpdate|TestAutoUpdateDue|TestScheduledUpdateSyncsBeforeQueue)' -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement persisted controls and timer hook**

Use the existing 5..1440 minute sanitizer and default disabled state. Extend settings DTO/update DTO. After a due source-sync succeeds or partially succeeds, invoke the same queue and write `success`, `partial`, `failed`, or `skipped` result data. Register the manual route; it syncs first, then runs the queue under the same lock.

- [ ] **Step 4: Verify focused scheduler checks**

Run: `go test ./internal/clientserver -run '^(TestClientSettingsPersistAutoUpdate|TestAutoUpdateDue|TestScheduledUpdateSyncsBeforeQueue)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ent internal/clientserver
git commit -m "feat: schedule client app updates"
```

### Task 3: Implement bulk-update and progress UX

**Files:**
- Modify: `client/src/modules/client/InstalledAppsView.tsx`, `InstallOptionsDialog.tsx`, `InstallActivityPanel.tsx`, `ClientCatalog.tsx`, `clientUxState.ts`
- Modify: `client/src/shared/types.ts`, `client/src/locales/zh.ts`, `client/src/locales/en.ts`, relevant client CSS

**Interfaces:**
- Consumes `UpdateQueueResultDTO` from `POST /api/client/v1/updates/run`.
- Adds an `onRunUpdates()` callback and `InstallOptionsDialog` phase `options | progress`.

- [ ] **Step 1: Add testable view-state helpers**

```ts
expect(buildUpdateConfirmation([
  { item: { appid: 'eligible', version: '1.0.0' }, source: { packageId: 'eligible', installProtected: false } },
  { item: { appid: 'protected', version: '1.0.0' }, source: { packageId: 'protected', installProtected: true } },
])).toEqual({ eligible: ['eligible'], skipped: ['protected'] });
expect(nextInstallDialogPhase('submitted')).toBe('progress');
```

Place helpers in `clientUxState.ts`; if no TypeScript test runner exists, assert their effects through client-server API tests and compile the client as the frontend gate.

- [ ] **Step 2: Implement controls and activity layering**

Show `Update all (N)` only for a nonempty updates group. Confirm eligible and skipped counts before posting. Render a sticky, expandable queue summary with current item and success/failed/skipped totals. For every `InstallOptionsDialog` entry point, including `SourceAppDetailPage` and `SourceAppGrid`, replace the options form with the current activity/timeline content inside the same `ModalLayer`; do not leave progress solely in a panel behind the backdrop.

Apply `transition: opacity 180ms cubic-bezier(0.23, 1, 0.32, 1), transform 180ms cubic-bezier(0.23, 1, 0.32, 1)` and `:active { transform: scale(0.97) }`. Under `prefers-reduced-motion`, retain opacity only.

- [ ] **Step 3: Add settings controls and manual trigger**

In the existing sync settings tab, add a switch, 5/15/30/60-minute selector, last update result, and `Run update check now`. Include fields in baseline/draft normalization and disable the action while running. Refresh settings and installed apps after completion.

- [ ] **Step 4: Compile and check locale coverage**

Run:

```bash
cd client && npm run build
rg -n "updateAll|autoUpdate|updateQueue" client/src/locales/zh.ts client/src/locales/en.ts
```

Expected: build exits 0 and both locale files contain every new key.

- [ ] **Step 5: Commit**

```bash
git add client/src
git commit -m "feat: add bulk and automatic client updates"
```

### Task 4: Run final client verification and release checks

- [ ] **Step 1: Run repository gates**

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
cd client && npm run build
git diff --check
```

- [ ] **Step 2: Build both final LPKs after version bump**

```bash
cd lazycat/server && lzc-cli project release -o ../../dist/community.lazycat.app-store-server-vNEXT.lpk
cd lazycat/client && lzc-cli project release -o ../../dist/community.lazycat.app-store-vNEXT.lpk
lzc-cli lpk lint ../../dist/community.lazycat.app-store-vNEXT.lpk
```

- [ ] **Step 3: Commit any remaining test/documentation changes**

```bash
git add internal/clientserver client/src
git commit -m "test: cover automatic client updates"
```
