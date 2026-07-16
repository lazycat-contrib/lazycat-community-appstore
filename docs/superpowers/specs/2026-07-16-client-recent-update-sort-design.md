# Client Recent Update Sort Design

## Goal

Add a "Recently updated" sort option to the standalone client catalog.

The server storefront already supports this sort. This change brings the same useful ordering to apps synced from subscribed sources without adding download-based sorting or changing package versions.

## Scope

- Add one standalone client catalog sort option: recently updated.
- Sort apps by the source application's `updatedAt` value in descending order.
- Keep the current default, name, and source ordering unchanged.
- Keep the server storefront sorting behavior unchanged.
- Do not add download counts or download-period statistics to source feeds.
- Do not change the server or client package version.

## API decision

No public server API extension is required.

The existing v2 source feed already includes `updatedAt` for every source application. The standalone client currently ignores that field during synchronization, so the work is limited to consuming data that is already part of the published source contract.

The client's local `/apps` response will expose the cached timestamp as an additive `updatedAt` field. This endpoint is internal to the standalone client application. Existing consumers that ignore unknown fields remain compatible.

## Data flow

1. Schema migration v3 invalidates legacy cached update times and source ETags.
2. The source synchronizer reads each app's existing `updatedAt` value.
3. The client cache stores that value in the existing `client_source_apps.updated_at` column.
4. The local app DTO returns the cached value as `updatedAt`.
5. The React `SourceApp` type accepts the optional timestamp.
6. The client catalog selector adds "Recently updated" and sorts timestamps from newest to oldest.

The existing cache column is appropriate because source apps are replaced as one synchronized snapshot. Before this change, Ent populated it with the synchronization time, so existing client databases contain values with the wrong meaning. Migration v3 resets those legacy values to the Unix epoch and clears every source ETag. This makes the fallback ordering safe immediately and forces the next synchronization to fetch a complete snapshot instead of accepting a `304 Not Modified` response.

## Compatibility and fallback

- A valid RFC 3339 source timestamp is preserved.
- A source that omits or supplies an invalid timestamp is cached with the Unix epoch and sorts after timestamped apps.
- Equal or missing timestamps use the localized app name as a deterministic tie-breaker.
- Existing cached synchronization timestamps are never presented as application update times.
- Clearing legacy ETags ensures the first post-upgrade synchronization repopulates real timestamps.
- Existing source feeds and older server versions continue to synchronize.
- The default catalog order does not change until the user selects "Recently updated".

## Interface behavior

The current sort selector gains one entry using the existing `search.recent` localization. No new visual component, layout, or styling is required.

The option appears beside the existing default, name, and source choices. Selecting it resets pagination to the first page through the current sort-state effect.

## Testing

Backend tests cover:

- source synchronization persists an RFC 3339 application update time;
- the local app response exposes the same timestamp;
- missing or invalid timestamps are cached with the Unix epoch.
- schema migration v3 resets legacy cached times, clears ETags, and records the new schema version.

Frontend tests cover:

- the selector includes the recent option;
- recent ordering is descending;
- missing timestamps sort after valid timestamps;
- equal timestamps use a stable name tie-breaker;
- existing default, name, and source ordering remains unchanged.

Repository verification includes Go tests, frontend tests, the production frontend build, embedded asset parity, Go vet, module tidy, golangci-lint, and the existing CI workflow.

## Delivery

Rebuild and synchronize `clientembed/dist`, then commit and push the design, plan, source changes, tests, and generated assets. Do not build an LPK artifact and do not modify package versions.
