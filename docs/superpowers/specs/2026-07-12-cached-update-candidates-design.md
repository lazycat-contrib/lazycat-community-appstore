# Cached Update Candidates Design

## Goal

Make manual and scheduled application updates start immediately from already-cached source data instead of synchronizing every software source before installation.

## Product behavior

- The installed-app page continues to calculate available updates from the loaded installed list and cached source applications.
- Opening the bulk-update confirmation creates a candidate snapshot from exactly the applications shown there.
- Starting the update processes only that snapshot. Updates that appear after confirmation wait for the next run.
- Scheduled updates calculate candidates from the client-local database cache and never trigger source synchronization.
- Source synchronization remains an independent action controlled by manual sync, startup sync, and the existing automatic source-sync schedule.
- A stale cache may produce an older candidate set; the update flow does not refresh it implicitly.

## Automatic sync dependency

Automatic application updates require automatic software-source synchronization:

- Enabling `autoUpdateEnabled` also enables `autoSyncEnabled` in the same settings update.
- While automatic updates are enabled, the automatic-source-sync switch is disabled in the UI and explains the dependency through a tip.
- Users must disable automatic application updates before they can disable automatic source synchronization.
- The server enforces the same invariant so older or alternate clients cannot persist `autoUpdateEnabled=true` with `autoSyncEnabled=false`.
- `autoSyncIntervalMinutes` must be less than or equal to `autoUpdateIntervalMinutes`.
- If automatic updates are enabled or their interval is shortened below the current sync interval, the server and client normalize the source-sync interval down to the automatic-update interval.
- Disabling automatic updates does not disable source synchronization; source sync may continue independently.

This is a settings dependency, not a runtime coupling. The source-sync scheduler refreshes the client database; the update scheduler consumes that database without initiating a source sync itself.

## Manual update API contract

Extend `POST /api/client/v1/updates/run` with an optional `candidates` array:

```json
{
  "candidates": [
    {
      "appId": 12,
      "sourceId": 3,
      "packageId": "community.lazycat.app.example",
      "installedVersion": "1.0.0",
      "targetVersion": "1.2.0"
    }
  ],
  "mirrorOverrides": []
}
```

Each candidate identifies the exact application and target version shown in the confirmation UI. Empty or missing `candidates` means the caller wants the backend to calculate candidates from the current client-local cache; this preserves internal callers and enables scheduled updates.

## Server validation

The server treats submitted candidates as untrusted input and validates each item before installation:

- `appId`, `sourceId`, and `packageId` must identify the same cached source application owned by the current client user.
- The cached application must still expose `targetVersion` with a non-empty download URL.
- Password-protected applications are skipped.
- The currently installed package and version are re-read from the LazyCat SDK immediately before queue construction.
- If the application is no longer installed, is already at or above the target, or its installed version differs from the submitted snapshot, it is skipped instead of downgraded or unexpectedly updated.
- Duplicate package IDs are collapsed to one candidate.
- Invalid individual candidates are skipped; malformed JSON or structurally invalid entries return the existing structured API error format.

The response remains `UpdateQueueResultDTO`, so polling and progress UI contracts do not change.

## Queue architecture

Split candidate selection from queue execution:

1. `resolveRequestedUpdateCandidates` validates a submitted snapshot against client-local cache and current installed state.
2. `eligibleUpdates` remains the cache-based selector for scheduled runs and backward-compatible callers without a snapshot.
3. `runResolvedUpdateQueue` owns serialization, progress publication, mirror resolution, installation, and final result calculation.

`handleRunUpdateQueue` must not call `syncAllSources`. The scheduler's `updateUser` must not call `syncAllSources`; it invokes the cache-based queue with `RespectAutoUpdatePolicy: true`.

## Client behavior

- `InstalledAppsView` builds request candidates from `installedGroups.updates`, excluding password-protected applications exactly as the confirmation preview does.
- Candidate fields come from the installed row and its matched cached source application.
- Mirror selection remains source-based and is sent alongside the candidate snapshot.
- The progress dialog begins at queue creation rather than displaying a source-sync waiting phase.
- After completion, the client refreshes installed applications, cached apps, and settings as it does today; it does not synchronize sources.

## Scheduled update result semantics

- Scheduled runs use cached candidates and respect per-application automatic-update policy.
- `no_updates` and `already_running` remain skipped outcomes.
- Success, partial failure, and failure are derived only from queue execution; removed source-sync counts no longer affect automatic-update status messages.
- Automatic source-sync history remains independent from automatic-update history.
- On scheduler ticks where source synchronization and application updates are both due, due source synchronizations complete before cached update candidates are calculated.

## Testing

- API regression test proves `POST /updates/run` succeeds from cached data while the configured source URL is unreachable.
- Manual snapshot test proves only submitted candidates are installed even when additional cached updates exist.
- Validation tests cover mismatched app/source/package identity, changed installed version, missing target version, duplicates, and password-protected applications.
- Scheduler test proves automatic updates consume cached data without contacting the source server.
- Existing queue progress, mirror override, update-policy, and install-serialization tests continue to pass.
- Frontend helper tests pin the exact candidate snapshot produced by the confirmation list.
- Settings API tests prove enabling automatic updates also enables automatic sync and clamps the sync interval to the update interval.
- Settings API tests prove a request cannot leave automatic updates enabled while disabling automatic sync.
- Frontend tests prove the source-sync switch is locked with explanatory copy while automatic updates are enabled.
