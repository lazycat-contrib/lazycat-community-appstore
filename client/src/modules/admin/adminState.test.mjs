import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { areAdminDraftsEqual } from './adminState.ts';

const adminPanelSource = readFileSync(new URL('./AdminPanel.tsx', import.meta.url), 'utf8');
const backupPanelSource = readFileSync(new URL('./AdminBackupPanel.tsx', import.meta.url), 'utf8');
const migrationPanelSource = readFileSync(new URL('./migration/AdminMigrationPanel.tsx', import.meta.url), 'utf8');

test('areAdminDraftsEqual ignores object key insertion order', () => {
  assert.equal(
    areAdminDraftsEqual(
      { site_title: 'Store', nested: { enabled: true, count: 2 } },
      { nested: { count: 2, enabled: true }, site_title: 'Store' },
    ),
    true,
  );
});

test('areAdminDraftsEqual orders canonically equivalent Unicode keys by raw code units', () => {
  const composed = '\u00e9';
  const decomposed = 'e\u0301';

  assert.equal(
    areAdminDraftsEqual(
      { [composed]: 'composed', [decomposed]: 'decomposed' },
      { [decomposed]: 'decomposed', [composed]: 'composed' },
    ),
    true,
  );
});

test('areAdminDraftsEqual preserves array order', () => {
  assert.equal(
    areAdminDraftsEqual(
      { storageKeys: ['primary', 'archive'] },
      { storageKeys: ['archive', 'primary'] },
    ),
    false,
  );
});

test('areAdminDraftsEqual detects nested edits', () => {
  assert.equal(
    areAdminDraftsEqual(
      { targets: [{ storageKey: 'primary', directory: 'backups/appstore' }] },
      { targets: [{ storageKey: 'primary', directory: 'backups/nightly' }] },
    ),
    false,
  );
});

test('areAdminDraftsEqual treats undefined optional properties as omitted', () => {
  assert.equal(
    areAdminDraftsEqual(
      { site_title: 'Store', site_description: undefined },
      { site_title: 'Store' },
    ),
    true,
  );
});

test('areAdminDraftsEqual supports an undefined draft without treating null as absent', () => {
  assert.equal(areAdminDraftsEqual(undefined, undefined), true);
  assert.equal(areAdminDraftsEqual(undefined, null), false);
});

test('areAdminDraftsEqual rejects values outside the JSON-compatible draft boundary', () => {
  const cyclic = {};
  cyclic.self = cyclic;
  const sparse = [];
  sparse.length = 1;

  for (const value of [new Date(0), new Map(), new Set(), Number.NaN, [undefined], sparse, cyclic]) {
    assert.throws(
      () => areAdminDraftsEqual(value, value),
      /JSON-compatible admin drafts/,
    );
  }
});

