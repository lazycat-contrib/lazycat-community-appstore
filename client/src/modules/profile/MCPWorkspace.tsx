import { type FormEvent, useEffect, useState } from 'react';
import { CalendarClock, Copy, HelpCircle, KeyRound, Server, Trash2, X } from 'lucide-react';
import { Banner as XBanner } from '@astryxdesign/core/Banner';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { MetadataList as XMetadataList, MetadataListItem as XMetadataListItem } from '@astryxdesign/core/MetadataList';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Text as XText } from '@astryxdesign/core/Text';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import { TokenHelpDialog, type TokenHelpExample } from '../../shared/components/TokenHelpDialog';
import type { MCPPrincipalType, MCPProfile, MCPTokenRecord, Toast, User } from '../../shared/types';
import { arrayOrEmpty, formatDate, runAction } from '../../shared/utils';

type MCPTokenDraft = {
  note: string;
  principalType: MCPPrincipalType;
  neverExpires: boolean;
  expiresAt: string;
};

const initialMCPTokenDraft: MCPTokenDraft = {
  note: '',
  principalType: 'USER',
  neverExpires: true,
  expiresAt: '',
};

const mcpExpiryPresets = [
  { days: 7, labelKey: 'mcp.expiresPreset7d' },
  { days: 30, labelKey: 'mcp.expiresPreset30d' },
  { days: 90, labelKey: 'mcp.expiresPreset90d' },
  { days: 365, labelKey: 'mcp.expiresPreset1y' },
] as const;

