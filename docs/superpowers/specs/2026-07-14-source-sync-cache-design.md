# Software Source Synchronization Cache Design

## Goal

Make software-source synchronization fast and predictable for both unchanged and changed feeds while preserving compatibility with older clients.

The target behavior is:

- an unchanged source completes with one conditional HTTP request and no feed parsing, database replacement, or icon downloads;
- a changed source rebuilds its client materialized view without unbounded sequential icon waits;
- the server builds each feed snapshot once per content generation and reuses it across requests;
- source changes invalidate the server cache immediately and warm the public v1/v2 snapshots;
- old clients continue to receive gzip or uncompressed JSON and never receive Brotli unless they advertise support;
- the relational application database remains authoritative, Badger remains a disposable derived-data store, and the client database remains a rebuildable materialized view.

No LPK build or release artifact is part of this work. The implementation will update the affected client and server versions, run automated verification, commit the source changes, and push them; the user will perform the product build.

## Current bottlenecks

The current client downloads and parses the complete source on every synchronization. It then materializes same-origin icons serially, with a ten-second timeout per icon. With 101 applications, unreachable icons alone can consume about seventeen minutes for one source, and repeated sources or retries can extend that to hours.

After icon processing, the client deletes every cached source application, bulk-inserts every row again, relinks asset records, and removes unreferenced assets. The full replacement is not the primary problem at the current feed size, but it is unnecessary when the feed has not changed.

The current server queries all approved applications and their visibility, users, categories, tags, screenshots, versions, outdated marks, settings, announcements, and ads for every source request. It then rebuilds and encodes the complete v1 or v2 response. It does not provide a validator or conditional `304 Not Modified` response.

Brotli reduces transferred bytes, but it cannot fix the serial icon timeout or repeated client-side materialization by itself.

## Data model and consistency

The design follows a record-system/derived-view split:

- The existing relational database is the only source of truth.
- Badger stores server-generated feed snapshots. Every Badger value can be deleted and rebuilt from the relational database.
- The client database stores the last successfully materialized source view and its HTTP validator.
- Client icon assets are content-addressed local derivatives associated with their original feed URL.

Source updates use read-after-write cache semantics within a running server process. After a feed-affecting mutation succeeds, the current cache generation is invalidated synchronously before the handler reports success. A single maintenance coordinator coalesces cleanup and public v1/v2 warming so mutation bursts cannot create one cleanup and two warm tasks per write. A request for the new generation either uses the completed warm result or joins the single in-flight rebuild.

Badger snapshots are not trusted across process boots. Each server start clears prior source-feed namespaces before creating a new boot namespace, so an interrupted database mutation/cache-invalidation sequence cannot expose an old snapshot after restart or accumulate one full cache budget per restart. This deliberately gives up warm-cache reuse across restarts in exchange for making the relational database unquestionably authoritative.

This design assumes one active server process, which matches the current deployment model. Badger directories must not be shared by multiple processes. A future horizontally scaled deployment will require a database-backed revision, CDC, or another cross-instance invalidation channel before enabling per-instance feed caches.

## Server snapshot cache

### Storage and lifecycle

Use `github.com/dgraph-io/badger/v4` v4.9.4. The default cache directory is `./data/source-cache`, configurable through `SOURCE_CACHE_PATH`. Tests use Badger's in-memory mode.

The server opens the cache during startup and closes it during normal shutdown. Failure to open or write Badger does not make the source endpoint unavailable: the server logs the failure, disables cache reads for the affected generation, and builds responses directly from the relational database. A feed build failure retains the existing `SOURCE_BUILD_FAILED` behavior.

The cache component exposes a small internal interface:

- `GetOrBuild(ctx, protocolVersion, accessScope)` returns one immutable snapshot and collapses concurrent misses with `singleflight`;
- `InvalidateAndWarm(ctx)` advances the in-memory generation immediately and schedules public v1/v2 rebuilds;
- `Close()` stops maintenance and closes Badger.

Persistent admission is bounded to 256 snapshots and 256 MiB per generation, and at most four distinct feed builds run concurrently. Once the persistent budget is full, additional scopes are still served but are not written to Badger. These limits bound arbitrary valid group-code combinations and invalid-code cache-bypass requests without changing the feed protocol.

The component owns Badger details; source handlers only authenticate, resolve access scope, request a snapshot, and negotiate the HTTP representation.

### Keys and values

Keys are namespaced by boot ID, generation, protocol version, and access-scope hash. The password is never part of a key or value.

Conceptually:

