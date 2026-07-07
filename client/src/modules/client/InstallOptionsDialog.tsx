import { type FormEvent, useEffect, useState } from 'react';
import { Download, KeyRound, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import type { SourceApp, SourceSubscription, SourceVersion, StoreApp, Version } from '../../shared/types';
import { applicableMirrorsForVersion, defaultMirrorIDForVersion, githubMirrorKindForURL } from '../../shared/utils';

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
  onSubmit: (options: { installPassword?: string; mirrorId?: string }) => void;
}) {
  const { t } = useTranslation();
  const [password, setPassword] = useState('');
  const [mirrorId, setMirrorId] = useState(() => defaultMirrorIDForVersion(source, version) || '');
  const [error, setError] = useState('');
  const dialogTitleId = `install-password-title-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const dialogBodyId = `install-password-body-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const requiresPassword = app.installProtected;
  const mirrorOptions = applicableMirrorsForVersion(source, version);
  const mirrorKind = githubMirrorKindForURL(version && 'upstreamDownloadUrl' in version ? version.upstreamDownloadUrl || version.downloadUrl : version?.downloadUrl);

  useEffect(() => {
    setMirrorId(defaultMirrorIDForVersion(source, version) || '');
  }, [source?.id, version?.version]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onCancel();
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel]);

  function submit(event: FormEvent) {
    event.preventDefault();
    const value = password.trim();
    if (requiresPassword && !value) {
      setError(t('installPassword.required'));
      return;
    }
    onSubmit({
      installPassword: requiresPassword ? value : undefined,
      mirrorId: mirrorOptions.length > 0 ? mirrorId : undefined,
    });
  }

  return (
    <div className="drawer-backdrop modal-backdrop" onClick={onCancel}>
      <form
        className="install-password-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={dialogTitleId}
        aria-describedby={dialogBodyId}
        onSubmit={submit}
        onClick={(event) => event.stopPropagation()}
      >
        <XIconButton type="button" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onCancel} />
        <div className="install-password-head">
          <span className="install-password-icon">
            {requiresPassword ? <KeyRound size={21} /> : <Download size={21} />}
          </span>
          <div>
            <h2 id={dialogTitleId}>{t(mirrorOptions.length > 0 ? 'installOptions.title' : 'installPassword.title')}</h2>
            <p id={dialogBodyId}>
              {requiresPassword
                ? t('installPassword.body', { name: app.name })
                : t('installOptions.body', { name: app.name })}
            </p>
          </div>
        </div>
        {requiresPassword && (
          <XTextInput
            type="password"
            label={t('installPassword.label')}
            value={password}
            hasAutoFocus
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
        {error && <p className="form-error">{error}</p>}
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onCancel} />
          <XButton type="submit" variant="primary" label={t('installPassword.confirm')} icon={<Download size={17} />} />
        </div>
      </form>
    </div>
  );
}
