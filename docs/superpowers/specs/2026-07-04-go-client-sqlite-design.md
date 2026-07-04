# Go SQLite Client Redesign

## Thesis

The standalone LazyCat app store client should become a small Go application with an embedded web UI and a durable per-user SQLite database. The browser should stop being the source of truth for software sources, source passwords, synced app caches, and install state. The Go client process should own persistence, source syncing, installed-app queries, and LPK installation through the LazyCat Go SDK.

This keeps the client independently deployable while fixing the current cross-device failure mode: a user who logs in from another device should see the same source subscriptions and cached catalog because those records live in the LazyCat app instance, not in that device's browser storage.

## Current Problem

The current `lazycat/client` package serves static Vite output directly from `file:///lzcapp/pkg/content/dist/`. The React app stores source subscriptions and synced app cache in `localStorage`:

- `lazycat.sources`
- `lazycat.sourceApps`

This makes source configuration device-local. A user who signs in from a second browser or device loses source subscriptions, mirror settings, source passwords, and synced catalog cache.

## Target Shape

Keep two independently deployable LazyCat apps:

- `lazycat/server`: the existing self-hosted app store server.
- `lazycat/client`: a standalone source browser and installer client, rebuilt as Go backend plus embedded frontend.

The client package changes from a static app to a background Go service:

- New entrypoint: `cmd/store-client/main.go`.
- New package: `internal/clientserver`.
- New embedded frontend package or reuse of the existing `web` embedding pattern with a client-specific build output.
- New SQLite database at `/lzcapp/var/data/client.db` in LazyCat and `./data/client.db` for local development.
- LazyCat manifest uses `backend_launch_command` to start the Go client binary.

## Data Model

Use ent and SQLite for the standalone client data. Keep the client schema separate from the server schema unless implementation shows reuse is simpler without coupling.

### ClientSource

Stores software source subscriptions per LazyCat user.

Fields:

- `id`: integer primary key.
- `user_id`: LazyCat user id from `x-hc-user-id`; local development fallback is `local`.
- `name`: display name.
- `url`: source feed URL.
- `password`: source password. Store in SQLite instead of browser `localStorage`.
- `mirror`: optional GitHub mirror prefix.
- `last_sync`: nullable timestamp.
- `last_error`: nullable string.
- `last_error_code`: nullable enum-like string: `auth`, `format`, `http`, `network`.
- `last_app_count`: integer default `0`.
- `last_installable_count`: integer default `0`.
- `created_at`, `updated_at`.

Indexes:

- Unique `(user_id, url)` to prevent duplicate source subscriptions for the same user.
- Index `(user_id, updated_at)` for source list queries.

### ClientSourceApp

Stores the synced app catalog cache per source.

Fields:

- `id`: integer primary key.
- `source_id`: foreign key to `ClientSource`.
- `external_id`: upstream app id when provided.
- `name`, `slug`, `summary`, `category`.
- `install_protected`: boolean.
- `latest_version_json`: JSON string containing version, download URL, upstream download URL, source type, sha256, and size.
- `created_at`, `updated_at`.

Indexes:

- Unique `(source_id, slug)` to replace apps on each source sync without duplicates.
- Index `(source_id, updated_at)`.

### Browser-Only Preferences

Do not create a settings table in the first implementation. Durable data is limited to software sources and cached source apps.

Keep these preferences in browser storage because they are device presentation choices, not shared client configuration:

- Theme mode stays in `localStorage`.
- Language detection cache stays in `localStorage`.

## User Boundary

Every client API request resolves a user id:

1. Prefer `x-hc-user-id` injected by LazyCat.
2. Use `local` in local development when the header is absent.

All source and app-cache queries are scoped by `user_id`. Users on the same LazyCat box should not see or mutate each other's source subscriptions.

## Client API

Expose standalone-client APIs under `/api/client/v1` to avoid colliding with the server store API.

### Sources

