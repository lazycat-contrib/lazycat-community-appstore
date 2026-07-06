# Storefront Frontend Redesign Design

## Goal

Redesign the server and standalone client frontends around task ownership. The server web entry should help anonymous users discover software, browse categories, and understand how to use the source feed. Login should lead to separate backstage areas for "My Apps" and "Site Admin". The standalone client should be a source-driven installer focused on software sources, source categories, source attribution, install history, and version rollback.

## Current Problems

- Storefront, publishing, account, groups, favorites, source management, installed apps, and admin operations currently compete for attention.
- Many pages expose metrics and readiness panels before the user has chosen a task.
- The server profile page mixes app submission, token management, group management, favorites, and account state.
- The standalone client installed-app page uses oversized cards for operational data.
- Known visual bug: the installed-app grid stretches every card in a row to the tallest card. Long app names and app IDs can wrap into one-character vertical columns, which inflates card height and breaks scanability. The fix should constrain card height, prevent arbitrary word breaking in app identity fields, and use truncation or two-line clamping.

## Reference Principles

Apple App Store product page guidance emphasizes that app name, icon, subtitle, previews, screenshots, and the first visible description should help users quickly understand the app and decide whether to download it:

- https://developer.apple.com/app-store/product-page/

Google Play guidance similarly treats the store listing as the first trust and discovery surface, recommends concise descriptions, quality screenshots, and layouts that scale across screen sizes:

- https://support.google.com/googleplay/android-developer/answer/13393723
- https://support.google.com/googleplay/android-developer/answer/4448378

For this project, those ideas translate to:

- Put icon, name, publisher/source, concise summary, install status, and primary action first.
- Move technical metadata behind detail views.
- Treat screenshots and app visuals as product proof, not decoration.
- Keep management tools out of the consumer browsing flow.
- Preserve trust signals such as checksum, version, source, and install password, but show them as compact badges or a detail section.

## Information Architecture

### Server App

Server frontend has three major areas with different mental models:

- Storefront: public software list, category browsing, app detail, featured collections, and "how to use this store" subscription instructions.
- My Apps: logged-in maintainer workspace for submission, owned app maintenance, versions, visibility groups, and API tokens.
- Admin: logged-in site governance workspace for review queue, site identity, announcements, taxonomy, collections, users, and storage/source policies.

The Storefront is consumer-facing. My Apps and Admin are backstage workspaces reached through login. Admin entry should be present but visually quiet on the public storefront.

### Standalone Client

Standalone client has five source-first task areas:

- Sources: add, sync, repair, and inspect source health.
- Source Catalog: browse apps grouped and filtered by software source and source-provided categories.
- Installed: current-device installed apps with clear source attribution when known.
- History: install attempts, successful installs, failed installs, and versions installed through this client.
- Versions: available versions for a selected source app, with rollback when an older compatible version is present in the synced source.

Installed apps must never look like a server store catalog. It is a device inventory surface: compact rows, clear status, source attribution, and no server-side persistence.

If the LazyCat runtime cannot report historical installs, the client records history for installations initiated through this app. Apps already installed before the client saw them should be marked as existing local apps with unknown source/history instead of inventing attribution.

## Screen Design

### Store Home

Use a marketplace layout:

- A simple first viewport with site title, category/search entry, and source subscription instructions.
- A visible category strip or rail so users can start from type of software, not operational status.
- Responsive sections for featured collections, recently updated, and popular apps.
- App cards show icon, name, one-line summary, category, version, and detail/open action.
- No dense readiness grids on the first viewport.

Use instructions should be a dedicated panel or page section:

- Subscription URL.
- Copy action.
- Short "how to add this store in the client" steps.
- Optional source password hint if the site is protected, without exposing the password.

### Discover/Search

Use a browsing layout:

- Search input and category chips remain visible near the top.
- Results use compact product cards or rows.
- Filters are progressive: category, source, installable, installed, status.
- Empty states explain one next action only.

### App Detail

Use an App Store-style product page:

- Header: icon, app name, publisher/source, short summary, install button.
- Snapshot row: version, size, checksum, install password, source.
- Screenshot strip if screenshots exist.
- Description and changelog below the decision area.
- Maintainer actions only appear in a separate manage section for users with permission.

