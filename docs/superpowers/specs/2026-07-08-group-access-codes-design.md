# Group Access Codes Design

## Thesis

Groups are site-level access controls, not personal workspace content. Move group management into the admin user area, then extend groups with short access codes so standalone clients can subscribe to private group-scoped app feeds without needing a full user login.

The code path must treat group codes as credentials. The client can paste a convenient base64 configuration, but the local source record stores only decoded structured fields. Every server response and package download must re-check that the submitted group codes are still valid.

## Current Problem

Groups currently live under the profile workspace. That makes group management feel like a personal app-owner tool even though groups affect site-wide software visibility.

The standalone client can only consume public `/source/v1/index.json` content. Apps restricted to groups are visible in the server UI for eligible users, but they do not flow to client installations. There is no portable, revocable way to let a standalone client subscribe to selected group-scoped apps.

## Goals

- Move group administration from the profile workspace to `Admin -> Users`.
- Split admin user management into `Users` and `Groups` tabs.
- Add a six-character alphanumeric group code to each group.
- Allow admins to reset a group's code.
- Allow admins to select multiple groups and generate a pasteable client configuration.
- Let the standalone client accept a normal source URL, a single group code, or a base64 group configuration in the add-source flow.
- Store decoded source configuration only: source URL plus normalized group code records.
- Include group names in synced client source state so the client UI can show meaningful names.
- Automatically remove expired or invalid group codes from the client source record.
- Prevent deletion of any group currently attached to app visibility; such groups can only rotate their code.
- Enforce group-code validity on source feed listing and package download/install paths.
- Hide group-bound apps from public pages and public package counts.

## Non-Goals

- Do not add a full client login flow.
- Do not make group codes a replacement for server user accounts.
- Do not store the original base64 string after client import.
- Do not expose private apps in the public source feed.
- Do not build a new design system. Use existing Astryx components and existing local wrappers.

## Data Model

### Server Group Code

Extend `UserGroup` with:

- `code`: uppercase six-character alphanumeric string.
- `code_updated_at`: timestamp.

The code character set is `A-Z` and `0-9`, excluding ambiguous characters only if implementation already has a local helper for this. The code is generated server-side. Clients and admins cannot choose arbitrary codes.

`code` is unique. Resetting a code replaces the current value immediately. Old codes are invalid from that moment.

### Client Source Group Codes

Extend the standalone client source model with structured group-code storage:

- `group_codes_json`: JSON array of current codes for the source.
- `group_names_json`: JSON array of resolved group summaries returned by the last successful sync.
- `last_invalid_group_codes_json`: JSON array of codes removed during the last sync.

The stored source record never keeps the pasted base64 configuration. Imports are normalized before save.

## Admin UI/UX

### Navigation

Move group management out of the profile workspace. `Admin -> Users` becomes a tabbed workspace:

- `Users`: existing user management.
- `Groups`: group list and group-code tooling.

Use framework tab/segmented controls where available. Avoid custom tab CSS unless the framework component cannot meet the layout.

### Groups Tab Layout

The groups tab is list-first:

- Header row: title, `New group`, `Generate client config`.
- Summary row: total groups, private app bindings, active codes.
- Group rows: name, description, members count, attached apps count, current code badge, updated timestamp, actions.

Actions:

- `Manage members`: opens or expands the member manager.
- `Copy code`: copies the six-character code.
- `Reset code`: confirmation dialog, then server-generated replacement.
- `Delete`: enabled only when no apps reference the group.
- `Generate config`: available globally for selected groups.

Use Astryx buttons, icon buttons, badges, checkbox inputs, dialogs, and selectors. Keep custom CSS scoped to layout only.

### Delete Rule

If a group has app visibility rows, disable delete and show a tooltip or inline helper: "This group is attached to apps. Remove those visibility rules or reset the group code instead."

The backend also enforces this rule. UI disablement is not a security boundary.

### Config Generation

Admins can select multiple groups and generate a client config:

```json
{
  "sourceUrl": "https://store.example.com/source/v1/index.json",
  "groupCodes": ["A7K2Q9", "M4D8XZ"]
}
```

The UI displays a base64 encoded value and provides one copy button. It also shows the decoded preview: source URL and selected group names. The generated config is a convenience wrapper only; the client stores decoded fields.

## Client UI/UX

### Add Source Flow

The client add-source form has one primary input: `Source URL or group config`.

Detection order:

1. Try to base64-decode and parse JSON.
2. If decoded JSON has `sourceUrl` and `groupCodes`, save those decoded fields.
3. If the input is a URL, save a public source with no group codes.
4. If the input is one six-character group code, require or infer a source URL.
5. Otherwise show a validation error.

For a single group code, infer the default source URL only when the runtime config has one. If no default exists, ask for a source URL in the same form.

### Stored State

After save, the source record contains:

- source name
- source URL
- password or source auth fields already supported today
- normalized group codes
- resolved group names from sync

It does not contain the pasted base64 string.

### Source Card

The source card shows:

- source URL
- sync status
- group names as badges or compact chips
- invalid-code cleanup status when applicable

Do not show group codes as primary UI text. They are credentials. Showing a short masked form such as `A7****` is acceptable in an advanced/details area if needed for troubleshooting.

