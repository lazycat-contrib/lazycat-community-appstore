# LPK Application Metadata Design

## Goal

Preserve and expose the application metadata already parsed from an LPK `package.yml`: `author`, `homepage`, `license`, and `min_os_version`.

These values describe the packaged software. They are distinct from the community app-store owner or submitter account.

## Data model

Store the four values on the server `App` entity as optional/default-empty application-level fields:

- `author`
- `homepage`
- `license`
- `min_os_version`

Mirror the same fields on the client `ClientSourceApp` entity. Empty values remain valid so old databases and old source documents stay compatible. Ent schema creation performs additive migrations on startup for SQLite, PostgreSQL, and MySQL.

The metadata is application-level because `package.yml` presents it as package identity and the UI needs one stable value. A later inspected or approved LPK may fill missing values. Explicit overwrite inspection may replace existing values, matching current name and description behavior.

## Server data flow

1. Keep parsing the four fields through `internal/lpkinspect.Metadata`.
2. Apply them when creating an app from an inspected LPK.
3. Apply them when an approved LPK refreshes app metadata.
4. Apply them during background LPK inspection, respecting fill-missing versus overwrite mode.
5. Include them in app DTOs, source v2 JSON, MCP summaries where app metadata is returned, and OpenAPI contracts.
6. Include them in logical migration export/import records so database-engine migrations preserve them.

Manually supplied name, description, and other existing fields retain their current precedence. These four fields currently have no manual submission controls, so inspected LPK values are authoritative when available.

## Client data flow

1. Decode the four optional fields from source v2 documents.
2. Persist them during source synchronization.
3. Return them from client app-list and app-detail APIs.
4. Add them to shared frontend types.
5. Show non-empty values in the source application detail metadata section.

The homepage is rendered as a safe external link. Author, license, and minimum OS version are plain text. Empty metadata rows are omitted to keep the detail screen compact.

## Compatibility and validation

- New JSON fields are optional, preserving compatibility with older servers and clients.
- Homepage is stored as source metadata but only rendered as a clickable link for an `http` or `https` URL.
- No installation blocking is introduced for `min_os_version`; this change is informational only.
- Existing apps can be backfilled using the current LPK inspection workflow.

## Tests

- Parser coverage confirms all four fields are extracted.
- Server create and inspection tests confirm persistence and overwrite behavior.
- Migration round-trip tests confirm the fields survive export/import.
- Source feed tests confirm serialization.
- Client sync tests confirm local persistence and API output.
- Frontend contract/type tests cover the metadata rendering path where existing test infrastructure supports it.
