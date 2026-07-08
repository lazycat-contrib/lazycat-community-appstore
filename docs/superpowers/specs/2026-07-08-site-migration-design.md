# Site Migration Import Export Design

## Thesis

Site migration should work as a whole-site move from device A to device B, not as a loose collection of settings downloads. The system will export a versioned zip package containing selected database records, optional attachment files, and a manifest that makes imports predictable and auditable.

The first implementation should optimize for reliable restore of this app store instance. It should avoid field-level pickers and instead provide a small set of module-level choices: people, apps, site configuration, and attachment files.

## Goals

- Add site migration import and export to `Admin -> Site Settings`.
- Export and import through a `.zip` package.
- Support module choices:
  - site configuration,
  - people and groups,
  - software catalog data,
  - attachment files.
- Make a complete migration the default choice.
- Preserve user password hashes so users can log in with existing passwords after migration.
- Include API token and MCP token hashes when people are selected so existing tokens continue to work after migration.
- Preserve group codes, app visibility rules, comments, favorites, reviews, announcements, collections, taxonomy, versions, screenshots, and storage metadata when their parent modules are selected.
- Copy local attachment files into the package when attachments are selected.
- Validate an import package before writing data.
- Keep import writes transactional for database records.
- Restrict all migration endpoints to site administrators.
- Keep migration implementation modular: backend import/export logic lives outside existing admin handlers, and frontend import/export UI lives in dedicated components.

## Non-Goals

- Do not add scheduled backups in this feature.
- Do not add cloud backup storage or remote restore.
- Do not encrypt migration packages in the first version.
- Do not export browser sessions or active login state.
- Do not support partial field-level selection inside each module.
- Do not migrate standalone client SQLite data.
- Do not make imported remote S3/WebDAV objects physically move unless they are referenced through local attachment records and the attachment module can open them through the storage backend.

## Package Format

The exported file uses a zip container:

```text
lazycat-appstore-migration-YYYYMMDD-HHMMSS.zip
  manifest.json
  data/site_settings.json
  data/storage_configs.json
  data/users.json
  data/groups.json
  data/catalog.json
  data/social.json
  data/chat.json
  data/announcements.json
  files/<storage-key>/<object-path>
```

`manifest.json` includes:

- format version,
- app server version,
- export timestamp,
- selected modules,
- record counts per entity,
- attachment file count and total bytes,
- per-file path, size, and SHA256,
- warnings for skipped files.

The package format is append-only for compatibility. Future fields are optional and must not break old importers that can ignore unknown fields.

## Export Semantics

### Site Configuration

Includes:

- `site_settings`,
- `storage_configs`,
- announcements.

Sensitive settings are included because the feature is designed for device migration. The UI must clearly warn that exported packages may contain storage keys, SMTP passwords, password hashes, API token hashes, MCP token hashes, and group codes.

### People And Groups

Includes:

- users,
- API tokens,
- MCP tokens,
- groups,
- group members,
- app visibility bindings,
- collaborator records and collaborator requests/invites where app data is also selected,
- registration invite records, including invite code hashes and remaining-use counts.

User password hashes and token hashes are preserved. Raw API tokens and raw MCP tokens are not available and are not exported.

### Software Catalog

Includes:

- categories and tags,
- apps,
- app versions,
- app screenshots,
- app tags,
- collections and collection app links,
- review requests,
- comments and comment notifications,
- favorites,
- outdated marks,
- chat conversations, participants, and messages when both people and software catalog modules are selected.

App owner and uploader references use original user IDs in the package. Import maps them to destination IDs through the user import map.

Registration invites use the mapped creator user. If the original creator is not included in the package or no matching destination user exists, the importer assigns the invite to the current site administrator applying the import.

### Attachment Files

Includes physical files referenced by:

- local app versions,
- app screenshots,
- user avatars,
- site icon uploads.

The exporter opens files through configured storage backends where possible. For external GitHub links, the package stores metadata only and does not download the remote LPK. For unavailable files, export continues and records a manifest warning.

## Import Semantics

The import flow has two phases:

1. Preview: parse `manifest.json`, validate module dependencies, validate zip paths, and show counts and warnings.
2. Apply: write selected modules and files.

### Import Modes

First version supports:

- Merge/update: upsert records by stable keys and keep unrelated destination data.
- Wipe and restore: delete managed app-store records, then import selected package records.

Wipe and restore requires a confirmation dialog with explicit text entry. It must be restricted to site admins and should not be the default.

### Identity Mapping

Stable keys:

- users: username,
- storage configs: key,
- groups: owner username plus group slug,
- categories: slug,
- tags: slug,
- apps: package ID,
- app versions: package ID plus version,
- collections: slug.

The importer builds ID maps after each entity group is inserted or matched. Later records reference the mapped destination IDs.

### Transactions And File Ordering

Database writes run inside a transaction. Attachment files are staged before the transaction commits:

1. Extract selected files to a temporary directory.
2. Validate file paths stay inside the temporary root.
3. Validate SHA256 and size against the manifest.
4. Write database records with the target storage paths.
5. Move staged files into their final storage location.
6. Commit the database transaction.

If the final file move fails, rollback the database transaction and remove staged files. If cleanup fails, record a server log entry without exposing paths in the API response.

## API Design

All endpoints require `SITE_ADMIN`.

### Export

`POST /api/v1/admin/migration/export`

Input:

```json
{
  "includeSite": true,
  "includePeople": true,
  "includeApps": true,
  "includeFiles": true
}
```

Response:

- `200 OK`
- `Content-Type: application/zip`
- `Content-Disposition: attachment; filename="lazycat-appstore-migration-...zip"`

### Preview Import

`POST /api/v1/admin/migration/import/preview`

Input:

- multipart form with `file`.

Response:

```json
{
  "preview": {
    "formatVersion": 1,
    "createdAt": "2026-07-08T00:00:00Z",
    "serverVersion": "0.1.16",
    "modules": ["site", "people", "apps", "files"],
    "counts": {
      "users": 12,
      "apps": 40,
      "versions": 55,
      "files": 80
    },
    "totalFileBytes": 123456,
    "warnings": []
  }
}
```

The preview endpoint stores no persistent state.

### Apply Import

`POST /api/v1/admin/migration/import`

Input:

- multipart form with `file`,
- `mode`: `merge` or `replace`,
- module booleans matching export choices,
- `confirmReplace`: required for replace mode.

Response:

```json
{
  "result": {
    "mode": "merge",
    "created": 10,
    "updated": 20,
    "skipped": 2,
    "warnings": []
  }
}
```

Errors use the existing structured API error shape.

## Module Boundaries

### Backend

Do not put migration business logic into the existing admin handlers. Add a dedicated internal package, for example `internal/migration`, with small focused files:

- `types.go`: package manifest, module options, preview/result DTOs.
- `exporter.go`: database snapshot collection and zip writing orchestration.
- `importer.go`: preview and apply orchestration.
- `records.go`: entity serialization and stable-key mapping helpers.
- `files.go`: attachment collection, SHA256 validation, path validation, and staging.
- `zip.go`: zip reader/writer limits and traversal protection.

Server handlers should stay thin in a separate handler file such as `internal/server/handlers_migration.go`. Handlers only authorize, decode request options, call the migration service, and write responses.

The migration package should depend on the ent client and storage interfaces, not on HTTP request objects. This keeps the core importer/exporter testable without a live HTTP server.

### Frontend

Do not add migration UI directly into `AdminPanel.tsx` beyond the site-settings tab registration and component wiring. Add dedicated components under `client/src/modules/admin/migration/`:

- `AdminMigrationPanel.tsx`: page-level layout and state orchestration.
- `MigrationExportCard.tsx`: module checklist and export action.
- `MigrationImportCard.tsx`: file picker, preview, import mode, and apply action.
- `MigrationPreviewSummary.tsx`: counts, module badges, warnings.
- `MigrationReplaceConfirmDialog.tsx`: destructive restore confirmation.
- `types.ts`: UI DTOs and option types.

Split non-visual logic out of TSX files:

- `api.ts`: export, preview, and import request helpers.
- `useMigrationImport.ts`: import file, preview state, mode state, and apply state.
- `useMigrationExport.ts`: export option state and download action.
- `constants.ts`: default module selections and UI labels when they are not pure i18n keys.

