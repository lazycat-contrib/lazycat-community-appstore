import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, InstallOptions, SortMode, SourceApp, StoreApp } from '../../shared/types';
import { localizedName } from '../../shared/utils';
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
          <XToggleButtonGroup value={activeCategory} onChange={(value) => onCategory(value || 'all')} label={t('search.categoryFilter')} size="sm">
            <XToggleButton value="all" label={t('common.all')} />
            {categories.map((category) => (
              <XToggleButton key={category.id} value={String(category.id)} label={localizedName(category)} />
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
          apps={apps}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{ title: t('search.noResultsTitle'), body: t('search.noResultsBody') }}
        />
      </section>
    </section>
  );
}
