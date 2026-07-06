# LPK V2 Metadata And Storefront Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete app-store flow where LPK V2 metadata is parsed accurately and the server/client frontends are split by user task instead of showing every feature on one screen.

**Architecture:** Add a focused LPK V2 metadata parser, persist LazyCat package identity as `package_id`, expose it through APIs and source feeds, then redesign the React shell into separate public storefront, server backstage, and standalone client workspaces. The public server frontend never shows device-installed apps; the standalone client remains source-first and device-local.

**Tech Stack:** Go HTTP handlers, ent + SQLite/Postgres/MySQL schema generation, LazyCat LPK V2 `package.yml`, React + TypeScript, Vite, vanilla CSS, lucide-react.

## Global Constraints

- No legacy data compatibility is required because the app is not listed yet.
- LPK scope is V2 only: metadata comes from `package.yml`, not v1 `manifest.yml`.
- Official LPK V2 packages are tar archives; URL/upload parsing may also accept a zip container only when it contains V2 `package.yml`.
- Server public frontend must focus on software list, categories, app detail, and source subscription instructions.
- Server backstage frontend is reached after login and is split into maintainer workspace and admin workspace.
- Server must not display client/device installed-app lists.
- Standalone client frontend must focus on software sources, source categories, installed source attribution, install history, and rollback.
- UI changes follow Emil-style interaction discipline: one primary action per screen, no dense information pile, transitions under 200ms for frequent actions, and stable responsive layouts.

---

## Brainstorming Audit

### Scope Decomposition

This is one product redesign with four independently testable delivery areas:

- **Server backend/API:** correct package identity and artifact metadata.
- **Server public frontend:** consumer storefront and source subscription.
- **Server backstage frontend:** maintainer and admin workflows behind login.
- **Standalone client frontend:** source-driven installer and local device inventory.

The backend metadata work should ship first because the frontend submission flow, source feed, client attribution, and rollback model all depend on stable `packageId`.

### Approaches Considered

| Approach | Shape | Tradeoff | Recommendation |
| --- | --- | --- | --- |
| Backend first, then split frontend by workspace | Parse/package identity first, then redesign server public/backstage and client UI against stable data | Slightly slower before visual changes land, but avoids UI work built on weak identity | Recommended |
| UI first, backend later | Redesign screens immediately, keep current slug/version assumptions | Faster visible progress, but app identity and upload forms remain inaccurate | Do not use |
| Rewrite frontend into many new files first | Component split before behavior changes | Cleaner structure, but high churn and harder review while behavior is still shifting | Avoid for this pass; split only after boundaries are clear |

### Product Decisions To Lock

- `packageId` is the LazyCat application identity and must come from `package.yml.package`.
- `slug` stays a storefront/display route field and must not be used as package identity.
- Form fields override package metadata when explicitly provided, but missing fields are filled from `package.yml`.
- URL-based LPK versions can auto-fill missing version and SHA256 by downloading the file once under size and timeout limits.
- Rollback means installing an older version still present in the source feed; it is not a hidden destructive operation.

### Risk Review

- **SSRF risk:** URL metadata fetching is user-controlled. Limit to `http`/`https`, apply timeout, cap bytes with `max_lpk_size`, reject non-2xx responses, and do not follow unsafe local file schemes.
- **Large file risk:** Parser reads from a temp file or bounded reader, not an unbounded memory buffer.
- **Identity mismatch risk:** Uploading a version whose `package.yml.package` differs from the app `package_id` should fail with a clear validation error.
- **UI scope risk:** Server front/back and client UI must be reviewed as information architecture, not only CSS polish.

---

## Task 1: LPK V2 Metadata Parser

**Files:**
- Create: `internal/lpkmeta/lpkmeta.go`
- Create: `internal/lpkmeta/lpkmeta_test.go`

**Interfaces:**
- Produces: `type Metadata struct { PackageID string; Version string; Name string; Description string; Author string; License string; Homepage string; MinOSVersion string }`
- Produces: `func ParseFile(path string) (Metadata, error)`
- Produces: `func ParseReaderAt(r io.ReaderAt, size int64) (Metadata, error)`

