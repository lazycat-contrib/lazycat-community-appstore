# App Version Retention and Deletion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add application-level version-retention overrides, deletion of any version including latest, best-effort managed-object cleanup, and version-snapshot download events that remain application-level analytics.

**Architecture:** Store a nullable retention override on `App`; `NULL` inherits the live site setting and `0` means unlimited. Put deletion and retention in one backend lifecycle service that commits database removal before optional storage cleanup. Replace download-event `version_id` with a denormalized version string, expose dedicated REST sub-resources, and manage the policy beside version history in the server storefront drawer.

**Tech Stack:** Go 1.26, Ent, SQLite/PostgreSQL/MySQL, React 19, TypeScript, Astryx Design, i18next, Node test runner, OpenAPI 3.

## Global Constraints

- Any application version, including the current latest version, is deletable by an application owner, approved collaborator, software administrator, or site administrator.
- Only an application owner, software administrator, or site administrator may change application-level retention.
- `version_retention_count = NULL` inherits the current site `max_versions`; `0` means unlimited; a positive value is the exact application limit.
- Saving an application policy immediately prunes excess approved versions; changing the site default never launches a whole-site destructive scan.
- Retention keeps approved versions ordered by `published_at DESC`, `created_at DESC`, then `id DESC`; pending and rejected versions are manual-delete only.
- `storage_path == ""` means an online artifact: delete only database state. A non-empty path means a managed object: commit database deletion first, then attempt local/WebDAV/S3 cleanup with an independent short timeout; cleanup failure never rolls back.
- Download events store the installed version string as an immutable snapshot, never a version-row ID. Analytics aggregate only by `app_id` and `created_at`, never by version.
- Old archives containing download `version_id` remain importable; new archives export the version string and omit `version_id`.
- Use Go 1.26 forms (`t.Context()`, `b.Loop()`, `sync.WaitGroup.Go`, `errors.AsType`) and add no production runtime dependency.
- Frontend tasks must compose current Astryx Design and project shared components; do not create replacement Button, Dialog, Input, Selector, Badge, Toast, table, or other design-system primitives.
- Preserve all existing rating, download-period sorting, site-timezone, Ent/OpenAPI, generated-asset, and hardening WIP outside the files named by each task.
- Do not run a frontend distribution build until final integration; only the main coordinator stages, commits, bumps versions, builds LPKs, and pushes.

---

### Task 1: Ent Schema and Live Download-Snapshot Migration

**Execution note:** Tasks 1 and 2 are implemented and reviewed as one data-foundation unit. Removing the generated `AppDownload.version_id` API immediately invalidates the recorder, importer, and tests, so neither half can compile independently.

**Files:**
- Modify: `ent/schema/app.go`
- Modify: `ent/schema/app_download.go`
- Modify: `internal/server/schema_migrations.go`
- Modify: `internal/server/server_test.go`
- Regenerate: `ent/**`

**Interfaces:**
- Produces `App.VersionRetentionCount *int` and `AppDownload.Version string`.
- Produces schema version `2` and `migrateDownloadVersionSnapshots(context.Context) error`.
- Removes generated application-code use of `AppDownload.VersionID`, `SetVersionID`, and `VersionIDEQ`.

- [x] **Step 1: Add a failing live-migration test**

Add `TestMigrateDownloadVersionSnapshotsBackfillsLegacyVersionID`. It must create a normal test server, add a legacy `version_id` column with:

```go
if _, err := app.server.sqlDB.ExecContext(
    ctx,
    "ALTER TABLE app_downloads ADD COLUMN version_id integer NOT NULL DEFAULT 0",
); err != nil {
    t.Fatalf("add legacy version_id: %v", err)
}
```

Insert an approved `AppVersion` named `2.4.1`, insert an `app_downloads` row with `version = ''` and that legacy ID, call `migrateDownloadVersionSnapshots`, and assert `download.Version == "2.4.1"`.

- [x] **Step 2: Verify RED**

```bash
go test ./internal/server -run TestMigrateDownloadVersionSnapshotsBackfillsLegacyVersionID -count=1
```

