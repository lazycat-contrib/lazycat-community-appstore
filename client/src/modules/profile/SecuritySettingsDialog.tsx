import { type FormEvent, useState } from 'react';
import { KeyRound, Save, ShieldCheck, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Toast, User } from '../../shared/types';
import { errorMessage, runAction } from '../../shared/utils';
import { TwoFactorSetupDialog } from './TwoFactorSetupDialog';

export function SecuritySettingsDialog({
  user,
  twoFactorAvailable,
  onClose,
  onSaved,
  setToast,
}: {
  user: User;
  twoFactorAvailable: boolean;
  onClose: () => void;
  onSaved: (user: User) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [passwordDraft, setPasswordDraft] = useState({ currentPassword: '', newPassword: '' });
  const [twoFactorPassword, setTwoFactorPassword] = useState('');
  const [twoFactorDialogOpen, setTwoFactorDialogOpen] = useState(false);
  const [disablingTwoFactor, setDisablingTwoFactor] = useState(false);

  async function savePassword(event: FormEvent) {
    event.preventDefault();
    if (!passwordDraft.currentPassword.trim() || !passwordDraft.newPassword.trim()) {
      setToast({ tone: 'neutral', message: t('profile.passwordFieldsRequired') });
      return;
    }
    await runAction(setToast, t('profile.passwordChangeFailed'), async () => {
      const data = await api<{ user: User }>('/api/v1/me/profile', {
        method: 'PATCH',
        body: JSON.stringify(passwordDraft),
      });
      setPasswordDraft({ currentPassword: '', newPassword: '' });
      setToast({ tone: 'success', message: t('profile.passwordChanged') });
      onSaved(data.user);
    });
  }

  async function disableTwoFactor() {
    if (!twoFactorPassword.trim()) {
      setToast({ tone: 'neutral', message: t('profile.twoFactorPasswordRequired') });
      return;
    }
    setDisablingTwoFactor(true);
    try {
      const data = await api<{ user: User }>('/api/v1/me/2fa/disable', {
        method: 'POST',
        body: JSON.stringify({ currentPassword: twoFactorPassword }),
      });
      setTwoFactorPassword('');
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
        <section className="modal-panel form-panel security-dialog" aria-label={t('profile.securityOptions')}>
          <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onClose} />
          <SectionTitle icon={ShieldCheck} title={t('profile.securityOptions')} />

          <form className="security-password-panel" onSubmit={savePassword}>
            <SectionTitle icon={KeyRound} title={t('profile.changePassword')} />
            <p className="inline-note">{t('profile.changePasswordBody')}</p>
            <XFormLayout>
              <XTextInput
                type="password"
                label={t('profile.currentPassword')}
                value={passwordDraft.currentPassword}
                onChange={(value) => setPasswordDraft({ ...passwordDraft, currentPassword: value })}
              />
              <XTextInput
                type="password"
                label={t('profile.newPassword')}
                description={t('profile.newPasswordHelp')}
                value={passwordDraft.newPassword}
                onChange={(value) => setPasswordDraft({ ...passwordDraft, newPassword: value })}
              />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="submit" variant="primary" label={t('profile.savePassword')} icon={<Save size={17} />} />
            </div>
          </form>

          {twoFactorAvailable && (
            <section className="profile-security-panel security-two-factor-panel">
              <div>
                <ShieldCheck size={18} />
                <div>
                  <strong>{t('profile.twoFactorTitle')}</strong>
                  <span>{user.twoFactorEnabled ? t('profile.twoFactorOn') : t('profile.twoFactorOff')}</span>
                </div>
              </div>
              {user.twoFactorEnabled ? (
                <div className="security-two-factor-actions">
                  <XTextInput
                    type="password"
                    label={t('profile.currentPassword')}
                    value={twoFactorPassword}
                    onChange={setTwoFactorPassword}
                  />
                  <XButton
                    type="button"
                    variant="secondary"
                    label={disablingTwoFactor ? t('common.submitting') : t('profile.disableTwoFactor')}
                    icon={<X size={17} />}
                    isDisabled={disablingTwoFactor}
                    onClick={() => void disableTwoFactor()}
                  />
                </div>
              ) : (
                <div className="security-two-factor-actions security-two-factor-actions-compact">
                  <XButton
                    type="button"
                    variant="secondary"
                    label={t('profile.enableTwoFactor')}
                    icon={<KeyRound size={17} />}
                    onClick={() => setTwoFactorDialogOpen(true)}
                  />
                </div>
              )}
            </section>
          )}

          <div className="dialog-actions">
            <XButton type="button" variant="secondary" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
          </div>
        </section>
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
