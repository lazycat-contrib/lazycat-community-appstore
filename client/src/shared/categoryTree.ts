import type { Category } from './types';
import { localizedName } from './utils';

export type CategoryTreeItem = {
  category: Category;
  depth: number;
  label: string;
  path: string;
};

function categoryParentID(category: Category) {
  return category.parentId ?? null;
}

function sortCategories(items: Category[]) {
  return [...items].sort((left, right) => {
    const sortDelta = (left.sortOrder || 0) - (right.sortOrder || 0);
    if (sortDelta !== 0) return sortDelta;
    return localizedName(left).localeCompare(localizedName(right), undefined, { numeric: true, sensitivity: 'base' });
  });
}

export function flattenCategoryTree(categories: Category[]): CategoryTreeItem[] {
  const byID = new Map(categories.map((category) => [category.id, category]));
  const childrenByParent = new Map<number | null, Category[]>();
  for (const category of categories) {
    const parentID = categoryParentID(category);
    const parentKey = parentID && byID.has(parentID) ? parentID : null;
    childrenByParent.set(parentKey, [...(childrenByParent.get(parentKey) || []), category]);
  }
  for (const [parentID, children] of childrenByParent.entries()) {
    childrenByParent.set(parentID, sortCategories(children));
  }

  const output: CategoryTreeItem[] = [];
  const visited = new Set<number>();

  function visit(parentID: number | null, depth: number, parentPath: string) {
    for (const category of childrenByParent.get(parentID) || []) {
      if (visited.has(category.id)) continue;
      visited.add(category.id);
      const name = localizedName(category);
      const path = parentPath ? `${parentPath} / ${name}` : name;
      output.push({
        category,
        depth,
        label: `${'  '.repeat(depth)}${path}`,
        path,
      });
      visit(category.id, depth + 1, path);
    }
  }

  visit(null, 0, '');
  for (const category of sortCategories(categories)) {
    if (!visited.has(category.id)) {
      const name = localizedName(category);
      output.push({ category, depth: 0, label: name, path: name });
    }
  }
  return output;
}

export function categoryPathLabel(categories: Category[], category: Category) {
  return flattenCategoryTree(categories).find((item) => item.category.id === category.id)?.path || localizedName(category);
}

export function categoryDescendantIds(categories: Category[], categoryID: number) {
  const childrenByParent = new Map<number, Category[]>();
  for (const category of categories) {
    if (!category.parentId) continue;
    childrenByParent.set(category.parentId, [...(childrenByParent.get(category.parentId) || []), category]);
  }
  const ids = new Set<number>();
  function visit(parentID: number) {
    for (const child of childrenByParent.get(parentID) || []) {
      if (ids.has(child.id)) continue;
      ids.add(child.id);
      visit(child.id);
    }
  }
  visit(categoryID);
  return ids;
}
