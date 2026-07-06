import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { SelectableCard as XSelectableCard } from '@astryxdesign/core/SelectableCard';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { Check, Cloud, Database, Folder, Link, Save, Server } from 'lucide-react';
import { useTranslation } from 'react-i18next';

export type StorageSettings = {
  provider: 'LOCAL' | 'S3' | 'CLOUDFLARE_R2' | 'WEBDAV' | string;
  deliveryMode: 'SERVER' | 'DIRECT' | string;
  localPath: string;
  endpointUrl: string;
  bucketName: string;
  region: string;
  pathStyle: boolean;
  accountId: string;
  rootPrefix: string;
  accessKeyId: string;
  secretAccessKey?: string;
  secretAccessKeySet: boolean;
  webdavUsername: string;
  webdavPassword?: string;
  webdavPasswordSet: boolean;
  publicBaseUrl: string;
  serverProxyBaseUrl: string;
  effectiveFileUrlMode: string;
};

export const defaultStorageSettings: StorageSettings = {
  provider: 'LOCAL',
  deliveryMode: 'SERVER',
  localPath: './data/files',
  endpointUrl: '',
  bucketName: '',
  region: 'auto',
  pathStyle: true,
  accountId: '',
  rootPrefix: '',
  accessKeyId: '',
  secretAccessKey: '',
  secretAccessKeySet: false,
  webdavUsername: '',
  webdavPassword: '',
  webdavPasswordSet: false,
  publicBaseUrl: '',
  serverProxyBaseUrl: '',
  effectiveFileUrlMode: 'SERVER',
};

type StorageSettingsPanelProps = {
  storage: StorageSettings;
  onChange: (storage: StorageSettings) => void;
  onSave: () => void | Promise<void>;
  onTest: () => void | Promise<void>;
};

