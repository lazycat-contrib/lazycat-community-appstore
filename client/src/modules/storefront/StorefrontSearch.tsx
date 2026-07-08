import { Search } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { flattenCategoryTree } from '../../shared/categoryTree';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, InstallOptions, SortMode, SourceApp, StoreApp } from '../../shared/types';
import { AppGrid } from './AppGrid';

const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];

export function StorefrontSearch({
  apps,
  categories,
  submitters,
  activeCategory,
  activeSubmitter,
  sortMode,
  onCategory,
  onSubmitter,
  onSortMode,
  onOpen,
  onInstall,
  defaultPageSize,
}: {
  apps: StoreApp[];
  categories: Category[];
  submitters: string[];
  activeCategory: string;
  activeSubmitter: string;
  sortMode: SortMode;
  onCategory: (category: string) => void;
  onSubmitter: (submitter: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  defaultPageSize: number;
}) {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(defaultPageSize || 24);
  const totalPages = Math.max(1, Math.ceil(apps.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedApps = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return apps.slice(start, start + pageSize);
  }, [apps, currentPage, pageSize]);
  const categoryTree = useMemo(() => flattenCategoryTree(categories), [categories]);

  useEffect(() => {
    setPage(1);
  }, [apps]);

  useEffect(() => {
    setPageSize(defaultPageSize || 24);
    setPage(1);
  }, [defaultPageSize]);

  return (
    <section className="page-grid">
      <div className="page-heading">
        <h1>{t('search.serverTitle')}</h1>
        <p>{t('search.serverDescription')}</p>
      </div>
      <section className="panel">
        <SectionTitle icon={Search} title={t('search.localStore')} />
        {categories.length > 0 && (
          <XToggleButtonGroup value={activeCategory} onChange={(value) => onCategory(value || 'all')} label={t('search.categoryFilter')} size="sm">
            <XToggleButton value="all" label={t('common.all')} />
            {categoryTree.map((item) => (
              <XToggleButton key={item.category.id} value={String(item.category.id)} label={item.path} />
            ))}
          </XToggleButtonGroup>
        )}
        <div className="filter-bar">
          <XSelector
            label={t('search.sort')}
            value={sortMode}
            options={[
              { value: 'recent', label: t('search.recent') },
              { value: 'downloads', label: t('search.downloads') },
              { value: 'name', label: t('search.name') },
            ]}
            onChange={(value) => onSortMode(value as SortMode)}
          />
          <XSelector
            label={t('search.submitter')}
            value={activeSubmitter}
            options={[
              { value: 'all', label: t('search.allSubmitters') },
              ...submitters.map((submitter) => ({ value: submitter, label: submitter })),
            ]}
            onChange={onSubmitter}
          />
        </div>
        <AppGrid
          apps={pagedApps}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{ title: t('search.noResultsTitle'), body: t('search.noResultsBody') }}
        />
        {apps.length > pageSize && (
          <XPagination
            className="list-pagination"
            page={currentPage}
            onChange={setPage}
            totalItems={apps.length}
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
