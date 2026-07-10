import {
  AlertCircle,
  Check,
  Cloud,
  KeyRound,
  Megaphone,
  MessageSquare,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Trash2,
  X,
} from 'lucide-react';
import { type FormEvent, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { DEFAULT_SOURCE_URL } from '../../config';
import { SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type {
  ClientSourceStats,
  InstalledApplication,
  SourceApp,
  SourceHealth,
  SourceID,
  SourceInput,
  SourceSubscription,
  SiteAd,
  Toast,
} from '../../shared/types';
import {
  belongsToSource,
  cx,
  errorMessage,
  hasInstallableVersion,
  isSourceStale,
  sourceMirrorOptions,
} from '../../shared/utils';
import { normalizeGroupCodes, normalizeSourceURL, parseSourceConfigInput } from './sourceConfig';
import { SourceOnboarding } from './SourceOnboarding';
import { SourceStatusRow } from './SourceStatusRow';

const emptySourceDraft: SourceInput = {
  name: '',
  url: DEFAULT_SOURCE_URL,
  password: '',
  defaultDownloadMirrorId: '',
  defaultRawMirrorId: '',
  groupCodes: [],
  chatEnabled: true,
  adsPreference: 'unset',
};

type SourcesViewProps = {
  sources: SourceSubscription[];
  sourceApps: SourceApp[];
  sourceAppsLoading?: boolean;
  onAddSource: (input: SourceInput) => Promise<void>;
  onUpdateSource: (source: SourceSubscription) => Promise<void>;
  onDeleteSource: (source: SourceSubscription) => Promise<void>;
  onSync: (source: SourceSubscription) => Promise<void>;
  onSyncAll: () => Promise<void | { success: number; failed: number }>;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void;
  installedApps: InstalledApplication[];
  sourceStats: ClientSourceStats;
  ads?: SiteAd[];
  setToast: (toast: Toast) => void;
};

type SourceAction = 'add' | 'edit' | 'delete' | 'sync-all' | `sync:${string}`;

export function SourcesView({
  sources,
  sourceApps,
  onAddSource,
  onUpdateSource,
  onDeleteSource,
  onSync,
  onSyncAll,
  setToast,
}: SourcesViewProps) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<SourceInput>(emptySourceDraft);
  const [editDraft, setEditDraft] = useState<SourceInput>(emptySourceDraft);
  const [editingSource, setEditingSource] = useState<SourceSubscription | null>(null);
  const [isAddSourceOpen, setIsAddSourceOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SourceSubscription | null>(null);
  const [syncingID, setSyncingID] = useState<SourceID | null>(null);
  const [syncingAll, setSyncingAll] = useState(false);
  const [savingSource, setSavingSource] = useState(false);
  const [formError, setFormError] = useState('');
  const [sourceErrors, setSourceErrors] = useState<Record<string, string>>({});
  const [syncAllResult, setSyncAllResult] = useState<{ success: number; failed: number } | null>(null);
  const [activeAction, setActiveAction] = useState<SourceAction | null>(null);
  const sourceActionRef = useRef<SourceAction | null>(null);
  const isBusy = activeAction !== null;

  const parsedDraftConfig = parseSourceConfigInput(draft.url, DEFAULT_SOURCE_URL);
  const normalizedDraftURL = parsedDraftConfig?.url || '';
  const draftGroupCodes = parsedDraftConfig?.groupCodes || [];
  const sourceNameReady = Boolean(draft.name.trim());
  const sourceURLReady = Boolean(normalizedDraftURL);
  const sourcePasswordReady = Boolean(draft.password.trim());
  const canAddSource = sourceNameReady && sourceURLReady;

  function openAddSource() {
    if (sourceActionRef.current) return;
    setFormError('');
    setDraft(emptySourceDraft);
    setIsAddSourceOpen(true);
  }

  function startSourceAction(action: SourceAction) {
    if (sourceActionRef.current) return false;
    sourceActionRef.current = action;
    setActiveAction(action);
    return true;
  }

  function finishSourceAction() {
    sourceActionRef.current = null;
    setActiveAction(null);
  }

  async function addSource(event: FormEvent) {
    event.preventDefault();
    const name = draft.name.trim();
    const url = normalizedDraftURL;
    if (!name) {
      setFormError(t('sources.nameRequired'));
      return;
    }
    if (!url) {
      setFormError(t('sources.invalid'));
      return;
    }
    if (!startSourceAction('add')) return;
    setSavingSource(true);
    setFormError('');
    try {
      const existingSource = sources.find((source) => normalizeSourceURL(source.url) === url);
      if (existingSource) {
        if (draftGroupCodes.length === 0) {
          setFormError(t('sources.duplicate'));
          return;
        }
        await onUpdateSource({
          ...existingSource,
          groupCodes: normalizeGroupCodes([...(existingSource.groupCodes || []), ...draftGroupCodes]),
        });
        setToast({ tone: 'success', message: t('sources.groupCodesMerged') });
      } else {
        await onAddSource({
          name,
          url,
          password: draft.password,
          defaultDownloadMirrorId: '',
          defaultRawMirrorId: '',
          groupCodes: draftGroupCodes,
          chatEnabled: draft.chatEnabled !== false,
          adsPreference: draft.adsPreference || 'unset',
        });
        setToast({ tone: 'success', message: t('sources.addedNext') });
      }
      setDraft(emptySourceDraft);
      setIsAddSourceOpen(false);
    } catch (error) {
      setFormError(errorMessage(error, t('sources.invalid')));
    } finally {
      setSavingSource(false);
      finishSourceAction();
    }
  }

  function openEditSource(source: SourceSubscription) {
    if (sourceActionRef.current) return;
    setFormError('');
    setEditingSource(source);
    setEditDraft({
      name: source.name,
      url: source.url,
      password: source.password,
      defaultDownloadMirrorId: source.defaultDownloadMirrorId || '',
      defaultRawMirrorId: source.defaultRawMirrorId || '',
      groupCodes: source.groupCodes || [],
      chatEnabled: source.chatEnabled !== false,
      adsPreference: source.adsPreference || 'unset',
    });
  }

  async function saveEditedSource(event: FormEvent) {
    event.preventDefault();
    if (!editingSource) return;
    const name = editDraft.name.trim();
    const url = normalizeSourceURL(editDraft.url);
    if (!name) {
      setFormError(t('sources.nameRequired'));
      return;
    }
    if (!url) {
      setFormError(t('sources.invalid'));
      return;
    }
    if (!startSourceAction('edit')) return;
    setSavingSource(true);
    setFormError('');
    try {
      await onUpdateSource({
        ...editingSource,
        name,
        url,
        password: editDraft.password,
        defaultDownloadMirrorId: editDraft.defaultDownloadMirrorId || '',
        defaultRawMirrorId: editDraft.defaultRawMirrorId || '',
        groupCodes: normalizeGroupCodes(editDraft.groupCodes || []),
        chatEnabled: editDraft.chatEnabled !== false,
        adsPreference: editDraft.adsPreference || editingSource.adsPreference || 'unset',
      });
      setEditingSource(null);
      setToast({ tone: 'success', message: t('sources.updated') });
    } catch (error) {
      setFormError(errorMessage(error, t('toast.sourceSaveFailed')));
    } finally {
      setSavingSource(false);
      finishSourceAction();
    }
  }

  function healthFor(source: SourceSubscription): SourceHealth {
    if (syncingID === source.id) return 'syncing';
    if (source.lastErrorCode === 'auth') return 'auth';
    if (source.lastError) return 'failed';
    if (isSourceStale(source)) return 'stale';
    if (source.lastSync) return 'synced';
    return 'unsynced';
  }

  async function runSourceSync(source: SourceSubscription) {
    const key = String(source.id);
    if (!startSourceAction(`sync:${key}`)) {
      setFormError(t('sources.health.syncing'));
      return;
    }
    setSyncingID(source.id);
    setFormError('');
    setSourceErrors((current) => ({ ...current, [key]: '' }));
    try {
      await onSync(source);
    } catch (error) {
      setSourceErrors((current) => ({ ...current, [key]: errorMessage(error, t('toast.sourceSyncFailed')) }));
    } finally {
      setSyncingID(null);
      finishSourceAction();
    }
  }

  async function runSyncAll() {
    if (sources.length === 0) return;
    if (!startSourceAction('sync-all')) {
      setFormError(t('sources.health.syncing'));
      return;
    }
    setSyncingAll(true);
    setFormError('');
    setSyncAllResult(null);
    try {
      const result = await onSyncAll();
      if (result) setSyncAllResult(result);
    } catch (error) {
      setFormError(errorMessage(error, t('toast.sourceSyncFailed')));
    } finally {
      setSyncingAll(false);
      finishSourceAction();
    }
  }

  async function confirmDelete() {
    if (!deleteTarget || !startSourceAction('delete')) return;
    setSavingSource(true);
    setFormError('');
    try {
      await onDeleteSource(deleteTarget);
      setDeleteTarget(null);
      setToast({ tone: 'success', message: t('sources.deleted') });
    } catch (error) {
      setFormError(errorMessage(error, t('toast.sourceSaveFailed')));
    } finally {
      setSavingSource(false);
      finishSourceAction();
    }
  }

  return (
    <>
      {sources.length === 0 ? (
        <SourceOnboarding onAdd={openAddSource} />
      ) : (
        <section className="page-grid client-sources-page">
          <div className="page-heading with-action">
            <div>
              <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
              <h1>{t('sources.title')}</h1>
              <p>{t('sources.managementSubtitle')}</p>
            </div>
            <div className="row-actions">
              <XButton type="button" variant="primary" label={t('sources.add')} icon={<Plus size={18} />} isDisabled={isBusy} onClick={openAddSource} />
              <XButton
                type="button"
                variant="secondary"
                label={syncingAll ? t('sources.syncingAll') : t('sources.syncAll')}
                icon={<RefreshCw size={18} className={syncingAll ? 'spin' : undefined} />}
                isDisabled={isBusy}
                onClick={() => void runSyncAll()}
              />
            </div>
          </div>
          {formError && !isAddSourceOpen && !editingSource && !deleteTarget && (
            <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>
          )}
          {syncAllResult && (
            <div className={cx('client-sync-result', syncAllResult.failed > 0 && 'partial')} role="status">
              <strong>{syncAllResult.failed > 0 ? t('sources.syncResultPartial') : t('sources.syncResultSuccess')}</strong>
              <span>{t('sources.syncResultCounts', syncAllResult)}</span>
            </div>
          )}
          <div className="client-source-list">
            {sources.map((source) => {
              const scopedApps = sourceApps.filter((app) => belongsToSource(app, source));
              return (
                <SourceStatusRow
                  key={source.id}
                  source={source}
                  health={healthFor(source)}
                  appCount={source.lastAppCount ?? scopedApps.length}
                  installableCount={source.lastInstallableCount ?? scopedApps.filter(hasInstallableVersion).length}
                  isSyncing={syncingID === source.id}
                  isBusy={isBusy}
                  error={sourceErrors[String(source.id)]}
                  onSync={() => runSourceSync(source)}
                  onEdit={() => openEditSource(source)}
                  onDelete={() => {
                    if (sourceActionRef.current) return;
                    setFormError('');
                    setDeleteTarget(source);
                  }}
                />
              );
            })}
          </div>
        </section>
      )}

      {isAddSourceOpen && (
        <ModalLayer onClose={() => !savingSource && setIsAddSourceOpen(false)} purpose="form">
          <form
            className="modal-panel form-panel source-add-dialog"
            aria-label={t('sources.addTitle')}
            aria-busy={savingSource}
            onSubmit={addSource}
            noValidate
          >
            <XIconButton type="button" label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={savingSource} onClick={() => setIsAddSourceOpen(false)} />
            <SectionTitle icon={Cloud} title={t('sources.addTitle')} />
            <div className="source-readiness" aria-label={t('sources.addReadiness')}>
              <div className={cx('readiness-step', sourceNameReady && 'ready')}>
                <StatusBadge tone={sourceNameReady ? 'approved' : 'unlisted'} icon={sourceNameReady ? <Check size={14} /> : <AlertCircle size={14} />} label={sourceNameReady ? t('sources.ready') : t('sources.needsValue')} />
                <strong>{t('sources.readinessName')}</strong>
                <small>{sourceNameReady ? t('sources.readinessNameReady') : t('sources.readinessNameMissing')}</small>
              </div>
              <div className={cx('readiness-step', sourceURLReady && 'ready')}>
                <StatusBadge tone={sourceURLReady ? 'approved' : 'unlisted'} icon={sourceURLReady ? <Check size={14} /> : <AlertCircle size={14} />} label={sourceURLReady ? t('sources.ready') : t('sources.needsValue')} />
                <strong>{t('sources.readinessUrl')}</strong>
                <small>{sourceURLReady ? t('sources.readinessUrlReady') : t('sources.readinessUrlMissing')}</small>
              </div>
              <div className={cx('readiness-step', sourcePasswordReady && 'ready')}>
                <StatusBadge tone={sourcePasswordReady ? 'synced' : 'unsynced'} icon={<KeyRound size={14} />} label={sourcePasswordReady ? t('sources.filled') : t('sources.optional')} />
                <strong>{t('sources.readinessPassword')}</strong>
                <small>{sourcePasswordReady ? t('sources.readinessPasswordReady') : t('sources.readinessPasswordOptional')}</small>
              </div>
            </div>
            <XTextInput label={t('common.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
            <XTextInput label={t('sources.urlOrConfig')} value={draft.url} onChange={(value) => setDraft({ ...draft, url: value })} />
            {draftGroupCodes.length > 0 && (
              <div className="source-group-preview" aria-label={t('sources.groupCodesDetected')}>
                <StatusBadge tone="synced" label={t('sources.groupCodesDetected')} />
                <span>{t('sources.groupCodesDetectedCount', { count: draftGroupCodes.length })}</span>
              </div>
            )}
            <XTextInput type="password" label={t('sources.password')} value={draft.password} onChange={(value) => setDraft({ ...draft, password: value })} />
            {!canAddSource && <p className="field-help">{t('sources.addBlocked')}</p>}
            {formError && <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} isDisabled={savingSource} onClick={() => setIsAddSourceOpen(false)} />
              <XButton type="submit" variant="primary" label={savingSource ? t('common.saving') : t('sources.add')} icon={savingSource ? <RefreshCw size={18} className="spin" /> : <Cloud size={18} />} isDisabled={!canAddSource || savingSource} />
            </div>
          </form>
        </ModalLayer>
      )}

      {editingSource && (
        <ModalLayer onClose={() => !savingSource && setEditingSource(null)} purpose="form">
          <form
            className="modal-panel form-panel source-add-dialog"
            aria-label={t('sources.editTitle')}
            aria-busy={savingSource}
            onSubmit={saveEditedSource}
            noValidate
          >
            <XIconButton type="button" label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={savingSource} onClick={() => setEditingSource(null)} />
            <SectionTitle icon={Pencil} title={t('sources.editTitle')} />
            <XTextInput label={t('common.name')} value={editDraft.name} onChange={(value) => setEditDraft({ ...editDraft, name: value })} />
            <XTextInput label={t('sources.url')} value={editDraft.url} onChange={(value) => setEditDraft({ ...editDraft, url: value })} />
            <XTextInput
              label={t('sources.groupCodes')}
              value={(editDraft.groupCodes || []).join(', ')}
              onChange={(value) => setEditDraft({ ...editDraft, groupCodes: value.split(/[,\s;]+/) })}
            />
            <XTextInput type="password" label={t('sources.password')} value={editDraft.password} onChange={(value) => setEditDraft({ ...editDraft, password: value })} />
            <XSwitch
              label={t('sources.chatEnabled')}
              description={editingSource.chatAvailable ? t('sources.chatEnabledHelp') : t('sources.chatUnavailableHelp')}
              value={editDraft.chatEnabled !== false}
              isDisabled={!editingSource.chatAvailable}
              disabledMessage={t('sources.chatUnavailableHelp')}
              labelSpacing="spread"
              width="100%"
              labelIcon={<MessageSquare size={16} />}
              onChange={(checked) => setEditDraft({ ...editDraft, chatEnabled: checked })}
            />
            {(editingSource.ads || []).length > 0 && (
              <XSwitch
                label={t('sources.adsEnabled')}
                description={t('sources.adsEnabledHelp')}
                value={editDraft.adsPreference === 'enabled'}
                labelSpacing="spread"
                width="100%"
                labelIcon={<Megaphone size={16} />}
                onChange={(checked) => setEditDraft({ ...editDraft, adsPreference: checked ? 'enabled' : 'disabled' })}
              />
            )}
            <XSelector
              label={t('sources.defaultDownloadMirror')}
              description={t('sources.defaultDownloadMirrorHelp')}
              value={editDraft.defaultDownloadMirrorId}
              options={sourceMirrorOptions(editingSource, 'download', t('sources.directMirror'))}
              onChange={(value) => setEditDraft({ ...editDraft, defaultDownloadMirrorId: value })}
            />
            <XSelector
              label={t('sources.defaultRawMirror')}
              description={t('sources.defaultRawMirrorHelp')}
              value={editDraft.defaultRawMirrorId}
              options={sourceMirrorOptions(editingSource, 'raw', t('sources.directMirror'))}
              onChange={(value) => setEditDraft({ ...editDraft, defaultRawMirrorId: value })}
            />
            {formError && <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} isDisabled={savingSource} onClick={() => setEditingSource(null)} />
              <XButton type="submit" variant="primary" label={savingSource ? t('common.saving') : t('common.save')} icon={savingSource ? <RefreshCw size={18} className="spin" /> : <Save size={18} />} isDisabled={savingSource} />
            </div>
          </form>
        </ModalLayer>
      )}

      {deleteTarget && (
        <ModalLayer onClose={() => !savingSource && setDeleteTarget(null)} purpose="form">
          <div className="modal-panel form-panel client-source-delete-dialog" role="alertdialog" aria-labelledby="client-source-delete-title">
            <SectionTitle icon={AlertCircle} title={t('sources.deleteTitle')} />
            <p id="client-source-delete-title">{t('sources.deleteBody', { name: deleteTarget.name })}</p>
            {formError && <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} isDisabled={savingSource} onClick={() => setDeleteTarget(null)} />
              <XButton type="button" variant="destructive" label={savingSource ? t('common.deleting') : t('sources.deleteConfirm')} icon={savingSource ? <RefreshCw size={18} className="spin" /> : <Trash2 size={18} />} isDisabled={savingSource} onClick={() => void confirmDelete()} />
            </div>
          </div>
        </ModalLayer>
      )}
    </>
  );
}
