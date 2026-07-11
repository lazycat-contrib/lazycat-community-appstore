# Client Updates and API-Token LPK Inspection Design

## Objective

Make the standalone client actionable when installed applications have updates, while making API-token first submissions resilient to release assets that become reachable shortly after CI/CD publishes them.

Success means a user can update all eligible local applications in one ordered task queue or on a configurable client-side schedule, and an API-token application submission can be accepted without blocking on an initially unavailable external LPK. Both workflows must show a durable, understandable result.

## Confirmed Product Decisions

- A bulk or scheduled update installs only applications that do not require an install password. Password-protected updates remain visible for manual action.
- A bulk update is a sequential queue. One failure records its result and does not prevent later eligible applications from running.
- A user can cancel a running installation through LazyCat's `CancelPendingTask` using its returned task ID; cancelling a bulk queue also marks not-yet-started items cancelled.
- The automatic-update scheduler runs in the standalone client service, not in the browser page.
- An API-token first `POST /api/v1/apps` returns after the application is created. LPK inspection runs asynchronously only when that first submission has an external LPK URL.
- The asynchronous job is post-create enrichment: API-token requests continue to meet the existing creation validation and do not rely on a deferred parse to supply fields required to create the application.
- API-token automatic inspection retries only within a configurable total wait window. The site-wide default is 30 seconds; `0` disables automatic inspection.
- Automatic inspection fills only missing application/version metadata. It never replaces explicit CI/CD values.
- App owners, collaborators, and administrators may manually inspect an LPK from the application management detail view. The manual UI defaults to filling missing fields and offers an explicit overwrite choice.

