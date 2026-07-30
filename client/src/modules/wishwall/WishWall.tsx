import { useCallback, useEffect, useMemo, useState, type CSSProperties, type FormEvent } from 'react';
import { Ban, Lightbulb, MessageSquareReply, Pencil, Plus, RefreshCw, SlidersHorizontal, Trash2 } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Collapsible as XCollapsible } from '@astryxdesign/core/Collapsible';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { useTranslation } from 'react-i18next';
import { api, clientApi } from '../../shared/api';
import type { PaginatedResponse, SourceSubscription, Toast, User, Wish, WishKind, WishStatus } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';
import { EmptyState } from '../../shared/components/Feedback';

const kinds: WishKind[] = ['SUGGESTION', 'APP_REQUEST', 'CUSTOMIZATION'];
const statuses: WishStatus[] = ['OPEN', 'PLANNED', 'IN_PROGRESS', 'COMPLETED', 'REJECTED'];

type WishDraft = {
  kind: WishKind;
  title: string;
  body: string;
  referenceUrl: string;
  contactEmail: string;
  contactOther: string;
  statusText: string;
};

const emptyDraft = (): WishDraft => ({
  kind: 'APP_REQUEST', title: '', body: '', referenceUrl: '', contactEmail: '', contactOther: '', statusText: '',
});

