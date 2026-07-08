# Group Access Codes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move group administration into admin user settings and add revocable group access codes that let standalone clients sync and install group-bound apps.

**Architecture:** Extend existing relational schemas and REST handlers instead of adding a parallel permissions system. The server remains the authority for group-code generation, validation, source-feed filtering, and download authorization; the standalone client stores only decoded structured source configuration and cleans invalid codes after sync failures or invalid-code responses.

**Tech Stack:** Go 1.x, ent, SQLite, net/http, React, TypeScript, Astryx components, Vite.

## Global Constraints

- Group code length is exactly six uppercase alphanumeric characters.
- Group codes are bearer credentials and must be generated server-side only.
- The client must not persist pasted base64 configuration strings.
- Group-bound apps must not appear in public storefront pages, public search, public package counts, or public-only source feeds.
- Group-bound apps are visible to site admins, app creator/owner, app management collaborators, and clients/users with a currently valid bound group code.
- Use existing Astryx components and local wrappers; do not build a new design system.
- Split admin and client UI modules by responsibility; do not keep expanding `ProfileView` or a single large groups component.
- Keep existing source password and install password behavior intact.
- After group access codes are complete, add PlayCaptcha to admin login after three failed password attempts.
- PlayCaptcha must use a random toy by leaving `target` unset.

---

## File Map

- Modify `ent/schema/user_group.go`: add generated code fields.
- Modify `ent/schema/client_source.go`: persist decoded group-code source configuration.
- Run `go generate ./ent`: update generated ent files.
- Modify `internal/server/handlers_groups.go`: group code generation, code rotation, config generation, delete guard, richer group DTOs.
- Modify `internal/server/server.go`: register code rotation and config endpoints.
- Modify `internal/server/handlers_source.go`: parse and validate group codes, include matching group-bound apps, return resolved groups and invalid codes.
- Modify `internal/server/handlers_apps.go`: require valid group code for unauthenticated group-bound downloads.
- Modify `internal/server/app_list_preload.go` and list/count handlers as needed: keep group-bound apps out of public listings and counts.
- Modify `internal/server/server_test.go`: server regression coverage.
- Modify `internal/clientserver/types.go`: source DTO/input fields for `groupCodes`, `groups`, `lastInvalidGroupCodes`.
- Modify `internal/clientserver/sources.go`: create/update/list decoded group-code fields.
- Modify `internal/clientserver/sync.go`: send group codes, parse source groups and invalid codes, clean invalid codes transactionally.
- Modify `internal/clientserver/install.go`: pass source group codes on cached-app install downloads.
- Modify `internal/clientserver/server_test.go`: client import/sync cleanup coverage.
- Create `client/src/modules/client/sourceConfig.ts`: client-side base64/code/URL detection and normalization.
- Modify `client/src/shared/types.ts`: group metadata and source configuration types.
- Create `client/src/modules/admin/AdminUsersWorkspace.tsx`: tab shell for users and groups.
- Create `client/src/modules/admin/AdminUsersPanel.tsx`: extracted existing user management surface.
- Create `client/src/modules/admin/AdminGroupsPanel.tsx`: group list and selection UI.
- Create `client/src/modules/admin/GroupMemberManager.tsx`: member management UI.
- Create `client/src/modules/admin/GroupCodeManager.tsx`: copy/rotate/generate config UI.
- Modify `client/src/modules/admin/AdminPanel.tsx`: replace inline users tab with `AdminUsersWorkspace`.
- Modify `client/src/modules/client/SourcesView.tsx`: add config input flow and group badges.
- Modify `client/src/App.tsx`: remove profile groups tab use and thread group data through admin/user surfaces.
- Modify `client/src/locales/en.ts` and `client/src/locales/zh.ts`: UI copy.
- Modify `client/src/styles.css`: scoped layout styles only.
- Install `playcaptcha` in `client/package.json` after the group-code feature is complete.
- Copy PlayCaptcha assets into the static frontend assets served by the app.
- Modify `client/src/modules/auth/LoginPage.tsx`: show random PlayCaptcha after three failed admin login attempts.
- Modify `internal/server/handlers_auth.go`: expose failed-login metadata and add backend-side admin login throttling.

