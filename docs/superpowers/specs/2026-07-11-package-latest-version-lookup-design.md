# Package Latest-Version Lookup Design

## Goal

Provide a compact public read API for update clients to retrieve the latest approved version by exact LPK `packageId`.

## Contract

`GET /api/v1/packages/{packageId}/latest-version`

The response is `{"packageId":"...","latestVersion":{...}}`, where `latestVersion` uses the existing version response shape. The path value must be non-empty after trimming and must match `packageId` exactly.

The endpoint only returns an application visible to the request and only an `APPROVED` version. For group-scoped applications, a caller may provide a valid code through the `groupCodes` query parameter or `X-Group-Codes` header, matching source-feed access. Missing, inaccessible, or versionless applications return the existing structured `404 APP_NOT_FOUND` error, so callers cannot distinguish unpublished content from absent content.

## Design Rationale

The existing `GET /api/v1/apps?q=` performs a fuzzy multi-field search. It is unsuitable for deterministic update checks even though `packageId` is unique. A package-scoped read endpoint makes the exact-match behavior explicit while avoiding a new database model or a full source-index download.

## Verification

Add handler tests for exact success, unknown package, missing approved version, non-public application, and group-code visibility. Add the route and response schema to OpenAPI, then run focused Go tests, OpenAPI validation, and the full Go test suite.
