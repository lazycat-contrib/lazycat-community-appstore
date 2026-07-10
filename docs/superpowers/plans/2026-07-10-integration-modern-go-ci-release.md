# Integration, Modern Go, CI, and LazyCat Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate the three frontend workstreams, finish Go 1.26 and lint cleanup, enforce the full CI gate, regenerate embedded assets, bump both LazyCat app patch versions, build valid LPK v2 packages, and push the verified result.

**Architecture:** Keep `App.tsx`, navigation, language dictionaries, global styles, CI, generated assets, version metadata, and Git push under one integration owner. Merge frontend contracts first, then make mechanical Go cleanup after backend functional work is stable, run all source gates, generate standalone and server-specific bundles once, and package both LazyCat applications without publishing them to the app store.

**Tech Stack:** React 19, TypeScript 5.9, Vite 7, Go 1.26.4, golangci-lint 2.12.2, GitHub Actions, lzc-cli 2.0.8, LPK v2.

## Global Constraints

- Execute after FE-1, FE-2, FE-3, backend runtime, and backend resource plans complete their focused tests.
- Preserve current locale, rating, time-zone, Ent, OpenAPI, and generated-client WIP; do not restore files from `HEAD`.
- Integration owns `client/src/App.tsx`, `modules/shell/navigation.ts`, shared types, locale files, `styles.css`, CI, generated assets, version metadata, release packages, and push.
- Do not hand-edit generated Ent files. Regenerate them only from the checked-in schema when schema changes are part of the preserved WIP.
- Use Go syntax no newer than 1.26.4; benchmarks use `b.Loop()`, tests use `t.Context()`, WaitGroups use `WaitGroup.Go`, and typed errors use `errors.AsType`.
- Do not change JSON empty-array/object behavior merely to adopt `omitzero`.
- Build `clientembed/dist` exactly once from standalone configuration after all source changes; `web/dist` is generated with API base `.` and remains ignored except its README.
- LazyCat metadata remains LPK v2 with existing package IDs and minimum OS versions.
- Bump server `0.1.27 → 0.1.28` and client `0.1.22 → 0.1.23`.
- Build and inspect both LPKs, but do not call `lzc-cli appstore publish` or `lzc-publish`.
- Before pushing, fetch `origin`; if remote `main` advanced, rebase, rerun affected verification, and only then push `main`.
- Stage explicit paths; never use `git add -A` in this dirty worktree.

---

## File Map

- Modify `client/src/App.tsx` and `client/src/modules/shell/navigation.ts` for standalone landing and install lifecycle.
- Modify `client/src/shared/types.ts` for truthful install activity identity.
- Modify `client/src/locales/zh.ts` and `en.ts` for exact FE handoff copy.
- Modify `client/src/styles.css` for immediate tabs and reduced-motion behavior.
- Modify Go source files named by current lint and modern-Go scans.
- Create `.golangci.yml` and modify `.github/workflows/ci.yml`.
- Modify `go.sum` through `go mod tidy` only.
- Regenerate tracked `clientembed/dist/**` from the final standalone build.
- Modify `lazycat/server/package.yml` and `lazycat/client/package.yml`.
- Produce ignored `dist/community.lazycat.app-store-server-v0.1.28.lpk` and `dist/community.lazycat.app-store-v0.1.23.lpk`.

### Task 1: Integrate Shared Frontend Navigation and Install Lifecycle

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/shell/navigation.ts`
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/modules/search/SearchView.tsx` only for prop threading required by FE-2.
- Test: `client/src/modules/client/clientUxState.test.mjs` and all FE contract tests.

**Interfaces:**
- Standalone tabs order: Discover, Installed, Sources, History, Settings.
- Standalone initial route: Discover when sources exist, Sources when none exist.
- `syncAllSources` returns `Promise<{ success: number; failed: number }>`.
- `InstallActivity` gains `appKey` and truthful queued/prepare/system stages.
- Active installation identity is threaded into source cards and detail.

