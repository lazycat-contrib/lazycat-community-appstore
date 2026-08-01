import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { softwareUpdatedAt } from '../../shared/catalogUpdate.ts';
import {
  autoUpdatePolicyPresentation,
  buildUpdateCandidateSnapshot,
  buildInstallTimeline,
  buildUpdateConfirmation,
  CLIENT_PACKAGE_ID,
  findStableSourceApp,
  installTaskProgress,
  installTaskState,
	installedRuntimeStatusPresentation,
	inspectionPresentation,
  normalizeEditableClientSettings,
  normalizeAutomationSettings,
  orderClientUpdateRowsLast,
  requiresInstallOptions,
  sameEditableClientSettings,
  sortClientCatalogApps,
} from './clientUxState.ts';

async function source(relativePath) {
  return readFile(new URL(relativePath, import.meta.url), 'utf8');
}

test('install mirror helpers consume the shared mirror config contract', async () => {
  const [types, utils] = await Promise.all([
    source('../../shared/types.ts'),
    source('../../shared/utils.ts'),
  ]);

  assert.match(types, /export type GitHubMirrorOption = Pick<GitHubMirror, 'id' \| 'kind' \| 'name'>/);
  assert.match(types, /export type InstallMirrorConfig = \{[\s\S]*githubMirrors: GitHubMirrorOption\[\][\s\S]*defaultDownloadMirrorId: string[\s\S]*defaultRawMirrorId: string[\s\S]*\}/);
  assert.match(utils, /applicableMirrorsForVersion\(mirrorConfig: InstallMirrorConfig \| undefined/);
  assert.match(utils, /arrayOrEmpty\(mirrorConfig\.githubMirrors\)\.filter\(\(entry\) => entry\.kind === kind\)/);
  assert.match(utils, /defaultMirrorIDForVersion\(mirrorConfig: InstallMirrorConfig \| undefined/);
  assert.match(utils, /kind === 'raw' \? mirrorConfig\.defaultRawMirrorId \|\| '' : mirrorConfig\.defaultDownloadMirrorId \|\| ''/);
  assert.match(utils, /const parsed = new URL\(value\)/);
  assert.match(utils, /parsed\.hostname\.toLowerCase\(\)/);
  assert.doesNotMatch(utils, /value\.includes\('github\.com\/'\)/);
});

test('install options dialog consumes a shared mirror config', async () => {
  const dialog = await source('./InstallOptionsDialog.tsx');

  assert.match(dialog, /mirrorConfig\?: InstallMirrorConfig/);
  assert.match(dialog, /applicableMirrorsForVersion\(mirrorConfig, version\)/);
  assert.match(dialog, /defaultMirrorIDForVersion\(mirrorConfig, version\)/);
  assert.doesNotMatch(dialog, /source\?: SourceSubscription/);
});

test('client catalog recent sorting uses published software time and stable fallbacks', () => {
  const apps = [
    { id: 1, name: 'Missing', sourceName: 'B' },
    { id: 2, name: 'Beta', sourceName: 'A', updatedAt: '2026-07-15T00:00:00Z', latestVersion: { publishedAt: '2026-07-17T00:00:00Z' } },
    { id: 3, name: 'Alpha', sourceName: 'B', updatedAt: '2026-07-15T00:00:00Z' },
    { id: 4, name: 'Newest', sourceName: 'B', updatedAt: '2026-07-16T00:00:00Z', latestVersion: { publishedAt: '2026-07-14T00:00:00Z' } },
    { id: 5, name: 'Invalid', sourceName: 'A', updatedAt: 'not-a-date' },
  ];

  assert.deepEqual(
    sortClientCatalogApps(apps, 'recent', (app) => app.name).map((app) => app.id),
    [2, 3, 4, 5, 1],
  );
  assert.equal(softwareUpdatedAt(apps[1]), '2026-07-17T00:00:00Z');
  assert.equal(softwareUpdatedAt(apps[2]), '2026-07-15T00:00:00Z');
  assert.deepEqual(sortClientCatalogApps(apps, 'name', (app) => app.name).map((app) => app.id), [3, 2, 5, 1, 4]);
  assert.deepEqual(sortClientCatalogApps(apps, 'source', (app) => app.name).map((app) => app.id), [2, 5, 3, 1, 4]);
});

test('client catalog download sorting uses stable recent and name fallbacks', () => {
  const apps = [
    { id: 1, name: 'Zulu', sourceName: 'B', downloadCount: 3, updatedAt: '2026-07-20T00:00:00Z' },
    { id: 2, name: 'Beta', sourceName: 'A', downloadCount: 9, updatedAt: '2026-07-18T00:00:00Z' },
    { id: 3, name: 'Alpha', sourceName: 'A', downloadCount: 9, updatedAt: '2026-07-19T00:00:00Z' },
    { id: 4, name: 'Missing', sourceName: 'C' },
  ];

  assert.deepEqual(
    sortClientCatalogApps(apps, 'downloads', (app) => app.name).map((app) => app.id),
    [3, 2, 1, 4],
  );
});

test('client catalog exposes the recently updated sort option', async () => {
  const catalog = await source('./ClientCatalog.tsx');

  assert.match(catalog, /\{ value: 'recent', label: t\('search\.recent'\) \}/);
  assert.match(catalog, /\{ value: 'downloads', label: t\('search\.downloads'\) \}/);
  assert.doesNotMatch(catalog, /value: 'default'/);
  assert.match(catalog, /sortClientCatalogApps\(filtered, sortMode, localizedAppName\)/);
});

test('client catalog defaults to recent sorting', async () => {
  const app = await source('../../App.tsx');
  assert.match(app, /selectedCategoryFilter: 'all',[\s\S]*sortMode: 'recent'/);
});

test('client automatically loads installed applications after authentication', async () => {
  const app = await source('../../App.tsx');

  assert.match(app, /await Promise\.all\(\[loadClientSources\(\), loadClientSettings\(\)\]\);/);
  assert.match(app, /void loadInstalledApps\(\{ quiet: true \}\);/);
  assert.doesNotMatch(app, /Promise\.all\(\[loadClientSources\(\), loadClientSettings\(\), loadInstalledApps/);
  assert.match(app, /installedRequestRef\.current !== requestID/);
  assert.match(app, /storeAppsRequestRef\.current !== requestID/);
  assert.match(app, /if \(!HAS_API \|\| loading \|\| setupRequired \|\| isLoginRoute \|\| storeAppsComplete\) return;/);
  assert.match(app, /tab !== 'home' && tab !== 'search' && tab !== 'profile'/);
});

test('client catalog browsing state survives opening and closing app details', async () => {
  const [app, searchView, catalog] = await Promise.all([
    source('../../App.tsx'),
    source('../search/SearchView.tsx'),
    source('./ClientCatalog.tsx'),
  ]);

  assert.match(app, /useState<ClientCatalogViewState>/);
  assert.match(app, /clientCatalogState=\{clientCatalogState\}/);
  assert.match(app, /onClientCatalogStateChange=\{setClientCatalogState\}/);
  assert.match(searchView, /clientCatalogState: ClientCatalogViewState/);
  assert.match(searchView, /onClientCatalogStateChange: Dispatch<SetStateAction<ClientCatalogViewState>>/);
  assert.match(catalog, /viewState: ClientCatalogViewState/);
  assert.match(catalog, /onViewStateChange: Dispatch<SetStateAction<ClientCatalogViewState>>/);
  assert.doesNotMatch(catalog, /const \[page, setPage\] = useState\(1\)/);
  assert.doesNotMatch(catalog, /const \[pageSize, setPageSize\] = useState/);
});

test('installed app automatic update policy defaults to enabled', () => {
  assert.deepEqual(autoUpdatePolicyPresentation(undefined), { enabled: true, state: 'automatic' });
  assert.deepEqual(autoUpdatePolicyPresentation(false), { enabled: false, state: 'manualOnly' });
});

test('installed app runtime status hides LazyCat SDK enum formatting', () => {
  assert.deepEqual(installedRuntimeStatusPresentation('Status_Running'), { key: 'running', raw: 'Status_Running' });
  assert.deepEqual(installedRuntimeStatusPresentation('STATUS_PAUSED'), { key: 'paused', raw: 'STATUS_PAUSED' });
  assert.deepEqual(installedRuntimeStatusPresentation('instance-status-stopped'), { key: 'stopped', raw: 'instance-status-stopped' });
  assert.deepEqual(installedRuntimeStatusPresentation('updating'), { key: 'processing', raw: 'updating' });
  assert.deepEqual(installedRuntimeStatusPresentation('install_err'), { key: 'error', raw: 'install_err' });
  assert.deepEqual(installedRuntimeStatusPresentation('Future_State'), { key: 'unknown', raw: 'Future_State' });
  assert.deepEqual(installedRuntimeStatusPresentation(''), { key: 'unknown', raw: '' });
});

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

test('LazyCat task status maps only SDK terminal states to terminal UI states', () => {
  assert.deepEqual(installTaskState({ taskId: 'task-1', status: 'DOWNLOADING' }), {
    status: 'running', stageKey: 'installActivity.stageSystem', isTerminal: false,
  });
  assert.deepEqual(installTaskState({ taskId: 'task-1', status: 'INSTALL_OK' }), {
    status: 'success', stageKey: 'installActivity.stageDone', isTerminal: true,
  });
  assert.deepEqual(installTaskState({ taskId: 'task-1', status: 'INSTALL_ERR' }), {
    status: 'error', stageKey: 'installActivity.stageFailed', isTerminal: true,
  });
});

test('install task progress stays indeterminate until LazyCat provides a total size', () => {
  assert.deepEqual(installTaskProgress({ taskId: 'task-1', status: 'DOWNLOADING', downloadedSize: 10 }), { progress: 0, progressKnown: false });
  assert.deepEqual(installTaskProgress({ taskId: 'task-1', status: 'DOWNLOADING', downloadedSize: 25, totalSize: 100 }), { progress: 25, progressKnown: true });
});

test('LPK inspection states expose terminal and active UI semantics', () => {
	assert.deepEqual(inspectionPresentation('PENDING'), { stateKey: 'pending', isActive: true, statusVariant: 'accent' });
	assert.deepEqual(inspectionPresentation('RUNNING'), { stateKey: 'running', isActive: true, statusVariant: 'accent' });
	assert.deepEqual(inspectionPresentation('SUCCEEDED'), { stateKey: 'succeeded', isActive: false, statusVariant: 'success' });
	assert.deepEqual(inspectionPresentation('TIMED_OUT'), { stateKey: 'timedOut', isActive: false, statusVariant: 'error' });
	assert.deepEqual(inspectionPresentation('unknown'), { stateKey: 'unknown', isActive: false, statusVariant: 'neutral' });
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
    autoUpdateNotifyEnabled: true,
    lastAutoSyncAt: '2026-07-10T00:00:00Z',
    lastAutoSyncStatus: 'success',
    autoUpdateEnabled: true,
    autoUpdateIntervalMinutes: 60,
    lastAutoUpdateAt: '2026-07-10T00:00:00Z',
    lastAutoUpdateStatus: 'success',
  };
  assert.equal(
    sameEditableClientSettings(base, {
      ...base,
      lastAutoSyncAt: '2026-07-11T00:00:00Z',
      lastAutoSyncStatus: 'partial',
      lastAutoUpdateAt: '2026-07-11T00:00:00Z',
      lastAutoUpdateStatus: 'skipped',
    }),
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
      autoUpdateNotifyEnabled: true,
      autoUpdateEnabled: true,
      autoUpdateIntervalMinutes: 0,
    }),
    {
      clientTitle: 'MiaoMiao',
      commentDisplayName: 'Cat',
      defaultPageSize: 24,
      autoSyncEnabled: true,
      autoSyncIntervalMinutes: 60,
      syncOnStartup: true,
      installSuccessDismissSeconds: 3,
      autoUpdateNotifyEnabled: true,
      autoUpdateEnabled: true,
      autoUpdateIntervalMinutes: 60,
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
    autoUpdateNotifyEnabled: true,
    autoUpdateEnabled: false,
    autoUpdateIntervalMinutes: 60,
  };
  assert.equal(sameEditableClientSettings(base, { ...base, syncOnStartup: true }), false);
  assert.equal(sameEditableClientSettings(base, { ...base, autoUpdateNotifyEnabled: false }), false);
});

test('install settings expose the LazyCat automatic update notification option', async () => {
  const [view, zh, en, manifest] = await Promise.all([
    source('./ClientSettingsView.tsx'),
    source('../../locales/zh.ts'),
    source('../../locales/en.ts'),
    source('../../../../lazycat/client/package.yml'),
  ]);
  assert.match(view, /autoUpdateNotifyEnabled/);
  assert.match(zh, /自动更新完成后通知/);
  assert.match(en, /Notify after automatic updates/);
  assert.match(manifest, /min_os_version:\s*1\.6\.0/);
  assert.match(manifest, /- user\.notify/);
});

test('bulk update confirmation excludes password-protected applications', () => {
  assert.deepEqual(
    buildUpdateConfirmation([
      { item: { appid: 'eligible' }, source: { packageId: 'eligible', installProtected: false } },
      { item: { appid: 'protected' }, source: { packageId: 'protected', installProtected: true } },
    ]),
    { eligible: ['eligible'], skipped: ['protected'] },
  );
});

test('bulk update candidate snapshot matches the visible confirmation rows', () => {
  assert.deepEqual(buildUpdateCandidateSnapshot([
    { item: { appid: 'first', version: '1.0.0' }, source: { id: 11, sourceId: 2, packageId: 'first', latestVersion: { version: '2.0.0' } } },
    { item: { appid: 'protected', version: '1.0.0' }, source: { id: 12, sourceId: 2, packageId: 'protected', installProtected: true, latestVersion: { version: '2.0.0' } } },
  ]), [{ appId: 11, sourceId: 2, packageId: 'first', installedVersion: '1.0.0', targetVersion: '2.0.0' }]);
});

test('bulk update order keeps the client itself last', () => {
  const rows = [
    { item: { appid: 'community.lazycat.app-store' }, source: { packageId: 'community.lazycat.app-store' } },
    { item: { appid: 'notes' }, source: { packageId: 'notes' } },
    { item: { appid: 'photos' }, source: { packageId: 'photos' } },
  ];
  assert.deepEqual(orderClientUpdateRowsLast(rows).map((row) => row.item.appid), [
    'notes',
    'photos',
    'community.lazycat.app-store',
  ]);
  assert.deepEqual(rows.map((row) => row.item.appid), [
    'community.lazycat.app-store',
    'notes',
    'photos',
  ]);
});

test('client self-update identity stays aligned with the package manifest and executor', async () => {
  const [manifest, executor] = await Promise.all([
    source('../../../../lazycat/client/package.yml'),
    source('../../../../internal/clientserver/update_queue.go'),
  ]);
  const manifestPackageID = manifest.match(/^package:\s*(\S+)$/m)?.[1];
  assert.equal(CLIENT_PACKAGE_ID, manifestPackageID);
  assert.match(executor, new RegExp(`const clientPackageID = ${JSON.stringify(CLIENT_PACKAGE_ID)}`));
});

test('install options open only when the dialog has a password or mirror control', () => {
  assert.equal(requiresInstallOptions({ installProtected: false, mirrorOptionCount: 0 }), false);
  assert.equal(requiresInstallOptions({ installProtected: true, mirrorOptionCount: 0 }), true);
  assert.equal(requiresInstallOptions({ installProtected: false, mirrorOptionCount: 1 }), true);
  assert.equal(requiresInstallOptions({ installProtected: false, mirrorOptionCount: 1, confirmed: true }), false);
  assert.equal(requiresInstallOptions({ installProtected: true, installPassword: 'secret', mirrorOptionCount: 0 }), false);
});

test('automatic updates require source sync and a fresh-enough sync interval', () => {
  assert.deepEqual(normalizeAutomationSettings({ autoSyncEnabled: false, autoSyncIntervalMinutes: 60, autoUpdateEnabled: true, autoUpdateIntervalMinutes: 15 }), {
    autoSyncEnabled: true,
    autoSyncIntervalMinutes: 15,
    autoUpdateEnabled: true,
    autoUpdateIntervalMinutes: 15,
  });
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
