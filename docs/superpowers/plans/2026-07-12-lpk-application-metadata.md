# LPK Application Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist LPK author, homepage, license, and minimum OS version on the server and client and display them in application details.

**Architecture:** Add four backward-compatible application-level string columns to the server and client Ent schemas. Carry them through existing LPK inspection, migration, source-feed, sync, API, and React detail-view paths. Existing records and older source documents use empty defaults.

**Tech Stack:** Go, Ent, SQLite/PostgreSQL/MySQL, JSON source v2, React, TypeScript, i18next.

## Global Constraints

- LPK author is distinct from the community store owner/submitter.
- New JSON fields remain optional and backward compatible.
- Homepage is clickable only for `http` and `https` URLs.
- Minimum OS version is informational and does not block installation.
- Empty metadata rows are not rendered.

---

### Task 1: Server persistence and LPK application

**Files:**
- Modify: `ent/schema/app.go`
- Modify: generated Ent files through `go generate ./ent`
- Modify: `internal/server/handlers_apps.go`
- Modify: `internal/server/lpk_inspection.go`
- Test: `internal/server/server_test.go`
- Test: `internal/server/lpk_inspection_test.go`

**Interfaces:**
- Consumes: `lpkinspect.Metadata.Author`, `License`, `Homepage`, and `MinOSVersion`.
- Produces: `ent.App.Author`, `Homepage`, `License`, and `MinOSVersion` with empty-string defaults.

- [ ] **Step 1: Extend existing LPK create and inspection tests**

Add package metadata to the LPK fixtures and assert that app creation, fill-missing inspection, and overwrite inspection persist all four values while keeping owner identity separate.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/server -run 'Test.*LPK|TestCreateAppMultipart'`

Expected: compilation or assertion failure because generated `ent.App` has no metadata fields.

- [ ] **Step 3: Add schema fields and metadata application**

Add to `App.Fields()`:

```go
field.String("author").Default(""),
field.String("homepage").Default(""),
field.String("license").Default(""),
field.String("min_os_version").Default(""),
```

Generate Ent code, then set these fields in `applyAppMetadata`, `updateAppFromApprovedLPKMetadata`, and `applyLPKInspectionMetadata`. Background inspection follows the existing fill-missing/overwrite rule.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/lpkinspect ./internal/server -run 'Test.*LPK|TestCreateAppMultipart'`

Expected: PASS.

### Task 2: Server DTO, feed, migration, and API contracts

**Files:**
- Modify: `internal/server/types.go`
- Modify: `internal/server/handlers_apps.go`
- Modify: `internal/server/app_list_preload.go`
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/feed/feed.go`
- Modify: `internal/server/handlers_mcp.go`
- Modify: `internal/migration/records.go`
- Modify: `internal/migration/importer.go`
- Modify: `docs/openapi.yaml`
- Test: `internal/feed/feed_test.go`
- Test: `internal/migration/migration_test.go`
- Test: `docs/openapi_test.go`

**Interfaces:**
- Consumes: server `ent.App` metadata fields.
- Produces: JSON properties `author`, `homepage`, `license`, and `minOSVersion` in app/source contracts and logical backups.

- [ ] **Step 1: Add failing serialization and migration assertions**

Seed all four values in feed and migration fixtures, assert their JSON representation, import them into a target database, and assert equality after restore.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/feed ./internal/migration ./docs`

Expected: FAIL because DTO and migration record fields are absent.

- [ ] **Step 3: Carry fields through all server outputs**

Add the four properties to `appSummary`, feed `AppInput`/`App`, source DTO mapping, MCP summary mapping, migration `AppRecord`, export select list, and importer create/update builders. Document them in OpenAPI as optional strings.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/feed ./internal/migration ./internal/server ./docs`

Expected: PASS.

### Task 3: Client source persistence and API output

**Files:**
- Modify: `ent/schema/client_source_app.go`
- Modify: generated Ent files through `go generate ./ent`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/apps.go`
- Test: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: optional source JSON properties from Task 2.
- Produces: locally persisted and returned client app metadata.

- [ ] **Step 1: Add a failing client synchronization test**

Serve a source app with all four fields, synchronize it, inspect the `ClientSourceApp` record and app API response, and assert exact values.

- [ ] **Step 2: Run focused test and confirm failure**

Run: `go test ./internal/clientserver -run 'Test.*Sync|Test.*Apps'`

Expected: compilation or assertion failure because the client entity and DTO lack the fields.

- [ ] **Step 3: Add client schema and synchronization mappings**

Add four default-empty string columns to `ClientSourceApp`, regenerate Ent, decode them in source input, place them in sync rows and create/update builders, and expose them in client app DTOs.

- [ ] **Step 4: Run client tests**

Run: `go test ./internal/clientserver`

Expected: PASS.

### Task 4: React detail metadata UI

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/modules/sources/SourceAppDetailPage.tsx`
- Modify: `client/src/modules/storefront/AppDrawer.tsx`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`
- Modify: relevant client CSS only if the existing metadata layout cannot accommodate links.
- Test: existing client contract tests.

**Interfaces:**
- Consumes: optional app metadata from server and client APIs.
- Produces: compact non-empty metadata rows, including a safe external homepage link.

- [ ] **Step 1: Extend TypeScript types and localization keys**

Add optional `author`, `homepage`, `license`, and `minOSVersion` properties plus localized labels for author, homepage, license, and minimum OS version.

- [ ] **Step 2: Render metadata in both detail surfaces**

Use the existing metadata list/grid components. Omit blank rows. Normalize homepage with `new URL`, accept only `http:` and `https:`, and render other values as text. Add `target="_blank"` and `rel="noreferrer"` to the link.

- [ ] **Step 3: Run frontend verification**

Run: `npm test -- --runInBand` or the repository's defined client test command, then `npm run build`, from `client/`.

Expected: tests and production build PASS.

### Task 5: Full verification and delivery

**Files:**
- Modify: `README.md` only if its documented parsed-field list remains incomplete.

**Interfaces:**
- Consumes: all previous tasks.
- Produces: a verified, backward-compatible feature commit.

- [ ] **Step 1: Run formatting and generated-code checks**

Run: `gofmt` on changed Go sources and `go generate ./ent` if schema output is stale.

- [ ] **Step 2: Run complete verification**

Run: `go test ./...`

Run the client lint/test/build commands defined in `client/package.json`.

Expected: all commands PASS.

- [ ] **Step 3: Inspect the final diff**

Confirm no generated distribution bundle is changed unless the repository's normal build/release process requires it, no metadata field is omitted from migration, and owner/submitter semantics are unchanged.

- [ ] **Step 4: Commit implementation**

```bash
git add ent internal client docs README.md
git commit -m "feat: preserve LPK application metadata"
```
