# Geju Planning Review For LazyCat Community App Store

Date: 2026-06-27

## Purpose

This review audits the existing high-level geju planning against the current product boundary and implementation state. The goal is to keep the useful direction while removing inflated concepts that make the app store harder to ship or easier to mis-model.

The product is a private NAS app store for LazyCat LPK packages. It has two independently deployable bare applications:

- Server: a publishing control plane plus a public storefront and embedded management console.
- Client: a source subscription, browsing, installed-app lookup, and install application.

The client can be deployed without the server. The server can also be deployed without the client.

## Review Thesis

The original direction of "trusted publishing control plane plus multi-source consumption client" is still the right product model. The correction is that every future planning step must be reduced to concrete user tasks, API behavior, and UI states before implementation.

The app store must not introduce unrelated platform concepts. In particular, "installed apps" is not a "device" feature. It is an application management query through the LazyCat SDK, using `pkgm.QueryApplication` to read installed application state.

## Keep

Keep these parts of the existing geju planning because they directly support the app store workflow:

- Independent server and client deliverables.
- Server-side approval, visibility, roles, storage, source feed, and settings.
- Server storefront for browsing approved applications, not only an admin console.
- Client source subscriptions with password, mirror, sync, search, browse, details, and install.
- SHA256 as a trust requirement for LPK install paths.
- LazyCat SDK install adapter through `InstallLPK`, with browser checksum fallback.
- LazyCat SDK installed-app lookup through `PkgManager.QueryApplication`.
- Web setup wizard in addition to environment-variable bootstrap.
- Multilingual UI as a product requirement, not an afterthought.

## Correct

These items need correction because they drift from the product model:

- Replace "device", "this device", and "device readiness" with "installed apps", "installed list", or "current LazyCat client installed list".
- Do not model device management in the app store. The app store installs and tracks applications, not devices.
- Do not make the client profile page a generic account or device dashboard in standalone mode. In standalone mode, it should focus on source health, install readiness, and installed apps.
- Do not hide the server's public storefront behind admin language. A self-hosted store still needs a front-facing app catalog.
- Do not let high-level governance language create UI bulk. Admin workflows should expose pending decisions, risky settings, and next actions only.

## Delay

These ideas may be valid later but should not drive the next implementation batches:

- Large public marketplace growth mechanics.
- Complex editorial publishing and marketing pages.
- Advanced analytics dashboards.
- Multi-device management.
- Heavy audit-log browsing UI before core review and source workflows are stable.
- Fine-grained governance pages that do not yet have clear user actions.

## Product Boundaries

### Server

The server owns the publishing truth:

- Approved apps and versions.
- Review state.
- Roles, collaborators, groups, and visibility.
- Storage and download URLs.
- Source feed policy.
- Initial setup and site settings.

The server UI must have two surfaces:

- Storefront: browse, search, app details, source endpoint visibility, and public trust signals.
- Back office: reviews, submissions, taxonomy, users, settings, groups, and publishing operations.

### Client

The client owns consumption:

- Add and manage software sources.
- Sync source feeds.
- Browse and search source apps.
- Inspect trust fields before install.
- Install LPKs through LazyCat SDK.
- Read installed applications through LazyCat SDK application management.

The client does not own final publication status, private app authorization, or device management.

## Review Of Recent Execution

Recent commits mostly moved in the correct direction by clarifying:

- Source subscription flow.
- App submission readiness.
- Version publishing workflow.
- Admin governance readiness.
- Install trust cues and SDK fallback behavior.

The latest "device readiness" batch is the main execution error. It added useful installed-app readiness behavior, but named it with the wrong concept. The behavior should stay; the device framing should be removed.

## Next Development Batches

The next work should be small, verifiable, and ordered by product correctness.

### Batch 1: Installed Apps Boundary Correction

Goal: remove the device concept from the client.

Scope:

- Rename standalone client nav from "Device" to "Installed".
- Rename UI text from "This device" to "Installed apps".
- Keep LazyCat SDK app query behavior, but document it as application management, not device state.
- Rename internal variables and CSS classes where practical to prevent future drift.
- Verify no visible Chinese or English device wording remains in app-store UI.

Acceptance:

- Standalone client shows source readiness, install readiness, and installed-app readiness.
- The installed-app list is described as data returned by the current LazyCat client SDK environment.
- No UI claims that the app store manages devices.

### Batch 2: Server Storefront Clarity

Goal: make the server front office feel like an app store, not only a management shell.

Scope:

- Clarify storefront navigation and entry states for anonymous users.
- Surface approved apps, categories, collections, app trust fields, and source feed information.
- Keep admin controls available only where relevant.

Acceptance:

- A user opening the server without admin intent sees a catalog first.
- Admin and reviewer workflows remain reachable without dominating the storefront.

### Batch 3: Admin Decision Simplicity

Goal: make back-office work action-oriented.

Scope:

- Review queue should show what changed, risk fields, checksum/source information, and approve/reject actions.
- Settings should emphasize source password, mirror, retention, size, and email verification policies.
- Avoid dashboard decorations that do not change decisions.

Acceptance:

- Admin can identify the next pending decision without reading dense prose.
- Risky fields are visible before approval.

### Batch 4: Client Source And Install Polish

Goal: make standalone client use reliable without server assumptions.

Scope:

- Strengthen source empty, failed, password-required, stale, and synced states.
- Keep install trust cues close to the install action.
- Show installed status when LazyCat SDK returns matching applications.

Acceptance:

- A client-only deployment can add a source, sync, browse, inspect, install, and read installed apps.
- Failure states explain the next action without mentioning server-only features.

## Geju Audit Rules Going Forward

Before implementing any future geju-generated plan, apply this checklist:

1. Does this serve a concrete actor: NAS user, submitter, reviewer, or site admin?
2. Can the change be verified through UI, API, test, or browser smoke?
3. Does the wording match the product boundary?
4. Does it avoid new platform concepts not present in the app store?
5. Can it ship as one small commit with focused verification?
6. Does it reuse existing libraries, SDKs, and code patterns?

If the answer is no, rewrite or delay the idea before implementation.

## Self-Review

- No placeholder sections remain.
- The server and client boundaries are explicit.
- The "installed apps, not devices" correction is stated as a hard product rule.
- The next batches are small enough for implementation planning.
- The document does not require new dependencies or speculative platform features.
