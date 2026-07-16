import type { Dispatch, SetStateAction } from 'react';
import type { Category, ClientSourceStats, InstalledApplication, InstallOptions, SourceApp, SourceSubscription, StoreApp } from '../../shared/types';
import type { AppDetailMode } from '../storefront/AppDrawer';
import { ClientCatalog, type ClientCatalogViewState } from '../client/ClientCatalog';
import { StorefrontSearch, type StorefrontSearchViewState } from '../storefront/StorefrontSearch';

export function SearchView({
  apps,
  sourceApps,
  sources,
  categories,
  submitters,
  tagOptions,
  mode,
  lazycatInstall,
  sourceStats,
  installedApps,
  onOpen,
  onOpenSource,
  onInstall,
  onGoSources,
  defaultPageSize,
  activeInstallKey,
  clientCatalogState,
  onClientCatalogStateChange,
  storefrontSearchState,
  onStorefrontSearchStateChange,
}: {
  apps: StoreApp[];
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  categories: Category[];
  submitters: string[];
  tagOptions: string[];
  mode: 'server' | 'client';
  lazycatInstall: boolean;
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  onGoSources: () => void;
  defaultPageSize: number;
  activeInstallKey?: string;
  clientCatalogState: ClientCatalogViewState;
  onClientCatalogStateChange: Dispatch<SetStateAction<ClientCatalogViewState>>;
  storefrontSearchState: StorefrontSearchViewState;
  onStorefrontSearchStateChange: Dispatch<SetStateAction<StorefrontSearchViewState>>;
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
        activeInstallKey={activeInstallKey}
        viewState={clientCatalogState}
        onViewStateChange={onClientCatalogStateChange}
      />
    );
  }

  return (
    <StorefrontSearch
      apps={apps}
      categories={categories}
      submitters={submitters}
      tagOptions={tagOptions}
      onOpen={onOpen}
      onInstall={onInstall}
      lazycatInstall={lazycatInstall}
      defaultPageSize={defaultPageSize}
      viewState={storefrontSearchState}
      onViewStateChange={onStorefrontSearchStateChange}
    />
  );
}