### Maintainer Workspace

Use task tabs:

- Overview: owned apps and review status summary.
- Submit: a focused publish wizard with app identity, artifact, review path.
- Apps: owned app maintenance, versions, screenshots, visibility, comments.
- Tokens: CI/API token management.
- Groups: visibility group management.

Only one task tab is visible at a time.

### Admin Workspace

Use a backstage console:

- Reviews first, because this is the highest-frequency admin task.
- Site identity and announcements in a dedicated settings tab.
- Taxonomy and collections in separate tabs.
- Users and roles in a separate tab.
- Storage/source policy in a separate tab when object storage is implemented.

Avoid showing every admin tool on one long page.

### Standalone Installed Apps

Use a device inventory layout:

- Header with current-device scope and a refresh action.
- Summary counts can stay compact, but not dominate the page.
- Installed apps render as fixed-height list rows or compact cards.
- App name is clamped to two lines; app ID is one line with ellipsis; source and version are compact chips.
- Grid rows must not stretch to the tallest card.

### Client Source Catalog

Use a source-first marketplace layout:

- Top-level source selector: all sources, or a single source.
- Category chips generated from the selected source apps.
- App rows/cards show source name, source category, current installed version if known, available version, and install/update/rollback action.
- Trust metadata such as checksum and size stays compact and close to the install action.

### Client Install History And Rollback

Use an event log plus per-app version panel:

- History rows show app, source, version, result, timestamp, and error when failed.
- Version rollback appears only when the selected app has older installable versions in the source feed.
- Rollback is an install action with a different target version, not a destructive hidden operation.
- If source metadata no longer includes the old version, show the historical record but disable rollback.

## Interaction Rules

- One primary action per screen: install, submit, sync, approve, or save.
- Technical metadata is visible near the action only when it affects trust or completion.
- Repeated interactions should be immediate; transitions stay under 200ms and use opacity, transform, border color, and background color.
- Hover states must be gated behind pointer/hover media queries.
- Keyboard and screen reader access must remain intact for all tabs, buttons, forms, and dialogs.
- Public server pages should not show device-installed app state.
- Client pages should not show server-side review/admin concepts.
- Login is a boundary between storefront and backstage, not just a small widget inside a mixed page.

## Implementation Notes

- Keep the existing React/Vite stack and API contracts.
- Do not introduce new UI dependencies unless a component becomes too expensive to maintain.
- Prefer small focused components inside `client/src/App.tsx` first, then split only if the file becomes harder to reason about.
- Reuse existing color tokens and radius values; avoid a wholesale theme rewrite.
- Fix the installed-app layout as part of the standalone client redesign, not as an isolated visual patch.

## Implemented Backend And Data Decisions

- LPK metadata support targets V2 packages only. A valid uploaded or URL-provided `.lpk` is treated as a zip/tar container with `package.yml`; V1-only or missing package metadata is rejected.
- `packageId` is the LazyCat package identity from `package.yml.package`. It is separate from `slug`: `packageId` drives source sync, install matching, installed attribution, and history; `slug` stays a storefront route/display identifier.
- JSON and form fields override LPK metadata. Missing package ID, name, summary, description, version, size, and SHA256 can be filled by parsing an uploaded V2 LPK or by fetching a reachable external LPK URL.
- External URL inspection has a timeout, max-size guard, redirect scheme validation, SHA256 calculation, and private/local host rejection by default.
- Source feed apps include `packageId` and a `versions` array so the standalone client can present older versions and perform rollback by installing a selected version.
- The standalone client stores install history in SQLite for installs initiated through this app. Each event records source, cached source app, package ID, app name, version, result, download URL, SHA256, error, and timestamp.
- The server storefront does not query or display device-installed applications. Device inventory remains local to the standalone client.

## Verification

- `cd client && npm run build`
- `CGO_ENABLED=0 go test ./...`
- `npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml`
- Browser verification at desktop and mobile widths for:
  - Server Store home and app detail.
  - Maintainer workspace.
  - Admin workspace.
  - Standalone Discover, Sources, and Installed pages.
- Installed-app regression check: long app names and app IDs must not produce vertical text columns or row-height stretching.
