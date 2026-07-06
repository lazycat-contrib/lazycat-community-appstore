import { Clock3, RefreshCw, Save, Settings, ShieldCheck, Sparkles } from 'lucide-react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import type { ClientSettings, ClientSourceStats, Toast } from '../../shared/types';
import { cx, errorMessage, formatDate } from '../../shared/utils';

const syncIntervalOptions = [5, 15, 30, 60, 360, 720, 1440];

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
  const [draft, setDraft] = useState<ClientSettings>(settings);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setDraft({
      commentDisplayName: settings.commentDisplayName || '',
      autoSyncEnabled: Boolean(settings.autoSyncEnabled),
      autoSyncIntervalMinutes: settings.autoSyncIntervalMinutes || 60,
      syncOnStartup: Boolean(settings.syncOnStartup),
      lastAutoSyncAt: settings.lastAutoSyncAt,
      lastAutoSyncStatus: settings.lastAutoSyncStatus,
      lastAutoSyncError: settings.lastAutoSyncError,
    });
  }, [
    settings.autoSyncEnabled,
    settings.autoSyncIntervalMinutes,
    settings.commentDisplayName,
    settings.lastAutoSyncAt,
    settings.lastAutoSyncError,
    settings.lastAutoSyncStatus,
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

  async function saveSettings(event?: Pick<FormEvent, 'preventDefault'>) {
    event?.preventDefault();
    setSaving(true);
    try {
      await onSave({
        ...draft,
        commentDisplayName: draft.commentDisplayName.trim(),
        autoSyncIntervalMinutes: Number(intervalValue) || 60,
      });
      setToast({ tone: 'success', message: t('clientSettings.saved') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('clientSettings.saveFailed')) });
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="page-grid client-settings-page">
      <div className="page-heading with-action settings-hero">
        <div>
          <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
          <h1>{t('clientSettings.title')}</h1>
          <p>{t('clientSettings.subtitle')}</p>
        </div>
        <button type="button" className="primary-button" disabled={saving} onClick={() => void saveSettings()}>
          {saving ? <RefreshCw size={18} className="spin" /> : <Save size={18} />}
          <span>{saving ? t('clientSettings.saving') : t('common.save')}</span>
        </button>
      </div>

      <div className="settings-overview-grid" aria-label={t('clientSettings.overview')}>
        <div className="settings-signal-card">
          <span>
            <Clock3 size={17} />
            {t('clientSettings.autoSync')}
          </span>
          <strong>{draft.autoSyncEnabled ? t('common.on') : t('common.off')}</strong>
          <small>{draft.autoSyncEnabled ? t('clientSettings.everyMinutes', { count: draft.autoSyncIntervalMinutes || 60 }) : t('clientSettings.autoSyncOffHint')}</small>
        </div>
        <div className="settings-signal-card">
          <span>
            <RefreshCw size={17} />
            {t('clientSettings.lastRun')}
          </span>
          <strong>{settings.lastAutoSyncAt ? formatDate(settings.lastAutoSyncAt) : t('clientSettings.neverRun')}</strong>
          <small>{settings.lastAutoSyncError || t(`clientSettings.syncStates.${syncState}`)}</small>
        </div>
        <div className="settings-signal-card">
          <span>
            <Sparkles size={17} />
            {t('clientSettings.cachedApps')}
          </span>
          <strong>{sourceStats.installableSourceAppCount}</strong>
          <small>{t('clientSettings.sourceSummary', { sources: sourceStats.sourceCount, synced: sourceStats.syncedSourceCount })}</small>
        </div>
      </div>

      <form className="client-settings-layout" onSubmit={saveSettings}>
        <section className="panel settings-card-panel">
          <div className="settings-card-head">
            <div>
              <Clock3 size={19} />
              <h2>{t('clientSettings.syncTitle')}</h2>
            </div>
            <span className={cx('status-badge', syncStatusClass)}>{t(`clientSettings.syncStates.${syncState}`)}</span>
          </div>
          <p className="muted-text">{t('clientSettings.syncBody')}</p>

          <div className="settings-toggle-row">
            <div>
              <strong>{t('clientSettings.autoSync')}</strong>
              <span>{t('clientSettings.autoSyncHelp')}</span>
            </div>
            <div className="segmented compact-segmented" aria-label={t('clientSettings.autoSync')}>
              <button type="button" className={cx(draft.autoSyncEnabled && 'active')} onClick={() => setDraft({ ...draft, autoSyncEnabled: true })}>
                {t('common.on')}
              </button>
              <button type="button" className={cx(!draft.autoSyncEnabled && 'active')} onClick={() => setDraft({ ...draft, autoSyncEnabled: false })}>
                {t('common.off')}
              </button>
            </div>
          </div>

          <XSelector
            label={t('clientSettings.interval')}
            description={t('clientSettings.intervalHelp')}
            value={intervalValue}
            options={syncIntervalOptions.map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
            onChange={(value) => setDraft({ ...draft, autoSyncIntervalMinutes: Number(value) || 60 })}
          />

          <div className="settings-toggle-row">
            <div>
              <strong>{t('clientSettings.syncOnStartup')}</strong>
              <span>{t('clientSettings.syncOnStartupHelp')}</span>
            </div>
            <div className="segmented compact-segmented" aria-label={t('clientSettings.syncOnStartup')}>
              <button type="button" className={cx(draft.syncOnStartup && 'active')} onClick={() => setDraft({ ...draft, syncOnStartup: true })}>
                {t('common.on')}
              </button>
              <button type="button" className={cx(!draft.syncOnStartup && 'active')} onClick={() => setDraft({ ...draft, syncOnStartup: false })}>
                {t('common.off')}
              </button>
            </div>
          </div>
        </section>

        <section className="panel settings-card-panel">
          <div className="settings-card-head">
            <div>
              <ShieldCheck size={19} />
              <h2>{t('clientSettings.identityTitle')}</h2>
            </div>
            <span className="status-badge synced">{t('clientSettings.localOnly')}</span>
          </div>
          <p className="muted-text">{t('clientSettings.identityBody')}</p>
          <XTextInput
            label={t('clientSettings.commentDisplayName')}
            description={t('clientSettings.commentDisplayNameHelp', { name: t('clientSettings.defaultCommentDisplayName') })}
            value={draft.commentDisplayName}
            placeholder={t('clientSettings.defaultCommentDisplayName')}
            onChange={(value) => setDraft({ ...draft, commentDisplayName: value })}
          />
        </section>

        <div className="settings-form-actions">
          <button type="submit" className="primary-button" disabled={saving}>
            {saving ? <RefreshCw size={18} className="spin" /> : <Settings size={18} />}
            <span>{saving ? t('clientSettings.saving') : t('clientSettings.saveSettings')}</span>
          </button>
        </div>
      </form>
    </section>
  );
}