Expected: compile failure because the new fields and migration function do not exist.

- [x] **Step 3: Change Ent schemas**

Add to `App.Fields()`:

```go
field.Int("version_retention_count").Optional().Nillable(),
```

Replace `AppDownload` fields and indexes with:

```go
func (AppDownload) Fields() []ent.Field {
    return []ent.Field{
        field.Int("app_id"),
        field.String("version").Default(""),
        field.Time("created_at").Default(time.Now),
    }
}

func (AppDownload) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("app_id", "created_at"),
        index.Fields("created_at", "app_id"),
    }
}
```

- [x] **Step 4: Regenerate Ent**

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

Expected: exit `0`; generated builders expose the new fields and no generated app-download version-ID API remains.

- [x] **Step 5: Add schema-version 2 backfill**

Set `currentServerSchemaVersion = 2`. After version 1, run `migrateDownloadVersionSnapshots`, then persist schema version 2.

The migration must probe `information_schema.columns` for PostgreSQL/MySQL and `PRAGMA table_info(app_downloads)` for SQLite. If the legacy column exists, run:

```sql
UPDATE app_downloads
SET version = COALESCE(
  (SELECT version FROM app_versions WHERE app_versions.id = app_downloads.version_id),
  ''
)
WHERE version = ''
```

The obsolete physical legacy column may remain in upgraded databases, but new Ent code must never read or write it. New databases do not create it.

- [x] **Step 6: Verify schema and migration**

```bash
go test ./internal/server -run 'TestMigrateDownloadVersionSnapshots|TestSchema' -count=1
go test ./ent/... -count=1
```

Expected: both commands exit `0`.

---

### Task 2: Download Recorder and Migration Archive Compatibility

**Execution note:** Execute in the same implementer/reviewer cycle as Task 1; the combined verification commands are the completion gate.

**Files:**
- Modify: `internal/server/app_metrics.go`
- Modify: `internal/server/app_metrics_test.go`
- Modify: `internal/migration/records.go`
- Modify: `internal/migration/importer.go`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- Changes recorder to `recordAppDownload(ctx context.Context, appID int, version string) error`.
- Adds `AppRecord.VersionRetentionCount *int`.
- Adds `AppDownloadRecord.Version string` and legacy-only `LegacyVersionID int` with JSON name `version_id`.

- [x] **Step 1: Add failing snapshot and archive tests**

Add `TestRecordAppDownloadStoresVersionSnapshot`:

```go
if err := testApp.server.recordAppDownload(ctx, appID, "3.2.0"); err != nil {
    t.Fatalf("record download: %v", err)
}
event := testApp.server.db.AppDownload.Query().
    Where(appdownload.AppIDEQ(appID)).
    OnlyX(ctx)
if event.Version != "3.2.0" {
    t.Fatalf("version = %q, want 3.2.0", event.Version)
}
```

Add migration tests proving an old `version_id` resolves to the archived version text, an unresolved legacy ID keeps the event with an empty version, and a new export carries `version` without a non-zero `version_id`.

- [x] **Step 2: Verify RED**

```bash
go test ./internal/server ./internal/migration -run 'Download.*Snapshot|LegacyDownloadVersion|AppRetentionOverride' -count=1
```

Expected: compile/test failure on old version-ID contracts.

- [x] **Step 3: Record version text without changing analytics**

Use:

```go
func (s *Server) recordAppDownload(ctx context.Context, appID int, version string) error {
    tx, err := s.db.Tx(ctx)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()
    if _, err := tx.App.UpdateOneID(appID).AddDownloadCount(1).Save(ctx); err != nil {
        return err
    }
    if _, err := tx.AppDownload.Create().
        SetAppID(appID).
        SetVersion(strings.TrimSpace(version)).
        SetCreatedAt(s.nowUTC()).
        Save(ctx); err != nil {
        return err
    }
    return tx.Commit()
}
```

The download handler passes the loaded version record's `Version` string. Do not add version predicates to period/ranking queries.

- [x] **Step 4: Update archive records**