- [ ] **Step 1: Run the frontend contracts before integration**

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/client/clientUxState.test.mjs
node --test client/src/modules/admin/adminUxState.test.mjs
npm --prefix client exec -- tsc -b
```

Expected: all component contracts pass; TypeScript may fail only at integration-owned optional-prop handoffs documented below.

- [ ] **Step 2: Reorder standalone navigation**

Use this exact array in `navigation.ts`:

```ts
const clientBaseTabs: NavItem[] = [
  { key: 'search', labelKey: 'nav.install', icon: Download },
  { key: 'profile', labelKey: 'nav.installed', icon: Archive },
  { key: 'sources', labelKey: 'nav.sources', icon: Cloud },
  { key: 'history', labelKey: 'nav.history', icon: History },
  { key: 'settings', labelKey: 'nav.settings', icon: Settings },
];
```

- [ ] **Step 3: Resolve the standalone landing once**

Initialize standalone `tab` to `search`, add `clientLandingResolvedRef`, and after `loadClientSources` resolves set the first landing to `search` when at least one source exists or `sources` when none exist. Leave later user navigation untouched.

- [ ] **Step 4: Return persistent sync-all results**

No-source returns `{ success: 0, failed: 0 }` after navigating to Sources. Successful/partial sync returns the exact result after setting the existing Toast. Caught errors are rethrown with the translated message and original cause so FE-2's page result panel owns persistence.

- [ ] **Step 5: Extend install activity and remove simulated progress**

Use:

```ts
export type InstallActivity = {
  appKey: string;
  appId: number;
  version: string;
  title: string;
  source: string;
  checksum: string;
  status: 'running' | 'success' | 'error';
  progress: number;
  stageKey: string;
  resultMode?: string;
  messageKey?: string;
  messageParams?: Record<string, string | number>;
};
```

Store the last install request, block a second install while any activity is running, emit queued → prepare → system stages without a timer, keep terminal progress 100, and provide Retry and History callbacks to `InstallActivityPanel`.

- [ ] **Step 6: Thread active installation identity**

Pass `activeInstallKey={installActivity?.status === 'running' ? installActivity.appKey : undefined}` through `SearchView` and FE-2 catalog/grid props. Pass `isInstallPending` to `SourceAppDetailPage` when its computed source app key equals the active key.

- [ ] **Step 7: Run frontend contracts and commit**

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/client/clientUxState.test.mjs
node --test client/src/modules/admin/adminUxState.test.mjs
npm --prefix client exec -- tsc -b
git diff --check
```

Expected: every command exits 0.

```bash
git add client/src/App.tsx client/src/modules/shell/navigation.ts client/src/shared/types.ts client/src/modules/search/SearchView.tsx
git commit -m "feat: integrate client navigation and install states"
```

### Task 2: Merge Exact Frontend Copy and Global Motion Rules

