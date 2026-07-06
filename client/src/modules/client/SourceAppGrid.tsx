import { Archive, Check, ChevronRight, Cloud, Download, KeyRound, Link, Plus, RefreshCw, ShieldCheck, Star, Tag } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ClickableCard as XClickableCard } from '@astryxdesign/core/ClickableCard';
import { AppIcon } from '../../components/AppIcon';
import type { InstalledApplication, SourceApp } from '../../shared/types';
import {
  cx,
  findInstalledApplication,
  hasInstallableVersion,
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
      <div className="empty-state action-empty">
        <Cloud size={28} />
        <strong>{emptyTitle || t('search.noSyncedApps')}</strong>
        {emptyBody && <p>{emptyBody}</p>}
        {showEmptyAction && (
          <XButton type="button" variant="secondary" label={t('search.noSyncedAppsAction')} icon={<Plus size={18} />} onClick={onGoSources} />
        )}
      </div>
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
        return (
          <XClickableCard
            className="source-app-card"
            key={`${app.sourceId || app.sourceName}-${app.id}`}
            label={t('app.open', { name: app.name })}
            onClick={() => onOpen(app)}
            padding={3}
          >
            <div className="app-open">
              <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} />
              <div>
                <h3>{app.name}</h3>
                <p>{app.summary || t('common.lpkApp')}</p>
              </div>
              <ChevronRight size={18} />
            </div>
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {localizedCategory(app, t('common.uncategorized'))}</span>
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
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
              <span className={cx('status-badge', hasSize ? 'synced' : 'unsynced')}>
                <Archive size={13} />
                {hasSize ? t('app.sizeReady') : t('app.sizeMissing')}
              </span>
              {installedMatch && (
                <span className={cx('status-badge', isUpdateAvailable ? 'pending' : 'synced')}>
                  {isUpdateAvailable ? <RefreshCw size={13} /> : <Check size={13} />}
                  {isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
                </span>
              )}
            </div>
            <XButton
              type="button"
              variant="primary"
              label={sourceActionLabel(t, installAction)}
              icon={isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
              isDisabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? t('app.install', { name: app.name }) : t('app.installUnavailable', { name: app.name })}
            />
          </XClickableCard>
        );
      })}
    </div>
  );
}