```text
source-feed/<boot>/<generation>/<v1|v2>/<scope-hash>/meta
source-feed/<boot>/<generation>/<v1|v2>/<scope-hash>/identity
source-feed/<boot>/<generation>/<v1|v2>/<scope-hash>/br
source-feed/<boot>/<generation>/<v1|v2>/<scope-hash>/gzip
```

The metadata contains the weak ETag, uncompressed size, representation sizes, and build time. The ETag is derived from SHA-256 of the uncompressed JSON and is shared by the semantically equivalent encodings as a weak validator.

Badger snapshot entries receive a maximum 24-hour TTL. When an enabled announcement or ad has a future `startsAt` or `endsAt` boundary, the snapshot expires at the earliest boundary instead, so scheduled content cannot be hidden or retained by a day-long cache entry. Obsolete generation prefixes are removed by the coalesced background maintenance worker. Cache cleanup must never block an HTTP response or a business-data mutation.

### Access-scope isolation

Source-password validation happens before cache lookup. The password is neither persisted nor logged.

Group codes are normalized and resolved before lookup. The access-scope key hashes sorted valid group IDs plus the resolved group metadata that affects the response; raw secret codes are never stored. Valid group metadata is sorted by group ID before encoding so a scope has one deterministic body. If the generation changes while a scoped feed is building, the handler re-resolves the group codes instead of retrying with stale group names or rotated codes. Group-code rotation, group metadata changes, and visibility changes invalidate the feed generation.

Responses that contain invalid group codes bypass persistent snapshot caching because the invalid-code list is request-specific and attacker-controlled. Requests accept at most 64 normalized group codes; larger requests receive a structured `400` response before database lookup. Public requests and normal valid-group combinations remain cacheable.

This separation prevents one group's private applications from being returned to another group or to the public source.

### Invalidation coverage

Every successful mutation that can change a feed calls the shared invalidation helper. Coverage includes:

- application create, update, delete, metadata/icon/category/comment setting, approval status, and bulk operations;
- application version create, update, delete, approval status, retention effects, and migration imports;
- application visibility, user groups, group names/codes, and group deletion;
- categories, tags, application-tag links, screenshots, and outdated-mark counts;
- submitter display-name changes;
- site public profile, source URL, client policy, global comments, mirrors, source v1 policy, announcements, and ads.

Invalidation is intentionally centralized at successful domain-mutation boundaries. Tests maintain an explicit mutation matrix so new feed-affecting write paths cannot silently omit it.

## HTTP protocol and compression

The source endpoints return these headers on successful `200` and `304` responses:

```text
ETag: W/"<sha256-of-identity-json>"
Cache-Control: private, no-cache
Vary: Accept-Encoding, X-Group-Codes, X-Source-Password
```

`private, no-cache` permits private storage but requires revalidation and prevents shared caches from reusing group-specific responses without authorization context.

The server parses `Accept-Encoding` including quality values and chooses the first acceptable representation in this preference order:

1. Brotli (`br`)
2. gzip
3. identity

The response sets `Content-Encoding` only for compressed bodies and provides the correct `Content-Length`. A request whose `If-None-Match` matches the current ETag receives `304 Not Modified` with no body.

Compatibility behavior is explicit:

- a new client advertises `br, gzip` and decodes either representation;
- an old Go client that advertises only gzip receives gzip;
- a client that advertises neither receives identity JSON;
- the server never sends Brotli to a client that did not advertise `br`;
- a `304` has no body and therefore needs no decompression.

The server precomputes identity, Brotli, and gzip bytes once per snapshot rather than recompressing every request.

## Client conditional synchronization

Add a default-empty `last_etag` field to `ClientSource` and advance the client schema migration version.

For each source sync, the client:

1. sends `If-None-Match` when it has a validator and the number of locally cached rows equals the source's stored `last_app_count`, including the legitimate zero-application case;
2. sends `Accept-Encoding: br, gzip`;
3. explicitly decodes Brotli, gzip, or identity according to `Content-Encoding`;
4. rejects unsupported or multiply-applied encodings and caps the decompressed feed at 64 MiB;
5. on `304`, updates sync success metadata only and skips JSON parsing, icon work, row replacement, and asset cleanup;
6. on `200`, parses and validates the feed, materializes icons, replaces the cached view transactionally, and stores the new ETag only after the transaction succeeds.

Editing a source clears the validator whenever the request identity changes: source URL, password, or group-code set. Local-only preferences keep the validator and existing derived feed metadata. A `304` is accepted only when the client actually sent `If-None-Match` for that request.

If decoding, validation, icon database work, or the final transaction fails, the previous local view and previous ETag remain intact. If the current cached-row count differs from `last_app_count`, the client omits `If-None-Match` and forces a full recovery response.

