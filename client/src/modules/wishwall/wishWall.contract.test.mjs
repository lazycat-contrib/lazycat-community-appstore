import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

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
});

test('server wish wall provides admin replies, status notes, and block action', () => {
  assert.match(wallSource, /admin\/wishes\/\$\{item\.id\}\/replies/);
  assert.match(wallSource, /admin\/wishes\/\$\{item\.id\}\/status/);
  assert.match(wallSource, /admin\/downstream-clients/);
  assert.match(adminSource, /\/unblock/);
});
