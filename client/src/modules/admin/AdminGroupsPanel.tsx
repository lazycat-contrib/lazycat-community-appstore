import { type FormEvent, useEffect, useMemo, useState } from 'react';
import { Copy, Pencil, Plus, RefreshCw, Trash2, Users, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import type { Group, Toast } from '../../shared/types';
import { errorMessage, runAction } from '../../shared/utils';
import { GroupCodeManager } from './GroupCodeManager';

type GroupDraft = { name: string; description: string };

export function AdminGroupsPanel({
  sourceURL,
  setToast,
}: {
  sourceURL: string;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [groups, setGroups] = useState<Group[]>([]);
  const [draft, setDraft] = useState<GroupDraft>({ name: '', description: '' });
  const [editDraft, setEditDraft] = useState<GroupDraft>({ name: '', description: '' });
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [selectedGroupIDs, setSelectedGroupIDs] = useState<number[]>([]);
  const [groupToEdit, setGroupToEdit] = useState<Group | null>(null);
  const [groupToDelete, setGroupToDelete] = useState<Group | null>(null);
  const [groupCodeToRotate, setGroupCodeToRotate] = useState<Group | null>(null);
  const [pendingAction, setPendingAction] = useState<'delete' | 'rotate' | null>(null);
  const [generatedConfig, setGeneratedConfig] = useState<{ encoded: string; config: { sourceUrl: string; groupCodes: string[]; groups: Array<{ name: string; code?: string }> } } | null>(null);
  const selectedGroups = useMemo(() => groups.filter((group) => selectedGroupIDs.includes(group.id)), [groups, selectedGroupIDs]);

  useEffect(() => {
    void loadGroups();
  }, []);

  async function loadGroups() {
    await runAction(setToast, t('groups.loadFailed'), async () => {
      const data = await api<{ groups: Group[] }>('/api/v1/groups');
      setGroups(data.groups || []);
    });
  }

  async function createGroup(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('groups.createFailed'), async () => {
      await api('/api/v1/groups', { method: 'POST', body: JSON.stringify(draft) });
      setDraft({ name: '', description: '' });
      setIsCreateOpen(false);
      setToast({ tone: 'success', message: t('groups.created') });
      await loadGroups();
    });
  }

  async function updateGroup(event: FormEvent) {
    event.preventDefault();
    if (!groupToEdit) return;
    await runAction(setToast, t('groups.updateFailed'), async () => {
      await api(`/api/v1/groups/${groupToEdit.id}`, { method: 'PATCH', body: JSON.stringify(editDraft) });
      setGroupToEdit(null);
      setEditDraft({ name: '', description: '' });
      setToast({ tone: 'success', message: t('groups.updated') });
      await loadGroups();
    });
  }

  async function deleteGroup(group: Group) {
    if (pendingAction) return;
    setPendingAction('delete');
    try {
      await runAction(setToast, t('groups.deleteFailed'), async () => {
        await api(`/api/v1/groups/${group.id}`, { method: 'DELETE' });
        setGroupToDelete(null);
        setToast({ tone: 'neutral', message: t('groups.deleted') });
        await loadGroups();
      });
    } finally {
      setPendingAction(null);
    }
  }

  async function rotateGroupCode(group: Group) {
    if (pendingAction) return;
    setPendingAction('rotate');
    try {
      await runAction(setToast, t('admin.groups.rotateFailed'), async () => {
        await api(`/api/v1/groups/${group.id}/code:rotate`, { method: 'POST' });
        setGroupCodeToRotate(null);
        setToast({ tone: 'success', message: t('admin.groups.rotated') });
        await loadGroups();
      });
    } finally {
      setPendingAction(null);
    }
  }

  async function generateConfig() {
    await runAction(setToast, t('admin.groups.configFailed'), async () => {
      const data = await api<{ encoded: string; config: { sourceUrl: string; groupCodes: string[]; groups: Array<{ name: string; code?: string }> } }>('/api/v1/groups/client-config', {
        method: 'POST',
        body: JSON.stringify({ sourceUrl: sourceURL, groupIds: selectedGroupIDs }),
      });
      setGeneratedConfig({ encoded: data.encoded, config: data.config });
    });
  }

  async function copyConfig() {
    try {
      if (!generatedConfig?.encoded || !navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(generatedConfig.encoded);
      setToast({ tone: 'success', message: t('admin.groups.configCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.groups.configCopyFailed')) });
    }
  }

  function toggleSelected(groupID: number, checked: boolean) {
    setSelectedGroupIDs((current) => checked ? Array.from(new Set([...current, groupID])) : current.filter((id) => id !== groupID));
  }

  function openGroupEditor(group: Group) {
    setGroupToEdit(group);
    setEditDraft({ name: group.name, description: group.description || '' });
  }

  return (
    <section className="workspace-pane admin-groups-workspace">
      <section className="panel">
        <div className="section-title with-action">
          <div>
            <Users size={19} />
            <h2>{t('admin.groups.title')}</h2>
          </div>
          <div className="row-actions">
            <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={16} />} onClick={() => void loadGroups()} />
            <XButton type="button" variant="primary" size="sm" label={t('groups.newGroup')} icon={<Plus size={16} />} onClick={() => setIsCreateOpen(true)} />
          </div>
        </div>
        {groups.length === 0 ? (
          <EmptyState icon={Users} title={t('groups.empty')} body={t('groups.emptyBody')} />
        ) : (
          <XList className="action-list admin-group-list" density="compact" hasDividers>
            {groups.map((group) => (
              <XListItem
                key={group.id}
                className="admin-group-row"
                startContent={(
                  <XCheckboxInput
                    label={t('admin.groups.selectGroup', { name: group.name })}
                    value={selectedGroupIDs.includes(group.id)}
                    onChange={(checked) => toggleSelected(group.id, checked)}
                  />
                )}
                label={group.name}
                description={group.description || group.slug}
                endContent={(
                  <div className="row-actions">
                    <XBadge label={t('admin.groups.appCount', { count: group.attachedAppCount || 0 })} variant={(group.attachedAppCount || 0) > 0 ? 'info' : 'neutral'} />
                    <GroupCodeManager group={group} onRotate={setGroupCodeToRotate} setToast={setToast} />
                    <XIconButton type="button" variant="ghost" size="sm" label={t('groups.editGroup')} icon={<Pencil size={16} />} onClick={() => openGroupEditor(group)} />
                    <XIconButton type="button" variant="destructive" size="sm" label={t('groups.deleteGroup')} icon={<Trash2 size={16} />} onClick={() => setGroupToDelete(group)} />
                  </div>
                )}
              />
            ))}
          </XList>
        )}
      </section>

      <section className="panel">
        <SectionTitle icon={Copy} title={t('admin.groups.clientConfig')} />
        <div className="group-config-summary">
          <span>{t('admin.groups.selectedCount', { count: selectedGroups.length })}</span>
          <XButton type="button" variant="secondary" size="sm" label={t('admin.groups.generateConfig')} icon={<Copy size={16} />} isDisabled={selectedGroupIDs.length === 0} onClick={() => void generateConfig()} />
        </div>
        {selectedGroups.length > 0 && (
          <div className="source-group-chips">
            {selectedGroups.map((group) => <XBadge key={group.id} label={group.name} variant="info" />)}
          </div>
        )}
      </section>

      {isCreateOpen && (
        <ModalLayer onClose={() => setIsCreateOpen(false)} purpose="form">
          <form className="modal-panel form-panel group-dialog" aria-label={t('groups.newGroup')} onSubmit={createGroup}>
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setIsCreateOpen(false)} />
            <SectionTitle icon={Users} title={t('groups.newGroup')} />
            <XFormLayout>
              <XTextInput label={t('groups.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
              <XTextInput label={t('groups.description')} value={draft.description} onChange={(value) => setDraft({ ...draft, description: value })} />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setIsCreateOpen(false)} />
              <XButton type="submit" variant="primary" label={t('groups.create')} icon={<Plus size={17} />} />
            </div>
          </form>
        </ModalLayer>
      )}

      {groupToEdit && (
        <ModalLayer onClose={() => setGroupToEdit(null)} purpose="form">
          <form className="modal-panel form-panel group-dialog" aria-label={t('groups.editGroup')} onSubmit={updateGroup}>
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setGroupToEdit(null)} />
            <SectionTitle icon={Pencil} title={t('groups.editGroup')} />
            <XFormLayout>
              <XTextInput label={t('groups.name')} value={editDraft.name} onChange={(value) => setEditDraft({ ...editDraft, name: value })} />
              <XTextInput label={t('groups.description')} value={editDraft.description} onChange={(value) => setEditDraft({ ...editDraft, description: value })} />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setGroupToEdit(null)} />
              <XButton type="submit" variant="primary" label={t('groups.saveChanges')} icon={<Pencil size={17} />} />
            </div>
          </form>
        </ModalLayer>
      )}

      {generatedConfig && (
        <ModalLayer onClose={() => setGeneratedConfig(null)} purpose="form">
          <div className="modal-panel form-panel group-config-dialog" aria-label={t('admin.groups.clientConfig')}>
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setGeneratedConfig(null)} />
            <SectionTitle icon={Copy} title={t('admin.groups.clientConfig')} />
            <div className="generated-config-box">
              <code>{generatedConfig.encoded}</code>
            </div>
            <div className="generated-config-preview">
              <strong>{generatedConfig.config.sourceUrl}</strong>
              <span>{generatedConfig.config.groups.map((group) => group.name).join(', ')}</span>
            </div>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.close')} icon={<X size={17} />} onClick={() => setGeneratedConfig(null)} />
              <XButton type="button" variant="primary" label={t('admin.groups.copyConfig')} icon={<Copy size={17} />} onClick={() => void copyConfig()} />
            </div>
          </div>
        </ModalLayer>
      )}

      {groupCodeToRotate && (
        <ModalLayer onClose={() => { if (!pendingAction) setGroupCodeToRotate(null); }} purpose="required">
          <div className="modal-panel form-panel confirm-dialog">
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={pendingAction !== null} onClick={() => setGroupCodeToRotate(null)} />
            <SectionTitle icon={RefreshCw} title={t('admin.groups.rotateCode')} />
            <p className="inline-note">{t('admin.groups.rotateConfirm', { name: groupCodeToRotate.name })}</p>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={pendingAction !== null} onClick={() => setGroupCodeToRotate(null)} />
              <XButton type="button" variant="primary" label={t('admin.groups.rotateCode')} icon={<RefreshCw size={17} />} isDisabled={pendingAction !== null} isLoading={pendingAction === 'rotate'} onClick={() => void rotateGroupCode(groupCodeToRotate)} />
            </div>
          </div>
        </ModalLayer>
      )}

      {groupToDelete && (
        <ModalLayer onClose={() => { if (!pendingAction) setGroupToDelete(null); }} purpose="required">
          <div className="modal-panel form-panel confirm-dialog">
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={pendingAction !== null} onClick={() => setGroupToDelete(null)} />
            <SectionTitle icon={Trash2} title={t('groups.deleteGroup')} />
            <p className="inline-note">{t('groups.deleteConfirm', { name: groupToDelete.name })}</p>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={pendingAction !== null} onClick={() => setGroupToDelete(null)} />
              <XButton type="button" variant="destructive" label={t('groups.deleteGroup')} icon={<Trash2 size={17} />} isDisabled={pendingAction !== null} isLoading={pendingAction === 'delete'} onClick={() => void deleteGroup(groupToDelete)} />
            </div>
          </div>
        </ModalLayer>
      )}
    </section>
  );
}
