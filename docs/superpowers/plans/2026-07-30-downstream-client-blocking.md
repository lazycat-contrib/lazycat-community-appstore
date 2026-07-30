# Downstream Client Blocking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let site administrators block the downstream client user IDs observed in comments and wishes, preventing those users from syncing the source or using client interaction APIs.

**Architecture:** Maintain a first-class downstream-client record keyed by the existing source-scoped `lc_...` client user ID. Upsert observations when a client comments or creates a wish, expose site-admin block management APIs, send the same ID during source synchronization, and enforce a shared block guard at every trusted-client entry point.

**Tech Stack:** Go 1.26, net/http, ent, source feed/client proxy headers, React 19, TypeScript 5.9.

## Global Constraints

- The block key is the exact client user ID already stored on comments and wishes; display name and device ID are not authorization keys.
- Only `SITE_ADMIN` can block or unblock; `SOFTWARE_ADMIN` can see downstream identity/status in administrative views but cannot mutate it.
- Historical comments, wishes, replies and status events remain intact.
- Requests with no client user ID remain compatible for old feed clients; they cannot be individually blocked.
- New clients must send the same source-scoped ID during source synchronization as during comments/wishes.
- A blocked client receives stable HTTP 403 code `CLIENT_BLOCKED`.
- No new dependency.

---

## File Map

- `ent/schema/downstream_client_user.go`: observed client identity, provenance and block audit fields.
- `ent/`: generated entity files.
- `internal/server/downstream_clients.go`: observe/query/block helpers and admin handlers.
- `internal/server/downstream_clients_test.go`: permissions, audit, idempotency and source-block tests.
- `internal/server/server.go`: management routes.
- `internal/server/handlers_social.go`: observe comment actors and guard comments/outdated marks.
- `internal/server/handlers_wishes.go`: observe and guard wish actors.
- `internal/server/handlers_chat.go`: guard client chat.
- `internal/server/handlers_source.go`: reject blocked identified feed clients.
- `internal/clientserver/sync.go`: send client identity headers and classify `CLIENT_BLOCKED`.
- `internal/clientserver/comments.go`, `internal/clientserver/chat.go`: shared proxy header helper/error mapping.
- `client/src/shared/types.ts`: downstream client DTO and source error type.
- `client/src/modules/admin/DownstreamClientsPanel.tsx`: site-admin list/block UI.
- `client/src/modules/admin/AdminUsersWorkspace.tsx`: downstream users tab.
- `client/src/components/CommentList.tsx`: administrative block action for client-authored comments.
- `client/src/modules/wishwall/WishWall.tsx`: administrative block action for client-authored wishes.
- `client/src/locales/en.ts`, `client/src/locales/zh.ts`: copy.
- `docs/openapi.yaml`: management API.

### Task 1: Persist downstream identities and block audit state

**Files:**
- Create: `ent/schema/downstream_client_user.go`
- Regenerate: `ent/`
- Create: `internal/server/downstream_clients.go`
- Test: `internal/server/downstream_clients_test.go`

**Interfaces:**
- Produces: `observeDownstreamClient(ctx, clientUserID, displayName, source string) error`.
- Produces: `clientUserBlocked(ctx, clientUserID string) (bool, error)`.

- [ ] **Step 1: Write failing observation/idempotency tests**

Observe the same ID from comment then wish and assert one row, stable `FirstSeenAt`, newer `LastSeenAt`, updated display name and provenance containing both sources:

```go
if err := store.server.observeDownstreamClient(ctx, "lc_abc", "Alice", "COMMENT"); err != nil {
	t.Fatal(err)
}
store.server.setNow(func() time.Time { return later })
if err := store.server.observeDownstreamClient(ctx, "lc_abc", "Alice L.", "WISH"); err != nil {
	t.Fatal(err)
}
records := store.server.db.DownstreamClientUser.Query().AllX(ctx)
if len(records) != 1 || records[0].DisplayName != "Alice L." || !records[0].SeenInComments || !records[0].SeenInWishes {
	t.Fatalf("observation was not merged: %#v", records)
}
```
- [ ] **Step 2: Run and confirm missing entity/helper**

```bash
go test ./internal/server -run TestDownstreamClientObservation -count=1
```

Expected: compile failure.

- [ ] **Step 3: Add schema and regenerate**

Fields:

```go
client_user_id string unique; display_name string;
seen_in_comments bool; seen_in_wishes bool;
blocked bool; block_reason text; blocked_by int optional/nillable; blocked_at time optional/nillable;
first_seen_at time; last_seen_at time; created_at time; updated_at time
```

Indexes: unique client ID, `(blocked,last_seen_at)`, `(last_seen_at)`.

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema
```

- [ ] **Step 4: Implement observe and lookup helpers**

Sanitize ID/display name with existing helpers. Reject empty/non-`lc_` IDs from persistence. Use create-or-update behavior that never clears an existing provenance flag or block state.

- [ ] **Step 5: Run focused tests**

```bash
go test ./internal/server -run TestDownstreamClient -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit persistence**

