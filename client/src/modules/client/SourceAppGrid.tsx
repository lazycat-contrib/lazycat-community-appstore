import { Archive, CalendarClock, Cloud, Download, Plus, RefreshCw, Tag } from 'lucide-react';
import { useEffect, useRef, useState, type ComponentProps } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import { softwareUpdatedAt } from '../../shared/catalogUpdate';
import { EmptyState } from '../../shared/components/Feedback';
import { InstalledAnimalStamp } from './InstalledAnimalStamp';
import type { InstalledApplication, SourceApp } from '../../shared/types';
import {
  findInstalledApplication,
  formatDate,
  hasInstallableVersion,
  localizedAppDescription,
  localizedAppName,
  localizedAppSummary,
  localizedCategory,
  normalizeAppIdentity,
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
  emptyAction,
  activeInstallKey,
  completedInstallKey,
}: {
  apps: SourceApp[];
  installedApps: InstalledApplication[];
  onOpen: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void | Promise<void>;
  onGoSources: () => void;
  showEmptyAction?: boolean;
  emptyTitle?: string;
  emptyBody?: string;
  emptyAction?: ComponentProps<typeof EmptyState>['action'];
  activeInstallKey?: string;
  completedInstallKey?: string;
}) {
  const { t } = useTranslation();
  const [pendingAppKey, setPendingAppKey] = useState('');
  const [celebrateAppKey, setCelebrateAppKey] = useState('');
  const lastCompletedKey = useRef(completedInstallKey);

  useEffect(() => {
    if (!completedInstallKey) {
      lastCompletedKey.current = undefined;
      return;
    }
    if (lastCompletedKey.current === completedInstallKey) return;
    const completedApp = apps.find((app) => `${app.sourceId ?? app.sourceName}:${app.id}` === completedInstallKey);
    if (!completedApp || !findInstalledApplication(completedApp, installedApps)) return;
    lastCompletedKey.current = completedInstallKey;
    setCelebrateAppKey(completedInstallKey);
  }, [apps, completedInstallKey, installedApps]);

  useEffect(() => {
    if (!celebrateAppKey) return;
    const timer = window.setTimeout(() => setCelebrateAppKey(''), 300);
    return () => window.clearTimeout(timer);
  }, [celebrateAppKey]);

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
        action={emptyAction ?? (showEmptyAction ? { label: t('search.noSyncedAppsAction'), icon: Plus, onClick: onGoSources } : undefined)}
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
        const statusLabel = isUpdateAvailable ? t('app.updateAvailable') : t('app.installed');
        const appName = localizedAppName(app);
        const appSummary = localizedAppSummary(app, localizedAppDescription(app, t('common.lpkApp')));
        const appKey = `${app.sourceId ?? app.sourceName}:${app.id}`;
        const isPending = pendingAppKey === appKey || activeInstallKey === appKey;
        return (
          <XClickableCard
            className="source-app-card client-source-app-card"
            key={appKey}
            label={`${t('app.open', { name: appName })}${installedMatch ? ` · ${statusLabel}` : ''}`}
            onClick={() => onOpen(app)}
            padding={3}
          >
            <div className="app-open">
              <span className="client-app-icon-stamp">
                <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={appName} size={64} />
                {installedMatch && (
                  <InstalledAnimalStamp
                    identity={normalizeAppIdentity(installedMatch.appid) || normalizeAppIdentity(app.packageId) || appKey}
                    updateAvailable={isUpdateAvailable}
                    label={statusLabel}
                    celebrate={celebrateAppKey === appKey}
                  />
                )}
              </span>
              <div>
                <h3>{appName}</h3>
                <p>{appSummary}</p>
              </div>
            </div>
            <div className="client-app-facts">
              <span><Cloud size={14} aria-hidden="true" /> {app.sourceName}</span>
              <span><Tag size={14} aria-hidden="true" /> {localizedCategory(app, t('common.uncategorized'))}</span>
              <span><Archive size={14} aria-hidden="true" /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
              <span><CalendarClock size={14} aria-hidden="true" /> {t('app.softwareUpdatedAt', { time: formatDate(softwareUpdatedAt(app)) })}</span>
            </div>
            <XButton
              className="client-app-primary-action"
              data-action={installAction}
              type="button"
              variant={isUpdateAvailable ? 'primary' : 'secondary'}
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
