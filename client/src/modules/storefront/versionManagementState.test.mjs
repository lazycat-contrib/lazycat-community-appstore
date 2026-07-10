import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { nextLatestVersion, retentionPruneCount } from './versionManagementState.ts';

test('retentionPruneCount ignores pending versions and treats zero as unlimited', () => {
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'PENDING' }],
      1,
    ),
    0,
  );
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'APPROVED' }],
      0,
    ),
    0,
  );
  assert.equal(
    retentionPruneCount(
      [{ status: 'APPROVED' }, { status: 'APPROVED' }],
      1,
    ),
    1,
  );
});

test('nextLatestVersion skips the deleted ID and non-approved versions', () => {
  const next = nextLatestVersion(
    [
      { id: 7, status: 'APPROVED', createdAt: '2026-07-07T00:00:00Z' },
      { id: 8, status: 'PENDING', createdAt: '2026-07-10T00:00:00Z' },
      { id: 9, status: 'APPROVED', createdAt: '2026-07-09T00:00:00Z' },
      { id: 10, status: 'APPROVED', createdAt: '2026-07-08T00:00:00Z' },
    ],
    9,
  );

  assert.equal(next?.id, 10);
});

test('version management dialogs apply real wrapping styles to dynamic copy', async () => {
  const source = await readFile(
    new URL('./VersionManagementDialogs.tsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /minWidth:\s*0/);
  assert.match(source, /overflowWrap:\s*'anywhere'/);
  assert.doesNotMatch(source, /wordBreak=/);
  assert.ok(
    (source.match(/style=\{wrappingTextStyle\}/g) || []).length >= 5,
    'dynamic title, summary, subject, and consequence containers must use wrappingTextStyle',
  );
});
