# Site Branding And Announcements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add site branding, canonical subscription URL, and announcement display/admin editing.

**Architecture:** Reuse `site_settings` for persistence, add typed helper methods in `internal/server/settings.go`, expose a public `siteProfile` API, include the profile in the source feed, then consume it in the React UI. The Admin page remains a single component but groups settings into product sections.

**Tech Stack:** Go HTTP handlers, ent SiteSetting key/value storage, React + TypeScript, CSS transitions, OpenAPI.

## Global Constraints

- Server must not expose device installed-app state.
- Site profile reads are public; writes require `SITE_ADMIN`.
- All URL settings must be HTTP(S) or empty.
- Announcement text is plain text only.

---

### Task 1: Backend Site Profile

**Files:**
- Modify: `internal/server/types.go`
- Modify: `internal/server/settings.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers_admin.go`
- Modify: `internal/server/handlers_source.go`
- Modify: `internal/feed/feed.go`
- Modify: `internal/server/server_test.go`

**Interfaces:**
- Produces: `siteProfile`, `siteAnnouncement`, `(*Server).siteProfile(ctx)`, `(*Server).sitePublicURL(ctx)`, `GET /api/v1/site/profile`.

- [x] Add typed profile DTOs and helper methods.
- [x] Add setting allowlist and validation for branding/announcement keys.
- [x] Add public site profile route.
- [x] Use canonical public URL for source feed base and generated downloads.
- [x] Include site and announcement metadata in feed output.
- [x] Add tests for profile defaults, setting validation, and feed metadata.

### Task 2: Frontend Site Profile And Announcement

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/i18n.ts`
- Modify: `client/src/config.ts`
- Modify: `client/src/styles.css`

**Interfaces:**
- Consumes: `{ site: SiteProfile }` from `/api/v1/site/profile`.
- Produces: branded sidebar, document title, announcement banner, Admin editor sections.

- [x] Add TypeScript profile types and load profile during server refresh.
- [x] Use site title/icon in the shell and Home source URL.
- [x] Add dismissible announcement banner and update toast.
- [x] Rework Admin settings into site identity, announcement center, and policy settings.
- [x] Add responsive CSS using explicit transitions.

### Task 3: Documentation And Verification

**Files:**
- Modify: `docs/openapi.yaml`
- Modify: `README.md`

- [x] Document public site profile and source feed metadata.
- [x] Run Go tests, frontend build, audit, OpenAPI/YAML validation, static binary checks, and diff checks.
- [ ] Commit and push to `origin/main`.
