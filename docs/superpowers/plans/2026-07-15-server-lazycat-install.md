# Server LazyCat Installation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let trusted LazyCat PC/mobile requests install a selected storefront LPK on the device hosting the server while ordinary web requests keep downloading it.

**Architecture:** A shared `internal/lazycatpkg` adapter owns Go SDK metadata, gateway lifetime, request construction, and result mapping. The server exposes a no-store capability endpoint plus a guarded install endpoint that resolves all artifact data from the database. The React app loads the capability fail-closed and changes server-storefront actions without affecting the standalone client flow.

**Tech Stack:** Go 1.25, `gitee.com/linakesi/lzc-sdk`, `net/http`, Ent, React 19, TypeScript, Vite, Node test runner, OpenAPI 3.

## Global Constraints

- LazyCat PC, iOS, and Android use the same trusted-header behavior.
- Installation always targets the device running the server application.
- Require `TRUST_LAZYCAT_CLIENT_INSTALL=true`, non-empty `x-hc-user-id`, and non-empty `x-hc-device-id`.
- Never accept an artifact URL, SHA256, package ID, user ID, or device ID from the browser.
- Capability and install responses use `Cache-Control: no-store`.
- Capability failure preserves the existing web download behavior.
- Server version becomes `0.1.33`; client version stays `0.1.28`.
- Do not build LPK release artifacts.

---

### Task 1: Shared Go SDK installer

**Files:**
- Create: `internal/lazycatpkg/install.go`
- Create: `internal/lazycatpkg/install_test.go`
- Modify: `internal/clientserver/lazycat.go`
- Modify: `internal/clientserver/lazycat_test.go`

**Interfaces:**
- Produces: `lazycatpkg.Identity{UserID, DeviceID string}`.
- Produces: `lazycatpkg.InstallRequest{DownloadURL, SHA256, PackageID, Name string}`.
- Produces: `lazycatpkg.InstallResult{Mode, TaskID, Status, Detail string}`.
- Produces: `lazycatpkg.InstallLPK(ctx context.Context, identity Identity, req InstallRequest) (InstallResult, error)`.

- [ ] **Step 1: Write a failing request-construction test**

Assert that the internal request waits for completion and maps URL, SHA256, package ID, and temporary title exactly.

- [ ] **Step 2: Run the focused test**

Run: `go test ./internal/lazycatpkg -run TestSynchronousInstallLPKRequest -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the shared installer**

Create an outgoing context with trimmed `x-hc-user-id` and `x-hc-device-id`, apply a 60-second timeout, create and close `gohelper.NewAPIGateway`, call `PkgManager.InstallLPK`, and map task fields into `InstallResult`.

- [ ] **Step 4: Rewire the standalone client adapter**

Keep its public `PackageManager` interface unchanged. Convert `InstallRequestDTO` to `lazycatpkg.InstallRequest`, call the shared function with the current user ID, and convert the result back to `InstallResultDTO`.

- [ ] **Step 5: Run focused client and shared tests**

Run: `go test ./internal/lazycatpkg ./internal/clientserver -count=1`

Expected: PASS.

### Task 2: Server capability and installation endpoints

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/server/server.go`
- Create: `internal/server/handlers_install.go`
- Create: `internal/server/handlers_install_test.go`
- Modify: `lazycat/server/lzc-manifest.yml`

**Interfaces:**
- Produces: `GET /api/v1/runtime/capabilities -> {"lazycatInstall": boolean}`.
- Produces: `POST /api/v1/apps/{id}/versions/{versionId}/install` with optional `{installPassword}`.
- Produces: an internal `lazycatInstaller` interface for deterministic handler tests.

- [ ] **Step 1: Add failing capability tests**

Cover trust disabled, one missing header, both headers present, and `Cache-Control: no-store`.

- [ ] **Step 2: Add failing install tests**

Use a fake installer to assert server-derived URL, SHA256, package ID, title, user ID, and device ID. Cover missing context, wrong password, version mismatch, SDK error, successful count increment, and no increment on failure.

- [ ] **Step 3: Run focused server tests**

Run: `go test ./internal/server -run 'TestLazyCat(RuntimeCapabilities|Install)' -count=1`

Expected: FAIL because routes and installer dependency do not exist.

- [ ] **Step 4: Add configuration and routes**

Load `TRUST_LAZYCAT_CLIENT_INSTALL` with a false default. Register the two routes and initialize the production installer in `server.New`.

- [ ] **Step 5: Implement guarded handlers**

Require the trust flag plus both platform headers on every install request. Reuse existing application visibility and installation-password rules, require approved app/version records, reject empty download URLs, call the fakeable installer, record a download only after success, and emit safe structured errors.

