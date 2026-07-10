import { AlertCircle, Download, RefreshCw } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { AvatarIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstalledApplication, SourceApp } from '../../shared/types';
import { compareVersions, cx } from '../../shared/utils';
import { findStableSourceApp } from './clientUxState';

type InstalledRow = { item: InstalledApplication; source?: SourceApp };
type InstalledGroupKind = 'updates' | 'managed' | 'local';

export function InstalledAppsView({
  installedApps,
  sourceApps,
  installedState,
  installedError,
  installedReadinessBody,
  onLoadInstalled,
}: {
  installedApps: InstalledApplication[];
  sourceApps: SourceApp[];
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  installedReadinessBody: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
}) {
  const { t } = useTranslation();
  const installedGroups = useMemo(() => {
    const updates: InstalledRow[] = [];
    const managed: InstalledRow[] = [];
    const local: InstalledRow[] = [];
    for (const item of installedApps) {
      const source = findStableSourceApp(item, sourceApps);
      if (!source) {
        local.push({ item });
        continue;
      }
      if (item.version && source.latestVersion?.version && compareVersions(item.version, source.latestVersion.version) < 0) {
        updates.push({ item, source });
        continue;
      }
      managed.push({ item, source });
    }
    return { updates, managed, local };
  }, [installedApps, sourceApps]);
  const installedEmptyTitle = installedState === 'loaded' ? t('profile.installedEmptyLoaded') : t('profile.installedEmpty');
  const installedEmptyBody = installedState === 'idle' ? t('profile.installedIdleBody') : undefined;

  function InstalledGroup({
    title,
    tone,
    kind,
    rows,
  }: {
    title: string;
    tone: 'stale' | 'synced' | 'unsynced';
    kind: InstalledGroupKind;
    rows: InstalledRow[];
  }) {
    if (rows.length === 0) return null;
    return (
      <section className={cx('installed-group', `installed-group-${kind}`)}>
        <div className="installed-group-head">
          <h3>{title}</h3>
          <StatusBadge tone={tone} label={String(rows.length)} />
        </div>
        <div className="installed-app-grid">
          {rows.map(({ item, source }) => {
            const updateAvailable = Boolean(
              source?.latestVersion?.version && item.version && compareVersions(item.version, source.latestVersion.version) < 0,
            );
            return (
              <article className="installed-app-card" key={item.appid || item.title}>
                {item.icon ? (
                  <img className="installed-app-icon" src={item.icon} alt="" />
                ) : (
                  <AvatarIcon seed={item.appid || item.title || 'installed-app'} title={item.title || item.appid} size={42} />
                )}
                <div>
                  <strong>{item.title || item.appid || t('common.app')}</strong>
                  <span title={item.appid || undefined}>{item.appid || t('profile.installedAppIdMissing')}</span>
                  <small>{source ? t('profile.installedFromSource', { source: source.sourceName }) : t('profile.installedLocalExisting')}</small>
                </div>
                <div className="installed-app-meta">
                  <span>{item.version || '-'}</span>
                  <small>{updateAvailable ? t('app.updateAvailable') : item.instanceStatus || item.status || t('statusLabels.unknown')}</small>
                </div>
              </article>
            );
          })}
        </div>
      </section>
    );
  }

  return (
    <section className="panel install-center-panel">
      <div className="install-center-head">
        <div className="install-center-title">
          <AvatarIcon seed="lazycat-standalone-client" title={t('profile.clientTitle')} size={58} className="avatar-large" />
          <div>
            <span className="eyebrow subtle">{t('profile.clientDeviceOnly')}</span>
            <h2>{t('profile.clientInstalledTitle')}</h2>
            <p>{t('profile.clientInstalledHelp')}</p>
          </div>
        </div>
        <div className="install-center-actions">
          <div className={cx('installed-state', installedState)}>
            <StatusBadge tone={installedState === 'error' ? 'failed' : installedState === 'loaded' ? 'synced' : installedState === 'loading' ? 'pending' : 'unsynced'} label={t(`profile.installedState.${installedState}`)} />
            <small>{installedReadinessBody}</small>
          </div>
          <XButton
            type="button"
            variant="primary"
            label={installedState === 'loading' ? t('profile.readingInstalled') : t('profile.readInstalled')}
            icon={<RefreshCw size={18} className={installedState === 'loading' ? 'spin' : undefined} />}
            isDisabled={installedState === 'loading'}
            onClick={() => void onLoadInstalled()}
          />
        </div>
      </div>
      <div className="install-center-metrics" aria-label={t('profile.clientInstalledTitle')}>
        <div className={cx(installedGroups.updates.length > 0 && 'warning')}>
          <span>{t('profile.installedUpdates')}</span>
          <strong>{installedGroups.updates.length}</strong>
        </div>
        <div>
          <span>{t('profile.installedManaged')}</span>
          <strong>{installedGroups.managed.length}</strong>
        </div>
        <div>
          <span>{t('profile.installedLocalGroup')}</span>
          <strong>{installedGroups.local.length}</strong>
        </div>
      </div>
      {installedState === 'error' && (
        <p className="inline-alert" role="alert">
          <AlertCircle size={15} />
          <span>{installedError}</span>
        </p>
      )}
      {installedApps.length === 0 ? (
        <EmptyState icon={Download} title={installedEmptyTitle} body={installedEmptyBody} />
      ) : (
        <div className="installed-groups">
          <InstalledGroup title={t('profile.installedUpdates')} tone="stale" kind="updates" rows={installedGroups.updates} />
          <InstalledGroup title={t('profile.installedManaged')} tone="synced" kind="managed" rows={installedGroups.managed} />
          <InstalledGroup title={t('profile.installedLocalGroup')} tone="unsynced" kind="local" rows={installedGroups.local} />
        </div>
      )}
    </section>
  );
}
