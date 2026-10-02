# Client and Server UI/UX Polish Implementation Plan

> **For agentic workers:** Implement the tightly coupled shell changes inline using executing-plans. Use requesting-code-review for a fresh whole-change review before release. Steps use checkbox syntax for tracking.

**Goal:** Fix the verified usability regressions and make both app-store surfaces focused, responsive, and consistent, then ship the client and server patch releases.

**Architecture:** Keep React, Vite, Astryx, Lucide, i18next, and the existing APIs. Add small shared shell utilities for URL-backed navigation, unsaved-edit protection, compact appearance controls, and accessible dialog/file input integration. Simplify task pages and edit the existing semantic CSS layers rather than introduce another styling system.

**Tech Stack:** React 19, TypeScript, CSS, Node test runner, Go embedded frontend.

**Spec:** The user-approved UI/UX audit from this session; existing product boundaries in docs/superpowers/specs/2026-07-06-frontend-redesign-consensus.md.

## Global Constraints

- Client subscriptions, installation, and local inventory remain client-owned; server storefront and administration remain server-owned.
- Preserve theme selection, language switching, source subscription, download/install, rollback, groups, upload policy, and existing management functions.
- New visible strings are present in both zh.ts and en.ts.
- Layout works at 320, 375, 768, 1024, and 1440px. Touch controls retain a 44px target.
- Reuse existing easing tokens; routine motion is at most 240ms, keyboard actions do not animate, reduced motion removes movement.
- Do not alter Go backend behavior or existing deployment/store-publication configuration.
- Release versions must match the separate client-v* and server-v* tags. Regenerate the tracked embedded frontend from the final source.

## Review Focus

- Browser Back/Forward, refreshed detail URLs, login returnTo, collaboration invites, and unavailable navigation destinations must remain predictable.
- Source-app identity is unambiguous across subscriptions, and failed detail fetches must recover without stale loading state.
- Dirty settings must survive attempted internal navigation and browser Back; saving must retain existing revision/concurrent-edit protections.
- Nested modals and Escape restore the correct focus without closing an outer layer; optional file inputs remain accessible by keyboard.
- Long English/Chinese names, narrow login controls, empty catalogs, permission-limited users, and dark/reduced-motion states must remain usable.

## Tasks

- [x] Shared usability: route helpers and meaningful behavior tests; URL-backed tabs/detail navigation with scroll restoration; unsaved-edit context and save/discard/stay dialog; shared modal focus restoration and file-picker semantics.
- [x] Shared visual system: compact responsive appearance menu; app-specific fallback artwork; contrast and grid repairs; coherent type, borders, corner radii, pointer feedback, and reduced motion.
- [x] Client pages: bring catalog and installed inventory forward; simplify source forms and settings; consolidate detail actions and hide oversized empty evidence.
- [x] Server pages: bring discovery forward on the home page; reduce card metadata, duplicate search counts and admin summaries; streamline login and detail actions without losing management capabilities.
- [ ] Verification and release: run frontend tests/build, Go verification and repository-required config checks, browser regression and axe checks, independent review, bump both package versions, rebuild embedded assets, commit and push main plus both release tags, verify remote tags and release workflows.

## Progress

- Baseline: 85 frontend tests pass; initial working tree clean at d31889a.
- Execution: isolated changes on fix/ui-ux-polish in the shared checkout; no additional worktree is needed for this clean, user-authorized task.

- Source verification: 96 frontend tests passed; Go tests, vet, race, tidy and dependency audit passed.
- Browser verification in progress: client Back/Forward/deep-link reload, internal dirty navigation, browser Back dirty navigation, and save-failure recovery pass. Both mobile headers now measure 58px; server home first app begins near y=395 at 375px.

- Independent source review completed; submit-login intent and logout guards were corrected and covered by regression checks. A focused re-review found no source blockers.
- Source-dialog keyboard sequence now restores the Add source button after autofocus and Escape; browser detail Back restored the internal shell scroll position to 280px.
- Added two login-destination tests, bringing the frontend suite to 98 tests. CI now runs npm test.

- Final production preview checks: default Neutral light/dark sources and server site settings have 0 confirmed WCAG A/AA violations; incomplete axe items remain manual checks. Login controls fit 320px; client catalog has three fluid columns at 1440px; source Retry text and failed-row actions fit at 320px.
- Both final Go binaries start on fresh isolated databases, serve their embedded frontend, and expose the expected runtime versions (client 0.1.46/server 0.1.53). Client runtime config was generated using the existing client build convention; the tracked shared config was restored to neutral afterward.
- Verification: frontend suite 98/98, TypeScript/Vite build, Go test/vet/race/tidy/lint, npm audit (0 vulnerabilities), OpenAPI, LazyCat YAML and actionlint passed. The tracked embedded frontend matches the Vite output (excluding runtime config as CI specifies).

- Direct installed-inventory URL regression fixed: profile also loads the source catalog, restoring source attribution and update matching without visiting the catalog first. Added a regression test; frontend suite is 99/99. Independent focused review approved this fix and the final failed-source card layout fix.

- Local release gate complete: independent source review approved; client/server patch versions match their intended tags; all required local checks passed; final embedded assets synchronized. Commit, main fast-forward, tag push and remote workflow observation follow this source snapshot.
