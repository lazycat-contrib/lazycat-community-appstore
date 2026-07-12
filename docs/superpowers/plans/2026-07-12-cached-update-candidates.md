# Cached Update Candidates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start manual and scheduled application updates from client-local cached candidates without synchronizing sources inside the update flow, while making automatic updates depend on automatic source synchronization.

**Architecture:** Extend the update request with an exact candidate snapshot, validate it against the current user's cached applications and installed state, and feed validated candidates into the existing serialized queue executor. Scheduled updates use the same cache selector without a submitted snapshot. Settings normalization enforces the auto-update-to-auto-sync dependency on both server and client.

**Tech Stack:** Go, Ent/SQLite, React, TypeScript, ASTRYX Design, Node test runner, Vite, LazyCat LPK tooling.

## Global Constraints

- Manual updates process only candidates visible in the confirmation snapshot.
- Update execution never invokes software-source synchronization.
- Scheduled updates consume client-local cached source applications and respect per-app automatic-update policy.
- Enabling automatic app updates enables automatic source sync and clamps source-sync interval to the update interval.
- Automatic source sync cannot be disabled while automatic app updates remain enabled.
- Keep structured API errors and the existing queue-result/progress response contract.
- Bump the LazyCat client package from `0.1.25` to `0.1.26` only after implementation passes verification.

---

### Task 1: Define and validate candidate snapshots

**Files:**
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/update_queue.go`
- Modify: `internal/clientserver/update_queue_test.go`

**Interfaces:**
- Produces: `UpdateQueueCandidateDTO` with `appId`, `sourceId`, `packageId`, `installedVersion`, and `targetVersion`.
- Produces: `resolveRequestedUpdateCandidates(ctx, userID, installed, requested)` returning validated `[]updateCandidate`.

- [ ] Add failing tests proving a submitted snapshot installs only submitted applications and rejects mismatched identity, changed installed version, duplicates, protected apps, and missing target versions.
- [ ] Run `go test ./internal/clientserver -run 'Test.*Update.*Candidate' -count=1` and confirm the new assertions fail.
- [ ] Add the request DTO and validation resolver using only current-user cached applications and cached version JSON.
- [ ] Run the focused tests and confirm they pass.

### Task 2: Remove source synchronization from update execution

**Files:**
- Modify: `internal/clientserver/update_queue.go`
- Modify: `internal/clientserver/scheduler.go`
- Modify: `internal/clientserver/update_queue_test.go`

**Interfaces:**
- `RunUpdateQueueWithOptions` selects submitted candidates when present; otherwise it selects from cache.
- `handleRunUpdateQueue` decodes and runs the request without `syncAllSources`.
- `sourceSyncScheduler.updateUser` runs the cache-based queue with `RespectAutoUpdatePolicy: true`.

- [ ] Replace tests that expected implicit source synchronization with tests using an unreachable source URL and pre-populated cached applications.
- [ ] Confirm `POST /updates/run` and scheduled updates succeed without contacting the source URL.
- [ ] Remove sync-result coupling from automatic-update result status.
- [ ] Run `go test ./internal/clientserver -run 'TestRunUpdateQueue|TestAutoUpdate' -count=1`.

### Task 3: Enforce automatic sync settings dependency

**Files:**
- Modify: `internal/clientserver/settings.go`
- Modify: `internal/clientserver/update_queue_test.go`
- Modify: `client/src/modules/client/clientUxState.ts`
- Modify: `client/src/modules/client/clientUxState.test.mjs`

**Interfaces:**
- Produces: server normalization that guarantees `autoUpdateEnabled => autoSyncEnabled` and `autoSyncIntervalMinutes <= autoUpdateIntervalMinutes`.
- Produces: `normalizeAutomationSettings(settings, patch)` for immediate client draft normalization.

- [ ] Add failing server and frontend tests for enable-linking, disable protection, and interval clamping.
- [ ] Implement identical normalization rules on server and client.
- [ ] Run focused Go and Node tests.

### Task 4: Send the exact manual candidate snapshot

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/modules/client/clientUxState.ts`
- Modify: `client/src/modules/client/clientUxState.test.mjs`
- Modify: `client/src/modules/client/InstalledAppsView.tsx`

**Interfaces:**
- Extends `UpdateQueueRequest.candidates` with the candidate DTO fields.
- Produces: `buildUpdateCandidateSnapshot(rows)` used by the confirmation preview and request.

- [ ] Add a frontend test proving protected apps are excluded and candidate IDs/versions match the visible confirmation list.
- [ ] Build the request snapshot once from `installedGroups.updates` and pass it with mirror overrides.
- [ ] Preserve the current modal progress and polling contract.
- [ ] Run Node tests and the frontend build.

### Task 5: Polish the settings dependency UI

**Files:**
- Modify: `client/src/modules/client/ClientSettingsView.tsx`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/styles/client.css` only if geometry needs a scoped dependency treatment.

**Interfaces:**
- Automatic source-sync Switch uses `isDisabled` plus `disabledMessage` while automatic updates are on.
- Automatic-update Switch immediately normalizes both booleans and intervals through the draft helper.

- [ ] Enabling automatic updates immediately enables source sync and adjusts its interval without extra confirmation or animation.
- [ ] Locked source-sync control remains focusable and exposes a concise dependency tip.
- [ ] Replace copy that says updates sync sources first with copy explaining cached updates and the independent sync schedule.
- [ ] Verify Chinese and English at desktop and narrow widths with browser automation.

### Task 6: Version, package, verify, and ship

**Files:**
- Modify: `lazycat/client/package.yml`
- Regenerate: `clientembed/dist/**`

- [ ] Change client package version from `0.1.25` to `0.1.26`.
- [ ] Run `lzc-cli project release -o ../../dist/lazycat-community-appstore-client-0.1.26.lpk` from `lazycat/client`.
- [ ] Inspect the package and confirm embedded version `0.1.26`.
- [ ] Run `go test ./... -count=1`, race tests, vet, module diff, frontend tests, frontend build, and `git diff --check`.
- [ ] Commit implementation/generated assets, re-read worktree and HEAD, push `main`, and verify remote HEAD.
