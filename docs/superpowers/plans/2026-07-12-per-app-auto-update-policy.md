# Per-App Automatic Update Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users disable scheduled automatic updates for individual installed source applications while preserving all manual update paths.

**Architecture:** Persist device-local user/package policies in the client SQLite database. Merge policies into installed-app responses, expose an idempotent PATCH endpoint, and pass a policy-filter flag only from the scheduler into the existing update queue. The installed-app UI saves each switch independently and labels manual bulk-update entries without filtering them.

**Tech Stack:** Go 1.26, Ent, SQLite, React 19, TypeScript, Astryx UI, i18next.

## Global Constraints

- Store policies only in the standalone client database (`client.db`).
- Missing policy means automatic updates are enabled.
- Scheduled updates respect disabled policies; manual single and bulk updates ignore them.
- Policies follow normalized `packageId` across source changes and reinstallations.
- Password-protected applications remain excluded from scheduled updates.
- No new dependencies.

---

### Task 1: Add the client-local policy schema

**Files:**
- Create: `ent/schema/client_app_update_policy.go`
- Generate: `ent/clientappupdatepolicy*`, `ent/migrate/schema.go`, `ent/mutation.go`, `ent/runtime.go`
- Test: `internal/clientserver/update_policy_test.go`

**Interfaces:**
- Produces Ent entity `ClientAppUpdatePolicy` with `user_id`, `package_id`, `auto_update_enabled`, `created_at`, and `updated_at`.
- Unique index: `(user_id, package_id)`.

- [ ] **Step 1: Write a failing schema behavior test**

Create a test that inserts one policy for `alice/community.lazycat.app.lark`, rejects a duplicate raw insert, and permits the same package for `bob`.

- [ ] **Step 2: Run the focused test**

Run: `go test ./internal/clientserver -run TestClientAppUpdatePolicyIsUserScoped -count=1`

Expected: FAIL because the generated entity does not exist.

- [ ] **Step 3: Add the schema and generate Ent code**

Use normalized package IDs at the service boundary; keep the schema field as a plain non-empty string. Run:

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

- [ ] **Step 4: Verify the focused test**

Run: `go test ./internal/clientserver -run TestClientAppUpdatePolicyIsUserScoped -count=1`

Expected: PASS.

### Task 2: Add policy service and HTTP contract

**Files:**
- Create: `internal/clientserver/update_policy.go`
- Modify: `internal/clientserver/install.go`, `internal/clientserver/server.go`, `internal/clientserver/types.go`
- Test: `internal/clientserver/update_policy_test.go`, `internal/clientserver/server_test.go`

**Interfaces:**
- `effectiveAutoUpdatePolicies(ctx context.Context, userID string, packageIDs []string) (map[string]bool, error)`.
- `setAutoUpdatePolicy(ctx context.Context, userID, packageID string, enabled bool) (ClientAppUpdatePolicyDTO, error)`.
- `PATCH /api/client/v1/installed-apps/{packageId}/update-policy`.
- Adds `autoUpdateEnabled` to `InstalledApplicationDTO`.

- [ ] **Step 1: Write failing API and merge tests**