```bash
git add ent internal/server/downstream_clients.go internal/server/downstream_clients_test.go
git commit -m "feat: track downstream client identities"
```

### Task 2: Add site-admin block management APIs

**Files:**
- Modify: `internal/server/downstream_clients.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/downstream_clients_test.go`

**Interfaces:**
- Produces: `GET /api/v1/admin/downstream-clients` for software/site admins.
- Produces: `POST /api/v1/admin/downstream-clients/{id}/block` and `/unblock` for site admins.

- [ ] **Step 1: Write failing permission and audit tests**

Assert anonymous/ordinary users cannot list, software admins can list but get 403 on mutations, and site admins can block with required reason:

```go
rec := store.asSiteAdmin(http.MethodPost, "/api/v1/admin/downstream-clients/lc_abc/block", map[string]string{"reason": "Repeated spam"})
if rec.Code != http.StatusOK {
	t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
}
record := store.server.db.DownstreamClientUser.Query().OnlyX(ctx)
if !record.Blocked || record.BlockReason != "Repeated spam" || record.BlockedBy == nil || record.BlockedAt == nil {
	t.Fatalf("block audit missing: %#v", record)
}
```

Unblock must clear reason/by/at and preserve observations.

- [ ] **Step 2: Run tests and confirm routes are absent**

```bash
go test ./internal/server -run 'TestDownstreamClient(List|Block|Unblock)' -count=1
```

Expected: 404/compile failure.

- [ ] **Step 3: Implement paginated management handlers**

List supports `blocked=true|false` and case-insensitive ID/display-name search, ordered by `last_seen_at DESC`. Block requires a 1–500 rune reason. Block/unblock routes use `s.withRole(userRoleSiteAdmin)`; list uses both admin roles.

- [ ] **Step 4: Run management tests**

```bash
go test ./internal/server -run TestDownstreamClient -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit management API**

```bash
git add internal/server/downstream_clients.go internal/server/downstream_clients_test.go internal/server/server.go
git commit -m "feat: manage blocked downstream clients"
```

### Task 3: Enforce blocks on source feed and client interactions

**Files:**
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/server/handlers_social.go`
- Modify: `internal/server/handlers_wishes.go`
- Modify: `internal/server/handlers_chat.go`
- Modify: `internal/server/downstream_clients.go`
- Test: `internal/server/downstream_clients_test.go`
- Test: `internal/server/handlers_wishes_test.go`

**Interfaces:**
- Produces: `rejectBlockedClient(w, r, clientUserID) bool` writing `403 CLIENT_BLOCKED`.

- [ ] **Step 1: Write a blocked-entry-point table test**

Block `lc_blocked`, then exercise feed, wish create/list-own, comment create/delete, outdated mark and chat. Each must return:

```go
if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"code":"CLIENT_BLOCKED"`) {
	t.Fatalf("blocked request status=%d body=%s", rec.Code, rec.Body.String())
}
```

Also assert anonymous v2 feed and an identified non-blocked client still return 200.

- [ ] **Step 2: Run the table and confirm blocked requests still pass**

```bash
go test ./internal/server -run TestBlockedClientEntryPoints -count=1
```

Expected: FAIL because no shared guard is applied.

- [ ] **Step 3: Apply the shared guard after identity validation**

For trusted client requests, sanitize the ID, check the database, and write:

```go
writeError(w, http.StatusForbidden, "CLIENT_BLOCKED", "This client user is blocked", nil)
```

The source feed checks only when both `X-LazyCat-Client-Proxy: lazycat-appstore-client` and a non-empty `X-LazyCat-Client-User-ID` are present; absence preserves old-client compatibility. Interaction handlers continue to require device ID as before.

- [ ] **Step 4: Observe identities on successful comment/wish creation**

Call `observeDownstreamClient` before persistence. Treat observation database failure as a server error so an interaction cannot be accepted without becoming administratively blockable.

- [ ] **Step 5: Run server tests**

```bash
go test ./internal/server -run 'Test(BlockedClient|DownstreamClient|Wish|Comment|Outdated|Chat)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit enforcement**

```bash
git add internal/server
git commit -m "feat: enforce downstream client blocks"
```

### Task 4: Send stable identity during source sync and surface block errors

