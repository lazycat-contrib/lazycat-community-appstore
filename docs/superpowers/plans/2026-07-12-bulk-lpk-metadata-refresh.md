# Bulk LPK Metadata Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add one-click bulk LPK metadata refresh with clear asynchronous progress to My software.

**Architecture:** Reuse existing inspection jobs through two owner-scoped bulk endpoints. Keep batch state local to `ProfileView`, poll compact status DTOs, and reload the app list after terminal completion.

**Tech Stack:** Go, Ent, React, TypeScript, ASTRYX design components, i18next.

## Global Constraints

- Default to fill-missing mode.
- Never duplicate active jobs.
- Never expose another owner’s applications or jobs.
- Require a confirmation dialog for reparse and delete; do not use decorative list animation.

### Task 1: Owner-scoped bulk inspection API

- [ ] Add failing server tests for enqueue and status authorization.
- [ ] Implement request/response DTOs and handlers in `internal/server/lpk_inspection.go`.
- [ ] Register routes in `internal/server/server.go` and document them in `docs/openapi.yaml`.
- [ ] Run focused server tests.

### Task 2: My software progress interaction

- [ ] Add TypeScript DTOs and localized copy.
- [ ] Add a secondary header action and anchored inline progress panel to `ProfileView.tsx`.
- [ ] Make actions selection-driven, add delete, and require explicit confirmation for both operations.
- [ ] Add an opt-in overwrite-existing-information control to the reparse confirmation.
- [ ] Remove the duplicate single-app inspection UI from application detail.
- [ ] Poll status with bounded intervals, reload authoritative apps on completion, and summarize failures.
- [ ] Add narrowly scoped CSS with reduced-motion handling and a source contract test.
- [ ] Run frontend tests and production build.

### Task 3: Delivery

- [ ] Rebuild `clientembed/dist`.
- [ ] Run `go test ./...`, frontend tests, production build, and `git diff --check`.
- [ ] Commit and fast-forward the feature to main.
