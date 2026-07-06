import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, InstallOptions, SortMode, SourceApp, StoreApp } from '../../shared/types';
import { cx } from '../../shared/utils';
import { AppGrid } from './AppGrid';

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
}) {
  const { t } = useTranslation();
  return (
    <section className="page-grid">
      <div className="page-heading">
        <h1>{t('search.serverTitle')}</h1>
        <p>{t('search.serverDescription')}</p>
      </div>
      <section className="panel">
        <SectionTitle icon={Search} title={t('search.localStore')} />
        {categories.length > 0 && (
          <div className="segmented filter-segmented" aria-label={t('search.categoryFilter')}>
            <button type="button" className={cx(activeCategory === 'all' && 'active')} onClick={() => onCategory('all')}>{t('common.all')}</button>
            {categories.map((category) => (
              <button type="button" key={category.id} className={cx(activeCategory === category.name && 'active')} onClick={() => onCategory(category.name)}>
                {category.name}
              </button>
            ))}
          </div>
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
          apps={apps}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{ title: t('search.noResultsTitle'), body: t('search.noResultsBody') }}
        />
      </section>
    </section>
  );
}