## Task 1: Server Group Code Model And Admin API

**Files:**
- Modify: `ent/schema/user_group.go`
- Modify: `internal/server/handlers_groups.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Produces `GroupDTO` with `id`, `ownerId`, `name`, `slug`, `description`, `code`, `codeUpdatedAt`, `memberCount`, `attachedAppCount`.
- Produces `POST /api/v1/groups/{id}/code:rotate`.
- Produces `POST /api/v1/groups/client-config` accepting `{ "sourceUrl": string, "groupIds": number[] }`.

- [ ] **Step 1: Add failing tests**

Add tests to `internal/server/server_test.go`:

```go
func TestGroupCreateRotateAndClientConfig(t *testing.T) {
	app := newTestApp(t)
	app.login("admin", "changeme")

	rec := app.do(http.MethodPost, "/api/v1/groups", map[string]any{"name": "Design Team"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create group status = %d body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Group struct {
			ID   int    `json:"id"`
			Code string `json:"code"`
		} `json:"group"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	if len(created.Group.Code) != 6 || strings.ToUpper(created.Group.Code) != created.Group.Code {
		t.Fatalf("generated code = %q", created.Group.Code)
	}

	rec = app.do(http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/code:rotate", created.Group.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate group status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), created.Group.Code) {
		t.Fatalf("rotated response still contains old code: %s", rec.Body.String())
	}

	rec = app.do(http.MethodPost, "/api/v1/groups/client-config", map[string]any{
		"sourceUrl": "https://store.example.com/source/v1/index.json",
		"groupIds":  []int{created.Group.ID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("client config status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"encoded"`) || !strings.Contains(rec.Body.String(), `"sourceUrl":"https://store.example.com/source/v1/index.json"`) {
		t.Fatalf("client config response missing fields: %s", rec.Body.String())
	}
}

func TestDeleteGroupWithVisibilityIsRejected(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	app.login("admin", "changeme")
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(admin.ID).SetName("Bound").SetSlug("bound").SetCode("ABC123").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.bound-delete").
		SetName("Bound Delete").
		SetSlug("bound-delete").
		SetSummary("Private").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(record.ID).SetGroupID(group.ID).SaveX(ctx)

	rec := app.do(http.MethodDelete, fmt.Sprintf("/api/v1/groups/%d", group.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("delete bound group status = %d body = %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/server -run 'Test(GroupCreateRotateAndClientConfig|DeleteGroupWithVisibilityIsRejected)'`

Expected: compile/test failure because `code` field and new endpoints are not implemented.

- [ ] **Step 3: Implement model and handlers**

Add `code` and `code_updated_at` fields to `UserGroup`, create helper functions:

```go
const groupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateGroupCode() string {
	var b [6]byte
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(groupCodeAlphabet))))
		if err != nil {
			panic(err)
		}
		b[i] = groupCodeAlphabet[n.Int64()]
	}
	return string(b[:])
}
```

Use a retry loop on create/rotate to handle uniqueness collisions. Register:

```go
s.mux.HandleFunc("POST /api/v1/groups/{id}/code:rotate", s.withAuth(s.handleRotateGroupCode))
s.mux.HandleFunc("POST /api/v1/groups/client-config", s.withAuth(s.handleGroupClientConfig))
```

Update delete to return `409 GROUP_IN_USE` when `AppVisibility` rows exist for the group.

- [ ] **Step 4: Generate ent code**

Run: `go generate ./ent`

Expected: generated ent files updated successfully.

- [ ] **Step 5: Verify task tests pass**

Run: `go test ./internal/server -run 'Test(GroupCreateRotateAndClientConfig|DeleteGroupWithVisibilityIsRejected)'`

Expected: PASS.

## Task 2: Source Feed Group-Code Filtering And Download Authorization

**Files:**
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/server/handlers_apps.go`
- Modify: `internal/server/app_list_preload.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Consumes `UserGroup.code`.
- Produces source-feed response fields `groups` and `invalidGroupCodes`.
- Download endpoint accepts `groupCodes` query or `X-Group-Codes` header.

- [ ] **Step 1: Add failing tests**

Add tests:

```go
func TestSourceFeedIncludesGroupAppsOnlyWithValidCode(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(admin.ID).SetName("Private Group").SetSlug("private-group").SetCode("ABC123").SaveX(ctx)
	privateApp := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.private-code").
		SetName("Private Code").
		SetSlug("private-code").
		SetSummary("Private").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(privateApp.ID).SetGroupID(group.ID).SaveX(ctx)
	app.server.db.AppVersion.Create().
		SetAppID(privateApp.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/private/releases/download/v1/app.lpk").
		SaveX(ctx)

	rec := app.do(http.MethodGet, "/source/v1/index.json", nil)
	if strings.Contains(rec.Body.String(), "Private Code") {
		t.Fatalf("public feed leaked group app: %s", rec.Body.String())
	}
	rec = app.do(http.MethodGet, "/source/v1/index.json?groupCodes=ABC123,OLD999", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("group feed status = %d body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"Private Code"`, `"groups"`, `"name":"Private Group"`, `"invalidGroupCodes":["OLD999"]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("group feed missing %q: %s", want, body)
		}
	}
}

func TestGroupBoundDownloadRequiresValidGroupCode(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	admin := app.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	group := app.server.db.UserGroup.Create().SetOwnerID(admin.ID).SetName("Private Download").SetSlug("private-download").SetCode("XYZ789").SaveX(ctx)
	record := app.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.private-download").
		SetName("Private Download").
		SetSlug("private-download").
		SetSummary("Private").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	version := app.server.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(admin.ID).
		SetVersion("1.0.0").
		SetStatus(appversion.StatusAPPROVED).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL("https://github.com/acme/private/releases/download/v1/app.lpk").
		SaveX(ctx)
	app.server.db.AppVisibility.Create().SetAppID(record.ID).SetGroupID(group.ID).SaveX(ctx)

	path := fmt.Sprintf("/api/v1/apps/%d/versions/%d/download", record.ID, version.ID)
	if rec := app.do(http.MethodGet, path, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("download without group code = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec := app.do(http.MethodGet, path+"?groupCodes=XYZ789", nil); rec.Code != http.StatusFound {
		t.Fatalf("download with group code = %d body = %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/server -run 'Test(SourceFeedIncludesGroupAppsOnlyWithValidCode|GroupBoundDownloadRequiresValidGroupCode)'`

Expected: FAIL because group-code feed/download logic does not exist.

- [ ] **Step 3: Implement group-code parsing and validation**

Add helpers in `handlers_source.go` or a new focused server file:

```go
type groupCodeAccess struct {
	validGroupIDs      []int
	validGroups        []map[string]string
	invalidGroupCodes  []string
	validCodeByGroupID map[int]string
}
```

Normalize codes from query/header, look up `UserGroup` by code, and return invalid normalized codes.

- [ ] **Step 4: Include matching private apps in source feed**

In `sourceIndexPreload`, keep public apps plus apps bound to `validGroupIDs`. Return `groups` and `invalidGroupCodes` in the source index response.

- [ ] **Step 5: Enforce download authorization**

In `handleDownloadVersion`, if an app has visibility rows and the request is unauthenticated, require a valid group code bound to that app before redirecting. Authenticated site admins, app owners, and management collaborators continue through `userCanSeeApp`.

- [ ] **Step 6: Verify task tests pass**

Run: `go test ./internal/server -run 'Test(SourceFeedIncludesGroupAppsOnlyWithValidCode|GroupBoundDownloadRequiresValidGroupCode|PrivateAppVisibilityUsesGroupsAndSourceFeedStaysPublic)'`

Expected: PASS.

## Task 3: Standalone Client Source Persistence And Sync Cleanup

**Files:**
- Modify: `ent/schema/client_source.go`
- Modify: `internal/clientserver/types.go`
- Modify: `internal/clientserver/sources.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/install.go`
- Modify: `internal/clientserver/server_test.go`

**Interfaces:**
- Source input/DTO include `groupCodes`, `groups`, `lastInvalidGroupCodes`.
- Sync sends `groupCodes` to source feed.
- Sync removes returned `invalidGroupCodes` before saving source state.

- [ ] **Step 1: Add failing clientserver tests**

Add tests:

```go
func TestClientSourceStoresDecodedGroupCodes(t *testing.T) {
	app := testServer(t)
	rec := app.request("POST", "/api/client/v1/sources", `{"name":"Private","url":"https://store.example/source/v1/index.json","groupCodes":["abc123","ABC123","old999"]}`, "alice")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create source = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"groupCodes":["ABC123","OLD999"]`) {
		t.Fatalf("group codes not normalized/deduped: %s", rec.Body.String())
	}
}

func TestSyncRemovesInvalidGroupCodesAndKeepsSource(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Group-Codes"); got != "ABC123,OLD999" {
			t.Fatalf("group code header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"groups": []map[string]string{{"name": "Private", "code": "ABC123"}},
			"invalidGroupCodes": []string{"OLD999"},
			"apps": []map[string]any{},
		})
	}))
	defer feed.Close()

	app := testServer(t)
	create := app.request("POST", "/api/client/v1/sources", `{"name":"Feed","url":"`+feed.URL+`","groupCodes":["ABC123","OLD999"]}`, "alice")
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	rec := app.request("POST", "/api/client/v1/sources/1/sync", ``, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "OLD999") && strings.Contains(body, `"groupCodes"`) {
		t.Fatalf("invalid code still present: %s", body)
	}
	if !strings.Contains(body, `"groups":[{"name":"Private"`) || !strings.Contains(body, `"lastInvalidGroupCodes":["OLD999"]`) {
		t.Fatalf("group metadata/cleanup missing: %s", body)
	}
}
```

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/clientserver -run 'Test(ClientSourceStoresDecodedGroupCodes|SyncRemovesInvalidGroupCodesAndKeepsSource)'`

