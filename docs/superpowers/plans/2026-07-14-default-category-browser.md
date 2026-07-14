# Default Category Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render child-category navigation in the shared category browser while `All categories` is the active default.

**Architecture:** Move the rail-selection calculation into a pure helper. The default state flattens all root children with parent-prefixed labels while keeping the `all` toggle selected; an active root/child preserves the existing scoped rail.

**Tech Stack:** React 19, TypeScript 5.9, Node 26 built-in test runner, ASTRYX Design.

## Global Constraints

- Do not auto-select a category or navigate when categories load.
- Keep `All categories` selected in the default state.
- Default child labels use localized `<parent> / <child>` text.
- Preserve the current selected-parent `All in <parent>` behavior.
- Reuse existing rail controls and styles.
- Apply the behavior to all shared `CategoryBrowser` callers.
- Do not run or commit a generated frontend bundle.

---

### Task 1: Add a failing category-rail state regression test

**Files:**
- Create: `client/src/modules/storefront/categoryBrowserState.ts`
- Create: `client/src/modules/storefront/categoryBrowserState.test.mjs`

**Interfaces:**
- Produce `categoryBrowserState(categories, activeCategory, localizedName)` returning `parentValue`, `selectedParent`, and labeled `railItems`.

- [ ] Write Node tests for default all-root flattening, parent-prefixed labels, active-root scoping, active-child parent resolution, and hierarchies without children.
- [ ] Run `node --test client/src/modules/storefront/categoryBrowserState.test.mjs` and confirm failure because the helper does not yet exist.
- [ ] Implement the pure helper with deterministic root/child order from `buildCategoryHierarchy`.
- [ ] Re-run the Node test and confirm all cases pass.

### Task 2: Render the default rail through the shared component

**Files:**
- Modify: `client/src/modules/storefront/CategoryBrowser.tsx`
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`

**Interfaces:**
- `CategoryBrowser` consumes `categoryBrowserState` and renders `railItems`; `selectedParent` controls whether the leading `All in <parent>` button is shown.

- [ ] Add a source contract assertion proving the component uses `categoryBrowserState` and no longer gates the rail solely on `selectedParent`.
- [ ] Replace the inline active/parent/children calculations with the helper result.
- [ ] Render the rail whenever `railItems.length > 0`, preserving arrow controls, callbacks, active variants, and ARIA labels.
- [ ] Sweep callers in `StorefrontHome.tsx`, `StorefrontSearch.tsx`, and `ClientCatalog.tsx`; adjust none unless TypeScript finds a value-shape mismatch.
- [ ] Run `node --test client/src/modules/storefront/categoryBrowserState.test.mjs client/src/modules/storefront/storefront.contract.test.mjs`.
- [ ] Run `npm exec --prefix client -- tsc -b client/tsconfig.json` without invoking Vite or regenerating `clientembed/dist`.
- [ ] Commit with `fix: show default category browser children`.
