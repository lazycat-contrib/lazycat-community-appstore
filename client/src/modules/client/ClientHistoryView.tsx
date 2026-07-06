import { AlertCircle, Check, ChevronRight, History, RefreshCw } from 'lucide-react';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import type { InstallHistoryEntry, SourceApp } from '../../shared/types';
import { cx, formatDate, normalizeAppIdentity, shortSHA } from '../../shared/utils';

export function ClientHistoryView({
  history,
  sourceApps,
  onRefresh,
  onOpenSource,
}: {
  history: InstallHistoryEntry[];
  sourceApps: SourceApp[];
  onRefresh: () => void;
  onOpenSource: (app: SourceApp) => void;
}) {
  const { t } = useTranslation();
  const sourceAppByPackage = useMemo(() => {
    const map = new Map<string, SourceApp>();
    sourceApps.forEach((app) => {
      if (app.packageId) map.set(normalizeAppIdentity(app.packageId), app);
      if (app.slug) map.set(normalizeAppIdentity(app.slug), app);
    });
    return map;
  }, [sourceApps]);
  const successCount = history.filter((item) => item.result === 'SUCCESS').length;
  const failedCount = history.filter((item) => item.result === 'FAILED').length;

  return (
    <section className="page-grid">
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
          <h1>{t('history.title')}</h1>
          <p>{t('history.body')}</p>
        </div>
        <XButton type="button" variant="primary" label={t('common.refresh')} icon={<RefreshCw size={18} />} onClick={onRefresh} />
      </div>
      <div className="client-summary-grid" aria-label={t('history.summary')}>
        <div>
          <span>{t('history.total')}</span>
          <strong>{history.length}</strong>
        </div>
        <div>
          <span>{t('history.success')}</span>
          <strong>{successCount}</strong>
        </div>
        <div className={cx(failedCount > 0 && 'warning')}>
          <span>{t('history.failed')}</span>
          <strong>{failedCount}</strong>
        </div>
      </div>
      <section className="panel">
        <SectionTitle icon={History} title={t('history.events')} />
        <div className="history-list">
          {history.length === 0 ? (
            <EmptyState icon={History} title={t('history.empty')} body={t('history.emptyBody')} />
          ) : (
            history.map((item) => {
              const matched = sourceAppByPackage.get(normalizeAppIdentity(item.packageId));
              return (
                <article className="history-row" key={item.id}>
                  <div className={cx('history-result', item.result === 'SUCCESS' ? 'success' : 'failed')}>
                    {item.result === 'SUCCESS' ? <Check size={18} /> : <AlertCircle size={18} />}
                  </div>
                  <div className="history-main">
                    <strong>{item.appName}</strong>
                    <span>{item.packageId}</span>
                    <div className="history-facts">
                      <small>{item.sourceName || t('profile.installedLocalExisting')}</small>
                      <small>{item.version || '-'}</small>
                      <small>{formatDate(item.createdAt)}</small>
                      <small>{shortSHA(item.sha256)}</small>
                    </div>
                    {item.error && (
                      <p className="inline-alert">
                        <AlertCircle size={15} />
                        <span>{item.error}</span>
                      </p>
                    )}
                  </div>
                  <div className="row-actions">
                    <span className={cx('status-badge', item.result === 'SUCCESS' ? 'approved' : 'failed')}>
                      {item.result === 'SUCCESS' ? t('history.success') : t('history.failed')}
                    </span>
                    {matched && (
                      <XButton
                        type="button"
                        variant="secondary"
                        size="sm"
                        label={t('history.openApp')}
                        icon={<ChevronRight size={17} />}
                        onClick={() => onOpenSource(matched)}
                      />
                    )}
                  </div>
                </article>
              );
            })
          )}
        </div>
      </section>
    </section>
  );
}
