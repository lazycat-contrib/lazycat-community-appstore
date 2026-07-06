# Frontend Redesign Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the React frontend into clear server storefront, server backstage, standalone client, and shared modules while applying the agreed app-store interaction model.

**Architecture:** Keep the current React/Vite/Astryx stack and move the 6080-line `client/src/App.tsx` into feature modules with typed boundaries. `App.tsx` remains the shell/container that owns global loading, navigation, theme, site profile, and high-level data refresh; feature modules render specific workflows through props. CSS is token-driven and split by shared shell plus feature surfaces.

**Tech Stack:** React 19, TypeScript, Vite, Astryx Design components, lucide-react, i18next, CSS custom properties.

## Global Constraints

- No historical frontend structure compatibility is required.
- Component splitting starts after the consensus and this plan.
- Design and visual quality take priority, followed by useful motion, operation feedback, ease of use, and stability.
- Public server pages do not display client/device installed-app lists.
- Logged-out server UI uses Login as the backstage entry and does not show "My Apps".
- Standalone client UI focuses on sources, source catalog, installed apps, updates, history, and rollback.
- Prefer Astryx inputs, text areas, selectors, tabs, buttons, badges, and cards. Native file inputs must be hidden behind a styled file-picker surface.
- Theme values must come from CSS variables and work in header, sidebar, cards, fields, modals, and empty states.
- All user-visible copy remains localized in Chinese and English.
- Touch targets are at least 44px, routine motion stays under 240ms, and reduced-motion is respected.
- Browser verification uses desktop and mobile widths, including 320px/375px checks for horizontal overflow.

---

## File Structure

Create or move toward this structure:

- `client/src/App.tsx`: application shell, top-level state, navigation, refresh orchestration, and routing between feature pages.
- `client/src/shared/types.ts`: shared API/domain types currently defined at the top of `App.tsx`.
- `client/src/shared/api.ts`: `api`, `clientApi`, response parsing, and API base constants.
- `client/src/shared/utils.ts`: `cx`, `arrayOrEmpty`, formatting, status keys, version comparison, source/app matching helpers.
- `client/src/shared/constants.ts`: storage keys, source stale duration, recommended mirror preset lists.
- `client/src/shared/theme.tsx`: theme mode helpers and `ThemeToggle`.
- `client/src/shared/components/FilePicker.tsx`: styled file selection surface using a hidden native file input.
- `client/src/shared/components/Feedback.tsx`: toast, empty state, loading state, and install activity panel components if they are split during implementation.
- `client/src/modules/storefront/StorefrontHome.tsx`: public server home.
- `client/src/modules/storefront/StorefrontSearch.tsx`: public app/category/search browsing.
- `client/src/modules/storefront/AppGrid.tsx`: storefront app cards.
- `client/src/modules/storefront/AppDetailPage.tsx`: public app detail/full page replacement for narrow drawer where practical.
- `client/src/modules/client/ClientCatalog.tsx`: source app discovery, source/category filters, update category, bulk update.
- `client/src/modules/client/SourceAppGrid.tsx`: client source app cards.
- `client/src/modules/client/SourcesView.tsx`: source cards, add/edit dialogs, mirror defaults, client comment-display settings.
- `client/src/modules/client/ClientHistoryView.tsx`: install history.
- `client/src/modules/client/InstalledAppsView.tsx`: installed inventory rows/cards with source attribution.
- `client/src/modules/client/SourceAppDetailPage.tsx`: full-page source app detail with comments, mirrors, versions, and rollback.
- `client/src/modules/admin/MaintainerWorkspace.tsx`: login/profile, owned apps, submit dialog, comments/notifications, tokens, groups.
- `client/src/modules/admin/AdminPanel.tsx`: admin reviews, site settings, taxonomy, collections, users, storage/email.
- `client/src/modules/admin/forms.ts`: small typed form helpers if needed.
- `client/src/locales/zh.ts` and `client/src/locales/en.ts`: keep current exports during this pass; split only if it does not destabilize build.
- `client/src/styles.css`: global tokens, shell, theme, and compatibility imports.
- `client/src/styles/storefront.css`: public storefront styles.
- `client/src/styles/client.css`: standalone client styles.
- `client/src/styles/admin.css`: backstage/admin styles.
- `client/src/styles/components.css`: shared component styles.

