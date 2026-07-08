import { AlertTriangle, CheckCircle2, ChevronLeft, ChevronRight, Info, Link, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { StatusBadge } from '../shared/components/StatusBadge';
import type { SiteAnnouncement } from '../shared/types';

function visibleAnnouncements(announcement?: SiteAnnouncement, announcements?: SiteAnnouncement[]) {
  const input = announcements && announcements.length > 0 ? announcements : announcement ? [announcement] : [];
  return input.filter((item) => item.enabled && Boolean(item.title || item.body));
}

function announcementKey(item: SiteAnnouncement) {
  return item.id ? `id:${item.id}:${item.updatedAt || ''}` : `${item.level}:${item.title || ''}:${item.body || ''}:${item.updatedAt || ''}`;
}

function levelIcon(level: SiteAnnouncement['level']) {
  if (level === 'warning') return <AlertTriangle size={16} />;
  if (level === 'success') return <CheckCircle2 size={16} />;
  return <Info size={16} />;
}

export function AnnouncementBanner({
  announcement,
  announcements,
  onDismiss,
}: {
  announcement?: SiteAnnouncement;
  announcements?: SiteAnnouncement[];
  onDismiss?: (announcement: SiteAnnouncement) => void;
}) {
  const { t } = useTranslation();
  const items = useMemo(() => visibleAnnouncements(announcement, announcements), [announcement, announcements]);
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
    }, 6800);
    return () => window.clearInterval(timer);
  }, [hasMultiple, items.length]);

  if (!current) return null;

  const tone = current.level || 'info';
  const sourceLabel = current.sourceName ? current.sourceName : '';

  return (
    <section className={`announcement-banner ${tone}`} aria-live="polite" data-announcement-key={announcementKey(current)}>
      <div className="announcement-content">
        <div className="announcement-meta">
          <StatusBadge tone={tone === 'warning' ? 'warning' : tone === 'success' ? 'success' : 'info'} icon={levelIcon(tone)} label={t(`site.announcementLevels.${tone}`)} />
          {sourceLabel && <span>{sourceLabel}</span>}
          {hasMultiple && <span>{t('site.announcementPosition', { current: index + 1, total: items.length })}</span>}
        </div>
        {current.title && <strong>{current.title}</strong>}
        {current.body && <p>{current.body}</p>}
      </div>
      <div className="announcement-actions">
        {hasMultiple && (
          <div className="announcement-pager" aria-label={t('site.announcementPager')}>
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
          </div>
        )}
        {current.linkUrl && (
          <XButton
            variant={tone === 'warning' ? 'primary' : 'secondary'}
            size="sm"
            label={current.linkLabel || t('site.announcementLink')}
            icon={<Link size={16} />}
            href={current.linkUrl}
            target="_blank"
            rel="noreferrer"
          />
        )}
        {onDismiss && (
          <XIconButton type="button" variant="ghost" label={t('site.dismissAnnouncement')} icon={<X size={17} />} onClick={() => onDismiss(current)} />
        )}
      </div>
    </section>
  );
}
