import { AlertTriangle, CheckCircle2, Download, History, RefreshCw, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { StatusDot } from '@astryxdesign/core/StatusDot';
import { useTranslation } from 'react-i18next';
import type { InstallActivity } from '../../shared/types';
import { cx } from '../../shared/utils';
import { buildInstallTimeline } from './clientUxState';

export function InstallActivityPanel({
  activity,
  onDismiss,
  onRetry,
  onOpenHistory,
  onCancel,
  embedded = false,
}: {
  activity: InstallActivity;
  onDismiss: () => void;
  onRetry?: () => void;
  onOpenHistory?: () => void;
  onCancel?: () => void;
  embedded?: boolean;
}) {
  const { t } = useTranslation();
  const progressVariant = activity.status === 'success' ? 'success' : activity.status === 'error' ? 'error' : 'accent';
  const StatusIcon = activity.status === 'success' ? CheckCircle2 : activity.status === 'error' ? AlertTriangle : Download;
  const statusVariant = activity.status === 'running' ? 'accent' : activity.status === 'success' ? 'success' : activity.status === 'error' ? 'error' : 'neutral';
  const timeline = buildInstallTimeline(activity);
  const isRunning = activity.status === 'running';

  return (
    <aside className={cx('install-panel', activity.status, embedded && 'install-panel-embedded')} aria-live="polite" aria-label={t('installActivity.title')}>
      <span className="install-panel-icon" aria-hidden="true">
        <StatusIcon size={20} />
      </span>
      <div className="install-panel-body">
        <div className="install-panel-head">
          <strong>{activity.title}</strong>
          <span className="install-panel-status">
            <StatusDot variant={statusVariant} label={t(`installActivity.status.${activity.status}`)} isPulsing={isRunning} />
            <span>{t(`installActivity.status.${activity.status}`)}</span>
          </span>
        </div>
        <span>{t(activity.stageKey)}</span>
        {activity.detail && <small className="install-panel-detail">{activity.detail}</small>}
        <XProgressBar
          className="install-panel-progress"
          label={t('installActivity.progressLabel')}
          isLabelHidden
          value={isRunning ? activity.progress : 100}
          isIndeterminate={isRunning && !activity.progressKnown}
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
        {(isRunning || onRetry || onOpenHistory || embedded) && (
          <div className="install-panel-actions">
            {isRunning && onCancel && (
              <XButton
                type="button"
                variant="secondary"
                size="sm"
                label={activity.isCancelling ? t('installActivity.cancelling') : t('installActivity.cancel')}
                icon={<X size={16} />}
                isDisabled={activity.isCancelling}
                onClick={onCancel}
              />
            )}
            {(activity.status === 'error' || activity.status === 'cancelled') && onRetry && (
              <XButton type="button" variant="primary" size="sm" label={t('common.retry')} icon={<RefreshCw size={16} />} onClick={onRetry} />
            )}
            {onOpenHistory && (
              <XButton type="button" variant="secondary" size="sm" label={t('history.title')} icon={<History size={16} />} onClick={onOpenHistory} />
            )}
            {embedded && !isRunning && (
              <XButton type="button" variant="secondary" size="sm" label={t('installActivity.dismiss')} icon={<X size={16} />} onClick={onDismiss} />
            )}
          </div>
        )}
      </div>
      {!embedded && (
        <XIconButton
          type="button"
          variant="ghost"
          label={t('installActivity.dismiss')}
          icon={<X size={17} />}
          isDisabled={isRunning}
          onClick={onDismiss}
        />
      )}
    </aside>
  );
}
