import assert from 'node:assert/strict';
import test from 'node:test';
import { categoryBrowserState } from './categoryBrowserState.ts';

const categories = [
  { id: 1, name: 'Artificial Intelligence', slug: 'ai', sortOrder: 1 },
  { id: 2, name: 'Agents', slug: 'agents', parentId: 1, sortOrder: 1 },
  { id: 3, name: 'Media', slug: 'media', sortOrder: 2 },
  { id: 4, name: 'Players', slug: 'players', parentId: 3, sortOrder: 1 },
];

const nameOf = (category) => category.name;

test('default category state shows every child without selecting a parent', () => {
  const state = categoryBrowserState(categories, 'all', nameOf);

  assert.equal(state.parentValue, 'all');
  assert.equal(state.selectedParent, undefined);
  assert.deepEqual(state.railItems.map((item) => item.label), [
    'Artificial Intelligence / Agents',
    'Media / Players',
  ]);
});

test('active root or child scopes the rail to its parent', () => {
  const root = categoryBrowserState(categories, '1', nameOf);
  assert.equal(root.parentValue, '1');
  assert.equal(root.selectedParent?.id, 1);
  assert.deepEqual(root.railItems.map((item) => item.label), ['Agents']);

  const child = categoryBrowserState(categories, '2', nameOf);
  assert.equal(child.parentValue, '1');
  assert.equal(child.selectedParent?.id, 1);
  assert.deepEqual(child.railItems.map((item) => item.label), ['Agents']);
});

test('hierarchies without children keep the secondary rail empty', () => {
  const state = categoryBrowserState(categories.filter((category) => !category.parentId), 'all', nameOf);
  assert.deepEqual(state.railItems, []);
});

test('large taxonomies keep the default rail compact and expose searchable paths', () => {
	const many = [...categories, ...Array.from({ length: 13 }, (_, index) => ({ id: index + 5, name: `Extra ${index}`, slug: `extra-${index}`, sortOrder: index + 3 }))];
	const state = categoryBrowserState(many, 'all', nameOf);
	assert.equal(state.railItems.length, 0);
	assert.deepEqual(state.searchItems.slice(0, 3).map((item) => item.label), [
		'Artificial Intelligence',
		'Artificial Intelligence / Agents',
		'Media',
	]);
	const scoped = categoryBrowserState(many, '1', nameOf);
	assert.deepEqual(scoped.railItems.map((item) => item.label), ['Agents']);
});
