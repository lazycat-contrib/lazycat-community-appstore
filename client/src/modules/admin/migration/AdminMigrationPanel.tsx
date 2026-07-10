import { DatabaseBackup } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SectionTitle } from '../../../shared/components/Feedback';
import type { Toast } from '../../../shared/types';
import { errorMessage } from '../../../shared/utils';
import { AdminOperationResultPanel } from '../AdminOperationResult';
import type { AdminOperationResult } from '../adminState';
import { MigrationExportCard } from './MigrationExportCard';
import { MigrationImportCard } from './MigrationImportCard';
import { MigrationReplaceConfirmDialog } from './MigrationReplaceConfirmDialog';
import { useMigrationExport } from './useMigrationExport';
import { useMigrationImport } from './useMigrationImport';
import type { ApiClient, MigrationImportMode, MigrationModuleOptions } from './types';

export function AdminMigrationPanel({ api, setToast }: { api: ApiClient; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const migrationExport = useMigrationExport(api);
  const migrationImport = useMigrationImport(api);
  const [activity, setActivity] = useState<{ result: AdminOperationResult; operation: 'export' | 'preview' | 'import'; mode?: 'merge' | 'replace' } | null>(null);
  const [activeOperation, setActiveOperation] = useState<'export' | 'preview' | 'import' | null>(null);
  const activeOperationRef = useRef<'export' | 'preview' | 'import' | null>(null);
  const panelBusy = activeOperation !== null || migrationImport.replaceDialogOpen;

  function startOperation(operation: 'export' | 'preview' | 'import') {
    if (activeOperationRef.current) return false;
    activeOperationRef.current = operation;
    setActiveOperation(operation);
    return true;
  }

  function finishOperation() {
    activeOperationRef.current = null;
    setActiveOperation(null);
  }

  async function exportPackage() {
    if (!migrationExport.hasSelection) return;
    if (!startOperation('export')) return;
    try {
      await migrationExport.exportPackage();
      const message = t('admin.migration.exportStarted');
      setActivity({ operation: 'export', result: { variant: 'success', title: t('admin.migration.exportTitle'), message, occurredAt: new Date().toISOString() } });
      setToast({ tone: 'success', message });
    } catch (error) {
      const message = errorMessage(error, t('admin.migration.exportFailed'));
      setActivity({ operation: 'export', result: { variant: 'error', title: t('admin.migration.exportTitle'), message, occurredAt: new Date().toISOString() } });
      setToast({ tone: 'error', message });
    } finally {
      finishOperation();
    }
  }

  async function previewPackage() {
    if (!migrationImport.file) return;
    if (!startOperation('preview')) return;
    try {
      await migrationImport.previewPackage();
      const message = t('admin.migration.previewLoaded');
      setActivity({ operation: 'preview', result: { variant: 'success', title: t('admin.migration.previewComplete'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name } });
      setToast({ tone: 'success', message });
    } catch (error) {
      const message = errorMessage(error, t('admin.migration.previewFailed'));
      setActivity({ operation: 'preview', result: { variant: 'error', title: t('admin.migration.previewRequired'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name } });
      setToast({ tone: 'error', message });
    } finally {
      finishOperation();
    }
  }

  async function applyImport(confirmReplace?: string) {
    if (!migrationImport.file || !migrationImport.preview) return;
    if (!startOperation('import')) return;
    const mode = migrationImport.mode;
    try {
      const result = await migrationImport.applyImport(confirmReplace);
      if (!result) return;
      const message = result.mode === 'replace' ? t('admin.migration.importFinishedRestarting') : t('admin.migration.importFinished');
      setActivity({
        operation: 'import',
        mode: result.mode,
        result: {
          variant: result.warnings?.length ? 'warning' : 'success',
          title: t('admin.migration.importComplete'),
          message: `${message} · ${t('admin.migration.importResult', { created: result.created, updated: result.updated, skipped: result.skipped })}`,
          occurredAt: new Date().toISOString(),
          target: migrationImport.file?.name,
        },
      });
      setToast({ tone: 'success', message });
    } catch (error) {
      const message = errorMessage(error, t('admin.migration.importFailed'));
      setActivity({ operation: 'import', mode, result: { variant: 'error', title: t('admin.migration.importTitle'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name } });
      setToast({ tone: 'error', message });
    } finally {
      finishOperation();
    }
  }

  function requestApplyImport() {
    if (panelBusy) return;
    if (migrationImport.mode === 'replace') {
      migrationImport.setReplaceDialogOpen(true);
      return;
    }
    void applyImport();
  }

  function changeExportOption(key: keyof MigrationModuleOptions, value: boolean) {
    migrationExport.setOption(key, value);
    if (activity?.operation === 'export') setActivity(null);
  }

  function changeImportFile(file: File | null) {
    migrationImport.changeFile(file);
    if (activity?.operation === 'preview' || activity?.operation === 'import') setActivity(null);
  }

  function changeImportMode(mode: MigrationImportMode) {
    migrationImport.setMode(mode);
    if (activity?.operation === 'import') setActivity(null);
  }

  function changeImportOption(key: keyof MigrationModuleOptions, value: boolean) {
    migrationImport.setOption(key, value);
    if (activity?.operation === 'import') setActivity(null);
  }

  function retryActivity() {
    if (!activity || panelBusy) return;
    if (activity.operation === 'export') {
      void exportPackage();
      return;
    }
    if (activity.operation === 'preview') {
      void previewPackage();
      return;
    }
    if (migrationImport.mode === 'replace') {
      migrationImport.setReplaceDialogOpen(true);
      return;
    }
    void applyImport();
  }

  const retryLabel = activity?.operation === 'export'
    ? t('admin.migration.exportAction')
    : activity?.operation === 'preview'
      ? t('admin.migration.previewAction')
      : activity?.operation === 'import'
        ? activity.mode === 'replace' ? t('admin.migration.replaceApplyAction') : t('admin.migration.mergeApplyAction')
        : undefined;
  const canRetry = activity?.operation === 'export'
    ? migrationExport.hasSelection
    : activity?.operation === 'preview'
      ? Boolean(migrationImport.file)
      : activity?.operation === 'import'
        ? Boolean(migrationImport.file && migrationImport.preview && activity.mode === migrationImport.mode)
        : false;

  return (
    <div className="migration-panel">
      <SectionTitle icon={DatabaseBackup} title={t('admin.migration.title')} />
      <p className="migration-panel-intro">{t('admin.migration.body')}</p>
      <div className="migration-panel-grid">
        <MigrationExportCard
          options={migrationExport.options}
          isExporting={migrationExport.isExporting}
          isBusy={panelBusy}
          hasSelection={migrationExport.hasSelection}
          t={t}
          onOptionChange={changeExportOption}
          onExport={() => void exportPackage()}
        />
        <MigrationImportCard
          file={migrationImport.file}
          preview={migrationImport.preview}
          options={migrationImport.options}
          mode={migrationImport.mode}
          result={migrationImport.result}
          isPreviewing={migrationImport.isPreviewing}
          isImporting={migrationImport.isImporting}
          isBusy={panelBusy}
          t={t}
          onFileChange={changeImportFile}
          onPreview={() => void previewPackage()}
          onModeChange={changeImportMode}
          onOptionChange={changeImportOption}
          onApply={requestApplyImport}
        />
      </div>
      <AdminOperationResultPanel
        result={activity?.result || null}
        retryLabel={canRetry ? retryLabel : undefined}
        isRetrying={activeOperation !== null}
        isRetryDisabled={panelBusy || !canRetry}
        onRetry={activity && canRetry ? retryActivity : undefined}
      />
      {migrationImport.replaceDialogOpen && (
        <MigrationReplaceConfirmDialog
          isImporting={migrationImport.isImporting}
          t={t}
          onClose={() => migrationImport.setReplaceDialogOpen(false)}
          onConfirm={(value) => void applyImport(value)}
        />
      )}
    </div>
  );
}