```go
type AppDownloadRecord struct {
    ID              int       `json:"id"`
    AppID           int       `json:"app_id"`
    Version         string    `json:"version,omitempty"`
    LegacyVersionID int       `json:"version_id,omitempty"`
    CreatedAt       time.Time `json:"created_at"`
}
```

Add `VersionRetentionCount *int` with JSON name `version_retention_count` to `AppRecord`. Export `appdownload.FieldVersion`; do not export a generated version-ID field.

- [x] **Step 5: Import old and new formats**

Build a legacy lookup before download import:

```go
legacyVersionNames := make(map[int]string, len(data.AppVersions))
for _, record := range data.AppVersions {
    legacyVersionNames[record.ID] = record.Version
}
```

Resolve each snapshot:

```go
version := strings.TrimSpace(record.Version)
if version == "" && record.LegacyVersionID > 0 {
    version = legacyVersionNames[record.LegacyVersionID]
}
```

Deduplicate by `app_id + version + created_at`, then create with `SetVersion(version)`. Set or clear the app override during replace import.

- [x] **Step 6: Verify migration and metrics**

```bash
go test ./internal/server ./internal/migration -run 'Download|Migration|RetentionCount' -count=1
go test ./internal/server -run '^$' -bench 'BenchmarkPreloadAppSummaries' -benchmem -count=1
```

Expected: tests pass and the accepted app/time-only four-query metrics path remains in production.

---

### Task 3: Version Lifecycle Service and Best-Effort Cleanup

**Files:**
- Create: `internal/server/version_lifecycle.go`
- Create: `internal/server/version_lifecycle_test.go`
- Modify: `internal/server/storage_config.go`
- Modify: `internal/server/handlers_apps.go`
- Modify: `internal/server/handlers_admin.go`

**Interfaces:**
- Produces `versionRetentionPolicy`, `deletedVersionResult`, and `versionCleanupWarning`.
- Produces `deleteAppVersion`, `updateAppVersionRetention`, and `enforceVersionRetention`.
- Existing direct-publish and review-approval paths consume `enforceVersionRetention`.

- [x] **Step 1: Write lifecycle tests**

Create tests for inherited/custom/unlimited policy, site-default change without a whole-site prune, immediate oldest-first pruning, latest deletion, only-version deletion, pending/rejected deletion with review-reference clearing, external artifact no-cleanup, managed artifact cleanup, cleanup failure without rollback, and concurrent publication/retention/manual deletion.

Use a fake backend:

```go
type recordingDeleteBackend struct {
    storage.Backend
    paths []string
    err   error
}

func (b *recordingDeleteBackend) Delete(_ context.Context, path string) error {
    b.paths = append(b.paths, path)
    return b.err
}
```

- [x] **Step 2: Verify RED**

```bash
go test ./internal/server -run 'TestVersionLifecycle|TestVersionRetention' -count=1
```

Expected: compile failure because lifecycle interfaces do not exist.

- [x] **Step 3: Add checked storage deletion**

```go
func (s *Server) deleteStoredObjectChecked(ctx context.Context, storageKey, objectPath string) error {
    if strings.TrimSpace(objectPath) == "" {
        return nil
    }
    backend, err := s.storageBackendForKey(ctx, storageKey)
    if err == nil {
        return backend.Delete(ctx, objectPath)
    }
    return s.storage.Delete(ctx, objectPath)
}

func (s *Server) deleteStoredObject(ctx context.Context, storageKey, objectPath string) {
    _ = s.deleteStoredObjectChecked(ctx, storageKey, objectPath)
}
```

- [x] **Step 4: Implement lifecycle contracts**

```go
const versionStorageCleanupTimeout = 5 * time.Second

type versionRetentionPolicy struct {
    Mode                 string `json:"mode"`
    SiteMaxVersions      int    `json:"siteMaxVersions"`
    AppMaxVersions       *int   `json:"appMaxVersions,omitzero"`
    EffectiveMaxVersions int    `json:"effectiveMaxVersions"`
}

type versionCleanupWarning struct {
    VersionID   int    `json:"versionId"`
    StorageKey  string `json:"storageKey"`
    StoragePath string `json:"storagePath"`
    Message     string `json:"message"`
}

type deletedVersionResult struct {
    Version version                `json:"version"`
    Warning *versionCleanupWarning `json:"cleanupWarning,omitzero"`
}
```