**Files:**
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/styles.css`

**Interfaces:**
- Adds one storefront key, FE-2 source/install/history/settings keys, and FE-3 save-state keys.
- Replaces six dangerous-operation confirmation strings.
- Removes positional animation from repeated tab switches.
- Reduced motion retains color, border-color, and opacity feedback.

- [ ] **Step 1: Add storefront and FE-3 copy**

Add `search.clearFilters` as `清除筛选` / `Clear filters`. Under `admin` add:

```ts
saveState: {
  unsaved: '有未保存的更改',
  saving: '正在保存…',
  saved: '已保存',
  failed: '保存失败',
  noChanges: '没有未保存的更改',
},
```

Use the English equivalents: `Unsaved changes`, `Saving…`, `Saved`, `Save failed`, `No unsaved changes`.

- [ ] **Step 2: Replace dangerous-operation consequences**

Replace user/category/tag/collection/storage/invite confirmation copy with the exact object name and consequences supplied in `2026-07-10-admin-ux-hardening.md`. Preserve interpolation names `name` and `code`.

- [ ] **Step 3: Add FE-2 keys**

Add the exact bilingual values from the FE-2 INT table for:

```text
search.updatesAvailable
sources.onboardingTitle
sources.onboardingBody
sources.managementSubtitle
sources.groupCount
sources.syncingAll
sources.syncResultSuccess
sources.syncResultPartial
sources.syncResultCounts
sources.deleteTitle
sources.deleteBody
sources.deleteConfirm
installActivity.stageQueued
installActivity.stageSystem
installActivity.timeline
installActivity.steps.queued
installActivity.steps.prepare
installActivity.steps.system
installActivity.steps.result
profile.installedUpdates
profile.installedManaged
profile.installedLocalGroup
history.currentPageSuccess
history.currentPageFailed
history.refreshFailed
clientSettings.saveStates.clean
clientSettings.saveStates.dirty
clientSettings.saveStates.saving
clientSettings.saveStates.saved
clientSettings.saveStates.error
```

Add `common.sync`, `common.retry`, and `common.deleting` only if absent, using `同步/Sync`, `重试/Retry`, and `删除中/Deleting`.

- [ ] **Step 4: Reconcile global motion**

Replace the complete global `.settings-tab-panel` transition and `@starting-style` block with:

```css
.settings-tab-panel {
  display: grid;
  gap: 14px;
  min-width: 0;
}
```

Replace the wildcard 1 ms reduced-motion transition with explicit movement removal:

```css
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 1ms !important;
    animation-iteration-count: 1 !important;
    scroll-behavior: auto !important;
  }

  :where(button, [role='button'], .app-card, .source-app-card, .detail-page-shell) {
    transform: none !important;
  }
}
```

Do not disable color, border-color, or opacity transitions.

- [ ] **Step 5: Run locale/type checks and commit**

```bash
npm --prefix client exec -- tsc -b
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/client/clientUxState.test.mjs
node --test client/src/modules/admin/adminUxState.test.mjs
git diff --check
```

Expected: PASS.

```bash
git add client/src/locales/zh.ts client/src/locales/en.ts client/src/styles.css
git commit -m "feat: integrate frontend copy and motion"
```

### Task 3: Finish Go 1.26 and golangci-lint Cleanup

**Files:**
- Modify: `internal/assetdata/assetdata.go`
- Modify: `internal/assetdata/assetdata_test.go`
- Modify: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/install.go`
- Modify: `internal/clientserver/lazycat.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/lpkmeta/lpkmeta.go`
- Modify: `internal/storage/local.go`
- Modify: `internal/storage/s3.go`
- Modify: current lint-owned backend files only when prior plans did not already resolve them.
- Delete unused private helpers in `internal/server/backup.go`, `group_codes.go`, `lpk_fetch.go`, `respond.go`, and `storage_config.go`.

**Interfaces:**
- No public API or JSON contract changes.
- All four benchmarks use `b.Loop()`.
- Typed source-sync errors use `errors.AsType[sourceSyncError]`.
- Iteration-only splitting uses `strings.SplitSeq`.
- S3 custom endpoints use `s3.Options.BaseEndpoint`, not deprecated global resolvers.
- Important local output close errors are returned.

- [ ] **Step 1: Capture current lint output**

Run the installed binary and save its complete output:

```bash
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m > /tmp/appstore-lint-before.txt 2>&1 || true
```

Expected baseline: 39 findings before backend plans; rerun output may be smaller after their fixes.

- [ ] **Step 2: Apply mechanical Go 1.26 forms**

Change three `assetdata` benchmarks and `BenchmarkPreloadAppSummaries` to `for b.Loop()`. Use `b.Context()` where a benchmark needs context. Replace two `errors.As` calls in `clientserver/sync.go` with `errors.AsType[sourceSyncError]`. Replace the three iteration-only `strings.Split` loops in `assetdata.go` and `migration/zip.go` with `strings.SplitSeq`.

- [ ] **Step 3: Remove ineffective assignments**

Initialize screenshots directly from `catalogmeta.DecodeScreenshots`. Declare `var source appversion.SourceType` before the source-type switch. Declare `var principalType mcptoken.PrincipalType` before the token-type switch.

- [ ] **Step 4: Fix staticcheck findings**

Return the HEAD check expression directly in `ServeImageMetadata`. Lowercase the three internal install error strings. Replace the empty ZIP-error branch with an explicit ZIP attempt followed by TAR fallback. Accept only `tar.TypeReg`. Configure S3 with:

```go
cfg := aws.Config{
    Region: region,
    Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
        options.AccessKey,
        options.SecretKey,
        "",
    )),
}
client := awss3.NewFromConfig(cfg, func(s3Options *awss3.Options) {
    s3Options.BaseEndpoint = aws.String(endpoint)
    s3Options.UsePathStyle = options.PathStyle
})
```

- [ ] **Step 5: Make close intent explicit**

Use `defer func() { _ = body.Close() }()` for HTTP/multipart/read-only handles. Use `defer func() { _ = gw.Close() }()` for LazyCat gateways. In `LocalBackend.saveAt`, close the output before returning success and remove the partial file when copy or close fails:

