import { useState } from 'react';
import { UserMinus, UserPlus } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import type { Group } from '../../shared/types';

export function GroupMemberManager({
  group,
  onAddMember,
  onRemoveMember,
}: {
  group: Group;
  onAddMember: (groupID: number, userID: number) => Promise<void>;
  onRemoveMember: (groupID: number, userID: number) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [userID, setUserID] = useState('');
  const parsedUserID = Number.parseInt(userID, 10);
  const canSubmit = Number.isFinite(parsedUserID) && parsedUserID > 0;

  async function run(action: 'add' | 'remove') {
    if (!canSubmit) return;
    if (action === 'add') {
      await onAddMember(group.id, parsedUserID);
    } else {
      await onRemoveMember(group.id, parsedUserID);
    }
    setUserID('');
  }

  return (
    <div className="group-member-manager">
      <XTextInput
        label={t('groups.userId')}
        value={userID}
        onChange={setUserID}
      />
      <div className="row-actions">
        <XButton type="button" size="sm" variant="secondary" label={t('groups.addMember')} icon={<UserPlus size={16} />} isDisabled={!canSubmit} onClick={() => void run('add')} />
        <XButton type="button" size="sm" variant="destructive" label={t('groups.removeMember')} icon={<UserMinus size={16} />} isDisabled={!canSubmit} onClick={() => void run('remove')} />
      </div>
    </div>
  );
}
