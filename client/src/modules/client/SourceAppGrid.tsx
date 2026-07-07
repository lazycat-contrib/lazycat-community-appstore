import { Archive, Check, ChevronRight, Cloud, Download, KeyRound, Link, Plus, RefreshCw, ShieldCheck, Star, Tag } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
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
}: {
  apps: SourceApp[];
  installedApps: InstalledApplication[];
  onOpen: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void;
  onGoSources: () => void;
  showEmptyAction?: boolean;
  emptyTitle?: string;
  emptyBody?: string;
}) {
  const { t } = useTranslation();
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
        const hasChecksum = Boolean(app.latestVersion?.sha256);
        const hasSize = Boolean(app.latestVersion?.size && app.latestVersion.size > 0);
        const installedMatch = findInstalledApplication(app, installedApps);
        const installAction = sourceInstallAction(app, installedMatch);
        const isUpdateAvailable = installAction === 'update';
        const appName = localizedAppName(app);
        const appSummary = localizedAppSummary(app, localizedAppDescription(app, t('common.lpkApp')));
        return (
          <XClickableCard
            className="source-app-card"
            key={`${app.sourceId || app.sourceName}-${app.id}`}
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
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <XBadge variant="neutral" icon={<Tag size={13} />} label={localizedCategory(app, t('common.uncategorized'))} />
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
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
              <XBadge variant={hasSize ? 'success' : 'warning'} icon={<Archive size={13} />} label={hasSize ? t('app.sizeReady') : t('app.sizeMissing')} />
              {installedMatch && (
                <XBadge
                  variant={isUpdateAvailable ? 'warning' : 'success'}
                  icon={isUpdateAvailable ? <RefreshCw size={13} /> : <Check size={13} />}
                  label={isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
                />
              )}
            </div>
            <XButton
              type="button"
              variant="primary"
              label={sourceActionLabel(t, installAction)}
              icon={isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
              isDisabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? t('app.install', { name: appName }) : t('app.installUnavailable', { name: appName })}
            />
          </XClickableCard>
        );
      })}
    </div>
  );
}
