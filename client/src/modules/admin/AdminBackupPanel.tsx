import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, CloudUpload, Database, HardDrive, Play, RefreshCw, Save, Server, XCircle } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { EmptyState as XEmptyState } from '@astryxdesign/core/EmptyState';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TimeInput as XTimeInput, type ISOTimeString } from '@astryxdesign/core/TimeInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import type { BackupRunResult, BackupSettings, Toast } from '../../shared/types';
import { errorMessage, formatBytes, formatDate } from '../../shared/utils';
import type { StorageSettings } from './StorageSettingsPanel';

type BackupDraft = {
  enabled: boolean;
  scheduleTime: string;
  storageKeys: string[];
};

const defaultBackupSettings: BackupSettings = {
  enabled: false,
  scheduleTime: '03:00',
  storageKeys: [],
  isRunning: false,
};

function draftFromSettings(settings: BackupSettings): BackupDraft {
  return {
    enabled: Boolean(settings.enabled),
    scheduleTime: settings.scheduleTime || '03:00',
    storageKeys: settings.storageKeys || [],
  };
}

function storageIcon(provider: string) {
  switch (provider) {
    case 'S3':
    case 'CLOUDFLARE_R2':
      return <Database size={17} />;
    case 'WEBDAV':
      return <CloudUpload size={17} />;
    case 'LOCAL':
      return <HardDrive size={17} />;
    default:
      return <Server size={17} />;
  }
}

function resultVariant(status?: string): 'neutral' | 'success' | 'warning' | 'error' | 'info' {
  if (status === 'success') return 'success';
  if (status === 'partial') return 'warning';
  if (status === 'failed') return 'error';
  return 'neutral';
}

function resultIcon(status?: string) {
  if (status === 'success') return <CheckCircle2 size={16} />;
  if (status === 'failed') return <XCircle size={16} />;
  return <AlertTriangle size={16} />;
}

