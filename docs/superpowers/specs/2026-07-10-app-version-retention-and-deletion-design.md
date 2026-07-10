# App Version Retention and Deletion Design

## Status

Approved in conversation on 2026-07-10. Written specification pending final user review.

## Objective

Give server-side application maintainers direct control over release history without weakening the existing site-wide retention policy.

The feature must:

- allow deleting any application version, including the current latest version;
- allow an application owner or administrator to override the site retention count;
- inherit the site retention count when no application override exists;
- apply an application override immediately and prune excess approved versions;
- avoid a destructive all-site scan when the site default changes;
- distinguish externally hosted packages from packages managed by configured storage;
- record the installed version as an immutable event snapshot while keeping analytics application-scoped rather than version-scoped.

## Decisions

### Data model

Add a nullable integer field to `App`:

```text
version_retention_count NULL  -> inherit the current site max_versions setting
version_retention_count 0     -> keep unlimited approved versions for this app
version_retention_count > 0   -> keep exactly this many newest approved versions
```

The application value takes priority over the site value. Inheritance is represented by `NULL`; the current site number is not copied into the application row. This ensures inherited applications follow later site-setting changes.

Replace `AppDownload.version_id` with a denormalized version string snapshot. Download events contain the application ID, the version text installed at that moment, and creation time. The string is historical event data, not a relationship to the current `AppVersion` row.

Existing total and day/week/month/year statistics continue to aggregate only by application ID and creation time. The version snapshot is not a filter, grouping, counter, or ranking dimension.

Old migration archives may still contain `version_id` in download records. Import resolves that ID to the archived version text when possible. If the referenced version is absent, the event is retained with an empty version string. Newly exported archives contain the version string and omit `version_id`.

### API boundaries

Use dedicated sub-resources instead of extending the existing application metadata update endpoint. Retention policy changes are operational settings and must not enter application-info review or resubmission workflows.

#### Read policy

`GET /api/v1/apps/{id}` adds an optional management-oriented object:

```json
{
  "versionRetention": {
    "mode": "INHERIT",
    "siteMaxVersions": 10,
    "appMaxVersions": null,
    "effectiveMaxVersions": 10
  }
}
```

`mode` is `INHERIT` or `CUSTOM`. A custom value of `0` means unlimited.

#### Update policy

`PATCH /api/v1/apps/{id}/version-retention`

Inheritance request:

```json
{ "mode": "INHERIT" }
```

Application override request:

```json
{ "mode": "CUSTOM", "maxVersions": 5 }
```

Validation rules:

- `mode` is required and accepts only `INHERIT` or `CUSTOM`;
- `CUSTOM` requires a non-negative integer `maxVersions`;
- `INHERIT` rejects a supplied `maxVersions` to keep the contract unambiguous.

The response returns the effective policy and the versions pruned by this update. Saving an application policy immediately removes excess approved versions, oldest first.

#### Delete version

`DELETE /api/v1/apps/{id}/versions/{versionId}`

Any matching version may be deleted, including the latest approved version. If another approved version remains, the existing latest-version query automatically promotes the newest remaining approved version. If none remains, the application stays present and approved but is temporarily not installable until another version is published.

The version ID must belong to the application in the path. Cross-application IDs return not found.

### Authorization

Version deletion follows release publishing permissions:

- application owner;
- approved application collaborator;
- software administrator;
- site administrator.

Application retention policy changes are limited to long-term application governors:

- application owner;
- software administrator;
- site administrator.

Collaborators can publish and delete releases but cannot change the persistent retention policy.

### Deletion transaction and storage cleanup

Database state is authoritative. Both manual deletion and automatic retention use one shared deletion service.

For each version:

1. In a database transaction, verify scope and authorization where applicable.
2. Preserve download events unchanged; their denormalized version strings are historical snapshots and do not reference the version row.
3. Preserve review history while clearing its optional `version_id` reference.
4. Delete the application-version row and commit.
5. If `storage_path` is empty, stop. The package is an online/external artifact and the server must not delete the remote URL.
6. If `storage_path` is non-empty, use the recorded storage key and an independent short cleanup context to attempt deletion from local, WebDAV, or S3 storage.
7. If object deletion fails, log a structured warning containing the app ID, former version ID, storage key, path, and error. Do not restore the database row and do not return a failure that encourages the destructive request to be repeated.

The distinction is based on `storage_path`, not `source_type`: an uploaded package can live in any configured storage backend, while an external URL has no server-managed storage path.

### Retention behavior

Retention considers approved versions only. Ordering is deterministic:

1. `published_at` descending;
2. `created_at` descending;
3. ID descending.

The newest effective-count versions are kept and all remaining approved versions are deleted through the shared deletion service. Pending and rejected versions are not automatically pruned, but an authorized release maintainer may delete them manually.

Retention is enforced:

- immediately after saving an application-level policy;
- after a version is published directly;
- after a pending version is approved.

Changing the site default does not trigger an immediate all-application destructive scan. Inherited applications use the new site value on their next version publication. Administrators may immediately handle a specific application by saving its policy or deleting versions manually.

### Management interface

