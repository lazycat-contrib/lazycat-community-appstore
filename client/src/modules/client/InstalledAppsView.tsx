import { AlertCircle, CalendarClock, CheckCircle2, Download, PackageOpen, RefreshCw, ShieldCheck, XCircle } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { Tooltip as XTooltip } from '@astryxdesign/core/Tooltip';
import { AvatarIcon } from '../../components/AppIcon';
import { EmptyState } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { ClientSettings, InstalledApplication, SourceApp, SourceSubscription, UpdateQueueRequest, UpdateQueueResult } from '../../shared/types';
import { compareVersions, cx, formatDate, sourceMirrorOptions } from '../../shared/utils';
import { autoUpdatePolicyPresentation, autoUpdateSchedulePresentation, buildUpdateCandidateSnapshot, buildUpdateConfirmation, findStableSourceApp, installedRuntimeStatusPresentation, isUpdateQueueActive, orderClientUpdateRowsLast } from './clientUxState';

type InstalledRow = { item: InstalledApplication; source?: SourceApp };
type InstalledGroupKind = 'updates' | 'managed' | 'local';

export function InstalledAppsView({
  installedApps,
  sourceApps,
  sources,
  autoUpdateSettings,
  installedState,
  installedError,
  installedReadinessBody,
  onLoadInstalled,
  onSetAutoUpdatePolicy,
  autoUpdatePolicySaving = new Set(),
  onRunUpdates,
  updateQueueResult,
  isUpdateQueueRunning = false,
}: {
  installedApps: InstalledApplication[];
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  autoUpdateSettings?: Pick<ClientSettings, 'autoUpdateEnabled' | 'autoUpdateIntervalMinutes' | 'lastAutoUpdateAt' | 'autoUpdateScheduleState' | 'nextAutoUpdateAt'>;
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  installedReadinessBody: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onSetAutoUpdatePolicy?: (packageID: string, enabled: boolean) => Promise<void>;
  autoUpdatePolicySaving?: Set<string>;
  onRunUpdates?: (options?: UpdateQueueRequest) => Promise<void>;
  updateQueueResult?: UpdateQueueResult | null;
  isUpdateQueueRunning?: boolean;
}) {
  const { t } = useTranslation();
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [hasStarted, setHasStarted] = useState(false);
  const [mirrorOverrides, setMirrorOverrides] = useState<Record<string, { downloadMirrorId: string; rawMirrorId: string }>>({});
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
    return { updates: orderClientUpdateRowsLast(updates), managed, local };
  }, [installedApps, sourceApps]);
  const installedEmptyTitle = installedState === 'loaded'
    ? t('profile.installedEmptyLoaded')
    : installedState === 'error'
      ? t('profile.clientInstalledError')
      : t('profile.installedEmpty');
  const installedEmptyBody = installedState === 'idle' ? t('profile.installedIdleBody') : undefined;
  const updateConfirmation = useMemo(() => buildUpdateConfirmation(installedGroups.updates), [installedGroups.updates]);
  const updateCandidates = useMemo(() => buildUpdateCandidateSnapshot(installedGroups.updates), [installedGroups.updates]);
  const canRunUpdates = Boolean(onRunUpdates && updateConfirmation.eligible.length > 0 && installedState === 'loaded');
  const updateItems = updateQueueResult?.items || [];
  const updateSummary = updateItems.reduce<Record<string, number>>((summary, item) => {
    summary[item.status] = (summary[item.status] || 0) + 1;
    return summary;
  }, {});
  const updateSources = useMemo(() => {
    const ids = new Set(installedGroups.updates.map(({ source }) => Number(source?.sourceId)).filter((id) => id > 0));
    return sources.filter((source) => ids.has(Number(source.id)));
  }, [installedGroups.updates, sources]);
  const currentItem = updateItems.find((item) => item.status === 'running');
  const completedCount = updateItems.filter((item) => ['success', 'failed', 'cancelled'].includes(item.status)).length;
  const currentNumber = currentItem ? Math.min(completedCount + 1, updateItems.length) : completedCount;
  const queueStatus = updateQueueResult?.status || 'running';
  const queueIsActive = isUpdateQueueActive(queueStatus);
  const queueProgress = updateItems.length > 0 ? Math.round((completedCount / updateItems.length) * 100) : 0;
  const queueTone = queueStatus === 'failed'
    ? 'failed'
    : queueStatus === 'partial' || queueStatus === 'cancelled' || queueStatus === 'requires_password'
      ? 'stale'
      : queueStatus === 'success' || queueStatus === 'no_updates'
        ? 'synced'
        : 'pending';
  const updateSourceChoices = updateSources.map((source) => ({
    source,
    downloadOptions: sourceMirrorOptions(source, 'download', t('installOptions.direct'), t('installOptions.autoMirror')),
    rawOptions: sourceMirrorOptions(source, 'raw', t('installOptions.direct'), t('installOptions.autoMirror')),
  })).filter(({ downloadOptions, rawOptions }) => downloadOptions.length > 1 || rawOptions.length > 1);
  const updateSourceCount = new Set(updateCandidates.map((candidate) => candidate.sourceId)).size;
  const autoUpdateSchedule = autoUpdateSchedulePresentation({
    enabled: autoUpdateSettings?.autoUpdateEnabled ?? false,
    intervalMinutes: autoUpdateSettings?.autoUpdateIntervalMinutes ?? 60,
    lastRunAt: autoUpdateSettings?.lastAutoUpdateAt,
    scheduleState: autoUpdateSettings?.autoUpdateScheduleState,
    nextRunAt: autoUpdateSettings?.nextAutoUpdateAt,
  });
  const nextAutoUpdateLabel = autoUpdateSchedule.state === 'scheduled'
    ? formatDate(autoUpdateSchedule.nextRunAt)
    : t(`clientSettings.autoUpdateSchedule.${autoUpdateSchedule.state}`);

  function updateItemTone(status: string) {
    if (status === 'success') return 'synced';
    if (status === 'failed') return 'failed';
    if (status === 'cancelled') return 'stale';
    if (status === 'running') return 'pending';
    return 'neutral';
  }

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
            const updatePolicy = autoUpdatePolicyPresentation(item.autoUpdateEnabled);
            const packageID = item.appid || source?.packageId || '';
            const isPolicySaving = autoUpdatePolicySaving.has(packageID.trim().toLowerCase());
            const runtimeStatus = installedRuntimeStatusPresentation(item.instanceStatus || item.status);
            const runtimeStatusTone = {
              running: 'synced',
              stopped: 'neutral',
              paused: 'warning',
              processing: 'pending',
              error: 'failed',
              unknown: 'unsynced',
            }[runtimeStatus.key];
            return (
              <article className="installed-app-card" key={item.appid || item.title}>
                {item.icon ? (
                  <img className="installed-app-icon" src={item.icon} alt="" />
                ) : (
                  <AvatarIcon seed={item.appid || item.title || 'installed-app'} title={item.title || item.appid} size={42} />
                )}
                <div className="installed-app-identity">
                  <strong title={item.title || item.appid || undefined}>{item.title || item.appid || t('common.app')}</strong>
                  <span title={item.appid || undefined}>{item.appid || t('profile.installedAppIdMissing')}</span>
                  <small>{source ? t('profile.installedFromSource', { source: source.sourceName }) : t('profile.installedLocalExisting')}</small>
                </div>
                <div className="installed-app-meta">
                  <span>{item.version || '-'}</span>
                  {updateAvailable ? (
                    <StatusBadge tone="stale" label={t('app.updateAvailable')} />
                  ) : (
                    <XTooltip content={runtimeStatus.raw || t('installedRuntimeStatus.unknownTip')} placement="above" delay={180}>
                      <div className="installed-runtime-status-tip">
                        <StatusBadge tone={runtimeStatusTone} label={t(`installedRuntimeStatus.${runtimeStatus.key}`)} />
                      </div>
                    </XTooltip>
                  )}
                </div>
                {source && packageID && onSetAutoUpdatePolicy && (
                  <div className="installed-auto-update-control">
                    <XSwitch
                      label={t('updatePolicy.autoUpdate')}
                      labelTooltip={t(`updatePolicy.states.${updatePolicy.state}`)}
                      value={updatePolicy.enabled}
                      isDisabled={isPolicySaving}
                      disabledMessage={isPolicySaving ? t('updatePolicy.saving') : undefined}
                      onChange={(checked) => void onSetAutoUpdatePolicy(packageID, checked)}
                    />
                  </div>
                )}
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
            label={installedState === 'loading' ? t('profile.readingInstalled') : t('common.refresh')}
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
			  isDisabled={false}
			  onClick={() => {
				setHasStarted(isUpdateQueueRunning);
				setIsConfirmOpen(true);
              }}
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
	  {updateConfirmation.skipped.length > 0 && (
		<p className="inline-alert" role="status">
		  <AlertCircle size={15} />
		  <span>{t('updateQueue.protectedPending', { count: updateConfirmation.skipped.length })}</span>
		</p>
	  )}
	  {updateQueueResult && (
		<aside className={cx('update-queue-summary', `is-${updateQueueResult.status}`)} aria-live="polite">
		  <div>
			<strong>{t(`updateQueue.result.${updateQueueResult.status}`)}</strong>
			<span>{updateQueueResult.error || (updateQueueResult.status === 'running' && updateItems.length === 0
			  ? t('updateQueue.checkingSources')
			  : updateQueueResult.passwordRequired
				? t('updateQueue.passwordRequired', { count: updateQueueResult.passwordRequired })
				: t('updateQueue.summary', { count: updateItems.length }))}</span>
		  </div>
		  {updateItems.length > 0 && (
			<small>{Object.entries(updateSummary).map(([status, count]) => t('updateQueue.itemCount', { status: t(`updateQueue.itemStates.${status}`), count })).join(' · ')}</small>
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
			<ModalLayer onClose={() => setIsConfirmOpen(false)} purpose="form" width="min(680px, calc(100vw - 36px))" maxHeight="min(90vh, 860px)" className="update-dialog-layer">
			  <section className="modal-panel update-confirm-dialog" aria-labelledby="update-confirm-title">
				{!hasStarted ? <>
                  <header className="update-dialog-hero">
                    <div className="update-dialog-icon" aria-hidden="true"><RefreshCw size={22} /></div>
                    <div>
                      <span className="eyebrow subtle">{t('updateQueue.readyEyebrow')}</span>
                      <h2 id="update-confirm-title">{t('updateQueue.confirmTitle')}</h2>
                      <p>{t('updateQueue.confirmBody', { count: updateConfirmation.eligible.length })}</p>
                    </div>
                  </header>

                  <div className="update-dialog-facts" aria-label={t('updateQueue.confirmSummary')}>
                    <div><PackageOpen size={17} aria-hidden="true" /><span>{t('updateQueue.appsLabel')}</span><strong>{updateConfirmation.eligible.length}</strong></div>
                    <div><ShieldCheck size={17} aria-hidden="true" /><span>{t('updateQueue.sourcesLabel')}</span><strong>{updateSourceCount}</strong></div>
                    <div className="is-schedule"><CalendarClock size={17} aria-hidden="true" /><span>{t('updateQueue.nextAutoLabel')}</span><strong>{nextAutoUpdateLabel}</strong></div>
                  </div>

                  {updateConfirmation.skipped.length > 0 && (
                    <p className="update-dialog-notice"><AlertCircle size={16} aria-hidden="true" /><span>{t('updateQueue.protectedSkipped', { count: updateConfirmation.skipped.length })}</span></p>
                  )}

                  {updateSourceChoices.length > 0 && <section className="update-mirror-groups">
				    <div><strong>{t('updateQueue.mirrorTitle')}</strong><small>{t('updateQueue.mirrorHelp')}</small></div>
				    {updateSourceChoices.map(({ source, downloadOptions, rawOptions }) => {
					  const current = mirrorOverrides[String(source.id)] || { downloadMirrorId: source.defaultDownloadMirrorId || '', rawMirrorId: source.defaultRawMirrorId || '' };
					  return <div className="update-mirror-source" key={source.id}>
					    <strong>{source.name}</strong>
					    {downloadOptions.length > 1 && <XSelector label={t('updateQueue.downloadMirror')} value={current.downloadMirrorId} options={downloadOptions} onChange={(value) => setMirrorOverrides((old) => ({ ...old, [String(source.id)]: { ...current, downloadMirrorId: value } }))} />}
					    {rawOptions.length > 1 && <XSelector label={t('updateQueue.rawMirror')} value={current.rawMirrorId} options={rawOptions} onChange={(value) => setMirrorOverrides((old) => ({ ...old, [String(source.id)]: { ...current, rawMirrorId: value } }))} />}
					  </div>;
				    })}
				  </section>}

                  <section className="update-app-section" aria-labelledby="update-preview-title">
                    <div className="update-app-section-head"><strong id="update-preview-title">{t('updateQueue.previewTitle')}</strong><small>{t('updateQueue.previewCount', { count: installedGroups.updates.length })}</small></div>
				    <div className="update-app-preview">
				      {installedGroups.updates.map(({ item, source }) => <div className={cx(source?.installProtected && 'is-skipped')} key={item.appid || source?.id}>
                        <div><strong>{item.title || source?.name || item.appid}</strong><small>{item.appid}</small></div>
                        <span>{item.version || '-'} <b aria-hidden="true">→</b> {source?.latestVersion?.version || '-'}</span>
                        <div className="update-app-badges">
                          {source?.installProtected && <small className="manual-update-only">{t('updateQueue.passwordBadge')}</small>}
                          {item.autoUpdateEnabled === false && <small className="manual-update-only">{t('updatePolicy.manualOnlyBadge')}</small>}
                        </div>
                      </div>)}
				    </div>
                  </section>

				  <footer className="dialog-actions update-dialog-actions">
				    <XButton type="button" variant="secondary" label={t('common.cancel')} onClick={() => setIsConfirmOpen(false)} />
				    <XButton type="button" variant="primary" label={t('updateQueue.start')} icon={<RefreshCw size={17} />} onClick={() => {
					  setHasStarted(true);
					  void onRunUpdates?.({ candidates: updateCandidates, mirrorOverrides: updateSources.map((source) => {
					    const value = mirrorOverrides[String(source.id)];
					    return { sourceId: Number(source.id), downloadMirrorId: value?.downloadMirrorId ?? source.defaultDownloadMirrorId ?? '', rawMirrorId: value?.rawMirrorId ?? source.defaultRawMirrorId ?? '' };
					  }) });
				    }} />
				  </footer>
				</> : <div className="update-progress-dialog" aria-live="polite">
                  <header className={cx('update-dialog-hero', `is-${queueStatus}`)}>
                    <div className="update-dialog-icon" aria-hidden="true">
                      {queueStatus === 'failed' ? <XCircle size={22} /> : queueStatus === 'success' || queueStatus === 'no_updates' ? <CheckCircle2 size={22} /> : <RefreshCw size={22} className={queueIsActive ? 'spin' : undefined} />}
                    </div>
                    <div>
                      <span className="eyebrow subtle">{t('updateQueue.progressEyebrow')}</span>
                      <h2 id="update-confirm-title">{t(`updateQueue.result.${queueStatus}`)}</h2>
                      <p>{updateItems.length
                        ? t(queueIsActive ? 'updateQueue.overallProgress' : 'updateQueue.completedProgress', { current: currentNumber, total: updateItems.length })
                        : updateQueueResult?.error || t('updateQueue.checkingSources')}</p>
                    </div>
                    <StatusBadge tone={queueTone} label={t(`updateQueue.result.${queueStatus}`)} />
                  </header>

                  <div className="update-overall-progress">
                    <XProgressBar label={t('installActivity.progressLabel')} isLabelHidden value={queueProgress} isIndeterminate={queueIsActive && updateItems.length === 0} variant="accent" />
                    {updateItems.length > 0 && <small>{t('updateQueue.queueCounts', { success: updateSummary.success || 0, failed: updateSummary.failed || 0, queued: (updateSummary.queued || 0) + (updateSummary.running || 0) })}</small>}
                  </div>

				  {currentItem && <div className="update-current-app">
					<strong>{currentItem.appName}</strong>
					<span>{t('updateQueue.stageStableInstall')}</span>
					<ol className="update-stage-timeline"><li className="complete">{t('installActivity.steps.queued')}</li><li className="current">{t('updateQueue.stageProcessing')}</li><li>{t('installActivity.steps.result')}</li></ol>
				  </div>}

                  {updateQueueResult?.error && <p className="update-dialog-error" role="alert"><AlertCircle size={16} aria-hidden="true" /><span>{updateQueueResult.error}</span></p>}

                  {updateItems.length > 0 && <div className="update-queue-items" aria-label={t('updateQueue.itemListLabel')}>
                    {updateItems.map((item) => <div className={cx('update-queue-item', `is-${item.status}`)} key={`${item.sourceId}:${item.packageId}`}>
                      <div><strong>{item.appName}</strong><small>{item.installedVersion || '-'} → {item.version || '-'}</small></div>
                      <StatusBadge tone={updateItemTone(item.status)} label={t(`updateQueue.itemStates.${item.status}`)} />
                      {item.detail && <p>{item.detail}</p>}
                    </div>)}
                  </div>}

				  <footer className="dialog-actions update-dialog-actions is-single"><XButton type="button" variant="secondary" label={queueIsActive ? t('updateQueue.keepRunning') : t('common.close')} onClick={() => setIsConfirmOpen(false)} /></footer>
				</div>}
			  </section>
			</ModalLayer>
		  )}
	</>
  );
}
