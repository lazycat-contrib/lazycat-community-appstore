import { AlertCircle, Download, RefreshCw, X } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { AvatarIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstalledApplication, SourceApp, UpdateQueueResult } from '../../shared/types';
import { compareVersions, cx } from '../../shared/utils';
import { buildUpdateConfirmation, findStableSourceApp } from './clientUxState';

type InstalledRow = { item: InstalledApplication; source?: SourceApp };
type InstalledGroupKind = 'updates' | 'managed' | 'local';

export function InstalledAppsView({
  installedApps,
  sourceApps,
  installedState,
  installedError,
  installedReadinessBody,
  onLoadInstalled,
  onRunUpdates,
  onCancelUpdates,
  updateQueueResult,
  isUpdateQueueRunning = false,
}: {
  installedApps: InstalledApplication[];
  sourceApps: SourceApp[];
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  installedReadinessBody: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onRunUpdates?: () => Promise<void>;
  onCancelUpdates?: () => Promise<void>;
  updateQueueResult?: UpdateQueueResult | null;
  isUpdateQueueRunning?: boolean;
}) {
  const { t } = useTranslation();
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
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
  const updateConfirmation = useMemo(() => buildUpdateConfirmation(installedGroups.updates), [installedGroups.updates]);
  const canRunUpdates = Boolean(onRunUpdates && updateConfirmation.eligible.length > 0 && installedState === 'loaded');
  const updateItems = updateQueueResult?.items || [];
  const updateSummary = updateItems.reduce<Record<string, number>>((summary, item) => {
    summary[item.status] = (summary[item.status] || 0) + 1;
    return summary;
  }, {});

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
    <>
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
		  {canRunUpdates && (
			<XButton
			  type="button"
			  variant="secondary"
			  label={isUpdateQueueRunning ? t('updateQueue.running') : t('updateQueue.updateAll', { count: updateConfirmation.eligible.length })}
			  icon={<RefreshCw size={18} className={isUpdateQueueRunning ? 'spin' : undefined} />}
			  isDisabled={isUpdateQueueRunning}
			  onClick={() => setIsConfirmOpen(true)}
			/>
		  )}
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
	  {updateQueueResult && (
		<aside className={cx('update-queue-summary', `is-${updateQueueResult.status}`)} aria-live="polite">
		  <div>
			<strong>{t(`updateQueue.result.${updateQueueResult.status}`)}</strong>
			<span>{updateQueueResult.error || t('updateQueue.summary', { count: updateItems.length })}</span>
		  </div>
		  {updateItems.length > 0 && (
			<small>{Object.entries(updateSummary).map(([status, count]) => t('updateQueue.itemCount', { status: t(`updateQueue.itemStates.${status}`), count })).join(' · ')}</small>
		  )}
		  {isUpdateQueueRunning && onCancelUpdates && (
			<XButton type="button" variant="secondary" size="sm" label={t('updateQueue.cancel')} icon={<X size={16} />} onClick={() => void onCancelUpdates()} />
		  )}
		</aside>
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
	  {isConfirmOpen && (
		<ModalLayer onClose={() => setIsConfirmOpen(false)} purpose="form" width="min(480px, calc(100vw - 36px))">
		  <section className="modal-panel update-confirm-dialog" aria-labelledby="update-confirm-title">
			<div>
			  <h2 id="update-confirm-title">{t('updateQueue.confirmTitle')}</h2>
			  <p>{t('updateQueue.confirmBody', { count: updateConfirmation.eligible.length })}</p>
			  {updateConfirmation.skipped.length > 0 && <small>{t('updateQueue.protectedSkipped', { count: updateConfirmation.skipped.length })}</small>}
			</div>
			<div className="dialog-actions">
			  <XButton type="button" variant="secondary" label={t('common.cancel')} onClick={() => setIsConfirmOpen(false)} />
			  <XButton type="button" variant="primary" label={t('updateQueue.start')} icon={<RefreshCw size={17} />} onClick={() => {
				setIsConfirmOpen(false);
				void onRunUpdates?.();
			  }} />
			</div>
		  </section>
		</ModalLayer>
	  )}
	</>
  );
}
