import { useEffect, useState } from 'react';
import { HelpCircle, KeyRound, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useTranslation } from 'react-i18next';
import { HAS_API } from '../../config';
import { api } from '../../shared/api';
import { ModalLayer } from '../../shared/components/ModalLayer';
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
        <div className="review-list">
          {tokens.map((token) => (
            <div className="review-row" key={token.id}>
              <div>
                <strong>{token.name}</strong>
                <span>{token.prefix} · {formatDate(token.createdAt || token.created_at)}</span>
              </div>
            </div>
          ))}
        </div>
        {newToken && <XCodeBlock code={newToken} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />}
        <XButton type="button" variant="secondary" label={t('token.generate')} icon={<KeyRound size={18} />} onClick={() => void createToken()} />
      </section>
      {isHelpOpen && <TokenHelpDialog onClose={() => setIsHelpOpen(false)} />}
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

function TokenHelpDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const titleId = 'token-help-title';

  return (
    <ModalLayer onClose={onClose} width="min(760px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
      <section
        className="modal-panel token-help-panel"
        aria-labelledby={titleId}
      >
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <div className="token-help-head">
          <span className="install-password-icon">
            <KeyRound size={21} />
          </span>
          <div>
            <h2 id={titleId}>{t('token.helpTitle')}</h2>
            <p>{t('token.helpBody')}</p>
          </div>
        </div>
        <div className="token-help-content">
          <TokenHelpExample title={t('token.helpCreateAppTitle')} body={t('token.helpCreateAppBody')} code={tokenCreateAppCurlExample} language="bash" />
          <TokenHelpExample title={t('token.helpPublishVersionTitle')} body={t('token.helpPublishVersionBody')} code={tokenPublishVersionCurlExample} language="bash" />
          <TokenHelpExample title={t('token.helpGithubActionsTitle')} body={t('token.helpGithubActionsBody')} code={tokenGithubActionsExample} language="yaml" />
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