`deleteAppVersion` queries by app and version IDs inside an Ent transaction, clears `ReviewRequest.version_id`, deletes the version row, commits, and only then calls cleanup. It never updates downloads.

Managed cleanup uses:

```go
cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), versionStorageCleanupTimeout)
defer cancel()
```

On cleanup error, log app/version/storage/path/error and return a warning; do not restore the row.

- [x] **Step 5: Implement retention**

`versionRetentionPolicyForApp` returns `INHERIT` for nil and `CUSTOM` otherwise. `updateAppVersionRetention` updates/clears the override and removes excess rows in one DB transaction. `enforceVersionRetention` uses the current effective value without changing the field.

Approved rows use:

```go
Order(
    entgo.Desc(appversion.FieldPublishedAt),
    entgo.Desc(appversion.FieldCreatedAt),
    entgo.Desc(appversion.FieldID),
)
```

Effective `0` deletes nothing. Positive values delete `records[limit:]`, then perform post-commit managed cleanup.

- [x] **Step 6: Replace old retention calls**

Remove the request-bound legacy helper. Publication and approval call:

```go
if _, _, err := s.enforceVersionRetention(r.Context(), appID); err != nil {
    slog.Warn("Could not enforce version retention", "app_id", appID, "error", err)
}
```

An already-successful publication remains successful if retention later fails.

- [x] **Step 7: Verify lifecycle and race safety**

```bash
go test ./internal/server -run 'TestVersionLifecycle|TestVersionRetention|TestApprovedExternalVersionsRespectRetention' -count=1
go test -race ./internal/server -run 'TestVersionLifecycle|TestVersionRetention' -count=5
```

Expected: all tests pass without races.

---

### Task 4: REST Endpoints, DTOs, Authorization, and OpenAPI

**Files:**
- Create: `internal/server/handlers_versions.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/types.go`
- Modify: `internal/server/handlers_apps.go`
- Modify: `docs/openapi.yaml`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Produces `PATCH /api/v1/apps/{id}/version-retention`.
- Produces `DELETE /api/v1/apps/{id}/versions/{versionId}`.
- Adds optional `versionRetention` to application detail.
- Success responses include deleted/pruned versions and cleanup warnings.

- [x] **Step 1: Write HTTP contract tests**

Cover owner/admin policy update, collaborator policy rejection, collaborator version deletion, latest fallback, deleting the only version, cross-app ID rejection, invalid mode/count, and cleanup-warning success.

Use this latest-delete assertion:

```go
rec := app.do(
    http.MethodDelete,
    fmt.Sprintf("/api/v1/apps/%d/versions/%d", record.ID, latest.ID),
    nil,
)
if rec.Code != http.StatusOK {
    t.Fatalf("delete latest status = %d, body = %s", rec.Code, rec.Body.String())
}
if app.server.db.AppVersion.Query().
    Where(appversion.AppIDEQ(record.ID)).
    ExistX(ctx) {
    t.Fatal("version still exists")
}
```

- [x] **Step 2: Verify RED**

```bash
go test ./internal/server -run 'Test(DeleteLatestVersion|DeleteVersionAuthorization|UpdateVersionRetention|VersionRetentionValidation)' -count=1
```

Expected: route/test failure because endpoints are absent.

- [x] **Step 3: Register routes and validate requests**

```go
s.mux.HandleFunc(
    "PATCH /api/v1/apps/{id}/version-retention",
    s.withAuth(s.handleUpdateVersionRetention),
)
s.mux.HandleFunc(
    "DELETE /api/v1/apps/{id}/versions/{versionId}",
    s.withAuth(s.handleDeleteVersion),
)
```

Use explicit presence tracking:

