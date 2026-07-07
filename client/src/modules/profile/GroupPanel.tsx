import { type FormEvent, useState } from 'react';
import { Plus, Trash2, Users } from 'lucide-react';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
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

  return (
    <section className="panel form-panel">
      <SectionTitle icon={Users} title={t('groups.title')} />
      <form className="inline-form" onSubmit={createGroup}>
        <XTextInput label={t('groups.name')} isLabelHidden placeholder={t('groups.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
        <XIconButton type="submit" variant="secondary" label={t('groups.create')} icon={<Plus size={17} />} />
      </form>
      <div className="review-list">
        {groups.length === 0 ? <EmptyState icon={Users} title={t('groups.empty')} /> : groups.map((group) => (
          <div className="review-row" key={group.id}>
            <div>
              <strong>{group.name}</strong>
              <span>{group.slug}</span>
            </div>
            <div className="inline-form compact-line group-member-actions">
              <XTextInput
                label={t('groups.userId')}
                isLabelHidden
                placeholder={t('groups.userId')}
                value={memberDrafts[group.id] || ''}
                onChange={(value) => setMemberDrafts((current) => ({ ...current, [group.id]: value }))}
              />
              <XIconButton type="button" variant="secondary" label={t('groups.addMember')} icon={<Plus size={17} />} onClick={() => void addMember(group.id)} />
              <XIconButton type="button" variant="destructive" label={t('groups.removeMember')} icon={<Trash2 size={17} />} onClick={() => void removeMember(group.id)} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
