import { Check, ChevronRight, Download, KeyRound, Link, PackagePlus, ShieldCheck, Star, Tag } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import type { StoreApp } from '../../shared/types';
import { cx, hasInstallableVersion } from '../../shared/utils';

export function AppGrid({
  apps,
  onOpen,
  onInstall,
  empty,
}: {
  apps: StoreApp[];
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  empty?: { title?: string; body?: string; action?: { label: string; icon?: LucideIcon; onClick: () => void } };
}) {
  const { t } = useTranslation();
  if (apps.length === 0) {
    return <EmptyState icon={PackagePlus} title={empty?.title || t('common.noApps')} body={empty?.body} action={empty?.action} />;
  }
  return (
    <div className="app-grid">
      {apps.map((app) => {
        const installable = hasInstallableVersion(app);
        const hasChecksum = Boolean(app.latestVersion?.sha256);
        return (
          <XClickableCard className="app-card" key={app.id} label={t('app.open', { name: app.name })} onClick={() => void onOpen(app)} padding={3}>
            <div className="app-open">
              <AppIcon src={app.iconUrl} seed={app.slug || app.name} title={app.name} />
              <div>
                <h3>{app.name}</h3>
                <p>{app.summary || app.description || t('common.lpkApp')}</p>
              </div>
              <ChevronRight size={18} />
            </div>
            <div className="app-meta">
              <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
              <span><Download size={14} /> {t('app.downloads', { count: app.downloadCount })}</span>
              {app.latestVersion?.sourceType && <span><Link size={14} /> {t('app.sourceType', { type: app.latestVersion.sourceType })}</span>}
            </div>
            <div className="app-readiness" aria-label={t('app.installSignals')}>
              <span className={cx('status-badge', installable ? 'approved' : 'blocked')}>
                <Download size={13} />
                {installable ? t('app.installReady') : t('app.installMissingVersion')}
              </span>
              <span className={cx('status-badge', hasChecksum ? 'synced' : 'unsynced')}>
                <ShieldCheck size={13} />
                {hasChecksum ? t('app.checksumReady') : t('app.checksumMissing')}
              </span>
              {app.installProtected && (
                <span className="status-badge pending">
                  <KeyRound size={13} />
                  {t('app.installPasswordRequired')}
                </span>
              )}
              {app.status === 'APPROVED' && (
                <span className="status-badge approved">
                  <Check size={13} />
                  {t('app.reviewed')}
                </span>
              )}
            </div>
            <XButton
              type="button"
              variant="primary"
              label={installable ? t('common.download') : t('common.unavailable')}
              icon={<Download size={17} />}
              isDisabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? `${t('common.download')} ${app.name}` : t('app.installUnavailable', { name: app.name })}
            />
          </XClickableCard>
        );
      })}
    </div>
  );
}
