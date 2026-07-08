import { AlertTriangle, CheckCircle2, Download, X } from 'lucide-react';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { useTranslation } from 'react-i18next';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { InstallActivity } from '../../shared/types';
import { cx } from '../../shared/utils';

export function InstallActivityPanel({ activity, onDismiss }: { activity: InstallActivity; onDismiss: () => void }) {
  const { t } = useTranslation();
  const statusTone = activity.status === 'running' ? 'syncing' : activity.status === 'success' ? 'approved' : 'failed';
  const progressVariant = activity.status === 'success' ? 'success' : activity.status === 'error' ? 'error' : 'accent';
  const StatusIcon = activity.status === 'success' ? CheckCircle2 : activity.status === 'error' ? AlertTriangle : Download;

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
          value={activity.progress}
          variant={progressVariant}
        />
        <div className="install-panel-meta">
          <small>{t('installActivity.source', { source: activity.source })}</small>
          <small>{t('installActivity.checksum', { checksum: activity.checksum })}</small>
          {activity.resultMode && (
            <small>{t('installActivity.resultMode', { mode: t(`installActivity.modes.${activity.resultMode}`) })}</small>
          )}
        </div>
        {activity.messageKey && <p>{t(activity.messageKey, activity.messageParams)}</p>}
      </div>
      <XIconButton type="button" variant="ghost" label={t('installActivity.dismiss')} icon={<X size={17} />} onClick={onDismiss} />
    </aside>
  );
}
