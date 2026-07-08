import { AlertTriangle, CalendarClock, DatabaseBackup } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { formatBytes, formatDate } from '../../../shared/utils';
import { migrationModules } from './constants';
import type { MigrationPreview } from './types';

export function MigrationPreviewSummary({ preview, t }: { preview: MigrationPreview; t: (key: string, options?: any) => string }) {
  const countEntries = Object.entries(preview.counts || {}).filter(([, value]) => value > 0);
  const warnings = preview.warnings || [];

  return (
    <div className="migration-preview-summary">
      <div className="migration-preview-head">
        <div>
          <strong>{t('admin.migration.previewReady')}</strong>
          <span>{t('admin.migration.previewMeta', { version: preview.serverVersion || '-', date: formatDate(preview.createdAt) })}</span>
        </div>
        <XBadge label={t('admin.migration.formatVersion', { version: preview.formatVersion })} variant="info" />
      </div>

      <div className="migration-module-badges">
        {migrationModules
          .filter((item) => preview.modules.includes(item.key))
          .map((item) => {
            const Icon = item.icon;
            return <XBadge key={item.key} label={t(item.labelKey)} icon={<Icon size={14} />} variant="neutral" />;
          })}
      </div>

      <XList className="migration-count-list" density="compact" hasDividers>
        <XListItem
          label={t('admin.migration.totalFileBytes')}
          description={formatBytes(preview.totalFileBytes)}
          startContent={<DatabaseBackup size={17} />}
        />
        {countEntries.map(([key, value]) => (
          <XListItem
            key={key}
            label={t(`admin.migration.counts.${key}`, { defaultValue: key })}
            description={String(value)}
            startContent={<CalendarClock size={17} />}
          />
        ))}
      </XList>

      {warnings.length > 0 && (
        <div className="migration-warning-list">
          <XBadge label={t('admin.migration.warningCount', { count: warnings.length })} variant="warning" icon={<AlertTriangle size={14} />} />
          {warnings.map((warning, index) => (
            <p key={`${warning}-${index}`}>{warning}</p>
          ))}
        </div>
      )}
    </div>
  );
}