```go
type updateVersionRetentionRequest struct {
    Mode           string `json:"mode"`
    MaxVersions    *int   `json:"maxVersions"`
    maxVersionsSet bool
}

func (input *updateVersionRetentionRequest) UnmarshalJSON(data []byte) error {
    type requestAlias updateVersionRetentionRequest
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }
    var decoded requestAlias
    if err := json.Unmarshal(data, &decoded); err != nil {
        return err
    }
    *input = updateVersionRetentionRequest(decoded)
    _, input.maxVersionsSet = raw["maxVersions"]
    return nil
}
```

`INHERIT` rejects any supplied `maxVersions`. `CUSTOM` requires a non-nil non-negative integer.

- [x] **Step 4: Implement endpoint authorization**

Policy update uses:

```go
if !isAdmin(u) && record.OwnerID != u.ID {
    writeError(
        w,
        http.StatusForbidden,
        "FORBIDDEN",
        "Only the app owner or an administrator can change version retention",
        nil,
    )
    return
}
```

Version deletion uses `s.canUploadVersion(r, record, u)`. Query the version with both app and version IDs; a cross-app ID returns `VERSION_NOT_FOUND`.

Return:

```go
writeJSON(w, http.StatusOK, map[string]any{
    "versionRetention": policy,
    "prunedVersions":   deleted,
})
```

and:

```go
writeJSON(w, http.StatusOK, map[string]any{
    "deletedVersion": deleted,
})
```

- [x] **Step 5: Add policy to detail and collaborator visibility**

Add:

```go
VersionRetention *versionRetentionPolicy `json:"versionRetention,omitzero"`
```

Populate it for release managers. Approved collaborators must also see pending/rejected versions:

```go
canManageReleases := u != nil && s.canUploadVersion(r, record, u)
if v.Status != appversion.StatusAPPROVED && !canManageReleases {
    continue
}
```

- [x] **Step 6: Document OpenAPI**

Add both paths plus `VersionRetentionPolicy`, `VersionCleanupWarning`, `INHERIT` and `CUSTOM` request schemas, and response/error shapes. `AppDetail.versionRetention` is optional for public consumers.

- [x] **Step 7: Verify API and authorization**

```bash
go test ./internal/server -run 'Test(Delete.*Version|UpdateVersionRetention|VersionRetention|Collaborator)' -count=1
go test -race ./internal/server -run 'Test(Delete.*Version|UpdateVersionRetention)' -count=3
```

Expected: all commands exit `0`.

---

### Task 5: Version Management State and Dialog Components

**Files:**
- Create: `client/src/modules/storefront/VersionManagementDialogs.tsx`
- Create: `client/src/modules/storefront/versionManagementState.ts`
- Create: `client/src/modules/storefront/versionManagementState.test.mjs`
- Modify: `client/src/shared/types.ts`

**Interfaces:**
- Produces `VersionRetentionPolicy` and `VersionCleanupWarning` TypeScript types.
- Produces `retentionPruneCount`, `nextLatestVersion`, `VersionRetentionDialog`, and `VersionDeleteDialog`.
- Task 6 consumes these exports.

- [x] **Step 1: Write failing pure-state tests**

```js
test('retentionPruneCount ignores pending versions and treats zero as unlimited', async () => {
  const { retentionPruneCount } = await loadStateHelpers();
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'PENDING' }],
      1,
    ),
    0,
  );
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'APPROVED' }],
      0,
    ),
    0,
  );
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'APPROVED' }],
      1,
    ),
    1,
  );
});
```

Add a second test proving `nextLatestVersion` skips the deleted ID and non-approved versions.

- [x] **Step 2: Verify RED**

```bash
node --test client/src/modules/storefront/versionManagementState.test.mjs
```

Expected: failure because the helper file does not exist.

- [x] **Step 3: Add shared types**

```ts
export type VersionRetentionPolicy = {
  mode: 'INHERIT' | 'CUSTOM';
  siteMaxVersions: number;
  appMaxVersions?: number | null;
  effectiveMaxVersions: number;
};

export type VersionCleanupWarning = {
  versionId: number;
  storageKey: string;
  storagePath: string;
  message: string;
};
```

Extend `StoreApp` with `versionRetention?: VersionRetentionPolicy`.

- [x] **Step 4: Implement pure helpers**