Cover default-enabled behavior, explicit disabled behavior, user isolation, idempotent PATCH, invalid JSON, and empty package ID.

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/clientserver -run 'Test(InstalledAppsIncludeAutoUpdatePolicy|UpdatePolicyEndpoint)' -count=1`

Expected: FAIL because the service and route do not exist.

- [ ] **Step 3: Implement policy normalization, query, upsert, and response merge**

Normalize with `strings.ToLower(strings.TrimSpace(packageID))`. Query all requested package IDs in one database call, default missing entries to `true`, and fail `GET /installed` when the policy query fails.

- [ ] **Step 4: Register and validate the PATCH endpoint**

Decode `{ "autoUpdateEnabled": boolean }`, upsert by the unique user/package key, and return the effective DTO using the existing error response shape.

- [ ] **Step 5: Verify focused tests**

Run: `go test ./internal/clientserver -run 'Test(InstalledAppsIncludeAutoUpdatePolicy|UpdatePolicyEndpoint)' -count=1`

Expected: PASS.

### Task 3: Filter only scheduled update candidates

**Files:**
- Modify: `internal/clientserver/update_queue.go`, `internal/clientserver/scheduler.go`, `internal/clientserver/types.go`
- Test: `internal/clientserver/update_queue_test.go`, `internal/clientserver/update_policy_test.go`

**Interfaces:**
- Adds internal `RespectAutoUpdatePolicy bool` to queue options.
- Scheduled calls set it to `true`; manual HTTP calls leave it `false`.

- [ ] **Step 1: Write failing queue tests**

Create one disabled and one enabled installed application. Assert the scheduled queue installs only the enabled application, while manual `RunUpdateQueue` installs both. Retain the protected-app exclusion assertion.

- [ ] **Step 2: Run focused queue tests**

Run: `go test ./internal/clientserver -run 'Test(ScheduledUpdateSkipsDisabledApps|ManualUpdateIncludesDisabledApps)' -count=1`

Expected: FAIL because queue options do not consult policies.

- [ ] **Step 3: Implement policy-aware candidate filtering**

Load disabled package IDs once before candidate selection only when `RespectAutoUpdatePolicy` is true. Keep `eligibleUpdates` pure by passing a disabled-ID set rather than querying inside it.

- [ ] **Step 4: Set scheduler semantics explicitly**

Call `RunUpdateQueueWithOptions` from scheduled updates with `RespectAutoUpdatePolicy: true`. Preserve the manual POST behavior with the zero value `false`.

- [ ] **Step 5: Verify queue tests**

Run: `go test ./internal/clientserver -run 'Test(ScheduledUpdateSkipsDisabledApps|ManualUpdateIncludesDisabledApps|EligibleUpdates)' -count=1`

Expected: PASS.

### Task 4: Add installed-app policy controls

**Files:**
- Modify: `client/src/shared/types.ts`, `client/src/App.tsx`, `client/src/modules/client/InstalledAppsView.tsx`
- Modify: `client/src/modules/client/clientUxState.ts`, `client/src/modules/client/clientUxState.test.mjs`
- Modify: `client/src/locales/zh.ts`, `client/src/locales/en.ts`, `client/src/styles/client.css`

**Interfaces:**
- Adds `autoUpdateEnabled?: boolean` to `InstalledApplication`.
- Adds `onSetAutoUpdatePolicy(packageId: string, enabled: boolean): Promise<void>` to `InstalledAppsView`.

- [ ] **Step 1: Add failing client helper tests**

Add a pure helper that interprets missing `autoUpdateEnabled` as `true` and returns the manual-only label state when explicitly false.

- [ ] **Step 2: Run the helper tests**

Run: `node --test client/src/modules/client/clientUxState.test.mjs`

Expected: FAIL because the helper does not exist.

- [ ] **Step 3: Implement API action and optimistic per-app state**

PATCH the encoded package ID, optimistically update only the matching installed application, track an in-flight package-ID set, and restore the prior array with an error toast on failure.

- [ ] **Step 4: Render the switch and manual-only state**

Show the switch only for source-managed installed applications. Disable it while its request is pending. Add an `Automatic updates disabled` marker in the manual bulk confirmation list without excluding the application.

- [ ] **Step 5: Add Chinese and English copy and focused styling**

Use existing semantic tokens and switch components. Avoid animation beyond existing opacity/color transitions.

- [ ] **Step 6: Verify client behavior**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
cd client && npm run build
```

Expected: tests and production build pass.

### Task 5: Final verification and shipment

- [ ] **Step 1: Run repository gates**

```bash
go test ./... -count=1
go test ./internal/clientserver -race -count=1
go vet ./...
cd client && npm run build
git diff --check
```

- [ ] **Step 2: Review the diff for scope and generated artifacts**

Confirm every changed file traces to the policy feature, generated Ent files are staged, and client embedded assets follow the repository's existing build process.

- [ ] **Step 3: Commit in reviewable units**

Use one backend commit for schema/API/queue filtering and one frontend commit for installed-app controls, unless generated-code coupling makes a single feature commit clearer.
