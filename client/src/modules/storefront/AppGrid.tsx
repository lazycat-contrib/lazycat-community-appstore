import { Check, ChevronRight, Download, KeyRound, Link, PackagePlus, ShieldCheck, Star, Tag } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import type { StoreApp } from '../../shared/types';
import { hasInstallableVersion, localizedAppDescription, localizedAppName, localizedAppSummary, localizedCategory } from '../../shared/utils';

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
        const appName = localizedAppName(app);
        const appSummary = localizedAppSummary(app, localizedAppDescription(app, t('common.lpkApp')));
        return (
          <XClickableCard className="app-card" key={app.id} label={t('app.open', { name: appName })} onClick={() => void onOpen(app)} padding={3}>
            <div className="app-open">
              <AppIcon src={app.iconUrl} seed={app.slug || app.name} title={appName} />
              <div>
                <h3>{appName}</h3>
                <p>{appSummary}</p>
              </div>
              <ChevronRight size={18} />
            </div>
            <div className="app-meta">
              <XBadge variant="neutral" icon={<Tag size={13} />} label={localizedCategory(app, t('common.uncategorized'))} />
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
              <span><Download size={14} /> {t('app.downloads', { count: app.downloadCount })}</span>
              {app.latestVersion?.sourceType && <span><Link size={14} /> {t('app.sourceType', { type: app.latestVersion.sourceType })}</span>}
            </div>
            <div className="app-readiness" aria-label={t('app.installSignals')}>
              <XBadge
                variant={installable ? 'success' : 'error'}
                icon={<Download size={13} />}
                label={installable ? t('app.installReady') : t('app.installMissingVersion')}
              />
              <XBadge
                variant={hasChecksum ? 'success' : 'warning'}
                icon={<ShieldCheck size={13} />}
                label={hasChecksum ? t('app.checksumReady') : t('app.checksumMissing')}
              />
              {app.installProtected && (
                <XBadge variant="warning" icon={<KeyRound size={13} />} label={t('app.installPasswordRequired')} />
              )}
              {app.status === 'APPROVED' && (
                <XBadge variant="success" icon={<Check size={13} />} label={t('app.reviewed')} />
              )}
            </div>
            <XButton
              type="button"
              variant="primary"
              label={installable ? t('common.download') : t('common.unavailable')}
              icon={<Download size={17} />}
              isDisabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? `${t('common.download')} ${appName}` : t('app.installUnavailable', { name: appName })}
            />
          </XClickableCard>
        );
      })}
    </div>
  );
}
