import { Link, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useTranslation } from 'react-i18next';
import type { SiteAnnouncement } from '../shared/types';

export function AnnouncementBanner({ announcement, onDismiss }: { announcement: SiteAnnouncement; onDismiss?: () => void }) {
  const { t } = useTranslation();
  const tone = announcement.level || 'info';
  return (
    <section className={`announcement-banner ${tone}`} aria-live="polite">
      <div>
        <span className="status-badge synced">{t(`site.announcementLevels.${tone}`)}</span>
        {announcement.title && <strong>{announcement.title}</strong>}
        {announcement.body && <p>{announcement.body}</p>}
      </div>
      <div className="announcement-actions">
        {announcement.linkUrl && (
          <XButton
            variant="secondary"
            size="sm"
            label={announcement.linkLabel || t('site.announcementLink')}
            icon={<Link size={16} />}
            href={announcement.linkUrl}
            target="_blank"
            rel="noreferrer"
          />
        )}
        {onDismiss && (
          <XIconButton type="button" variant="ghost" label={t('site.dismissAnnouncement')} icon={<X size={17} />} onClick={onDismiss} />
        )}
      </div>
    </section>
  );
}
