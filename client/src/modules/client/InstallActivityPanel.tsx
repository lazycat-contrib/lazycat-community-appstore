import { AlertTriangle, CheckCircle2, Download, History, RefreshCw, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { useTranslation } from 'react-i18next';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstallActivity } from '../../shared/types';
import { cx } from '../../shared/utils';
import { buildInstallTimeline } from './clientUxState';

export function InstallActivityPanel({
  activity,
  onDismiss,
  onRetry,
  onOpenHistory,
}: {
  activity: InstallActivity;
  onDismiss: () => void;
  onRetry?: () => void;
  onOpenHistory?: () => void;
}) {
  const { t } = useTranslation();
  const statusTone = activity.status === 'running' ? 'syncing' : activity.status === 'success' ? 'approved' : 'failed';
  const progressVariant = activity.status === 'success' ? 'success' : activity.status === 'error' ? 'error' : 'accent';
  const StatusIcon = activity.status === 'success' ? CheckCircle2 : activity.status === 'error' ? AlertTriangle : Download;
  const timeline = buildInstallTimeline(activity);

  return (
    <aside className={cx('install-panel', activity.status)} aria-live="polite" aria-label={t('installActivity.title')}>
      <span className="install-panel-icon" aria-hidden="true">
        <StatusIcon size={20} />
      </span>
      <div className="install-panel-body">
        <div className="install-panel-head">
          <strong>{activity.title}</strong>
          <StatusBadge tone={statusTone} label={t(`installActivity.status.${activity.status}`)} />
        </div>
        <span>{t(activity.stageKey)}</span>
        <XProgressBar
          className="install-panel-progress"
          label={t('installActivity.progressLabel')}
          isLabelHidden
          value={activity.status === 'running' ? undefined : 100}
          isIndeterminate={activity.status === 'running'}
          variant={progressVariant}
        />
        <ol className="install-timeline" aria-label={t('installActivity.timeline')}>
          {timeline.map((item) => (
            <li key={item.key} className={item.state} aria-current={item.state === 'current' ? 'step' : undefined}>
              <span aria-hidden="true" />
              <strong>{t(`installActivity.steps.${item.key}`)}</strong>
            </li>
          ))}
        </ol>
        <div className="install-panel-meta">
          <small>{t('installActivity.source', { source: activity.source })}</small>
          <small>{t('installActivity.checksum', { checksum: activity.checksum })}</small>
          {activity.resultMode && (
            <small>{t('installActivity.resultMode', { mode: t(`installActivity.modes.${activity.resultMode}`) })}</small>
          )}
        </div>
        {activity.messageKey && (
          <p className={cx('install-panel-message', activity.status === 'error' && 'error')}>
            {t(activity.messageKey, activity.messageParams)}
          </p>
        )}
        {activity.status !== 'running' && (onRetry || onOpenHistory) && (
          <div className="install-panel-actions">
            {activity.status === 'error' && onRetry && (
              <XButton type="button" variant="primary" size="sm" label={t('common.retry')} icon={<RefreshCw size={16} />} onClick={onRetry} />
            )}
            {onOpenHistory && (
              <XButton type="button" variant="secondary" size="sm" label={t('history.title')} icon={<History size={16} />} onClick={onOpenHistory} />
            )}
          </div>
        )}
      </div>
      <XIconButton
        type="button"
        variant="ghost"
        label={t('installActivity.dismiss')}
        icon={<X size={17} />}
        isDisabled={activity.status === 'running'}
        onClick={onDismiss}
      />
    </aside>
  );
}