```go
size, err := io.Copy(out, readerWithContext{ctx: ctx, reader: r})
if err != nil {
    _ = out.Close()
    _ = os.Remove(full)
    return Object{}, err
}
if err := out.Close(); err != nil {
    _ = os.Remove(full)
    return Object{}, err
}
```

Do not also defer `out.Close`.

- [ ] **Step 6: Remove only confirmed unused private helpers**

Delete the six helpers reported by the current `unused` analyzer only after `rg` confirms no call site. Keep `storageBackendForKey`.

- [ ] **Step 7: Run Go verification and commit**

```bash
gofmt -w internal/assetdata internal/clientserver internal/lpkmeta internal/migration internal/server internal/storage
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
```

Expected: every command exits 0 and lint reports zero findings.

Stage only files changed by this task and commit `refactor: adopt Go 1.26 conventions`.

### Task 4: Add Repeatable CI Gates and Tidy Dependencies

**Files:**
- Create: `.golangci.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `go.sum`

**Interfaces:**
- CI runs test, vet, race, lint, and tidy.
- Frontend keeps `npm ci`, high-severity audit, TypeScript/Vite build.
- OpenAPI and all LazyCat YAML remain validated.
- golangci-lint version is 2.12.2.

- [ ] **Step 1: Add focused linter configuration**

```yaml
version: "2"
run:
  timeout: 5m
linters:
  enable:
    - bodyclose
    - errcheck
    - govet
    - ineffassign
    - noctx
    - staticcheck
    - unused
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 2: Expand the server CI job**

Use separate steps in this order:

```yaml
- name: Test
  run: go test ./...
- name: Vet
  run: go vet ./...
- name: Race
  run: go test -race ./...
- name: Verify tidy
  run: go mod tidy -diff
- uses: golangci/golangci-lint-action@v8
  with:
    version: v2.12.2
    args: --timeout=5m
```

- [ ] **Step 3: Tidy the module**

Run `go mod tidy` once, inspect that only the known obsolete checksums are removed, then run `go mod tidy -diff`.

Expected: the diff command exits 0 and `go mod verify` succeeds.

