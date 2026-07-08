import { DatabaseBackup } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { SectionTitle } from '../../../shared/components/Feedback';
import type { Toast } from '../../../shared/types';
import { errorMessage } from '../../../shared/utils';
import { MigrationExportCard } from './MigrationExportCard';
import { MigrationImportCard } from './MigrationImportCard';
import { MigrationReplaceConfirmDialog } from './MigrationReplaceConfirmDialog';
import { useMigrationExport } from './useMigrationExport';
import { useMigrationImport } from './useMigrationImport';
import type { ApiClient } from './types';

export function AdminMigrationPanel({ api, setToast }: { api: ApiClient; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const migrationExport = useMigrationExport(api);
  const migrationImport = useMigrationImport(api);

  async function exportPackage() {
    try {
      await migrationExport.exportPackage();
      setToast({ tone: 'success', message: t('admin.migration.exportStarted') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.migration.exportFailed')) });
    }
  }

  async function previewPackage() {
    try {
      await migrationImport.previewPackage();
      setToast({ tone: 'success', message: t('admin.migration.previewLoaded') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.migration.previewFailed')) });
    }
  }

  async function applyImport(confirmReplace?: string) {
    try {
      const result = await migrationImport.applyImport(confirmReplace);
      setToast({ tone: 'success', message: result?.mode === 'replace' ? t('admin.migration.importFinishedRestarting') : t('admin.migration.importFinished') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.migration.importFailed')) });
    }
  }

  function requestApplyImport() {
    if (migrationImport.mode === 'replace') {
      migrationImport.setReplaceDialogOpen(true);
      return;
    }
    void applyImport();
  }

  return (
    <div className="migration-panel">
      <SectionTitle icon={DatabaseBackup} title={t('admin.migration.title')} />
      <p className="migration-panel-intro">{t('admin.migration.body')}</p>
      <div className="migration-panel-grid">
        <MigrationExportCard
          options={migrationExport.options}
          isExporting={migrationExport.isExporting}
          hasSelection={migrationExport.hasSelection}
          t={t}
          onOptionChange={migrationExport.setOption}
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
          t={t}
          onFileChange={migrationImport.changeFile}
          onPreview={() => void previewPackage()}
          onModeChange={migrationImport.setMode}
          onOptionChange={migrationImport.setOption}
          onApply={requestApplyImport}
        />
      </div>
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
