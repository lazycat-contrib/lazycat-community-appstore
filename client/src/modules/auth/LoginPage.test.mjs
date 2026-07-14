import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const loginPageSource = readFileSync(new URL('./LoginPage.tsx', import.meta.url), 'utf8');

test('registration request sends only fields accepted by RegisterRequest', () => {
  const registerStart = loginPageSource.indexOf("await runAction(setToast, t('auth.registerFailed')");
  const registerEnd = loginPageSource.indexOf('\n      setUser(data.user);', registerStart);
  const registerSource = loginPageSource.slice(registerStart, registerEnd);

  assert.ok(registerStart >= 0 && registerEnd > registerStart, 'registration request block must exist');
  assert.match(
    registerSource,
    /body: JSON\.stringify\(\{\s*username: submittedForm\.username,\s*email: submittedForm\.email,\s*password: submittedForm\.password,\s*inviteCode: submittedForm\.inviteCode,?\s*\}\)/,
  );
  assert.doesNotMatch(registerSource, /totpCode/);
});
