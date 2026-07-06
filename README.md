# LazyCat Community App Store

Self-hosted app store for LazyCat LPK packages. The project ships as two independent LazyCat bare apps:

- `lazycat/server`: Go API server with the Vite web console embedded into the binary.
- `lazycat/client`: standalone source browser and installer client. It can be deployed without the server and subscribe to any compatible `/source/v1/index.json`.

## Server

Local API server:

```bash
go run ./cmd/store-server
```

Default server URL: `http://localhost:8080`

First-run initialization supports two paths:

- Web setup wizard: when no site administrator exists and no admin environment variables are set, open the server URL and create the first administrator from the browser.
- Environment bootstrap: set `ADMIN_USERNAME` and/or `ADMIN_PASSWORD` before the first start to create the initial site administrator automatically.

If only one admin environment variable is set, the other keeps its development fallback (`admin` / `changeme`). For production, set both values explicitly.

To build the server with the web console embedded, build the client first and copy it into `web/dist` before compiling Go:

```bash
(cd client && npm ci && VITE_API_BASE_URL=. npm run build)
rm -rf web/dist
mkdir -p web/dist
cp -R client/dist/. web/dist/
go build -o dist/store-server ./cmd/store-server
```

The LazyCat server package does this automatically from `lazycat/server/build.sh`.

## Client

Local standalone client:

```bash
go run ./cmd/store-client
```

Default client URL: `http://127.0.0.1:8090`

The standalone client now runs as a Go app with the React UI embedded into the binary. Source subscriptions and synced source apps are stored in SQLite, so they persist on the LazyCat device instead of browser `localStorage`. Browser storage is only used for UI preferences such as theme and language.

Default local database path: `./data/client.db`. In the LazyCat client LPK it uses `/lzcapp/var/data/client.db`.

For frontend-only development, run Vite separately:

```bash
cd client
npm install
npm run dev
```

Users can open the Software Sources page, add a source URL, sync it, and install LPKs through the LazyCat Go SDK-backed client API.

Optional runtime config is loaded from `app-config.js`:

```js
window.LAZYCAT_APPSTORE_CONFIG = {
  apiBaseURL: "",
  defaultSourceURL: "https://example.com/source/v1/index.json",
  defaultSourceName: "Example Store"
};
```

When `apiBaseURL` is empty, server-backed features such as login, submission, and admin review are hidden or disabled.

## LazyCat Bare Apps

Build server LPK:

```bash
cd lazycat/server
lzc-cli project release -o ../../dist/lazycat-community-appstore-server.lpk
```

Build standalone client LPK:

```bash
cd lazycat/client
lzc-cli project release -o ../../dist/lazycat-community-appstore-client.lpk
```

The client build accepts optional build-time defaults:

```bash
CLIENT_DEFAULT_SOURCE_URL=https://store.example.com/source/v1/index.json \
CLIENT_DEFAULT_SOURCE_NAME="Example Store" \
lzc-cli project release -o ../../dist/lazycat-community-appstore-client.lpk
```

## Server Capabilities

- User registration/login, email verification token flow, API tokens.
- Role-based access for users, software admins, and site admins.
- App submission, `.lpk` upload, external GitHub/WebDAV/S3 URL versions, review approval.
- SHA256 calculation for uploaded LPK files; external URL versions require a SHA256 value.
- Local, WebDAV, S3-compatible storage backends, and GitHub external-link mode.
- App screenshots, comments, favorites, outdated marks, collaborator requests.
- User groups and app visibility filtering.
- Categories, tags, manual collections, recent-updated collections, most-downloaded collections.
- Site title, icon, public source URL, and storefront announcement customization.
- Source feed password protection, password rotation setting, GitHub mirror rewriting.
- Source feed versions expose upstream URLs so subscribed clients can apply their own GitHub mirror.
- Download endpoint with download-count tracking.
- SMTP email delivery for email verification when `SMTP_HOST` and `SMTP_FROM` are configured.

## Verification

```bash
go test ./...
(cd client && npm audit --audit-level=high --registry=https://registry.npmjs.org)
(cd client && npm run build)
npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
npx --yes js-yaml lazycat/server/package.yml
npx --yes js-yaml lazycat/server/lzc-manifest.yml
npx --yes js-yaml lazycat/server/lzc-deploy-params.yml
npx --yes js-yaml lazycat/server/lzc-build.yml
npx --yes js-yaml lazycat/client/package.yml
npx --yes js-yaml lazycat/client/lzc-manifest.yml
npx --yes js-yaml lazycat/client/lzc-build.yml
```

Smoke-test the source feed:

```bash
curl http://localhost:8080/source/v1/index.json
```

## API Contract

- OpenAPI: `docs/openapi.yaml`
- Architecture and delivery plan: `docs/plan.md`