- `GET /api/client/v1/sources`: list current user's sources.
- `POST /api/client/v1/sources`: create a source; rejects duplicate normalized URL for that user.
- `PATCH /api/client/v1/sources/{id}`: update name, URL, password, or mirror.
- `DELETE /api/client/v1/sources/{id}`: delete source and cached apps.
- `POST /api/client/v1/sources/{id}/sync`: sync one source from the Go backend.
- `POST /api/client/v1/sources/sync`: sync all current user's sources.

### Apps

- `GET /api/client/v1/apps`: list cached apps, with query/filter support for search, source id, installable status, and installed status.
- `GET /api/client/v1/apps/{id}`: return cached app detail.

### Installed Apps And Installation

- `GET /api/client/v1/installed`: query LazyCat installed applications through the Go SDK.
- `POST /api/client/v1/install`: install one cached app version through the Go SDK.

The Go SDK is the only intended installation entrypoint. The frontend should no longer call `@lazycatcloud/sdk` for install or installed-app lookup once the Go SDK adapter is implemented.

## Source Sync Flow

The frontend sends sync commands to the Go client backend. The backend:

1. Loads the source for the current user.
2. Fetches the source feed URL.
3. Sends source password as both query parameter and `X-Source-Password`, preserving existing feed compatibility.
4. Validates the response shape: top-level `apps` must be an array.
5. Rewrites GitHub download URLs through the source mirror when a mirror is configured and the app is not install protected.
6. Replaces cached apps for that source in one transaction.
7. Updates `last_sync`, `last_error`, `last_error_code`, `last_app_count`, and `last_installable_count`.

Failure behavior:

- `401` maps to `auth`.
- Invalid JSON or missing `apps` maps to `format`.
- Non-OK HTTP status maps to `http`.
- Network and timeout failures map to `network`.

Use a bounded timeout for sync requests so one slow source cannot hang the whole sync-all operation.

## LazyCat Go SDK Integration

Create an adapter inside `internal/clientserver`, not directly in handlers.

Responsibilities:

- Build the LazyCat API gateway with context.
- Add outgoing user metadata when the SDK call is user-specific.
- Query installed applications.
- Install LPK packages.
- Close the gateway after each operation or per operation batch.

The implementation must use the actual Go SDK request and method names discovered from the installed package at coding time. The public clientserver interface should stay stable even if the SDK names differ from the JavaScript `InstallLPK` naming.

Adapter interface:

```go
type LazyCatPackageManager interface {
    QueryInstalled(ctx context.Context, userID string) ([]InstalledApplication, error)
    InstallLPK(ctx context.Context, userID string, req InstallLPKRequest) (InstallLPKResult, error)
}
```

Local development behavior:

- If LazyCat SDK gateway creation fails outside LazyCat, return clear API errors for install and installed-app lookup.
- Source management and source sync should still work locally.

## Frontend Changes

Keep React/Vite for now. The frontend migration is a data-layer change, not a full framework rewrite.

Remove browser persistence for durable client data:

- Delete `lazycat.sources` reads/writes.
- Delete `lazycat.sourceApps` reads/writes.
- Replace direct source feed `fetch` with `/api/client/v1/sources/{id}/sync`.
- Replace installed-app JS SDK calls with `/api/client/v1/installed`.
- Replace install JS SDK calls with `/api/client/v1/install`.

Keep browser-local preferences:

- `lazycat.theme`.
- i18next language cache.

Default source seeding:

- Continue supporting `app-config.js` default source values.
- Backend seeds the default source for a user only when that user has no sources.

## UI Direction

Use the current restrained operational UI as the base. Improve the server and client frontend together so the two apps feel like one product.

### Client UI

Focus the standalone client on three jobs:

1. Maintain software sources.
2. Browse synced installable apps.
3. Install and verify installed apps.

Changes:

