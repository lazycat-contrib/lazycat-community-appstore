import { AlertCircle, Check, Clock3, Download, Info, RefreshCw, Save, ShieldCheck, Sparkles } from 'lucide-react';
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
import { normalizeEditableClientSettings, sameEditableClientSettings } from './clientUxState';

const syncIntervalOptions = [5, 15, 30, 60, 360, 720, 1440];
const pageSizeOptions = [12, 24, 48, 96, 100];
const installDismissOptions = [0, 3, 5, 10, 30];
type ClientSettingsTab = 'sync' | 'identity' | 'install' | 'about';
type SaveResult = 'idle' | 'saving' | 'saved' | 'error';
type SaveState = 'clean' | 'dirty' | Exclude<SaveResult, 'idle'>;
type PendingSave = { settings: ClientSettings; revision: number };

export function ClientSettingsView({
  settings,
  sourceStats,
  onSave,
  setToast,
}: {
  settings: ClientSettings;
  sourceStats: ClientSourceStats;
  onSave: (settings: ClientSettings) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [baseline, setBaseline] = useState<ClientSettings>(settings);
  const [draft, setDraft] = useState<ClientSettings>(settings);
  const [activeTab, setActiveTab] = useState<ClientSettingsTab>('sync');
  const [saveResult, setSaveResult] = useState<SaveResult>('idle');
  const [saveError, setSaveError] = useState('');
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
    settings.clientTitle,
    settings.commentDisplayName,
    settings.defaultPageSize,
    settings.installSuccessDismissSeconds,
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
  const syncStatusClass = syncState === 'failed' ? 'failed' : syncState === 'partial' ? 'stale' : syncState === 'off' ? 'unsynced' : 'synced';
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
            description={t('clientSettings.autoSyncHelp')}
            value={draft.autoSyncEnabled}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => updateDraft({ ...draft, autoSyncEnabled: checked })}
          />

          <XSelector
            label={t('clientSettings.interval')}
            description={t('clientSettings.intervalHelp')}
            value={intervalValue}
            options={syncIntervalOptions.map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
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
