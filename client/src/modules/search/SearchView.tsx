import type { Category, ClientSourceStats, InstalledApplication, InstallOptions, SortMode, SourceApp, SourceSubscription, StoreApp } from '../../shared/types';
import type { AppDetailMode } from '../storefront/AppDrawer';
import { ClientCatalog } from '../client/ClientCatalog';
import { StorefrontSearch } from '../storefront/StorefrontSearch';

export function SearchView({
  apps,
  sourceApps,
  sources,
  categories,
  submitters,
  activeCategory,
  tagOptions,
  sortMode,
  mode,
  sourceStats,
  installedApps,
  onCategory,
  onSortMode,
  onOpen,
  onOpenSource,
  onInstall,
  onGoSources,
  defaultPageSize,
}: {
  apps: StoreApp[];
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  categories: Category[];
  submitters: string[];
  activeCategory: string;
  tagOptions: string[];
  sortMode: SortMode;
  mode: 'server' | 'client';
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onCategory: (category: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  onGoSources: () => void;
  defaultPageSize: number;
}) {

  if (mode === 'client') {
    return (
      <ClientCatalog
        sourceApps={sourceApps}
        sources={sources}
        sourceStats={sourceStats}
        installedApps={installedApps}
        onOpenSource={onOpenSource}
        onInstall={onInstall}
        onGoSources={onGoSources}
        defaultPageSize={defaultPageSize}
      />
    );
  }

  return (
    <StorefrontSearch
      apps={apps}
      categories={categories}
      submitters={submitters}
      activeCategory={activeCategory}
      tagOptions={tagOptions}
      sortMode={sortMode}
      onCategory={onCategory}
      onSortMode={onSortMode}
      onOpen={onOpen}
      onInstall={onInstall}
      defaultPageSize={defaultPageSize}
    />
  );
}
