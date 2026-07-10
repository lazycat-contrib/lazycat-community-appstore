import { type FormEvent, useEffect, useState } from 'react';
import { Download, KeyRound, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { Heading as XHeading } from '@astryxdesign/core/Heading';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Text as XText } from '@astryxdesign/core/Text';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { ModalLayer } from '../../shared/components/ModalLayer';
import type { SourceApp, SourceSubscription, SourceVersion, StoreApp, Version } from '../../shared/types';
import { applicableMirrorsForVersion, defaultMirrorIDForVersion, githubMirrorKindForURL, localizedAppName } from '../../shared/utils';

export function InstallOptionsDialog({
  app,
  source,
  version,
  onCancel,
  onSubmit,
}: {
  app: StoreApp | SourceApp;
  source?: SourceSubscription;
  version?: Version | SourceVersion;
  onCancel: () => void;
  onSubmit: (options: { installPassword?: string; mirrorId?: string }) => void | Promise<void>;
}) {
  const { t } = useTranslation();
  const [password, setPassword] = useState('');
  const [mirrorId, setMirrorId] = useState(() => defaultMirrorIDForVersion(source, version) || '');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const dialogTitleId = `install-password-title-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const dialogBodyId = `install-password-body-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const requiresPassword = app.installProtected;
  const mirrorOptions = applicableMirrorsForVersion(source, version);
  const mirrorKind = githubMirrorKindForURL(version && 'upstreamDownloadUrl' in version ? version.upstreamDownloadUrl || version.downloadUrl : version?.downloadUrl);
  const appName = localizedAppName(app);

  useEffect(() => {
    setMirrorId(defaultMirrorIDForVersion(source, version) || '');
  }, [source?.id, version?.version]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (submitting) return;
    const value = password.trim();
    if (requiresPassword && !value) {
      setError(t('installPassword.required'));
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      await Promise.resolve(onSubmit({
        installPassword: requiresPassword ? value : undefined,
        mirrorId: mirrorOptions.length > 0 ? mirrorId : undefined,
      }));
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t('toast.installFailed'));
      setSubmitting(false);
    }
  }

  return (
    <ModalLayer onClose={onCancel} purpose="form" width="min(430px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <form
        className="install-password-dialog"
        aria-labelledby={dialogTitleId}
        aria-describedby={dialogBodyId}
        aria-busy={submitting}
        onSubmit={submit}
      >
        <XIconButton type="button" variant="ghost" label={t('common.close')} icon={<X size={17} />} isDisabled={submitting} onClick={onCancel} />
        <div className="install-password-head">
          <span className="install-password-icon">
            {requiresPassword ? <KeyRound size={21} /> : <Download size={21} />}
          </span>
          <div>
            <XHeading id={dialogTitleId} level={2}>
              {t(mirrorOptions.length > 0 ? 'installOptions.title' : 'installPassword.title')}
            </XHeading>
            <XText id={dialogBodyId} type="supporting" as="p" display="block" wordBreak="break-word">
              {requiresPassword
                ? t('installPassword.body', { name: appName })
                : t('installOptions.body', { name: appName })}
            </XText>
          </div>
        </div>
        <XFormLayout>
          {requiresPassword && (
            <XTextInput
              type="password"
              label={t('installPassword.label')}
              value={password}
              hasAutoFocus
              status={error ? { type: 'error', message: error } : undefined}
              onChange={(value) => {
                setPassword(value);
                if (error) setError('');
              }}
            />
          )}
          {mirrorOptions.length > 0 && (
            <XSelector
              label={t('installOptions.mirror')}
              description={t(mirrorKind === 'raw' ? 'installOptions.rawMirrorHelp' : 'installOptions.downloadMirrorHelp')}
              value={mirrorId}
              options={[
                { value: '', label: t('installOptions.direct') },
                ...mirrorOptions.map((entry) => ({ value: entry.id, label: entry.name })),
              ]}
              onChange={setMirrorId}
            />
          )}
        </XFormLayout>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={submitting} onClick={onCancel} />
          <XButton
            type="submit"
            variant="primary"
            label={submitting ? t('installActivity.status.running') : t('installPassword.confirm')}
            icon={submitting ? <Download size={17} className="spin" /> : <Download size={17} />}
            isDisabled={submitting}
          />
        </div>
      </form>
    </ModalLayer>
  );
}
