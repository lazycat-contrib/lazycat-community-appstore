# Client Recent Update Sort Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a recently updated sort option to the standalone client catalog by preserving the source feed's existing application update timestamp.

**Architecture:** Consume the existing `updatedAt` source field during synchronization, normalize it into the current client cache timestamp column, and expose it through the local app DTO. Keep sorting as a pure frontend operation so the standalone client does not need a new public server query parameter or source schema field.

**Tech Stack:** Go 1.25, Ent, SQLite, React 19, TypeScript, Node test runner, Vite.

## Global Constraints

- Add only the standalone client "Recently updated" sort option.
- Keep the current default, name, and source ordering unchanged.
- Do not add download count fields or download-period sorting.
- Do not change the public source feed schema because `updatedAt` already exists.
- Cache missing or invalid timestamps as the Unix epoch so they sort last.
- Use localized app name as the deterministic tie-breaker.
- Do not change the server package version `0.1.34` or client package version `0.1.28`.
- Do not build an LPK artifact.
- Commit and push all source, test, plan, design, and embedded asset changes.

---

### Task 1: Preserve source application update timestamps

**Files:**
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/apps.go`
- Test: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: source feed app field `updatedAt` as an RFC 3339 string.
- Produces: `SourceAppDTO.UpdatedAt time.Time` serialized as `updatedAt`.
- Produces: `sourceAppUpdatedAt(string) time.Time`, returning UTC or `time.Unix(0, 0).UTC()`.

- [ ] **Step 1: Write failing synchronization and fallback tests**

Extend the feed fixture in `TestSyncSourceCachesAppsAndUpdatesSource`:

```go
"updatedAt": "2026-07-15T05:04:03Z",
```

Assert that the local response and cached Ent record preserve it:

```go
if !strings.Contains(body, `"updatedAt":"2026-07-15T05:04:03Z"`) {
    t.Fatalf("cached app did not expose source update time: %s", body)
}
record := app.server.db.ClientSourceApp.Query().OnlyX(t.Context())
if got := record.UpdatedAt.UTC().Format(time.RFC3339); got != "2026-07-15T05:04:03Z" {
    t.Fatalf("cached update time = %q", got)
}
```

Add this table test:

```go
func TestSourceAppUpdatedAt(t *testing.T) {
    epoch := time.Unix(0, 0).UTC()
    tests := []struct {
        name string
        raw  string
        want time.Time
    }{
        {name: "valid", raw: "2026-07-15T05:04:03.123Z", want: time.Date(2026, 7, 15, 5, 4, 3, 123000000, time.UTC)},
        {name: "missing", want: epoch},
        {name: "invalid", raw: "yesterday", want: epoch},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := sourceAppUpdatedAt(tt.raw); !got.Equal(tt.want) {
                t.Fatalf("sourceAppUpdatedAt(%q) = %s, want %s", tt.raw, got, tt.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the focused test and confirm failure**

Run:

```bash
go test ./internal/clientserver -run 'TestSyncSourceCachesAppsAndUpdatesSource|TestSourceAppUpdatedAt' -count=1
```

Expected: FAIL because the feed timestamp is not decoded, cached, or returned.

- [ ] **Step 3: Add timestamp fields and normalization**

Add to `feedApp`:

```go
UpdatedAt string `json:"updatedAt"`
```

Add to `sourceAppCacheRow`:

```go
UpdatedAt time.Time
```

Add the normalization helper:

```go
func sourceAppUpdatedAt(raw string) time.Time {
    value, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
    if err != nil {
        return time.Unix(0, 0).UTC()
    }
    return value.UTC()
}
```

Set `UpdatedAt: sourceAppUpdatedAt(app.UpdatedAt)` in `buildSourceAppCacheRow`, call `SetUpdatedAt(row.UpdatedAt)` in `sourceAppCreateBuilder`, add `UpdatedAt time.Time \`json:"updatedAt"\`` to `SourceAppDTO`, and return `UpdatedAt: app.UpdatedAt` from `sourceAppDTO`.

- [ ] **Step 4: Run focused backend tests**

Run:

```bash
gofmt -w internal/clientserver/sync.go internal/clientserver/types.go internal/clientserver/apps.go internal/clientserver/server_test.go
go test ./internal/clientserver -run 'TestSyncSourceCachesAppsAndUpdatesSource|TestSourceAppUpdatedAt' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the backend timestamp propagation**

```bash
git add internal/clientserver/sync.go internal/clientserver/types.go internal/clientserver/apps.go internal/clientserver/server_test.go
git diff --cached --check
git commit -m "feat: preserve source app update times"
```

### Task 2: Add deterministic recent sorting to the client catalog

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/modules/client/clientUxState.ts`
- Modify: `client/src/modules/client/clientUxState.test.mjs`
- Modify: `client/src/modules/client/ClientCatalog.tsx`

**Interfaces:**
- Produces: `SourceApp.updatedAt?: string`.
- Produces: `ClientCatalogSortMode = 'default' | 'recent' | 'name' | 'source'`.
- Produces: `sortClientCatalogApps<T>(apps, mode, displayName): T[]`.

- [ ] **Step 1: Write failing sort behavior and selector tests**

Import `sortClientCatalogApps` in `clientUxState.test.mjs` and add:

```js
test('client catalog recent sorting uses source update time and stable name fallback', () => {
  const apps = [
    { id: 1, name: 'Missing', sourceName: 'A' },
    { id: 2, name: 'Beta', sourceName: 'A', updatedAt: '2026-07-15T00:00:00Z' },
    { id: 3, name: 'Alpha', sourceName: 'B', updatedAt: '2026-07-15T00:00:00Z' },
    { id: 4, name: 'Newest', sourceName: 'B', updatedAt: '2026-07-16T00:00:00Z' },
  ];
  assert.deepEqual(
    sortClientCatalogApps(apps, 'recent', (app) => app.name).map((app) => app.id),
    [4, 3, 2, 1],
  );
  assert.deepEqual(sortClientCatalogApps(apps, 'default', (app) => app.name), apps);
});
```

Read `ClientCatalog.tsx` in the contract test and assert:

```js
assert.match(catalog, /\{ value: 'recent', label: t\('search\.recent'\) \}/);
assert.match(catalog, /sortClientCatalogApps\(filtered, sortMode, localizedAppName\)/);
```

- [ ] **Step 2: Run frontend tests and confirm failure**

```bash
node --test client/src/modules/client/clientUxState.test.mjs
```

Expected: FAIL because the sort mode and helper do not exist.

- [ ] **Step 3: Add the shared app timestamp and pure sorting helper**

Add `updatedAt?: string` to `SourceApp` in `client/src/shared/types.ts`.

Add to `clientUxState.ts`:

```ts
export type ClientCatalogSortMode = 'default' | 'recent' | 'name' | 'source';

type SortableSourceApp = { sourceName: string; updatedAt?: string };

function sourceAppUpdatedAtMillis(value?: string) {
  const timestamp = Date.parse(value || '');
  return Number.isFinite(timestamp) ? timestamp : 0;
}

export function sortClientCatalogApps<T extends SortableSourceApp>(
  apps: T[],
  mode: ClientCatalogSortMode,
  displayName: (app: T) => string,
) {
  if (mode === 'default') return [...apps];
  return [...apps].sort((a, b) => {
    if (mode === 'recent') {
      const timeDelta = sourceAppUpdatedAtMillis(b.updatedAt) - sourceAppUpdatedAtMillis(a.updatedAt);
      if (timeDelta !== 0) return timeDelta;
    }
    if (mode === 'source') {
      const sourceDelta = a.sourceName.localeCompare(b.sourceName);
      if (sourceDelta !== 0) return sourceDelta;
    }
    return displayName(a).localeCompare(displayName(b));
  });
}
```

- [ ] **Step 4: Use the helper and add the selector option**

In `ClientCatalog.tsx`, import `ClientCatalogSortMode` and `sortClientCatalogApps`, remove the local sort type, replace the inline sort with `return sortClientCatalogApps(filtered, sortMode, localizedAppName)`, and add:

```tsx
{ value: 'recent', label: t('search.recent') },
```

- [ ] **Step 5: Run frontend tests and production build**

```bash
find client/src -name '*.test.mjs' -print0 | xargs -0 node --test
npm run build --prefix client
```

Expected: PASS.

### Task 3: Refresh embedded assets, verify, commit, and push

**Files:**
- Modify: `clientembed/dist/**`
- Verify: `lazycat/server/package.yml`
- Verify: `lazycat/client/package.yml`

**Interfaces:**
- Consumes: `client/dist` from Task 2.
- Produces: embedded assets matching the frontend build except for `app-config.js`.

- [ ] **Step 1: Synchronize embedded frontend assets**

```bash
before=$(sha256sum clientembed/dist/app-config.js | cut -d' ' -f1)
find clientembed/dist -mindepth 1 -depth ! -path clientembed/dist/app-config.js -delete
find client/dist -mindepth 1 -maxdepth 1 ! -name app-config.js -exec cp -a {} clientembed/dist/ \;
after=$(sha256sum clientembed/dist/app-config.js | cut -d' ' -f1)
test "$before" = "$after"
diff -qr --exclude=app-config.js client/dist clientembed/dist
```

- [ ] **Step 2: Confirm package versions remain unchanged**

```bash
test "$(sed -n 's/^version: //p' lazycat/server/package.yml)" = "0.1.34"
test "$(sed -n 's/^version: //p' lazycat/client/package.yml)" = "0.1.28"
```

- [ ] **Step 3: Run complete verification**

```bash
go test ./... -count=1
go test -race ./internal/clientserver -count=1
go vet ./...
go mod tidy -diff
golangci-lint run --timeout=5m
find client/src -name '*.test.mjs' -print0 | xargs -0 node --test
npm audit --prefix client --audit-level=high --registry=https://registry.npmjs.org
npm run build --prefix client
diff -qr --exclude=app-config.js client/dist clientembed/dist
```

Expected: all commands exit zero and the audit reports no high or critical vulnerabilities.

- [ ] **Step 4: Commit frontend and generated assets**

```bash
git add client/src/shared/types.ts client/src/modules/client/clientUxState.ts client/src/modules/client/clientUxState.test.mjs client/src/modules/client/ClientCatalog.tsx clientembed/dist
git diff --cached --check
git commit -m "feat: sort client apps by recent updates"
```

- [ ] **Step 5: Push and confirm remote state**

```bash
git status --short --branch -uall
git push origin main
git fetch origin main
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
```

- [ ] **Step 6: Confirm GitHub CI**

```bash
run_id=$(gh run list --commit "$(git rev-parse HEAD)" --limit 1 --json databaseId --jq '.[0].databaseId')
test -n "$run_id"
gh run view "$run_id" --json status,conclusion,jobs
```

Require Server, Client, API Contract, and LazyCat Config to complete successfully.