- [ ] **Step 4: Run local equivalents and commit**

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
```

Expected: PASS.

```bash
git add .golangci.yml .github/workflows/ci.yml go.sum
git commit -m "ci: enforce Go quality gates"
```

### Task 5: Generate Final Frontend Assets Once

**Files:**
- Regenerate: `clientembed/dist/index.html` and `clientembed/dist/assets/**`.
- Preserve: `clientembed/dist/app-config.js` and static toys/playcaptcha files.
- Verify ignored: `client/dist/**` and `web/dist/**`.

- [ ] **Step 1: Install and run source checks**

```bash
npm --prefix client ci
node --test client/src/modules/storefront/storefront.contract.test.mjs
node --test client/src/modules/client/clientUxState.test.mjs
node --test client/src/modules/admin/adminUxState.test.mjs
npm --prefix client exec -- tsc -b
npm --prefix client audit --audit-level=high --registry=https://registry.npmjs.org
```

Expected: tests and type checking pass; audit has no high or critical vulnerability.

- [ ] **Step 2: Build standalone and refresh tracked embed assets**

```bash
VITE_API_BASE_URL="" npm --prefix client run build
rm -rf clientembed/dist/assets clientembed/dist/index.html
cp -R client/dist/assets clientembed/dist/assets
cp client/dist/index.html clientembed/dist/index.html
```

Expected: `clientembed/dist/app-config.js` remains and its version will be refreshed by the LazyCat client build in Task 6.

- [ ] **Step 3: Build the server bundle**

```bash
VITE_API_BASE_URL="." npm --prefix client run build
rm -rf web/dist
mkdir -p web/dist
cp -R client/dist/. web/dist/
```

Restore/create tracked `web/dist/README.md` with its existing explanatory text if the copy removed it.

- [ ] **Step 4: Verify hashed references and commit**

For both embed directories, extract each `src`/`href` beginning with `/assets/` or `./assets/` and assert the referenced file exists. Run `go test ./...` so both embed packages compile.

```bash
git add clientembed/dist web/dist/README.md
git commit -m "build: refresh embedded frontend assets"
```

### Task 6: Bump and Build Both LazyCat Applications

**Files:**
- Modify: `lazycat/server/package.yml`
- Modify: `lazycat/client/package.yml`
- Generate ignored release packages under `dist/`.
- Regenerate tracked `clientembed/dist/app-config.js` if the client buildscript changes its version.

**Interfaces:**
- Server package remains `community.lazycat.app-store-server`, version becomes `0.1.28`, minimum OS remains `1.5.2`.
- Client package remains `community.lazycat.app-store`, version becomes `0.1.23`, minimum OS remains `1.5.0`.
- No image copy or app-store publish occurs.

- [ ] **Step 1: Patch only the top-level versions**

Change exactly the two `version:` lines. Do not change package IDs, permissions, manifests, deploy parameters, or minimum OS versions.

- [ ] **Step 2: Validate LazyCat configuration shape**

```bash
npx --yes js-yaml lazycat/server/package.yml
npx --yes js-yaml lazycat/server/lzc-manifest.yml
npx --yes js-yaml lazycat/server/lzc-deploy-params.yml
npx --yes js-yaml lazycat/server/lzc-build.yml
npx --yes js-yaml lazycat/client/package.yml
npx --yes js-yaml lazycat/client/lzc-manifest.yml
npx --yes js-yaml lazycat/client/lzc-build.yml
lzc-cli --version
```

Expected: YAML parses and lzc-cli reports 2.0.8 or newer.

- [ ] **Step 3: Build explicit LPK v2 outputs**

```bash
cd lazycat/server
lzc-cli project release -o ../../dist/community.lazycat.app-store-server-v0.1.28.lpk
cd ../client
lzc-cli project release -o ../../dist/community.lazycat.app-store-v0.1.23.lpk
cd ../..
```

Expected: both commands exit 0. The client build refreshes `clientembed/dist/app-config.js` with `appVersion: "0.1.23"`.

- [ ] **Step 4: Inspect both packages**

```bash
lzc-cli lpk info dist/community.lazycat.app-store-server-v0.1.28.lpk
lzc-cli lpk info dist/community.lazycat.app-store-v0.1.23.lpk
```

Expected: both are LPK v2/tar, package IDs and versions match this task, and neither package ID has a `.dev` suffix.

- [ ] **Step 5: Commit metadata and regenerated client config**

```bash
git add lazycat/server/package.yml lazycat/client/package.yml clientembed/dist/app-config.js
git commit -m "release: bump LazyCat app versions"
```

Do not stage ignored `dist/*.lpk` or generated `lazycat/*/content`.

### Task 7: Final Verification, Review, and Push

**Files:**
- Modify only files already owned by this integration plan if a final gate exposes a defect.

- [ ] **Step 1: Run the complete gate from a clean index**

```bash
go test ./...
go vet ./...
go test -race ./...
/home/czyt/.local/share/mise/installs/golangci-lint/2.12.2/golangci-lint-2.12.2-linux-amd64/golangci-lint run --timeout=5m
go mod tidy -diff
go mod verify
npm --prefix client ci
npm --prefix client audit --audit-level=high --registry=https://registry.npmjs.org
npm --prefix client run build
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 2: Verify generated and release outputs**

Confirm `clientembed/dist/index.html` references existing assets, both LPK files still pass `lzc-cli lpk info`, and `git status --short` contains no unexplained source change. Ignored `client/dist`, `web/dist`, `dist`, and `lazycat/*/content` are allowed.

- [ ] **Step 3: Run broad code review**

Review the full range from commit `f280f90` through `HEAD` for spec compliance, concurrency correctness, resource bounds, UI accessibility, generated-asset consistency, and accidental user-WIP loss. Fix every Critical or Important finding and rerun its covering tests.

- [ ] **Step 4: Fetch and reconcile remote main**

```bash
git fetch origin
git rev-list --left-right --count origin/main...HEAD
```

If the left count is non-zero, run `git pull --rebase origin main`, resolve without discarding local work, and rerun Go tests, frontend build, and both LPK info checks. If the left count is zero, do not create a merge commit.

- [ ] **Step 5: Push the verified branch**

```bash
git push origin main
```

Expected: push succeeds and `git status --short --branch` shows `main...origin/main` with no ahead/behind count. Report the final commit SHA and both LazyCat versions.
