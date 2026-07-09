import { ChevronLeft, ChevronRight, ExternalLink } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SiteAd } from '../shared/types';
import { cx } from '../shared/utils';

function isAdCurrentlyVisible(item: SiteAd) {
  if (!item.enabled || (!item.title && !item.body && !item.imageUrl)) return false;
  const now = Date.now();
  if (item.startsAt && new Date(item.startsAt).getTime() > now) return false;
  if (item.endsAt && new Date(item.endsAt).getTime() <= now) return false;
  return true;
}

export function visibleSiteAds(ads?: SiteAd[]) {
  return (ads || []).filter(isAdCurrentlyVisible);
}

function adKey(item: SiteAd) {
  return item.id ? `id:${item.id}:${item.updatedAt || ''}` : `${item.title || ''}:${item.imageUrl || ''}:${item.updatedAt || ''}`;
}

export function AdSpot({ ads, className }: { ads?: SiteAd[]; className?: string }) {
  const { t } = useTranslation();
  const items = useMemo(() => visibleSiteAds(ads), [ads]);
  const [index, setIndex] = useState(0);
  const current = items[index] || items[0];
  const hasMultiple = items.length > 1;

  useEffect(() => {
    if (index >= items.length) setIndex(0);
  }, [index, items.length]);

  useEffect(() => {
    if (!hasMultiple) return;
    const timer = window.setInterval(() => {
      setIndex((currentIndex) => (currentIndex + 1) % items.length);
    }, 7200);
    return () => window.clearInterval(timer);
  }, [hasMultiple, items.length]);

  if (!current) {
    return null;
  }

  const hasImage = Boolean(current.imageUrl);
  const image = current.imageUrl ? <img src={current.imageUrl} alt="" loading="lazy" /> : null;

  return (
    <aside className={cx('ad-spot', !hasImage && 'ad-spot-text-only', className)} aria-label={t('ads.slot')} data-ad-key={adKey(current)}>
      {hasImage && (
        <div className="ad-spot-media">
          {current.linkUrl ? (
            <a href={current.linkUrl} target="_blank" rel="noreferrer" aria-label={current.title || current.linkLabel || t('ads.open')}>
              {image}
            </a>
          ) : image}
        </div>
      )}
      <div className="ad-spot-content">
        <div className="ad-spot-meta">
          <span>{t('ads.label')}</span>
          {current.sourceName && <small>{current.sourceName}</small>}
          {hasMultiple && <small>{t('site.announcementPosition', { current: index + 1, total: items.length })}</small>}
        </div>
        {current.title && <strong>{current.title}</strong>}
        {current.body && <p>{current.body}</p>}
        <div className="ad-spot-actions">
          {hasMultiple && (
            <span className="ad-spot-pager">
              <XIconButton
                type="button"
                variant="ghost"
                size="sm"
                label={t('common.previous')}
                icon={<ChevronLeft size={16} />}
                onClick={() => setIndex((currentIndex) => (currentIndex - 1 + items.length) % items.length)}
              />
              <XIconButton
                type="button"
                variant="ghost"
                size="sm"
                label={t('common.next')}
                icon={<ChevronRight size={16} />}
                onClick={() => setIndex((currentIndex) => (currentIndex + 1) % items.length)}
              />
            </span>
          )}
          {current.linkUrl && (
            <XButton
              variant="secondary"
              size="sm"
              label={current.linkLabel || t('ads.open')}
              icon={<ExternalLink size={15} />}
              href={current.linkUrl}
              target="_blank"
              rel="noreferrer"
            />
          )}
        </div>
      </div>
    </aside>
  );
}