Expected: FAIL because schema/API fields are missing.

- [ ] **Step 3: Implement client source schema and DTOs**

Add fields:

```go
field.Text("group_codes_json").Default(""),
field.Text("group_names_json").Default(""),
field.Text("last_invalid_group_codes_json").Default(""),
```

Normalize codes with the same six-character uppercase rule. Add JSON encode/decode helpers for group codes and group summaries.

- [ ] **Step 4: Update sync request and cleanup**

When `source.GroupCodesJSON` is non-empty:

```go
req.Header.Set("X-Group-Codes", strings.Join(groupCodes, ","))
```

Parse feed response fields `groups` and `invalidGroupCodes`, remove invalid codes transactionally in `saveSourceApps`, and keep the source when all codes are removed.

- [ ] **Step 5: Pass group codes during install**

When installing a cached source app, append `groupCodes` to the package download URL or send the header if the package manager path supports headers. If only URL is supported, append a query parameter so server-side download authorization can validate the code.

- [ ] **Step 6: Generate ent code**

Run: `go generate ./ent`

Expected: generated code updated.

- [ ] **Step 7: Verify task tests pass**

Run: `go test ./internal/clientserver -run 'Test(ClientSourceStoresDecodedGroupCodes|SyncRemovesInvalidGroupCodesAndKeepsSource|SyncSourceCachesAppsAndUpdatesSource)'`