**Files:**
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/chat.go`
- Modify: `internal/clientserver/comments.go`
- Modify: `internal/clientserver/types.go`
- Test: `internal/clientserver/server_test.go`

**Interfaces:**
- Consumes: server `403 CLIENT_BLOCKED`.
- Produces: sync headers `X-LazyCat-Client-Proxy` and `X-LazyCat-Client-User-ID`.
- Produces local sync error code `blocked` and message suitable for `SourceStatusRow`.

- [ ] **Step 1: Write failing sync header/error tests**

Capture a source sync request and assert the exact pseudonymous ID. Return `403` JSON with `CLIENT_BLOCKED` and assert source state records `last_error_code=blocked` rather than `http` or `auth`.

- [ ] **Step 2: Run and confirm failure**

```bash
go test ./internal/clientserver -run 'Test.*(SyncHeaders|ClientBlocked)' -count=1
```

Expected: FAIL because sync currently sends no client identity and normalizes 403 as a generic HTTP error.

- [ ] **Step 3: Reuse one proxy-header helper**

Split identity header setup from optional request/device/group headers:

```go
func applySourceIdentityHeaders(req *http.Request, sourceURL, userID, displayName, deviceID string) {
	req.Header.Set("X-LazyCat-Client-User-ID", pseudonymousClientUserID(sourceURL, userID))
	req.Header.Set("X-LazyCat-Client-Display-Name", displayName)
	req.Header.Set("X-LazyCat-Client-Device-ID", strings.TrimSpace(deviceID))
	req.Header.Set("X-LazyCat-Client-Proxy", "lazycat-appstore-client")
}
```

Source sync passes source URL/user ID and an empty device ID; comment/chat/wish proxies pass the request device ID.

- [ ] **Step 4: Parse stable upstream error codes**

For non-2xx feed responses, decode the bounded error body. Map `CLIENT_BLOCKED` to `sourceSyncError{code:"blocked", status:403, message:"This client user is blocked by the source"}`; preserve existing password handling.

- [ ] **Step 5: Run clientserver tests**

```bash
go test ./internal/clientserver -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit sync identity**

```bash
git add internal/clientserver
git commit -m "feat: identify clients during source sync"
```

### Task 5: Add downstream-user and inline block controls to the UI

**Files:**
- Create: `client/src/modules/admin/DownstreamClientsPanel.tsx`
- Modify: `client/src/modules/admin/AdminUsersWorkspace.tsx`
- Modify: `client/src/components/CommentList.tsx`
- Modify: `client/src/modules/storefront/AppDrawer.tsx`
- Modify: `client/src/modules/wishwall/WishWall.tsx`
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/styles/admin.css`
- Test: `client/src/modules/admin/adminState.test.mjs`

**Interfaces:**
- Consumes admin downstream-client endpoints.
- Consumes privacy-aware `clientUserId` and `clientBlocked` fields only for admin viewers.

- [ ] **Step 1: Add failing UI contract tests**

Assert the Users workspace includes a downstream tab, block action is gated by `SITE_ADMIN`, and comment/wish actions are rendered only when a client ID exists.

- [ ] **Step 2: Run and confirm failure**

```bash
cd client && node --test --experimental-strip-types src/modules/admin/adminState.test.mjs
```

Expected: FAIL for missing panel/tab/actions.

- [ ] **Step 3: Implement the admin list**

Add search, blocked filter, pagination, provenance badges, last-seen time, block reason modal, and unblock action. Software admins see read-only state; site admins see mutations.

- [ ] **Step 4: Add inline comment and wish actions**

Thread `viewerRole` and `onManageClient` into administrative comment/wish views. Do not display the raw client ID to anonymous or ordinary viewers. After block/unblock, refresh the affected record and list.

- [ ] **Step 5: Display explicit source-block state**

Map client source `lastErrorCode === 'blocked'` to localized “已被该软件源封禁 / Blocked by this source” in `SourceStatusRow` and sync toasts.

- [ ] **Step 6: Run frontend tests and build**

```bash
cd client && node --test --experimental-strip-types 'src/**/*.test.mjs'
cd client && npm run build
```

Expected: PASS.

- [ ] **Step 7: Commit UI controls**

```bash
git add client/src/modules/admin client/src/components/CommentList.tsx client/src/modules/storefront/AppDrawer.tsx client/src/modules/wishwall client/src/shared/types.ts client/src/locales client/src/styles/admin.css client/src/modules/client/SourceStatusRow.tsx
git commit -m "feat: manage blocked downstream clients"
```

### Task 6: Document blocking APIs and compatibility

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `docs/openapi_test.go`
- Modify: `README.md`

**Interfaces:**
- Documents downstream client list/block/unblock routes and `CLIENT_BLOCKED` on feed/client interaction endpoints.

- [ ] **Step 1: Add failing OpenAPI assertions**

Assert paths for downstream client list/block/unblock and a shared `ClientBlockedError` response exist.

- [ ] **Step 2: Run and confirm failure**

```bash
go test ./docs -count=1
```

Expected: FAIL.

- [ ] **Step 3: Update OpenAPI and README**

Document role requirements, required block reason, audit fields, no historical deletion, and old-client compatibility when feed identity headers are absent.

- [ ] **Step 4: Run plan-level verification**

```bash
go test ./internal/server/... ./internal/clientserver/... ./docs
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
cd client && npm run build
```

Expected: PASS.

- [ ] **Step 5: Commit documentation**

```bash
git add docs/openapi.yaml docs/openapi_test.go README.md
git commit -m "docs: document downstream client blocking"
```
