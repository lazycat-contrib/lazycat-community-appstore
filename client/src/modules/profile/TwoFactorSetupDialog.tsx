import { useEffect, useState } from 'react';
import { Check, KeyRound, QrCode, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { api } from '../../shared/api';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Toast, User } from '../../shared/types';
import { errorMessage } from '../../shared/utils';

type TwoFactorSetup = {
  secret: string;
  otpAuthUrl: string;
  qrDataUrl: string;
};

export function TwoFactorSetupDialog({
  onClose,
  onEnabled,
  setToast,
}: {
  onClose: () => void;
  onEnabled: (user: User) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [setup, setSetup] = useState<TwoFactorSetup | null>(null);
  const [step, setStep] = useState<'scan' | 'verify'>('scan');
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let active = true;
    api<{ setup: TwoFactorSetup }>('/api/v1/me/2fa/setup', { method: 'POST' })
      .then((data) => {
        if (active) setSetup(data.setup);
      })
      .catch((error) => {
        setToast({ tone: 'error', message: errorMessage(error, t('profile.twoFactorSetupFailed')) });
        onClose();
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [onClose, setToast, t]);

  async function enable() {
    if (!setup) return;
    setSubmitting(true);
    try {
      const data = await api<{ user: User }>('/api/v1/me/2fa/enable', {
        method: 'POST',
        body: JSON.stringify({ secret: setup.secret, code }),
      });
      setToast({ tone: 'success', message: t('profile.twoFactorEnabled') });
      onEnabled(data.user);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('profile.twoFactorEnableFailed')) });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <ModalLayer onClose={onClose} purpose="form" width="min(520px, calc(100vw - 36px))">
      <section className="modal-panel form-panel two-factor-dialog" aria-label={t('profile.twoFactorTitle')}>
        <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onClose} />
        <SectionTitle icon={step === 'scan' ? QrCode : KeyRound} title={t('profile.twoFactorTitle')} />
        {loading || !setup ? (
          <p className="inline-note">{t('common.loading')}</p>
        ) : step === 'scan' ? (
          <>
            <p className="inline-note">{t('profile.twoFactorScanBody')}</p>
            <div className="two-factor-qr">
              <img src={setup.qrDataUrl} alt={t('profile.twoFactorQRCode')} />
            </div>
            <div className="two-factor-secret" aria-label={t('profile.twoFactorManualKey')}>
              <span>{t('profile.twoFactorManualKey')}</span>
              <code>{setup.secret}</code>
            </div>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
              <XButton type="button" variant="primary" label={t('common.next')} icon={<Check size={17} />} onClick={() => setStep('verify')} />
            </div>
          </>
        ) : (
          <>
            <p className="inline-note">{t('profile.twoFactorVerifyBody')}</p>
            <XTextInput label={t('auth.twoFactorCode')} value={code} isRequired onChange={(value) => setCode(value.replace(/\D/g, '').slice(0, 6))} />
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.back')} icon={<QrCode size={17} />} onClick={() => setStep('scan')} />
              <XButton type="button" variant="primary" label={submitting ? t('common.submitting') : t('profile.twoFactorConfirm')} icon={<Check size={17} />} isDisabled={submitting || code.trim().length < 6} onClick={() => void enable()} />
            </div>
          </>
        )}
      </section>
    </ModalLayer>
  );
}
