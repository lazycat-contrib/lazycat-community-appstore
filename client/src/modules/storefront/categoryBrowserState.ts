import type { Category } from '../../shared/types';
import { buildCategoryHierarchy } from '../../shared/categoryHierarchy.ts';

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
  const hierarchy = buildCategoryHierarchy(categories, (category) => categoryName(category as BrowserCategory));
  const roots = hierarchy.roots as BrowserCategory[];
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
