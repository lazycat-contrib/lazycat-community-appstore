import type { Category } from '../../shared/types';

export type BrowserCategory = Category & { value?: string };

export type CategoryRailItem = {
  category: BrowserCategory;
  label: string;
};

export function categoryValue(category: BrowserCategory) {
  return category.value || String(category.id);
}

export function categoryBrowserState(
  categories: BrowserCategory[],
  activeCategory: string,
  categoryName: (category: BrowserCategory) => string,
) {
  const hierarchy = buildBrowserCategoryHierarchy(categories, categoryName);
  const roots = hierarchy.roots;
  const activeRecord = activeCategory === 'all'
    ? undefined
    : categories.find((category) => categoryValue(category) === activeCategory);
  const selectedParentID = activeRecord?.parentId || activeRecord?.id || null;
  const selectedParent = selectedParentID
    ? hierarchy.byID.get(selectedParentID) as BrowserCategory | undefined
    : undefined;
  const parentValue = selectedParent ? categoryValue(selectedParent) : 'all';
  const railItems: CategoryRailItem[] = [];

  if (selectedParent) {
    for (const category of hierarchy.childrenByParent.get(selectedParent.id) || []) {
      const browserCategory = category as BrowserCategory;
      railItems.push({ category: browserCategory, label: categoryName(browserCategory) });
    }
  } else {
    for (const parent of roots) {
      for (const category of hierarchy.childrenByParent.get(parent.id) || []) {
        const browserCategory = category as BrowserCategory;
        railItems.push({
          category: browserCategory,
          label: `${categoryName(parent)} / ${categoryName(browserCategory)}`,
        });
      }
    }
  }

  return { roots, selectedParent, parentValue, railItems };
}

function buildBrowserCategoryHierarchy(
  categories: BrowserCategory[],
  categoryName: (category: BrowserCategory) => string,
) {
  const byID = new Map(categories.map((category) => [category.id, category]));
  const roots: BrowserCategory[] = [];
  const childrenByParent = new Map<number, BrowserCategory[]>();
  const sorted = (items: BrowserCategory[]) => [...items].sort((left, right) => {
    const order = (left.sortOrder || 0) - (right.sortOrder || 0);
    return order || categoryName(left).localeCompare(categoryName(right), undefined, { numeric: true, sensitivity: 'base' });
  });

  for (const category of categories) {
    if (category.parentId && byID.has(category.parentId)) {
      childrenByParent.set(category.parentId, [...(childrenByParent.get(category.parentId) || []), category]);
    } else {
      roots.push(category);
    }
  }
  for (const [parentID, children] of childrenByParent.entries()) {
    childrenByParent.set(parentID, sorted(children));
  }
  return { roots: sorted(roots), childrenByParent, byID };
}