```ts
import type { Version } from '../../shared/types';

export function retentionPruneCount(
  versions: Version[],
  effectiveMaxVersions: number,
): number {
  if (effectiveMaxVersions === 0) return 0;
  const approved = versions.filter((version) => version.status === 'APPROVED');
  return Math.max(0, approved.length - effectiveMaxVersions);
}

export function nextLatestVersion(
  versions: Version[],
  deletingID: number,
): Version | null {
  const approved = versions
    .filter(
      (version) =>
        version.status === 'APPROVED' && version.id !== deletingID,
    )
    .sort((left, right) => {
      const leftTime = Date.parse(left.publishedAt || left.createdAt);
      const rightTime = Date.parse(right.publishedAt || right.createdAt);
      return rightTime - leftTime || right.id - left.id;
    });
  return approved[0] || null;
}
```

- [x] **Step 5: Create accessible dialogs**

`VersionRetentionDialog` composes the existing Astryx `Dialog`, `Button`, `Selector`, and `TextInput` primitives with existing project feedback components. It uses one `Dialog purpose="form"`, explicit `INHERIT`/`CUSTOM` choices, a non-negative numeric input, expected prune count, and a destructive save label when pruning. It must not implement replacement form or dialog primitives.

`VersionDeleteDialog` composes the existing Astryx `Dialog` and `Button` plus the project `SectionTitle`. It uses one root `role="alertdialog"`; cancel carries `data-autofocus="true"`; Escape closes only while idle.

Expose:

```ts
export type VersionRetentionDialogProps = {
  policy: VersionRetentionPolicy;
  versions: Version[];
  isSaving: boolean;
  onCancel: () => void;
  onSave: (
    input:
      | { mode: 'INHERIT' }
      | { mode: 'CUSTOM'; maxVersions: number },
  ) => Promise<void>;
};

export type VersionDeleteDialogProps = {
  appName: string;
  version: Version;
  consequence: string;
  isDeleting: boolean;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
};
```

- [x] **Step 6: Verify state and TypeScript**

```bash
node --test client/src/modules/storefront/versionManagementState.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected: state tests pass; TypeScript has no diagnostics.

---

### Task 6: App Drawer Integration, Locales, Styling, and Contracts

**Files:**
- Modify: `client/src/modules/storefront/AppDrawer.tsx`
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/styles/storefront.css`

**Interfaces:**
- Consumes Task 4 responses and Task 5 components/helpers.
- Preserves existing download/install and rating behavior.
- Produces retention summary, manual deletion, busy states, and persistent result feedback.

- [x] **Step 1: Add failing storefront contracts**

```js
test('version management distinguishes policy, download, delete, and cleanup warning states', async () => {
  const drawer = await source('./AppDrawer.tsx');
  assert.match(drawer, /version-retention-summary/);
  assert.match(drawer, /VersionRetentionDialog/);
  assert.match(drawer, /VersionDeleteDialog/);
  assert.match(drawer, /method:\s*'PATCH'/);
  assert.match(drawer, /method:\s*'DELETE'/);
  assert.match(drawer, /cleanupWarning/);
  assert.match(drawer, /stopPropagation\(\)/);
});
```

- [x] **Step 2: Verify RED**

