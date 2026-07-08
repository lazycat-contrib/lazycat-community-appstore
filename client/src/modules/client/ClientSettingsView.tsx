import { Clock3, Download, RefreshCw, Save, Settings, ShieldCheck, Sparkles } from 'lucide-react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { ClientSettings, ClientSourceStats, Toast } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';

const syncIntervalOptions = [5, 15, 30, 60, 360, 720, 1440];
const pageSizeOptions = [12, 24, 48, 96, 100];
const installDismissOptions = [0, 3, 5, 10, 30];

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
      defaultPageSize: settings.defaultPageSize || 24,
      autoSyncEnabled: Boolean(settings.autoSyncEnabled),
      autoSyncIntervalMinutes: settings.autoSyncIntervalMinutes || 60,
      syncOnStartup: Boolean(settings.syncOnStartup),
      installSuccessDismissSeconds: Number(settings.installSuccessDismissSeconds ?? 3),
      lastAutoSyncAt: settings.lastAutoSyncAt,
      lastAutoSyncStatus: settings.lastAutoSyncStatus,
      lastAutoSyncError: settings.lastAutoSyncError,
    });
  }, [
    settings.autoSyncEnabled,
    settings.autoSyncIntervalMinutes,
    settings.commentDisplayName,
    settings.defaultPageSize,
    settings.installSuccessDismissSeconds,
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
        defaultPageSize: Number(draft.defaultPageSize) || 24,
        autoSyncIntervalMinutes: Number(intervalValue) || 60,
        installSuccessDismissSeconds: Number(draft.installSuccessDismissSeconds ?? 3),
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
        <XButton
          type="button"
          variant="primary"
          label={saving ? t('clientSettings.saving') : t('common.save')}
          icon={saving ? <RefreshCw size={18} className="spin" /> : <Save size={18} />}
          isDisabled={saving}
          onClick={() => void saveSettings()}
        />
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
        <section className="panel settings-card-panel">
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
            onChange={(checked) => setDraft({ ...draft, autoSyncEnabled: checked })}
          />

          <XSelector
            label={t('clientSettings.interval')}
            description={t('clientSettings.intervalHelp')}
            value={intervalValue}
            options={syncIntervalOptions.map((value) => ({ value: String(value), label: t('clientSettings.intervalOption', { count: value }) }))}
            onChange={(value) => setDraft({ ...draft, autoSyncIntervalMinutes: Number(value) || 60 })}
          />

          <XSwitch
            label={t('clientSettings.syncOnStartup')}
            description={t('clientSettings.syncOnStartupHelp')}
            value={draft.syncOnStartup}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => setDraft({ ...draft, syncOnStartup: checked })}
          />
        </section>

        <section className="panel settings-card-panel">
          <div className="settings-card-head">
            <div>
              <ShieldCheck size={19} />
              <h2>{t('clientSettings.identityTitle')}</h2>
            </div>
            <StatusBadge tone="synced" label={t('clientSettings.localOnly')} />
          </div>
          <p className="muted-text">{t('clientSettings.identityBody')}</p>
          <XTextInput
            label={t('clientSettings.commentDisplayName')}
            description={t('clientSettings.commentDisplayNameHelp', { name: t('clientSettings.defaultCommentDisplayName') })}
            value={draft.commentDisplayName}
            placeholder={t('clientSettings.defaultCommentDisplayName')}
            onChange={(value) => setDraft({ ...draft, commentDisplayName: value })}
          />
          <XSelector
            label={t('clientSettings.defaultPageSize')}
            description={t('clientSettings.defaultPageSizeHelp')}
            value={String(draft.defaultPageSize || 24)}
            options={pageSizeOptions.map((value) => ({ value: String(value), label: t('clientSettings.pageSizeOption', { count: value }) }))}
            onChange={(value) => setDraft({ ...draft, defaultPageSize: Number(value) || 24 })}
          />
          <div className="settings-card-head compact">
            <div>
              <Download size={19} />
              <h2>{t('clientSettings.installTitle')}</h2>
            </div>
            <StatusBadge tone="synced" label={t('clientSettings.localOnly')} />
          </div>
          <XSelector
            label={t('clientSettings.installSuccessDismiss')}
            description={t('clientSettings.installSuccessDismissHelp')}
            value={String(draft.installSuccessDismissSeconds ?? 3)}
            options={installDismissOptions.map((value) => ({
              value: String(value),
              label: value === 0 ? t('clientSettings.installDismissNever') : t('clientSettings.installDismissSeconds', { count: value }),
            }))}
            onChange={(value) => setDraft({ ...draft, installSuccessDismissSeconds: Number(value) })}
          />
        </section>

        <div className="settings-form-actions">
          <XButton
            type="submit"
            variant="primary"
            label={saving ? t('clientSettings.saving') : t('clientSettings.saveSettings')}
            icon={saving ? <RefreshCw size={18} className="spin" /> : <Settings size={18} />}
            isDisabled={saving}
          />
        </div>
      </form>
    </section>
  );
}