test('site settings requests guard drafts with revisions, an in-flight lock, and a normalized baseline refresh', () => {
  const reloadStart = adminPanelSource.indexOf('async function reload(');
  const reloadEnd = adminPanelSource.indexOf('\n  function summarizeReviewNote', reloadStart);
  const reloadSource = adminPanelSource.slice(reloadStart, reloadEnd);
  assert.match(reloadSource, /const settingsRevision = settingsRevisionRef\.current;/);
  assert.match(reloadSource, /const settingsRequestID = \+\+settingsRequestSequenceRef\.current;/);
  assert.match(reloadSource, /settingsRequestID === settingsRequestSequenceRef\.current/);
  assert.match(reloadSource, /settingsRevision === settingsRevisionRef\.current/);
  assert.match(reloadSource, /!settingsSaveInFlightRef\.current/);

  const saveStart = adminPanelSource.indexOf('async function saveSettings(');
  const saveEnd = adminPanelSource.indexOf('\n  async function sendTestEmail', saveStart);
  const saveSource = adminPanelSource.slice(saveStart, saveEnd);
  const lockPosition = saveSource.indexOf('settingsSaveInFlightRef.current = true;');
  const patchPosition = saveSource.indexOf("await api('/api/v1/admin/settings', { method: 'PATCH'");
  assert.ok(lockPosition >= 0 && lockPosition < patchPosition, 'the synchronous lock must be acquired before PATCH');
  assert.match(saveSource, /const settingsSnapshot = settingsRef\.current;/);
  assert.match(saveSource, /body: JSON\.stringify\(settingsSnapshot\)/);
  assert.match(saveSource, /method: 'GET'/);
  assert.match(saveSource, /settingsRevision === settingsRevisionRef\.current/);
  assert.match(saveSource, /setSavedSettings\(normalizedSettings\)/);
  assert.match(saveSource, /settingsSaveInFlightRef\.current = false;/);
  assert.match(saveSource, /setToast\(\{ tone: 'neutral', message: `\$\{t\('admin\.settingsSaved'\)\}/);

  const updateStart = adminPanelSource.indexOf('function updateSetting(');
  const updateEnd = adminPanelSource.indexOf('\n  function recommendedMirrorsForSetting', updateStart);
  const updateSource = adminPanelSource.slice(updateStart, updateEnd);
  assert.match(updateSource, /settingsRevisionRef\.current \+= 1;/);
  assert.match(updateSource, /if \(!settingsSaveInFlightRef\.current\) \{\s+setSettingsSaveStatus\('dirty'\);/);
  assert.match(adminPanelSource, /isDisabled=\{settingsSaveInFlightRef\.current\}/);
});

test('storage selection syncing preserves unsaved drafts and does not erase saved feedback', () => {
  const effectStart = adminPanelSource.indexOf('const selectionChanged = storageSelectionRef.current !== selectedStorageKey;');
  const effectEnd = adminPanelSource.indexOf('\n  }, [selectedStorageKey, storageRecords]);', effectStart);
  const effectSource = adminPanelSource.slice(effectStart, effectEnd);

  assert.ok(effectStart >= 0, 'storage selection effect must track whether the selected storage changed');
  assert.match(effectSource, /storageSaveStatus !== 'dirty' && storageSaveStatus !== 'error'/);
  assert.match(effectSource, /if \(selectionChanged\) setStorageSaveStatus\('idle'\);/);
  assert.doesNotMatch(effectSource, /^\s*setStorageSaveStatus\('idle'\);/m);
});

test('admin destructive actions use named dialogs instead of click-again state', () => {
  assert.doesNotMatch(adminPanelSource, /\bconfirmDelete\b|setConfirmDelete/);
  assert.match(adminPanelSource, /type AdminDeleteTarget =/);
  assert.match(adminPanelSource, /<AdminDeleteDialog/);
  assert.match(adminPanelSource, /if \(!deleteTarget \|\| isDeleting\) return;/);
});

test('storage saves use a synchronous lock and preserve edits made while saving', () => {
  const saveStart = adminPanelSource.indexOf('async function saveStorageSettings()');
  const saveEnd = adminPanelSource.indexOf('\n  async function testStorageSettings()', saveStart);
  const saveSource = adminPanelSource.slice(saveStart, saveEnd);

  assert.match(saveSource, /startStorageAction\('save'\)/);
  assert.match(saveSource, /const storageSnapshot = storageDraftRef\.current;/);
  assert.match(saveSource, /const storageRevision = storageRevisionRef\.current;/);
  assert.match(saveSource, /storageRevision === storageRevisionRef\.current/);
  assert.match(saveSource, /setStorageSaveStatus\('dirty'\)/);
});

test('successful deletes close before refresh work that may fail independently', () => {
  const deleteStart = adminPanelSource.indexOf('async function confirmAdminDelete()');
  const deleteEnd = adminPanelSource.indexOf('\n  function deleteDialogCopy', deleteStart);
  const deleteSource = adminPanelSource.slice(deleteStart, deleteEnd);
  const closePosition = deleteSource.indexOf('setDeleteTarget(null);');
  const refreshPosition = deleteSource.indexOf('await fetchUsersPage');

  assert.ok(closePosition >= 0 && closePosition < refreshPosition);
  assert.match(deleteSource, /catch \(refreshError\)/);
});

test('backup mutations serialize and do not overwrite a newer draft or run dirty settings', () => {
  const saveStart = backupPanelSource.indexOf('async function saveSettings()');
  const saveEnd = backupPanelSource.indexOf('\n  async function runBackup()', saveStart);
  const saveSource = backupPanelSource.slice(saveStart, saveEnd);
  const runStart = saveEnd;
  const runEnd = backupPanelSource.indexOf('\n  return (', runStart);
  const runSource = backupPanelSource.slice(runStart, runEnd);

  assert.match(saveSource, /activeActionRef\.current/);
  assert.match(saveSource, /const draftSnapshot = draftRef\.current;/);
  assert.match(saveSource, /draftRevision === draftRevisionRef\.current/);
  assert.match(runSource, /areAdminDraftsEqual\(draftRef\.current, savedDraftRef\.current\)/);
  assert.doesNotMatch(runSource, /setDraft\(|setSavedDraft\(/);
});

test('migration export, preview, and import share one synchronous operation lock', () => {
  assert.match(migrationPanelSource, /const activeOperationRef = useRef/);
  assert.match(migrationPanelSource, /if \(activeOperationRef\.current\) return false;/);
  assert.match(migrationPanelSource, /if \(!startOperation\('export'\)\) return;/);
  assert.match(migrationPanelSource, /if \(!startOperation\('preview'\)\) return;/);
  assert.match(migrationPanelSource, /if \(!startOperation\('import'\)\) return;/);
  assert.match(migrationPanelSource, /isBusy=\{panelBusy\}/g);
  assert.match(migrationPanelSource, /activity\.mode === migrationImport\.mode/);
  assert.match(migrationPanelSource, /if \(activity\?\.operation === 'import'\) setActivity\(null\);/);
  assert.match(migrationPanelSource, /isRetryDisabled=\{panelBusy \|\| !canRetry\}/);
});