export function AdminBackupPanel({
  storages,
  setToast,
}: {
  storages: StorageSettings[];
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [settings, setSettings] = useState<BackupSettings>(defaultBackupSettings);
  const [draft, setDraft] = useState<BackupDraft>(draftFromSettings(defaultBackupSettings));
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isRunning, setIsRunning] = useState(false);

  const selectedCount = draft.storageKeys.length;
  const selectedStorages = useMemo(() => new Set(draft.storageKeys), [draft.storageKeys]);
  const lastRun = settings.lastRun;

  useEffect(() => {
    void loadSettings();
  }, []);

  async function loadSettings() {
    setIsLoading(true);
    try {
      const data = await api<{ settings: BackupSettings }>('/api/v1/admin/backups/settings');
      const next = data.settings || defaultBackupSettings;
      setSettings(next);
      setDraft(draftFromSettings(next));
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.backup.loadFailed')) });
    } finally {
      setIsLoading(false);
    }
  }

  function toggleStorage(key: string, checked: boolean) {
    setDraft((current) => {
      const keys = new Set(current.storageKeys);
      if (checked) {
        keys.add(key);
      } else {
        keys.delete(key);
      }
      return { ...current, storageKeys: Array.from(keys) };
    });
  }

  async function saveSettings() {
    setIsSaving(true);
    try {
      const data = await api<{ settings: BackupSettings }>('/api/v1/admin/backups/settings', {
        method: 'PATCH',
        body: JSON.stringify(draft),
      });
      const next = data.settings || defaultBackupSettings;
      setSettings(next);
      setDraft(draftFromSettings(next));
      setToast({ tone: 'success', message: t('admin.backup.saved') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.backup.saveFailed')) });
    } finally {
      setIsSaving(false);
    }
  }

  async function runBackup() {
    setIsRunning(true);
    try {
      const data = await api<{ result: BackupRunResult; settings: BackupSettings }>('/api/v1/admin/backups/run', { method: 'POST' });
      const next = data.settings || { ...settings, lastRun: data.result };
      setSettings(next);
      setDraft(draftFromSettings(next));
      if (data.result.status === 'success') {
        setToast({ tone: 'success', message: t('admin.backup.runSucceeded') });
      } else if (data.result.status === 'partial') {
        setToast({ tone: 'neutral', message: t('admin.backup.runPartial') });
      } else {
        setToast({ tone: 'error', message: t('admin.backup.runFailed') });
      }
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.backup.runFailed')) });
    } finally {
      setIsRunning(false);
    }
  }

  return (
    <div className="settings-section backup-manager">
      <div className="settings-section-head with-action">
        <div>
          <strong>{t('admin.backup.title')}</strong>
          <span>{t('admin.backup.body')}</span>
        </div>
        <div className="backup-head-actions">
          <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={16} />} isDisabled={isLoading || isSaving || isRunning} isLoading={isLoading} onClick={() => void loadSettings()} />
          <XButton type="button" variant="primary" size="sm" label={t('admin.backup.runNow')} icon={<Play size={16} />} isDisabled={isRunning || settings.isRunning || selectedCount === 0} isLoading={isRunning || settings.isRunning} onClick={() => void runBackup()} />
        </div>
      </div>

      <XFormLayout>
        <XSwitch
          label={t('admin.backup.enabled')}
          description={t('admin.backup.enabledHelp')}
          value={draft.enabled}
          labelPosition="start"
          labelSpacing="spread"
          width="100%"
          onChange={(enabled) => setDraft((current) => ({ ...current, enabled }))}
        />
        <XTimeInput
          label={t('admin.backup.scheduleTime')}
          description={t('admin.backup.scheduleTimeHelp')}
          value={(draft.scheduleTime || '03:00') as ISOTimeString}
          hourFormat="24h"
          increment={5}
          width="100%"
          onChange={(scheduleTime) => setDraft((current) => ({ ...current, scheduleTime: scheduleTime || '03:00' }))}
        />
      </XFormLayout>

      <div className="migration-sensitive-warning backup-sensitive-warning">
        <XBadge label={t('admin.migration.sensitiveWarningTitle')} variant="warning" icon={<AlertTriangle size={14} />} />
        <span>{t('admin.backup.sensitiveWarningBody')}</span>
      </div>

      <div className="backup-storage-section">
        <div className="settings-section-head">
          <strong>{t('admin.backup.targets')}</strong>
          <span>{t('admin.backup.targetsBody', { count: selectedCount })}</span>
        </div>
        {storages.length === 0 ? (
          <XEmptyState title={t('admin.backup.noStorages')} description={t('admin.backup.noStoragesBody')} icon={<CloudUpload size={22} />} />
        ) : (
          <div className="backup-storage-list">
            {storages.map((storage) => (
              <div className="backup-storage-row" key={storage.key}>
                <XCheckboxInput
                  label={storage.name || storage.key}
                  description={`${storage.key} · ${storage.provider}`}
                  labelIcon={storageIcon(storage.provider)}
                  value={selectedStorages.has(storage.key)}
                  width="100%"
                  onChange={(checked) => toggleStorage(storage.key, checked)}
                />
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="settings-form-actions backup-form-actions">
        <XButton type="button" variant="primary" label={t('admin.backup.save')} icon={<Save size={17} />} isDisabled={isSaving || isRunning} isLoading={isSaving} onClick={() => void saveSettings()} />
      </div>

      <BackupLastRun lastRun={lastRun} />
    </div>
  );
}

function BackupLastRun({ lastRun }: { lastRun?: BackupRunResult }) {
  const { t } = useTranslation();
  if (!lastRun) {
    return (
      <div className="backup-last-run-empty">
        <CloudUpload size={18} />
        <span>{t('admin.backup.noLastRun')}</span>
      </div>
    );
  }
  return (
    <div className="backup-last-run">
      <div className="backup-last-run-head">
        <div>
          <strong>{t('admin.backup.lastRun')}</strong>
          <span>{t('admin.backup.lastRunMeta', { trigger: t(`admin.backup.triggers.${lastRun.trigger}`, { defaultValue: lastRun.trigger }), time: formatDate(lastRun.startedAt) })}</span>
        </div>
        <XBadge label={t(`admin.backup.statuses.${lastRun.status}`, { defaultValue: lastRun.status })} variant={resultVariant(lastRun.status)} icon={resultIcon(lastRun.status)} />
      </div>
      <div className="backup-artifact-meta">
        <span>{t('admin.backup.artifactSize', { size: formatBytes(lastRun.size || 0) })}</span>
        {lastRun.sha256 && <code>{lastRun.sha256.slice(0, 12)}</code>}
        {lastRun.error && <span className="backup-error-text">{lastRun.error}</span>}
      </div>
      {lastRun.targets && lastRun.targets.length > 0 && (
        <XList className="action-list backup-target-list" density="compact" hasDividers>
          {lastRun.targets.map((target) => (
            <XListItem
              key={`${target.storageKey}:${target.objectPath || target.error || target.status}`}
              label={target.storageName || target.storageKey}
              description={target.error || target.objectPath || t('admin.backup.noObjectPath')}
              endContent={<XBadge label={t(`admin.backup.statuses.${target.status}`, { defaultValue: target.status })} variant={resultVariant(target.status)} />}
            />
          ))}
        </XList>
      )}
    </div>
  );
}
