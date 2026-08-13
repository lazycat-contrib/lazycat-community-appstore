# Force Ads Display Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a server-controlled, default-off policy that forces subscribed clients to display source ads regardless of their local ad preference.

**Architecture:** Store `force_ads_display` as a site setting and publish it as `site.clientPolicy.forceAdsDisplay` in source v2. The standalone client persists the policy on each subscribed source, exposes it through its API, and makes the display decision from `forceAdsDisplay || adsPreference === 'enabled'`. The local preference remains unchanged so disabling the server policy restores the user's previous choice.

**Tech Stack:** Go 1.26, Ent, React 19, TypeScript 5.9, i18next, Astryx Design components.

## Global Constraints

- The server setting defaults to `false`.
- Source v1 behavior remains unchanged.
- Old clients must remain compatible by ignoring the additive JSON field.
- A forced policy must suppress the client's first-run ad preference prompt and disable its local ad switch.
- Turning the forced policy off must restore the prior local preference.

---

### Task 1: Publish the server policy

**Files:**
- Modify: `internal/server/settings.go`
- Modify: `internal/server/types.go`
- Modify: `internal/server/handlers_admin.go`
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/feed/feed.go`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Produces: JSON setting `force_ads_display` and source-v2 field `site.clientPolicy.forceAdsDisplay: boolean`.

- [x] **Step 1: Write the failing server test**

Add a test that reads `/api/v1/admin/settings`, asserts `force_ads_display` is `false`, enables it through `PATCH /api/v1/admin/settings`, and asserts `/source/v2/index.json` contains `site.clientPolicy.forceAdsDisplay: true`.

- [x] **Step 2: Run the focused test and verify it fails**

Run: `go test ./internal/server -run TestAdminSettingPublishesForceAdsDisplayPolicy -count=1`

- [x] **Step 3: Implement the setting and feed field**

Add `settingForceAdsDisplay = "force_ads_display"`, include it in the public settings map and boolean validation, resolve it with a `false` fallback, add `ForceAdsDisplay bool` to both policy DTOs, and copy it into source v2.

- [x] **Step 4: Run the focused server test**

Run: `go test ./internal/server -run TestAdminSettingPublishesForceAdsDisplayPolicy -count=1`

### Task 2: Persist the policy in the standalone client service

**Files:**
- Modify: `ent/schema/client_source.go`
- Regenerate: `ent/clientsource*`, `ent/migrate/schema.go`, and related Ent generated files
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/source_client_policy.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/sources.go`
- Test: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: `site.clientPolicy.forceAdsDisplay` from source v2.
- Produces: `source.clientPolicy.forceAdsDisplay` from the standalone client API.

- [x] **Step 1: Write the failing sync test**

Extend a source sync fixture with `clientPolicy.forceAdsDisplay: true`, sync it, then assert both the Ent row and returned source DTO retain the flag.

- [x] **Step 2: Run the focused test and verify it fails**

Run: `go test ./internal/clientserver -run TestClientSourceSyncPersistsForceAdsDisplayPolicy -count=1`

- [x] **Step 3: Implement and regenerate the persistence layer**

Add the Ent boolean field with default `false`, regenerate Ent, include the field in policy normalization, save it during sync, and expose it from `sourceDTO`.

- [x] **Step 4: Run the focused client-service test**

Run: `go test ./internal/clientserver -run TestClientSourceSyncPersistsForceAdsDisplayPolicy -count=1`

### Task 3: Enforce and administer the policy in React

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/client/SourcesView.tsx`
- Modify: `client/src/modules/admin/AdminAdsPanel.tsx`
- Modify: `client/src/modules/admin/AdminPanel.tsx`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

**Interfaces:**
- Consumes: `ClientPolicy.forceAdsDisplay?: boolean` and admin setting `force_ads_display`.
- Produces: effective visibility `forceAdsDisplay || adsPreference === 'enabled'`.

- [x] **Step 1: Add the shared TypeScript field**

Extend `ClientPolicy` with `forceAdsDisplay?: boolean`.

- [x] **Step 2: Apply the effective display decision**

Include ads from forced sources, exclude forced sources from the pending-preference modal, disable the local source ad switch when forced, and explain why it is disabled.

- [x] **Step 3: Add the admin switch**

Render the existing Astryx switch above the ad list, bind it to `settings.force_ads_display`, and save it using the existing settings API flow.

- [x] **Step 4: Add Chinese and English copy**

Add labels and descriptions that make the server override and default-off behavior explicit.

- [x] **Step 5: Build the client**

Run: `npm run build` in `client/`.

### Task 4: Full verification and embedded bundle

**Files:**
- Regenerate: `clientembed/dist/**`

**Interfaces:**
- Consumes: completed Go and React changes.
- Produces: a server binary embedding the updated client.

- [x] **Step 1: Run Go formatting and tests**

Run: `gofmt` on changed Go files, then `go test ./...`.

- [x] **Step 2: Refresh the embedded client bundle**

Replace `clientembed/dist` with the successful `client/dist` output using the repository's documented build flow.

- [x] **Step 3: Verify generated bundle parity**

Run: `diff -qr --exclude=app-config.js client/dist clientembed/dist` and expect no output.

- [x] **Step 4: Review the final diff**

Confirm every changed line implements the policy, persistence, enforcement, admin control, localization, tests, or generated artifacts required above.
