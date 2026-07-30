import { useCallback, useEffect, useState } from 'react';
import { Ban, RefreshCw, Search, ShieldCheck } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import type { DownstreamClientUser, PaginatedResponse, Toast } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';
import { EmptyState } from '../../shared/components/Feedback';

export function DownstreamClientsPanel({ setToast }: { setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [items, setItems] = useState<DownstreamClientUser[]>([]);
  const [search, setSearch] = useState('');
  const [blockedOnly, setBlockedOnly] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const query = new URLSearchParams({ pageSize: '100' });
      if (search.trim()) query.set('search', search.trim());
      if (blockedOnly) query.set('blocked', 'true');
      const data = await api<PaginatedResponse<DownstreamClientUser>>(`/api/v1/admin/downstream-clients?${query}`);
      setItems(data.items || []);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.downstream.loadFailed')) });
    } finally {
      setLoading(false);
    }
  }, [blockedOnly, search, setToast, t]);

  useEffect(() => { void load(); }, [load]);

  async function toggleBlocked(item: DownstreamClientUser) {
    try {
      if (item.blocked) {
        await api(`/api/v1/admin/downstream-clients/${encodeURIComponent(item.clientUserId)}/unblock`, { method: 'POST' });
      } else {
        const reason = window.prompt(t('admin.downstream.reasonPrompt'))?.trim();
        if (!reason) return;
        await api(`/api/v1/admin/downstream-clients/${encodeURIComponent(item.clientUserId)}/block`, { method: 'POST', body: JSON.stringify({ reason }) });
      }
      setToast({ tone: 'success', message: t(item.blocked ? 'admin.downstream.unblocked' : 'admin.downstream.blocked') });
      await load();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.downstream.updateFailed')) });
    }
  }

  return (
    <section className="panel downstream-clients-panel">
      <div className="page-heading with-action">
        <div><h2>{t('admin.downstream.title')}</h2><p>{t('admin.downstream.description')}</p></div>
        <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={16} />} isLoading={loading} onClick={() => void load()} />
      </div>
      <form className="catalog-search-toolbar" onSubmit={(event) => { event.preventDefault(); void load(); }}>
        <label className="downstream-search"><Search size={16} /><input value={search} placeholder={t('admin.downstream.search')} onChange={(event) => setSearch(event.target.value)} /></label>
        <label className="downstream-filter"><input type="checkbox" checked={blockedOnly} onChange={(event) => setBlockedOnly(event.target.checked)} />{t('admin.downstream.blockedOnly')}</label>
      </form>
      {items.length === 0 ? <EmptyState icon={ShieldCheck} title={t('admin.downstream.empty')} body={t('admin.downstream.emptyBody')} /> : (
        <div className="downstream-client-list">
          {items.map((item) => (
            <article key={item.id}>
              <div><strong>{item.displayName || item.clientUserId}</strong><code>{item.clientUserId}</code><small>{t('admin.downstream.lastSeen', { date: formatDate(item.lastSeenAt) })}</small></div>
              <div className="row-actions">
                {item.seenInComments && <XBadge label={t('admin.downstream.comments')} variant="neutral" />}
                {item.seenInWishes && <XBadge label={t('admin.downstream.wishes')} variant="neutral" />}
                {item.blocked && <XBadge label={item.blockReason || t('admin.downstream.blockedBadge')} variant="error" />}
                <XButton type="button" size="sm" variant={item.blocked ? 'secondary' : 'destructive'} label={t(item.blocked ? 'admin.downstream.unblock' : 'admin.downstream.block')} icon={<Ban size={15} />} onClick={() => void toggleBlocked(item)} />
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
