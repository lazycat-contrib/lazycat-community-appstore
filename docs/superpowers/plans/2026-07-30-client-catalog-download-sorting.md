# Client Catalog Download Sorting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the standalone client catalog default to software update time and support stable cumulative-download sorting using data carried by the v2 source feed.

**Architecture:** Add `downloadCount` to the v2 feed app contract, fill it from the existing `App.DownloadCount`, cache it in `ClientSourceApp`, and expose it in `SourceAppDTO`. Keep sorting in the existing pure TypeScript helper so missing third-party values fall back to zero without changing v1 behavior.

**Tech Stack:** Go 1.26, ent, source feed v2 JSON, React 19, TypeScript 5.9, Node test runner.

## Global Constraints

- Do not change the v1 feed schema.
- Missing or invalid third-party `downloadCount` values behave as `0`.
- Recent sorting uses `softwareUpdatedAtMillis`; downloads ties use recent time then localized name.
- No new dependencies.
- Generated ent files must come only from `go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema`.

---

## File Map

- `internal/feed/feed.go`: shared app input/output download count field.
- `internal/feed/v2/v2_test.go`: v2 JSON contract regression.
- `internal/server/handlers_source.go`: fill feed input from `App.DownloadCount`.
- `internal/server/server_test.go`: source endpoint contract.
- `ent/schema/client_source_app.go`: local cached cumulative download count.
- `ent/`: regenerated client entities and migrations.
- `internal/clientserver/sync.go`: decode, normalize and persist feed download count.
- `internal/clientserver/types.go`: source app API DTO.
- `internal/clientserver/apps.go`: expose cached count.
- `internal/clientserver/schema_migrations.go`: increment local schema version and invalidate source ETags.
- `internal/clientserver/schema_migrations_test.go`: migration regression.
- `client/src/shared/types.ts`: `SourceApp.downloadCount`.
- `client/src/modules/client/clientUxState.ts`: stable downloads sort.
- `client/src/modules/client/clientUxState.test.mjs`: default and downloads behavior.
- `client/src/modules/client/ClientCatalog.tsx`: selector option.
- `client/src/App.tsx`: initial sort mode.

### Task 1: Extend the v2 feed contract

**Files:**
- Modify: `internal/feed/feed.go`
- Modify: `internal/server/handlers_source.go`
- Test: `internal/feed/v2/v2_test.go`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Produces: `feed.AppInput.DownloadCount int` and `feed.App.DownloadCount int`, serialized as `downloadCount`.
- Consumes: existing `ent.App.DownloadCount`.

- [ ] **Step 1: Write failing feed contract tests**

Add an approved app with `DownloadCount: 27` and assert both the built v2 value and JSON field:

```go
if got := index.Apps[0].DownloadCount; got != 27 {
	t.Fatalf("download count = %d, want 27", got)
}
raw, err := json.Marshal(index)
if err != nil {
	t.Fatal(err)
}
if !bytes.Contains(raw, []byte(`"downloadCount":27`)) {
	t.Fatalf("v2 feed missing downloadCount: %s", raw)
}
```
- [ ] **Step 2: Run the focused tests and confirm the contract is absent**

Run:

```bash
go test ./internal/feed/v2 ./internal/server -run 'Test.*Source|TestBuildIndex' -count=1
```

Expected: compile failure or assertion failure because `DownloadCount` is not in the feed app contract.

- [ ] **Step 3: Add the shared field and populate it**

Add the exact fields and copy operation:

```go
type AppInput struct {
	// existing fields
	DownloadCount int `json:"downloadCount,omitempty"`
}

type App struct {
	// existing fields
	DownloadCount int `json:"downloadCount,omitempty"`
}
```

In `BuildApp` set `DownloadCount: max(inApp.DownloadCount, 0)`. In `buildSourceFeed`, set `DownloadCount: max(record.DownloadCount, 0)` on `feed.AppInput`.

- [ ] **Step 4: Run the focused feed/server tests**

```bash
go test ./internal/feed/... ./internal/server -run 'Test.*Source|TestBuildIndex' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the v2 contract**

```bash
git add internal/feed/feed.go internal/feed/v2/v2_test.go internal/server/handlers_source.go internal/server/server_test.go
git commit -m "feat: expose app downloads in source feed"
```

### Task 2: Cache download counts in the standalone client

**Files:**
- Modify: `ent/schema/client_source_app.go`
- Regenerate: `ent/`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/schema_migrations.go`
- Test: `internal/clientserver/schema_migrations_test.go`
- Test: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: v2 app JSON `downloadCount`.
- Produces: `SourceAppDTO.DownloadCount int` with JSON key `downloadCount`.

- [ ] **Step 1: Add failing sync and DTO assertions**

Extend a source fixture with `"downloadCount":27`, sync it, and assert persistence plus API output:

