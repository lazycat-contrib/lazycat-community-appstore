import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('./ProfileView.tsx', import.meta.url), 'utf8');

test('bulk metadata refresh is selection-driven and stays anchored in My software', () => {
  assert.match(source, /selectedOwnedAppIDs\.size > 0/);
  assert.match(source, /<XCheckboxInput/);
  assert.match(source, /<XIconButton[\s\S]*bulkRefreshSelected/);
  assert.match(source, /setBulkAction\('refresh'\)/);
  assert.match(source, /setBulkAction\('delete'\)/);
  assert.match(source, /bulkRefreshConfirmAction/);
  assert.match(source, /bulkRefreshOverwriteConfirmAction/);
  assert.match(source, /overwriteExistingMetadata: bulkRefreshOverwrite/);
  assert.match(source, /bulkDeleteConfirmAction/);
  assert.match(source, /className="bulk-metadata-refresh"/);
  assert.match(source, /purpose="required"/);
});
