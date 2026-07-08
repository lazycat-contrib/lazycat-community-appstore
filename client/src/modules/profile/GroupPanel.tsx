import { type FormEvent, useState } from 'react';
import { Plus, Trash2, UserPlus, Users, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { EmptyState } from '../../shared/components/Feedback';
import type { Group, Toast } from '../../shared/types';
import { runAction } from '../../shared/utils';

export function GroupPanel({
  groups,
  setGroups,
  setToast,
}: {
  groups: Group[];
  setGroups: (groups: Group[]) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState({ name: '', description: '' });
  const [memberDrafts, setMemberDrafts] = useState<Record<number, string>>({});
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [activeGroupID, setActiveGroupID] = useState<number | null>(null);

  async function reload() {
    await runAction(setToast, t('groups.loadFailed'), async () => {
      const data = await api<{ groups: Group[] }>('/api/v1/groups');
      setGroups(data.groups);
    });
  }

  async function createGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await runAction(setToast, t('groups.createFailed'), async () => {
      await api('/api/v1/groups', { method: 'POST', body: JSON.stringify(draft) });
      setDraft({ name: '', description: '' });
      setIsCreateOpen(false);
      setToast({ tone: 'success', message: t('groups.created') });
      await reload();
    });
  }

  async function addMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, t('groups.addMemberFailed'), async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'POST' });
      setToast({ tone: 'success', message: t('groups.memberAdded') });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  async function removeMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, t('groups.removeMemberFailed'), async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('groups.memberRemoved') });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  async function deleteGroup(group: Group) {
    if (!window.confirm(t('groups.deleteConfirm', { name: group.name }))) return;
    await runAction(setToast, t('groups.deleteFailed'), async () => {
      await api(`/api/v1/groups/${group.id}`, { method: 'DELETE' });
      setGroups(groups.filter((item) => item.id !== group.id));
      setMemberDrafts((current) => {
        const next = { ...current };
        delete next[group.id];
        return next;
      });
      if (activeGroupID === group.id) setActiveGroupID(null);
      setToast({ tone: 'neutral', message: t('groups.deleted') });
    });
  }

  return (
    <section className="panel form-panel group-panel">
      <div className="section-title with-action">
        <div>
          <Users size={19} />
          <h2>{t('groups.title')}</h2>
        </div>
        <XButton
          type="button"
          variant={isCreateOpen ? 'secondary' : 'primary'}
          size="sm"
          label={isCreateOpen ? t('groups.cancelCreate') : t('groups.newGroup')}
          icon={isCreateOpen ? <X size={17} /> : <Plus size={17} />}
          onClick={() => setIsCreateOpen((value) => !value)}
        />
      </div>

      {isCreateOpen && (
        <form className="group-create-card" onSubmit={createGroup}>
          <XTextInput label={t('groups.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
          <XTextInput label={t('groups.description')} value={draft.description} onChange={(value) => setDraft({ ...draft, description: value })} />
          <div className="dialog-actions">
            <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setIsCreateOpen(false)} />
            <XButton type="submit" variant="primary" label={t('groups.create')} icon={<Plus size={17} />} />
          </div>
        </form>
      )}

      {groups.length === 0 ? (
        <EmptyState icon={Users} title={t('groups.empty')} body={t('groups.emptyBody')} action={{ label: t('groups.newGroup'), icon: Plus, onClick: () => setIsCreateOpen(true) }} />
      ) : (
        <div className="group-list" role="list" aria-label={t('groups.title')}>
          {groups.map((group) => (
            <article key={group.id} className="group-row" role="listitem">
              <div className="group-row-main">
                <div className="group-row-title">
                  <strong>{group.name}</strong>
                  <span>{group.description || group.slug}</span>
                </div>
                <div className="row-actions">
                  <XButton
                    type="button"
                    variant={activeGroupID === group.id ? 'primary' : 'secondary'}
                    size="sm"
                    label={activeGroupID === group.id ? t('groups.hideManagement') : t('groups.manage')}
                    icon={<Users size={17} />}
                    onClick={() => setActiveGroupID((current) => (current === group.id ? null : group.id))}
                  />
                  <XIconButton
                    className="fixed-row-icon-button"
                    type="button"
                    variant="destructive"
                    size="sm"
                    label={t('groups.deleteGroup')}
                    tooltip={t('groups.deleteGroup')}
                    icon={<Trash2 size={17} />}
                    onClick={() => void deleteGroup(group)}
                  />
                </div>
              </div>
              {activeGroupID === group.id && (
                <div className="group-member-panel">
                  <div className="group-member-copy">
                    <strong>{t('groups.memberManagement')}</strong>
                    <span>{t('groups.memberManagementBody')}</span>
                  </div>
                  <div className="group-member-form">
                    <XTextInput
                      label={t('groups.userId')}
                      placeholder={t('groups.userId')}
                      value={memberDrafts[group.id] || ''}
                      onChange={(value) => setMemberDrafts((current) => ({ ...current, [group.id]: value }))}
                    />
                    <div className="row-actions group-member-buttons">
                      <XButton type="button" variant="secondary" size="sm" label={t('groups.addMember')} icon={<UserPlus size={17} />} onClick={() => void addMember(group.id)} />
                      <XButton type="button" variant="destructive" size="sm" label={t('groups.removeMember')} icon={<Trash2 size={17} />} onClick={() => void removeMember(group.id)} />
                    </div>
                  </div>
                </div>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
