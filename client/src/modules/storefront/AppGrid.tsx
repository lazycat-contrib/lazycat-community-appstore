import { ChevronRight, Download, PackagePlus, Sparkles, Star, Tag } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { type MouseEvent, useState } from 'react';
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
  onInstall: (app: StoreApp) => void | Promise<void>;
  empty?: { title?: string; body?: string; action?: { label: string; icon?: LucideIcon; onClick: () => void } };
}) {
  const { t } = useTranslation();
  const [pendingAppID, setPendingAppID] = useState<number | null>(null);

  async function installApp(event: MouseEvent<HTMLButtonElement>, app: StoreApp) {
    event.stopPropagation();
    if (pendingAppID !== null) return;
    setPendingAppID(app.id);
    try {
      await onInstall(app);
    } finally {
      setPendingAppID((current) => (current === app.id ? null : current));
    }
  }

  if (apps.length === 0) {
    return <EmptyState icon={PackagePlus} title={empty?.title || t('common.noApps')} body={empty?.body} action={empty?.action} />;
  }
  return (
    <div className="app-grid">
      {apps.map((app) => {
        const installable = hasInstallableVersion(app);
        const isInstalling = pendingAppID === app.id;
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
              <span><Sparkles size={14} /> {t('app.rating', { score: app.rating?.score || 0, count: app.rating?.voteCount || 0 })}</span>
            </div>
            <XButton
              className="app-card-primary-action"
              type="button"
              variant="primary"
              label={installable ? `${t('common.download')} ${appName}` : t('app.installUnavailable', { name: appName })}
              icon={<Download size={17} />}
              isDisabled={!installable || pendingAppID !== null}
              isLoading={isInstalling}
              onClick={(event) => void installApp(event, app)}
            >
              {installable ? t('common.download') : t('common.unavailable')}
            </XButton>
          </XClickableCard>
        );
      })}
    </div>
  );
}
