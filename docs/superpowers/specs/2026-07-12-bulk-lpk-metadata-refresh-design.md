# Bulk LPK Metadata Refresh Design

## Goal

Let a publisher select one or more applications and refresh their LPK-derived information from the “My software” screen with one action.

## Behavior

- The operation targets the selected applications owned by the current user.
- Only applications with an external LPK version are eligible.
- Existing active inspection jobs are reused rather than duplicated.
- The default fills missing metadata and does not overwrite manually maintained values.
- One unavailable application does not stop the remaining applications.
- The server returns queued jobs plus skipped applications and reasons.
- The client polls the returned job IDs until every job reaches a terminal state, then reloads the authoritative app list.

## Interface

Each row has a compact checkbox so the same selection can be reparsed or deleted. The “My software” header keeps “Submit app” as the primary action and reveals icon-only reparse and delete actions only after one or more rows are selected. While jobs run, a small inline status panel shows one determinate overall progress bar and concise counts for completed and failed jobs.

The same contextual icon group includes delete. Both reparse and delete require an explicit confirmation dialog listing the selected applications before any request is sent. Reparse explains fill-missing behavior; delete uses destructive styling and clearly states that the operation cannot be undone.

Following Emil Kowalski’s interaction principles, the high-frequency selection and list interactions do not animate. The occasional confirmation dialog is required for user consent. Progress feedback stays anchored to the trigger, transitions only opacity/color/transform, remains under 200ms, and respects reduced motion. The completed panel disappears after the authoritative refresh; failures remain visible through a toast summary. Reparse defaults to filling missing values, with an explicit “overwrite existing information” checkbox and stronger confirmation label. The duplicate single-app inspection UI is removed from application detail.

## API

- `POST /api/v1/me/apps/lpk-inspections`
  - Request: `{ "appIds": [1, 2], "overwriteExistingMetadata": false }`
  - Response: `{ inspections: [{ appId, appName, inspection }], skipped: [{ appId, appName, reason }] }`
- `POST /api/v1/me/apps/lpk-inspections/status`
  - Request: `{ "ids": [1, 2, 3] }`
  - Response: `{ inspections: [{ appId, appName, inspection }] }`

Both endpoints require authentication and only expose jobs for applications owned by the current user.

## Verification

- Server tests cover ownership, eligible/ineligible apps, active-job reuse, and status authorization.
- Frontend contract tests cover selection-driven actions, explicit confirmation, overwrite opt-in, and anchored progress.
- Full Go tests, frontend contract tests, and production build must pass.
