import { useEffect, useRef, useState, type FormEvent } from 'react';
import { AlertCircle, Check, Copy, HelpCircle, KeyRound, RefreshCw, Trash2, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { EmptyState } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { TokenHelpDialog, type TokenHelpExample } from '../../shared/components/TokenHelpDialog';
import type { APITokenRecord, Toast, User } from '../../shared/types';
import { formatDate } from '../../shared/utils';

type CreatedToken = { token: string; record: APITokenRecord };

export function APITokenWorkspace({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [loadState, setLoadState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [tokenName, setTokenName] = useState('');
  const [newToken, setNewToken] = useState<CreatedToken | null>(null);
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const [isHelpOpen, setIsHelpOpen] = useState(false);
  const [tokenToDelete, setTokenToDelete] = useState<APITokenRecord | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [actionError, setActionError] = useState('');
  const mutationPending = useRef(false);

  useEffect(() => {
    const controller = new AbortController();
    setLoadState('loading');
    if (!HAS_API) {
      setLoadState('error');
      return;
    }
    void api<{ tokens: APITokenRecord[] }>('/api/v1/me/tokens', { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        setTokens([...(data.tokens || [])].sort((a, b) => b.id - a.id));
        setLoadState('ready');
      })
      .catch(() => {
        if (!controller.signal.aborted) setLoadState('error');
      });
    return () => controller.abort();
  }, [user.id, loadAttempt]);

  function openCreate() {
    setTokenName('');
    setActionError('');
    setIsCreateOpen(true);
  }

  function closeCreate() {
    if (!mutationPending.current) setIsCreateOpen(false);
  }

  function closeDelete() {
    if (!mutationPending.current) setTokenToDelete(null);
  }

  async function createToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (mutationPending.current || !tokenName.trim()) return;
    mutationPending.current = true;
    setIsSaving(true);
    setActionError('');
    try {
      const data = await api<CreatedToken>('/api/v1/me/tokens', {
        method: 'POST',
        body: JSON.stringify({ name: tokenName.trim() }),
      });
      setTokens((current) => [data.record, ...current]);
      setCopied(false);
      setCopyFailed(false);
      setNewToken(data);
      setIsCreateOpen(false);
    } catch {
      setActionError(t('token.createFailed'));
    } finally {
      mutationPending.current = false;
      setIsSaving(false);
    }
  }

  async function copyToken() {
    if (!newToken) return;
    try {
      await navigator.clipboard.writeText(newToken.token);
      setCopied(true);
      setCopyFailed(false);
      setToast({ tone: 'success', message: t('token.copied') });
    } catch {
      setCopyFailed(true);
    }
  }

  async function deleteToken(token: APITokenRecord) {
    if (mutationPending.current) return;
    mutationPending.current = true;
    setIsSaving(true);
    setActionError('');
    try {
      await api(`/api/v1/me/tokens/${token.id}`, { method: 'DELETE' });
      setTokens((current) => current.filter((item) => item.id !== token.id));
      setTokenToDelete(null);
      setToast({ tone: 'neutral', message: t('token.deleted') });
    } catch {
      setActionError(t('token.deleteFailed'));
    } finally {
      mutationPending.current = false;
      setIsSaving(false);
    }
  }

  return (
    <section className="workspace-pane api-token-workspace">
      <section className="panel">
        <div className="section-title with-action api-token-header">
          <div><KeyRound size={19} /><h2>{t('token.title')}</h2></div>
          <div className="api-token-actions">
            <XIconButton type="button" variant="ghost" label={t('token.help')} icon={<HelpCircle size={17} />} onClick={() => setIsHelpOpen(true)} />
            <XButton type="button" variant="primary" size="sm" label={t('token.generate')} icon={<KeyRound size={17} />} isDisabled={loadState !== 'ready'} onClick={openCreate} />
          </div>
        </div>
        <p className="inline-note">{t('token.description')}</p>
        {loadState === 'loading' ? (
          <p className="inline-note" role="status">{t('common.loading')}</p>
        ) : loadState === 'error' ? (
          <EmptyState icon={AlertCircle} title={t('token.loadFailed')} body={t('token.loadFailedHelp')} action={{ label: t('common.retry'), icon: RefreshCw, onClick: () => setLoadAttempt((attempt) => attempt + 1) }} />
        ) : tokens.length === 0 ? (
          <EmptyState icon={KeyRound} title={t('token.empty')} body={t('token.emptyHelp')} />
        ) : (
          <ul className="api-token-list" aria-label={t('token.title')}>
            {tokens.map((token) => (
              <li key={token.id} className="api-token-row">
                <div className="api-token-info">
                  <strong>{token.name}</strong>
                  <code>{token.prefix}…</code>
                  <div className="api-token-dates">
                    <span>{t('token.createdAt', { date: formatDate(token.createdAt || token.created_at) })}</span>
                    <span>{token.last_used_at ? t('token.lastUsedAt', { date: formatDate(token.last_used_at) }) : t('token.neverUsed')}</span>
                  </div>
                </div>
                <XButton type="button" variant="secondary" size="sm" label={t('token.revokeToken')} aria-label={t('token.revokeNamed', { name: token.name })} icon={<Trash2 size={16} />} onClick={() => { setActionError(''); setTokenToDelete(token); }} />
              </li>
            ))}
          </ul>
        )}
      </section>
      {isCreateOpen && (
        <ModalLayer onClose={closeCreate} purpose="form">
          <form className="modal-panel form-panel api-token-dialog" aria-labelledby="api-token-create-title" onSubmit={(event) => void createToken(event)}>
            <div className="section-title with-action">
              <h2 id="api-token-create-title">{t('token.generate')}</h2>
              <XIconButton type="button" label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={isSaving} onClick={closeCreate} />
            </div>
            <XTextInput label={t('token.name')} description={t('token.nameHelp')} placeholder={t('token.namePlaceholder')} value={tokenName} hasAutoFocus isRequired isDisabled={isSaving} onChange={setTokenName} />
            <p className="inline-note">{t('token.permissionsHelp')}</p>
            {actionError && <p className="api-token-error" role="alert">{actionError}</p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} isDisabled={isSaving} onClick={closeCreate} />
              <XButton type="submit" variant="primary" label={t(isSaving ? 'token.creating' : 'token.generate')} icon={<KeyRound size={17} />} isDisabled={isSaving || !tokenName.trim()} />
            </div>
          </form>
        </ModalLayer>
      )}
      {newToken && (
        <ModalLayer onClose={() => {}} purpose="required">
          <section className="modal-panel form-panel api-token-dialog" aria-labelledby="api-token-created-title">
            <div className="section-title"><Check size={19} /><h2 id="api-token-created-title">{t('token.created')}</h2></div>
            <strong className="api-token-name">{newToken.record.name}</strong>
            <p className="inline-note" id="api-token-save-help">{t('token.saveOnce')}</p>
            <label className="api-token-secret-label" htmlFor="api-token-secret">{t('token.secret')}</label>
            <textarea id="api-token-secret" className="api-token-secret" aria-describedby="api-token-save-help" readOnly spellCheck={false} rows={3} value={newToken.token} onFocus={(event) => event.target.select()} />
            {copyFailed && <p className="api-token-error" role="alert">{t('token.copyFailed')}</p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t(copied ? 'token.copied' : 'token.copy')} icon={copied ? <Check size={17} /> : <Copy size={17} />} onClick={() => void copyToken()} />
              <XButton type="button" variant="primary" label={t('token.saved')} onClick={() => setNewToken(null)} />
            </div>
          </section>
        </ModalLayer>
      )}
      {isHelpOpen && (
        <TokenHelpDialog icon={KeyRound} title={t('token.helpTitle')} body={t('token.helpBody')} titleId="token-help-title" examples={apiTokenHelpExamples(t)} onClose={() => setIsHelpOpen(false)} />
      )}
      {tokenToDelete && (
        <ModalLayer onClose={closeDelete} purpose="required">
          <section className="modal-panel form-panel api-token-dialog" aria-labelledby="api-token-delete-title">
            <div className="section-title with-action">
              <h2 id="api-token-delete-title">{t('token.deleteToken')}</h2>
              <XIconButton type="button" label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={isSaving} onClick={closeDelete} />
            </div>
            <p className="inline-note">{t('token.deleteConfirm', { name: tokenToDelete.name || tokenToDelete.prefix })}</p>
            <code>{tokenToDelete.prefix}…</code>
            {actionError && <p className="api-token-error" role="alert">{actionError}</p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} isDisabled={isSaving} onClick={closeDelete} />
              <XButton type="button" variant="destructive" label={t(isSaving ? 'token.revoking' : 'token.revokeToken')} icon={<Trash2 size={17} />} isDisabled={isSaving} onClick={() => void deleteToken(tokenToDelete)} />
            </div>
          </section>
        </ModalLayer>
      )}
    </section>
  );
}

const tokenCreateAppCurlExample = [
  'export APPSTORE_URL="https://store.example.com"',
  'export APPSTORE_TOKEN="lcst_..."',
  '',
  'curl -fsS -X POST "$APPSTORE_URL/api/v1/apps" \\',
  '  -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '  -H "Content-Type: application/json" \\',
  "  -d '{",
  '    "packageId": "cloud.lazycat.example.app",',
  '    "name": "Example App",',
  '    "summary": "Published from CI",',
  '    "version": "1.2.3",',
  '    "sourceType": "GITHUB",',
  '    "downloadUrl": "https://github.com/acme/app/releases/download/v1.2.3/app.lpk",',
  '    "sha256": "REPLACE_WITH_64_CHAR_SHA256"',
  "  }'",
].join('\n');

const tokenPublishVersionCurlExample = String.raw`export APPSTORE_URL="https://store.example.com"
export APPSTORE_TOKEN="lcst_..."
export APP_ID="123"

curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \
  -H "Authorization: Bearer $APPSTORE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.2.4",
    "changelog": "Automated release",
    "sourceType": "GITHUB",
    "downloadUrl": "https://github.com/acme/app/releases/download/v1.2.4/app.lpk"
  }'`;

const tokenGithubActionsExample = String.raw`name: Publish LPK URL

on:
  release:
    types: [published]

permissions:
  contents: read

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - name: Publish release LPK URL
        env:
          GH_TOKEN: \${{ github.token }}
          GH_REPO: \${{ github.repository }}
          RELEASE_TAG: \${{ github.event.release.tag_name }}
          RELEASE_NOTES: \${{ github.event.release.body }}
          APPSTORE_URL: \${{ secrets.APPSTORE_URL }}
          APPSTORE_TOKEN: \${{ secrets.APPSTORE_TOKEN }}
          APP_ID: \${{ secrets.APP_ID }}
        run: |
          set -euo pipefail
          lpk_url="$(gh release view "$RELEASE_TAG" --json assets --jq '[.assets[] | select(.name | endswith(".lpk"))][0].url // empty')"
          test -n "$lpk_url"
          jq -n --arg version "\${RELEASE_TAG#v}" \
            --arg url "$lpk_url" --arg notes "$RELEASE_NOTES" \
            '{version: $version, downloadUrl: $url, sourceType: "GITHUB", changelog: $notes}' \
            > release.json
          curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \
            -H "Authorization: Bearer $APPSTORE_TOKEN" \
            -H "Content-Type: application/json" \
            --data-binary @release.json`.replaceAll('\\$', '$');

function apiTokenHelpExamples(t: (key: string) => string): TokenHelpExample[] {
  return [
    { title: t('token.helpCreateAppTitle'), body: t('token.helpCreateAppBody'), code: tokenCreateAppCurlExample, language: 'bash' },
    { title: t('token.helpPublishVersionTitle'), body: t('token.helpPublishVersionBody'), code: tokenPublishVersionCurlExample, language: 'bash' },
    { title: t('token.helpGithubActionsTitle'), body: t('token.helpGithubActionsBody'), code: tokenGithubActionsExample, language: 'yaml' },
  ];
}
