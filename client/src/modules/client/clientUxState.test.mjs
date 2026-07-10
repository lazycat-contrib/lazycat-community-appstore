import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildInstallTimeline,
  findStableSourceApp,
  normalizeEditableClientSettings,
  sameEditableClientSettings,
} from './clientUxState.ts';

test('install timeline exposes the system step without inventing determinate progress', () => {
  assert.deepEqual(
    buildInstallTimeline({ status: 'running', stageKey: 'installActivity.stageSystem' }),
    [
      { key: 'queued', state: 'complete' },
      { key: 'prepare', state: 'complete' },
      { key: 'system', state: 'current' },
      { key: 'result', state: 'pending' },
    ],
  );
});

test('failed installs leave the result stage in error state', () => {
  assert.deepEqual(buildInstallTimeline({ status: 'error', stageKey: 'installActivity.stageFailed' }).at(-1), {
    key: 'result',
    state: 'error',
  });
});

test('editable settings comparison ignores server-owned sync result fields', () => {
  const base = {
    clientTitle: 'MiaoMiao',
    commentDisplayName: 'Cat',
    defaultPageSize: 24,
    autoSyncEnabled: true,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
    installSuccessDismissSeconds: 3,
    lastAutoSyncAt: '2026-07-10T00:00:00Z',
    lastAutoSyncStatus: 'success',
  };
  assert.equal(
    sameEditableClientSettings(base, { ...base, lastAutoSyncAt: '2026-07-11T00:00:00Z', lastAutoSyncStatus: 'partial' }),
    true,
  );
});

test('editable settings normalization trims strings and applies numeric defaults', () => {
  assert.deepEqual(
    normalizeEditableClientSettings({
      clientTitle: '  MiaoMiao  ',
      commentDisplayName: '  Cat  ',
      defaultPageSize: 0,
      autoSyncEnabled: true,
      autoSyncIntervalMinutes: 0,
      syncOnStartup: true,
      installSuccessDismissSeconds: Number.NaN,
    }),
    {
      clientTitle: 'MiaoMiao',
      commentDisplayName: 'Cat',
      defaultPageSize: 24,
      autoSyncEnabled: true,
      autoSyncIntervalMinutes: 60,
      syncOnStartup: true,
      installSuccessDismissSeconds: 3,
    },
  );
});

test('legacy handoff and verify keys map to the real system-owned phase', () => {
  for (const stageKey of ['installActivity.stageHandoff', 'installActivity.stageVerify']) {
    assert.equal(
      buildInstallTimeline({ status: 'running', stageKey }).find((item) => item.state === 'current')?.key,
      'system',
    );
  }
});

test('editable settings comparison detects user-owned changes', () => {
  const base = {
    clientTitle: '',
    commentDisplayName: '',
    defaultPageSize: 24,
    autoSyncEnabled: false,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
    installSuccessDismissSeconds: 3,
  };
  assert.equal(sameEditableClientSettings(base, { ...base, syncOnStartup: true }), false);
});

test('installed app source matching requires package or slug identity and never falls back to title', () => {
  const sourceApps = [
    { id: 1, packageId: 'cloud.lazycat.notes', slug: 'notes', name: 'Notes' },
    { id: 2, packageId: 'cloud.lazycat.other', slug: 'other', name: 'Same title' },
  ];
  assert.equal(findStableSourceApp({ appid: 'cloud.lazycat.notes', title: 'Anything' }, sourceApps)?.id, 1);
  assert.equal(findStableSourceApp({ appid: 'notes', title: 'Anything' }, sourceApps)?.id, 1);
  assert.equal(findStableSourceApp({ title: 'Same title' }, sourceApps), undefined);
  assert.equal(findStableSourceApp({ appid: 'unknown', title: 'Notes' }, sourceApps), undefined);
});