## Commands

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
cd client && npm run build
cd lazycat/server && lzc-cli project release -o ../../dist/community.lazycat.app-store-server-vNEXT.lpk
cd lazycat/client && lzc-cli project release -o ../../dist/community.lazycat.app-store-vNEXT.lpk
```

## Project Structure

- `client/src/modules/client/`: installed-app view, install dialog, activity panel, update queue controls, and client settings UI.
- `internal/clientserver/`: installed-app lookup, source synchronization, package-manager installation, persisted client settings, and client scheduler.
- `internal/server/`: API-token authentication, app submission/version creation, authorization, LPK inspection handlers, and background task orchestration.
- `internal/lpkinspect/`: bounded external LPK fetching, validation, checksumming, and `package.yml` metadata extraction.
- `ent/schema/`: persisted inspection-job and client update-schedule state.
- `docs/openapi.yaml`: public request/response contracts for app submission, inspection status, manual inspection, and settings.

## Client Design

### Bulk updates

The installed-app page keeps the existing update grouping and adds `Update all (N)` beside its heading. Its confirmation dialog lists the N eligible applications and separately states which password-protected applications were skipped. The queue uses each source's already-configured default mirror and creates a normal install-history record for every attempted item.

The queue states are `queued`, `running`, `success`, `failed`, `skipped`, and `cancelled`. Only one client update queue may run for a user at a time. A manual single-app installation joins the same exclusion boundary: it can be started only when no scheduled/bulk queue is active, and a scheduled/bulk queue waits when an interactive installation is running. Cancellation sends the current `taskId` to the LazyCat package manager and prevents the queue from starting later items.

### Installation progress

For every installation entry point, including the software-detail page and the installed-app/update interface, installation creates an asynchronous LazyCat pending task and immediately returns its task ID. `InstallOptionsDialog` becomes an in-place progress view instead of leaving a separate fixed activity panel behind the modal backdrop. It polls task status, downloaded/total bytes, and detail through the client service, retains accessible `aria-live` feedback, and offers `Cancel installation`. Cancellation calls LazyCat `CancelPendingTask` for that task ID. The completed state follows the existing success-dismiss setting; errors remain actionable with retry and history controls.

For a multi-app queue, the installed-app page shows a compact, sticky summary with current item, completed/total count, and an expand control for per-app outcomes. It remains in normal page stacking rather than competing with an open modal. Motion is limited to opacity and transform transitions under 250ms, honors reduced-motion preferences, and gives pressable controls a short active-state response.

### Scheduled updates

Client settings add:

- `autoUpdateEnabled`
- `autoUpdateIntervalMinutes` with 5, 15, 30, and 60 minute choices
- last update attempt/result/summary
- `Run update check now`

When due, the client service silently synchronizes configured sources, loads the local installed-app list through the LazyCat SDK, recomputes eligible updates, and executes the sequential queue. It skips password-protected apps, local/unknown-source apps, applications without an approved/installable latest version, and any app that is already at or above the source version. A scheduler lock prevents duplicate work across timer ticks and manual triggers.

## Installation Task API

`POST /api/client/v1/install` remains the creation endpoint but now returns `202 Accepted` with `{ "task": { "taskId", "status", "downloadedSize", "totalSize", "detail" } }`. Add `GET /api/client/v1/install-tasks/{taskId}` for the task snapshot and `DELETE /api/client/v1/install-tasks/{taskId}` for cancellation. Both routes are client-user scoped; a task not returned by that user's LazyCat task list is `404 INSTALL_TASK_NOT_FOUND`. Task validation and cancellation errors use the existing structured error shape.

## Server Design

### Persisted LPK inspection job

Create a small app-scoped inspection-job model containing the application, optional version, triggering user, source URL snapshot, reason (`api_token_first_submission` or `manual`), overwrite mode, state, attempts, last error, next-attempt time, deadline, and completion time. Valid states are `pending`, `running`, `succeeded`, `failed`, `timed_out`, and `cancelled`.

The API-token path creates at most one automatic job for a newly-created application with an external LPK URL. The job begins immediately. Transient fetch availability failures use bounded backoff within the configured deadline (default 30 seconds): immediate, then approximately 1, 3, 7, and 15 seconds while time remains. An unavailable asset after the deadline becomes `timed_out`; the submitted app and its externally linked version remain intact.

Malformed LPK data, URL validation failures, size-limit failures, and metadata/package mismatches are terminal `failed` states and are not retried. A process restart resumes eligible `pending` jobs whose deadline has not passed. A per-app lock and database state transition prevent duplicate automatic/manual inspection.

### Metadata update rules

Inspection uses the existing bounded URL fetcher and LPK metadata parser. In automatic mode, it fills only missing app fields (`packageId`, name, summary, description) and missing first-version fields (version, file size, SHA256). It never changes an explicit API-token submission value.

The management detail view exposes `Inspect LPK and update details` to authorized owners, collaborators, and administrators. Its request creates or reuses a manual job and accepts `overwriteExistingMetadata: false` by default. When `true`, parsed app/version metadata replaces the corresponding stored parsed fields after package-identity validation. The app detail API returns the latest job status and result so the management UI can poll or refresh reliably.

### Site setting and APIs

Add the site setting `apiTokenInitialLPKInspectionTimeoutSeconds`, range `0..300`, default `30`. It applies only to newly-triggered automatic jobs. Administrators can change it in the existing site settings UI/API. `0` disables future automatic jobs, and does not cancel an already-created manual job.

Add a protected manual-inspection endpoint under the application management API and expose inspection status in the app-detail response. Document new request/response fields and state enum in OpenAPI. The existing app-create response remains backward-compatible and is not delayed for asynchronous parsing.

## Code Style

Use explicit state enums, single-purpose helpers, and the existing structured error responses. Background work keeps request contexts out of long-lived goroutines and creates time-bounded contexts per fetch attempt.

```go
if job.Deadline.Before(now) {
	return s.finishInspectionJob(ctx, job, inspectionjob.StateTIMED_OUT, "LPK was not reachable before the deadline")
}
if err := s.inspectAndApplyLPK(ctx, job); err != nil {
	return s.rescheduleTransientInspection(ctx, job, err, now)
}
return s.finishInspectionJob(ctx, job, inspectionjob.StateSUCCEEDED, "")
```

## Testing Strategy

- Server unit/integration tests cover API-token first-submission creation, immediate return, automatic job creation, bounded backoff, configurable deadline, terminal versus transient failures, restart recovery, and per-app duplicate prevention.
- Server authorization tests cover owner, collaborator, administrator, and unauthorized manual-inspection attempts.
- Metadata tests prove automatic jobs fill only empty values; manual jobs overwrite only with explicit consent; package-ID mismatches leave data unchanged.
- Client-server tests cover persisted scheduler settings, due-time evaluation, exclusion of password-protected apps, ordered execution, failure continuation, and scheduler/manual mutual exclusion.
- Client-server tests cover cancellation forwarding with the active LazyCat task ID and queue cancellation of unstarted items.
- Frontend tests cover update count/confirmation, skipped-password messaging, dialog progress state, queue summary, result rendering, and translation keys.
- Run the commands above plus both LPK lint/info checks before release.

## Boundaries

- Always: use the existing LPK URL safety checks and maximum size; keep tasks idempotent; retain install and inspection history/errors; run the verification suite before committing.
- Ask first: change the 0..300 second setting range, allow automatic overwrite of CI/CD metadata, add a new external worker/queue dependency, or change installation-password behavior.
- Never: run client-device package installation from the store server; log API tokens or install passwords; bypass authorization for manual inspection; retry deterministic LPK validation errors indefinitely.

## Success Criteria

- A visible update list has a working bulk-update entry point and a clear count of password-protected skips.
- Installing from any modal always shows live progress in that modal rather than behind its backdrop.
- With scheduled updates enabled, the client service updates only eligible newer applications, serially and without concurrent duplicate queues.
- A first API-token external-LPK submission returns without waiting for asset availability, then creates a visible automatic inspection job when enabled.
- A temporarily unreachable LPK is retried only until the configured total window; at 30 seconds it stops automatically and records `timed_out`.
- Auto parsing does not overwrite explicit CI/CD values; authorized manual parsing can overwrite only after explicit opt-in.

## Open Questions

None. The confirmed decisions above define the initial scope.
