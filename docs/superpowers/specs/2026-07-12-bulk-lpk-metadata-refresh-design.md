# Bulk LPK Metadata Refresh Design

## Goal

Let a publisher refresh LPK-derived information for every eligible application from the “My software” screen with one action.

## Behavior

- The operation targets applications owned by the current user.
- Only applications with an external LPK version are eligible.
- Existing active inspection jobs are reused rather than duplicated.
- The default fills missing metadata and does not overwrite manually maintained values.
- One unavailable application does not stop the remaining applications.
- The server returns queued jobs plus skipped applications and reasons.
- The client polls the returned job IDs until every job reaches a terminal state, then reloads the authoritative app list.

## Interface

The “My software” header keeps “Submit app” as the primary action and adds a compact secondary “Refresh info” action. While submitting jobs, the label shows submitted/eligible progress. While jobs run, a small inline status panel shows one determinate overall progress bar and concise counts for completed and failed jobs.

Following Emil Kowalski’s interaction principles, this frequent workspace action does not open a modal or animate the list. Feedback stays anchored to the trigger, transitions only opacity/color/transform, remains under 200ms, and respects reduced motion. The completed panel disappears after the authoritative refresh; failures remain visible through a toast summary.

## API

- `POST /api/v1/me/apps/lpk-inspections`
  - Request: `{ "overwriteExistingMetadata": false }`
  - Response: `{ inspections: [{ appId, appName, inspection }], skipped: [{ appId, appName, reason }] }`
- `POST /api/v1/me/apps/lpk-inspections/status`
  - Request: `{ "ids": [1, 2, 3] }`
  - Response: `{ inspections: [{ appId, appName, inspection }] }`

Both endpoints require authentication and only expose jobs for applications owned by the current user.

## Verification

- Server tests cover ownership, eligible/ineligible apps, active-job reuse, and status authorization.
- Frontend contract tests cover anchored progress and the absence of a modal flow.
- Full Go tests, frontend contract tests, and production build must pass.
