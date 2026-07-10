import { RotateCcw, Upload } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { FilePicker } from '../../../shared/components/FilePicker';
import { migrationModules, importModeOptions } from './constants';
import { MigrationPreviewSummary } from './MigrationPreviewSummary';
import type { MigrationImportMode, MigrationImportResult, MigrationModuleOptions, MigrationPreview } from './types';

export function MigrationImportCard({
  file,
  preview,
  options,
  mode,
  result,
  isPreviewing,
  isImporting,
  isBusy,
  t,
  onFileChange,
  onPreview,
  onModeChange,
  onOptionChange,
  onApply,
}: {
  file: File | null;
  preview: MigrationPreview | null;
  options: MigrationModuleOptions;
  mode: MigrationImportMode;
  result: MigrationImportResult | null;
  isPreviewing: boolean;
  isImporting: boolean;
  isBusy: boolean;
  t: (key: string, options?: any) => string;
  onFileChange: (file: File | null) => void;
  onPreview: () => void;
  onModeChange: (mode: MigrationImportMode) => void;
  onOptionChange: (key: keyof MigrationModuleOptions, value: boolean) => void;
  onApply: () => void;
}) {
  return (
    <XCard className="migration-card" padding={4}>
      <div className="migration-card-head">
        <div>
          <strong>{t('admin.migration.importTitle')}</strong>
          <span>{t('admin.migration.importBody')}</span>
        </div>
        <XBadge label={preview ? t('admin.migration.previewComplete') : t('admin.migration.previewRequired')} variant={preview ? 'success' : 'neutral'} />
      </div>

      <div className="migration-import-grid">
        <FilePicker
          label={t('admin.migration.pickPackage')}
          help={t('admin.migration.pickPackageHelp')}
          value={file}
          accept=".zip"
          disabled={isBusy}
          onChange={(nextFile) => onFileChange(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
        />
        <div className="migration-import-actions">
          <XButton
            type="button"
            variant="secondary"
            label={t('admin.migration.previewAction')}
            icon={<Upload size={17} />}
            isDisabled={!file || isBusy}
            isLoading={isPreviewing}
            onClick={onPreview}
          />
        </div>
      </div>

      {preview && (
        <>
          <MigrationPreviewSummary preview={preview} t={t} />
          <div className="migration-import-options">
            <XSelector
              label={t('admin.migration.importMode')}
              value={mode}
              options={importModeOptions.map((item) => ({ value: item.value, label: t(item.labelKey) }))}
              isDisabled={isBusy}
              onChange={(value) => onModeChange(value as MigrationImportMode)}
            />
            <p className="migration-import-note">{t('admin.migration.importModeHelp')}</p>
            <div className="migration-module-list compact">
              {migrationModules
                .filter((item) => preview.modules.includes(item.key))
                .map((item) => (
                  <XCheckboxInput
                    key={item.key}
                    label={t(item.labelKey)}
                    value={Boolean(options[item.optionKey])}
                    isDisabled={isBusy}
                    onChange={(checked) => onOptionChange(item.optionKey, checked)}
                  />
                ))}
            </div>
            <XButton
              type="button"
              variant={mode === 'replace' ? 'destructive' : 'primary'}
              label={mode === 'replace' ? t('admin.migration.replaceApplyAction') : t('admin.migration.mergeApplyAction')}
              icon={<RotateCcw size={17} />}
              isDisabled={isBusy}
              isLoading={isImporting}
              onClick={onApply}
            />
          </div>
        </>
      )}

      {result && (
        <div className="migration-result-line">
          <XBadge label={t('admin.migration.importComplete')} variant="success" />
          <span>{t('admin.migration.importResult', { created: result.created, updated: result.updated, skipped: result.skipped })}</span>
        </div>
      )}
    </XCard>
  );
}
