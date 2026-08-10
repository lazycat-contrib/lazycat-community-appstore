import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent } from 'react';
import {
  Ban,
  CalendarClock,
  CheckCircle2,
  CircleDot,
  Lightbulb,
  ListFilter,
  LoaderCircle,
  History,
  MessageCircle,
  MessageSquareReply,
  PackagePlus,
  Pencil,
  Plus,
  RefreshCw,
  Settings2,
  Shapes,
  Trash2,
  Wrench,
  X,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { useTranslation } from 'react-i18next';
import { api, clientApi } from '../../shared/api';
import type { PaginatedResponse, SourceSubscription, Toast, User, Wish, WishKind, WishStatus } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';
import { EmptyState } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { hasNextWishPage, mergeWishPage } from './wishWallState';

const kinds: WishKind[] = ['SUGGESTION', 'APP_REQUEST', 'CUSTOMIZATION'];
const statuses: WishStatus[] = ['OPEN', 'PLANNED', 'IN_PROGRESS', 'COMPLETED', 'REJECTED'];
const PAGE_SIZE = 24;

const kindIcons: Record<WishKind, LucideIcon> = {
  SUGGESTION: MessageCircle,
  APP_REQUEST: PackagePlus,
  CUSTOMIZATION: Wrench,
};

const statusIcons: Record<WishStatus, LucideIcon> = {
  OPEN: CircleDot,
  PLANNED: CalendarClock,
  IN_PROGRESS: LoaderCircle,
  COMPLETED: CheckCircle2,
  REJECTED: XCircle,
};

type WishDraft = {
  kind: WishKind;
  title: string;
  body: string;
  referenceUrl: string;
  contactEmail: string;
  contactOther: string;
  statusText: string;
};

type WishDialogView = 'activity' | 'manage';

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
  const [loadingMore, setLoadingMore] = useState(false);
  const [page, setPage] = useState(1);
  const [totalItems, setTotalItems] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [kindFilter, setKindFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [draft, setDraft] = useState<WishDraft>(emptyDraft);
  const [editingID, setEditingID] = useState<number | null>(null);
  const [replyDrafts, setReplyDrafts] = useState<Record<number, string>>({});
  const [statusDrafts, setStatusDrafts] = useState<Record<number, { status: WishStatus; text: string }>>({});
  const [wishDialog, setWishDialog] = useState<{ itemID: number; view: WishDialogView } | null>(null);
  const loadEpoch = useRef(0);
  const loadingMoreRef = useRef(false);
  const loadMoreSentinel = useRef<HTMLDivElement | null>(null);
  const isAdmin = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';

  useEffect(() => {
    if (mode !== 'client') return;
    if (supportedSources.some((source) => String(source.id) === sourceID)) return;
    setSourceID(supportedSources[0] ? String(supportedSources[0].id) : '');
  }, [mode, sourceID, supportedSources]);

  const loadPage = useCallback(async (nextPage: number, append: boolean, epoch: number) => {
    if (mode === 'client' && !sourceID) {
      setItems([]);
      setLoading(false);
      setLoadingMore(false);
      loadingMoreRef.current = false;
      setPage(1);
      setTotalItems(0);
      setTotalPages(1);
      return;
    }
    if (append) {
      if (loadingMoreRef.current) return;
      loadingMoreRef.current = true;
      setLoadingMore(true);
    } else {
      setLoading(true);
    }
    try {
      const query = new URLSearchParams({ page: String(nextPage), pageSize: String(PAGE_SIZE) });
      if (kindFilter) query.set('kind', kindFilter);
      if (statusFilter) query.set('status', statusFilter);
      const data = mode === 'client'
        ? await clientApi<PaginatedResponse<Wish>>(`/sources/${sourceID}/wishes?${query}`)
        : await api<PaginatedResponse<Wish>>(`/api/v1/wishes?${query}`);
      if (epoch !== loadEpoch.current) return;
      setItems((current) => mergeWishPage(current, data.items || [], append));
      setPage(data.pagination?.page || nextPage);
      setTotalItems(data.pagination?.totalItems || 0);
      setTotalPages(data.pagination?.totalPages || 1);
    } catch (error) {
      if (epoch !== loadEpoch.current) return;
      setToast({ tone: 'error', message: errorMessage(error, t('wishWall.loadFailed')) });
    } finally {
      if (append) {
        loadingMoreRef.current = false;
        if (epoch === loadEpoch.current) setLoadingMore(false);
      } else if (epoch === loadEpoch.current) {
        setLoading(false);
      }
    }
  }, [kindFilter, mode, setToast, sourceID, statusFilter, t]);

  const load = useCallback(() => {
    const epoch = loadEpoch.current + 1;
    loadEpoch.current = epoch;
    loadingMoreRef.current = false;
    setLoadingMore(false);
    return loadPage(1, false, epoch);
  }, [loadPage]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    const sentinel = loadMoreSentinel.current;
    if (!sentinel || !hasNextWishPage(page, totalPages) || loading || loadingMore) return;
    const observer = new IntersectionObserver((entries) => {
      if (!entries[0]?.isIntersecting || loadingMoreRef.current) return;
      void loadPage(page + 1, true, loadEpoch.current);
    }, { rootMargin: '320px 0px' });
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [loadPage, loading, loadingMore, page, totalPages]);

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
  const clientEditing = mode === 'client' && editingID !== null;
  const visibleKinds = kinds.filter((kind) => mode === 'client' || isAdmin || kind !== 'SUGGESTION');

  function FilterButton({
    active,
    icon: Icon,
    label,
    tone,
    onClick,
  }: {
    active: boolean;
    icon: LucideIcon;
    label: string;
    tone: string;
    onClick: () => void;
  }) {
    return (
      <button
        type="button"
        className="wish-filter-button"
        data-active={active}
        data-tone={tone}
        aria-pressed={active}
        aria-label={label}
        title={label}
        onClick={onClick}
      >
        <Icon size={17} aria-hidden="true" />
        <span>{label}</span>
      </button>
    );
  }

  const dialogItem = wishDialog ? items.find((item) => item.id === wishDialog.itemID) : undefined;
  const dialogStatusDraft = dialogItem
    ? statusDrafts[dialogItem.id] || { status: dialogItem.status, text: '' }
    : undefined;

  return (
    <section className="page-grid wish-wall-page" data-mode={mode}>
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('wishWall.eyebrow')}</span>
          <h1>{t('wishWall.title')}</h1>
          <p>{t(mode === 'client' ? 'wishWall.clientDescription' : 'wishWall.serverDescription')}</p>
        </div>
        {!clientEditing && <div className="row-actions">
          <XButton type="button" variant="secondary" label={t('common.refresh')} icon={<RefreshCw size={17} />} isLoading={loading} onClick={() => void load()} />
          {canCreate && <XButton type="button" variant="primary" label={t('wishWall.create')} icon={<Plus size={17} />} onClick={() => { setEditingID(null); setDraft(emptyDraft()); setShowCreate((value) => !value); }} />}
        </div>}
      </div>

      {mode === 'client' && supportedSources.length > 0 && !clientEditing && (
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

          {!clientEditing && <>
          <div className="wish-filter-shelf" role="group" aria-label={t('wishWall.filters')}>
            <div className="wish-filter-group">
              <span className="wish-filter-group-label"><Shapes size={15} aria-hidden="true" />{t('wishWall.kindLabel')}</span>
              <div className="wish-filter-row">
                <FilterButton active={!kindFilter} icon={ListFilter} label={t('common.all')} tone="all" onClick={() => setKindFilter('')} />
                {visibleKinds.map((kind) => <FilterButton key={kind} active={kindFilter === kind} icon={kindIcons[kind]} label={t(`wishWall.kinds.${kind}`)} tone={kind} onClick={() => setKindFilter(kind)} />)}
              </div>
            </div>
            <div className="wish-filter-group">
              <span className="wish-filter-group-label"><CircleDot size={15} aria-hidden="true" />{t('wishWall.statusLabel')}</span>
              <div className="wish-filter-row">
                <FilterButton active={!statusFilter} icon={ListFilter} label={t('common.all')} tone="all" onClick={() => setStatusFilter('')} />
                {statuses.map((status) => <FilterButton key={status} active={statusFilter === status} icon={statusIcons[status]} label={t(`wishWall.statuses.${status}`)} tone={status} onClick={() => setStatusFilter(status)} />)}
              </div>
            </div>
            <output className="wish-filter-count" aria-live="polite" aria-label={t('wishWall.totalCount', { count: totalItems })}>{totalItems}</output>
          </div>

          {items.length === 0 ? <EmptyState icon={Lightbulb} title={t('wishWall.empty')} body={t('wishWall.emptyBody')} /> : (
            <div className={mode === 'client' ? 'wish-maintenance-list' : 'wish-board'} aria-busy={loading || loadingMore}>
              {mode === 'server' && <div className="wish-board-seam" aria-hidden="true" />}
              {items.map((item, index) => {
                const ownClientWish = mode === 'client' && item.canManage === true;
                const KindIcon = kindIcons[item.kind];
                const StatusIcon = statusIcons[item.status];
                return (
                  <article
                    className="wish-card"
                    data-kind={item.kind}
                    key={item.id}
                    style={{ '--wish-index': Math.min(index, 5) } as CSSProperties}
                  >
                    {mode === 'server' && <div className="wish-fastener" aria-hidden="true"><span /></div>}
                    {mode === 'client' && <div className="wish-list-kind" aria-hidden="true"><KindIcon size={18} /></div>}
                    <header>
                      <div className="wish-title-block"><span className="wish-kind"><KindIcon size={13} aria-hidden="true" />#{item.id} · {t(`wishWall.kinds.${item.kind}`)}</span><h2>{item.title}</h2></div>
                      <span className="wish-status" data-status={item.status}><StatusIcon size={14} aria-hidden="true" />{t(`wishWall.statuses.${item.status}`)}</span>
                    </header>
                    <p className="wish-body">{item.body}</p>
                    {item.referenceUrl && <a href={item.referenceUrl} target="_blank" rel="noreferrer">{item.referenceUrl}</a>}
                    <div className="wish-card-footer">
                      <div className="wish-meta">{t('wishWall.by', { name: item.authorName })} · {formatDate(item.createdAt)}</div>
                      {ownClientWish && <div className="row-actions"><XButton type="button" size="sm" variant="secondary" label={t('common.edit')} icon={<Pencil size={15} />} onClick={() => editWish(item)} /><XButton type="button" size="sm" variant="destructive" label={t('common.delete')} icon={<Trash2 size={15} />} onClick={() => void deleteWish(item)} /></div>}
                    </div>

                    {mode === 'server' && <div className="wish-card-fab" role="group" aria-label={t('wishWall.cardActions')}>
                      <XIconButton type="button" size="sm" variant="secondary" label={t('wishWall.openActivity')} tooltip={t('wishWall.openActivity')} icon={<History size={16} />} onClick={() => setWishDialog({ itemID: item.id, view: 'activity' })} />
                      {isAdmin && <XIconButton type="button" size="sm" variant="primary" label={t('wishWall.openManage')} tooltip={t('wishWall.openManage')} icon={<Settings2 size={16} />} onClick={() => setWishDialog({ itemID: item.id, view: 'manage' })} />}
                    </div>}
                  </article>
                );
              })}
            </div>
          )}
          <div className="wish-load-more" ref={loadMoreSentinel} aria-live="polite">
            {loadingMore && <><LoaderCircle className="wish-load-spinner" size={18} aria-hidden="true" /><span>{t('wishWall.loadingMore')}</span></>}
            {!loading && !loadingMore && items.length > 0 && page >= totalPages && <span>{t('wishWall.allLoaded')}</span>}
          </div>
          </>}
        </>
      )}

      {wishDialog && dialogItem && (
        <ModalLayer
          onClose={() => setWishDialog(null)}
          purpose={wishDialog.view === 'manage' ? 'form' : 'info'}
          width="min(680px, calc(100vw - 32px))"
          maxHeight="min(88vh, 860px)"
        >
          <section className="modal-panel wish-dialog" aria-labelledby="wish-dialog-title">
            <XIconButton className="close" type="button" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setWishDialog(null)} />
            <header className="wish-dialog-header">
              <span className="eyebrow subtle">#{dialogItem.id} · {t(`wishWall.kinds.${dialogItem.kind}`)}</span>
              <h2 id="wish-dialog-title">{wishDialog.view === 'manage' ? t('wishWall.manageTitle') : t('wishWall.activityTitle')}</h2>
              <p>{dialogItem.title}</p>
            </header>

            {wishDialog.view === 'activity' && (
              <div className="wish-dialog-content">
                {(dialogItem.statusHistory || []).length > 0 && <section className="wish-timeline">
                  <h3>{t('wishWall.statusHistory')}</h3>
                  {dialogItem.statusHistory.map((event) => <div key={event.id}><XBadge label={t(`wishWall.statuses.${event.toStatus}`)} variant="neutral" /><span>{event.text}</span><small>{event.actorName} · {formatDate(event.createdAt)}</small></div>)}
                </section>}
                {(dialogItem.replies || []).length > 0 && <section className="wish-replies"><h3>{t('wishWall.replies')}</h3>{dialogItem.replies.map((reply) => <blockquote key={reply.id}><p>{reply.body}</p><footer>{reply.authorName} · {formatDate(reply.createdAt)}</footer></blockquote>)}</section>}
                {(dialogItem.statusHistory || []).length === 0 && (dialogItem.replies || []).length === 0 && <p className="wish-dialog-empty">{t('wishWall.noActivity')}</p>}
              </div>
            )}

            {wishDialog.view === 'manage' && isAdmin && dialogStatusDraft && (
              <section className="wish-admin-actions">
                {(dialogItem.contactEmail || dialogItem.contactOther) && <div className="wish-private"><strong>{t('wishWall.contact')}</strong><span>{[dialogItem.contactEmail, dialogItem.contactOther].filter(Boolean).join(' · ')}</span></div>}
                {dialogItem.clientUserId && <code className="wish-client-id">{dialogItem.clientUserId}</code>}
                <XTextArea label={t('wishWall.reply')} rows={3} maxLength={5000} value={replyDrafts[dialogItem.id] || ''} onChange={(value) => setReplyDrafts((current) => ({ ...current, [dialogItem.id]: value }))} />
                <XButton type="button" size="sm" variant="secondary" label={t('wishWall.sendReply')} icon={<MessageSquareReply size={15} />} onClick={() => void reply(dialogItem)} />
                <XSelector label={t('wishWall.nextStatus')} value={dialogStatusDraft.status} options={statuses.map((status) => ({ value: status, label: t(`wishWall.statuses.${status}`) }))} onChange={(value) => setStatusDrafts((current) => ({ ...current, [dialogItem.id]: { ...dialogStatusDraft, status: value as WishStatus } }))} />
                <XTextArea label={t('wishWall.statusText')} isRequired rows={3} maxLength={1000} value={dialogStatusDraft.text} onChange={(value) => setStatusDrafts((current) => ({ ...current, [dialogItem.id]: { ...dialogStatusDraft, text: value } }))} />
                <div className="row-actions"><XButton type="button" size="sm" variant="primary" label={t('wishWall.updateStatus')} onClick={() => void updateStatus(dialogItem)} />{user?.role === 'SITE_ADMIN' && dialogItem.clientUserId && <XButton type="button" size="sm" variant="destructive" label={t('wishWall.blockAuthor')} icon={<Ban size={15} />} onClick={() => void blockAuthor(dialogItem)} />}</div>
              </section>
            )}
          </section>
        </ModalLayer>
      )}
    </section>
  );
}