The first implementation pass may keep a few complex components in `App.tsx` if moving them would require unrelated API work, but every moved module must have one clear owner and typed props.

## Task 1: Shared Types, API, Utilities, And Theme Helpers

**Files:**
- Create: `client/src/shared/types.ts`
- Create: `client/src/shared/api.ts`
- Create: `client/src/shared/utils.ts`
- Create: `client/src/shared/constants.ts`
- Create: `client/src/shared/theme.tsx`
- Modify: `client/src/App.tsx`

**Interfaces:**
- Produces `api<T>(path: string, options?: RequestInit): Promise<T>`.
- Produces `clientApi<T>(path: string, options?: RequestInit): Promise<T>`.
- Produces shared domain types including `StoreApp`, `SourceApp`, `SourceSubscription`, `InstalledApplication`, `InstallHistoryEntry`, `SiteProfile`, `Toast`, `ThemeMode`, and `ResolvedTheme`.
- Produces utility functions used by storefront/client/admin modules.

**Steps:**

- [ ] Move domain type definitions from `App.tsx` into `client/src/shared/types.ts` and export them.
- [ ] Move mirror preset arrays, storage keys, and `SOURCE_STALE_MS` into `client/src/shared/constants.ts`.
- [ ] Move `readResponseJSON`, `api`, `clientApi`, and `CLIENT_API_BASE` into `client/src/shared/api.ts`.
- [ ] Move formatting, class-name, app/source matching, mirror, version, status, and localization helpers into `client/src/shared/utils.ts`.
- [ ] Move `readThemeMode`, `readSystemTheme`, `nextThemeMode`, and `ThemeToggle` into `client/src/shared/theme.tsx`.
- [ ] Update imports in `App.tsx`.
- [ ] Run `npm run build --prefix client`.
- [ ] Fix TypeScript import/export errors before continuing.

## Task 2: Shared UI Components And Styled File Picker

**Files:**
- Create: `client/src/shared/components/FilePicker.tsx`
- Create: `client/src/shared/components/Feedback.tsx`
- Modify: `client/src/App.tsx`
- Modify: `client/src/styles/components.css`
- Modify: `client/src/styles.css`

**Interfaces:**
- Produces `FilePicker` props: `label`, `fileName`, `accept`, `inputRef`, `onChange`, optional `disabled`.
- Produces `ToastMessage`, `EmptyState`, and `LoadingPanel` components if existing markup can be moved without changing behavior.

**Steps:**

- [ ] Move the existing file-drop markup into `FilePicker` while preserving hidden native input behavior.
- [ ] Replace upload/version/screenshot file inputs with `FilePicker`.
- [ ] Move reusable feedback markup that has no feature-specific data dependency into `Feedback.tsx`.
- [ ] Add component CSS with stable 44px+ targets, press feedback, focus states, and reduced-motion handling.
- [ ] Run `npm run build --prefix client`.