### Automatic Cleanup

When sync returns invalid group codes:

- Remove those codes from the local source record.
- Store the removed codes in `last_invalid_group_codes_json` for the latest sync message.
- Show a toast such as "2 expired group codes were removed."
- Keep the source itself.
- If no group codes remain, downgrade the source to public-only sync.

This cleanup is automatic. The user should not have to manually edit expired codes.

## Visibility Rules

An app with one or more group visibility bindings is treated as group-bound.

Group-bound apps:

- Do not appear on public storefront pages.
- Do not appear in public search results.
- Do not count toward public app/package totals, category counts, or any other public-facing package count.
- Do not appear in the public-only source feed.

Group-bound apps are visible only to:

- site admins,
- the app creator/owner,
- app administrators or collaborators with management permission,
- users or clients presenting a currently valid group code for one of the bound groups.

The public count rule is important: hidden group-bound apps must not leak through aggregate numbers even when their names and details are hidden.

## Server API Design

### Admin Groups

Add or extend endpoints:

- `GET /api/v1/groups`: include code metadata and attached app count for admins.
- `POST /api/v1/groups`: create group with generated code.
- `PATCH /api/v1/groups/{id}`: update group name/description.
- `POST /api/v1/groups/{id}/code:rotate`: rotate group code.
- `DELETE /api/v1/groups/{id}`: delete only when no app visibility rows reference the group.
- `POST /api/v1/groups/client-config`: generate encoded config for selected group IDs.

All group mutation endpoints require auth. Non-admin users can only manage groups they own where existing rules allow it, but admin UI uses the admin path.

### Source Feed

Extend the source feed request to accept group codes:

- Query parameter: `groupCodes=A7K2Q9,M4D8XZ`.
- Header: `X-Group-Codes: A7K2Q9,M4D8XZ`.

The server normalizes codes to uppercase, removes duplicates, validates current codes, and returns:

```json
{
  "apps": [],
  "groups": [
    { "name": "Design Team", "code": "A7K2Q9" }
  ],
  "invalidGroupCodes": ["OLD123"]
}
```

The public feed remains public-only when no valid group code is provided.

### Download Authorization

Package download URLs for private group apps must require valid current group code context. The same group-code validation used for feed listing must be enforced before serving or redirecting an LPK download.

If a code is missing or invalid, return `403` with a structured error. The client treats this as a signal to resync and clean invalid codes.

## Security Model

Group codes are bearer credentials:

- Generate them server-side only.
- Keep them short because the user explicitly requested six characters, but do not expose brute-force-friendly validation endpoints.
- Validate codes only as part of feed and download flows.
- Return generic invalid-code errors in public contexts.
- Never trust client-side detection for access control.
- Do not log submitted group codes in server logs.

Resetting a group code invalidates all existing client configurations for that group. Clients clean up the old code on the next sync or protected download failure.

## Component And Module Boundaries

Use existing framework components and split implementation modules:

- `client/src/modules/admin/AdminUsersWorkspace.tsx`: tab container.
- `client/src/modules/admin/AdminUsersPanel.tsx`: existing user management.
- `client/src/modules/admin/AdminGroupsPanel.tsx`: group list, selection, and row actions.
- `client/src/modules/admin/GroupMemberManager.tsx`: member add/remove surface.
- `client/src/modules/admin/GroupCodeManager.tsx`: copy/reset code and generated config dialog.
- `client/src/modules/client/sourceConfig.ts`: client-side import detection, base64 decode, normalization, and validation helpers.
- `internal/server/handlers_groups.go`: group CRUD, code rotation, config generation.
- `internal/server/handlers_source.go`: feed group-code filtering.
- `internal/server/handlers_download.go` or existing download handler: protected download code validation.
- `internal/clientserver`: persist decoded group-code source configuration and cleanup invalid codes.

Do not keep adding unrelated logic to `ProfileView` or a single large groups component.

## Testing

Server tests:

- Creating a group generates a unique six-character code.
- Rotating a group code invalidates the old code.
- A private group app appears in the source feed only when a valid code is supplied.
- Invalid codes are returned in `invalidGroupCodes`.
- Protected package download rejects missing or invalid group codes.
- Group-bound apps are excluded from public storefront lists and public app counts.
- Site admins, app owners, and app management collaborators can still see group-bound apps in management surfaces.
- A group attached to app visibility cannot be deleted.
- A group without app visibility can be deleted.

Clientserver tests:

- Base64 config imports are decoded and stored as structured fields.
- The raw base64 input is not persisted.
- Invalid group codes returned by sync are removed from the source.
- When all group codes are removed, the source remains as public-only.

Frontend checks:

- Admin user page shows `Users` and `Groups` tabs.
- Group list actions use framework components and do not overflow at mobile widths.
- Client add-source form accepts URL, code, and base64 config.
- Source cards show group names and cleanup messages without exposing codes as primary content.

## Resolved Decisions

The default source URL for single-code imports uses runtime config when present. If no default source URL exists, the client asks for a source URL before saving.

Group code length is fixed at six uppercase alphanumeric characters because that is an explicit product requirement.
