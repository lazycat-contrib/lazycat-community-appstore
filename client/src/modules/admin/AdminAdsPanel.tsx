import { type FormEvent, useState } from 'react';
import { Image as ImageIcon, Megaphone, Pencil, Plus, Save, Trash2, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { DateTimeInput as XDateTimeInput, type ISODateTimeString } from '@astryxdesign/core/DateTimeInput';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import type { SiteAd, SiteProfile, Toast } from '../../shared/types';
import { errorMessage, formatDate } from '../../shared/utils';

type AdDraft = {
  enabled: boolean;
  title: string;
  body: string;
  imageUrl: string;
  linkLabel: string;
  linkUrl: string;
  startsAt: string;
  endsAt: string;
  sortOrder: string;
};

function emptyDraft(): AdDraft {
  return {
    enabled: true,
    title: '',
    body: '',
    imageUrl: '',
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

function draftFromAd(item: SiteAd): AdDraft {
  return {
    enabled: item.enabled,
    title: item.title || '',
    body: item.body || '',
    imageUrl: item.imageUrl || '',
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

function payloadFromDraft(draft: AdDraft) {
  const sortOrder = Number.parseInt(draft.sortOrder || '0', 10);
  return {
    enabled: draft.enabled,
    title: draft.title.trim(),
    body: draft.body.trim(),
    imageUrl: draft.imageUrl.trim(),
    linkLabel: draft.linkLabel.trim(),
    linkUrl: draft.linkUrl.trim(),
    startsAt: isoFromDateTimeLocal(draft.startsAt),
    endsAt: isoFromDateTimeLocal(draft.endsAt),
    sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
  };
}

function scheduleState(item: SiteAd) {
  const now = Date.now();
  if (!item.enabled) return 'disabled';
  if (item.startsAt && new Date(item.startsAt).getTime() > now) return 'scheduled';
  if (item.endsAt && new Date(item.endsAt).getTime() <= now) return 'expired';
  return 'active';
}

export function AdminAdsPanel({
  ads,
  onReload,
  onSiteProfileSaved,
  setToast,
}: {
  ads: SiteAd[];
  onReload: () => Promise<void>;
  onSiteProfileSaved: (site?: SiteProfile) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState<SiteAd | null>(null);
  const [draft, setDraft] = useState<AdDraft>(emptyDraft);
  const [deleteTarget, setDeleteTarget] = useState<SiteAd | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [busyAdID, setBusyAdID] = useState<number | null>(null);

  function openCreate() {
    setEditing({ enabled: true });
    setDraft(emptyDraft());
  }

  function openEdit(item: SiteAd) {
    setEditing(item);
    setDraft(draftFromAd(item));
  }

  async function saveAd(event: FormEvent) {
    event.preventDefault();
    if (isSaving) return;
    setIsSaving(true);
    try {
      const payload = payloadFromDraft(draft);
      const data = editing?.id
        ? await api<{ ad: SiteAd; site?: SiteProfile }>(`/api/v1/admin/ads/${editing.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        })
        : await api<{ ad: SiteAd; site?: SiteProfile }>('/api/v1/admin/ads', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
      setToast({ tone: 'success', message: editing?.id ? t('admin.adUpdated') : t('admin.adCreated') });
      setEditing(null);
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.adSaveFailed')) });
    } finally {
      setIsSaving(false);
    }
  }

  async function toggleAd(item: SiteAd) {
    if (!item.id || busyAdID === item.id) return;
    setBusyAdID(item.id);
    try {
      const data = await api<{ ad: SiteAd; site?: SiteProfile }>(`/api/v1/admin/ads/${item.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled: !item.enabled }),
      });
      setToast({ tone: 'success', message: item.enabled ? t('admin.adDisabledToast') : t('admin.adEnabledToast') });
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.adSaveFailed')) });
    } finally {
      setBusyAdID(null);
    }
  }

  async function deleteAd() {
    if (!deleteTarget?.id) return;
    setBusyAdID(deleteTarget.id);
    try {
      const data = await api<{ site?: SiteProfile }>(`/api/v1/admin/ads/${deleteTarget.id}`, { method: 'DELETE' });
      setDeleteTarget(null);
      setToast({ tone: 'neutral', message: t('admin.adDeleted') });
      await onReload();
      await onSiteProfileSaved(data.site);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.adDeleteFailed')) });
    } finally {
      setBusyAdID(null);
    }
  }

  return (
    <div className="settings-section ad-manager">
      <div className="settings-section-head with-action">
        <div>
          <strong>{t('admin.adCenter')}</strong>
          <span>{t('admin.adCenterBody')}</span>
        </div>
        <XButton type="button" variant="primary" size="sm" label={t('admin.createAd')} icon={<Plus size={17} />} onClick={openCreate} />
      </div>

      {ads.length === 0 ? (
        <EmptyState icon={Megaphone} title={t('admin.noAds')} body={t('admin.noAdsBody')} action={{ label: t('admin.createAd'), icon: Plus, onClick: openCreate }} />
      ) : (
        <XList className="action-list ad-list" density="compact" hasDividers>
          {ads.map((item) => {
            const state = scheduleState(item);
            return (
              <XListItem
                key={item.id || `${item.title}:${item.updatedAt}`}
                label={item.title || item.body || t('admin.adUntitled')}
                description={(
                  <span className="action-list-description">
                    <span>{item.body || item.imageUrl || t('admin.adNoBody')}</span>
                    <small>
                      {item.startsAt ? t('admin.adStartsAt', { time: formatDate(item.startsAt) }) : t('admin.adNoStart')}
                      {' · '}
                      {item.endsAt ? t('admin.adEndsAt', { time: formatDate(item.endsAt) }) : t('admin.adNoEnd')}
                    </small>
                  </span>
                )}
                startContent={item.imageUrl ? <img className="ad-list-thumb" src={item.imageUrl} alt="" /> : <span className="ad-list-thumb ad-list-thumb-empty"><ImageIcon size={17} /></span>}
                endContent={(
                  <div className="row-actions">
                    <XBadge label={t(`admin.adStates.${state}`)} variant={state === 'active' ? 'success' : state === 'scheduled' ? 'info' : state === 'expired' ? 'warning' : 'neutral'} />
                    <XButton type="button" variant="secondary" size="sm" label={item.enabled ? t('admin.disableAd') : t('admin.enableAd')} isDisabled={busyAdID === item.id} isLoading={busyAdID === item.id} onClick={() => void toggleAd(item)} />
                    <XIconButton type="button" variant="ghost" size="sm" label={t('admin.editAd')} icon={<Pencil size={16} />} isDisabled={busyAdID === item.id} onClick={() => openEdit(item)} />
                    <XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteAd')} icon={<Trash2 size={16} />} isDisabled={busyAdID === item.id} onClick={() => setDeleteTarget(item)} />
                  </div>
                )}
              />
            );
          })}
        </XList>
      )}

      {editing && (
        <ModalLayer onClose={() => setEditing(null)} purpose="form" width="min(720px, calc(100vw - 36px))">
          <form className="modal-panel form-panel ad-dialog" aria-label={editing.id ? t('admin.editAd') : t('admin.createAd')} onSubmit={saveAd}>
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setEditing(null)} />
            <SectionTitle icon={Megaphone} title={editing.id ? t('admin.editAd') : t('admin.createAd')} />
            <XFormLayout>
              <XSwitch
                label={t('admin.settings.adEnabled')}
                value={draft.enabled}
                labelSpacing="spread"
                width="100%"
                onChange={(checked) => setDraft((current) => ({ ...current, enabled: checked }))}
              />
              <XTextInput label={t('admin.settings.adTitle')} description={t('admin.settingsHelp.adTitle')} value={draft.title} onChange={(title) => setDraft((current) => ({ ...current, title }))} />
              <XTextArea label={t('admin.settings.adBody')} description={t('admin.settingsHelp.adBody')} value={draft.body} rows={3} onChange={(body) => setDraft((current) => ({ ...current, body }))} />
              <XTextInput label={t('admin.settings.adImageURL')} description={t('admin.settingsHelp.adImageURL')} value={draft.imageUrl} onChange={(imageUrl) => setDraft((current) => ({ ...current, imageUrl }))} />
              <XTextInput label={t('admin.settings.adLinkLabel')} description={t('admin.settingsHelp.adLinkLabel')} value={draft.linkLabel} onChange={(linkLabel) => setDraft((current) => ({ ...current, linkLabel }))} />
              <XTextInput label={t('admin.settings.adLinkURL')} description={t('admin.settingsHelp.adLinkURL')} value={draft.linkUrl} onChange={(linkUrl) => setDraft((current) => ({ ...current, linkUrl }))} />
              <XDateTimeInput
                label={t('admin.adStartTime')}
                description={t('admin.settingsHelp.adStartTime')}
                value={draft.startsAt ? draft.startsAt as ISODateTimeString : undefined}
                onChange={(startsAt) => setDraft((current) => ({ ...current, startsAt: startsAt || '' }))}
                hasClear
                hourFormat="24h"
              />
              <XDateTimeInput
                label={t('admin.adEndTime')}
                description={t('admin.settingsHelp.adEndTime')}
                value={draft.endsAt ? draft.endsAt as ISODateTimeString : undefined}
                onChange={(endsAt) => setDraft((current) => ({ ...current, endsAt: endsAt || '' }))}
                hasClear
                hourFormat="24h"
              />
              <XTextInput label={t('admin.categorySortOrder')} value={draft.sortOrder} onChange={(sortOrder) => setDraft((current) => ({ ...current, sortOrder }))} />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setEditing(null)} />
              <XButton type="submit" variant="primary" label={editing.id ? t('admin.saveAd') : t('admin.createAd')} icon={editing.id ? <Save size={17} /> : <Plus size={17} />} isDisabled={isSaving} isLoading={isSaving} />
            </div>
          </form>
        </ModalLayer>
      )}

      {deleteTarget && (
        <ModalLayer onClose={() => setDeleteTarget(null)} purpose="required">
          <div className="modal-panel form-panel confirm-dialog">
            <SectionTitle icon={Trash2} title={t('admin.deleteAd')} />
            <p>{t('admin.confirmDeleteAd', { title: deleteTarget.title || deleteTarget.body || t('admin.adUntitled') })}</p>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setDeleteTarget(null)} />
              <XButton type="button" variant="destructive" label={t('admin.deleteAd')} icon={<Trash2 size={17} />} isDisabled={busyAdID === deleteTarget.id} isLoading={busyAdID === deleteTarget.id} onClick={() => void deleteAd()} />
            </div>
          </div>
        </ModalLayer>
      )}
    </div>
  );
}
