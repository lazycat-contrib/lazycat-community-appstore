import { AlertCircle, Download, RefreshCw } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { AvatarIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstalledApplication, SourceApp } from '../../shared/types';
import { cx, findSourceForInstalled } from '../../shared/utils';

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
  const installedSummary = useMemo(() => {
    const versioned = installedApps.filter((item) => item.version).length;
    const statusKnown = installedApps.filter((item) => item.status || item.instanceStatus).length;
    const active = installedApps.filter((item) => /running|active|started/i.test(`${item.status || ''} ${item.instanceStatus || ''}`)).length;
    return { total: installedApps.length, versioned, statusKnown, active };
  }, [installedApps]);
  const installedEmptyTitle = installedState === 'loaded' ? t('profile.installedEmptyLoaded') : t('profile.installedEmpty');
  const installedEmptyBody = installedState === 'idle' ? t('profile.installedIdleBody') : undefined;

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
            icon={<RefreshCw size={18} />}
            isDisabled={installedState === 'loading'}
            onClick={() => void onLoadInstalled()}
          />
        </div>
      </div>
      <div className="install-center-metrics" aria-label={t('profile.clientInstalledTitle')}>
        <div>
          <span>{t('profile.installedTotal')}</span>
          <strong>{installedSummary.total}</strong>
        </div>
        <div>
          <span>{t('profile.installedVersioned')}</span>
          <strong>{installedSummary.versioned}</strong>
        </div>
        <div>
          <span>{t('profile.installedStatusKnown')}</span>
          <strong>{installedSummary.statusKnown}</strong>
        </div>
        <div>
          <span>{t('profile.installedActive')}</span>
          <strong>{installedSummary.active}</strong>
        </div>
      </div>
      {installedState === 'error' && (
        <p className="inline-alert">
          <AlertCircle size={15} />
          <span>{installedError}</span>
        </p>
      )}
      {installedApps.length === 0 ? (
        <EmptyState icon={Download} title={installedEmptyTitle} body={installedEmptyBody} />
      ) : (
        <div className="installed-app-grid">
          {installedApps.map((item) => {
            const sourceMatch = findSourceForInstalled(item, sourceApps);
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
                  <small>{sourceMatch ? t('profile.installedFromSource', { source: sourceMatch.sourceName }) : t('profile.installedLocalExisting')}</small>
                </div>
                <div className="installed-app-meta">
                  <span>{item.version || '-'}</span>
                  <small>{item.instanceStatus || item.status || t('statusLabels.unknown')}</small>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
