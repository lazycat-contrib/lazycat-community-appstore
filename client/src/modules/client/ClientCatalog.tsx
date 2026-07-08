import { Cloud, Download, PackageCheck, RefreshCw, Search, Server } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { PowerSearch, type PowerSearchConfig, type PowerSearchFilter } from '@astryxdesign/core/PowerSearch';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { SourceAppGrid } from './SourceAppGrid';
import { CategoryBrowser } from '../storefront/CategoryBrowser';
import type {
  ClientSourceStats,
  InstallOptions,
  InstalledApplication,
  SourceApp,
  SourceSubscription,
  StoreApp,
} from '../../shared/types';
import {
  cx,
  defaultMirrorIDForVersion,
  findInstalledApplication,
  hasInstallableVersion,
  isSourceAppUpdateAvailable,
  localizedAppDescription,
  localizedAppName,
  localizedAppSummary,
  localizedCategory,
  selectedSourceVersion,
  sourceForApp,
} from '../../shared/utils';
import { enumOptionsFromEntries, filterSignature, matchesChoiceFilter, matchesStringFilter } from '../search/searchFilterHelpers';
import {
  buildSourceCategoryFilterContext,
  matchesSourceAppCategory,
  sourceAppCategoryOptions,
  sourceAppSourceOptions,
} from './sourceAppFilters';

const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];
type ClientCatalogSortMode = 'default' | 'name' | 'source';

