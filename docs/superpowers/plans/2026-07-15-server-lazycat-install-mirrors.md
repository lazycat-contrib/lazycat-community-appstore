# Server LazyCat Install Mirror Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let trusted server storefront installs choose an applicable configured GitHub mirror while keeping direct download as the default.

**Architecture:** Extend the no-store runtime capability response with safe mirror summaries, reuse the existing install options dialog through a small shared mirror-config interface, and send only a mirror ID back to the server. The server reloads and validates current mirror configuration, rewrites its database-derived version URL, and keeps the frontend unable to supply an artifact URL.

**Tech Stack:** Go 1.25, `net/http`, Ent, existing `internal/mirror`, React 19, TypeScript, Vite, Node test runner, OpenAPI 3.

## Global Constraints

- Direct download is always the default server selection.
- GitHub Release/Code URLs use only `download` mirrors; raw GitHub URLs use only `raw` mirrors.
- Runtime capability mirror summaries contain only `id`, `kind`, and `name`.
- The install request contains only `installPassword` and `mirrorId`; it never contains a URL.
- The server independently validates the mirror against current configuration and the approved version URL.
- Existing standalone client source defaults remain unchanged.
- Server version becomes `0.1.34`; client version remains `0.1.28`.
- Do not build an LPK artifact; commit and push the source changes.

---

### Task 1: Server mirror discovery and validation

**Files:**
- Modify: `internal/server/handlers_install.go`
- Modify: `internal/server/handlers_install_test.go`

**Interfaces:**
- Produces: runtime JSON `{lazycatInstall: bool, githubMirrors: []runtimeMirrorOption}`.
- Produces: `installVersionInput{InstallPassword string, MirrorID string}`.
- Consumes: `mirror.FindApplicable`, `mirror.IsGitHubURL`, and `mirror.RewriteGitHub`.

- [ ] **Step 1: Write failing capability tests**

Add configured download/raw mirrors, request trusted capabilities, and assert JSON includes ID/kind/name but excludes configured base URLs. Assert an untrusted request returns `githubMirrors:[]`.

- [ ] **Step 2: Write failing installation mirror tests**

Assert direct installs preserve the version URL; valid download mirrors rewrite a GitHub Release URL; raw/download mismatches and unknown IDs return `422`; a non-GitHub URL plus `mirrorId` returns `MIRROR_NOT_APPLICABLE`; and a JSON `mirrorUrl` field is rejected as `INVALID_JSON`.

- [ ] **Step 3: Run focused tests and confirm failure**

Run: `go test ./internal/server -run 'TestLazyCat(RuntimeCapabilities|Install.*Mirror)' -count=1`

Expected: FAIL because capabilities omit mirrors and the install body does not accept or validate `mirrorId`.

- [ ] **Step 4: Implement safe runtime mirror summaries**

Add:

```go
type runtimeMirrorOption struct {
    ID   string `json:"id"`
    Kind string `json:"kind"`
    Name string `json:"name"`
}
```

Return an empty slice when LazyCat installation is unavailable; otherwise map `s.effectiveGitHubMirrors(r.Context())` without copying `URL`.

- [ ] **Step 5: Implement server-side mirror selection**

Extend the input with `MirrorID string \`json:"mirrorId"\``. Start from `versionRecord.DownloadURL`; when the trimmed ID is non-empty, reject non-GitHub URLs, call `mirror.FindApplicable`, and rewrite with `mirror.RewriteGitHub`. Pass the resulting server-derived URL to `lazycatInstaller.InstallLPK`.

- [ ] **Step 6: Run focused backend tests**

Run: `go test ./internal/server -run 'TestLazyCat(RuntimeCapabilities|Install)' -count=1`

Expected: PASS.

### Task 2: Shared frontend mirror configuration and dialog reuse

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/shared/utils.ts`
- Modify: `client/src/modules/client/InstallOptionsDialog.tsx`
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/modules/client/clientUxState.test.mjs`

**Interfaces:**
- Produces: `GitHubMirrorOption {id, kind, name}` and `InstallMirrorConfig {githubMirrors, defaultDownloadMirrorId, defaultRawMirrorId}`.
- Extends: `RuntimeCapabilities.githubMirrors: GitHubMirrorOption[]`.
- Changes: `InstallOptionsDialog` consumes `mirrorConfig?: InstallMirrorConfig` instead of deriving mirrors only from `source`.

- [ ] **Step 1: Write failing helper and contract tests**

