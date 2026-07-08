import { useEffect, useState } from 'react';
import { HelpCircle, KeyRound, Trash2, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Text as XText } from '@astryxdesign/core/Text';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { TokenHelpDialog, type TokenHelpExample } from '../../shared/components/TokenHelpDialog';
import type { APITokenRecord, Toast, User } from '../../shared/types';
import { formatDate, runAction } from '../../shared/utils';

export function APITokenWorkspace({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [newToken, setNewToken] = useState('');
  const [isHelpOpen, setIsHelpOpen] = useState(false);
  const [tokenToDelete, setTokenToDelete] = useState<APITokenRecord | null>(null);

  useEffect(() => {
    if (!HAS_API || !user) return;
    void api<{ tokens: APITokenRecord[] }>('/api/v1/me/tokens')
      .then((data) => setTokens(data.tokens))
      .catch(() => setTokens([]));
  }, [user]);

  async function createToken() {
    await runAction(setToast, t('token.createFailed'), async () => {
      const data = await api<{ token: string; record: APITokenRecord }>('/api/v1/me/tokens', {
        method: 'POST',
        body: JSON.stringify({ name: 'CI publish token' }),
      });
      setTokens((current) => [data.record, ...current]);
      setNewToken(data.token);
    });
  }

  async function deleteToken(token: APITokenRecord) {
    await runAction(setToast, t('token.deleteFailed'), async () => {
      await api(`/api/v1/me/tokens/${token.id}`, { method: 'DELETE' });
      setTokens((current) => current.filter((item) => item.id !== token.id));
      setTokenToDelete(null);
      setToast({ tone: 'neutral', message: t('token.deleted') });
    });
  }

  return (
    <section className="workspace-pane">
      <section className="panel">
        <div className="section-title with-action">
          <div>
            <KeyRound size={19} />
            <h2>{t('token.title')}</h2>
          </div>
          <XIconButton type="button" variant="ghost" label={t('token.help')} icon={<HelpCircle size={17} />} onClick={() => setIsHelpOpen(true)} />
        </div>
        {tokens.length === 0 ? (
          <EmptyState icon={KeyRound} title={t('token.empty')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {tokens.map((token) => (
              <XListItem
                key={token.id}
                label={token.name}
                description={(
                  <XText type="supporting" display="block" wordBreak="break-word">
                    {token.prefix} · {formatDate(token.createdAt || token.created_at)}
                  </XText>
                )}
                endContent={(
                  <XButton
                    type="button"
                    variant="destructive"
                    size="sm"
                    label={t('token.revokeToken')}
                    icon={<Trash2 size={17} />}
                    onClick={() => setTokenToDelete(token)}
                  />
                )}
              />
            ))}
          </XList>
        )}
        {newToken && <XCodeBlock code={newToken} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />}
        <XButton type="button" variant="secondary" label={t('token.generate')} icon={<KeyRound size={18} />} onClick={() => void createToken()} />
      </section>
      {isHelpOpen && (
        <TokenHelpDialog
          icon={KeyRound}
          title={t('token.helpTitle')}
          body={t('token.helpBody')}
          titleId="token-help-title"
          examples={apiTokenHelpExamples(t)}
          onClose={() => setIsHelpOpen(false)}
        />
      )}
      {tokenToDelete && (
        <ModalLayer onClose={() => setTokenToDelete(null)} purpose="required">
          <div className="modal-panel form-panel confirm-dialog">
            <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setTokenToDelete(null)} />
            <SectionTitle icon={Trash2} title={t('token.deleteToken')} />
            <p className="inline-note">{t('token.deleteConfirm', { name: tokenToDelete.name || tokenToDelete.prefix })}</p>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setTokenToDelete(null)} />
              <XButton type="button" variant="destructive" label={t('token.revokeToken')} icon={<Trash2 size={17} />} onClick={() => void deleteToken(tokenToDelete)} />
            </div>
          </div>
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

const tokenPublishVersionCurlExample = [
  'export APPSTORE_URL="https://store.example.com"',
  'export APPSTORE_TOKEN="lcst_..."',
  'export APP_ID="123"',
  '',
  'curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \\',
  '  -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '  -F "version=1.2.4" \\',
  '  -F "changelog=Automated release" \\',
  '  -F "file=@dist/app.lpk"',
].join('\n');

const tokenGithubActionsExample = [
  'name: Publish LazyCat LPK',
  '',
  'on:',
  '  release:',
  '    types: [published]',
  '',
  'jobs:',
  '  publish:',
  '    runs-on: ubuntu-latest',
  '    steps:',
  '      - uses: actions/checkout@v4',
  '      - name: Build LPK',
  '        run: lzc-cli project release -o dist/app.lpk',
  '      - name: Publish version',
  '        env:',
  '          APPSTORE_URL: ${{ secrets.APPSTORE_URL }}',
  '          APPSTORE_TOKEN: ${{ secrets.APPSTORE_TOKEN }}',
  '          APP_ID: ${{ secrets.APP_ID }}',
  '        run: |',
  '          curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \\',
  '            -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '            -F "version=${GITHUB_REF_NAME#v}" \\',
  '            -F "changelog=${{ github.event.release.body }}" \\',
  '            -F "file=@dist/app.lpk"',
].join('\n');

function apiTokenHelpExamples(t: (key: string) => string): TokenHelpExample[] {
  return [
    { title: t('token.helpCreateAppTitle'), body: t('token.helpCreateAppBody'), code: tokenCreateAppCurlExample, language: 'bash' },
    { title: t('token.helpPublishVersionTitle'), body: t('token.helpPublishVersionBody'), code: tokenPublishVersionCurlExample, language: 'bash' },
    { title: t('token.helpGithubActionsTitle'), body: t('token.helpGithubActionsBody'), code: tokenGithubActionsExample, language: 'yaml' },
  ];
}