Expected: PASS.

## Task 4: Client Import Parser And Source UI

**Files:**
- Create: `client/src/modules/client/sourceConfig.ts`
- Modify: `client/src/modules/client/SourcesView.tsx`
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/App.tsx`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/styles.css`

**Interfaces:**
- Produces `parseSourceConfigInput(raw: string, defaultSourceUrl: string): ParsedSourceConfig`.
- Produces `SourceInput.groupCodes`.

- [ ] **Step 1: Create parser tests or typecheckable helper**

Create `sourceConfig.ts` with exported pure functions:

```ts
export type ParsedSourceConfig =
  | { kind: 'url'; url: string; groupCodes: string[] }
  | { kind: 'group-code'; url: string; groupCodes: string[] }
  | { kind: 'config'; url: string; groupCodes: string[] };

export function normalizeGroupCode(value: string): string {
  const code = value.trim().toUpperCase();
  return /^[A-Z0-9]{6}$/.test(code) ? code : '';
}
```

- [ ] **Step 2: Implement base64/config detection**

Decode with browser `atob`, parse JSON, and store decoded URL/codes only. Deduplicate codes. Do not return the raw input in the parsed object.

- [ ] **Step 3: Wire add-source form**

In `SourcesView.tsx`, replace separate URL-first mental model with `Source URL or group config`, while keeping advanced fields for name, password, and mirrors. Submit sends `groupCodes` on `SourceInput`.

