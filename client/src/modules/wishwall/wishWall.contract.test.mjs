import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { hasNextWishPage, mergeWishPage } from './wishWallState.ts';

const wallSource = readFileSync(new URL('./WishWall.tsx', import.meta.url), 'utf8');
const navigationSource = readFileSync(new URL('../shell/navigation.ts', import.meta.url), 'utf8');
const catalogSource = readFileSync(new URL('../client/ClientCatalog.tsx', import.meta.url), 'utf8');
const adminSource = readFileSync(new URL('../admin/DownstreamClientsPanel.tsx', import.meta.url), 'utf8');

test('wish wall is a top-level destination and a client catalog action', () => {
  assert.match(navigationSource, /key: 'wishwall'/);
  assert.match(catalogSource, /onGoWishWall/);
});

test('client wish wall supports create, edit, delete, and source selection', () => {
  assert.match(wallSource, /method: 'PATCH'/);
  assert.match(wallSource, /method: 'DELETE'/);
  assert.match(wallSource, /wishWallAvailable/);
  assert.match(wallSource, /statusText/);
  assert.match(wallSource, /wish-maintenance-list/);
  assert.match(wallSource, /const clientEditing = mode === 'client' && editingID !== null/);
  assert.match(wallSource, /!clientEditing && <>/);
});

test('server wish details reflow inside narrow cards', async () => {
  const styles = readFileSync(new URL('../../styles/components.css', import.meta.url), 'utf8');
  assert.match(styles, /container-type:\s*inline-size/);
  assert.match(styles, /@container \(max-width:\s*300px\)/);
  assert.match(styles, /\.wish-timeline > div\s*\{\s*grid-template-columns:\s*minmax\(0,\s*1fr\)/s);
});

test('server wish wall provides admin replies, status notes, and block action', () => {
  assert.match(wallSource, /admin\/wishes\/\$\{item\.id\}\/replies/);
  assert.match(wallSource, /admin\/wishes\/\$\{item\.id\}\/status/);
  assert.match(wallSource, /admin\/downstream-clients/);
  assert.match(adminSource, /\/unblock/);
  assert.match(wallSource, /wish-board/);
});

test('wish wall uses icon filters and observer-driven pagination', () => {
  assert.match(wallSource, /kindIcons/);
  assert.match(wallSource, /statusIcons/);
  assert.match(wallSource, /IntersectionObserver/);
  assert.match(wallSource, /pageSize: String\(PAGE_SIZE\)/);
  assert.match(wallSource, /loadMoreSentinel/);
});

test('wish wall pagination appends uniquely and stops on the final page', () => {
  const current = [{ id: 1, title: 'old' }, { id: 2, title: 'stale' }];
  const incoming = [{ id: 2, title: 'fresh' }, { id: 3, title: 'new' }];

  assert.deepEqual(mergeWishPage(current, incoming, true), [
    { id: 1, title: 'old' },
    { id: 2, title: 'fresh' },
    { id: 3, title: 'new' },
  ]);
  assert.deepEqual(mergeWishPage(current, incoming, false), incoming);
  assert.equal(hasNextWishPage(1, 2), true);
  assert.equal(hasNextWishPage(2, 2), false);
});