The existing full transactional replacement remains for changed feeds. At roughly 101 applications it is simpler and sufficiently fast once unchanged feeds and icon waits are removed; a row-level diff is outside this change.

## Client icon caching and bounded concurrency

Add an `icon_origin_url` field to `ClientSourceApp`. Before deleting the previous rows, build a map keyed by package ID containing the original icon URL, local materialized URL, and asset identity.

Icon processing follows these rules:

- unchanged content-addressed origin URL plus an existing local asset reuses that asset without an HTTP request;
- repeated origin URLs within one feed are materialized once and shared;
- data URLs are hashed and deduplicated locally;
- cross-origin HTTP icons remain remote URLs and are not proxied;
- same-origin icons are fetched by a bounded worker pool of eight workers;
- each request has a five-second timeout and the complete icon phase has a twenty-second deadline;
- a failed or timed-out icon falls back to its remote feed URL and does not fail the source synchronization;
- asset links for reused/new rows are created before old links are removed, so shared content-addressed assets are not deleted during replacement.

Stable but mutable HTTP icon URLs are fetched again after a changed feed response. Reuse without revalidation is limited to data-URL hashes and this server's immutable `/api/v1/assets/{id}` URLs; unchanged feeds still skip all icon work through `304`.

The overall icon deadline, rather than `application count × per-icon timeout`, bounds the tail latency. Therefore 101 unreachable icons cannot hold one source synchronization for more than approximately twenty seconds of icon processing.

The application store's own asset URLs change when icon content changes, so origin-URL reuse remains correct for locally served source icons. Changed origin URLs are fetched again.

## Error handling and operational behavior

- Authentication and group resolution errors occur before cache access and retain their current structured error responses.
- Badger open/read/write/corruption errors degrade to direct feed generation and are logged without secrets.
- A cache miss or invalidation burst produces one build per version/access scope through `singleflight`.
- A request canceled while waiting for a build stops waiting; the shared build may complete for other callers.
- Compression errors discard the candidate snapshot and fall back to direct identity generation for that request; incomplete cache entries are never published.
- Client decompression and size-limit errors are reported as source format errors while retaining the previous cache.
- Individual icon failures are non-fatal and bounded; database errors remain fatal and transactional.
- Shutdown waits for active cache maintenance within the existing shutdown timeout and then closes Badger.

## Testing and verification

### Server tests

- identity, gzip, and Brotli negotiation, including quality values and old-client fallback;
- `ETag` issuance and matching/non-matching `If-None-Match` behavior;
- `304` responses contain no body;
- repeated requests hit one Badger snapshot without rerunning the feed builder;
- concurrent misses collapse to one build;
- a successful feed-affecting mutation changes the generation, body, and ETag;
- public and different valid-group scopes cannot reuse one another's snapshots;
- invalid group-code requests bypass persistent caching and excessive code counts are rejected;
- Badger failure falls back to correct uncached responses;
- startup boot namespaces never serve snapshots from a previous boot;
- server shutdown closes Badger cleanly without goroutine leaks.

### Client tests

- Brotli, gzip, and identity feeds decode correctly;
- unsupported encoding and decompressed-size overflow fail safely;
- a stored ETag is sent and a `304` skips row replacement and all icon requests;
- a `200` stores its ETag only after successful materialization;
- a validator with an empty local cache forces an unconditional recovery request;
- unchanged icons reuse existing assets without network access;
- duplicate icon URLs are fetched once;
- 101 slow/failing icons complete within the overall phase deadline and do not fail synchronization;
- old asset links are cleaned without deleting assets reused by new rows;
- client schema migration preserves existing sources and cached applications.

### Regression and performance checks

- existing source v1/v2, password, group visibility, announcement, ad, mirror, and client-policy tests continue to pass;
- existing client source synchronization and asset-serving tests continue to pass;
- `go test ./...` passes;
- targeted race/goroutine-leak tests cover snapshot rebuild and icon workers;
- benchmarks or counters demonstrate one feed build for repeated cache hits and zero icon fetches for unchanged conditional syncs;
- dependency and staged-secret checks are run before the final commit.

## Rollout and versioning

The server can be deployed before the client:

- old clients continue to receive gzip or identity responses;
- they benefit from faster server-side Badger snapshot generation but still perform their existing local rebuild and icon behavior;
- new clients gain Brotli, conditional `304` synchronization, and bounded icon processing after they are upgraded.

Both affected component versions are updated. Source changes are committed and pushed without building release artifacts.