The server storefront application drawer owns this workflow because it already contains release publishing and version history.

Above version history, show a compact retention summary:

- `继承站点设置（当前 10）` or `应用自定义（当前 5）`;
- current approved-version count;
- effective retention count, with `不限` for zero;
- a settings action visible only to the owner and administrators.

The policy dialog offers two explicit choices:

- inherit the current site setting;
- use a custom non-negative count.

Before submission, calculate and display the expected number of approved versions that will be removed. When the number is greater than zero, the primary action states the consequence, for example `保存并删除 3 个旧版本`. The server response remains authoritative if another publication changes the count concurrently.

Each visible version row has its existing download/install action and, for release maintainers, a separate destructive delete action. Deletion always uses a confirmation dialog containing the application and version names.

For the current latest version, the dialog additionally states either:

- which remaining approved version will become latest; or
- that the application will temporarily have no installable version and may be republished later.

For a historical version, the dialog states that the version will no longer be available for new downloads while existing installations remain unaffected.

While a delete or policy request is running, Escape/backdrop dismissal and repeated submission are blocked. On completion, refresh the application detail in place and show persistent success or warning feedback. Storage cleanup failure is represented as a warning, not a restored version.

### Error semantics

Use the existing structured API error envelope.

- `400 BAD_REQUEST`: malformed JSON or invalid path IDs;
- `403 FORBIDDEN`: authenticated user lacks the required application permission;
- `404 APP_NOT_FOUND` or `VERSION_NOT_FOUND`: application/version missing or version belongs to another application;
- `422 VALIDATION_ERROR`: invalid retention mode or count;
- `500 VERSION_DELETE_FAILED` / `VERSION_RETENTION_UPDATE_FAILED`: database transaction failed before commit.

Storage cleanup failure after commit does not become a 500 response. The successful response includes a cleanup warning so the interface can explain that the version is gone but the stored object could not be removed.

## Compatibility and migration

- The nullable application field is an additive relational schema change and defaults existing applications to site inheritance.
- Migration export/import includes the optional application override.
- Old archives without the override import as inherited.
- Existing databases backfill the new download-version string from the old `version_id` where the referenced version still exists; unresolved events remain valid with an empty version string.
- Old archives with download `version_id` import successfully by resolving the archived version text when possible.
- New download exports include the version string and omit `version_id`.
- OpenAPI documents both new endpoints, the retention object, request unions, response shape, and cleanup warning.
- Public source feeds continue deriving their latest and version lists from remaining approved rows; no source-feed schema change is required.
- Installed clients are not modified when a server release row is deleted.

## Testing strategy

### Backend

- inherited policy uses the current site value;
- custom value takes priority, including custom zero/unlimited;
- application policy update immediately prunes the expected oldest approved rows;
- site setting update does not scan/prune all inherited applications;
- direct publication and review approval enforce the effective policy;
- latest-version deletion promotes the next approved version;
- deleting the only approved version leaves the application un-installable but intact;
- online artifacts delete database state only;
- managed artifacts attempt local/WebDAV/S3 cleanup after commit;
- cleanup failure does not restore the row and returns a warning;
- pending/rejected manual deletion and review-reference clearing;
- collaborator deletion allowed but policy update forbidden;
- owner and both administrator roles can update policy;
- cross-application version ID rejected;
- application download events retain a version string snapshot but have no version-row association;
- download aggregation and ranking queries do not group, filter, or count by version;
- old/new migration archive compatibility;
- race tests cover concurrent publication, retention, and manual deletion without data races or duplicate destructive effects.

### Frontend

- retention inheritance/custom/unlimited labels;
- expected prune count and destructive confirmation copy;
- latest-version fallback/no-version consequences;
- separate download and delete actions without event propagation;
- busy, success, warning, and error states;
- keyboard focus, Escape behavior, 320/375px wrapping, and reduced motion;
- TypeScript and storefront contract tests.

### Release verification

- full Go unit and race suites;
- `go vet`, `golangci-lint`, and `go mod tidy -diff`;
- frontend contract tests and TypeScript build;
- final server/client frontend build through the LazyCat packaging workflow;
- LPK metadata inspection confirms the intended package IDs and bumped versions.

## Alternatives rejected

### Extend the application metadata PATCH

Rejected because application metadata can enter review and resubmission workflows. Retention is an operational policy and must apply directly without creating an application-info review request.

### Store overrides as dynamic site-setting keys

Rejected because keys such as `app.<id>.max_versions` lack referential integrity, complicate application deletion, and make migration validation weaker.

## Acceptance criteria

- Any version, including latest, can be deleted by a release maintainer.
- Deleting latest deterministically exposes the next approved version or leaves the application temporarily without an installable version.
- Application retention inherits the live site setting unless a nullable application override is set.
- Application policy changes prune immediately; site setting changes do not mass-delete.
- External packages only lose database records; managed packages receive best-effort post-commit storage deletion without rollback.
- Download events retain the installed version string but analytics contain no version-row dependency and do not use version as a statistical dimension.
- Permissions, migration compatibility, OpenAPI, frontend UX, and automated tests match this specification.
