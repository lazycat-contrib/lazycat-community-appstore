import { AlertCircle, CalendarClock, Check, Clock3, Download, Gauge, History, Info, KeyRound, RefreshCw, Save, ShieldCheck, Sparkles } from 'lucide-react';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { APP_VERSION } from '../../config';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { ClientSettings, ClientSourceStats, Toast } from '../../shared/types';
import { cx, errorMessage, formatDate } from '../../shared/utils';
import { autoUpdateSchedulePresentation, normalizeAutomationSettings, normalizeEditableClientSettings, sameEditableClientSettings } from './clientUxState';

const syncIntervalOptions = [5, 15, 30, 60, 360, 720, 1440];
const pageSizeOptions = [12, 24, 48, 96, 100];
const installDismissOptions = [0, 3, 5, 10, 30];
const mirrorBenchmarkIntervalOptions = [30, 60, 360, 720, 1440];
type ClientSettingsTab = 'sync' | 'identity' | 'install' | 'about';
type SaveResult = 'idle' | 'saving' | 'saved' | 'error';
type SaveState = 'clean' | 'dirty' | Exclude<SaveResult, 'idle'>;
type PendingSave = { settings: ClientSettings; revision: number };

export function ClientSettingsView({
  settings,
  sourceStats,
  onSave,
  onRunUpdates,
  onRunMirrorBenchmark,
  isUpdateQueueRunning = false,
  setToast,
}: {
  settings: ClientSettings;
  sourceStats: ClientSourceStats;
  onSave: (settings: ClientSettings) => Promise<void>;
  onRunUpdates?: () => Promise<void>;
  onRunMirrorBenchmark?: () => Promise<void>;
  isUpdateQueueRunning?: boolean;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [baseline, setBaseline] = useState<ClientSettings>(settings);
  const [draft, setDraft] = useState<ClientSettings>(settings);
  const [activeTab, setActiveTab] = useState<ClientSettingsTab>('sync');
  const [saveResult, setSaveResult] = useState<SaveResult>('idle');
  const [saveError, setSaveError] = useState('');
  const [isBenchmarkRunning, setIsBenchmarkRunning] = useState(false);
  const editRevisionRef = useRef(0);
  const saveInFlightRef = useRef(false);
  const pendingSaveRef = useRef<PendingSave | null>(null);
  const isDirty = !sameEditableClientSettings(draft, baseline);
  const effectiveSaveState: SaveState = saveResult === 'saving' || saveResult === 'error' || saveResult === 'saved'
    ? saveResult
    : isDirty
      ? 'dirty'
      : 'clean';

  function updateDraft(next: ClientSettings) {
    editRevisionRef.current += 1;
    setDraft(next);
    setSaveError('');
    setSaveResult((current) => current === 'saving' ? 'saving' : 'idle');
  }

  useEffect(() => {
    const next = { ...settings, ...normalizeEditableClientSettings(settings) };
    const pending = pendingSaveRef.current;
    if (pending) {
      const hasNewerEdits = editRevisionRef.current !== pending.revision;
      setBaseline(next);
      if (!hasNewerEdits) setDraft(next);
      setSaveError('');
      if (!saveInFlightRef.current) setSaveResult(hasNewerEdits ? 'idle' : 'saved');
      pendingSaveRef.current = null;
      return;
    }
    if (saveInFlightRef.current) return;
    setBaseline(next);
    setDraft(next);
    setSaveError('');
    setSaveResult('idle');
  }, [
    settings.autoSyncEnabled,
    settings.autoSyncIntervalMinutes,
	settings.autoUpdateEnabled,
	settings.autoUpdateIntervalMinutes,
	settings.autoUpdateNotifyEnabled,
    settings.clientTitle,
    settings.commentDisplayName,
    settings.defaultPageSize,
    settings.installSuccessDismissSeconds,
	settings.lastAutoUpdateAt,
	settings.lastAutoUpdateError,
	settings.lastAutoUpdateStatus,
    settings.lastMirrorBenchmarkAt,
    settings.lastMirrorBenchmarkStatus,
    settings.mirrorBenchmarkEnabled,
    settings.mirrorBenchmarkIntervalMinutes,
    settings.syncOnStartup,
  ]);

  const syncState = useMemo(() => {
    if (!draft.autoSyncEnabled) return 'off';
    if (settings.lastAutoSyncStatus === 'failed') return 'failed';
    if (settings.lastAutoSyncStatus === 'partial') return 'partial';
    if (settings.lastAutoSyncAt) return 'ready';
    return 'waiting';
  }, [draft.autoSyncEnabled, settings.lastAutoSyncAt, settings.lastAutoSyncStatus]);

  const intervalValue = String(draft.autoSyncIntervalMinutes || 60);
  const updateIntervalValue = String(draft.autoUpdateIntervalMinutes || 60);
  const syncStatusClass = syncState === 'failed' ? 'failed' : syncState === 'partial' ? 'stale' : syncState === 'off' ? 'unsynced' : 'synced';
  const autoUpdateState = !draft.autoUpdateEnabled
    ? 'off'
    : settings.lastAutoUpdateStatus === 'failed'
      ? 'failed'
      : settings.lastAutoUpdateStatus === 'partial'
        ? 'partial'
        : settings.lastAutoUpdateAt
          ? 'ready'
          : 'waiting';
  const autoUpdateStatusClass = autoUpdateState === 'failed' ? 'failed' : autoUpdateState === 'partial' ? 'stale' : autoUpdateState === 'off' ? 'unsynced' : 'synced';
  const autoUpdateSchedule = autoUpdateSchedulePresentation({
    enabled: settings.autoUpdateEnabled,
    intervalMinutes: settings.autoUpdateIntervalMinutes,
    lastRunAt: settings.lastAutoUpdateAt,
    scheduleState: settings.autoUpdateScheduleState,
    nextRunAt: settings.nextAutoUpdateAt,
  });
  const nextAutoUpdateLabel = autoUpdateSchedule.state === 'scheduled'
    ? formatDate(autoUpdateSchedule.nextRunAt)
    : t(`clientSettings.autoUpdateSchedule.${autoUpdateSchedule.state}`);
  const mirrorBenchmarkState = !draft.mirrorBenchmarkEnabled
    ? 'off'
    : settings.lastMirrorBenchmarkStatus === 'failed'
      ? 'failed'
      : settings.lastMirrorBenchmarkStatus === 'partial'
        ? 'partial'
        : settings.lastMirrorBenchmarkAt
          ? 'ready'
          : 'waiting';
  const mirrorBenchmarkSchedule = autoUpdateSchedulePresentation({
    enabled: settings.mirrorBenchmarkEnabled,
    intervalMinutes: settings.mirrorBenchmarkIntervalMinutes,
    lastRunAt: settings.lastMirrorBenchmarkAt,
    scheduleState: settings.mirrorBenchmarkScheduleState,
    nextRunAt: settings.nextMirrorBenchmarkAt,
  });
  const nextMirrorBenchmarkLabel = mirrorBenchmarkSchedule.state === 'scheduled'
    ? formatDate(mirrorBenchmarkSchedule.nextRunAt)
    : t(`clientSettings.autoUpdateSchedule.${mirrorBenchmarkSchedule.state}`);
  const effectiveClientTitle = draft.clientTitle.trim() || t('appName');
  const settingsTabs = [
    { key: 'sync', label: t('clientSettings.tabs.sync'), icon: Clock3 },
    { key: 'identity', label: t('clientSettings.tabs.identity'), icon: ShieldCheck },
    { key: 'install', label: t('clientSettings.tabs.install'), icon: Download },
    { key: 'about', label: t('clientSettings.tabs.about'), icon: Info },
  ] satisfies Array<{ key: ClientSettingsTab; label: string; icon: typeof Clock3 }>;

  async function saveSettings(event?: Pick<FormEvent, 'preventDefault'>) {
    event?.preventDefault();
    if (!isDirty || saveInFlightRef.current) return;
    const payload = normalizeEditableClientSettings(draft);
    const submitted = { ...draft, ...payload };
    const submission: PendingSave = { settings: submitted, revision: editRevisionRef.current };
    saveInFlightRef.current = true;
    pendingSaveRef.current = submission;
    setSaveResult('saving');
    setSaveError('');
    try {
      await onSave(payload);
      const hasNewerEdits = editRevisionRef.current !== submission.revision;
      if (pendingSaveRef.current === submission) {
        setBaseline(submitted);
        if (!hasNewerEdits) setDraft(submitted);
      }
      setSaveResult(hasNewerEdits ? 'idle' : 'saved');
      setToast({ tone: 'success', message: t('clientSettings.saved') });
    } catch (error) {
      if (pendingSaveRef.current === submission) pendingSaveRef.current = null;
      setSaveError(errorMessage(error, t('clientSettings.saveFailed')));
      setSaveResult('error');
    } finally {
      saveInFlightRef.current = false;
    }
  }

  async function runMirrorBenchmarkNow() {
    if (!onRunMirrorBenchmark || isBenchmarkRunning) return;
    setIsBenchmarkRunning(true);
    try {
      await onRunMirrorBenchmark();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('clientSettings.mirrorBenchmarkFailed')) });
    } finally {
      setIsBenchmarkRunning(false);
    }
  }

  return (
    <section className="page-grid client-settings-page">
      <div className="page-heading settings-hero">
        <div>
          <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
          <h1>{t('clientSettings.title')}</h1>
          <p>{t('clientSettings.subtitle')}</p>
        </div>
      </div>

      <div className="settings-overview-grid" aria-label={t('clientSettings.overview')}>
        <XCard className="settings-signal-card" padding={4}>
          <span>
            <Clock3 size={17} />
            {t('clientSettings.autoSync')}
          </span>
          <strong>{draft.autoSyncEnabled ? t('common.on') : t('common.off')}</strong>
          <small>{draft.autoSyncEnabled ? t('clientSettings.everyMinutes', { count: draft.autoSyncIntervalMinutes || 60 }) : t('clientSettings.autoSyncOffHint')}</small>
        </XCard>
        <XCard className="settings-signal-card" padding={4}>
          <span>
            <RefreshCw size={17} />
            {t('clientSettings.lastRun')}
          </span>
          <strong>{settings.lastAutoSyncAt ? formatDate(settings.lastAutoSyncAt) : t('clientSettings.neverRun')}</strong>
          <small>{settings.lastAutoSyncError || t(`clientSettings.syncStates.${syncState}`)}</small>
        </XCard>
        <XCard className="settings-signal-card" padding={4}>
          <span>
            <Sparkles size={17} />
            {t('clientSettings.cachedApps')}
          </span>
          <strong>{sourceStats.installableSourceAppCount}</strong>
          <small>{t('clientSettings.sourceSummary', { sources: sourceStats.sourceCount, synced: sourceStats.syncedSourceCount })}</small>
        </XCard>
      </div>

      <form className="client-settings-layout" onSubmit={saveSettings}>
        <div className="horizontal-control-scroll client-settings-tabs">
          <XTabList value={activeTab} onChange={(value) => setActiveTab(value as ClientSettingsTab)} hasDivider size="md">
            {settingsTabs.map((item) => {
              const Icon = item.icon;
              return <XTab key={item.key} value={item.key} label={item.label} icon={<Icon size={16} />} />;
            })}
          </XTabList>
        </div>

        {activeTab === 'sync' && (
        <section className="panel settings-card-panel settings-tab-panel client-settings-panel">
          <div className="settings-card-head">
            <div>
              <Clock3 size={19} />
              <h2>{t('clientSettings.syncTitle')}</h2>
            </div>
            <StatusBadge tone={syncStatusClass} label={t(`clientSettings.syncStates.${syncState}`)} />
          </div>
          <p className="muted-text">{t('clientSettings.syncBody')}</p>

          <XSwitch
            label={t('clientSettings.autoSync')}
            labelTooltip={draft.autoUpdateEnabled ? t('clientSettings.autoSyncRequiredByUpdates') : t('clientSettings.autoSyncHelp')}
            value={draft.autoSyncEnabled}
            isDisabled={draft.autoUpdateEnabled}
            disabledMessage={draft.autoUpdateEnabled ? t('clientSettings.autoSyncRequiredByUpdates') : undefined}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => updateDraft({ ...draft, autoSyncEnabled: checked })}
          />

          <XSelector
            label={t('clientSettings.interval')}
            description={t('clientSettings.intervalHelp')}
            labelTooltip={draft.autoUpdateEnabled ? t('clientSettings.syncIntervalBoundByUpdates') : undefined}
            value={intervalValue}
            options={syncIntervalOptions.filter((value) => !draft.autoUpdateEnabled || value <= (draft.autoUpdateIntervalMinutes || 60)).map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
            onChange={(value) => updateDraft({ ...draft, autoSyncIntervalMinutes: Number(value) || 60 })}
          />

          <XSwitch
            label={t('clientSettings.syncOnStartup')}
            description={t('clientSettings.syncOnStartupHelp')}
            value={draft.syncOnStartup}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => updateDraft({ ...draft, syncOnStartup: checked })}
          />

          <section className={cx('client-auto-update-control', `is-${autoUpdateState}`)} aria-labelledby="client-auto-update-title">
            <header className="client-auto-update-head">
              <div className="client-auto-update-icon" aria-hidden="true">
                <RefreshCw size={20} />
              </div>
              <div>
                <span className="eyebrow subtle">{t('clientSettings.automation')}</span>
                <h3 id="client-auto-update-title">{t('clientSettings.autoUpdate')}</h3>
                <p>{t('clientSettings.autoUpdateHelp')}</p>
              </div>
              <StatusBadge tone={autoUpdateStatusClass} label={t(`clientSettings.syncStates.${autoUpdateState}`)} />
            </header>

            <div className="client-auto-update-schedule" aria-label={t('clientSettings.autoUpdateScheduleLabel')}>
              <div>
                <CalendarClock size={18} aria-hidden="true" />
                <span>{t('clientSettings.autoUpdateNext')}</span>
                <strong>
                  {autoUpdateSchedule.state === 'scheduled'
                    ? <time dateTime={autoUpdateSchedule.nextRunAt}>{nextAutoUpdateLabel}</time>
                    : nextAutoUpdateLabel}
                </strong>
              </div>
              <div>
                <History size={18} aria-hidden="true" />
                <span>{t('clientSettings.autoUpdatePrevious')}</span>
                <strong>
                  {settings.lastAutoUpdateAt
                    ? <time dateTime={settings.lastAutoUpdateAt}>{formatDate(settings.lastAutoUpdateAt)}</time>
                    : t('clientSettings.neverRun')}
                </strong>
              </div>
            </div>

            <div className="client-auto-update-fields">
              <XSwitch
                label={t('clientSettings.autoUpdate')}
                description={draft.autoUpdateEnabled ? t('clientSettings.autoUpdateOnHint') : t('clientSettings.autoUpdateOffHint')}
                value={draft.autoUpdateEnabled}
                labelSpacing="spread"
                width="100%"
                onChange={(checked) => updateDraft(normalizeAutomationSettings({ ...draft, autoUpdateEnabled: checked }))}
              />
              <XSelector
                label={t('clientSettings.autoUpdateInterval')}
                description={t('clientSettings.autoUpdateIntervalHelp')}
                value={updateIntervalValue}
                options={syncIntervalOptions.map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
                onChange={(value) => updateDraft(normalizeAutomationSettings({ ...draft, autoUpdateIntervalMinutes: Number(value) || 60 }))}
              />
            </div>

            <p className="client-auto-update-note">
              <KeyRound size={16} aria-hidden="true" />
              <span>{t('clientSettings.autoUpdatePasswordHint')}</span>
            </p>
            {settings.lastAutoUpdateError && (
              <p className="client-auto-update-error" role="alert">
                <AlertCircle size={16} aria-hidden="true" />
                <span>{settings.lastAutoUpdateError}</span>
              </p>
            )}
            {onRunUpdates && (
              <div className="client-auto-update-actions">
                <XButton
                  type="button"
                  variant="secondary"
                  label={isUpdateQueueRunning ? t('updateQueue.running') : t('clientSettings.runUpdateNow')}
                  icon={<RefreshCw size={17} className={isUpdateQueueRunning ? 'spin' : undefined} />}
                  isDisabled={isUpdateQueueRunning}
                  onClick={() => void onRunUpdates()}
                />
              </div>
            )}
          </section>
        </section>
        )}

        {activeTab === 'identity' && (
        <section className="panel settings-card-panel settings-tab-panel client-settings-panel">
          <div className="settings-card-head">
            <div>
              <ShieldCheck size={19} />
              <h2>{t('clientSettings.identityTitle')}</h2>
            </div>
            <StatusBadge tone="synced" label={t('clientSettings.localOnly')} />
          </div>
          <p className="muted-text">{t('clientSettings.identityBody')}</p>
          <XTextInput
            label={t('clientSettings.clientTitle')}
            description={t('clientSettings.clientTitleHelp', { name: t('appName') })}
            value={draft.clientTitle}
            placeholder={t('appName')}
            onChange={(value) => updateDraft({ ...draft, clientTitle: value })}
          />
          <XTextInput
            label={t('clientSettings.commentDisplayName')}
            description={t('clientSettings.commentDisplayNameHelp', { name: t('clientSettings.defaultCommentDisplayName') })}
            value={draft.commentDisplayName}
            placeholder={t('clientSettings.defaultCommentDisplayName')}
            onChange={(value) => updateDraft({ ...draft, commentDisplayName: value })}
          />
          <XSelector
            label={t('clientSettings.defaultPageSize')}
            description={t('clientSettings.defaultPageSizeHelp')}
            value={String(draft.defaultPageSize || 24)}
            options={pageSizeOptions.map((value) => ({ value: String(value), label: t('clientSettings.pageSizeOption', { count: value }) }))}
            onChange={(value) => updateDraft({ ...draft, defaultPageSize: Number(value) || 24 })}
          />
        </section>
        )}

        {activeTab === 'install' && (
        <section className="panel settings-card-panel settings-tab-panel client-settings-panel">
          <div className="settings-card-head">
            <div>
              <Download size={19} />
              <h2>{t('clientSettings.installTitle')}</h2>
            </div>
            <StatusBadge tone="synced" label={t('clientSettings.localOnly')} />
          </div>
          <p className="muted-text">{t('clientSettings.installBody')}</p>
          <XSwitch
            label={t('clientSettings.autoUpdateNotify')}
            description={t('clientSettings.autoUpdateNotifyHelp')}
            value={draft.autoUpdateNotifyEnabled}
            onChange={(checked) => updateDraft({ ...draft, autoUpdateNotifyEnabled: checked })}
          />
          <section className={cx('client-auto-update-control client-mirror-benchmark-control', `is-${mirrorBenchmarkState}`)} aria-labelledby="client-mirror-benchmark-title">
            <header className="client-auto-update-head">
              <div className="client-auto-update-icon" aria-hidden="true">
                <Gauge size={20} />
              </div>
              <div>
                <span className="eyebrow subtle">{t('clientSettings.localEvaluation')}</span>
                <h3 id="client-mirror-benchmark-title">{t('clientSettings.mirrorBenchmark')}</h3>
                <p>{t('clientSettings.mirrorBenchmarkHelp')}</p>
              </div>
              <StatusBadge
                tone={mirrorBenchmarkState === 'failed' ? 'failed' : mirrorBenchmarkState === 'partial' ? 'stale' : mirrorBenchmarkState === 'off' ? 'unsynced' : 'synced'}
                label={t(`clientSettings.mirrorBenchmarkStates.${mirrorBenchmarkState}`)}
              />
            </header>
            <div className="client-auto-update-schedule" aria-label={t('clientSettings.mirrorBenchmarkScheduleLabel')}>
              <div>
                <CalendarClock size={18} aria-hidden="true" />
                <span>{t('clientSettings.mirrorBenchmarkNext')}</span>
                <strong>{mirrorBenchmarkSchedule.state === 'scheduled'
                  ? <time dateTime={mirrorBenchmarkSchedule.nextRunAt}>{nextMirrorBenchmarkLabel}</time>
                  : nextMirrorBenchmarkLabel}</strong>
              </div>
              <div>
                <History size={18} aria-hidden="true" />
                <span>{t('clientSettings.mirrorBenchmarkPrevious')}</span>
                <strong>{settings.lastMirrorBenchmarkAt
                  ? <time dateTime={settings.lastMirrorBenchmarkAt}>{formatDate(settings.lastMirrorBenchmarkAt)}</time>
                  : t('clientSettings.neverRun')}</strong>
              </div>
            </div>
            <div className="client-auto-update-fields">
              <XSwitch
                label={t('clientSettings.mirrorBenchmarkEnabled')}
                description={draft.mirrorBenchmarkEnabled ? t('clientSettings.mirrorBenchmarkOnHint') : t('clientSettings.mirrorBenchmarkOffHint')}
                value={draft.mirrorBenchmarkEnabled}
                labelSpacing="spread"
                width="100%"
                onChange={(checked) => updateDraft({ ...draft, mirrorBenchmarkEnabled: checked })}
              />
              <XSelector
                label={t('clientSettings.mirrorBenchmarkInterval')}
                description={t('clientSettings.mirrorBenchmarkIntervalHelp')}
                value={String(draft.mirrorBenchmarkIntervalMinutes || 360)}
                options={mirrorBenchmarkIntervalOptions.map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
                onChange={(value) => updateDraft({ ...draft, mirrorBenchmarkIntervalMinutes: Number(value) || 360 })}
              />
            </div>
            <p className="client-auto-update-note">
              <ShieldCheck size={16} aria-hidden="true" />
              <span>{t('clientSettings.mirrorBenchmarkTrafficHint')}</span>
            </p>
            {onRunMirrorBenchmark && (
              <div className="client-auto-update-actions">
                <XButton
                  type="button"
                  variant="secondary"
                  label={isBenchmarkRunning ? t('clientSettings.mirrorBenchmarkRunning') : t('clientSettings.runMirrorBenchmarkNow')}
                  icon={<Gauge size={17} className={isBenchmarkRunning ? 'spin' : undefined} />}
                  isDisabled={isBenchmarkRunning}
                  onClick={() => void runMirrorBenchmarkNow()}
                />
              </div>
            )}
          </section>
          <XSelector
            label={t('clientSettings.installSuccessDismiss')}
            description={t('clientSettings.installSuccessDismissHelp')}
            value={String(draft.installSuccessDismissSeconds ?? 3)}
            options={installDismissOptions.map((value) => ({
              value: String(value),
              label: value === 0 ? t('clientSettings.installDismissNever') : t('clientSettings.installDismissSeconds', { count: value }),
            }))}
            onChange={(value) => updateDraft({ ...draft, installSuccessDismissSeconds: Number(value) })}
          />
        </section>
        )}

        {activeTab === 'about' && (
        <section className="panel settings-card-panel settings-tab-panel client-settings-panel client-about-panel">
          <div className="settings-card-head">
            <div>
              <Info size={19} />
              <h2>{t('clientSettings.aboutTitle')}</h2>
            </div>
            <StatusBadge tone="info" label={t('clientSettings.aboutBadge')} />
          </div>
          <p className="muted-text">{t('clientSettings.aboutBody')}</p>
          <div className="client-about-list" aria-label={t('clientSettings.aboutTitle')}>
            <div>
              <span>{t('clientSettings.clientVersion')}</span>
              <strong>{APP_VERSION}</strong>
            </div>
            <div>
              <span>{t('clientSettings.runtimeMode')}</span>
              <strong>{t('mode.standaloneClient')}</strong>
            </div>
            <div>
              <span>{t('clientSettings.effectiveTitle')}</span>
              <strong>{effectiveClientTitle}</strong>
            </div>
          </div>
        </section>
        )}

        <div className={cx('client-settings-save-bar', effectiveSaveState)} role="status" aria-live="polite">
          <div>
            {effectiveSaveState === 'error' ? <AlertCircle size={18} /> : effectiveSaveState === 'saved' ? <Check size={18} /> : <Save size={18} />}
            <span>{t(`clientSettings.saveStates.${effectiveSaveState}`)}</span>
          </div>
          {saveError && <p role="alert">{saveError}</p>}
          <XButton
            type="submit"
            variant="primary"
            label={effectiveSaveState === 'saving' ? t('clientSettings.saving') : t('clientSettings.saveSettings')}
            icon={effectiveSaveState === 'saving' ? <RefreshCw size={18} className="spin" /> : <Save size={18} />}
            isDisabled={!isDirty || saveResult === 'saving'}
          />
        </div>
      </form>
    </section>
  );
}
