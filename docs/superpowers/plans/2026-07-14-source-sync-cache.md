# Software Source Synchronization Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Badger-backed server feed snapshots, HTTP validators and Brotli/gzip negotiation, plus client conditional synchronization and bounded icon caching.

**Architecture:** The relational database remains authoritative. The server materializes immutable v1/v2 feed snapshots into a boot-scoped Badger namespace and invalidates them after successful feed-affecting mutations. The client stores the last ETag with its derived source view, skips all work on `304`, and reuses icons through origin-aware bounded workers on changed feeds.

**Tech Stack:** Go 1.26, Ent, SQLite/PostgreSQL/MySQL, Badger v4.9.4, `andybalholm/brotli` v1.2.2, `singleflight`, HTTP conditional requests, Node 26 contract tests.

## Global Constraints

- Pin `github.com/dgraph-io/badger/v4` to v4.9.4 and `github.com/andybalholm/brotli` to v1.2.2.
- Default `SOURCE_CACHE_PATH` is `./data/source-cache`; empty test configuration uses in-memory Badger.
- Cache entries use a 24-hour TTL and a unique boot namespace; no snapshot from a previous process boot is served.
- Support one active server process per Badger directory.
- Authenticate source passwords before cache lookup and never store or log passwords/group codes.
- Accept at most 64 normalized group codes; requests with invalid codes bypass persistent snapshot caching.
- Prefer `br`, then `gzip`, then identity according to parsed quality values.
- Use a weak ETag derived from the identity JSON and return `304` without a body on a match.
- Cap decompressed client feeds at 64 MiB.
- Materialize same-origin icons with eight workers, five seconds per request, and twenty seconds overall.
- Do not build LPK artifacts or regenerate the full embedded frontend bundle.
- Bump client `0.1.27 -> 0.1.28` and server `0.1.31 -> 0.1.32` after verification.

---

### Task 1: Add snapshot dependencies and the Badger cache core

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/server/source_feed_cache.go`
- Create: `internal/server/source_feed_cache_test.go`

**Interfaces:**
- Produce `sourceFeedSnapshot` with `Identity`, `Brotli`, `Gzip`, `ETag`, and `BuiltAt`.
- Produce `sourceFeedAccessScope` with canonical sorted group IDs and resolved group metadata.
- Produce `newSourceFeedCache(path, build)` and methods `GetOrBuild`, `InvalidateAndWarm`, and `Close`.

- [ ] Add failing tests for Badger miss/hit persistence within one boot, 24-hour TTL metadata, generation invalidation, concurrent miss collapse, and a fresh boot namespace ignoring an old snapshot.
- [ ] Run `go test ./internal/server -run 'TestSourceFeedCache' -count=1` and confirm the new tests fail because the cache types do not exist.
- [ ] Add Badger v4.9.4 and Brotli v1.2.2 with `go get`, then implement boot-scoped keys:

```text
source-feed/<boot>/<generation>/<version>/<scope-hash>/{meta,identity,br,gzip}
```

- [ ] Serialize only non-secret metadata, publish all representations in one Badger transaction, and use `singleflight.DoChan` so a canceled waiter does not cancel the shared build.
- [ ] Make Badger read/write failures bypass the cache and call the builder directly without publishing partial values.
- [ ] Expire entries after 24 hours and remove the previous generation prefix asynchronously after invalidation; cleanup errors are logged and never fail a request.
- [ ] Run the focused cache tests and confirm they pass.
- [ ] Commit with `feat: add badger source feed cache`.

### Task 2: Refactor feed generation and serve conditional compressed snapshots

**Files:**
- Modify: `internal/config/config.go`, `internal/config/config_test.go`
- Modify: `internal/server/server.go`, `internal/server/handlers_source.go`, `internal/server/group_codes.go`
- Modify: `internal/server/server_test.go`, `internal/server/lifecycle_test.go`

**Interfaces:**
- `buildSourceFeed(ctx, version, access) ([]byte, error)` produces the exact identity JSON.
- `serveSourceFeedSnapshot(w, r, snapshot)` performs ETag and encoding negotiation.
- `sourceFeedCache *sourceFeedCache` is owned and closed by `Server`.

- [ ] Add failing HTTP tests for identity, gzip, Brotli, quality values, `Vary`, private revalidation headers, matching/non-matching ETags, and bodyless `304`.
- [ ] Add failing compatibility tests proving requests without `br` never receive Brotli and old gzip clients still decode the response.
- [ ] Add failing group tests proving public and valid-group scopes have different bodies/ETags, invalid-code requests are not stored, and more than 64 normalized codes return `400`.
- [ ] Extract current feed assembly from `handleSourceIndex` into `buildSourceFeed`, preserving v1/v2 output and authorization behavior.
- [ ] Initialize the cache after database migration, using `SOURCE_CACHE_PATH`; use in-memory mode when the path is empty in tests, and continue with uncached generation if Badger cannot open.
- [ ] Implement RFC-aware `Accept-Encoding` parsing, `Content-Encoding`, `Content-Length`, `ETag`, `Cache-Control: private, no-cache`, and `Vary` headers.
- [ ] Close Badger after server background work stops and before the relational client closes.
- [ ] Run `go test ./internal/server -run 'TestSource|TestServerClose|TestShutdown' -count=1` and confirm all focused tests pass.
- [ ] Commit with `feat: serve cached compressed source feeds`.

### Task 3: Invalidate snapshots after feed-affecting mutations

**Files:**
- Modify: `internal/server/server.go`
- Create: `internal/server/source_feed_invalidation.go`
- Modify: `internal/server/lpk_inspection.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- `withSourceFeedInvalidation(next)` records the response status and calls `InvalidateAndWarm` only after a successful mutating handler returns.
- `invalidateSourceFeed()` advances generation synchronously and warms public v1/v2 using the server lifecycle context.