**Steps:**

- [ ] Add tests for tar LPK V2 with top-level `package.yml`.
- [ ] Add tests for zip container with top-level `package.yml`.
- [ ] Add tests for missing `package.yml`, malformed YAML, empty `package`, and empty `version`.
- [ ] Implement archive detection by content, not filename.
- [ ] Parse only `package.yml`; do not read v1 `manifest.yml`.
- [ ] Normalize metadata by trimming whitespace and preferring `locales.zh.name` only when top-level `name` is empty.
- [ ] Run `go test ./internal/lpkmeta`.

## Task 2: Server Database And API Identity

**Files:**
- Modify: `ent/schema/app.go`
- Regenerate: `ent/*`
- Modify: `internal/server/types.go`
- Modify: `internal/feed/feed.go`
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/install.go`

**Interfaces:**
- Produces database field: `apps.package_id`, unique and non-empty.
- Produces API field: `packageId`.
- Produces feed field: `apps[].packageId`.
- Client install uses `PackageID` from feed/package identity, not `slug`.

**Steps:**

- [ ] Add `package_id` to `App` schema and regenerate ent.
- [ ] Add `PackageID string json:"packageId"` to server app DTOs and source feed types.
- [ ] Add `PackageID string json:"packageId"` to client source app DTOs and cache schema.
- [ ] Update source sync to persist feed `packageId`.
- [ ] Update client install request construction to send `PackageID: dto.PackageID`.
- [ ] Add tests that source feed and client cache preserve `packageId`.
- [ ] Run `go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema`.
- [ ] Run `go test ./internal/feed ./internal/clientserver ./internal/server`.

## Task 3: Upload And URL Metadata Extraction

**Files:**
- Modify: `internal/server/handlers_apps.go`
- Create: `internal/server/lpk_fetch.go`
- Modify: `internal/server/server_test.go`
- Modify: `README.md`

**Interfaces:**
- Multipart app creation fills missing `name`, `slug`, `summary`, `description`, `packageId`, and `version` from uploaded LPK V2 metadata.
- Multipart version upload fills missing `version` from uploaded LPK V2 metadata.
- External URL app/version creation fills missing `version` and `sha256` by fetching the LPK URL once.
- External URL app creation fills missing app identity fields from URL LPK metadata when the file is reachable.

**Steps:**

- [ ] Change multipart app creation order so file metadata is parsed before `createAppRecord`.
- [ ] Add package mismatch validation for version uploads.
- [ ] Add bounded URL fetch helper with `http.Client{Timeout: 30 * time.Second}`, `max_lpk_size`, temporary file storage, SHA256 calculation, and V2 metadata parsing.
- [ ] Allow external version SHA256 to be omitted only when URL fetch succeeds.
- [ ] Keep explicit form/JSON fields authoritative over package metadata.
- [ ] Add tests for local upload creating app from `package.yml`.
- [ ] Add tests for URL app creation auto-filling metadata and SHA256 using `httptest.Server`.
- [ ] Add tests for package mismatch rejection.
- [ ] Update README capability text from "external versions require SHA256" to "SHA256 can be auto-detected from reachable LPK URLs".

## Task 4: Server Public Frontend Redesign

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/i18n.ts`
- Modify: `client/src/styles.css`

**Screens:**
- Public storefront home.
- Category/search browsing.
- App detail.
- Source subscription instructions.
- Login entry.

**Steps:**

- [ ] Replace the first server screen with storefront content: category/search entry, featured/recent apps, and source subscription panel.
- [ ] Move operational metrics and maintainer/admin actions out of the public first viewport.
- [ ] Make app cards show icon/avatar, name, one-line summary, category, latest version, and a single detail action.
- [ ] Make app detail follow product-page hierarchy: identity header, trust snapshot, screenshots, description, changelog, then actions.
- [ ] Add responsive layout checks for 320px, 768px, 1024px, and 1440px.

