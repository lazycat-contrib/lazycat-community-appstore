import { localizedCategory } from '../../shared/utils';
import type { Category, SourceID, SourceSubscription } from '../../shared/types';
import { localizedName } from '../../shared/utils';

export type SourceAppFilterOption = {
  key: string;
  label: string;
  count: number;
};

export type SourceScopedApp = {
  sourceId?: SourceID;
  sourceName: string;
  categoryId?: number;
  category?: string;
  categoryI18n?: Record<string, string>;
};

export type SourceScopedCategory = Category & {
  value: string;
  sourceKey: string;
  sourceName: string;
  originalId: number;
};

export type SourceCategoryFilterContext = {
  categories: SourceScopedCategory[];
  selectedValues: Map<string, Set<string>>;
};

const UNCATEGORIZED_CATEGORY_KEY = '__uncategorized__';

export function sourceFilterKey(app: SourceScopedApp) {
  return app.sourceId !== undefined && app.sourceId !== null ? String(app.sourceId) : `name:${app.sourceName}`;
}

export function sourceAppSourceOptions(apps: SourceScopedApp[]): SourceAppFilterOption[] {
  const options = new Map<string, SourceAppFilterOption>();
  for (const app of apps) {
    const key = sourceFilterKey(app);
    const current = options.get(key);
    if (current) {
      current.count += 1;
    } else {
      options.set(key, { key, label: app.sourceName || String(app.sourceId || ''), count: 1 });
    }
  }
  return Array.from(options.values()).sort((a, b) => a.label.localeCompare(b.label));
}

export function sourceSubscriptionFilterKey(source: Pick<SourceSubscription, 'id' | 'name'>) {
  return source.id !== undefined && source.id !== null ? String(source.id) : `name:${source.name}`;
}

export function sourceAppCategoryOptions(apps: SourceScopedApp[], uncategorizedLabel: string): SourceAppFilterOption[] {
  const options = new Map<string, SourceAppFilterOption>();
  for (const app of apps) {
    const label = localizedCategory(app, uncategorizedLabel);
    const key = legacySourceAppCategoryKey(app, label);
    const current = options.get(key);
    if (current) {
      current.count += 1;
    } else {
      options.set(key, { key, label, count: 1 });
    }
  }
  return Array.from(options.values()).sort((a, b) => a.label.localeCompare(b.label));
}

export function matchesSourceAppSource(app: SourceScopedApp, selectedSource: string) {
  return selectedSource === 'all' || sourceFilterKey(app) === selectedSource;
}

export function buildSourceCategoryFilterContext(sources: SourceSubscription[]): SourceCategoryFilterContext {
  const structuredSources = sources.filter((source) => (source.categories?.length || 0) > 0);
  const includeSourceName = structuredSources.length > 1;
  const categories: SourceScopedCategory[] = [];
  const selectedValues = new Map<string, Set<string>>();
  let scopedID = 1;

  for (const source of structuredSources) {
    const sourceKey = sourceSubscriptionFilterKey(source);
    const valid = normalizeSourceCategories(source.categories || []);
    const scopedIDs = new Map<number, number>();
    for (const category of valid) {
      scopedIDs.set(category.id, scopedID);
      scopedID += 1;
    }
    for (const category of valid) {
      const value = sourceCategoryValue(sourceKey, category.id);
      const parentScopedID = category.parentId ? scopedIDs.get(category.parentId) : undefined;
      const name = includeSourceName && !category.parentId ? `${source.name} / ${localizedName(category)}` : category.name;
      categories.push({
        ...category,
        id: scopedIDs.get(category.id) || scopedID++,
        originalId: category.id,
        sourceKey,
        sourceName: source.name,
        value,
        name,
        nameI18n: includeSourceName && !category.parentId ? undefined : category.nameI18n,
        parentId: parentScopedID,
      });
    }

    const childrenByParent = new Map<number, number[]>();
    for (const category of valid) {
      if (!category.parentId) continue;
      childrenByParent.set(category.parentId, [...(childrenByParent.get(category.parentId) || []), category.id]);
    }
    for (const category of valid) {
      const values = new Set<string>([sourceCategoryValue(sourceKey, category.id)]);
      const pending = [...(childrenByParent.get(category.id) || [])];
      while (pending.length > 0) {
        const childID = pending.shift();
        if (!childID) continue;
        values.add(sourceCategoryValue(sourceKey, childID));
        pending.push(...(childrenByParent.get(childID) || []));
      }
      selectedValues.set(sourceCategoryValue(sourceKey, category.id), values);
    }
  }

  return { categories, selectedValues };
}

export function matchesSourceAppCategory(app: SourceScopedApp, selectedCategory: string, context?: SourceCategoryFilterContext) {
  if (selectedCategory === 'all') return true;
  const scopedValues = context?.selectedValues.get(selectedCategory);
  if (scopedValues) {
    const key = structuredSourceAppCategoryKey(app);
    return Boolean(key && scopedValues.has(key));
  }
  return legacySourceAppCategoryKey(app, localizedCategory(app, UNCATEGORIZED_CATEGORY_KEY)) === selectedCategory;
}

function structuredSourceAppCategoryKey(app: SourceScopedApp) {
  if (!app.categoryId || app.categoryId <= 0) return '';
  return sourceCategoryValue(sourceFilterKey(app), app.categoryId);
}

function legacySourceAppCategoryKey(app: SourceScopedApp, fallback: string) {
  return app.category?.trim() || fallback || UNCATEGORIZED_CATEGORY_KEY;
}

function sourceCategoryValue(sourceKey: string, categoryID: number) {
  return `${sourceKey}::${categoryID}`;
}

function normalizeSourceCategories(input: Category[]) {
  const byID = new Map<number, Category>();
  for (const category of input) {
    if (!category.id || category.id <= 0) continue;
    const name = localizedName(category).trim();
    if (!name) continue;
    byID.set(category.id, { ...category, name });
  }
  const output: Category[] = [];
  for (const category of byID.values()) {
    if (!category.parentId || !byID.has(category.parentId)) {
      output.push({ ...category, parentId: undefined });
      continue;
    }
    const parent = byID.get(category.parentId);
    output.push(parent?.parentId ? { ...category, parentId: undefined } : category);
  }
  return output;
}