TSX files should render components and handle local event wiring only. They should not contain zip/package parsing rules, request construction details, manifest normalization, or large inline data transforms. As a guideline, keep each new TSX component under roughly 220 lines. If a component grows past that because it owns multiple states or subviews, split it before continuing.

This keeps export and import independently testable and prevents `AdminPanel.tsx` or a single migration TSX file from growing into another catch-all file.

## UI And UX

Migration lives in `Admin -> Site Settings` as a new `Migration` tab beside identity, announcements, registration, policy, storage, and mail.

Use Astryx framework components and existing local wrappers:

- `TabList` and `Tab` for the site settings tab.
- `Card` or existing `panel` sections for export and import areas.
- `CheckboxInput` or framework checkbox components for module selection.
- `FilePicker` for import zip selection.
- `Button` and `IconButton` with lucide icons for export, upload, preview, apply, and close actions.
- `Badge` for selected modules, package validity, warning counts, and import mode.
- `List` and `ListItem` for preview counts and warnings.
- `ModalLayer` plus framework buttons for replace-mode confirmation.

Do not introduce custom controls for module selection, import mode selection, file picking, or confirmation. Use framework components unless a required capability is missing.

### Export Panel

The export panel is dense and task-focused:

- Header: `Export migration package`.
- Body: one sentence describing that packages can move a site to another device.
- Module checklist with four rows:
  - site configuration,
  - people and groups,
  - software catalog,
  - attachment files.
- A compact sensitive-data warning when people or site configuration is selected.
- Primary button: `Export package`.

The default state selects all modules. The export button is disabled if no modules are selected.

### Import Panel

The import panel is staged:

1. Pick package.
2. Preview.
3. Choose import mode.
4. Apply.

The UI should not show a destructive apply button until preview succeeds. The preview result displays:

- package version,
- export time,
- source server version,
- module badges,
- counts,
- attachment size,
- warnings.

Merge/update is the default mode. Wipe and restore is visually secondary until selected, then opens a required confirmation modal before apply.

### Motion And Polish

- Do not animate normal checkbox or form interaction.
- Use existing component transitions where available.
- Any custom transition must stay under 200ms and use opacity or transform only.
- Do not use keyframe animations for state changes.
- Buttons should use framework press feedback; if local styling is needed, add only scoped `:active` transform behavior.
- Dialogs stay centered; anchored popovers should use framework origin behavior.
- The import preview should avoid layout shift by reserving stable space for count rows and warning badges.

## Security

Trust boundary: uploaded zip files are untrusted input.

Required controls:

- Site-admin authorization on all endpoints.
- Maximum upload size.
- Maximum decompressed size.
- Maximum file count.
- Reject absolute paths and `..` traversal paths.
- Reject symlinks and special files.
- Validate JSON shape and module names.
- Validate manifest hashes before file placement.
- Never execute package contents.
- Never write files outside configured storage roots.
- Do not expose internal file paths in API errors.
- Log import summary and warnings server-side.

Sensitive export warning is mandatory because migration packages can contain secrets and credential hashes.

## DDIA Notes

The migration package is a batch snapshot. The database remains the source of truth. Imports must favor deterministic, idempotent operations:

- stable keys define identity,
- unknown future fields are ignored,
- merge mode can be retried,
- replace mode has an explicit destructive confirmation,
- transaction boundaries keep relational data consistent.

This feature is not replication. It does not provide continuous sync or conflict resolution between two active devices.

## Testing

Backend tests:

- export includes selected module files and manifest counts,
- export omits unselected modules,
- import preview rejects malformed zip packages,
- import preview rejects path traversal entries,
- merge import creates missing records,
- merge import updates existing records by stable keys,
- replace import requires explicit confirmation,
- attachment import validates SHA256,
- non-admin users cannot call migration endpoints.

Frontend tests or browser verification:

- migration tab appears only for site admins,
- all export checkboxes selected by default,
- export button disabled when nothing is selected,
- import preview shows counts and warnings,
- apply is unavailable before preview,
- replace mode requires modal confirmation,
- text does not overflow on desktop or mobile widths.

End-to-end verification:

- create a fixture site with users, groups, private apps, versions, screenshots, comments, and local attachments,
- export a full package,
- import into a fresh database,
- verify login with migrated user password,
- verify app list, private visibility, source feed, and file downloads.