export function WishWall({
  mode,
  sources,
  user,
  setToast,
}: {
  mode: 'server' | 'client';
  sources: SourceSubscription[];
  user: User | null;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const supportedSources = useMemo(() => sources.filter((source) => source.wishWallAvailable), [sources]);
  const [sourceID, setSourceID] = useState<string>('');
  const [items, setItems] = useState<Wish[]>([]);
  const [loading, setLoading] = useState(false);
  const [kindFilter, setKindFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [draft, setDraft] = useState<WishDraft>(emptyDraft);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [replyDrafts, setReplyDrafts] = useState<Record<number, string>>({});
  const [statusDrafts, setStatusDrafts] = useState<Record<number, { status: WishStatus; text: string }>>({});
  const isAdmin = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';

  useEffect(() => {
    if (mode !== 'client') return;
    if (supportedSources.some((source) => String(source.id) === sourceID)) return;
    setSourceID(supportedSources[0] ? String(supportedSources[0].id) : '');
  }, [mode, sourceID, supportedSources]);

  const load = useCallback(async () => {
    if (mode === 'client' && !sourceID) {
      setItems([]);
      return;
    }
    setLoading(true);
    try {
      const query = new URLSearchParams({ pageSize: '100' });
      if (kindFilter) query.set('kind', kindFilter);
      if (statusFilter) query.set('status', statusFilter);
      const data = mode === 'client'
        ? await clientApi<PaginatedResponse<Wish>>(`/sources/${sourceID}/wishes?${query}`)
        : await api<PaginatedResponse<Wish>>(`/api/v1/wishes?${query}`);
      setItems(data.items || []);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.loadFailed')) });
    } finally {
      setLoading(false);
    }
  }, [kindFilter, mode, setToast, sourceID, statusFilter, t]);

  useEffect(() => { void load(); }, [load]);

  async function submitWish(event: FormEvent) {
    event.preventDefault();
    if (!sourceID) return;
    try {
      if (editingID) {
        await clientApi(`/sources/${sourceID}/wishes/${editingID}`, {
          method: 'PATCH',
          body: JSON.stringify({ title: draft.title, body: draft.body, referenceUrl: draft.referenceUrl, contactEmail: draft.contactEmail, contactOther: draft.contactOther }),
        });
        setToast({ tone: 'success', message: t('wishWall.updated') });
      } else {
        await clientApi(`/sources/${sourceID}/wishes`, { method: 'POST', body: JSON.stringify(draft) });
        setToast({ tone: 'success', message: t('wishWall.created') });
      }
      setDraft(emptyDraft());
      setEditingID(null);
      setShowCreate(false);
      await load();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.saveFailed')) });
    }
  }

  function editWish(item: Wish) {
    setEditingID(item.id);
    setDraft({ kind: item.kind, title: item.title, body: item.body, referenceUrl: item.referenceUrl || '', contactEmail: item.contactEmail || '', contactOther: item.contactOther || '', statusText: '' });
    setShowCreate(true);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  async function deleteWish(item: Wish) {
    if (!window.confirm(t('wishWall.deleteConfirm', { id: item.id }))) return;
    try {
      await clientApi(`/sources/${sourceID}/wishes/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'success', message: t('wishWall.deleted') });
      await load();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.deleteFailed')) });
    }
  }

  async function reply(item: Wish) {
    const body = (replyDrafts[item.id] || '').trim();
    if (!body) return;
    try {
      await api(`/api/v1/admin/wishes/${item.id}/replies`, { method: 'POST', body: JSON.stringify({ body }) });
      setReplyDrafts((current) => ({ ...current, [item.id]: '' }));
      await load();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.replyFailed')) });
    }
  }

  async function updateStatus(item: Wish) {
    const next = statusDrafts[item.id] || { status: item.status, text: '' };
    try {
      await api(`/api/v1/admin/wishes/${item.id}/status`, { method: 'POST', body: JSON.stringify({ status: next.status, statusText: next.text }) });
      setStatusDrafts((current) => ({ ...current, [item.id]: { status: next.status, text: '' } }));
      await load();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.statusFailed')) });
    }
  }

  async function blockAuthor(item: Wish) {
    if (!item.clientUserId) return;
    const reason = window.prompt(t('wishWall.blockReason'))?.trim();
    if (!reason) return;
    try {
      await api(`/api/v1/admin/downstream-clients/${encodeURIComponent(item.clientUserId)}/block`, { method: 'POST', body: JSON.stringify({ reason }) });
      setToast({ tone: 'success', message: t('wishWall.blocked') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.blockFailed')) });
    }
  }

  const canCreate = mode === 'client' && Boolean(sourceID);

  return (
    <section className="page-grid wish-wall-page" data-mode={mode}>
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('wishWall.eyebrow')}</span>
          <h1>{t('wishWall.title')}</h1>
          <p>{t(mode === 'client' ? 'wishWall.clientDescription' : 'wishWall.serverDescription')}</p>
        </div>
        <div className="row-actions">
          <XButton type="button" variant="secondary" label={t('common.refresh')} icon={<RefreshCw size={17} />} isLoading={loading} onClick={() => void load()} />
          {canCreate && <XButton type="button" variant="primary" label={t('wishWall.create')} icon={<Plus size={17} />} onClick={() => { setEditingID(null); setDraft(emptyDraft()); setShowCreate((value) => !value); }} />}
        </div>
      </div>

      {mode === 'client' && supportedSources.length > 0 && (
        <div className="wish-source-picker">
          <XSelector label={t('wishWall.targetSource')} value={sourceID} options={supportedSources.map((source) => ({ value: String(source.id), label: source.name }))} onChange={setSourceID} />
        </div>
      )}

      {mode === 'client' && supportedSources.length === 0 ? (
        <EmptyState icon={Lightbulb} title={t('wishWall.noSupportedSource')} body={t('wishWall.noSupportedSourceBody')} />
      ) : (
        <>
          {showCreate && canCreate && (
            <form className="panel wish-form" onSubmit={submitWish}>
              <h2>{t(editingID ? 'wishWall.edit' : 'wishWall.create')}</h2>
              {!editingID && (
                <XSelector label={t('wishWall.kindLabel')} value={draft.kind} options={kinds.map((kind) => ({ value: kind, label: t(`wishWall.kinds.${kind}`) }))} onChange={(value) => setDraft((current) => ({ ...current, kind: value as WishKind }))} />
              )}
              <label><span>{t('wishWall.titleLabel')}</span><input required maxLength={160} value={draft.title} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} /></label>
              <XTextArea label={t('wishWall.bodyLabel')} isRequired maxLength={5000} rows={5} value={draft.body} onChange={(body) => setDraft((current) => ({ ...current, body }))} />
              {draft.kind === 'APP_REQUEST' && <label><span>{t('wishWall.referenceUrl')}</span><input type="url" maxLength={500} value={draft.referenceUrl} onChange={(event) => setDraft((current) => ({ ...current, referenceUrl: event.target.value }))} /></label>}
              {draft.kind === 'CUSTOMIZATION' && <>
                <label><span>{t('wishWall.contactEmail')}</span><input required type="email" maxLength={254} value={draft.contactEmail} onChange={(event) => setDraft((current) => ({ ...current, contactEmail: event.target.value }))} /></label>
                <label><span>{t('wishWall.contactOther')}</span><input maxLength={500} value={draft.contactOther} onChange={(event) => setDraft((current) => ({ ...current, contactOther: event.target.value }))} /></label>
              </>}
              {!editingID && <XTextArea label={t('wishWall.initialStatusText')} isRequired maxLength={1000} rows={3} value={draft.statusText} onChange={(statusText) => setDraft((current) => ({ ...current, statusText }))} />}
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} onClick={() => { setShowCreate(false); setEditingID(null); }} />
                <XButton type="submit" variant="primary" label={t('common.save')} />
              </div>
            </form>
          )}

          <div className="wish-filter-shelf">
            <div className="wish-filter-label" aria-hidden="true"><SlidersHorizontal size={16} /></div>
            <div className="wish-filter-row">
              <XSelector label={t('wishWall.kindLabel')} value={kindFilter} options={[{ value: '', label: t('common.all') }, ...kinds.filter((kind) => mode === 'client' || isAdmin || kind !== 'SUGGESTION').map((kind) => ({ value: kind, label: t(`wishWall.kinds.${kind}`) }))]} onChange={setKindFilter} />
              <XSelector label={t('wishWall.statusLabel')} value={statusFilter} options={[{ value: '', label: t('common.all') }, ...statuses.map((status) => ({ value: status, label: t(`wishWall.statuses.${status}`) }))]} onChange={setStatusFilter} />
            </div>
            <output className="wish-filter-count" aria-live="polite" aria-label={`${t('wishWall.title')}: ${items.length}`}>{items.length}</output>
          </div>

          {items.length === 0 ? <EmptyState icon={Lightbulb} title={t('wishWall.empty')} body={t('wishWall.emptyBody')} /> : (
            <div className="wish-board" aria-busy={loading}>
              <div className="wish-board-seam" aria-hidden="true" />
              {items.map((item, index) => {
                const ownClientWish = mode === 'client' && Boolean(item.clientUserId);
                const statusDraft = statusDrafts[item.id] || { status: item.status, text: '' };
                return (
                  <article
                    className="wish-card"
                    data-kind={item.kind}
                    key={item.id}
                    style={{ '--wish-index': Math.min(index, 5) } as CSSProperties}
                  >
                    <div className="wish-fastener" aria-hidden="true"><span /></div>
                    <header>
                      <div className="wish-title-block"><span className="wish-kind">#{item.id} · {t(`wishWall.kinds.${item.kind}`)}</span><h2>{item.title}</h2></div>
                      <XBadge label={t(`wishWall.statuses.${item.status}`)} variant={item.status === 'COMPLETED' ? 'success' : item.status === 'REJECTED' ? 'error' : 'neutral'} />
                    </header>
                    <p className="wish-body">{item.body}</p>
                    {item.referenceUrl && <a href={item.referenceUrl} target="_blank" rel="noreferrer">{item.referenceUrl}</a>}
                    <div className="wish-card-footer">
                      <div className="wish-meta">{t('wishWall.by', { name: item.authorName })} · {formatDate(item.createdAt)}</div>
                      {ownClientWish && <div className="row-actions"><XButton type="button" size="sm" variant="secondary" label={t('common.edit')} icon={<Pencil size={15} />} onClick={() => editWish(item)} /><XButton type="button" size="sm" variant="destructive" label={t('common.delete')} icon={<Trash2 size={15} />} onClick={() => void deleteWish(item)} /></div>}
                    </div>

                    {((item.statusHistory || []).length > 0 || (item.replies || []).length > 0) && (
                      <XCollapsible
                        className="wish-disclosure"
                        defaultIsOpen={false}
                        trigger={<span className="wish-disclosure-trigger"><span>{t('wishWall.statusHistory')}</span><span className="wish-disclosure-count">{(item.statusHistory || []).length + (item.replies || []).length}</span></span>}
                      >
                        <div className="wish-disclosure-content">
                          {(item.statusHistory || []).length > 0 && <section className="wish-timeline">
                            <h3>{t('wishWall.statusHistory')}</h3>
                            {(item.statusHistory || []).map((event) => <div key={event.id}><XBadge label={t(`wishWall.statuses.${event.toStatus}`)} variant="neutral" /><span>{event.text}</span><small>{event.actorName} · {formatDate(event.createdAt)}</small></div>)}
                          </section>}
                          {(item.replies || []).length > 0 && <section className="wish-replies"><h3>{t('wishWall.replies')}</h3>{item.replies.map((reply) => <blockquote key={reply.id}><p>{reply.body}</p><footer>{reply.authorName} · {formatDate(reply.createdAt)}</footer></blockquote>)}</section>}
                        </div>
                      </XCollapsible>
                    )}

                    {mode === 'server' && isAdmin && <XCollapsible className="wish-disclosure wish-admin-disclosure" defaultIsOpen={false} trigger={t('wishWall.nextStatus')}>
                      <section className="wish-admin-actions">
                        {(item.contactEmail || item.contactOther) && <div className="wish-private"><strong>{t('wishWall.contact')}</strong><span>{[item.contactEmail, item.contactOther].filter(Boolean).join(' · ')}</span></div>}
                        {item.clientUserId && <code className="wish-client-id">{item.clientUserId}</code>}
                        <XTextArea label={t('wishWall.reply')} rows={2} maxLength={5000} value={replyDrafts[item.id] || ''} onChange={(value) => setReplyDrafts((current) => ({ ...current, [item.id]: value }))} />
                        <XButton type="button" size="sm" variant="secondary" label={t('wishWall.sendReply')} icon={<MessageSquareReply size={15} />} onClick={() => void reply(item)} />
                        <XSelector label={t('wishWall.nextStatus')} value={statusDraft.status} options={statuses.map((status) => ({ value: status, label: t(`wishWall.statuses.${status}`) }))} onChange={(value) => setStatusDrafts((current) => ({ ...current, [item.id]: { ...statusDraft, status: value as WishStatus } }))} />
                        <XTextArea label={t('wishWall.statusText')} isRequired rows={2} maxLength={1000} value={statusDraft.text} onChange={(value) => setStatusDrafts((current) => ({ ...current, [item.id]: { ...statusDraft, text: value } }))} />
                        <div className="row-actions"><XButton type="button" size="sm" variant="primary" label={t('wishWall.updateStatus')} onClick={() => void updateStatus(item)} />{user?.role === 'SITE_ADMIN' && item.clientUserId && <XButton type="button" size="sm" variant="destructive" label={t('wishWall.blockAuthor')} icon={<Ban size={15} />} onClick={() => void blockAuthor(item)} />}</div>
                      </section>
                    </XCollapsible>}
                  </article>
                );
              })}
            </div>
          )}
        </>
      )}
    </section>
  );
}