Cover download/raw filtering, non-GitHub empty options, direct server default, source defaults, server dialog opening only when applicable mirrors exist or a password is required, and install POST containing `mirrorId` but no URL.

- [ ] **Step 2: Run frontend tests and confirm failure**

Run: `find client/src -name '*.test.mjs' -print0 | xargs -0 node --test`

Expected: FAIL on the new mirror-config and server-dialog assertions.

- [ ] **Step 3: Generalize mirror types and helpers**

Define:

```ts
export type GitHubMirrorOption = Pick<GitHubMirror, 'id' | 'kind' | 'name'>;
export type InstallMirrorConfig = {
  githubMirrors: GitHubMirrorOption[];
  defaultDownloadMirrorId: string;
  defaultRawMirrorId: string;
};
```

Update `applicableMirrorsForVersion` and `defaultMirrorIDForVersion` to accept `InstallMirrorConfig | undefined`. A `SourceSubscription` remains structurally compatible.

- [ ] **Step 4: Reuse the existing dialog**

Pass `mirrorConfig` into `InstallOptionsDialog`, compute options with the generalized helpers, and preserve the existing direct option plus source-specific defaults.

- [ ] **Step 5: Add server install-option behavior**

Build a server config from runtime capabilities with empty default IDs. Before setting `installPasswordRequest`, determine whether the selected store version has applicable mirrors. Open the dialog when `app.installProtected` or applicable server mirrors exist; otherwise install directly. Include `mirrorId: options.mirrorId || ''` in the server POST.

- [ ] **Step 6: Run frontend tests and build**

Run: `find client/src -name '*.test.mjs' -print0 | xargs -0 node --test`

Run: `npm run build --prefix client`

Expected: PASS.

### Task 3: API contract, embedded assets, and version

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `docs/openapi_test.go`
- Modify: `lazycat/server/package.yml`
- Modify: `clientembed/dist/**`

**Interfaces:**
- Documents `RuntimeMirrorOption`, `RuntimeCapabilities.githubMirrors`, `LazyCatInstallRequest.mirrorId`, and mirror error responses.

- [ ] **Step 1: Extend the OpenAPI contract**

Add the mirror summary array and optional `mirrorId`; describe `422 MIRROR_NOT_APPLICABLE` and `MIRROR_NOT_FOUND`. Extend the Go doc guard to require the new schema/property names.

- [ ] **Step 2: Update only the server version**

Change `lazycat/server/package.yml` from `0.1.33` to `0.1.34`. Confirm `lazycat/client/package.yml` remains `0.1.28`.

- [ ] **Step 3: Refresh embedded assets**

Run the frontend build, replace tracked `clientembed/dist` content from `client/dist`, and preserve the existing generated `clientembed/dist/app-config.js`.

- [ ] **Step 4: Validate parity and contracts**

Run: `diff -qr --exclude=app-config.js client/dist clientembed/dist`

Run: `go test ./docs -count=1`

Run: `npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml`

Expected: PASS.

### Task 4: Full verification, commit, and push

**Files:**
- Verify every changed source, generated asset, manifest, and plan/spec file.

- [ ] **Step 1: Run Go verification**

Run: `go test ./... -count=1`

Run: `go test -race ./internal/server ./internal/lazycatpkg -run 'TestLazyCat|TestWithIdentity' -count=1`

Run: `go vet ./...`

Run: `go mod tidy -diff`

Run: `golangci-lint run --timeout=5m`

Expected: all exit zero.

- [ ] **Step 2: Run frontend and packaging verification**

Run: `find client/src -name '*.test.mjs' -print0 | xargs -0 node --test`

Run: `npm audit --prefix client --audit-level=high --registry=https://registry.npmjs.org`

Run: `npm run build --prefix client`

Run: `diff -qr --exclude=app-config.js client/dist clientembed/dist`

Expected: all exit zero and audit reports no high/critical findings.

- [ ] **Step 3: Commit intended changes**

Run: `git add <changed feature files> && git diff --cached --check && git commit -m "feat: add server install mirror selection"`

- [ ] **Step 4: Push and verify remote state**

Run: `git push origin main && git fetch origin main && test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"`

Expected: push succeeds, hashes match, and the worktree is clean.

- [ ] **Step 5: Confirm GitHub CI**

Read the run for the pushed commit with `gh run list --commit <sha>` and `gh run view <run-id> --json status,conclusion,jobs`. Require Server, Client, API Contract, and LazyCat Config to complete successfully.
