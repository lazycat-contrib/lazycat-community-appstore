import { type FormEvent, useEffect, useState } from 'react';
import { Copy, HelpCircle, KeyRound, Server, Trash2, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
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

export function MCPWorkspace({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [mcpProfile, setMcpProfile] = useState<MCPProfile | null>(null);
  const [mcpTokens, setMcpTokens] = useState<MCPTokenRecord[]>([]);
  const [newMcpToken, setNewMcpToken] = useState('');
  const [isHelpOpen, setIsHelpOpen] = useState(false);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [draft, setDraft] = useState<MCPTokenDraft>(initialMCPTokenDraft);
  const endpoint = mcpProfile?.endpoint || `${window.location.origin}/mcp`;
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

  async function createToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await runAction(setToast, t('mcp.createFailed'), async () => {
      const expiresAt = draft.neverExpires || !draft.expiresAt ? undefined : new Date(draft.expiresAt).toISOString();
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
      setIsCreateOpen(false);
    });
  }

  async function deleteToken(tokenID: number) {
    await runAction(setToast, t('mcp.deleteFailed'), async () => {
      await api(`/api/v1/me/mcp/tokens/${tokenID}`, { method: 'DELETE' });
      setMcpTokens((current) => current.filter((token) => token.id !== tokenID));
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
            <XButton type="button" variant="primary" size="sm" label={t('mcp.createToken')} icon={<KeyRound size={17} />} onClick={() => setIsCreateOpen(true)} />
          </div>
        </div>

        <div className="mcp-endpoint-row">
          <div>
            <span>{t('mcp.endpoint')}</span>
            <XCodeBlock code={endpoint} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" container="section" isWrapped />
          </div>
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

        {newMcpToken && (
          <div className="token-reveal">
            <div>
              <strong>{t('mcp.newToken')}</strong>
              <span>{t('mcp.newTokenHelp')}</span>
            </div>
            <XCodeBlock code={newMcpToken} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />
            <XButton type="button" variant="secondary" size="sm" label={t('common.copy')} icon={<Copy size={16} />} onClick={() => void copyText(newMcpToken, t('mcp.tokenCopied'))} />
          </div>
        )}

        <div className="review-list">
          {mcpTokens.length === 0 ? (
            <EmptyState icon={Server} title={t('mcp.empty')} body={t('mcp.emptyBody')} />
          ) : (
            mcpTokens.map((token) => (
              <div className="review-row mcp-token-row" key={token.id}>
                <div>
                  <strong>{token.note || t('mcp.untitledToken')}</strong>
                  <span>{token.prefix} · {t(`mcp.principal.${token.principalType.toLowerCase()}`)} · {mcpExpiryLabel(token, t)}</span>
                  <small className="workflow-hint">{token.lastUsedAt ? t('mcp.lastUsedAt', { date: formatDate(token.lastUsedAt) }) : t('mcp.neverUsed')}</small>
                </div>
                <div className="row-actions">
                  <StatusBadge
                    tone={mcpTokenExpired(token) ? 'failed' : token.principalType === 'ADMIN' ? 'approved' : 'synced'}
                    label={mcpTokenExpired(token) ? t('mcp.expired') : t(`mcp.principal.${token.principalType.toLowerCase()}`)}
                  />
                  <XIconButton
                    className="fixed-row-icon-button"
                    type="button"
                    variant="destructive"
                    size="sm"
                    label={t('mcp.deleteToken')}
                    tooltip={t('mcp.deleteToken')}
                    icon={<Trash2 size={17} />}
                    onClick={() => void deleteToken(token.id)}
                  />
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      {isHelpOpen && <MCPHelpDialog endpoint={endpoint} onClose={() => setIsHelpOpen(false)} />}
      {isCreateOpen && (
        <MCPTokenDialog
          draft={draft}
          canCreateAdmin={canCreateAdminToken}
          onDraftChange={setDraft}
          onSubmit={createToken}
          onClose={() => setIsCreateOpen(false)}
        />
      )}
    </section>
  );
}

function mcpTokenExpired(token: MCPTokenRecord) {
  return Boolean(token.expiresAt && Date.parse(token.expiresAt) <= Date.now());
}

function mcpExpiryLabel(token: MCPTokenRecord, t: (key: string, options?: any) => string) {
  if (!token.expiresAt) return t('mcp.neverExpires');
  return mcpTokenExpired(token) ? t('mcp.expiredAt', { date: formatDate(token.expiresAt) }) : t('mcp.expiresAt', { date: formatDate(token.expiresAt) });
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

function MCPHelpDialog({ endpoint, onClose }: { endpoint: string; onClose: () => void }) {
  const { t } = useTranslation();
  const titleId = 'mcp-help-title';

  return (
    <ModalLayer onClose={onClose} width="min(760px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <section
        className="modal-panel token-help-panel"
        aria-labelledby={titleId}
      >
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <div className="token-help-head">
          <span className="install-password-icon">
            <Server size={21} />
          </span>
          <div>
            <h2 id={titleId}>{t('mcp.helpTitle')}</h2>
            <p>{t('mcp.helpBody')}</p>
          </div>
        </div>
        <div className="token-help-content">
          <TokenHelpExample title={t('mcp.helpEndpointTitle')} body={t('mcp.helpEndpointBody')} code={mcpClientConfigExample(endpoint)} language="json" />
          <TokenHelpExample title={t('mcp.helpToolsTitle')} body={t('mcp.helpToolsBody')} code={mcpToolCallExample} language="json" />
          <TokenHelpExample title={t('mcp.helpPermissionTitle')} body={t('mcp.helpPermissionBody')} code={['USER: appstore_list_my_apps, appstore_create_app_from_url, appstore_publish_version_from_url', 'ADMIN: USER tools + appstore_admin_list_apps'].join('\n')} language="plaintext" />
        </div>
      </section>
    </ModalLayer>
  );
}

function TokenHelpExample({ title, body, code, language }: { title: string; body: string; code: string; language: string }) {
  return (
    <section className="token-help-section">
      <div>
        <strong>{title}</strong>
        <span>{body}</span>
      </div>
      <XCodeBlock code={code} language={language} width="100%" size="sm" />
    </section>
  );
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

  return (
    <ModalLayer onClose={onClose} purpose="form" width="min(430px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <form
        className="modal-panel form-panel mcp-token-dialog"
        aria-labelledby={titleId}
        onSubmit={onSubmit}
      >
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <SectionTitle icon={KeyRound} title={t('mcp.createToken')} />
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
          onChange={(neverExpires) => onDraftChange({ ...draft, neverExpires })}
        />
        {!draft.neverExpires && (
          <XTextInput
            label={t('mcp.expiresAtInput')}
            value={draft.expiresAt}
            onChange={(expiresAt) => onDraftChange({ ...draft, expiresAt })}
          />
        )}
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
          <XButton type="submit" variant="primary" label={t('mcp.createToken')} icon={<KeyRound size={17} />} />
        </div>
      </form>
    </ModalLayer>
  );
}
