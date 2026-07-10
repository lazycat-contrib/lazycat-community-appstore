import { RotateCcw, Search, Tag, UserRound } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { PowerSearch, type PowerSearchConfig, type PowerSearchField, type PowerSearchFilter } from '@astryxdesign/core/PowerSearch';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { SectionTitle } from '../../shared/components/Feedback';
import { categoryDescendantIds } from '../../shared/categoryTree';
import type { Category, InstallOptions, SortMode, SourceApp, StoreApp } from '../../shared/types';
import { localizedAppDescription, localizedAppName, localizedAppSummary, localizedName } from '../../shared/utils';
import { filterSignature, matchesChoiceFilter, matchesStringFilter, uniqueEnumOptions } from '../search/searchFilterHelpers';
import { AppGrid } from './AppGrid';
import { CategoryBrowser } from './CategoryBrowser';

const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];

export function StorefrontSearch({
  apps,
  categories,
  submitters,
  activeCategory,
  tagOptions,
  sortMode,
  onCategory,
  onSortMode,
  onOpen,
  onInstall,
  defaultPageSize,
}: {
  apps: StoreApp[];
  categories: Category[];
  submitters: string[];
  activeCategory: string;
  tagOptions: string[];
  sortMode: SortMode;
  onCategory: (category: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  defaultPageSize: number;
}) {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<PowerSearchFilter[]>([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(defaultPageSize || 24);
  const filterKey = filterSignature(filters);
  const searchConfig = useMemo<PowerSearchConfig>(() => {
    const fields: PowerSearchField[] = [
      {
        key: 'content',
        label: t('search.catalogSearchKeyword'),
        defaultOperator: 'contains',
        operators: [
          { key: 'contains', label: t('search.operators.contains'), value: { type: 'string' } },
        ],
        isValueMatchAllowed: false,
      },
    ];
    if (submitters.length > 0) {
      fields.push({
        key: 'submitter',
        label: t('search.submitter'),
        icon: <UserRound size={15} />,
        defaultOperator: 'is',
        operators: [
          { key: 'is', label: t('search.operators.is'), value: { type: 'enum', values: uniqueEnumOptions(submitters) } },
          { key: 'is_not', label: t('search.operators.isNot'), value: { type: 'enum', values: uniqueEnumOptions(submitters) } },
        ],
      });
    }
    if (tagOptions.length > 0) {
      fields.push({
        key: 'tags',
        label: t('search.catalogSearchTags'),
        icon: <Tag size={15} />,
        defaultOperator: 'is_any_of',
        operators: [
          { key: 'is_any_of', label: t('search.operators.isAnyOf'), value: { type: 'enum_list', values: uniqueEnumOptions(tagOptions) } },
          { key: 'is_none_of', label: t('search.operators.isNoneOf'), value: { type: 'enum_list', values: uniqueEnumOptions(tagOptions) } },
        ],
      });
    }
    return { name: 'StorefrontSearch', contentSearchFieldKey: 'content', fields };
  }, [submitters, tagOptions, t]);
  const categoryFilteredApps = useMemo(() => {
    const selectedCategoryID = Number(activeCategory);
    const selectedCategoryIDs = new Set<number>();
    if (activeCategory !== 'all' && Number.isFinite(selectedCategoryID) && selectedCategoryID > 0) {
      selectedCategoryIDs.add(selectedCategoryID);
      for (const childID of categoryDescendantIds(categories, selectedCategoryID)) {
        selectedCategoryIDs.add(childID);
      }
    }
    return apps.filter((app) => activeCategory === 'all' || (app.categoryId ? selectedCategoryIDs.has(app.categoryId) : false));
  }, [activeCategory, apps, categories]);
  const filteredApps = useMemo(() => {
    const searched = categoryFilteredApps.filter((app) => matchesStoreAppFilters(app, filters));
    return [...searched].sort((a, b) => {
      if (sortMode === 'downloads') return b.downloadCount - a.downloadCount;
      if (sortMode === 'downloads_day') return (b.downloadStats?.day || 0) - (a.downloadStats?.day || 0);
      if (sortMode === 'downloads_week') return (b.downloadStats?.week || 0) - (a.downloadStats?.week || 0);
      if (sortMode === 'downloads_month') return (b.downloadStats?.month || 0) - (a.downloadStats?.month || 0);
      if (sortMode === 'downloads_year') return (b.downloadStats?.year || 0) - (a.downloadStats?.year || 0);
      if (sortMode === 'name') return localizedAppName(a).localeCompare(localizedAppName(b));
      return Date.parse(b.updatedAt) - Date.parse(a.updatedAt);
    });
  }, [categoryFilteredApps, filterKey, filters, sortMode]);
  const totalPages = Math.max(1, Math.ceil(filteredApps.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pagedApps = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredApps.slice(start, start + pageSize);
  }, [filteredApps, currentPage, pageSize]);
  const selectedCategory = categories.find((category) => String(category.id) === activeCategory);
  const activeCategoryLabel = selectedCategory ? localizedName(selectedCategory) : t('search.allCategories');
  const hasActiveFilters = activeCategory !== 'all' || filters.length > 0;

  function clearSearch() {
    setFilters([]);
    onCategory('all');
    setPage(1);
  }

  useEffect(() => {
    setPage(1);
  }, [activeCategory, filterKey, sortMode]);

  useEffect(() => {
    setPageSize(defaultPageSize || 24);
    setPage(1);
  }, [defaultPageSize]);

  return (
    <section className="page-grid storefront-search-page">
      <div className="page-heading">
        <h1>{t('search.serverTitle')}</h1>
        <p>{t('search.serverDescription')}</p>
      </div>
      <section className="panel storefront-search-panel">
        <SectionTitle icon={Search} title={t('search.localStore')} />
        {categories.length > 0 && (
          <CategoryBrowser categories={categories} activeCategory={activeCategory} onCategory={onCategory} />
        )}
        <div className="catalog-search-toolbar">
          <PowerSearch
            className="catalog-filter-search"
            config={searchConfig}
            filters={filters}
            onChange={(nextFilters) => setFilters([...nextFilters])}
            label={t('search.catalogSearchLabel')}
            placeholder={t('search.catalogSearchPlaceholder')}
            startIcon={<Search size={16} />}
            resultCount={t('search.resultCount', { count: filteredApps.length })}
            popoverSaveButtonLabel={t('common.apply')}
            tokenOverflowBehavior="unfocusedInline"
            hasClear
          />
          <XSelector
            label={t('search.sort')}
            isLabelHidden
            value={sortMode}
            options={[
              { value: 'recent', label: t('search.recent') },
              { value: 'downloads', label: t('search.downloads') },
              { value: 'downloads_day', label: t('search.downloadsDay') },
              { value: 'downloads_week', label: t('search.downloadsWeek') },
              { value: 'downloads_month', label: t('search.downloadsMonth') },
              { value: 'downloads_year', label: t('search.downloadsYear') },
              { value: 'name', label: t('search.name') },
            ]}
            onChange={(value) => onSortMode(value as SortMode)}
            width="100%"
          />
        </div>
        <div className="catalog-result-summary" role="status" aria-live="polite" aria-atomic="true">
          <span>{activeCategoryLabel}</span>
          <strong>{t('search.resultCount', { count: filteredApps.length })}</strong>
        </div>
        <AppGrid
          apps={pagedApps}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{
            title: t('search.noResultsTitle'),
            body: t('search.noResultsBody'),
            action: hasActiveFilters
              ? {
                  label: t('search.clearFilters', { defaultValue: t('search.allCategories') }),
                  icon: RotateCcw,
                  onClick: clearSearch,
                }
              : undefined,
          }}
        />
        {filteredApps.length > pageSize && (
          <XPagination
            className="list-pagination"
            page={currentPage}
            onChange={setPage}
            totalItems={filteredApps.length}
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

function matchesStoreAppFilters(app: StoreApp, filters: PowerSearchFilter[]) {
  const searchText = [
    app.name,
    localizedAppName(app),
    app.summary,
    localizedAppSummary(app),
    localizedAppDescription(app),
    app.owner,
    app.packageId,
    app.slug,
    app.category,
    app.tags.join(' '),
  ].filter(Boolean).join(' ');
  return filters.every((filter) => {
    if (filter.field === 'content') return matchesStringFilter(searchText, filter);
    if (filter.field === 'submitter') return matchesChoiceFilter(app.owner, filter);
    if (filter.field === 'tags') return matchesChoiceFilter(app.tags, filter);
    return true;
  });
}
