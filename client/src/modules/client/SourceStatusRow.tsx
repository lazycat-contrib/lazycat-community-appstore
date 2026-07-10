import { AlertCircle, KeyRound, Pencil, RefreshCw, Server, Trash2, UsersRound } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useTranslation } from 'react-i18next';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { SourceHealth, SourceSubscription } from '../../shared/types';
import { cx, formatDate } from '../../shared/utils';

export function SourceStatusRow({
  source,
  health,
  appCount,
  installableCount,
  isSyncing,
  isBusy,
  error,
  onSync,
  onEdit,
  onDelete,
}: {
  source: SourceSubscription;
  health: SourceHealth;
  appCount: number;
  installableCount: number;
  isSyncing: boolean;
  isBusy: boolean;
  error?: string;
  onSync: () => void | Promise<void>;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  const groupCount = source.groups?.length || source.groupCodes?.length || 0;
  const issue = error || source.lastError || '';
  return (
    <article className={cx('client-source-row', issue && 'has-error')} aria-busy={isSyncing}>
      <div className="client-source-row-main">
        <span className="client-source-row-icon" aria-hidden="true"><Server size={19} /></span>
        <div>
          <div className="client-source-row-title">
            <strong>{source.name}</strong>
            <StatusBadge tone={health} label={t(`sources.health.${health}`)} />
          </div>
          <span className="client-source-url" title={source.url}>{source.url}</span>
          <div className="client-source-facts">
            <span>{source.lastSync ? t('sources.lastSync', { time: formatDate(source.lastSync) }) : t('sources.neverSynced')}</span>
            <span>{t('sources.syncedAppCount', { count: appCount })}</span>
            <span>{t('sources.installableAppCount', { count: installableCount })}</span>
            <span><UsersRound size={14} /> {t('sources.groupCount', { count: groupCount })}</span>
          </div>
        </div>
      </div>
      {issue && (
        <div className="client-source-error" role="alert">
          {source.lastErrorCode === 'auth' ? <KeyRound size={16} /> : <AlertCircle size={16} />}
          <span>{issue}</span>
          <XButton
            type="button"
            variant="secondary"
            size="sm"
            label={t('common.retry')}
            icon={<RefreshCw size={16} />}
            isDisabled={isBusy}
            onClick={() => void onSync()}
          />
        </div>
      )}
      <div className="client-source-row-actions">
        {!issue && (
          <XButton
            type="button"
            variant="secondary"
            size="sm"
            label={isSyncing ? t('sources.health.syncing') : t('common.sync')}
            icon={<RefreshCw size={16} className={isSyncing ? 'spin' : undefined} />}
            isDisabled={isBusy}
            onClick={() => void onSync()}
          />
        )}
        <XIconButton type="button" variant="ghost" label={t('sources.editTitle')} icon={<Pencil size={17} />} isDisabled={isBusy} onClick={onEdit} />
        <XIconButton type="button" variant="destructive" label={t('sources.deleteSource', { name: source.name })} icon={<Trash2 size={17} />} isDisabled={isBusy} onClick={onDelete} />
      </div>
    </article>
  );
}
