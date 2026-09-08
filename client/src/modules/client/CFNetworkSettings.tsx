import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Selector } from '@astryxdesign/core/Selector';
import { Switch } from '@astryxdesign/core/Switch';
import { TextInput } from '@astryxdesign/core/TextInput';
import type { ClientSettings } from '../../shared/types';
import { StatusBadge } from '../../shared/components/StatusBadge';
import { validCFEndpointSyntax } from './cfNetworkState';

export function CFNetworkSettings({ settings, onChange, error = '' }: {
  settings: ClientSettings;
  error?: string;
  onChange: (next: ClientSettings) => void;
}) {
  const { t } = useTranslation();
  const [custom, setCustom] = useState(false);
  const [touched, setTouched] = useState(false);
  const endpoint = settings.cfEndpoint ?? 'saas.sin.fan';
  const presets = settings.cfPresets?.length ? settings.cfPresets : [{ endpoint: 'saas.sin.fan', sourceName: '' }];
  const isCustom = custom || !presets.some((item) => item.endpoint === endpoint);
  const invalid = touched && !validCFEndpointSyntax(endpoint);
  return (
    <section className="panel settings-card-panel settings-tab-panel client-settings-panel">
      <div className="settings-card-head">
        <h2>{t('cfNetwork.title')}</h2>
        <StatusBadge tone="info" label={t('cfNetwork.optional')} />
      </div>
      <p className="muted-text">{t('cfNetwork.body')}</p>
      <Switch label={t('cfNetwork.enabled')} description={t('cfNetwork.enabledHelp')}
        value={Boolean(settings.cfEnabled)}
        onChange={(checked) => onChange({ ...settings, cfEnabled: checked })} />
      <Selector label={t('cfNetwork.preset')} value={isCustom ? '__custom' : endpoint}
        options={[
          ...presets.map((item) => ({ value: item.endpoint, label: `${item.endpoint} · ${item.sourceName || t('cfNetwork.builtin')}` })),
          { value: '__custom', label: t('cfNetwork.custom') },
        ]}
        onChange={(value) => {
          setCustom(value === '__custom');
          setTouched(false);
          if (value !== '__custom') onChange({ ...settings, cfEndpoint: value });
        }} />
      {isCustom && <TextInput label={t('cfNetwork.endpoint')} description={t('cfNetwork.endpointHelp')}
        value={endpoint} placeholder="saas.sin.fan"
        status={error || invalid ? { type: 'error', message: error || t('cfNetwork.invalid') } : undefined}
        onBlur={() => setTouched(true)}
        onChange={(value) => onChange({ ...settings, cfEndpoint: value })} />}
      <p className="muted-text">{t('cfNetwork.hint')}</p>
    </section>
  );
}
