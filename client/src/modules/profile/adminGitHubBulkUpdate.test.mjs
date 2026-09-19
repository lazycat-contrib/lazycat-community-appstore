import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('./ProfileView.tsx', import.meta.url), 'utf8');

test('administrator app management supports selection-driven GitHub update enablement', () => {
  assert.match(source, /selectedManagedAppIDs/);
  assert.match(source, /toggleManagedAppSelection/);
  assert.match(source, /toggleAllManageableApps/);
  assert.match(source, /bulkEnableGitHubUpdates/);
  assert.match(source, /\/api\/v1\/apps\/\$\{appID\}\/github-lpk-update-policy/);
  assert.match(source, /JSON\.stringify\(\{ enabled: true, intervalMinutes: bulkGitHubUpdateIntervalHours \* 60 \}\)/);
  assert.match(source, /githubLPKUpdateIntervalHours/);
  assert.match(source, /bulkGitHubUpdateConfirmTitle/);
  assert.match(source, /bulkGitHubUpdateCompletedWithFailures/);
});