export function MCPWorkspace({ user, siteSourceUrl, setToast }: { user: User; siteSourceUrl?: string; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [mcpProfile, setMcpProfile] = useState<MCPProfile | null>(null);
  const [mcpTokens, setMcpTokens] = useState<MCPTokenRecord[]>([]);
  const [newMcpToken, setNewMcpToken] = useState('');
  const [isHelpOpen, setIsHelpOpen] = useState(false);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [draft, setDraft] = useState<MCPTokenDraft>(initialMCPTokenDraft);
  const endpoint = mcpEndpointFromSourceURL(mcpProfile?.sourceUrl || siteSourceUrl) || mcpProfile?.endpoint || `${window.location.origin}/mcp`;
  const canCreateAdminToken = Boolean(mcpProfile?.principalTypes.includes('ADMIN'));

  useEffect(() => {
    if (!HAS_API || !user) return;
    void loadProfile();
    void loadTokens();
  }, [user]);

  useEffect(() => {
    if (!canCreateAdminToken && draft.principalType === 'ADMIN') {
      setDraft((current) => ({ ...current, principalType: 'USER' }));
    }
  }, [canCreateAdminToken, draft.principalType]);

  async function loadProfile() {
    try {
      const data = await api<MCPProfile>('/api/v1/me/mcp');
      setMcpProfile(data);
    } catch {
      setMcpProfile(null);
    }
  }

  async function loadTokens() {
    try {
      const data = await api<{ tokens: MCPTokenRecord[] }>('/api/v1/me/mcp/tokens');
      setMcpTokens(arrayOrEmpty(data.tokens));
    } catch {
      setMcpTokens([]);
    }
  }

  async function copyText(value: string, successMessage: string) {
    await runAction(setToast, t('mcp.copyFailed'), async () => {
      if (!navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(value);
      setToast({ tone: 'success', message: successMessage });
    });
  }

  function openCreateTokenDialog() {
    setDraft(initialMCPTokenDraft);
    setIsCreateOpen(true);
  }

  function closeCreateTokenDialog() {
    setIsCreateOpen(false);
    setDraft(initialMCPTokenDraft);
  }

  async function createToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await runAction(setToast, t('mcp.createFailed'), async () => {
      let expiresAt: string | undefined;
      if (!draft.neverExpires) {
        const expiresAtDate = new Date(draft.expiresAt);
        if (!draft.expiresAt || Number.isNaN(expiresAtDate.getTime())) {
          throw new Error(t('mcp.expiresAtInvalid'));
        }
        if (expiresAtDate.getTime() <= Date.now()) {
          throw new Error(t('mcp.expiresAtPast'));
        }
        expiresAt = expiresAtDate.toISOString();
      }
      const data = await api<{ token: string; record: MCPTokenRecord }>('/api/v1/me/mcp/tokens', {
        method: 'POST',
        body: JSON.stringify({
          note: draft.note.trim(),
          principalType: draft.principalType,
          neverExpires: draft.neverExpires,
          expiresAt,
        }),
      });
      setMcpTokens((current) => [data.record, ...current]);
      setNewMcpToken(data.token);
      setDraft(initialMCPTokenDraft);
      closeCreateTokenDialog();
    });
  }

  async function deleteToken(token: MCPTokenRecord) {
    if (!window.confirm(t('mcp.deleteConfirm', { name: token.note || token.prefix }))) return;
    await runAction(setToast, t('mcp.deleteFailed'), async () => {
      await api(`/api/v1/me/mcp/tokens/${token.id}`, { method: 'DELETE' });
      setMcpTokens((current) => current.filter((item) => item.id !== token.id));
      setToast({ tone: 'neutral', message: t('mcp.deleted') });
    });
  }

  return (
    <section className="workspace-pane">
      <section className="panel mcp-panel">
        <div className="section-title with-action">
          <div>
            <Server size={19} />
            <h2>{t('mcp.title')}</h2>
          </div>
          <div className="row-actions">
            <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('mcp.help')} tooltip={t('mcp.help')} icon={<HelpCircle size={17} />} onClick={() => setIsHelpOpen(true)} />
            <XButton type="button" variant="primary" size="sm" label={t('mcp.createToken')} icon={<KeyRound size={17} />} onClick={openCreateTokenDialog} />
          </div>
        </div>

        <XMetadataList className="mcp-endpoint-row" columns="single" label={{ position: 'top' }}>
          <XMetadataListItem label={t('mcp.endpoint')}>
            <div className="mcp-code-action-row">
              <XCodeBlock code={endpoint} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" container="section" isWrapped />
              <XIconButton
                className="fixed-row-icon-button"
                type="button"
                variant="ghost"
                size="sm"
                label={t('mcp.copyEndpoint')}
                tooltip={t('mcp.copyEndpoint')}
                icon={<Copy size={17} />}
                onClick={() => void copyText(endpoint, t('mcp.endpointCopied'))}
              />
            </div>
          </XMetadataListItem>
        </XMetadataList>

        {newMcpToken && (
          <XBanner
            className="token-reveal"
            status="success"
            title={t('mcp.newToken')}
            description={t('mcp.newTokenHelp')}
            defaultIsExpanded
            endContent={<XButton type="button" variant="secondary" size="sm" label={t('common.copy')} icon={<Copy size={16} />} onClick={() => void copyText(newMcpToken, t('mcp.tokenCopied'))} />}
          >
            <XCodeBlock code={newMcpToken} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />
          </XBanner>
        )}

        {mcpTokens.length === 0 ? (
          <EmptyState icon={Server} title={t('mcp.empty')} body={t('mcp.emptyBody')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {mcpTokens.map((token) => (
              <XListItem
                key={token.id}
                className="mcp-token-row"
                label={token.note || t('mcp.untitledToken')}
                description={(
                  <span className="action-list-description">
                    <XText type="supporting" display="block" wordBreak="break-word">
                      {token.prefix} · {t(`mcp.principal.${token.principalType.toLowerCase()}`)} · {mcpExpiryLabel(token, t)}
                    </XText>
                    <XText type="supporting" display="block" wordBreak="break-word">
                      {token.lastUsedAt ? t('mcp.lastUsedAt', { date: formatDate(token.lastUsedAt) }) : t('mcp.neverUsed')}
                    </XText>
                  </span>
                )}
                endContent={(
                  <div className="row-actions">
                    <StatusBadge
                      tone={mcpTokenExpired(token) ? 'failed' : token.principalType === 'ADMIN' ? 'approved' : 'synced'}
                      label={mcpTokenExpired(token) ? t('mcp.expired') : t(`mcp.principal.${token.principalType.toLowerCase()}`)}
                    />
                    <XButton
                      type="button"
                      variant="destructive"
                      size="sm"
                      label={t('mcp.revokeToken')}
                      icon={<Trash2 size={17} />}
                      onClick={() => void deleteToken(token)}
                    />
                  </div>
                )}
              />
            ))}
          </XList>
        )}
      </section>

      {isHelpOpen && (
        <TokenHelpDialog
          icon={Server}
          title={t('mcp.helpTitle')}
          body={t('mcp.helpBody')}
          titleId="mcp-help-title"
          examples={mcpHelpExamples(t, endpoint)}
          onClose={() => setIsHelpOpen(false)}
        />
      )}
      {isCreateOpen && (
        <MCPTokenDialog
          draft={draft}
          canCreateAdmin={canCreateAdminToken}
          onDraftChange={setDraft}
          onSubmit={createToken}
          onClose={closeCreateTokenDialog}
        />
      )}
    </section>
  );
}

function mcpTokenExpired(token: MCPTokenRecord) {
  return Boolean(token.expiresAt && Date.parse(token.expiresAt) <= Date.now());
}

function mcpEndpointFromSourceURL(sourceURL?: string) {
  if (!sourceURL) return '';
  try {
    const url = new URL(sourceURL, window.location.origin);
    url.search = '';
    url.hash = '';
    url.pathname = url.pathname.replace(/\/source\/v1\/index\.json\/?$/, '/mcp');
    if (!url.pathname.endsWith('/mcp')) {
      url.pathname = '/mcp';
    }
    return url.toString().replace(/\/$/, '');
  } catch {
    const endpoint = sourceURL.replace(/\/source\/v1\/index\.json\/?$/, '/mcp').replace(/\/$/, '');
    return endpoint.endsWith('/mcp') ? endpoint : '';
  }
}

function mcpExpiryLabel(token: MCPTokenRecord, t: (key: string, options?: any) => string) {
  if (!token.expiresAt) return t('mcp.neverExpires');
  return mcpTokenExpired(token) ? t('mcp.expiredAt', { date: formatDate(token.expiresAt) }) : t('mcp.expiresAt', { date: formatDate(token.expiresAt) });
}

function toDatetimeLocalValue(date: Date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function expiryPresetValue(days: number) {
  const date = new Date();
  date.setDate(date.getDate() + days);
  date.setMinutes(0, 0, 0);
  return toDatetimeLocalValue(date);
}

function mcpClientConfigExample(endpoint: string) {
  return [
    '{',
    '  "mcpServers": {',
    '    "lazycat-appstore": {',
    '      "type": "http",',
    `      "url": "${endpoint}",`,
    '      "headers": {',
    '        "Authorization": "Bearer lcmcp_..."',
    '      }',
    '    }',
    '  }',
    '}',
  ].join('\n');
}

const mcpToolCallExample = [
  'Tool: appstore_publish_version_from_url',
  '',
  '{',
  '  "packageId": "cloud.lazycat.example.app",',
  '  "downloadUrl": "https://github.com/acme/app/releases/download/v1.2.4/app.lpk",',
  '  "changelog": "Automated release",',
  '  "useMirrorDownload": true',
  '}',
].join('\n');

function mcpHelpExamples(t: (key: string) => string, endpoint: string): TokenHelpExample[] {
  return [
    { title: t('mcp.helpEndpointTitle'), body: t('mcp.helpEndpointBody'), code: mcpClientConfigExample(endpoint), language: 'json' },
    { title: t('mcp.helpToolsTitle'), body: t('mcp.helpToolsBody'), code: mcpToolCallExample, language: 'json' },
    {
      title: t('mcp.helpPermissionTitle'),
      body: t('mcp.helpPermissionBody'),
      code: ['USER: appstore_list_my_apps, appstore_create_app_from_url, appstore_publish_version_from_url', 'ADMIN: USER tools + appstore_admin_list_apps'].join('\n'),
      language: 'plaintext',
    },
  ];
}

function MCPTokenDialog({
  draft,
  canCreateAdmin,
  onDraftChange,
  onSubmit,
  onClose,
}: {
  draft: MCPTokenDraft;
  canCreateAdmin: boolean;
  onDraftChange: (draft: MCPTokenDraft) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const titleId = 'mcp-token-dialog-title';
  const minimumExpiry = toDatetimeLocalValue(new Date(Date.now() + 60_000));

  function setNeverExpires(neverExpires: boolean) {
    onDraftChange({
      ...draft,
      neverExpires,
      expiresAt: neverExpires ? '' : draft.expiresAt || expiryPresetValue(30),
    });
  }

  return (
    <ModalLayer onClose={onClose} purpose="form" width="min(430px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <form
        className="modal-panel form-panel mcp-token-dialog"
        aria-labelledby={titleId}
        onSubmit={onSubmit}
      >
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <SectionTitle icon={KeyRound} title={t('mcp.createToken')} />
        <XFormLayout>
          <XTextInput label={t('mcp.note')} value={draft.note} onChange={(note) => onDraftChange({ ...draft, note })} />
          <XSelector
            label={t('mcp.principalType')}
            value={draft.principalType}
            onChange={(principalType) => onDraftChange({ ...draft, principalType: principalType as MCPPrincipalType })}
            options={[
              { value: 'USER', label: t('mcp.principal.user') },
              ...(canCreateAdmin ? [{ value: 'ADMIN', label: t('mcp.principal.admin') }] : []),
            ]}
          />
          <XCheckboxInput
            label={t('mcp.neverExpires')}
            value={draft.neverExpires}
            onChange={setNeverExpires}
          />
          {!draft.neverExpires && (
            <div className="datetime-field">
              <label htmlFor="mcp-token-expires-at">
                <CalendarClock size={16} aria-hidden="true" />
                <span>{t('mcp.expiresAtInput')}</span>
              </label>
              <input
                id="mcp-token-expires-at"
                type="datetime-local"
                value={draft.expiresAt}
                min={minimumExpiry}
                onChange={(event) => onDraftChange({ ...draft, expiresAt: event.target.value })}
              />
              <small>{t('mcp.expiresAtHelp')}</small>
              <div className="preset-row" aria-label={t('mcp.expiresPresetLabel')}>
                {mcpExpiryPresets.map((preset) => (
                  <XButton
                    key={preset.days}
                    type="button"
                    variant="secondary"
                    size="sm"
                    label={t(preset.labelKey)}
                    onClick={() => onDraftChange({ ...draft, expiresAt: expiryPresetValue(preset.days) })}
                  />
                ))}
              </div>
            </div>
          )}
        </XFormLayout>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
          <XButton type="submit" variant="primary" label={t('mcp.createToken')} icon={<KeyRound size={17} />} />
        </div>
      </form>
    </ModalLayer>
  );
}