- Source page becomes a source management workspace: source list, health state, sync metadata, password/mirror editing, and cached app count in one scannable row.
- Search/install page emphasizes app readiness: installable, installed, incomplete metadata, checksum state, and protected-install status.
- Installed page uses Go-backed installed app data and clear SDK-unavailable states.
- Loading states use skeleton blocks that match the page shape rather than a generic blank loading panel.
- Errors are inline and action-specific.

### Server UI

Apply the same visual system to the server-backed mode:

- Reuse button press feedback, focus rings, status chips, drawer polish, and loading/empty states.
- Keep admin and review screens dense and work-focused.
- Avoid adding a marketing-style hero or decorative visual noise.

### Interaction Polish

Apply `emil-design-eng` principles conservatively:

- Buttons and clickable cards get subtle `:active { transform: scale(0.97); }`.
- Hover transitions are gated behind `@media (hover: hover) and (pointer: fine)`.
- Dynamic UI transitions use `transform` and `opacity`, not `width`, `height`, `top`, or `left`.
- Drawers, modals, toasts, and install panels enter with short ease-out transitions under 300ms.
- Respect `prefers-reduced-motion`.
- Do not animate keyboard-heavy actions.

## Packaging

Update `lazycat/client`:

- Build frontend.
- Embed frontend into `store-client` binary.
- Build Go binary into `lazycat/client/content/store-client`.
- Manifest changes:
  - `background_task: true`.
  - Upstream backend `http://127.0.0.1:<port>/`.
  - `backend_launch_command: /lzcapp/pkg/content/store-client`.
  - `CLIENT_DB_DSN=/lzcapp/var/data/client.db`.
  - Keep `net.internet`.

Do not add speculative LazyCat permissions. Keep the manifest minimal unless the Go SDK compiler path or LazyCat package validation identifies a concrete required permission name.

The client remains independently releasable as `community.lazycat.app.lazycat-community-appstore-client`.

## Migration Behavior

There is no reliable server-side way to read another device's browser `localStorage`. Existing browser-local source lists can remain as stale browser state during the transition, but the new API-backed frontend should stop reading them.

Recommended first-run behavior after upgrade:

- If SQLite has no sources for the user and `app-config.js` contains a default source, seed it.
- Otherwise show an empty source state with an add-source action.

Manual migration from old browser storage is not part of the first implementation.

## Testing

Backend:

- `go test ./...`
- ent tests using in-memory SQLite for source CRUD, source uniqueness, source deletion cascade, and user isolation.
- Handler tests for source CRUD, sync success, sync auth failure, invalid feed format, and sync-all partial failure.
- SDK adapter tests with a fake `LazyCatPackageManager`.

Frontend:

- `cd client && npm run build`
- Verify no source/app durable data writes to `localStorage`.
- Manual responsive pass at 320px, 768px, 1024px, and 1440px.

Packaging:

- Validate `lazycat/client/package.yml`, `lzc-manifest.yml`, and `lzc-build.yml` with `js-yaml`.
- Build the client LPK path after the Go binary compiles.

## Out Of Scope

- Syncing source configuration to the central app store server.
- Cross-box replication.
- Encrypting source passwords at rest.
- Manual import from existing browser `localStorage`.
- Generic client settings storage.
- Replacing React/Vite with server-rendered templates.
- Redesigning product information architecture beyond the source/install/installed workflows.

## Payoff Ledger

| Move | Price Paid Now | Unlock |
| --- | --- | --- |
| Go backend for standalone client | More code than static hosting | Cross-device durable client configuration |
| ent + SQLite | New schema and generated code | Clear data boundaries, transactions, tests, future migration path |
| User-scoped source data | More handler plumbing | Multiple LazyCat users can share one app instance safely |
| Go SDK install adapter | SDK method discovery and wrapper code | One install path, less browser dependency, better error handling |
| API-backed React state | More frontend data-loading states | Removes sensitive source data from `localStorage` |
| Shared UI polish pass | More CSS review | Server and client feel like one coherent product |
