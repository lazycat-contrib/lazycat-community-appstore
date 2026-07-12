# Installed App Card Compact Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent installed-app cards from overlapping while presenting automatic-update controls and LazyCat runtime status compactly.

**Architecture:** Add pure presentation helpers beside the existing client UX helpers, consume them from `InstalledAppsView`, and constrain the control through existing ASTRYX Switch tooltip properties. CSS only governs compact geometry; raw status interpretation remains testable TypeScript-free JavaScript logic.

**Tech Stack:** React, TypeScript, ASTRYX Design Switch, CSS Grid, Node test runner, Vite.

## Global Constraints

- Keep `lazycat/client/package.yml` at version `0.1.25`.
- Never render raw SDK status values as visible card text.
- Keep the installed-app grid minimum card width at `320px` and allow three title lines.
- Preserve keyboard-accessible labels and tips.

---

### Task 1: Status presentation helper

**Files:**
- Modify: `client/src/modules/client/clientUxState.mjs`
- Modify: `client/src/modules/client/clientUxState.test.mjs`

**Interfaces:**
- Produces: `installedRuntimeStatusPresentation(value)` returning `{ key, raw }`.

- [ ] Add tests for `Status_Running`, `Status_Paused`, stopped, processing, failure, and unknown values.
- [ ] Run `node --test client/src/modules/client/clientUxState.test.mjs` and confirm the new tests fail.
- [ ] Implement normalization by lowercasing and removing SDK prefixes/separators.
- [ ] Run the Node tests and confirm all cases pass.

### Task 2: Compact card UI

**Files:**
- Modify: `client/src/modules/client/InstalledAppsView.tsx`
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`
- Modify: `client/src/styles/client.css`
- Modify: `client/src/styles.css`

**Interfaces:**
- Consumes: `installedRuntimeStatusPresentation(value)`.
- Produces: compact status badge, switch label tooltip, and saving-state tooltip.

- [ ] Replace raw runtime text with localized short status labels while retaining the original value in `title`.
- [ ] Remove the persistent switch description and use `labelTooltip` for policy details.
- [ ] Use a compact intrinsic-width update control and prevent the metadata column from forcing card overflow.
- [ ] Add Chinese and English status and saving-tip copy.
- [ ] Run the focused Node tests and `npm run build --prefix client`.

### Task 3: Package and ship

**Files:**
- Regenerate: `clientembed/dist/**`
- Do not modify: `lazycat/client/package.yml`

- [ ] Run `lzc-cli project release -o ../../dist/lazycat-community-appstore-client-0.1.25.lpk` from `lazycat/client`.
- [ ] Inspect the LPK and confirm package version remains `0.1.25`.
- [ ] Run Go tests, race tests, vet, frontend tests, frontend build, and `git diff --check`.
- [ ] Commit implementation and generated assets.
- [ ] Re-read worktree and HEAD, push `main`, and confirm remote HEAD.
