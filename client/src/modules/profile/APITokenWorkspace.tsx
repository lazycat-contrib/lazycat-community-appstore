import { useEffect, useState } from 'react';
import { HelpCircle, KeyRound } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Text as XText } from '@astryxdesign/core/Text';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { EmptyState } from '../../shared/components/Feedback';
import { TokenHelpDialog, type TokenHelpExample } from '../../shared/components/TokenHelpDialog';
import type { APITokenRecord, Toast, User } from '../../shared/types';
import { formatDate, runAction } from '../../shared/utils';

export function APITokenWorkspace({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [newToken, setNewToken] = useState('');
  const [isHelpOpen, setIsHelpOpen] = useState(false);

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