## Task 3: Shell And Navigation Cleanup

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/styles.css`
- Modify: `client/src/styles/components.css`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

**Interfaces:**
- `App.tsx` keeps top-level navigation and passes props to feature modules.
- Server logged-out nav contains Store/Discover/Login, not My Apps.
- Client nav contains Discover/Updates when available/Installed/History/Sources.

**Steps:**

- [ ] Rename nav intent in code so `profile` no longer means both login, my apps, and installed apps. Use derived labels during this pass if changing the tab enum is too disruptive.
- [ ] Remove redundant visible labels such as "服务端商店" and "Server API" from the shell.
- [ ] Ensure theme toggle applies to sidebar and topbar through CSS variables.
- [ ] Add an Updates peer nav/category for the standalone client only when updateable apps exist.
- [ ] Preserve focus-on-route-change and skip link behavior.
- [ ] Run `npm run build --prefix client`.

## Task 4: Server Storefront Modules

**Files:**
- Create: `client/src/modules/storefront/StorefrontHome.tsx`
- Create: `client/src/modules/storefront/StorefrontSearch.tsx`
- Create: `client/src/modules/storefront/AppGrid.tsx`
- Create: `client/src/modules/storefront/AppDetailPage.tsx`
- Modify: `client/src/App.tsx`
- Modify: `client/src/styles/storefront.css`
- Modify: `client/src/styles.css`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

**Interfaces:**
- `StorefrontHome` consumes site profile, store apps, categories, collections, source feed copy handlers, and login/submit actions.
- `StorefrontSearch` consumes filtered apps, filters, sort state, and `onOpen`.
- `AppGrid` renders public app cards with Download/Details language, not Install.
- `AppDetailPage` renders the selected server app as a full page or full-width detail area.

**Steps:**

- [ ] Move public home markup out of `App.tsx` into `StorefrontHome`.
- [ ] Move public search/list markup and storefront app cards into `StorefrontSearch` and `AppGrid`.
- [ ] Replace public app-card primary copy with Download/Details.
- [ ] Ensure source subscription instructions are visible from home without crowding app lists.
- [ ] Make login a quiet public entry and keep submit/admin actions behind authentication.
- [ ] Run `npm run build --prefix client`.

## Task 5: Standalone Client Modules

**Files:**
- Create: `client/src/modules/client/ClientCatalog.tsx`
- Create: `client/src/modules/client/SourceAppGrid.tsx`
- Create: `client/src/modules/client/SourcesView.tsx`
- Create: `client/src/modules/client/ClientHistoryView.tsx`
- Create: `client/src/modules/client/InstalledAppsView.tsx`
- Create: `client/src/modules/client/SourceAppDetailPage.tsx`
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/client/sourceAppFilters.ts`
- Modify: `client/src/styles/client.css`
- Modify: `client/src/styles.css`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

**Interfaces:**
- `ClientCatalog` consumes sources, source apps, installed apps, filters, query, and install callbacks.
- `SourcesView` keeps add/edit/delete/sync behavior and renders compact source cards.
- `InstalledAppsView` consumes installed apps and source apps, then renders fixed-height inventory rows/cards.
- `SourceAppDetailPage` replaces the narrow drawer for standalone app details and supports versions, comments, mirror selection, and rollback install.

**Steps:**

- [ ] Move source catalog state and markup into `ClientCatalog`.
- [ ] Keep source/category filtering from `sourceAppFilters.ts`; add update filtering as a first-class category.
- [ ] Show the bulk update button only when updateable apps exist.
- [ ] Move source cards and add/edit dialogs into `SourcesView`; make cards compact and expose edit/delete on hover plus touch-accessible actions.
- [ ] Move installed inventory markup into `InstalledAppsView`; clamp long app names to two lines and app IDs to one ellipsis line.
- [ ] Move history markup into `ClientHistoryView`.
- [ ] Replace standalone `SourceAppDrawer` with `SourceAppDetailPage` rendered in the main content area or a full-screen mobile-friendly overlay.
- [ ] Run `npm run build --prefix client`.

## Task 6: Server Backstage Modules

**Files:**
- Create: `client/src/modules/admin/MaintainerWorkspace.tsx`
- Modify: `client/src/modules/admin/AdminPanel.tsx`
- Modify: `client/src/modules/admin/CollectionAppPicker.tsx`
- Modify: `client/src/App.tsx`
- Modify: `client/src/styles/admin.css`
- Modify: `client/src/styles.css`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

