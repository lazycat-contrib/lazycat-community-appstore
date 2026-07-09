import { type FormEvent, useEffect, useState } from 'react';
import { Save, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { Heading as XHeading } from '@astryxdesign/core/Heading';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Text as XText } from '@astryxdesign/core/Text';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { UserAvatar } from '../../components/AppIcon';
import { api } from '../../shared/api';
import { defaultUploadStorageKey, displayUserName, storageSelectOptions } from '../../shared/appHelpers';
import { FilePicker } from '../../shared/components/FilePicker';
import { ModalLayer } from '../../shared/components/ModalLayer';
import type { StorageOption, Toast, User } from '../../shared/types';
import { runAction } from '../../shared/utils';

export function ProfileSettingsDialog({
  user,
  storageOptions,
  onClose,
  onSaved,
  setToast,
}: {
  user: User;
  storageOptions: StorageOption[];
  onClose: () => void;
  onSaved: (user: User) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState({
    nickname: user.nickname || '',
    email: user.email || '',
  });
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarStorageKey, setAvatarStorageKey] = useState(defaultUploadStorageKey(storageOptions));
  const storageChoices = storageSelectOptions(storageOptions);

  useEffect(() => {
    setDraft({
      nickname: user.nickname || '',
      email: user.email || '',
    });
  }, [user]);

  useEffect(() => {
    const fallback = defaultUploadStorageKey(storageOptions);
    setAvatarStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
  }, [storageOptions]);

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('profile.profileSaveFailed'), async () => {
      let nextUser = user;
      const profileData = await api<{ user: User }>('/api/v1/me/profile', {
        method: 'PATCH',
        body: JSON.stringify({
          nickname: draft.nickname,
          email: draft.email,
        }),
      });
      nextUser = profileData.user;
      if (avatarFile) {
        const form = new FormData();
        form.set('file', avatarFile);
        form.set('storageKey', avatarStorageKey);
        const avatarData = await api<{ user: User; url: string }>('/api/v1/me/avatar', { method: 'POST', body: form });
        nextUser = avatarData.user;
      }
      setToast({ tone: 'success', message: t('profile.profileSaved') });
      onSaved(nextUser);
    });
  }

  return (
    <ModalLayer onClose={onClose} purpose="form">
      <form className="modal-panel form-panel profile-dialog" aria-label={t('profile.personalProfile')} onSubmit={saveProfile}>
        <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onClose} />
        <div className="profile-dialog-head">
          <UserAvatar user={user} size={72} className="avatar-large" />
          <div>
            <XHeading level={2}>{t('profile.personalProfile')}</XHeading>
            <XText type="supporting" as="p" display="block" wordBreak="break-word">
              {displayUserName(user)}
            </XText>
          </div>
        </div>
        <XFormLayout>
          <XTextInput label={t('profile.nickname')} value={draft.nickname} onChange={(value) => setDraft({ ...draft, nickname: value })} />
          <XTextInput type="email" label={t('common.email')} value={draft.email} onChange={(value) => setDraft({ ...draft, email: value })} />
        </XFormLayout>
        <div className="avatar-upload-grid">
          <FilePicker
            label={t('profile.avatar')}
            help={t('profile.avatarHelp')}
            value={avatarFile}
            accept=".png,.jpg,.jpeg,.webp"
            onChange={(nextFile) => setAvatarFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
          />
          {storageChoices.length > 0 && (
            <XSelector
              label={t('common.storage')}
              value={avatarStorageKey}
              options={storageChoices}
              onChange={setAvatarStorageKey}
            />
          )}
        </div>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
          <XButton type="submit" variant="primary" label={t('profile.saveProfile')} icon={<Save size={17} />} />
        </div>
      </form>
    </ModalLayer>
  );
}
