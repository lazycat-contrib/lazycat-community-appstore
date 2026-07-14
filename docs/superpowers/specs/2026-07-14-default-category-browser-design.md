# Default Category Browser Design

## Goal

Show useful child-category navigation when the shared category browser is in its default `all` state.

## Root cause

`client/src/modules/storefront/CategoryBrowser.tsx` deliberately maps `activeCategory === 'all'` to no active record. That leaves `selectedParent` undefined, makes `childCategories` empty, and prevents the conditional subcategory rail from rendering. The screenshot symptom—top-level tabs visible with no default child-category row—follows directly from this state path.

## Behavior

- Keep `All categories` selected while `activeCategory` is `all`; do not silently activate a category or navigate away from the current page.
- In the default state, flatten the children of all root categories into the existing horizontal subcategory rail.
- Prefix default-state child labels with their localized parent name, for example `Artificial Intelligence / Agents`, so duplicate child names remain unambiguous.
- When a root or child category is active, preserve the existing behavior: show only that root's children, including the `All in <root>` action.
- If the hierarchy has no child categories, render only the top-level tabs as today.
- Reuse the existing scrolling controls and styling; no unrelated visual redesign is included.

Because `CategoryBrowser` is shared, the same correction applies to the storefront home, storefront search, and client catalog default states.

## Verification

- A pure exported helper produces the expected default flattened child list and selected-parent child list, allowing a deterministic regression check without browser timing.
- TypeScript checking passes.
- Runtime checklist: with `All categories` active, the all-root child rail is visible; clicking a root narrows the rail; clicking `All categories` restores the all-root rail; clicking a child still invokes the existing category selection callback.
- Sibling sweep covers every `CategoryBrowser` caller so no caller supplies an incompatible category value shape.

The implementation changes only the shared category-browser component and its focused regression guard unless type-checking proves an additional caller adjustment is necessary.
