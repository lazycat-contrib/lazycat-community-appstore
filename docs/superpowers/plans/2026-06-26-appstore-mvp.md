# LazyCat Community App Store MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable self-hosted LPK app store as two applications: API server and web client.

**Architecture:** A Go service serves REST APIs, source feed endpoints, and local file downloads. A separate client app runs its own dev server and calls the backend through `VITE_API_BASE_URL`. ent owns relational persistence with SQLite, PostgreSQL, and MySQL selected by environment variables.

**Tech Stack:** Go 1.26, ent, standard `net/http`, bcrypt, local disk storage, vanilla HTML/CSS/JS client.

## Global Constraints

- `DB_DRIVER` supports `sqlite3`, `postgres`, and `mysql`.
- `DB_DSN` selects the database DSN and defaults to `./data/store.db`.
- `SITE_MAX_LPK_SIZE` defaults to `524288000`.
- `SITE_MAX_VERSIONS` defaults to `10`, where `0` keeps every version.
- Only `.lpk` files are accepted for uploaded packages.
- Uploaded files get SHA256 hashes stored on the version record.
- Initial admin is created from `ADMIN_USERNAME` and `ADMIN_PASSWORD`.

---

## Task 1: Scaffold Core Backend Modules

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/password.go`
- Create: `internal/storage/local_test.go`
- Create: `internal/storage/storage.go`
- Create: `internal/storage/local.go`
- Create: `internal/feed/feed_test.go`
- Create: `internal/feed/feed.go`

**Interfaces:**
- Produces `config.Load() Config`.
- Produces `auth.HashPassword(string) (string, error)` and `auth.CheckPassword(hash, password string) bool`.
- Produces `storage.Backend` and `storage.SaveLPK(ctx, backend, reader, filename, maxBytes)`.
- Produces `feed.BuildIndex(feed.Input) feed.Index`.

## Task 2: Add ent Data Model

**Files:**
- Create: `ent/schema/*.go`
- Generate: `ent/*`

**Interfaces:**
- Produces generated ent clients for users, apps, versions, reviews, categories, tags, comments, favorites, groups, collaborators, outdated marks, API tokens, and settings.

## Task 3: Implement Server Bootstrap

**Files:**
- Create: `cmd/store-server/main.go`
- Create: `internal/server/server.go`
- Create: `internal/server/auth.go`
- Create: `internal/server/respond.go`
- Create: `internal/server/bootstrap.go`

**Interfaces:**
- Produces `server.New(config.Config) (*Server, error)`.
- Produces `(*Server).Handler() http.Handler`.

## Task 4: Implement REST API

**Files:**
- Create: `internal/server/handlers_auth.go`
- Create: `internal/server/handlers_apps.go`
- Create: `internal/server/handlers_admin.go`
- Create: `internal/server/handlers_social.go`
- Create: `internal/server/handlers_source.go`

**Interfaces:**
- Produces auth, app, version, review, comment, favorite, outdated mark, category, tag, source feed, and file download endpoints.

## Task 5: Build Web Client

**Files:**
- Create: `client/package.json`
- Create: `client/src/App.tsx`
- Create: `client/src/main.tsx`
- Create: `client/src/styles.css`

**Interfaces:**
- Produces a responsive app-store style interface for browsing, source subscriptions, upload, review, comments, favorites, and simulated install progress.

## Task 6: Verify

**Commands:**
- `go test ./...`
- `go run ./cmd/store-server`
- `cd client && npm run dev -- --host 127.0.0.1`
- Open `http://127.0.0.1:5173`

**Manual Acceptance:**
- Admin can log in with environment defaults.
- Admin can upload an `.lpk`.
- Uploaded version has a SHA256 value.
- Admin can approve pending review.
- Approved app appears in `/source/v1/index.json`.
- Web client can browse and trigger install flow.
