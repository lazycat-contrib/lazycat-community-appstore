import { Archive, ChevronRight, Cloud, Download, Plus, RefreshCw, Tag } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstalledApplication, SourceApp } from '../../shared/types';
import {
  findInstalledApplication,
  hasInstallableVersion,
  localizedAppDescription,
  localizedAppName,
  localizedAppSummary,
  localizedCategory,
  sourceActionLabel,
  sourceInstallAction,
} from '../../shared/utils';

export function SourceAppGrid({
  apps,
  installedApps,
  onOpen,
  onInstall,
  onGoSources,
  showEmptyAction = true,
  emptyTitle,
  emptyBody,
  activeInstallKey,
}: {
  apps: SourceApp[];
  installedApps: InstalledApplication[];
  onOpen: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void | Promise<void>;
  onGoSources: () => void;
  showEmptyAction?: boolean;
  emptyTitle?: string;
  emptyBody?: string;
  activeInstallKey?: string;
}) {
  const { t } = useTranslation();
  const [pendingAppKey, setPendingAppKey] = useState('');

  async function runInstall(app: SourceApp, appKey: string) {
    if (pendingAppKey || activeInstallKey) return;
    setPendingAppKey(appKey);
    try {
      await Promise.resolve(onInstall(app));
    } finally {
      setPendingAppKey('');
    }
  }

  if (apps.length === 0) {
    return (
      <EmptyState
        icon={Cloud}
        title={emptyTitle || t('search.noSyncedApps')}
        body={emptyBody}
        action={showEmptyAction ? { label: t('search.noSyncedAppsAction'), icon: Plus, onClick: onGoSources } : undefined}
      />
    );
  }
  return (
    <div className="source-app-grid">
      {apps.map((app) => {
        const installable = hasInstallableVersion(app);
        const installedMatch = findInstalledApplication(app, installedApps);
        const installAction = sourceInstallAction(app, installedMatch);
        const actionLabel = sourceActionLabel(t, installAction);
        const isUpdateAvailable = installAction === 'update';
        const appName = localizedAppName(app);
        const appSummary = localizedAppSummary(app, localizedAppDescription(app, t('common.lpkApp')));
        const appKey = `${app.sourceId ?? app.sourceName}:${app.id}`;
        const isPending = pendingAppKey === appKey || activeInstallKey === appKey;
        return (
          <XClickableCard
            className="source-app-card client-source-app-card"
            key={appKey}
            label={t('app.open', { name: appName })}
            onClick={() => onOpen(app)}
            padding={3}
          >
            <div className="app-open">
              <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={appName} />
              <div>
                <h3>{appName}</h3>
                <p>{appSummary}</p>
              </div>
              <ChevronRight size={18} />
            </div>
            <div className="client-app-facts">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {localizedCategory(app, t('common.uncategorized'))}</span>
              <span><Archive size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
            </div>
            {installedMatch && (
              <StatusBadge
                tone={isUpdateAvailable ? 'stale' : 'synced'}
                label={isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
              />
            )}
            <XButton
              type="button"
              variant="primary"
              label={isPending ? t('installActivity.status.running') : actionLabel}
              icon={isPending ? <RefreshCw size={17} className="spin" /> : isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
              isDisabled={!installable || Boolean(pendingAppKey) || Boolean(activeInstallKey)}
              onClick={(event) => {
                event.stopPropagation();
                void runInstall(app, appKey);
              }}
              aria-label={installable ? `${actionLabel}: ${appName}` : t('app.installUnavailable', { name: appName })}
            />
          </XClickableCard>
        );
      })}
    </div>
  );
}
