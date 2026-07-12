# Per-App Automatic Update Policy Design

## Goal

Allow a client user to disable scheduled automatic updates for individual installed applications without blocking manual single-app or bulk updates.

## User Experience

- Every installed source application shows an `Automatic updates` switch.
- The switch defaults to enabled when no stored policy exists.
- Turning the switch off persists immediately and changes the application state label to `Manual updates only`.
- Scheduled automatic updates skip applications whose switch is off.
- Manual `Update all` continues to include those applications and labels them `Automatic updates disabled` in the confirmation list.
- Manual single-app installation and update behavior is unchanged.
- Password-protected applications remain excluded from scheduled updates under the existing rule.

## Data Model

Create `ClientAppUpdatePolicy` in the standalone client's local database (`client.db`), not in the app-store server database, with:

- `user_id`: non-empty client user identifier.
- `package_id`: normalized, non-empty application package ID.
- `auto_update_enabled`: boolean, default `true`.
- `created_at` and `updated_at` timestamps.
- Unique index on `(user_id, package_id)`.

Absence of a row means automatic updates are enabled. A disabled row remains when an application is removed, its source changes, or it is reinstalled, so the preference follows the stable package identity.

The preference is device-local. It applies only to scheduled updates performed by this client installation and is not synchronized to another client device using the same LazyCat account.

## API Contract

Extend each `InstalledApplicationDTO` returned by `GET /api/client/v1/installed` with:

```json
{
  "autoUpdateEnabled": false
}
```

Add:

```http
PATCH /api/client/v1/installed-apps/{packageId}/update-policy
Content-Type: application/json

{
  "autoUpdateEnabled": false
}
```

The endpoint validates a non-empty package ID, normalizes it for persistence, upserts the user-scoped policy, and returns the effective policy. It does not require the application to remain installed, which keeps the preference stable across reinstallations.

Errors use the existing JSON error contract:

- `400 INVALID_PACKAGE_ID` for an empty path value.
- `400 INVALID_JSON` for an invalid body.
- `200` for a successful idempotent upsert.

## Update Queue Semantics

Extend the internal queue options with `RespectAutoUpdatePolicy bool`.

- Scheduled automatic updates call the queue with `RespectAutoUpdatePolicy=true`.
- Manual `POST /api/client/v1/updates/run` calls it with `false`.
- Candidate calculation receives the disabled package-ID set and excludes matching applications only when policy filtering is enabled.
- Existing version, source, password, and installability checks remain unchanged.

## Client State and Components

- Add `autoUpdateEnabled?: boolean` to `InstalledApplication`; undefined is interpreted as enabled for backward compatibility.
- `InstalledAppsView` renders a switch for source-managed installed applications.
- Saving a switch is optimistic per application, disables repeated input while the request is active, and rolls back with an error toast if the request fails.
- Local/unmatched applications do not show the switch because the update queue cannot manage them.
- The bulk confirmation list displays a supporting badge for applications whose automatic updates are disabled, without excluding them.

## Failure Handling

- A policy write failure restores the previous switch state and shows the server error.
- A policy-read failure is treated as enabled only when the installed-app request itself succeeds without the optional field, preserving compatibility with older servers.
- Failure to load policies on the server fails `GET /installed`; silently enabling scheduled updates would violate the stored user preference.

## Testing

- Schema migration and unique user/package behavior.
- Installed-app response merges enabled-by-default and explicitly disabled policies.
- Policy endpoint is user-scoped and idempotent.
- Scheduled queue excludes disabled applications.
- Manual queue includes disabled applications.
- Existing protected-app exclusion remains intact.
- Client helper treats missing `autoUpdateEnabled` as enabled.
- Client production build and full Go tests remain green.
