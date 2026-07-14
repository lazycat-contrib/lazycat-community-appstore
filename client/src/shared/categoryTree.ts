import type { Category } from './types';
import { localizedName } from './utils';
import { buildCategoryHierarchy as buildHierarchy } from './categoryHierarchy';

export type CategoryTreeItem = {
  category: Category;
  depth: number;
  label: string;
  path: string;
};

export function buildCategoryHierarchy(categories: Category[]) {
  return buildHierarchy(categories, localizedName);
}

export function flattenCategoryTree(categories: Category[]): CategoryTreeItem[] {
  const { roots, childrenByParent } = buildCategoryHierarchy(categories);

  const output: CategoryTreeItem[] = [];
  const visited = new Set<number>();

  function visit(items: Category[], depth: number, parentPath: string) {
    for (const category of items) {
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
      visit(childrenByParent.get(category.id) || [], depth + 1, path);
    }
  }

  visit(roots, 0, '');
  for (const category of [...categories].sort((left, right) => {
    const sortDelta = (left.sortOrder || 0) - (right.sortOrder || 0);
    return sortDelta || localizedName(left).localeCompare(localizedName(right), undefined, { numeric: true, sensitivity: 'base' });
  })) {
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
