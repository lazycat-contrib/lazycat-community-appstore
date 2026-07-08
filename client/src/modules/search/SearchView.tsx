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
  activeSubmitter,
  activeTags,
  tagOptions,
  sortMode,
  query,
  mode,
  sourceStats,
  installedApps,
  onCategory,
  onSubmitter,
  onTags,
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
  activeSubmitter: string;
  activeTags: string[];
  tagOptions: string[];
  sortMode: SortMode;
  query: string;
  mode: 'server' | 'client';
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onCategory: (category: string) => void;
  onSubmitter: (submitter: string) => void;
  onTags: (tags: string[]) => void;
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
        query={query}
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
      activeSubmitter={activeSubmitter}
      activeTags={activeTags}
      tagOptions={tagOptions}
      sortMode={sortMode}
      onCategory={onCategory}
      onSubmitter={onSubmitter}
      onTags={onTags}
      onSortMode={onSortMode}
      onOpen={onOpen}
      onInstall={onInstall}
      defaultPageSize={defaultPageSize}
    />
  );
}
