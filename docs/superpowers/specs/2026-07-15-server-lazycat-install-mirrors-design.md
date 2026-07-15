# Server LazyCat Install Mirror Selection Design

## Goal

Give the server storefront the same GitHub mirror-selection experience as the standalone client when installing through the LazyCat Go SDK.

The behavior applies only when request-scoped LazyCat installation is available. Ordinary web visitors continue to download as before.

## Product behavior

- Direct download is always the default selection.
- A GitHub Release or GitHub Code ZIP LPK shows configured `download` mirrors.
- A `raw.githubusercontent.com` LPK shows configured `raw` mirrors.
- A non-GitHub LPK does not show a mirror selector.
- A GitHub LPK with no applicable configured mirrors installs directly without opening an otherwise empty options dialog.
- Password-protected applications continue to open the install options dialog even when no mirrors exist.
- When both password and mirror selection apply, they share the existing install options dialog.
- Latest and historical versions use the same rules based on the selected version URL.

The server does not gain a new default-mirror setting. This matches the standalone client principle that a mirror is preselected only when an explicit default exists; the server has no such per-source default.

## Runtime capability data

Extend:

```text
GET /api/v1/runtime/capabilities
```

Trusted LazyCat response:

```json
{
  "lazycatInstall": true,
  "githubMirrors": [
    {
      "id": "ghm_0123456789ab",
      "kind": "download",
      "name": "Fast mirror"
    }
  ]
}
```

The response includes only the mirror ID, kind, and display name. The frontend does not need the mirror base URL and therefore does not receive it from this endpoint.

When `lazycatInstall` is false, `githubMirrors` is an empty array. The existing `Cache-Control: no-store` behavior remains mandatory so Cloudflare cannot mix request-scoped installation capabilities or stale mirror configuration.

The runtime response is a discovery hint, not authorization. The install endpoint reloads current mirror configuration and validates the submitted mirror ID independently.

## Frontend data flow

Introduce a shared mirror-option shape containing `id`, `kind`, and `name`. Existing source mirror records extend that shape with `url`; runtime capability mirrors use only the shared fields.

Generalize the existing mirror helper functions and `InstallOptionsDialog` so they consume an install mirror configuration rather than requiring a complete `SourceSubscription`.

For a standalone source application, the configuration remains:

- the source's mirror list;
- the source's explicit default download/raw mirror IDs.

For a server storefront application, the configuration is:

- mirrors from runtime capabilities;
- empty default download/raw mirror IDs, which selects direct download.

Before installation, the application determines the selected version and applicable mirror list:

- source apps keep opening the options dialog as today;
- server apps open it when a password is required or at least one applicable mirror exists;
- server apps with neither condition install immediately;
- submitting the dialog sends only `installPassword` and `mirrorId`.

The frontend never rewrites an LPK URL.

## Server installation behavior

Extend the install request body:

```json
{
  "installPassword": "...",
  "mirrorId": "ghm_0123456789ab"
}
```

`mirrorId` is optional. When empty, the server sends the approved version's direct database URL to the shared LazyCat SDK installer.

When non-empty, the server:

1. verifies that the approved version URL is a supported GitHub URL;
2. reloads the current effective GitHub mirror configuration;
3. finds the mirror by ID and requires its kind to match the selected version URL;
4. rewrites the database-derived version URL with the validated mirror entry;
5. passes only the rewritten server-derived URL to the SDK adapter.

Failure responses match the existing download endpoint semantics:

- `422 MIRROR_NOT_APPLICABLE` when the selected version is not a GitHub URL;
- `422 MIRROR_NOT_FOUND` when the mirror no longer exists or its kind does not match.

The server never accepts a mirror URL from the client. Existing trusted Header checks, installation password validation, single-install concurrency gate, safe SDK error mapping, and success-only download counting remain unchanged.

## Error handling and refresh behavior

- Runtime capability failure preserves current direct download/web behavior.
- A mirror removed after the dialog opens is rejected by the server instead of silently choosing another mirror.
- A stale mirror selection returns a clear error; reopening the install dialog uses the latest runtime mirror list and allows direct download or another current mirror.
- SDK failure after mirror validation uses the existing `LAZYCAT_INSTALL_FAILED` result.
- Mirror configuration is request-time data and is never persisted in browser storage.

## Testing and verification

Server tests cover:

- trusted capabilities expose mirror ID, kind, and name but not URL;
- untrusted capabilities return no mirrors;
- direct install preserves the original version URL;
- a valid download mirror rewrites a GitHub Release URL;
- raw and download mirror kinds cannot be mixed;
- unknown, removed, and non-applicable mirrors return structured `422` errors;
- the request cannot provide a mirror URL or other unknown fields.

Frontend tests cover:

- runtime capability mirror data is loaded fail-closed;
- GitHub Release versions show only download mirrors;
- raw GitHub versions show only raw mirrors;
- non-GitHub versions skip mirror selection;
- direct is selected by default for server installs;
- password and mirror selection share one dialog;
- latest and historical server installs submit the selected `mirrorId` without any URL;
- standalone client source defaults continue to work unchanged.

Repository verification includes all Go and frontend tests, race coverage for the server installation handler and shared SDK adapter, frontend build and embedded-asset parity, dependency audit, OpenAPI validation, Go vet, module tidy, and golangci-lint.

The server package advances from `0.1.33` to `0.1.34`. The client package remains `0.1.28`. No LPK release artifact is built; source changes are committed and pushed for the user to build.