export function StorageSettingsPanel({ storage, onChange, onSave, onTest }: StorageSettingsPanelProps) {
  const { t } = useTranslation();
  const isLocal = storage.provider === 'LOCAL';
  const isObjectStorage = storage.provider === 'S3' || storage.provider === 'CLOUDFLARE_R2';
  const isR2 = storage.provider === 'CLOUDFLARE_R2';
  const isWebDAV = storage.provider === 'WEBDAV';
  const usesDirectURL = storage.deliveryMode === 'DIRECT';

  function update<K extends keyof StorageSettings>(key: K, value: StorageSettings[K]) {
    onChange({ ...storage, [key]: value });
  }

  function setProvider(provider: string) {
    onChange({
      ...storage,
      provider,
      deliveryMode: provider === 'LOCAL' ? 'SERVER' : storage.deliveryMode || 'SERVER',
      region: storage.region || 'auto',
    });
  }

  const providerOptions = [
    { value: 'LOCAL', label: t('admin.storageProviders.local'), description: t('admin.storageProviderDescriptions.local'), icon: <Folder size={18} /> },
    { value: 'S3', label: t('admin.storageProviders.s3'), description: t('admin.storageProviderDescriptions.s3'), icon: <Database size={18} /> },
    { value: 'CLOUDFLARE_R2', label: t('admin.storageProviders.r2'), description: t('admin.storageProviderDescriptions.r2'), icon: <Cloud size={18} /> },
    { value: 'WEBDAV', label: t('admin.storageProviders.webdav'), description: t('admin.storageProviderDescriptions.webdav'), icon: <Server size={18} /> },
  ];

  return (
    <div className="settings-section storage-settings-panel">
      <div className="settings-section-head">
        <strong>{t('admin.storageSettings')}</strong>
        <span>{t('admin.storageSettingsBody')}</span>
      </div>

      <div className="storage-flow">
        <section className="storage-step">
          <div className="storage-step-head">
            <span>1</span>
            <div>
              <strong>{t('admin.storageSteps.provider')}</strong>
              <small>{t('admin.storageHelp.provider')}</small>
            </div>
          </div>
          <div className="storage-provider-grid" role="group" aria-label={t('admin.storageFields.provider')}>
            {providerOptions.map((option) => (
              <XSelectableCard
                key={option.value}
                label={option.label}
                isSelected={(storage.provider || 'LOCAL') === option.value}
                onChange={() => setProvider(option.value)}
                padding={3}
              >
                <div className="storage-provider-card">
                  <span>{option.icon}</span>
                  <strong>{option.label}</strong>
                  <small>{option.description}</small>
                </div>
              </XSelectableCard>
            ))}
          </div>
        </section>

        <section className="storage-step">
          <div className="storage-step-head">
            <span>2</span>
            <div>
              <strong>{t('admin.storageSteps.connection')}</strong>
              <small>{connectionSummary(storage, t)}</small>
            </div>
          </div>

          {isLocal && (
            <XFormLayout>
              <XTextInput
                label={t('admin.storageFields.localPath')}
                description={t('admin.storageHelp.localPath')}
                value={storage.localPath || ''}
                onChange={(value) => update('localPath', value)}
              />
            </XFormLayout>
          )}

          {isObjectStorage && (
            <XFormLayout>
              {isR2 && (
                <XTextInput
                  label={t('admin.storageFields.accountId')}
                  description={t('admin.storageHelp.accountId')}
                  value={storage.accountId || ''}
                  onChange={(value) => update('accountId', value)}
                />
              )}
              <XTextInput
                label={t('admin.storageFields.endpointUrl')}
                description={t(isR2 ? 'admin.storageHelp.r2EndpointUrl' : 'admin.storageHelp.s3EndpointUrl')}
                value={storage.endpointUrl || ''}
                onChange={(value) => update('endpointUrl', value)}
              />
              <XTextInput
                label={t('admin.storageFields.bucketName')}
                description={t('admin.storageHelp.bucketName')}
                value={storage.bucketName || ''}
                onChange={(value) => update('bucketName', value)}
              />
              <XTextInput
                label={t('admin.storageFields.region')}
                description={t('admin.storageHelp.region')}
                value={storage.region || 'auto'}
                onChange={(value) => update('region', value)}
              />
              <XSelector
                label={t('admin.storageFields.pathStyle')}
                description={t('admin.storageHelp.pathStyle')}
                value={String(Boolean(storage.pathStyle))}
                options={[
                  { value: 'true', label: t('common.on') },
                  { value: 'false', label: t('common.off') },
                ]}
                onChange={(value) => update('pathStyle', value === 'true')}
              />
              <XTextInput
                label={t('admin.storageFields.rootPrefix')}
                description={t('admin.storageHelp.rootPrefix')}
                value={storage.rootPrefix || ''}
                onChange={(value) => update('rootPrefix', value)}
              />
              <XTextInput
                label={t('admin.storageFields.accessKeyId')}
                description={t('admin.storageHelp.accessKeyId')}
                value={storage.accessKeyId || ''}
                onChange={(value) => update('accessKeyId', value)}
              />
              <XTextInput
                type="password"
                label={t('admin.storageFields.secretAccessKey')}
                description={storage.secretAccessKeySet ? t('admin.storageSecretConfigured') : t('admin.storageHelp.secretAccessKey')}
                value={storage.secretAccessKey || ''}
                onChange={(value) => update('secretAccessKey', value)}
              />
            </XFormLayout>
          )}

          {isWebDAV && (
            <XFormLayout>
              <XTextInput
                label={t('admin.storageFields.endpointUrl')}
                description={t('admin.storageHelp.webdavEndpointUrl')}
                value={storage.endpointUrl || ''}
                onChange={(value) => update('endpointUrl', value)}
              />
              <XTextInput
                label={t('admin.storageFields.rootPrefix')}
                description={t('admin.storageHelp.rootPrefix')}
                value={storage.rootPrefix || ''}
                onChange={(value) => update('rootPrefix', value)}
              />
              <XTextInput
                label={t('admin.storageFields.webdavUsername')}
                description={t('admin.storageHelp.webdavUsername')}
                value={storage.webdavUsername || ''}
                onChange={(value) => update('webdavUsername', value)}
              />
              <XTextInput
                type="password"
                label={t('admin.storageFields.webdavPassword')}
                description={storage.webdavPasswordSet ? t('admin.storageWebDAVPasswordConfigured') : t('admin.storageHelp.webdavPassword')}
                value={storage.webdavPassword || ''}
                onChange={(value) => update('webdavPassword', value)}
              />
            </XFormLayout>
          )}
        </section>

        <section className="storage-step">
          <div className="storage-step-head">
            <span>3</span>
            <div>
              <strong>{t('admin.storageSteps.delivery')}</strong>
              <small>{t('admin.storageHelp.deliveryMode')}</small>
            </div>
          </div>
          <XToggleButtonGroup value={storage.deliveryMode || 'SERVER'} onChange={(value) => value && update('deliveryMode', value)} label={t('admin.storageFields.deliveryMode')} size="sm">
            <XToggleButton value="SERVER" label={t('admin.storageDeliveryModes.server')} icon={<Server size={16} />} />
            <XToggleButton value="DIRECT" label={t('admin.storageDeliveryModes.direct')} icon={<Link size={16} />} />
          </XToggleButtonGroup>
          {usesDirectURL && (
            <XFormLayout>
              <XTextInput
                label={t('admin.storageFields.publicBaseUrl')}
                description={t('admin.storageHelp.publicBaseUrl')}
                value={storage.publicBaseUrl || ''}
                onChange={(value) => update('publicBaseUrl', value)}
              />
            </XFormLayout>
          )}
          <div className="source-url-preview">
            <span>{t(usesDirectURL ? 'admin.storageDirectLink' : 'admin.storageServerProxy')}</span>
            <code>{usesDirectURL ? storage.publicBaseUrl || t('admin.storageDirectLinkEmpty') : storage.serverProxyBaseUrl || '/api/v1/files/'}</code>
          </div>
        </section>
      </div>

      <div className="storage-settings-footer">
        <div className="storage-summary">
          <XBadge label={t(usesDirectURL ? 'admin.storageDeliveryModes.direct' : 'admin.storageDeliveryModes.server')} variant={usesDirectURL ? 'info' : 'success'} />
          <span>{connectionSummary(storage, t)}</span>
        </div>
        <div className="row-actions">
          <XButton type="button" variant="secondary" size="sm" label={t('admin.testStorage')} icon={<Check size={17} />} onClick={() => void onTest()} />
          <XButton type="button" variant="primary" size="sm" label={t('admin.saveStorage')} icon={<Save size={17} />} onClick={() => void onSave()} />
        </div>
      </div>
    </div>
  );
}

function connectionSummary(storage: StorageSettings, t: (key: string) => string) {
  switch (storage.provider) {
    case 'S3':
      return `${t('admin.storageProviders.s3')} · ${storage.bucketName || '-'} · ${storage.endpointUrl || '-'}`;
    case 'CLOUDFLARE_R2':
      return `${t('admin.storageProviders.r2')} · ${storage.bucketName || '-'} · ${storage.endpointUrl || '-'}`;
    case 'WEBDAV':
      return `${t('admin.storageProviders.webdav')} · ${storage.endpointUrl || '-'}`;
    default:
      return `${t('admin.storageProviders.local')} · ${storage.localPath || '-'}`;
  }
}