**Interfaces:**
- `MaintainerWorkspace` consumes user, owned apps, groups, install state, and submission callbacks.
- `AdminPanel` remains the site-admin console but uses clearer task tabs and Astryx controls.
- `CollectionAppPicker` remains the app-selection interface for collections.

**Steps:**

- [ ] Move login/profile/my-apps/upload/groups/token markup into `MaintainerWorkspace`.
- [ ] Convert submit and edit flows to button-triggered dialogs or focused panels instead of default visible forms.
- [ ] Keep app list and submit in one maintainer surface, with submit triggered by a button.
- [ ] Ensure category/tag/collection edit rows use Astryx components or a consistent styled input surface.
- [ ] Hide expire/unexpire actions unless the item has the corresponding state; make the two states mutually exclusive.
- [ ] Keep admin reviews first and split site, taxonomy, collections, users, storage/email into separate tabs.
- [ ] Run `npm run build --prefix client`.

## Task 7: CSS Token And Feature Style Split

**Files:**
- Create: `client/src/styles/storefront.css`
- Create: `client/src/styles/client.css`
- Create: `client/src/styles/admin.css`
- Create: `client/src/styles/components.css`
- Modify: `client/src/styles.css`
- Modify: `client/src/main.tsx` or `client/src/App.tsx`

**Interfaces:**
- `styles.css` owns root tokens, reset, shell, and imports feature CSS files.
- Feature CSS files own only their module selectors.

**Steps:**

- [ ] Add `@import` statements for the new CSS files at the top of `styles.css`.
- [ ] Move feature-specific selectors from `styles.css` into the matching feature CSS file.
- [ ] Keep all colors based on existing CSS variables or new semantic variables declared in `:root` and `[data-theme='dark']`.
- [ ] Add `@media (prefers-reduced-motion: reduce)` rules for list entrance, dialogs, hover transforms, and press feedback.
- [ ] Add `@media (hover: hover) and (pointer: fine)` gates for hover-only edit/delete affordances.
- [ ] Run `npm run build --prefix client`.

## Task 8: Browser Verification And Polish

**Files:**
- Modify frontend files only as required by browser findings.

**Verification Commands:**
- `npm run build --prefix client`
- `go test ./...`
- `git diff --check`

**Browser Checks:**
- Server public home at 1440px, 1024px, 768px, 375px, and 320px.
- Server public app list and app detail.
- Server login/backstage, maintainer app list, submit dialog, admin site settings, admin taxonomy, and collection editor.
- Standalone client Discover, Updates when updateable apps exist, Installed, History, Sources, add/edit source dialogs, and app detail.
- Light and dark themes for shell, sidebar, header, cards, forms, dialogs, and empty states.

**Steps:**

- [ ] Start the Vite dev server with `npm run dev --prefix client -- --host 127.0.0.1`.
- [ ] Use agent-browser screenshots for the browser checks listed above.
- [ ] Fix any overlapping text, horizontal scrolling, non-themed panels, clipped buttons, or broken interaction states.
- [ ] Re-run the verification commands.
- [ ] Commit the completed refactor.
- [ ] Push `main` to origin.

## Acceptance Criteria

- `client/src/App.tsx` is no longer the owner of all feature UI; storefront, admin, and client modules exist and are imported.
- Component split follows the consensus boundaries and does not create generic abstractions before real reuse exists.
- Public server UI focuses on app discovery, category browsing, app detail, and source usage.
- Server logged-out UI uses Login as the backstage entry and hides My Apps.
- Server backstage is task-tabbed and avoids one long mixed page.
- Standalone client supports source/category filtering, update category, update badges, bulk update when available, source cards, installed source attribution, history, and rollback/detail flows.
- Forms use Astryx components where possible and file inputs use a styled picker surface.
- Theme switching affects all major surfaces.
- Mobile widths have no horizontal overflow or text overlap.
- Build, Go tests, diff check, and browser verification pass before commit/push.
