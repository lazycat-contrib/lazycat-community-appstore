import { type FormEvent, useEffect, useState } from 'react';
import { KeyRound, Save, ShieldCheck, X } from 'lucide-react';
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
import { errorMessage, runAction } from '../../shared/utils';
import { TwoFactorSetupDialog } from './TwoFactorSetupDialog';

export function ProfileSettingsDialog({
  user,
  storageOptions,
  onClose,
  onSaved,
  setToast,
  twoFactorAvailable,
}: {
  user: User;
  storageOptions: StorageOption[];
  onClose: () => void;
  onSaved: (user: User) => void;
  setToast: (toast: Toast) => void;
  twoFactorAvailable: boolean;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState({
    nickname: user.nickname || '',
    email: user.email || '',
    currentPassword: '',
    newPassword: '',
  });
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarStorageKey, setAvatarStorageKey] = useState(defaultUploadStorageKey(storageOptions));
  const [twoFactorDialogOpen, setTwoFactorDialogOpen] = useState(false);
  const [disablingTwoFactor, setDisablingTwoFactor] = useState(false);
  const storageChoices = storageSelectOptions(storageOptions);

  useEffect(() => {
    setDraft({
      nickname: user.nickname || '',
      email: user.email || '',
      currentPassword: '',
      newPassword: '',
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
          currentPassword: draft.currentPassword,
          newPassword: draft.newPassword,
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

  async function disableTwoFactor() {
    if (!draft.currentPassword.trim()) {
      setToast({ tone: 'neutral', message: t('profile.twoFactorPasswordRequired') });
      return;
    }
    setDisablingTwoFactor(true);
    try {
      const data = await api<{ user: User }>('/api/v1/me/2fa/disable', {
        method: 'POST',
        body: JSON.stringify({ currentPassword: draft.currentPassword }),
      });
      setToast({ tone: 'success', message: t('profile.twoFactorDisabled') });
      onSaved(data.user);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('profile.twoFactorDisableFailed')) });
    } finally {
      setDisablingTwoFactor(false);
    }
  }

  return (
    <>
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
            <XTextInput type="password" label={t('profile.currentPassword')} value={draft.currentPassword} onChange={(value) => setDraft({ ...draft, currentPassword: value })} />
            <XTextInput type="password" label={t('profile.newPassword')} description={t('profile.newPasswordHelp')} value={draft.newPassword} onChange={(value) => setDraft({ ...draft, newPassword: value })} />
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
          {twoFactorAvailable && (
            <section className="profile-security-panel">
              <div>
                <ShieldCheck size={18} />
                <div>
                  <strong>{t('profile.twoFactorTitle')}</strong>
                  <span>{user.twoFactorEnabled ? t('profile.twoFactorOn') : t('profile.twoFactorOff')}</span>
                </div>
              </div>
              {user.twoFactorEnabled ? (
                <XButton
                  type="button"
                  variant="secondary"
                  label={disablingTwoFactor ? t('common.submitting') : t('profile.disableTwoFactor')}
                  icon={<X size={17} />}
                  isDisabled={disablingTwoFactor}
                  onClick={() => void disableTwoFactor()}
                />
              ) : (
                <XButton
                  type="button"
                  variant="secondary"
                  label={t('profile.enableTwoFactor')}
                  icon={<KeyRound size={17} />}
                  onClick={() => setTwoFactorDialogOpen(true)}
                />
              )}
            </section>
          )}
          <div className="dialog-actions">
            <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
            <XButton type="submit" variant="primary" label={t('profile.saveProfile')} icon={<Save size={17} />} />
          </div>
        </form>
      </ModalLayer>
      {twoFactorDialogOpen && (
        <TwoFactorSetupDialog
          onClose={() => setTwoFactorDialogOpen(false)}
          onEnabled={(nextUser) => {
            setTwoFactorDialogOpen(false);
            onSaved(nextUser);
          }}
          setToast={setToast}
        />
      )}
    </>
  );
}
