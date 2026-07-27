import { Copy, ExternalLink, History, Layers3, Link, LogIn, PackagePlus, Search, Tag } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { API_BASE } from '../../config';
import { AdSpot, visibleSiteAds } from '../../components/AdSpot';
import { softwareUpdatedAtMillis } from '../../shared/catalogUpdate';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, Collection, SiteAd, SiteProfile, StoreApp } from '../../shared/types';
import { localizedAppName } from '../../shared/utils';
import { AppGrid } from './AppGrid';
import { CategoryBrowser } from './CategoryBrowser';

type SourceCopyStatus = 'idle' | 'copied' | 'failed' | 'unsupported';
const PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];

export function StorefrontHome({
  apps,
  appCount,
  categories,
  collections,
  siteProfile,
  lazycatInstall,
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
  lazycatInstall: boolean;
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
  const [sourceCopyStatus, setSourceCopyStatus] = useState<SourceCopyStatus>('idle');
  const [latestPage, setLatestPage] = useState(1);
  const [latestPageSize, setLatestPageSize] = useState(siteProfile.defaultPageSize || 24);
  const latest = useMemo(() => [...apps].sort((a, b) => {
    const updateDelta = softwareUpdatedAtMillis(b) - softwareUpdatedAtMillis(a);
    return updateDelta !== 0 ? updateDelta : localizedAppName(a).localeCompare(localizedAppName(b));
  }), [apps]);
  const latestTotalPages = Math.max(1, Math.ceil(latest.length / latestPageSize));
  const currentLatestPage = Math.max(1, Math.min(latestPage, latestTotalPages));
  const latestStart = (currentLatestPage - 1) * latestPageSize;
  const pagedLatest = latest.slice(latestStart, latestStart + latestPageSize);
  const approvedCount = appCount ?? apps.filter((app) => app.status === 'APPROVED').length;
  const sourceFeedURL = siteProfile.sourceUrl || `${API_BASE || window.location.origin}/source/v2/index.json`;
  const BackstageIcon = isAuthenticated ? PackagePlus : LogIn;
  const backstageLabel = isAuthenticated ? t('home.submitApp') : t('topbar.login');
  const visibleAds = visibleSiteAds(ads);
  const sourceCopyMessage = sourceCopyStatus === 'copied'
    ? t('home.sourceCopied')
    : sourceCopyStatus === 'unsupported'
      ? t('home.copySourceUnsupported')
      : sourceCopyStatus === 'failed'
        ? t('home.copySourceFailed')
        : '';

  useEffect(() => {
    if (latestPage === currentLatestPage) return;
    setLatestPage(currentLatestPage);
  }, [currentLatestPage, latestPage]);

  async function copySourceFeed() {
    if (!navigator.clipboard?.writeText) {
      setSourceCopyStatus('unsupported');
      return;
    }
    try {
      await navigator.clipboard.writeText(sourceFeedURL);
      setSourceCopyStatus('copied');
    } catch {
      setSourceCopyStatus('failed');
    }
  }

  function openSourceFeed() {
    window.open(sourceFeedURL, '_blank', 'noopener,noreferrer');
  }

  return (
    <section className="page-grid storefront-page">
      <div className={`hero-band storefront-hero${visibleAds.length === 0 ? ' storefront-hero-without-ad' : ''}`}>
        <div className="storefront-hero-copy">
          <span className="eyebrow">{t('home.eyebrow')}</span>
          <h1>{siteProfile.title || t('home.title')}</h1>
          <p>{siteProfile.subtitle || t('home.body')}</p>
          <div className="hero-actions">
            <XButton type="button" variant="primary" label={t('nav.discover')} icon={<Search size={18} />} onClick={() => onNavigate('search')} />
            <XButton type="button" variant="secondary" label={backstageLabel} icon={<BackstageIcon size={18} />} onClick={onSubmitApp} />
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
      </section>

      <section className="panel storefront-subscribe-panel" aria-labelledby="storefront-subscribe-title">
        <div className="storefront-subscribe-copy">
          <div className="section-title">
            <Link size={19} />
            <h2 id="storefront-subscribe-title">{t('home.openSourceFeed')}</h2>
          </div>
          <p>{t('sources.subtitle')}</p>
          <div className="storefront-source-meta">
            <span>{t('common.version')}</span>
            <strong>v2</strong>
          </div>
        </div>
        <div className="storefront-subscribe-command">
          <XCodeBlock code={sourceFeedURL} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />
          <div className="storefront-subscribe-actions">
            <XButton type="button" variant="secondary" label={t('home.copySourceFeed')} icon={<Copy size={17} />} onClick={() => void copySourceFeed()} />
            <XButton type="button" variant="secondary" label={t('home.openSourceFeed')} icon={<ExternalLink size={17} />} onClick={openSourceFeed} />
          </div>
          {sourceCopyMessage && (
            <p className="storefront-copy-status" role="status" aria-live="polite" data-tone={sourceCopyStatus}>
              {sourceCopyMessage}
            </p>
          )}
        </div>
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
          apps={pagedLatest}
          onOpen={onOpen}
          onInstall={onInstall}
          lazycatInstall={lazycatInstall}
          empty={{
            title: t('home.emptyTitle'),
            body: isAuthenticated ? t('home.emptyBody') : t('home.emptyLoginBody'),
            action: { label: backstageLabel, icon: BackstageIcon, onClick: onSubmitApp },
          }}
        />
        {latest.length > latestPageSize && (
          <XPagination
            className="list-pagination"
            page={currentLatestPage}
            onChange={setLatestPage}
            totalItems={latest.length}
            pageSize={latestPageSize}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            onPageSizeChange={(value) => {
              setLatestPage(1);
              setLatestPageSize(value);
            }}
            variant="pages"
            size="sm"
            label={t('pagination.label')}
          />
        )}
      </section>
      {collections.map((collection) => (
        <section className="panel" key={collection.id}>
          <SectionTitle icon={Layers3} title={collection.name} />
          <AppGrid apps={collection.apps || []} onOpen={onOpen} onInstall={onInstall} lazycatInstall={lazycatInstall} />
        </section>
      ))}
    </section>
  );
}