export function ClientCatalog({
  sourceApps,
  sources,
  sourceStats,
  installedApps,
  onOpenSource,
  onInstall,
  onGoSources,
  defaultPageSize,
}: {
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  onGoSources: () => void;
  defaultPageSize: number;
}) {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<PowerSearchFilter[]>([]);
  const [selectedCategoryFilter, setSelectedCategoryFilter] = useState('all');
  const [sortMode, setSortMode] = useState<ClientCatalogSortMode>('default');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(defaultPageSize || 24);
  const filterKey = filterSignature(filters);
  const sourceOptions = sourceAppSourceOptions(sourceApps);
  const sourceStatusItems = [
    { key: 'updates', label: t('search.sourceFilters.updates'), count: sourceApps.filter((app) => isSourceAppUpdateAvailable(app, installedApps)).length },
    { key: 'installable', label: t('search.sourceFilters.installable'), count: sourceApps.filter(hasInstallableVersion).length },
    { key: 'installed', label: t('search.sourceFilters.installed'), count: sourceApps.filter((app) => Boolean(findInstalledApplication(app, installedApps))).length },
    {
      key: 'incomplete',
      label: t('search.sourceFilters.incomplete'),
      count: sourceApps.filter((app) => !hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size).length,
    },
  ];
  const searchConfig = useMemo<PowerSearchConfig>(() => ({
    name: 'ClientCatalogSearch',
    contentSearchFieldKey: 'content',
    fields: [
      {
        key: 'content',
        label: t('search.catalogSearchKeyword'),
        defaultOperator: 'contains',
        operators: [
          { key: 'contains', label: t('search.operators.contains'), value: { type: 'string' } },
        ],
        isValueMatchAllowed: false,
      },
      {
        key: 'source',
        label: t('search.catalogSearchSource'),
        icon: <Server size={15} />,
        defaultOperator: 'is',
        operators: [
          { key: 'is', label: t('search.operators.is'), value: { type: 'enum', values: enumOptionsFromEntries(sourceOptions.map((option) => ({ value: option.key, label: option.label }))) } },
          { key: 'is_not', label: t('search.operators.isNot'), value: { type: 'enum', values: enumOptionsFromEntries(sourceOptions.map((option) => ({ value: option.key, label: option.label }))) } },
          { key: 'is_any_of', label: t('search.operators.isAnyOf'), value: { type: 'enum_list', values: enumOptionsFromEntries(sourceOptions.map((option) => ({ value: option.key, label: option.label }))) } },
        ],
      },
      {
        key: 'status',
        label: t('search.catalogSearchStatus'),
        icon: <PackageCheck size={15} />,
        defaultOperator: 'is_any_of',
        operators: [
          { key: 'is_any_of', label: t('search.operators.isAnyOf'), value: { type: 'enum_list', values: enumOptionsFromEntries(sourceStatusItems.map((item) => ({ value: item.key, label: item.label }))) } },
          { key: 'is_none_of', label: t('search.operators.isNoneOf'), value: { type: 'enum_list', values: enumOptionsFromEntries(sourceStatusItems.map((item) => ({ value: item.key, label: item.label }))) } },
        ],
      },
    ],
  }), [sourceOptions, sourceStatusItems, t]);
  const searchedSourceApps = useMemo(() => sourceApps.filter((app) => matchesClientCatalogFilters(app, filters, installedApps)), [filterKey, filters, installedApps, sourceApps]);
  const categoryOptions = sourceAppCategoryOptions(searchedSourceApps, t('common.uncategorized'));
  const categoryContext = useMemo(() => buildSourceCategoryFilterContext(sources), [sources]);
  const hasStructuredCategories = categoryContext.categories.length > 0;
  const updateSourceApps = searchedSourceApps.filter((app) => isSourceAppUpdateAvailable(app, installedApps));
  const filteredSourceApps = useMemo(() => {
    const filtered = searchedSourceApps.filter((app) => {
      if (!matchesSourceAppCategory(app, selectedCategoryFilter, hasStructuredCategories ? categoryContext : undefined)) return false;
      return true;
    });
    return [...filtered].sort((a, b) => {
      if (sortMode === 'name') return localizedAppName(a).localeCompare(localizedAppName(b));
      if (sortMode === 'source') {
        const sourceDelta = a.sourceName.localeCompare(b.sourceName);
        return sourceDelta !== 0 ? sourceDelta : localizedAppName(a).localeCompare(localizedAppName(b));
      }
      return sourceApps.indexOf(a) - sourceApps.indexOf(b);
    });
  }, [categoryContext, hasStructuredCategories, searchedSourceApps, selectedCategoryFilter, sortMode, sourceApps]);
  const totalPages = Math.max(1, Math.ceil(filteredSourceApps.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedSourceApps = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredSourceApps.slice(start, start + pageSize);
  }, [filteredSourceApps, currentPage, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [filterKey, selectedCategoryFilter, sortMode, sourceApps.length, installedApps.length]);

  useEffect(() => {
    setPageSize(defaultPageSize || 24);
    setPage(1);
  }, [defaultPageSize]);

  async function updateAllSourceApps() {
    for (const app of updateSourceApps) {
      const source = sourceForApp(app, sources);
      const version = selectedSourceVersion(app);
      await Promise.resolve(onInstall(app, { mirrorId: defaultMirrorIDForVersion(source, version) }));
    }
  }

  const sourceEmptyTitle = sourceApps.length === 0 ? t('search.noSyncedApps') : t('search.noResultsTitle');
  const sourceEmptyBody =
    sourceApps.length === 0
      ? t('search.noSyncedAppsBody')
      : filters.length === 0 && selectedCategoryFilter === 'all'
        ? t('search.noResultsBody')
        : t('search.noFilterResultsBody');

  return (
    <section className="page-grid">
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('search.sourceCount', { count: sourceStats.sourceCount })}</span>
          <h1>{t('search.clientTitle')}</h1>
          <p>{t('search.clientDescription')}</p>
        </div>
        <XButton type="button" variant="secondary" label={t('search.noSyncedAppsAction')} icon={<Cloud size={18} />} onClick={onGoSources} />
      </div>
      <div className="client-summary-grid" aria-label={t('search.installReadiness')}>
        <div>
          <span>{t('search.sourcesTotal')}</span>
          <strong>{sourceStats.sourceCount}</strong>
        </div>
        <div>
          <span>{t('search.syncedAppsTotal')}</span>
          <strong>{sourceStats.sourceAppCount}</strong>
        </div>
        <div>
          <span>{t('search.installableApps')}</span>
          <strong>{sourceStats.installableSourceAppCount}</strong>
        </div>
        <div className={cx(sourceStats.staleSourceCount > 0 && 'warning')}>
          <span>{t('search.staleSources')}</span>
          <strong>{sourceStats.staleSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.authSourceCount > 0 && 'warning')}>
          <span>{t('search.authSources')}</span>
          <strong>{sourceStats.authSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.failedSourceCount > 0 && 'warning')}>
          <span>{t('search.failedSources')}</span>
          <strong>{sourceStats.failedSourceCount}</strong>
        </div>
      </div>
      <section className="panel">
        <div className="section-title with-action">
          <div>
            <Download size={19} />
            <h2>{t('search.subscribedApps')}</h2>
          </div>
          {updateSourceApps.length > 0 && (
            <XButton type="button" variant="primary" size="sm" label={t('search.updateAll')} icon={<RefreshCw size={17} />} onClick={() => void updateAllSourceApps()} />
          )}
        </div>
        <div className="catalog-search-toolbar">
          <PowerSearch
            className="catalog-filter-search"
            config={searchConfig}
            filters={filters}
            onChange={(nextFilters) => setFilters([...nextFilters])}
            label={t('search.catalogSearchLabel')}
            placeholder={t('search.clientCatalogSearchPlaceholder')}
            startIcon={<Search size={16} />}
            resultCount={t('search.resultCount', { count: filteredSourceApps.length })}
            popoverSaveButtonLabel={t('common.apply')}
            tokenOverflowBehavior="unfocusedInline"
            hasClear
          />
          <XSelector
            label={t('search.sort')}
            isLabelHidden
            value={sortMode}
            options={[
              { value: 'default', label: t('search.defaultOrder') },
              { value: 'name', label: t('search.name') },
              { value: 'source', label: t('search.sourceName') },
            ]}
            onChange={(value) => setSortMode(value as ClientCatalogSortMode)}
            width="100%"
          />
        </div>
        {!hasStructuredCategories && (
          <div className="filter-bar">
            <XSelector
              label={t('search.categoryFilter')}
              value={selectedCategoryFilter}
              options={[
                { value: 'all', label: t('search.allCategories') },
                ...categoryOptions.map((option) => ({ value: option.key, label: `${option.label} (${option.count})` })),
              ]}
              onChange={setSelectedCategoryFilter}
            />
          </div>
        )}
        {hasStructuredCategories && (
          <CategoryBrowser categories={categoryContext.categories} activeCategory={selectedCategoryFilter} onCategory={setSelectedCategoryFilter} />
        )}
        <SourceAppGrid
          apps={pagedSourceApps}
          installedApps={installedApps}
          onOpen={onOpenSource}
          onInstall={onInstall}
          onGoSources={onGoSources}
          emptyTitle={sourceEmptyTitle}
          emptyBody={sourceEmptyBody}
        />
        {filteredSourceApps.length > pageSize && (
          <XPagination
            className="list-pagination"
            page={currentPage}
            onChange={setPage}
            totalItems={filteredSourceApps.length}
            pageSize={pageSize}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            onPageSizeChange={setPageSize}
            variant="pages"
            size="sm"
            label={t('pagination.label')}
          />
        )}
      </section>
    </section>
  );
}

