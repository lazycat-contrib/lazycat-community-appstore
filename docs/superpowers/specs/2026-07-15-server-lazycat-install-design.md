# Server LazyCat Installation Design

## Goal

Let the server storefront install an LPK directly on the LazyCat device that is running the store server when the request came through a trusted LazyCat client context. Keep the existing browser download behavior for ordinary web visitors.

The behavior applies consistently to LazyCat PC, iOS, and Android clients. The server does not select or control another device: every Go SDK call targets the current device hosting the server application.

## Environment detection

The server is authoritative for deciding whether installation is available. The browser does not send an `isLazyCatClient` flag and does not infer the environment from the user agent.

Add a public runtime-capabilities endpoint that reports whether the current request has a trusted LazyCat installation context. The capability is enabled only when both conditions hold:

- server configuration explicitly enables trusted LazyCat installation headers;
- the request contains non-empty `x-hc-user-id` and `x-hc-device-id` platform headers.

The LazyCat server package sets `TRUST_LAZYCAT_CLIENT_INSTALL=true`. Other deployments default to disabled even if a caller supplies similarly named headers. Capability responses use `Cache-Control: no-store` so Cloudflare or another shared cache cannot reuse a LazyCat response for an ordinary browser, or the reverse.

## User experience

The storefront loads the runtime capability once during application startup.

- When `lazycatInstall` is true, the primary action for the latest version is **Install**.
- When `lazycatInstall` is false or capability discovery fails, the primary action remains **Download**.
- Historical versions follow the same rule: **Install** in a trusted LazyCat context and the existing historical-version download wording on the web.
- Applications without an installable version remain unavailable in both environments.
- Password-protected applications keep the existing password prompt before either action.
- A successful SDK call reports installation completion using the existing installation activity UI.
- An SDK failure reports the server error and leaves the version available for a subsequent retry. Ordinary browser download behavior remains unchanged.

Capability discovery is deliberately fail-closed for installation: network errors, invalid JSON, missing headers, and disabled trust all preserve the current download-only behavior.

## Server API

### Runtime capability

Add:

```text
GET /api/v1/runtime/capabilities
```

Response:

```json
{
  "lazycatInstall": true
}
```

The endpoint is public because the storefront must choose its primary action before site login. It returns only a boolean and never exposes raw LazyCat identity or device values.

### Install version

Add:

```text
POST /api/v1/apps/{id}/versions/{versionId}/install
```

Optional request body for protected applications:

```json
{
  "installPassword": "..."
}
```

The handler repeats the trusted-context check; the capability response is never authorization. A request without the enabled trust setting and required LazyCat headers receives `403 LAZYCAT_INSTALL_UNAVAILABLE`.

The server loads the application and the requested published version from its own database. It derives the LPK download URL, SHA256, package ID, and localized temporary title server-side. The request body cannot supply or override an LPK URL, checksum, package ID, user ID, or device ID.

For password-protected applications, the handler validates the submitted installation password using the same rules as the existing download endpoint. Incorrect or missing passwords retain the existing structured password error behavior.

The handler appends `x-hc-user-id` and `x-hc-device-id` to the outgoing Go SDK context, creates an API gateway, calls `PkgManager.InstallLPK` with a bounded 60-second operation timeout, and always closes the gateway. SDK permission checks remain authoritative for whether the LazyCat user may install applications.

Success response:

```json
{
  "mode": "lazycat-go-sdk",
  "taskId": "...",
  "status": "INSTALL_OK",
  "detail": "..."
}
```

Gateway creation, permission, download, checksum, and installation failures return a structured `502 LAZYCAT_INSTALL_FAILED` response without leaking credentials or internal certificate paths.

## Shared SDK boundary

Move the reusable LazyCat package-manager adapter out of the standalone client-server package into a focused internal package. Both binaries use the same adapter rather than duplicating Go SDK setup, metadata propagation, request construction, timeout handling, task mapping, and gateway closing.

The shared adapter accepts an explicit identity containing user and device IDs. The standalone client continues to pass its current user context and remains behaviorally unchanged. The store server passes values obtained only from the trusted request headers.

## Caching and security

- Runtime capabilities and install responses use `Cache-Control: no-store`.
- The install endpoint accepts only `POST` and remains unavailable unless `TRUST_LAZYCAT_CLIENT_INSTALL=true`.
- Both `x-hc-user-id` and `x-hc-device-id` are required and normalized by trimming whitespace.
- Platform header values are never logged or returned to the browser.
- Application and version IDs are parsed and validated; the version must belong to the application and be published/installable.
- The server selects the artifact and checksum from trusted database state.
- Existing download counting remains attached to the download endpoint. Direct SDK installation increments the application download count once only after the SDK reports success, matching a completed artifact acquisition rather than an attempted click.
- Cloudflare must not cache `/api/v1/runtime/capabilities` or any install response; application-level `no-store` headers enforce this even without a special cache rule.

## Error handling

- Capability lookup failure: keep **Download** with no blocking error.
- Missing/disabled LazyCat context: return `403 LAZYCAT_INSTALL_UNAVAILABLE`.
- Missing application or version: return the existing `404` application/version error.
- Unpublished or non-installable version: return `409 VERSION_NOT_INSTALLABLE`.
- Invalid installation password: return the existing password-required/password-invalid response.
- SDK failure or timeout: return `502 LAZYCAT_INSTALL_FAILED` with a safe user-facing message; log the wrapped server-side cause without platform header values.
- Repeated clicks while an installation is running use the existing single in-flight installation guard in the frontend.

## Testing and verification

Server regression tests cover:

- capability false by default, including spoofed headers when trust is disabled;
- capability true only with the trust flag and both required headers;
- `Cache-Control: no-store` on both capability variants;
- install rejection without a trusted context;
- database-derived URL, checksum, package ID, title, user ID, and device ID passed to a fake package manager;
- protected-version password validation;
- version/application mismatch and non-installable version rejection;
- SDK failure mapping and safe response body;
- successful download-count increment without incrementing failed attempts.

Frontend regression tests cover:

- LazyCat capability changes latest and historical actions from download to install;
- false or failed capability lookup preserves download wording and behavior;
- install calls the new server endpoint with only the selected version and optional password;
- browser download still opens the existing URL;
- unavailable versions remain disabled;
- existing single-install guard and activity result behavior remain intact.

Repository verification includes frontend type checking/build, focused Go tests, `go test ./...`, `go vet ./...`, formatting, module-tidiness checks, and the existing CI-equivalent checks. The server package advances from `0.1.32` to `0.1.33`; the client package remains `0.1.28`.