```bash
node --test --test-name-pattern='version management' client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: missing version-management hooks.

- [x] **Step 3: Add state and API actions**

Policy:

```ts
await api(`/api/v1/apps/${app.id}/version-retention`, {
  method: 'PATCH',
  body: JSON.stringify(input),
});
```

Delete:

```ts
const data = await api<{
  deletedVersion: {
    version: Version;
    cleanupWarning?: VersionCleanupWarning;
  };
}>(`/api/v1/apps/${app.id}/versions/${version.id}`, {
  method: 'DELETE',
});
```

After success, await `onRefresh()` and `onListRefresh()` before clearing busy state. Manual deletion checks `deletedVersion.cleanupWarning`; policy save checks every `prunedVersions[*].cleanupWarning`. Any cleanup failure produces a localized warning result while deleted rows remain gone.

- [x] **Step 4: Render policy and row actions**

Above version history, render `.version-retention-summary` in management mode when policy exists. Compose existing `Card`, `StatusBadge`, `MetadataList`, and `Button` components; do not build a custom card/badge/table primitive. Show inheritance/custom label, approved count, and effective count; `0` uses localized unlimited copy. Only `canMaintain` sees settings.

Each row keeps download/install. When `isManageMode && canUploadVersion`, add a separate destructive delete action. Both action handlers call `event.stopPropagation()`.

Latest consequence names the next approved version or says the app becomes temporarily unavailable. Historical consequence says existing installations are unaffected.

- [x] **Step 5: Add matching Chinese and English locale keys**

Add:

```text
versionRetentionTitle
versionRetentionInherited
versionRetentionCustom
versionRetentionUnlimited
versionRetentionApprovedCount
versionRetentionEffectiveCount
versionRetentionSettings
versionRetentionSave
versionRetentionSaveAndPrune
versionRetentionSaved
versionRetentionPruned
versionRetentionFailed
versionDeleteTitle
versionDeleteLatestFallback
versionDeleteLatestEmpty
versionDeleteHistorical
versionDeleteConfirm
versionDeleted
versionDeleteCleanupWarning
versionDeleteFailed
```

Chinese labels include `保存并删除 {{count}} 个旧版本` and `删除版本 {{version}}`; English labels include `Save and delete {{count}} old versions` and `Delete version {{version}}`.

- [x] **Step 6: Add scoped responsive styles**

Add retention card, metric, row-action, result, and dialog-copy rules beneath `.server-detail-page`. Text wraps at 320/375px and buttons become full width at `max-width: 640px`. Do not animate policy/version rows.

- [x] **Step 7: Verify frontend**

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/storefront/versionManagementState.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected: all tests pass; TypeScript has no diagnostics.

---

### Task 7: Cross-Layer Verification and Release Handoff

**Files:**
- Modify only when verification finds a defect in files already owned by Tasks 1–6.
- The coordinator performs LazyCat version bumps, final frontend builds, LPK builds, commit, and push after all existing plans are complete.

**Interfaces:**
- Consumes Tasks 1–6.
- Produces fresh verification evidence and release-ready source state.

- [x] **Step 1: Run focused feature suites**

```bash
go test ./internal/server ./internal/migration -run 'Version|Retention|Download|Migration' -count=1
go test -race ./internal/server -run 'Version|Retention|Download' -count=3
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/storefront/versionManagementState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
```

Expected: every command exits `0`.

- [x] **Step 2: Run complete Go gates**

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
golangci-lint run --timeout=5m ./...
go mod tidy -diff
```

Expected: all commands exit `0`; tidy prints no diff.

- [x] **Step 3: Validate generated code and OpenAPI**

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
git diff --exit-code -- ent
ruby -e 'require "yaml"; YAML.load_file("docs/openapi.yaml"); puts "openapi ok"'
rg -n 'version_id' ent/schema/app_download.go internal/server/app_metrics.go internal/server/version_lifecycle.go
```

Expected: Ent regeneration adds no diff; YAML prints `openapi ok`; the final search prints no app-download version-ID use.

- [x] **Step 4: Verify archive and analytics invariants**

```bash
go test ./internal/migration -run 'LegacyDownloadVersion|VersionRetention|Migration' -count=3
go test ./internal/server -run '^$' -bench 'BenchmarkPreloadAppSummaries' -benchmem -count=3
```

Expected: old/new archives pass; accepted app/time-only four-query statistics remain the production path.

- [x] **Step 5: Record handoff**

Append:

```text
- App version retention/deletion Tasks 1–7: complete; per-task reviews clean; feature verification passed.
```

The coordinator then completes remaining storefront/admin tasks, performs final whole-branch review, updates server/client versions, builds and inspects both LPKs, stages explicit paths, commits, and pushes `main` without app-store publication.

- App version retention/deletion Tasks 1–7: complete; feature tests, race tests, full Go gates, frontend contracts, TypeScript, OpenAPI parsing, dependency audit, Ent regeneration, and benchmark verification passed.