- [ ] **Step 6: Enable the feature only in the LazyCat server manifest**

Add `TRUST_LAZYCAT_CLIENT_INSTALL=true` to the server environment list.

- [ ] **Step 7: Run focused server tests**

Run: `go test ./internal/config ./internal/server -count=1`

Expected: PASS.

### Task 3: Storefront capability-driven actions

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/storefront/AppGrid.tsx`
- Modify: `client/src/modules/storefront/StorefrontHome.tsx`
- Modify: `client/src/modules/storefront/StorefrontSearch.tsx`
- Modify: `client/src/modules/search/SearchView.tsx`
- Modify: `client/src/modules/storefront/AppDrawer.tsx`
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`

**Interfaces:**
- Consumes: `{lazycatInstall: boolean}` from the runtime-capabilities endpoint.
- Produces: `lazycatInstall: boolean` props for server storefront grid/search/detail components.

- [ ] **Step 1: Add failing frontend contract tests**

Assert that App startup requests runtime capabilities, server installs POST to the selected version endpoint, web mode still opens the download endpoint, and grid/detail/history labels depend on `lazycatInstall`.

- [ ] **Step 2: Run frontend contract tests**

Run: `node --test client/src/modules/storefront/storefront.contract.test.mjs`

Expected: FAIL on the new capability/install assertions.

- [ ] **Step 3: Load capability fail-closed**

Add `RuntimeCapabilities`, initialize `{lazycatInstall:false}`, include the endpoint in server refresh with `Promise.allSettled`, and keep false on any failure.

- [ ] **Step 4: Route server installs through the new endpoint**

In `installApp`, keep source-app behavior unchanged. For a store app with `lazycatInstall=true`, POST only `{installPassword}` to `/api/v1/apps/{appId}/versions/{versionId}/install`; otherwise retain `window.open` on the existing download URL.

- [ ] **Step 5: Propagate action presentation**

Pass `lazycatInstall` through home, search, grid, and drawer components. Use **Install** and a package icon in LazyCat mode; preserve current web download and historical-download wording otherwise.

- [ ] **Step 6: Run contract and build checks**

Run: `node --test client/src/modules/storefront/storefront.contract.test.mjs`

Run: `npm run build --prefix client`

Expected: PASS.

### Task 4: API contract, embedded assets, and server version

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `docs/openapi_test.go`
- Modify: `lazycat/server/package.yml`
- Modify: `clientembed/dist/**`
- Modify: `docs/superpowers/specs/2026-07-15-server-lazycat-install-design.md`

**Interfaces:**
- Documents the capability response, install input/result, and structured error statuses.

- [ ] **Step 1: Extend OpenAPI and its route guard**

Describe both endpoints, the optional password body, success result, `403`, `404`, `409`, and `502` responses. Assert both paths exist in `docs/openapi_test.go`.

- [ ] **Step 2: Advance only the server package version**

Change `lazycat/server/package.yml` from `0.1.32` to `0.1.33`; leave `lazycat/client/package.yml` at `0.1.28`.

- [ ] **Step 3: Refresh embedded frontend assets**

Run: `npm run build --prefix client`

Replace `clientembed/dist` with the resulting `client/dist` while preserving the repository's generated asset layout.

- [ ] **Step 4: Validate contracts and embedded parity**

Run: `go test ./docs -count=1`

Run: `diff -qr --exclude=app-config.js client/dist clientembed/dist`

Expected: PASS with no diff output.

### Task 5: Full verification, commit, and push

**Files:**
- Verify all changed files.

- [ ] **Step 1: Format and inspect**

Run: `gofmt -w internal/lazycatpkg/*.go internal/clientserver/lazycat.go internal/server/handlers_install*.go internal/config/config*.go`

Run: `git diff --check`

Expected: no errors.

- [ ] **Step 2: Run backend verification**

Run: `go test ./... -count=1`

Run: `go test -race ./internal/lazycatpkg ./internal/server ./internal/clientserver -count=1`

Run: `go vet ./...`

Run: `go mod tidy -diff`

Expected: all pass and tidy produces no diff.

- [ ] **Step 3: Run frontend and configuration verification**

Run: `npm audit --prefix client --audit-level=high --registry=https://registry.npmjs.org`

Run: `npm run build --prefix client`

Run: `npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml`

Run: `npx --yes js-yaml lazycat/server/package.yml && npx --yes js-yaml lazycat/server/lzc-manifest.yml`

Expected: all pass.

- [ ] **Step 4: Commit implementation**

Run: `git add -A && git commit -m "feat: install storefront apps on LazyCat server"`

- [ ] **Step 5: Push and verify remote parity**

Run: `git push origin main && git fetch origin main && test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"`

Expected: push succeeds and local/remote hashes match.
