import {
  AlertCircle,
  Check,
  Cloud,
  Download,
  KeyRound,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Server,
  X,
} from 'lucide-react';
import { FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { DEFAULT_SOURCE_URL } from '../../config';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import type {
  ClientSourceStats,
  InstalledApplication,
  SourceApp,
  SourceHealth,
  SourceHealthFilter,
  SourceID,
  SourceInput,
  SourceSubscription,
  Toast,
} from '../../shared/types';
import {
  belongsToSource,
  cx,
  errorMessage,
  formatDate,
  hasInstallableVersion,
  isSourceStale,
  sourceMirrorOptions,
  sourceMirrorSummary,
} from '../../shared/utils';
import { SourceAppGrid } from './SourceAppGrid';
import {
  matchesSourceAppCategory,
  matchesSourceAppSource,
  sourceAppCategoryOptions,
  sourceAppSourceOptions,
} from './sourceAppFilters';

export function SourcesView({
  sources,
  sourceApps,
  onAddSource,
  onUpdateSource,
  onDeleteSource,
  onSync,
  onSyncAll,
  onOpenSource,
  onInstall,
  installedApps,
  sourceStats,
  setToast,
}: {
  sources: SourceSubscription[];
  sourceApps: SourceApp[];
  onAddSource: (input: SourceInput) => Promise<void>;
  onUpdateSource: (source: SourceSubscription) => Promise<void>;
  onDeleteSource: (source: SourceSubscription) => Promise<void>;
  onSync: (source: SourceSubscription) => Promise<void>;
  onSyncAll: () => Promise<void>;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void;
  installedApps: InstalledApplication[];
  sourceStats: ClientSourceStats;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const emptyDraft: SourceInput = { name: '', url: DEFAULT_SOURCE_URL, password: '', defaultDownloadMirrorId: '', defaultRawMirrorId: '' };
  const [draft, setDraft] = useState(emptyDraft);
  const [syncingID, setSyncingID] = useState<SourceID | null>(null);
  const [confirmDeleteSource, setConfirmDeleteSource] = useState<SourceID | null>(null);
  const [sourceHealthFilter, setSourceHealthFilter] = useState<SourceHealthFilter>('all');
  const [isAddSourceOpen, setIsAddSourceOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<SourceSubscription | null>(null);
  const [editDraft, setEditDraft] = useState<SourceInput>(emptyDraft);
  const [selectedSyncedSource, setSelectedSyncedSource] = useState('all');
  const [selectedSyncedCategory, setSelectedSyncedCategory] = useState('all');

  function normalizedSourceURL(rawURL: string) {
    try {
      const parsed = new URL(rawURL.trim());
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
      return parsed.toString();
    } catch {
      return '';
    }
  }

  const normalizedDraftURL = normalizedSourceURL(draft.url);
  const sourceNameReady = Boolean(draft.name.trim());
  const sourceURLReady = Boolean(normalizedDraftURL);
  const sourcePasswordReady = Boolean(draft.password.trim());
  const canAddSource = sourceNameReady && sourceURLReady;

  async function addSource(event: FormEvent) {
    event.preventDefault();
    const name = draft.name.trim();
    const url = normalizedDraftURL;
    if (!name) {
      setToast({ tone: 'error', message: t('sources.nameRequired') });
      return;
    }
    if (!url) {
      setToast({ tone: 'error', message: t('sources.invalid') });
      return;
    }
    if (sources.some((source) => normalizedSourceURL(source.url) === url)) {
      setToast({ tone: 'neutral', message: t('sources.duplicate') });
      return;
    }
    try {
      await onAddSource({ name, url, password: draft.password, defaultDownloadMirrorId: '', defaultRawMirrorId: '' });
      setDraft(emptyDraft);
      setIsAddSourceOpen(false);
      setToast({ tone: 'success', message: t('sources.addedNext') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('sources.invalid')) });
    }
  }

  function openEditSource(source: SourceSubscription) {
    setEditingSource(source);
    setEditDraft({
      name: source.name,
      url: source.url,
      password: source.password,
      defaultDownloadMirrorId: source.defaultDownloadMirrorId || '',
      defaultRawMirrorId: source.defaultRawMirrorId || '',
    });
  }

  async function saveEditedSource(event: FormEvent) {
    event.preventDefault();
    if (!editingSource) return;
    const name = editDraft.name.trim();
    const url = normalizedSourceURL(editDraft.url);
    if (!name) {
      setToast({ tone: 'error', message: t('sources.nameRequired') });
      return;
    }
    if (!url) {
      setToast({ tone: 'error', message: t('sources.invalid') });
      return;
    }
    try {
      await onUpdateSource({
        ...editingSource,
        name,
        url,
        password: editDraft.password,
        defaultDownloadMirrorId: editDraft.defaultDownloadMirrorId || '',
        defaultRawMirrorId: editDraft.defaultRawMirrorId || '',
      });
      setEditingSource(null);
      setToast({ tone: 'success', message: t('sources.updated') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.sourceSaveFailed')) });
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

  const sourceHealthFilterItems: Array<{ key: SourceHealthFilter; label: string; count: number }> = [
    { key: 'all', label: t('sources.filters.all'), count: sources.length },
    { key: 'synced', label: t('sources.filters.synced'), count: sources.filter((source) => healthFor(source) === 'synced').length },
    { key: 'stale', label: t('sources.filters.stale'), count: sources.filter((source) => healthFor(source) === 'stale').length },
    { key: 'auth', label: t('sources.filters.auth'), count: sources.filter((source) => healthFor(source) === 'auth').length },
    { key: 'unsynced', label: t('sources.filters.unsynced'), count: sources.filter((source) => healthFor(source) === 'unsynced').length },
    { key: 'failed', label: t('sources.filters.failed'), count: sources.filter((source) => healthFor(source) === 'failed').length },
  ];
  const filteredSources = sources.filter((source) => sourceHealthFilter === 'all' || healthFor(source) === sourceHealthFilter);
  const syncedSourceOptions = sourceAppSourceOptions(sourceApps);
  const syncedCategoryOptions = sourceAppCategoryOptions(sourceApps, t('common.uncategorized'));
  const filteredSyncedSourceApps = sourceApps.filter(
    (app) => matchesSourceAppSource(app, selectedSyncedSource) && matchesSourceAppCategory(app, selectedSyncedCategory),
  );

  async function deleteSource(source: SourceSubscription) {
    if (confirmDeleteSource !== source.id) {
      setConfirmDeleteSource(source.id);
      setToast({ tone: 'neutral', message: t('sources.confirmDelete', { name: source.name }) });
      return;
    }
    try {
      await onDeleteSource(source);
      setConfirmDeleteSource(null);
      setToast({ tone: 'success', message: t('sources.deleted') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.sourceSaveFailed')) });
    }
  }

  return (
    <section className="page-grid">
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
          <h1>{t('sources.title')}</h1>
          <p>{t('sources.subtitle')}</p>
        </div>
        <div className="row-actions">
          <XButton type="button" variant="primary" label={t('sources.add')} icon={<Plus size={18} />} onClick={() => setIsAddSourceOpen(true)} />
          <XButton type="button" variant="secondary" label={t('sources.syncAll')} icon={<RefreshCw size={18} />} onClick={() => void onSyncAll()} />
        </div>
      </div>

      <div className="client-summary-grid source-summary" aria-label={t('sources.summary')}>
        <div>
          <span>{t('search.sourcesTotal')}</span>
          <strong>{sourceStats.sourceCount}</strong>
        </div>
        <div>
          <span>{t('search.syncedSources')}</span>
          <strong>{sourceStats.syncedSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.staleSourceCount > 0 && 'warning')}>
          <span>{t('search.staleSources')}</span>
          <strong>{sourceStats.staleSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.authSourceCount > 0 && 'warning')}>
          <span>{t('search.authSources')}</span>
          <strong>{sourceStats.authSourceCount}</strong>
        </div>
        <div>
          <span>{t('search.installableApps')}</span>
          <strong>{sourceStats.installableSourceAppCount}</strong>
        </div>
        <div className={cx(sourceStats.failedSourceCount > 0 && 'warning')}>
          <span>{t('search.failedSources')}</span>
          <strong>{sourceStats.failedSourceCount}</strong>
        </div>
      </div>

      {isAddSourceOpen && (
        <div className="modal-backdrop" role="presentation" onClick={() => setIsAddSourceOpen(false)}>
          <form
            className="modal-panel form-panel source-add-dialog"
            role="dialog"
            aria-modal="true"
            aria-label={t('sources.addTitle')}
            onClick={(event) => event.stopPropagation()}
            onSubmit={addSource}
            noValidate
          >
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setIsAddSourceOpen(false)} />
            <SectionTitle icon={Cloud} title={t('sources.addTitle')} />
            <div className="source-readiness" aria-label={t('sources.addReadiness')}>
              <div className={cx('readiness-step', sourceNameReady && 'ready')}>
                <span className={cx('status-badge', sourceNameReady ? 'approved' : 'unlisted')}>
                  {sourceNameReady ? <Check size={14} /> : <AlertCircle size={14} />}
                  {sourceNameReady ? t('sources.ready') : t('sources.needsValue')}
                </span>
                <strong>{t('sources.readinessName')}</strong>
                <small>{sourceNameReady ? t('sources.readinessNameReady') : t('sources.readinessNameMissing')}</small>
              </div>
              <div className={cx('readiness-step', sourceURLReady && 'ready')}>
                <span className={cx('status-badge', sourceURLReady ? 'approved' : 'unlisted')}>
                  {sourceURLReady ? <Check size={14} /> : <AlertCircle size={14} />}
                  {sourceURLReady ? t('sources.ready') : t('sources.needsValue')}
                </span>
                <strong>{t('sources.readinessUrl')}</strong>
                <small>{sourceURLReady ? t('sources.readinessUrlReady') : t('sources.readinessUrlMissing')}</small>
              </div>
              <div className={cx('readiness-step', sourcePasswordReady && 'ready')}>
                <span className={cx('status-badge', sourcePasswordReady ? 'synced' : 'unsynced')}>
                  <KeyRound size={14} />
                  {sourcePasswordReady ? t('sources.filled') : t('sources.optional')}
                </span>
                <strong>{t('sources.readinessPassword')}</strong>
                <small>{sourcePasswordReady ? t('sources.readinessPasswordReady') : t('sources.readinessPasswordOptional')}</small>
              </div>
            </div>
            <XTextInput label={t('common.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
            <XTextInput label={t('sources.url')} value={draft.url} onChange={(value) => setDraft({ ...draft, url: value })} />
            <XTextInput type="password" label={t('sources.password')} value={draft.password} onChange={(value) => setDraft({ ...draft, password: value })} />
            {!canAddSource && <p className="field-help">{t('sources.addBlocked')}</p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setIsAddSourceOpen(false)} />
              <XButton type="submit" variant="primary" label={t('sources.add')} icon={<Cloud size={18} />} isDisabled={!canAddSource} />
            </div>
          </form>
        </div>
      )}

      {editingSource && (
        <div className="modal-backdrop" role="presentation" onClick={() => setEditingSource(null)}>
          <form
            className="modal-panel form-panel source-add-dialog"
            role="dialog"
            aria-modal="true"
            aria-label={t('sources.editTitle')}
            onClick={(event) => event.stopPropagation()}
            onSubmit={saveEditedSource}
            noValidate
          >
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setEditingSource(null)} />
            <SectionTitle icon={Pencil} title={t('sources.editTitle')} />
            <XTextInput label={t('common.name')} value={editDraft.name} onChange={(value) => setEditDraft({ ...editDraft, name: value })} />
            <XTextInput label={t('sources.url')} value={editDraft.url} onChange={(value) => setEditDraft({ ...editDraft, url: value })} />
            <XTextInput type="password" label={t('sources.password')} value={editDraft.password} onChange={(value) => setEditDraft({ ...editDraft, password: value })} />
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
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setEditingSource(null)} />
              <XButton type="submit" variant="primary" label={t('common.save')} icon={<Save size={18} />} />
            </div>
          </form>
        </div>
      )}

      <section className="panel">
        <SectionTitle icon={Server} title={t('sources.subscriptions')} />
        <XToggleButtonGroup value={sourceHealthFilter} onChange={(value) => setSourceHealthFilter((value || 'all') as SourceHealthFilter)} label={t('sources.statusFilter')} size="sm">
          {sourceHealthFilterItems.map((item) => (
            <XToggleButton
              key={item.key}
              value={item.key}
              label={`${item.label} ${item.count}`}
            />
          ))}
        </XToggleButtonGroup>
        <div className="source-list">
          {sources.length === 0 ? (
            <EmptyState icon={Cloud} title={t('sources.empty')} />
          ) : filteredSources.length === 0 ? (
            <EmptyState icon={Cloud} title={t('sources.emptyFiltered')} body={t('sources.emptyFilteredBody')} />
          ) : (
            filteredSources.map((source) => {
              const sourceScopedApps = sourceApps.filter((app) => belongsToSource(app, source));
              const syncedAppCount = source.lastAppCount ?? sourceScopedApps.length;
              const installableAppCount = source.lastInstallableCount ?? sourceScopedApps.filter(hasInstallableVersion).length;
              const health = healthFor(source);
              const healthHint =
                health === 'auth'
                  ? t('sources.healthHints.auth')
                  : health === 'failed'
                    ? t('sources.healthHints.failed')
                    : health === 'stale'
                      ? t('sources.healthHints.stale')
                      : health === 'unsynced'
                        ? t('sources.healthHints.unsynced')
                        : health === 'syncing'
                          ? t('sources.healthHints.syncing')
                          : t('sources.healthHints.synced');
              return (
                <div className="source-row" key={source.id}>
                  <div>
                    <div className="source-row-header">
                      <strong>{source.name}</strong>
                      <span className={cx('status-badge', health)} aria-live="polite">{t(`sources.health.${health}`)}</span>
                    </div>
                    <span className="source-url" title={source.url}>{source.url}</span>
                    <div className="source-facts">
                      <small>{source.lastSync ? t(health === 'stale' ? 'sources.lastSyncStale' : 'sources.lastSync', { time: formatDate(source.lastSync) }) : t('sources.neverSynced')}</small>
                      <small>{t('sources.syncedAppCount', { count: syncedAppCount })}</small>
                      <small>{t('sources.installableAppCount', { count: installableAppCount })}</small>
                    </div>
                    {source.lastError && (
                      <p className={cx(health === 'auth' ? 'inline-warning' : 'inline-alert')}>
                        {health === 'auth' ? <KeyRound size={15} /> : <AlertCircle size={15} />}
                        <span>{source.lastError}</span>
                      </p>
                    )}
                    {(health === 'auth' || !source.lastError) && (
                      <p className={cx(health === 'synced' ? 'inline-success' : 'inline-warning')}>
                        {health === 'synced' ? <Check size={15} /> : health === 'auth' ? <KeyRound size={15} /> : <AlertCircle size={15} />}
                        <span>{healthHint}</span>
                      </p>
                    )}
                    <div className="source-facts source-config-facts">
                      <small>{source.password ? t('sources.passwordConfigured') : t('sources.passwordNotConfigured')}</small>
                      <small>{t('sources.downloadMirrorConfigured', { name: sourceMirrorSummary(source, 'download', t('sources.directMirror')) })}</small>
                      <small>{t('sources.rawMirrorConfigured', { name: sourceMirrorSummary(source, 'raw', t('sources.directMirror')) })}</small>
                    </div>
                  </div>
                  <div className="row-actions">
                    <XIconButton label={t('sources.editSource', { name: source.name })} variant="ghost" icon={<Pencil size={17} />} onClick={() => openEditSource(source)} />
                    <XIconButton
                      label={t('sources.syncSource', { name: source.name })}
                      variant="ghost"
                      icon={<RefreshCw size={17} />}
                      isDisabled={syncingID === source.id}
                      onClick={() =>
                        void (async () => {
                          setSyncingID(source.id);
                          try {
                            await onSync(source);
                          } catch (error) {
                            setToast({ tone: 'error', message: error instanceof Error ? error.message : t('toast.sourceSyncFailed') });
                          } finally {
                            setSyncingID(null);
                          }
                        })()
                      }
                    />
                    <XIconButton label={t('sources.deleteSource', { name: source.name })} variant="destructive" icon={<X size={17} />} onClick={() => deleteSource(source)} />
                  </div>
                </div>
              );
            })
          )}
        </div>
      </section>

      <section className="panel">
        <SectionTitle icon={Download} title={t('sources.syncedApps')} />
        <div className="filter-bar">
          <XSelector
            label={t('search.sourceFilter')}
            value={selectedSyncedSource}
            options={[
              { value: 'all', label: t('search.allSources') },
              ...syncedSourceOptions.map((option) => ({ value: option.key, label: `${option.label} (${option.count})` })),
            ]}
            onChange={setSelectedSyncedSource}
          />
          <XSelector
            label={t('search.categoryFilter')}
            value={selectedSyncedCategory}
            options={[
              { value: 'all', label: t('search.allCategories') },
              ...syncedCategoryOptions.map((option) => ({ value: option.key, label: `${option.label} (${option.count})` })),
            ]}
            onChange={setSelectedSyncedCategory}
          />
        </div>
        <SourceAppGrid
          apps={filteredSyncedSourceApps}
          installedApps={installedApps}
          onOpen={onOpenSource}
          onInstall={onInstall}
          onGoSources={() => setIsAddSourceOpen(true)}
          showEmptyAction={sourceApps.length === 0}
          emptyTitle={sourceApps.length === 0 ? t('search.noSyncedApps') : t('search.noResultsTitle')}
          emptyBody={sourceApps.length === 0 ? t('search.noSyncedAppsBody') : t('search.noFilterResultsBody')}
        />
      </section>
    </section>
  );
}
