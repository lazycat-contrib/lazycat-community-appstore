import { AlertTriangle, Download } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { migrationModules } from './constants';
import type { MigrationModuleOptions } from './types';

export function MigrationExportCard({
  options,
  isExporting,
  hasSelection,
  t,
  onOptionChange,
  onExport,
}: {
  options: MigrationModuleOptions;
  isExporting: boolean;
  hasSelection: boolean;
  t: (key: string, options?: any) => string;
  onOptionChange: (key: keyof MigrationModuleOptions, value: boolean) => void;
  onExport: () => void;
}) {
  const containsSensitiveData = options.includePeople || options.includeSite;

  return (
    <XCard className="migration-card" padding={4}>
      <div className="migration-card-head">
        <div>
          <strong>{t('admin.migration.exportTitle')}</strong>
          <span>{t('admin.migration.exportBody')}</span>
        </div>
        <XButton
          type="button"
          variant="primary"
          label={t('admin.migration.exportAction')}
          icon={<Download size={17} />}
          isDisabled={!hasSelection || isExporting}
          isLoading={isExporting}
          onClick={onExport}
        />
      </div>

      <div className="migration-module-list">
        {migrationModules.map((item) => {
          const Icon = item.icon;
          return (
            <div className="migration-module-row" key={item.key}>
              <XCheckboxInput
                label={t(item.labelKey)}
                description={t(item.descriptionKey)}
                labelIcon={<Icon size={18} />}
                width="100%"
                value={Boolean(options[item.optionKey])}
                onChange={(checked) => onOptionChange(item.optionKey, checked)}
              />
            </div>
          );
        })}
      </div>

      {containsSensitiveData && (
        <div className="migration-sensitive-warning">
          <XBadge label={t('admin.migration.sensitiveWarningTitle')} variant="warning" icon={<AlertTriangle size={14} />} />
          <span>{t('admin.migration.sensitiveWarningBody')}</span>
        </div>
      )}
    </XCard>
  );
}
