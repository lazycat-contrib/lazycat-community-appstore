import assert from 'node:assert/strict';
import test from 'node:test';
import { filterCollectionApps } from './collectionAppPickerState.ts';

const apps = [
  { id: 1, name: 'Hermes Studio', nameI18n: { 'zh-CN': '赫耳墨斯工作台' }, slug: 'hermes-studio', packageId: 'community.lazycat.app.hermes-studio' },
  { id: 2, name: 'Notes', nameI18n: { 'zh-CN': '笔记' }, slug: 'simple-notes', packageId: 'community.lazycat.notes' },
];

test('collection app search matches localized name, package id, and slug', () => {
  assert.deepEqual(filterCollectionApps(apps, '赫耳墨斯').map((app) => app.id), [1]);
  assert.deepEqual(filterCollectionApps(apps, 'HERMES-STUDIO').map((app) => app.id), [1]);
  assert.deepEqual(filterCollectionApps(apps, 'simple-notes').map((app) => app.id), [2]);
});

test('collection app search trims queries and preserves the full list when empty', () => {
  assert.deepEqual(filterCollectionApps(apps, '  notes  ').map((app) => app.id), [2]);
  assert.equal(filterCollectionApps(apps, ''), apps);
});
