import { History, Layers3, LogIn, PackagePlus, Search, Tag } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { API_BASE } from '../../config';
import { AdSpot, visibleSiteAds } from '../../components/AdSpot';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, Collection, SiteAd, SiteProfile, StoreApp } from '../../shared/types';
import { AppGrid } from './AppGrid';
import { CategoryBrowser } from './CategoryBrowser';

export function StorefrontHome({
  apps,
  appCount,
  categories,
  collections,
  siteProfile,
  onOpen,
  onInstall,
  onNavigate,
  onSubmitApp,
  activeCategory,
  onCategory,
  isAuthenticated,
  ads,
}: {
  apps: StoreApp[];
  appCount?: number;
  categories: Category[];
  collections: Collection[];
  siteProfile: SiteProfile;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onNavigate: (tab: 'search' | 'profile') => void;
  onSubmitApp: () => void;
  activeCategory: string;
  onCategory: (category: string) => void;
  isAuthenticated: boolean;
  ads?: SiteAd[];
}) {
  const { t } = useTranslation();
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  const approvedCount = appCount ?? apps.filter((app) => app.status === 'APPROVED').length;
  const sourceFeedURL = siteProfile.sourceUrl || `${API_BASE || window.location.origin}/source/v2/index.json`;
  const BackstageIcon = isAuthenticated ? PackagePlus : LogIn;
  const backstageLabel = isAuthenticated ? t('home.submitApp') : t('topbar.login');
  const visibleAds = visibleSiteAds(ads);

  return (
    <section className="page-grid storefront-page">
      <div className={`hero-band storefront-hero${visibleAds.length === 0 ? ' storefront-hero-without-ad' : ''}`}>
        <div className="storefront-hero-copy">
          <span className="eyebrow">{t('home.eyebrow')}</span>
          <h1>{siteProfile.title || t('home.title')}</h1>
          <p>{siteProfile.subtitle || t('home.body')}</p>
          <div className="hero-actions">
            <XButton type="button" variant="primary" label={t('nav.discover')} icon={<Search size={18} />} onClick={() => onNavigate('search')} />
            <XButton type="button" variant="primary" label={backstageLabel} icon={<BackstageIcon size={18} />} onClick={onSubmitApp} />
          </div>
        </div>
        {visibleAds.length > 0 && <AdSpot ads={visibleAds} className="storefront-hero-ad" />}
      </div>

      <section className="store-metrics" aria-label={t('nav.store')}>
        <XCard className="metric-card" padding={4}>
          <span>{t('common.apps')}</span>
          <strong>{approvedCount}</strong>
          <small>{t('home.approvedCount', { count: approvedCount })}</small>
        </XCard>
        <XCard className="metric-card" padding={4}>
          <span>{t('common.category')}</span>
          <strong>{categories.length}</strong>
          <small>{t('home.categoryCount', { count: categories.length })}</small>
        </XCard>
        <XCard className="metric-card source-feed-card" padding={4}>
          <span>{t('home.sourceUrl')}</span>
          <XCodeBlock code={sourceFeedURL} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />
        </XCard>
      </section>

      {categories.length > 0 && (
        <section className="panel category-rail-panel">
          <SectionTitle icon={Tag} title={t('home.categories')} />
          <CategoryBrowser categories={categories} activeCategory={activeCategory} onCategory={onCategory} />
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
            action: { label: backstageLabel, icon: BackstageIcon, onClick: onSubmitApp },
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