```go
cached := app.db.ClientSourceApp.Query().OnlyX(ctx)
if cached.DownloadCount != 27 {
	t.Fatalf("download count = %d, want 27", cached.DownloadCount)
}
if !strings.Contains(rec.Body.String(), `"downloadCount":27`) {
	t.Fatalf("app response missing download count: %s", rec.Body.String())
}
```

Add a fixture with `"downloadCount":-4` and assert the cached value is zero.

- [ ] **Step 2: Run clientserver tests and confirm failure**

```bash
go test ./internal/clientserver -run 'Test.*Sync|Test.*SchemaMigration' -count=1
```

Expected: compile or assertion failure for the missing field.

- [ ] **Step 3: Add the ent field and regenerate**

Add to `ClientSourceApp.Fields()`:

```go
field.Int("download_count").Default(0).NonNegative(),
```

Then run:

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

- [ ] **Step 4: Thread the field through sync and DTO code**

Add `DownloadCount int` to `feedApp`, `sourceAppCacheRow`, and `SourceAppDTO`. Normalize before building the row:

```go
downloadCount := max(app.DownloadCount, 0)
```

Set it in `sourceAppCreateBuilder`, expose `app.DownloadCount` in `sourceAppDTO`, and include it in the source fixture expectations.

- [ ] **Step 5: Add migration version 4**

Set `currentClientSchemaVersion = 4`. The version-4 migration clears stored ETags so all existing sources fetch the new field:

```go
if version < 4 {
	if err := server.invalidateSourceFeedETags(ctx); err != nil {
		return err
	}
	if err := setSystemClientSetting(ctx, db, settingClientSchemaVersion, "4"); err != nil {
		return err
	}
}
```

The helper only sets `ClientSource.last_etag` to empty; it must not overwrite application update times.

- [ ] **Step 6: Run clientserver tests**

```bash
go test ./internal/clientserver -count=1
```

Expected: PASS, including negative-value normalization and migration ETag invalidation.

- [ ] **Step 7: Commit cache support**

```bash
git add ent internal/clientserver
git commit -m "feat: cache source app download counts"
```

### Task 3: Make recent the default and add download sorting

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/modules/client/clientUxState.ts`
- Modify: `client/src/modules/client/clientUxState.test.mjs`
- Modify: `client/src/modules/client/ClientCatalog.tsx`
- Modify: `client/src/App.tsx`

**Interfaces:**
- Consumes: `SourceApp.downloadCount?: number`.
- Produces: `ClientCatalogSortMode = 'recent' | 'downloads' | 'name' | 'source'`.

- [ ] **Step 1: Write failing pure sorting tests**

Add apps with ties and missing values:

```js
const apps = [
  { id: 1, name: 'Zulu', sourceName: 'B', downloadCount: 3, updatedAt: '2026-07-20T00:00:00Z' },
  { id: 2, name: 'Beta', sourceName: 'A', downloadCount: 9, updatedAt: '2026-07-18T00:00:00Z' },
  { id: 3, name: 'Alpha', sourceName: 'A', downloadCount: 9, updatedAt: '2026-07-19T00:00:00Z' },
  { id: 4, name: 'Missing', sourceName: 'C' },
];
assert.deepEqual(
  sortClientCatalogApps(apps, 'downloads', (app) => app.name).map((app) => app.id),
  [3, 2, 1, 4],
);
```

Add source assertions that `App.tsx` initializes `sortMode: 'recent'` and the selector contains downloads but no default-order option.

- [ ] **Step 2: Run the client state tests and confirm failure**

```bash
cd client && node --test --experimental-strip-types src/modules/client/clientUxState.test.mjs
```

Expected: FAIL because `downloads` is not a client catalog mode and the initial mode is `default`.

- [ ] **Step 3: Implement stable sorting and initial state**

Use this decision order in `sortClientCatalogApps`:

```ts
if (mode === 'downloads') {
  const downloadDelta = Math.max(0, Number(b.downloadCount) || 0) - Math.max(0, Number(a.downloadCount) || 0);
  if (downloadDelta !== 0) return downloadDelta;
  const timeDelta = softwareUpdatedAtMillis(b) - softwareUpdatedAtMillis(a);
  if (timeDelta !== 0) return timeDelta;
}
```

Keep the existing name fallback. Change the `App.tsx` client catalog initial state to `recent`; remove `default` from the type and selector; add `{ value: 'downloads', label: t('search.downloads') }`.

- [ ] **Step 4: Run frontend tests and build**

```bash
cd client && node --test --experimental-strip-types 'src/**/*.test.mjs'
cd client && npm run build
```

Expected: all tests PASS and TypeScript/Vite build succeeds.

- [ ] **Step 5: Run plan-level Go tests**

```bash
go test ./internal/feed/... ./internal/server/... ./internal/clientserver/...
```

Expected: PASS.

- [ ] **Step 6: Commit the catalog UI**

```bash
git add client/src/shared/types.ts client/src/modules/client/clientUxState.ts client/src/modules/client/clientUxState.test.mjs client/src/modules/client/ClientCatalog.tsx client/src/App.tsx
git commit -m "feat: sort client catalog by recent updates"
```
