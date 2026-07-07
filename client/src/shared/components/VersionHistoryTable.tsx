import type { ReactNode } from 'react';
import { Badge as XBadge, type BadgeVariant } from '@astryxdesign/core/Badge';
import { Table as XTable, pixel, proportional, type TableColumn } from '@astryxdesign/core/Table';
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
      renderCell: (row) => <strong className="version-table-primary">{row.version || '-'}</strong>,
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
      renderCell: (row) => <code>{shortSHA(row.sha256)}</code>,
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

  return (
    <div className="version-table-wrap">
      <XTable
        data={rows as VersionHistoryTableRow[]}
        columns={columns}
        idKey={(row) => row.id}
        density="compact"
        dividers="rows"
        textOverflow="truncate"
        hasHover
      />
    </div>
  );
}
