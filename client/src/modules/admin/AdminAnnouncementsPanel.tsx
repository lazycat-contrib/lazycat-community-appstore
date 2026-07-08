import { type FormEvent, useState } from 'react';
import { Bell, CalendarClock, MessageSquare, Pencil, Plus, Trash2, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { SiteAnnouncement, SiteProfile, Toast } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';

type AnnouncementDraft = {
  enabled: boolean;
  level: SiteAnnouncement['level'];
  title: string;
  body: string;
  linkLabel: string;
  linkUrl: string;
  startsAt: string;
  endsAt: string;
  sortOrder: string;
};

function emptyDraft(): AnnouncementDraft {
  return {
    enabled: true,
    level: 'info',
    title: '',
    body: '',
    linkLabel: '',
    linkUrl: '',
    startsAt: '',
    endsAt: '',
    sortOrder: '0',
  };
}

function dateTimeLocal(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offset = date.getTimezoneOffset() * 60000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function draftFromAnnouncement(item: SiteAnnouncement): AnnouncementDraft {
  return {
    enabled: item.enabled,
    level: item.level || 'info',
    title: item.title || '',
    body: item.body || '',
    linkLabel: item.linkLabel || '',
    linkUrl: item.linkUrl || '',
    startsAt: dateTimeLocal(item.startsAt),
    endsAt: dateTimeLocal(item.endsAt),
    sortOrder: String(item.sortOrder || 0),
  };
}

function isoFromDateTimeLocal(value: string) {
  if (!value.trim()) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toISOString();
}

function payloadFromDraft(draft: AnnouncementDraft) {
  const sortOrder = Number.parseInt(draft.sortOrder || '0', 10);
  return {
    enabled: draft.enabled,
    level: draft.level,
    title: draft.title.trim(),
    body: draft.body.trim(),
    linkLabel: draft.linkLabel.trim(),
    linkUrl: draft.linkUrl.trim(),
    startsAt: isoFromDateTimeLocal(draft.startsAt),
    endsAt: isoFromDateTimeLocal(draft.endsAt),
    sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
  };
}

function AnnouncementDateTimeField({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="datetime-field">
      <label htmlFor={id}>
        <CalendarClock size={16} aria-hidden="true" />
        <span>{label}</span>
      </label>
      <input id={id} type="datetime-local" aria-label={label} value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function scheduleState(item: SiteAnnouncement) {
  const now = Date.now();
  if (item.startsAt && new Date(item.startsAt).getTime() > now) return 'scheduled';
  if (item.endsAt && new Date(item.endsAt).getTime() <= now) return 'expired';
  return item.enabled ? 'active' : 'disabled';
}

export function AdminAnnouncementsPanel({
  announcements,
  onReload,
  onSiteProfileSaved,
  setToast,
}: {
  announcements: SiteAnnouncement[];
  onReload: () => Promise<void>;
  onSiteProfileSaved: (site?: SiteProfile) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<SiteAnnouncement | null>(null);
  const [draft, setDraft] = useState<AnnouncementDraft>(emptyDraft);
  const [deleteTarget, setDeleteTarget] = useState<SiteAnnouncement | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  const levelOptions = [
    { value: 'info', label: t('site.announcementLevels.info') },
    { value: 'warning', label: t('site.announcementLevels.warning') },
    { value: 'success', label: t('site.announcementLevels.success') },
  ];

  function openCreate() {
    setEditing({ enabled: true, level: 'info' });
    setDraft(emptyDraft());
  }

  function openEdit(item: SiteAnnouncement) {
    setEditing(item);
    setDraft(draftFromAnnouncement(item));
  }

  async function saveAnnouncement(event: FormEvent) {
    event.preventDefault();
    setIsSaving(true);
    try {
      const payload = payloadFromDraft(draft);
      const data = editing?.id
        ? await api<{ announcement: SiteAnnouncement; site?: SiteProfile }>(`/api/v1/admin/announcements/${editing.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        })
        : await api<{ announcement: SiteAnnouncement; site?: SiteProfile }>('/api/v1/admin/announcements', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
      setToast({ tone: 'success', message: editing?.id ? t('admin.announcementUpdated') : t('admin.announcementCreated') });
      setEditing(null);
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.announcementSaveFailed')) });
    } finally {
      setIsSaving(false);
    }
  }

  async function toggleAnnouncement(item: SiteAnnouncement) {
    try {
      const data = await api<{ announcement: SiteAnnouncement; site?: SiteProfile }>(`/api/v1/admin/announcements/${item.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled: !item.enabled }),
      });
      setToast({ tone: 'success', message: item.enabled ? t('admin.announcementDisabledToast') : t('admin.announcementEnabledToast') });
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.announcementSaveFailed')) });
    }
  }

  async function deleteAnnouncement() {
    if (!deleteTarget?.id) return;
    try {
      const data = await api<{ site?: SiteProfile }>(`/api/v1/admin/announcements/${deleteTarget.id}`, { method: 'DELETE' });
      setDeleteTarget(null);
      setToast({ tone: 'neutral', message: t('admin.announcementDeleted') });
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.announcementDeleteFailed')) });
    }
  }

  return (
    <div className="settings-section announcement-manager">
      <div className="settings-section-head with-action">
        <div>
          <strong>{t('admin.announcementCenter')}</strong>
          <span>{t('admin.announcementCenterBody')}</span>
        </div>
        <XButton type="button" variant="primary" size="sm" label={t('admin.createAnnouncement')} icon={<Plus size={17} />} onClick={openCreate} />
      </div>

      {announcements.length === 0 ? (
        <EmptyState icon={MessageSquare} title={t('admin.noAnnouncements')} body={t('admin.noAnnouncementsBody')} action={{ label: t('admin.createAnnouncement'), icon: Plus, onClick: openCreate }} />
      ) : (
        <XList className="action-list announcement-list" density="compact" hasDividers>
          {announcements.map((item) => {
            const state = scheduleState(item);
            return (
              <XListItem
                key={item.id || `${item.level}:${item.title}:${item.updatedAt}`}
                label={item.title || item.body || t('admin.announcementUntitled')}
                description={(
                  <span className="action-list-description">
                    <span>{item.body || t('admin.announcementNoBody')}</span>
                    <small>
                      {item.startsAt ? t('admin.announcementStartsAt', { time: formatDate(item.startsAt) }) : t('admin.announcementNoStart')}
                      {' · '}
                      {item.endsAt ? t('admin.announcementEndsAt', { time: formatDate(item.endsAt) }) : t('admin.announcementNoEnd')}
                    </small>
                  </span>
                )}
                startContent={<CalendarClock size={17} />}
                endContent={(
                  <div className="row-actions">
                    <StatusBadge tone={item.level === 'warning' ? 'warning' : item.level === 'success' ? 'success' : 'info'} label={t(`site.announcementLevels.${item.level || 'info'}`)} />
                    <XBadge label={t(`admin.announcementStates.${state}`)} variant={state === 'active' ? 'success' : state === 'scheduled' ? 'info' : state === 'expired' ? 'warning' : 'neutral'} />
                    <XButton type="button" variant="secondary" size="sm" label={item.enabled ? t('admin.disableAnnouncement') : t('admin.enableAnnouncement')} onClick={() => void toggleAnnouncement(item)} />
                    <XIconButton type="button" variant="ghost" size="sm" label={t('admin.editAnnouncement')} icon={<Pencil size={16} />} onClick={() => openEdit(item)} />
                    <XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteAnnouncement')} icon={<Trash2 size={16} />} onClick={() => setDeleteTarget(item)} />
                  </div>
                )}
              />
            );
          })}
        </XList>
      )}

      {editing && (
        <ModalLayer onClose={() => setEditing(null)} purpose="form" width="min(720px, calc(100vw - 36px))">
          <form className="modal-panel form-panel announcement-dialog" aria-label={editing.id ? t('admin.editAnnouncement') : t('admin.createAnnouncement')} onSubmit={saveAnnouncement}>
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setEditing(null)} />
            <SectionTitle icon={editing.level === 'warning' ? Bell : MessageSquare} title={editing.id ? t('admin.editAnnouncement') : t('admin.createAnnouncement')} />
            <XFormLayout>
              <XSwitch
                label={t('admin.settings.announcementEnabled')}
                value={draft.enabled}
                labelSpacing="spread"
                width="100%"
                onChange={(checked) => setDraft((current) => ({ ...current, enabled: checked }))}
              />
              <XSelector
                label={t('admin.settings.announcementLevel')}
                description={t('admin.settingsHelp.announcementLevel')}
                value={draft.level}
                options={levelOptions}
                onChange={(value) => setDraft((current) => ({ ...current, level: value as SiteAnnouncement['level'] }))}
              />
              <XTextInput label={t('admin.settings.announcementTitle')} value={draft.title} onChange={(title) => setDraft((current) => ({ ...current, title }))} />
              <XTextArea label={t('admin.settings.announcementBody')} value={draft.body} rows={4} onChange={(body) => setDraft((current) => ({ ...current, body }))} />
              <XTextInput label={t('admin.settings.announcementLinkLabel')} value={draft.linkLabel} onChange={(linkLabel) => setDraft((current) => ({ ...current, linkLabel }))} />
              <XTextInput label={t('admin.settings.announcementLinkURL')} value={draft.linkUrl} onChange={(linkUrl) => setDraft((current) => ({ ...current, linkUrl }))} />
              <AnnouncementDateTimeField id="announcement-starts-at" label={t('admin.announcementStartTime')} value={draft.startsAt} onChange={(startsAt) => setDraft((current) => ({ ...current, startsAt }))} />
              <AnnouncementDateTimeField id="announcement-ends-at" label={t('admin.announcementEndTime')} value={draft.endsAt} onChange={(endsAt) => setDraft((current) => ({ ...current, endsAt }))} />
              <XTextInput label={t('admin.categorySortOrder')} value={draft.sortOrder} onChange={(sortOrder) => setDraft((current) => ({ ...current, sortOrder }))} />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setEditing(null)} />
              <XButton type="submit" variant="primary" label={editing.id ? t('admin.saveAnnouncement') : t('admin.createAnnouncement')} icon={<Plus size={17} />} isDisabled={isSaving} isLoading={isSaving} />
            </div>
          </form>
        </ModalLayer>
      )}

      {deleteTarget && (
        <ModalLayer onClose={() => setDeleteTarget(null)} purpose="required">
          <div className="modal-panel form-panel confirm-dialog">
            <SectionTitle icon={Trash2} title={t('admin.deleteAnnouncement')} />
            <p>{t('admin.confirmDeleteAnnouncement', { title: deleteTarget.title || deleteTarget.body || t('admin.announcementUntitled') })}</p>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setDeleteTarget(null)} />
              <XButton type="button" variant="destructive" label={t('admin.deleteAnnouncement')} icon={<Trash2 size={17} />} onClick={() => void deleteAnnouncement()} />
            </div>
          </div>
        </ModalLayer>
      )}
    </div>
  );
}
