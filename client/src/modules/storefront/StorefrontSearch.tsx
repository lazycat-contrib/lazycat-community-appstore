import { Search } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Tokenizer as XTokenizer } from '@astryxdesign/core/Tokenizer';
import { createStaticSource, TypeaheadItem as XTypeaheadItem, type SearchableItem } from '@astryxdesign/core/Typeahead';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, InstallOptions, SortMode, SourceApp, StoreApp } from '../../shared/types';
import { AppGrid } from './AppGrid';
import { CategoryBrowser } from './CategoryBrowser';

const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];
type TagFilterItem = SearchableItem;

function tagFilterItem(tag: string): TagFilterItem {
  return { id: tag.toLowerCase(), label: tag };
}

function uniqueTags(tags: string[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  tags.forEach((tag) => {
    const normalized = tag.trim();
    const key = normalized.toLowerCase();
    if (!normalized || seen.has(key)) return;
    seen.add(key);
    out.push(normalized);
  });
  return out;
}

export function StorefrontSearch({
  apps,
  categories,
  submitters,
  activeCategory,
  activeSubmitter,
  activeTags,
  tagOptions,
  sortMode,
  onCategory,
  onSubmitter,
  onTags,
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
  activeTags: string[];
  tagOptions: string[];
  sortMode: SortMode;
  onCategory: (category: string) => void;
  onSubmitter: (submitter: string) => void;
  onTags: (tags: string[]) => void;
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
  const tagItems = useMemo(() => uniqueTags([...tagOptions, ...activeTags]).map(tagFilterItem), [activeTags, tagOptions]);
  const selectedTagItems = useMemo(() => uniqueTags(activeTags).map(tagFilterItem), [activeTags]);
  const tagSearchSource = useMemo(() => createStaticSource(tagItems), [tagItems]);
  const pagedApps = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return apps.slice(start, start + pageSize);
  }, [apps, currentPage, pageSize]);

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
          <CategoryBrowser categories={categories} activeCategory={activeCategory} onCategory={onCategory} />
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
          {(tagOptions.length > 0 || activeTags.length > 0) && (
            <XTokenizer
              label={t('search.tagFilter')}
              searchSource={tagSearchSource}
              value={selectedTagItems}
              hasClear
              hasEntriesOnFocus
              placeholder={t('search.tagFilterPlaceholder')}
              debounceMs={0}
              width="100%"
              tokenOverflowBehavior="unfocusedInline"
              renderItem={(item) => <XTypeaheadItem item={item} />}
              onChange={(items) => onTags(uniqueTags(items.map((item) => item.label)))}
            />
          )}
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