function matchesClientCatalogFilters(app: SourceApp, filters: PowerSearchFilter[], installedApps: InstalledApplication[]) {
  const searchText = [
    app.name,
    localizedAppName(app),
    app.summary,
    localizedAppSummary(app),
    localizedAppDescription(app),
    app.category,
    localizedCategory(app),
    app.sourceName,
    app.packageId,
    app.slug,
    app.latestVersion?.version,
  ].filter(Boolean).join(' ');
  const statuses = sourceAppStatusValues(app, installedApps);
  return filters.every((filter) => {
    if (filter.field === 'content') return matchesStringFilter(searchText, filter);
    if (filter.field === 'source') return matchesChoiceFilter(app.sourceId !== undefined && app.sourceId !== null ? String(app.sourceId) : `name:${app.sourceName}`, filter);
    if (filter.field === 'status') return matchesChoiceFilter(statuses, filter);
    return true;
  });
}

function sourceAppStatusValues(app: SourceApp, installedApps: InstalledApplication[]) {
  const values: string[] = [];
  if (isSourceAppUpdateAvailable(app, installedApps)) values.push('updates');
  if (hasInstallableVersion(app)) values.push('installable');
  if (findInstalledApplication(app, installedApps)) values.push('installed');
  if (!hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size) values.push('incomplete');
  return values;
}
