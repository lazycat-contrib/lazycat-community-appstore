export type SourceAppFilterOption = {
  key: string;
  label: string;
  count: number;
};

export type SourceScopedApp = {
  sourceId?: number | string;
  sourceName: string;
  category?: string;
};

export function sourceFilterKey(app: SourceScopedApp) {
  return app.sourceId ? String(app.sourceId) : `name:${app.sourceName}`;
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

export function sourceAppCategoryOptions(apps: SourceScopedApp[], uncategorizedLabel: string): SourceAppFilterOption[] {
  const options = new Map<string, SourceAppFilterOption>();
  for (const app of apps) {
    const label = app.category?.trim() || uncategorizedLabel;
    const key = app.category?.trim() || '__uncategorized__';
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

export function matchesSourceAppCategory(app: SourceScopedApp, selectedCategory: string) {
  const categoryKey = app.category?.trim() || '__uncategorized__';
  return selectedCategory === 'all' || categoryKey === selectedCategory;
}