- [ ] **Step 4: Display group state**

Render group names as badges/chips in source cards. Render cleanup notice when `lastInvalidGroupCodes.length > 0`. Do not show full group codes as primary text.

- [ ] **Step 5: Verify frontend build**

Run: `npm --prefix client run build`

Expected: PASS.

## Task 5: Admin Users/Groups UI Split

**Files:**
- Create: `client/src/modules/admin/AdminUsersWorkspace.tsx`
- Create: `client/src/modules/admin/AdminUsersPanel.tsx`
- Create: `client/src/modules/admin/AdminGroupsPanel.tsx`
- Create: `client/src/modules/admin/GroupMemberManager.tsx`
- Create: `client/src/modules/admin/GroupCodeManager.tsx`
- Modify: `client/src/modules/admin/AdminPanel.tsx`
- Modify: `client/src/modules/profile/ProfileView.tsx`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/styles.css`

**Interfaces:**
- `AdminUsersWorkspace` owns user/group subtabs.
- `AdminGroupsPanel` calls `/api/v1/groups`, `/api/v1/groups/{id}/members/{userId}`, `/api/v1/groups/{id}/code:rotate`, and `/api/v1/groups/client-config`.

- [ ] **Step 1: Extract user panel**

Move existing user list/create/edit/invite UI from `AdminPanel.tsx` into `AdminUsersPanel.tsx` without behavior changes.

- [ ] **Step 2: Add groups panel**

Use Astryx `TabList`, `Button`, `IconButton`, `Badge`, `CheckboxInput`, `TextInput`, and existing modal wrappers. Group rows show name, description, member count, attached app count, current code badge, and actions.

- [ ] **Step 3: Add code manager**

`GroupCodeManager` supports copy code, rotate code with confirmation, and generate selected group config. Generated config dialog shows encoded value and decoded preview.

- [ ] **Step 4: Remove profile groups tab**

Remove the groups tab from `ProfileView` and profile workspace navigation. Keep any shared type imports needed by other panels.

- [ ] **Step 5: Verify frontend build**

Run: `npm --prefix client run build`

Expected: PASS.

## Task 6: Group Access-Code End-To-End Verification

**Files:**
- Modify: `clientembed/dist/*`
- Confirm: `lazycat/client/package.yml`
- Confirm: `lazycat/server/package.yml`

**Interfaces:**
- Produces verified server/client group access-code behavior before the post-feature login captcha task.

- [ ] **Step 1: Run full automated checks**

Run:

```bash
npm --prefix client run build
go test ./...
git diff --check
```

Expected: all exit 0.

- [ ] **Step 2: Browser QA with agent-browser**

Use desktop and mobile sessions:

```bash
agent-browser --session lazycat-admin open http://127.0.0.1:18083
agent-browser --session lazycat-standalone-client open http://127.0.0.1:18084
```

Check:

- Admin -> Users -> Groups tab renders and actions fit.
- Generate config copies a base64 payload.
- Client add source accepts generated config and shows group names after sync.
- Rotating a group code makes the old client code clean itself on next sync.
- Public source page excludes group-bound apps and public counts do not include them.

## Task 7: Admin Login PlayCaptcha After Three Failures

**Files:**
- Modify: `client/package.json`
- Modify: `client/package-lock.json`
- Copy: `client/public/toys/*`
- Copy: `client/public/playcaptcha.svg`
- Modify: `client/src/modules/auth/LoginPage.tsx`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/styles.css`
- Modify: `internal/server/handlers_auth.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Consumes PlayCaptcha's `ClawCaptcha` component.
- Produces login UI that shows a random captcha only after three failed admin-login password attempts.
- Produces backend failed-login metadata so the UI can reliably know when captcha should be displayed.

- [ ] **Step 1: Install dependency and copy assets**

Run:

```bash
npm --prefix client install playcaptcha
```

Then copy the package assets into `client/public` so Vite serves `/toys/*` and `/playcaptcha.svg`.

- [ ] **Step 2: Add backend tests for admin failure metadata**

Add tests to `internal/server/server_test.go` proving repeated failed admin logins return structured metadata after the third failure:

```go
func TestAdminLoginRequiresCaptchaAfterThreeFailures(t *testing.T) {
	app := newTestApp(t)
	for i := 1; i <= 3; i++ {
		rec := app.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong-password"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d body = %s", i, rec.Code, rec.Body.String())
		}
		if i < 3 && strings.Contains(rec.Body.String(), `"captchaRequired":true`) {
			t.Fatalf("captcha required too early on attempt %d: %s", i, rec.Body.String())
		}
		if i == 3 && !strings.Contains(rec.Body.String(), `"captchaRequired":true`) {
			t.Fatalf("captcha missing after third failure: %s", rec.Body.String())
		}
	}
}
```

- [ ] **Step 3: Implement backend failed-login metadata**

Add per-username/IP in-memory failure tracking for admin accounts. On invalid credentials for an existing admin user, increment the counter. Return structured error details with `failedAttempts` and `captchaRequired` after the third failure. Clear the counter after successful login. Keep error messages generic.

- [ ] **Step 4: Add PlayCaptcha to login UI**

In `LoginPage.tsx`, import:

```ts
import { ClawCaptcha } from 'playcaptcha';
import 'playcaptcha/clawcaptcha.css';
```

Track `captchaRequired` and `captchaVerified`. Render `<ClawCaptcha onVerify={() => setCaptchaVerified(true)} />` after three failed admin-login attempts. Leave `target` unset so every mount uses a random toy.

Do not animate frequent login form actions. The captcha panel can enter with an opacity/translate transition under 250ms and must respect reduced motion.

- [ ] **Step 5: Verify task**

Run:

```bash
npm --prefix client run build
go test ./internal/server -run TestAdminLoginRequiresCaptchaAfterThreeFailures
```

Expected: both pass.

## Task 8: Final Verification, Embed Sync, Versioned Commit, Push

**Files:**
- Modify: `clientembed/dist/*`
- Confirm: `lazycat/client/package.yml`
- Confirm: `lazycat/server/package.yml`

**Interfaces:**
- Produces verified group-code and admin-login captcha behavior with updated embedded frontend assets.

- [ ] **Step 1: Run full automated checks**

Run:

```bash
npm --prefix client run build
go test ./...
git diff --check
```

Expected: all exit 0.

- [ ] **Step 2: Sync embedded frontend**

Copy `client/dist/.` into `clientembed/dist/`, then remove stale embedded asset files not present in `client/dist/assets`.

- [ ] **Step 3: Browser QA with agent-browser**

Use desktop and mobile sessions:

```bash
agent-browser --session lazycat-admin open http://127.0.0.1:18083
agent-browser --session lazycat-standalone-client open http://127.0.0.1:18084
```

Check:

- Admin -> Users -> Groups tab renders and actions fit.
- Generate config copies a base64 payload.
- Client add source accepts generated config and shows group names after sync.
- Rotating a group code makes the old client code clean itself on next sync.
- Public source page excludes group-bound apps and public counts do not include them.
- Admin login shows PlayCaptcha after three failed admin-password attempts.
- PlayCaptcha uses random toy selection because no `target` prop is provided.

- [ ] **Step 4: Commit and push**

Use focused commits where practical:

```bash
git status --short
git add <implemented files>
git commit -m "feat: add group access codes"
git push origin main
```

Expected: push succeeds.

## Self-Review

- Spec coverage: plan covers admin relocation, group codes, config generation, client import normalization, invalid-code cleanup, public visibility/count rules, download authorization, and module splitting.
- Red-flag scan: no incomplete markers or undefined future work remains.
- Type consistency: planned server fields use `code`, `codeUpdatedAt`, `groupCodes`, `groups`, and `invalidGroupCodes`; planned client fields mirror those names.
