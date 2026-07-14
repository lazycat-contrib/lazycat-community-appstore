import type { Category } from './types';

export type CategoryHierarchy = {
  roots: Category[];
  childrenByParent: Map<number, Category[]>;
  byID: Map<number, Category>;
};

export function buildCategoryHierarchy(
  categories: Category[],
  categoryName: (category: Category) => string,
): CategoryHierarchy {
  const byID = new Map(categories.map((category) => [category.id, category]));
  const roots: Category[] = [];
  const childrenByParent = new Map<number, Category[]>();
  const sorted = (items: Category[]) => [...items].sort((left, right) => {
    const sortDelta = (left.sortOrder || 0) - (right.sortOrder || 0);
    if (sortDelta !== 0) return sortDelta;
    return categoryName(left).localeCompare(categoryName(right), undefined, { numeric: true, sensitivity: 'base' });
  });

  for (const category of categories) {
    const parentID = category.parentId ?? null;
    if (parentID && byID.has(parentID)) {
      childrenByParent.set(parentID, [...(childrenByParent.get(parentID) || []), category]);
    } else {
      roots.push(category);
    }
  }
  for (const [parentID, children] of childrenByParent.entries()) {
    childrenByParent.set(parentID, sorted(children));
  }
  return { roots: sorted(roots), childrenByParent, byID };
}
