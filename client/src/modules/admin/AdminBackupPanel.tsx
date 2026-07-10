import { useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, CheckCircle2, CloudUpload, Database, HardDrive, Play, RefreshCw, Server, XCircle } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { EmptyState as XEmptyState } from '@astryxdesign/core/EmptyState';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { NumberInput as XNumberInput } from '@astryxdesign/core/NumberInput';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { TimeInput as XTimeInput, type ISOTimeString } from '@astryxdesign/core/TimeInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import type { BackupRunResult, BackupSettings, BackupTargetSettings, Toast } from '../../shared/types';
import { errorMessage, formatBytes, formatDate } from '../../shared/utils';
import { AdminOperationResultPanel } from './AdminOperationResult';
import { AdminSaveBar } from './AdminSaveBar';
import type { StorageSettings } from './StorageSettingsPanel';
import { areAdminDraftsEqual, type AdminOperationResult, type AdminSaveStatus } from './adminState';

type BackupDraft = {
  enabled: boolean;
  scheduleTime: string;
  retentionCount: number;
  storageKeys: string[];
  targets: BackupTargetSettings[];
};

const defaultBackupDirectory = 'backups/appstore';

const defaultBackupSettings: BackupSettings = {
  enabled: false,
  scheduleTime: '03:00',
  retentionCount: 0,
  storageKeys: [],
  targets: [],
  isRunning: false,
};

