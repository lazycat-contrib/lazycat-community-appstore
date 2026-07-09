import { type FormEvent, useState } from 'react';
import { KeyRound, Mail, Send, ShieldCheck, X } from 'lucide-react';
import { ClawCaptcha } from 'playcaptcha';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Toast } from '../../shared/types';
import { errorMessage, runAction } from '../../shared/utils';

export function passwordResetTokenFromURL() {
  const params = new URLSearchParams(window.location.search);
  if (window.location.pathname.includes('reset-password')) return params.get('token') || '';
  if (window.location.pathname === '/login' && params.get('mode') === 'reset') return params.get('token') || '';
  return '';
}

export function ForgotPasswordDialog({
  onClose,
  setToast,
}: {
  onClose: () => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [email, setEmail] = useState('');
  const [captchaVerified, setCaptchaVerified] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function submit(event?: FormEvent) {
    event?.preventDefault();
    if (!captchaVerified) {
      setToast({ tone: 'neutral', message: t('auth.resetCaptchaRequired') });
      return;
    }
    setSubmitting(true);
    try {
      await api('/api/v1/auth/password-reset/request', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });
      setToast({ tone: 'success', message: t('auth.resetEmailSent') });
      onClose();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('auth.resetEmailFailed')) });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <ModalLayer onClose={onClose} purpose="form" width="min(520px, calc(100vw - 36px))">
      <form className="modal-panel form-panel password-reset-dialog" onSubmit={submit}>
        <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onClose} />
        <SectionTitle icon={Mail} title={t('auth.forgotPassword')} />
        <p className="inline-note">{t('auth.forgotPasswordBody')}</p>
        <XTextInput htmlName="email" type="email" label={t('common.email')} value={email} isRequired onChange={setEmail} />
        <div className="captcha-panel" role="group" aria-label={t('auth.captchaTitle')}>
          {!captchaVerified && <ClawCaptcha title={t('auth.captchaTitle')} onVerify={() => setCaptchaVerified(true)} />}
          <p className={captchaVerified ? 'inline-success' : 'inline-warning'}>
            <ShieldCheck size={15} />
            <span>{captchaVerified ? t('auth.captchaVerified') : t('auth.resetCaptchaBody')}</span>
          </p>
        </div>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
          <XButton type="submit" variant="primary" label={submitting ? t('common.submitting') : t('auth.sendResetLink')} icon={<Send size={17} />} isDisabled={submitting || !captchaVerified} />
        </div>
      </form>
    </ModalLayer>
  );
}

export function PasswordResetForm({
  token,
  setToast,
  onComplete,
}: {
  token: string;
  setToast: (toast: Toast) => void;
  onComplete: () => void;
}) {
  const { t } = useTranslation();
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  async function submit() {
    if (!token.trim()) {
      setToast({ tone: 'error', message: t('auth.resetTokenMissing') });
      return;
    }
    if (newPassword !== confirmPassword) {
      setToast({ tone: 'error', message: t('auth.passwordMismatch') });
      return;
    }
    await runAction(setToast, t('auth.passwordResetFailed'), async () => {
      await api('/api/v1/auth/password-reset/confirm', {
        method: 'POST',
        body: JSON.stringify({ token, newPassword }),
      });
      setToast({ tone: 'success', message: t('auth.passwordResetDone') });
      onComplete();
    });
  }

  return (
    <>
      <SectionTitle icon={KeyRound} title={t('auth.resetPassword')} />
      <p className="inline-note">{t('auth.resetPasswordBody')}</p>
      <XTextInput htmlName="newPassword" type="password" label={t('profile.newPassword')} description={t('profile.newPasswordHelp')} value={newPassword} isRequired onChange={setNewPassword} />
      <XTextInput htmlName="confirmPassword" type="password" label={t('setup.confirmPassword')} value={confirmPassword} isRequired onChange={setConfirmPassword} />
      <XButton type="button" variant="primary" label={t('auth.resetPassword')} icon={<KeyRound size={18} />} onClick={() => void submit()} />
    </>
  );
}
