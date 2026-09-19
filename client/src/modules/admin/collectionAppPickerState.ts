export type SearchableCollectionApp = {
  id: number;
  name: string;
  nameI18n?: Record<string, string>;
  slug: string;
  packageId?: string;
};

export function filterCollectionApps<T extends SearchableCollectionApp>(apps: T[], query: string): T[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) return apps;
  return apps.filter((app) => [
    app.name,
    ...Object.values(app.nameI18n || {}),
    app.packageId,
    app.slug,
  ].some((value) => value?.toLowerCase().includes(normalizedQuery)));
}