function draftFromSettings(settings: BackupSettings): BackupDraft {
  const targets = settings.targets && settings.targets.length > 0
    ? settings.targets
    : (settings.storageKeys || []).map((storageKey) => ({ storageKey, directory: defaultBackupDirectory }));
  return {
    enabled: Boolean(settings.enabled),
    scheduleTime: settings.scheduleTime || '03:00',
    retentionCount: settings.retentionCount ?? 0,
    storageKeys: targets.map((target) => target.storageKey),
    targets: targets.map((target) => ({ storageKey: target.storageKey, directory: target.directory || defaultBackupDirectory })),
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
  const [savedDraft, setSavedDraft] = useState<BackupDraft>(draftFromSettings(defaultBackupSettings));
  const [saveStatus, setSaveStatus] = useState<AdminSaveStatus>('idle');
  const [operationResult, setOperationResult] = useState<AdminOperationResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const draftRef = useRef<BackupDraft>(draftFromSettings(defaultBackupSettings));
  const savedDraftRef = useRef<BackupDraft>(draftFromSettings(defaultBackupSettings));
  const draftRevisionRef = useRef(0);
  const activeActionRef = useRef<'load' | 'save' | 'run' | null>(null);

  const selectedCount = draft.storageKeys.length;
  const selectedStorages = useMemo(() => new Set(draft.storageKeys), [draft.storageKeys]);
  const targetByStorage = useMemo(() => new Map(draft.targets.map((target) => [target.storageKey, target])), [draft.targets]);
  const lastRun = settings.lastRun;
  const isDirty = !areAdminDraftsEqual(draft, savedDraft);

  useEffect(() => {
    void loadSettings();
  }, []);

  async function loadSettings() {
    if (activeActionRef.current || !areAdminDraftsEqual(draftRef.current, savedDraftRef.current)) return;
    activeActionRef.current = 'load';
    setIsLoading(true);
    try {
      const data = await api<{ settings: BackupSettings }>('/api/v1/admin/backups/settings');
      const next = data.settings || defaultBackupSettings;
      const nextDraft = draftFromSettings(next);
      setSettings(next);
      draftRef.current = nextDraft;
      savedDraftRef.current = nextDraft;
      setDraft(nextDraft);
      setSavedDraft(nextDraft);
      setSaveStatus('idle');
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.backup.loadFailed')) });
    } finally {
      setIsLoading(false);
      activeActionRef.current = null;
    }
  }

  function editDraft(update: (current: BackupDraft) => BackupDraft) {
    const next = update(draftRef.current);
    draftRevisionRef.current += 1;
    draftRef.current = next;
    setSaveStatus('dirty');
    setDraft(next);
  }

  function toggleStorage(key: string, checked: boolean) {
    editDraft((current) => {
      const keys = new Set(current.storageKeys);
      let targets = current.targets;
      if (checked) {
        keys.add(key);
        if (!targets.some((target) => target.storageKey === key)) {
          targets = [...targets, { storageKey: key, directory: defaultBackupDirectory }];
        }
      } else {
        keys.delete(key);
        targets = targets.filter((target) => target.storageKey !== key);
      }
      return { ...current, storageKeys: Array.from(keys), targets };
    });
  }

  function updateTargetDirectory(key: string, directory: string) {
    editDraft((current) => ({
      ...current,
      targets: current.targets.map((target) => (target.storageKey === key ? { ...target, directory } : target)),
    }));
  }

  async function saveSettings() {
    if (areAdminDraftsEqual(draftRef.current, savedDraftRef.current) || activeActionRef.current) return;
    activeActionRef.current = 'save';
    const draftSnapshot = draftRef.current;
    const draftRevision = draftRevisionRef.current;
    setIsSaving(true);
    setSaveStatus('saving');
    try {
      const payload = {
        ...draftSnapshot,
        storageKeys: draftSnapshot.targets.map((target) => target.storageKey),
      };
      const data = await api<{ settings: BackupSettings }>('/api/v1/admin/backups/settings', {
        method: 'PATCH',
        body: JSON.stringify(payload),
      });
      const next = data.settings || defaultBackupSettings;
      const nextDraft = draftFromSettings(next);
      setSettings(next);
      savedDraftRef.current = nextDraft;
      setSavedDraft(nextDraft);
      if (draftRevision === draftRevisionRef.current) {
        draftRef.current = nextDraft;
        setDraft(nextDraft);
        setSaveStatus('saved');
      } else {
        setSaveStatus('dirty');
      }
      setOperationResult({ variant: 'success', title: t('admin.backup.title'), message: t('admin.backup.saved'), occurredAt: new Date().toISOString() });
      setToast({ tone: 'success', message: t('admin.backup.saved') });
    } catch (error) {
      const message = errorMessage(error, t('admin.backup.saveFailed'));
      setSaveStatus(draftRevision === draftRevisionRef.current ? 'error' : 'dirty');
      setOperationResult({ variant: 'error', title: t('admin.backup.title'), message, occurredAt: new Date().toISOString() });
      setToast({ tone: 'error', message });
    } finally {
      setIsSaving(false);
      activeActionRef.current = null;
    }
  }

  async function runBackup() {
    if (!areAdminDraftsEqual(draftRef.current, savedDraftRef.current) || activeActionRef.current) return;
    activeActionRef.current = 'run';
    setIsRunning(true);
    try {
      const data = await api<{ result: BackupRunResult; settings: BackupSettings }>('/api/v1/admin/backups/run', { method: 'POST' });
      const next = data.settings || { ...settings, lastRun: data.result };
      setSettings(next);
      const variant = data.result.status === 'success' ? 'success' : data.result.status === 'partial' ? 'warning' : 'error';
      const message = data.result.status === 'success'
        ? t('admin.backup.runSucceeded')
        : data.result.status === 'partial'
          ? t('admin.backup.runPartial')
          : t('admin.backup.runFailed');
      setOperationResult({
        variant,
        title: t('admin.backup.lastRun'),
        message,
        occurredAt: data.result.startedAt || new Date().toISOString(),
        target: data.result.targets?.map((target) => target.storageName || target.storageKey).join(', '),
      });
      if (data.result.status === 'success') {
        setToast({ tone: 'success', message: t('admin.backup.runSucceeded') });
      } else if (data.result.status === 'partial') {
        setToast({ tone: 'neutral', message: t('admin.backup.runPartial') });
      } else {
        setToast({ tone: 'error', message: t('admin.backup.runFailed') });
      }
    } catch (error) {
      const message = errorMessage(error, t('admin.backup.runFailed'));
      const target = storages.filter((storage) => selectedStorages.has(storage.key)).map((storage) => storage.name || storage.key).join(', ');
      setOperationResult({ variant: 'error', title: t('admin.backup.lastRun'), message, occurredAt: new Date().toISOString(), target });
      setToast({ tone: 'error', message });
    } finally {
      setIsRunning(false);
      activeActionRef.current = null;
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
          <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={16} />} isDisabled={isLoading || isSaving || isRunning || isDirty} isLoading={isLoading} onClick={() => void loadSettings()} />
          <XButton type="button" variant="primary" size="sm" label={t('admin.backup.runNow')} icon={<Play size={16} />} isDisabled={isLoading || isSaving || isRunning || settings.isRunning || selectedCount === 0 || isDirty} isLoading={isRunning || settings.isRunning} onClick={() => void runBackup()} />
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
          onChange={(enabled) => editDraft((current) => ({ ...current, enabled }))}
        />
        <XTimeInput
          label={t('admin.backup.scheduleTime')}
          description={t('admin.backup.scheduleTimeHelp')}
          value={(draft.scheduleTime || '03:00') as ISOTimeString}
          hourFormat="24h"
          increment={5}
          width="100%"
          onChange={(scheduleTime) => editDraft((current) => ({ ...current, scheduleTime: scheduleTime || '03:00' }))}
        />
        <XNumberInput
          label={t('admin.backup.retentionCount')}
          description={t('admin.backup.retentionCountHelp')}
          value={draft.retentionCount}
          min={0}
          max={100}
          step={1}
          isIntegerOnly
          width="100%"
          onChange={(retentionCount) => editDraft((current) => ({ ...current, retentionCount }))}
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
                {selectedStorages.has(storage.key) && (
                  <XTextInput
                    label={t('admin.backup.targetDirectory')}
                    description={t('admin.backup.targetDirectoryHelp')}
                    value={targetByStorage.get(storage.key)?.directory || defaultBackupDirectory}
                    width="100%"
                    onChange={(directory) => updateTargetDirectory(storage.key, directory)}
                  />
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      <AdminSaveBar status={saveStatus} isDirty={isDirty} saveLabel={t('admin.backup.save')} isDisabled={isRunning} onSave={() => void saveSettings()} />
      <AdminOperationResultPanel result={operationResult} retryLabel={!isDirty ? t('admin.backup.runNow') : undefined} isRetrying={isRunning} isRetryDisabled={isLoading || isSaving || isRunning || isDirty} onRetry={!isDirty ? () => void runBackup() : undefined} />
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