## Task 5: Server Backstage Frontend Redesign

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/i18n.ts`
- Modify: `client/src/styles.css`

**Screens:**
- Login boundary.
- Maintainer workspace: overview, submit, apps, versions, screenshots, API tokens, groups.
- Admin workspace: reviews, site identity, announcements, taxonomy, collections, users, storage/source policy.

**Steps:**

- [ ] Split logged-in server navigation into "My Apps" and "Admin" areas.
- [ ] Convert mixed profile content into task tabs so only one workflow is visible at a time.
- [ ] Build a focused submit wizard where LPK upload/URL metadata can prefill identity and version fields.
- [ ] Move Admin review queue to the first admin tab.
- [ ] Separate Admin settings into site identity, announcement, taxonomy, collections, users, and storage/source policy sections.
- [ ] Ensure public server pages never render installed-device data.

## Task 6: Standalone Client Frontend Redesign

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/i18n.ts`
- Modify: `client/src/styles.css`

**Screens:**
- Sources.
- Source catalog.
- Installed apps.
- Install history.
- Versions and rollback panel.

**Steps:**

- [ ] Make Sources the primary client workspace with add/sync/health actions.
- [ ] Make Source Catalog filter by source and category, and show source attribution on every app row/card.
- [ ] Fix installed app layout so long app names and app IDs clamp/ellipsis instead of creating vertical text columns.
- [ ] Add compact installed inventory rows with source attribution when known and "existing local app" when unknown.
- [ ] Add install history surface backed by client-side persisted history.
- [ ] Add version selection and rollback action for older versions present in the source feed.
- [ ] Keep server review/admin concepts hidden in standalone client mode.

## Task 7: Client History And Version Data

**Files:**
- Modify: `ent/schema/client_source_app.go`
- Create: `ent/schema/client_install_history.go`
- Regenerate: `ent/*`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/install.go`
- Modify: `internal/clientserver/apps.go`
- Modify: `internal/clientserver/server.go`
- Modify: `internal/clientserver/server_test.go`

**Interfaces:**
- Client source app cache stores all source versions, not only latest.
- `GET /api/client/v1/history` returns install attempts initiated by this client.
- `GET /api/client/v1/apps/{id}/versions` returns installable versions for rollback.
- `POST /api/client/v1/install` accepts a selected version.

**Steps:**

- [ ] Store full `versions_json` for each synced source app.
- [ ] Add install history schema with app identity, source, version, result, error, and timestamp.
- [ ] Record successful and failed install attempts.
- [ ] Add versions endpoint for selected source app.
- [ ] Update install endpoint to accept an explicit version string and install that version when present.
- [ ] Add tests for history recording, older version listing, and rollback install request construction.

## Task 8: Verification And Release

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-06-storefront-frontend-redesign-design.md`

**Steps:**

- [ ] Update OpenAPI for `packageId`, URL auto-detection behavior, client history, and version endpoints.
- [ ] Update the redesign spec with the final LPK V2 metadata decisions.
- [ ] Run `go test ./...`.
- [ ] Run `cd client && npm run build`.
- [ ] Run `npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml`.
- [ ] Run LazyCat YAML validation commands from README.
- [ ] Run `git diff --check`.
- [ ] Browser-check server public storefront, server backstage, client source catalog, client installed list, history, and rollback at desktop and mobile widths.
- [ ] Commit the completed implementation to `main`.
- [ ] Push `main` to origin.

## Acceptance Criteria

- Creating an app from an uploaded LPK V2 can succeed with identity/version fields filled from `package.yml`.
- Creating an app or version from a reachable LPK URL can auto-fill version and SHA256.
- Source feed contains stable `packageId`; standalone client installs with package identity from `packageId`.
- Server public frontend is a storefront, not an admin dashboard.
- Server backstage frontend is split into maintainer and admin workspaces.
- Standalone client frontend is source-first and includes source catalog, installed inventory, history, and rollback.
- Long installed app names and app IDs do not stretch grid rows or wrap into vertical columns.
- All tests and builds listed in Task 8 pass before commit and push.
