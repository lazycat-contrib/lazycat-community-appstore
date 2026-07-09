import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { Check, Cloud, Copy, Database, Folder, Link, Plus, Save, Server, Star, Trash2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { cx } from '../../shared/utils';

export type StorageSettings = {
  key: string;
  name: string;
  isDefault?: boolean;
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
  key: 'primary',
  name: 'Primary',
  isDefault: true,
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
  storages: StorageSettings[];
  defaultKey: string;
  selectedKey: string;
  draft: StorageSettings;
  createDraft: StorageSettings;
  isCreateOpen: boolean;
  onSelect: (key: string) => void;
  onDraftChange: (storage: StorageSettings) => void;
  onCreateDraftChange: (storage: StorageSettings) => void;
  onOpenCreate: () => void;
  onCloseCreate: () => void;
  onCreate: () => void | Promise<void>;
  onSave: () => void | Promise<void>;
  onTestDraft: () => void | Promise<void>;
  onTestSaved: (storage: StorageSettings) => void | Promise<void>;
  onSetDefault: (storage: StorageSettings) => void | Promise<void>;
  onDelete: (storage: StorageSettings) => void | Promise<void>;
};

export function StorageSettingsPanel({
  storages,
  defaultKey,
  selectedKey,
  draft,
  createDraft,
  isCreateOpen,
  onSelect,
  onDraftChange,
  onCreateDraftChange,
  onOpenCreate,
  onCloseCreate,
  onCreate,
  onSave,
  onTestDraft,
  onTestSaved,
  onSetDefault,
  onDelete,
}: StorageSettingsPanelProps) {
  const { t } = useTranslation();
  const selectedStorage = storages.find((storage) => storage.key === selectedKey) || storages[0];
  const effectiveDefaultKey = defaultKey || storages.find((storage) => storage.isDefault)?.key || storages[0]?.key || '';
  const defaultStorageOptions = storages.map((storage) => ({ value: storage.key, label: storage.name || storage.key }));

  return (
    <div className="settings-section storage-settings-panel">
      <div className="settings-section-head with-action">
        <div>
          <strong>{t('admin.storageSettings')}</strong>
          <span>{t('admin.storageSettingsBody')}</span>
        </div>
        <XButton type="button" variant="primary" size="sm" label={t('admin.createStorage')} icon={<Plus size={17} />} onClick={onOpenCreate} />
      </div>

      <div className="storage-admin-grid">
        <div className="storage-config-overview">
          <div className="storage-config-overview-head">
            <div>
              <strong>{t('admin.storageConfigs')}</strong>
              <span>{t('admin.defaultStoragePickerHelp')}</span>
            </div>
            {defaultStorageOptions.length > 0 && (
              <div className="storage-default-control">
                <XSelector
                  label={t('admin.defaultStoragePicker')}
                  value={effectiveDefaultKey}
                  options={defaultStorageOptions}
                  onChange={(nextKey) => {
                    const storage = storages.find((item) => item.key === nextKey);
                    if (storage && storage.key !== effectiveDefaultKey) {
                      void onSetDefault(storage);
                    }
                  }}
                />
              </div>
            )}
          </div>

          <div className="storage-config-list" role="list" aria-label={t('admin.storageConfigs')}>
            {storages.map((storage) => {
              const isDefault = storage.key === effectiveDefaultKey;
              const canDelete = !isDefault && storage.key !== defaultStorageSettings.key;
              return (
                <div key={storage.key} className={cx('storage-config-row', storage.key === selectedKey && 'selected')} role="listitem">
                  <button type="button" className="storage-config-main" onClick={() => onSelect(storage.key)}>
                    <span className="storage-config-icon">{providerIcon(storage.provider)}</span>
                    <span className="storage-config-body">
                      <strong>{storage.name || storage.key}</strong>
                      <small>{storage.key} · {connectionSummary(storage, t)}</small>
                    </span>
                    <span className="storage-config-meta">
                      {isDefault && <XBadge label={t('admin.defaultStorage')} variant="success" icon={<Star size={13} />} />}
                    </span>
                  </button>
                  <span className="storage-config-actions">
                    {!isDefault && (
                      <XButton className="storage-default-button" type="button" variant="secondary" size="sm" label={t('admin.setDefaultStorage')} icon={<Star size={16} />} onClick={() => void onSetDefault(storage)} />
                    )}
                    <XIconButton type="button" variant="ghost" size="sm" label={t('admin.testStorageNamed', { name: storage.name || storage.key })} icon={<Check size={16} />} onClick={() => void onTestSaved(storage)} />
                    {canDelete && (
                      <XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteStorageNamed', { name: storage.name || storage.key })} icon={<Trash2 size={16} />} onClick={() => void onDelete(storage)} />
                    )}
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        <section className="storage-editor-panel">
          <div className="storage-editor-head">
            <div>
              <strong>{selectedStorage?.name || draft.name || draft.key}</strong>
              <span>{draft.key || selectedKey}</span>
            </div>
            <div className="storage-editor-actions">
              <XButton className="storage-action-button" type="button" variant="secondary" size="sm" label={t('admin.testStorage')} icon={<Check size={17} />} onClick={() => void onTestDraft()} />
              <XButton className="storage-action-button" type="button" variant="primary" size="sm" label={t('admin.saveStorage')} icon={<Save size={17} />} onClick={() => void onSave()} />
            </div>
          </div>
          <StorageFields storage={draft} onChange={onDraftChange} mode="edit" />
        </section>
      </div>

      {isCreateOpen && (
        <ModalLayer onClose={onCloseCreate} purpose="form" width="min(720px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
          <form
            className="modal-panel form-panel storage-dialog"
            aria-label={t('admin.createStorage')}
            onSubmit={(event) => {
              event.preventDefault();
              void onCreate();
            }}
          >
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onCloseCreate} />
            <div className="section-title">
              <Plus size={19} />
              <h2>{t('admin.createStorage')}</h2>
            </div>
            <StorageFields storage={createDraft} onChange={onCreateDraftChange} mode="create" />
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onCloseCreate} />
              <XButton type="submit" variant="primary" label={t('admin.createStorage')} icon={<Save size={17} />} />
            </div>
          </form>
        </ModalLayer>
      )}
    </div>
  );
}

function StorageFields({ storage, onChange, mode }: { storage: StorageSettings; onChange: (storage: StorageSettings) => void; mode: 'create' | 'edit' }) {
  const { t } = useTranslation();
  const isLocal = storage.provider === 'LOCAL';
  const isObjectStorage = storage.provider === 'S3' || storage.provider === 'CLOUDFLARE_R2';
  const isR2 = storage.provider === 'CLOUDFLARE_R2';
  const isWebDAV = storage.provider === 'WEBDAV';
  const usesDirectURL = storage.deliveryMode === 'DIRECT';
  const deliveryPreviewURL = usesDirectURL ? storage.publicBaseUrl.trim() : (storage.serverProxyBaseUrl || '/api/v1/files/');

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

  return (
    <div className="storage-flow">
      <section className="storage-step">
        <div className="storage-step-head">
          <span>1</span>
          <div>
            <strong>{t('admin.storageSteps.identity')}</strong>
            <small>{t('admin.storageHelp.identity')}</small>
          </div>
        </div>
        <XFormLayout>
          {mode === 'create' ? (
            <XTextInput
              label={t('admin.storageFields.key')}
              description={t('admin.storageHelp.key')}
              value={storage.key || ''}
              onChange={(value) => update('key', value)}
            />
          ) : (
            <div className="readonly-field">
              <span>{t('admin.storageFields.key')}</span>
              <code>{storage.key}</code>
            </div>
          )}
          <XTextInput
            label={t('admin.storageFields.name')}
            description={t('admin.storageHelp.name')}
            value={storage.name || ''}
            onChange={(value) => update('name', value)}
          />
        </XFormLayout>
      </section>

      <section className="storage-step">
        <div className="storage-step-head">
          <span>2</span>
          <div>
            <strong>{t('admin.storageSteps.provider')}</strong>
            <small>{t('admin.storageHelp.provider')}</small>
          </div>
        </div>
        <XToggleButtonGroup value={storage.provider || 'LOCAL'} onChange={(value) => value && setProvider(value)} label={t('admin.storageFields.provider')} size="sm">
          <XToggleButton value="LOCAL" label={t('admin.storageProviders.local')} icon={<Folder size={16} />} />
          <XToggleButton value="S3" label={t('admin.storageProviders.s3')} icon={<Database size={16} />} />
          <XToggleButton value="CLOUDFLARE_R2" label={t('admin.storageProviders.r2')} icon={<Cloud size={16} />} />
          <XToggleButton value="WEBDAV" label={t('admin.storageProviders.webdav')} icon={<Server size={16} />} />
        </XToggleButtonGroup>
      </section>

      <section className="storage-step">
        <div className="storage-step-head">
          <span>3</span>
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
            <XSwitch
              label={t('admin.storageFields.pathStyle')}
              description={t('admin.storageHelp.pathStyle')}
              value={Boolean(storage.pathStyle)}
              labelSpacing="spread"
              width="100%"
              onChange={(checked) => update('pathStyle', checked)}
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
          <span>4</span>
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
          <code>{deliveryPreviewURL || t('admin.storageDirectLinkEmpty')}</code>
          {deliveryPreviewURL && <Copy size={15} aria-hidden="true" />}
        </div>
      </section>
    </div>
  );
}

function providerIcon(provider: string) {
  switch (provider) {
    case 'S3':
      return <Database size={18} />;
    case 'CLOUDFLARE_R2':
      return <Cloud size={18} />;
    case 'WEBDAV':
      return <Server size={18} />;
    default:
      return <Folder size={18} />;
  }
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
