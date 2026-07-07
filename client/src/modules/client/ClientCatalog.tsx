import { Cloud, Download, RefreshCw } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { SourceAppGrid } from './SourceAppGrid';
import type {
  ClientSourceStats,
  InstallOptions,
  InstalledApplication,
  SourceApp,
  SourceAppFilter,
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
import {
  matchesSourceAppCategory,
  matchesSourceAppSource,
  sourceAppCategoryOptions,
  sourceAppSourceOptions,
} from './sourceAppFilters';

const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];

export function ClientCatalog({
  sourceApps,
  sources,
  query,
  sourceStats,
  installedApps,
  onOpenSource,
  onInstall,
  onGoSources,
  defaultPageSize,
}: {
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  query: string;
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  onGoSources: () => void;
  defaultPageSize: number;
}) {
  const { t } = useTranslation();
  const [sourceAppFilter, setSourceAppFilter] = useState<SourceAppFilter>('all');
  const [selectedSourceFilter, setSelectedSourceFilter] = useState('all');
  const [selectedCategoryFilter, setSelectedCategoryFilter] = useState('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(defaultPageSize || 24);
  const sourceNeedle = query.trim().toLowerCase();
  const searchableSourceApps = sourceApps.filter((app) => {
    if (!sourceNeedle) return true;
    return [
      app.name,
      localizedAppName(app),
      app.summary,
      localizedAppSummary(app),
      localizedAppDescription(app),
      app.category,
      localizedCategory(app),
      app.sourceName,
    ].filter(Boolean).join(' ').toLowerCase().includes(sourceNeedle);
  });
  const sourceOptions = sourceAppSourceOptions(searchableSourceApps);
  const categoryOptions = sourceAppCategoryOptions(searchableSourceApps, t('common.uncategorized'));
  const updateSourceApps = searchableSourceApps.filter((app) => isSourceAppUpdateAvailable(app, installedApps));
  const sourceAppFilterItems: Array<{ key: SourceAppFilter; label: string; count: number }> = [
    { key: 'all', label: t('search.sourceFilters.all'), count: searchableSourceApps.length },
    { key: 'updates', label: t('search.sourceFilters.updates'), count: updateSourceApps.length },
    { key: 'installable', label: t('search.sourceFilters.installable'), count: searchableSourceApps.filter(hasInstallableVersion).length },
    { key: 'installed', label: t('search.sourceFilters.installed'), count: searchableSourceApps.filter((app) => Boolean(findInstalledApplication(app, installedApps))).length },
    {
      key: 'incomplete',
      label: t('search.sourceFilters.incomplete'),
      count: searchableSourceApps.filter((app) => !hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size).length,
    },
  ];
  const visibleSourceAppFilterItems = updateSourceApps.length > 0
    ? sourceAppFilterItems
    : sourceAppFilterItems.filter((item) => item.key !== 'updates');
  const effectiveSourceAppFilter = updateSourceApps.length === 0 && sourceAppFilter === 'updates' ? 'all' : sourceAppFilter;
  const filteredSourceApps = searchableSourceApps.filter((app) => {
    if (!matchesSourceAppSource(app, selectedSourceFilter)) return false;
    if (!matchesSourceAppCategory(app, selectedCategoryFilter)) return false;
    if (effectiveSourceAppFilter === 'updates') return isSourceAppUpdateAvailable(app, installedApps);
    if (effectiveSourceAppFilter === 'installable') return hasInstallableVersion(app);
    if (effectiveSourceAppFilter === 'installed') return Boolean(findInstalledApplication(app, installedApps));
    if (effectiveSourceAppFilter === 'incomplete') return !hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size;
    return true;
  });
  const totalPages = Math.max(1, Math.ceil(filteredSourceApps.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedSourceApps = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredSourceApps.slice(start, start + pageSize);
  }, [filteredSourceApps, currentPage, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [sourceNeedle, selectedSourceFilter, selectedCategoryFilter, effectiveSourceAppFilter, sourceApps.length, installedApps.length]);

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
      : effectiveSourceAppFilter === 'all'
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
        <div className="filter-bar">
          <XSelector
            label={t('search.sourceFilter')}
            value={selectedSourceFilter}
            options={[
              { value: 'all', label: t('search.allSources') },
              ...sourceOptions.map((option) => ({ value: option.key, label: `${option.label} (${option.count})` })),
            ]}
            onChange={setSelectedSourceFilter}
          />
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
        <XToggleButtonGroup value={effectiveSourceAppFilter} onChange={(value) => setSourceAppFilter((value || 'all') as SourceAppFilter)} label={t('search.sourceAppFilter')} size="sm">
          {visibleSourceAppFilterItems.map((item) => (
            <XToggleButton
              key={item.key}
              value={item.key}
              label={`${item.label} ${item.count}`}
            />
          ))}
        </XToggleButtonGroup>
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