- [ ] Add a failing mutation matrix test covering app approval/update/delete, version approval/deletion, visibility/group/category/tag changes, site settings, announcements, ads, outdated marks, migration import, and user display-name changes.
- [ ] Wrap only feed-affecting mutating routes in `withSourceFeedInvalidation`; do not invalidate for login, downloads, comments, chat, favorites, or read-only requests.
- [ ] Add an explicit post-commit invalidation call to background LPK metadata application because it does not pass through HTTP routing.
- [ ] Confirm unsuccessful (`4xx/5xx`) mutations do not change generation and successful mutations cause the next source request to receive a new ETag/body.
- [ ] Run `go test ./internal/server -run 'TestSourceFeedInvalidation' -count=1`.
- [ ] Commit with `feat: refresh source cache after updates`.

### Task 4: Store client validators and decode compressed feeds

**Files:**
- Modify: `ent/schema/client_source.go`
- Regenerate: `ent/` via `go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema`
- Modify: `internal/clientserver/schema_migrations.go`
- Modify: `internal/clientserver/http_clients.go`
- Modify: `internal/clientserver/sync.go`
- Modify: `internal/clientserver/server_test.go`, `internal/clientserver/http_clients_test.go`

**Interfaces:**
- `ClientSource.LastEtag string` stores the validator for the committed local view.
- `sourceFetchResult` distinguishes `NotModified` from a decoded feed and carries the response ETag.
- `sourceResponseBody(resp)` returns a bounded identity/Brotli/gzip reader.

- [ ] Add failing tests proving the client sends `If-None-Match` only when cached row count equals `last_app_count`, advertises `br, gzip`, and handles identity/gzip/Brotli.
- [ ] Add failing tests for unsupported/multiple encodings and a decompressed body larger than 64 MiB.
- [ ] Add a failing `304` test proving no JSON parse, row replacement, asset relink, or icon request occurs while sync-success metadata updates.
- [ ] Add `last_etag`, regenerate Ent, and advance `currentClientSchemaVersion` with an explicit migration path.
- [ ] Implement explicit gzip/Brotli decoding because setting `Accept-Encoding` disables Go's automatic gzip behavior.
- [ ] Store a response ETag in the same transaction as the successful source-view replacement; retain the old ETag on every failure.
- [ ] Run `go test ./internal/clientserver -run 'TestSyncSource|TestSourceResponse|TestClientSourceSchema' -count=1`.
- [ ] Commit with `feat: add conditional client source sync`.

### Task 5: Reuse icons and bound icon synchronization latency

**Files:**
- Modify: `ent/schema/client_source_app.go`
- Regenerate: `ent/`
- Modify: `internal/clientserver/assets.go`, `internal/clientserver/sync.go`
- Modify: `internal/clientserver/server_test.go`, `internal/clientserver/schema_migrations.go`

**Interfaces:**
- `ClientSourceApp.IconOriginURL string` stores the raw feed URL separately from the local materialized URL.
- `materializeSourceIcons(ctx, source, apps, oldApps)` returns per-app icon results using eight workers and a twenty-second phase context.

- [ ] Add failing tests proving an unchanged origin reuses the old local asset, duplicate origins fetch once, data URLs deduplicate, and cross-origin icons are not fetched.
- [ ] Add a deterministic deadline test using more blocked icon URLs than worker slots; release the test server or cancel context without a wall-clock sleep longer than the five-second request timeout.
- [ ] Add `icon_origin_url`, regenerate Ent, and migrate existing rows with an empty origin so the next changed feed performs one safe refresh.
- [ ] Build an old-app map before materialization, reuse only valid local asset URLs with unchanged origins, and deduplicate jobs by origin.
- [ ] Use eight workers, five-second child contexts, and one twenty-second parent phase; on fetch failure preserve the remote feed URL and continue synchronization.
- [ ] Link new/reused assets before deleting old links and verify reused assets survive cleanup.
- [ ] Run `go test ./internal/clientserver -run 'TestSyncSource.*Icon|TestMaterializeSourceIcons' -count=1`.
- [ ] Commit with `perf: bound source icon synchronization`.

### Task 6: Version and verify source synchronization changes

**Files:**
- Modify: `lazycat/client/package.yml`
- Modify: `lazycat/server/package.yml`
- Modify: `clientembed/dist/app-config.js`

**Interfaces:**
- Client package/config version is `0.1.28`.
- Server package version is `0.1.32`.

- [ ] Update only the version metadata files; do not run the LPK release/build pipeline or regenerate hashed frontend assets.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `go test -race ./internal/server ./internal/clientserver -count=1`.
- [ ] Run `go vet ./...`, `go mod tidy -diff`, and `git diff --check`.
- [ ] Run `govulncheck ./...` when available; otherwise record that the command is unavailable, and run `npm audit --audit-level=high --registry=https://registry.npmjs.org` in `client`.
- [ ] Inspect staged dependency and secret-related diffs before committing.
- [ ] Commit with `chore: bump source sync versions`.

### Task 7: Integrate and push the completed change set

**Files:**
- Verify: all files changed by this plan and `2026-07-14-default-category-browser.md`

**Interfaces:**
- Local `main` and `origin/main` end at the same verified implementation commit.

- [ ] Run `git status --short --branch`, inspect every remaining diff, and confirm no generated LPK or unintended `clientembed/dist/assets/**` changes exist.
- [ ] Run the focused category Node tests and the full Go verification commands one final time after all commits are present.
- [ ] Push `main` with `git push origin main`.
- [ ] Verify `git rev-parse HEAD` equals `git rev-parse origin/main` and report the final commits, versions, tests, and the fact that product builds were intentionally not run.
