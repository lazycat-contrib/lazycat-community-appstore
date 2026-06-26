# LazyCat Community App Store Design

## Thesis

This project should be built as a self-hosted application distribution control plane, not as a simple LPK upload dashboard. The durable center is the release lifecycle: publisher submits an LPK, the server verifies and stores it, a reviewer approves it, a source feed exposes it, and a client installs it.

Confidence: medium-high. The product shape is clear from the requirements, but the LazyCat official installer SDK API still needs confirmation before the client install adapter can be final.

## High-Level Direction

The clean target is a monorepo with two independently deployed applications: one Go server and one browser client:

- Go backend exposes REST APIs, source feed endpoints, and file downloads. It does not own the client runtime.
- The browser client is a separate app with its own dev server, build output, and API base URL configuration.
- ent owns the data model and supports SQLite, PostgreSQL, and MySQL through `DB_DRIVER` and `DB_DSN`.
- Storage is abstracted behind a backend interface. The first implementation is local disk; WebDAV, S3, and GitHub release links plug into the same version model.
- The client is an app-store style web app with source subscriptions, browsing, search, details, account actions, and an install adapter.

## Non-Negotiable Principles

- One release lifecycle: every app version has one status model, whether it was uploaded by the UI or CI.
- Authorization lives on the server. The client may hide actions, but the API enforces ownership, collaborator, software admin, and site admin checks.
- Database portability is designed in from the first migration. Core data is relational, IDs are server-generated, and app logic does not rely on database-specific SQL.
- Files and metadata are separate. Versions reference stored files or external download URLs, which enables retention cleanup and mirror rewriting without changing app records.
- First version proves the distribution loop before polishing secondary governance features.

## Kill List

- Do not build separate "server app store" and "client app store" data models.
- Do not special-case GitHub releases as apps. GitHub is a source type for an app version.
- Do not put approval state only on the app. App metadata changes and app versions both need review records.
- Do not defer PostgreSQL/MySQL until after SQLite works. The configuration and schema must support all three immediately.
- Do not make the install SDK a hard dependency of the first client. Use an adapter so UI and source behavior can be tested before the SDK is wired.

## Phased Plan

### Phase 1: Full-Stack Distribution Loop

Build a Go service with ent schemas, database bootstrap, local storage, auth, app/version APIs, review approval, source feed output, and file downloads. Build a separate responsive web client that talks to the server API. This phase is independently useful for a single self-hosted store.

### Phase 2: Collaboration And Visibility

Add collaborator application approval, user groups, private app visibility checks, outdated marks visible to maintainers, and richer owner dashboards.

### Phase 3: Storage And Source Hardening

Add WebDAV, S3, GitHub release link storage, source password rotation, mirror replacement policies, version retention cleanup, and source password UX.

### Phase 4: CI/CD And Admin Operations

Add API token management, CI upload endpoints, audit logs, OpenAPI docs, admin settings UI, Docker packaging, and release migration checks.

## First Proof Point

The first proof point is a running local store where an admin can log in, upload an `.lpk`, approve it, see it in `/source/v1/index.json`, and browse it from the web client.

## Falsifier

The thesis is wrong if the LazyCat client installation flow cannot consume a generic source feed and requires a completely different package discovery contract. If that happens, the source feed becomes an adapter target instead of the primary public interface.

## Payoff Ledger

| Move | Price Paid Now | Unlock |
| --- | --- | --- |
| Build around release lifecycle | More schema and status work in Phase 1 | UI upload, CI upload, review, retention, and source feed all reuse one model |
| Support SQLite/PostgreSQL/MySQL from day one | More integration configuration and tests | Private single-node and formal multi-user deployments share the same code |
| Abstract storage immediately | Slightly more code than direct local writes | WebDAV, S3, and GitHub release links do not rewrite app/version logic |
| Separate source feed from admin APIs | Two API surfaces to maintain | Clients can subscribe without inheriting admin API behavior |
| Use installer adapter | Initial install button is simulated until SDK binding is known | Client screens can be built and tested before SDK details are finalized |
