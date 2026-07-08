import type { ReactNode } from 'react';
import { Badge as XBadge, type BadgeVariant } from '@astryxdesign/core/Badge';
import { Collapsible as XCollapsible } from '@astryxdesign/core/Collapsible';
import { Table as XTable, pixel, proportional, type TableColumn } from '@astryxdesign/core/Table';
import { Text as XText } from '@astryxdesign/core/Text';
import { useTranslation } from 'react-i18next';
import { formatBytes, formatDate, shortSHA } from '../utils';

export type VersionHistoryRow = {
  id: string | number;
  version: string;
  sourceType?: string;
  sizeBytes?: number;
  sha256?: string;
  publishedAt?: string;
  statusLabel?: string;
  statusVariant?: BadgeVariant;
  isLatest?: boolean;
  action?: ReactNode;
};

type VersionHistoryTableRow = VersionHistoryRow & Record<string, unknown>;

export function VersionHistoryTable({ rows }: { rows: VersionHistoryRow[] }) {
  const { t } = useTranslation();
  const hasStatus = rows.some((row) => row.statusLabel);
  const hasAction = rows.some((row) => row.action);
  const columns: TableColumn<VersionHistoryTableRow>[] = [
    {
      key: 'version',
      header: t('common.version'),
      width: proportional(1, { minWidth: 140 }),
      renderCell: (row) => (
        <XText className="version-table-primary" type="label" display="block" wordBreak="break-word">
          {row.version || '-'}
        </XText>
      ),
    },
    {
      key: 'sourceType',
      header: t('common.source'),
      width: proportional(1, { minWidth: 132 }),
      renderCell: (row) => row.sourceType || '-',
    },
    {
      key: 'sizeBytes',
      header: t('common.fileSize'),
      width: pixel(120),
      renderCell: (row) => (row.sizeBytes && row.sizeBytes > 0 ? formatBytes(row.sizeBytes) : t('drawer.sizeMissing')),
    },
    {
      key: 'sha256',
      header: t('common.checksum'),
      width: proportional(1, { minWidth: 132 }),
      renderCell: (row) => (
        <XText type="code" display="block" wordBreak="break-word">
          {shortSHA(row.sha256)}
        </XText>
      ),
    },
    {
      key: 'publishedAt',
      header: t('common.publishedAt'),
      width: pixel(150),
      renderCell: (row) => (row.publishedAt ? formatDate(row.publishedAt) : '-'),
    },
  ];

  if (hasStatus) {
    columns.push({
      key: 'statusLabel',
      header: t('common.status'),
      width: pixel(120),
      renderCell: (row) => row.statusLabel ? <XBadge variant={row.statusVariant || 'neutral'} label={row.statusLabel} /> : '-',
    });
  }

  if (hasAction) {
    columns.push({
      key: 'action',
      header: t('common.action'),
      width: pixel(128),
      align: 'end',
      renderCell: (row) => row.action || null,
    });
  }

  function renderTable(tableRows: VersionHistoryTableRow[], label: string) {
    return (
      <XTable
        aria-label={label}
        data={tableRows}
        columns={columns}
        idKey={(row) => row.id}
        density="compact"
        dividers="rows"
        textOverflow="truncate"
        hasHover
      />
    );
  }

  const latestRows = rows.filter((row) => row.isLatest);
  const latest = latestRows.length > 0 ? latestRows : rows.slice(0, 1);
  const latestIds = new Set(latest.map((row) => row.id));
  const historical = rows.filter((row) => !latestIds.has(row.id));

  return (
    <div className="version-history-stack">
      <div className="version-table-wrap">
        {renderTable(latest as VersionHistoryTableRow[], t('versionHistory.latestTable'))}
      </div>
      {historical.length > 0 && (
        <XCollapsible
          className="version-history-collapsible"
          defaultIsOpen={false}
          trigger={(
            <span className="version-history-trigger">
              <strong>{t('versionHistory.historyTitle')}</strong>
              <span>{t('versionHistory.historyCount', { count: historical.length })}</span>
            </span>
          )}
        >
          <div className="version-table-wrap history">
            {renderTable(historical as VersionHistoryTableRow[], t('versionHistory.historyTable'))}
          </div>
        </XCollapsible>
      )}
    </div>
  );
}
