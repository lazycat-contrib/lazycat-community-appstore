import { Copy, Download, History, Layers3, LogIn, PackagePlus, Search, Tag } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { API_BASE } from '../../config';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, Collection, SiteProfile, StoreApp, Toast } from '../../shared/types';
import { errorMessage, localizedName } from '../../shared/utils';
import { AppGrid } from './AppGrid';

export function StorefrontHome({
  apps,
  categories,
  collections,
  siteProfile,
  onOpen,
  onInstall,
  onNavigate,
  onCategory,
  setToast,
  isAuthenticated,
}: {
  apps: StoreApp[];
  categories: Category[];
  collections: Collection[];
  siteProfile: SiteProfile;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onNavigate: (tab: 'search' | 'profile') => void;
  onCategory: (category: string) => void;
  setToast: (toast: Toast) => void;
  isAuthenticated: boolean;
}) {
  const { t } = useTranslation();
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  const approvedCount = apps.filter((app) => app.status === 'APPROVED').length;
  const sourceFeedURL = siteProfile.sourceUrl || `${API_BASE || window.location.origin}/source/v1/index.json`;
  const BackstageIcon = isAuthenticated ? PackagePlus : LogIn;
  const backstageLabel = isAuthenticated ? t('home.submitApp') : t('topbar.login');

  async function copySourceFeed() {
    try {
      if (!navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(sourceFeedURL);
      setToast({ tone: 'success', message: t('home.sourceCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('home.copySourceFailed')) });
    }
  }

  return (
    <section className="page-grid storefront-page">
      <div className="hero-band storefront-hero">
        <div className="storefront-hero-copy">
          <span className="eyebrow">{t('home.eyebrow')}</span>
          <h1>{siteProfile.title || t('home.title')}</h1>
          <p>{t('home.body')}</p>
          <div className="hero-actions">
            <XButton type="button" variant="primary" label={t('nav.discover')} icon={<Search size={18} />} onClick={() => onNavigate('search')} />
            <XButton type="button" variant="primary" label={backstageLabel} icon={<BackstageIcon size={18} />} onClick={() => onNavigate('profile')} />
          </div>
        </div>
      </div>

      <section className="store-metrics" aria-label={t('nav.store')}>
        <div className="metric-card">
          <span>{t('common.apps')}</span>
          <strong>{approvedCount}</strong>
          <small>{t('home.approvedCount', { count: approvedCount })}</small>
        </div>
        <div className="metric-card">
          <span>{t('common.category')}</span>
          <strong>{categories.length}</strong>
          <small>{t('home.categoryCount', { count: categories.length })}</small>
        </div>
        <div className="metric-card source-feed-card">
          <span>{t('home.sourceUrl')}</span>
          <strong>{sourceFeedURL}</strong>
          <small>{t('home.openSourceFeed')}</small>
          <div className="source-feed-actions">
            <XButton type="button" variant="secondary" size="sm" label={t('home.copySourceFeed')} icon={<Copy size={16} />} onClick={() => void copySourceFeed()} />
            <XButton type="button" variant="secondary" size="sm" label={t('home.browseInstallable')} icon={<Download size={16} />} onClick={() => onNavigate('search')} />
          </div>
        </div>
      </section>

      {categories.length > 0 && (
        <section className="panel category-rail-panel">
          <SectionTitle icon={Tag} title={t('home.categories')} />
          <div className="category-rail" aria-label={t('home.categories')}>
            <XButton type="button" variant="secondary" size="sm" label={t('common.all')} icon={<Layers3 size={16} />} onClick={() => onCategory('all')} />
            {categories.map((category) => (
              <XButton type="button" variant="secondary" size="sm" key={category.id} label={localizedName(category)} icon={<Tag size={16} />} onClick={() => onCategory(String(category.id))} />
            ))}
          </div>
        </section>
      )}

      <section className="panel">
        <SectionTitle icon={History} title={t('home.latest')} />
        <AppGrid
          apps={latest}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{
            title: t('home.emptyTitle'),
            body: isAuthenticated ? t('home.emptyBody') : t('home.emptyLoginBody'),
            action: { label: backstageLabel, icon: BackstageIcon, onClick: () => onNavigate('profile') },
          }}
        />
      </section>
      {collections.map((collection) => (
        <section className="panel" key={collection.id}>
          <SectionTitle icon={Layers3} title={collection.name} />
          <AppGrid apps={collection.apps || []} onOpen={onOpen} onInstall={onInstall} />
        </section>
      ))}
    </section>
  );
}
