import assert from 'node:assert/strict';
import test from 'node:test';
import { authenticationDestination, needsClientCatalog, readShellRoute, shellRouteURL } from './routes.ts';

test('server and client landing pages have distinct defaults', () => {
  assert.equal(readShellRoute('/', true).tab, 'home');
  assert.equal(readShellRoute('/', false).tab, 'search');
  assert.equal(readShellRoute('/?view=unknown', false).tab, 'search');
});

test('detail URLs retain the list destination and source identity', () => {
  const route = { tab: 'search', appId: 31, sourceId: 'community / private', mode: 'detail' };
  assert.deepEqual(readShellRoute(shellRouteURL(route), false), route);
  assert.equal(readShellRoute(shellRouteURL({ tab: 'profile', appId: 12, mode: 'manage' }), true).mode, 'manage');
});

test('invalid app identifiers cannot issue detail requests', () => {
  for (const value of ['0', '-1', '3.2', '3garbage', 'Infinity', '9007199254740992']) {
    assert.equal(readShellRoute('/?view=search&app='+value, true).appId, undefined);
  }
});

test('legacy login return destinations still resolve to the intended page', () => {
  assert.equal(readShellRoute('/admin', true).tab, 'admin');
  assert.equal(readShellRoute('/profile', true).tab, 'profile');
  assert.equal(readShellRoute('/login?view=admin&app=1', true).appId, undefined);
});

test('submission and administrator login intents have URL-backed destinations', () => {
  assert.equal(readShellRoute(authenticationDestination('/', 'submit', false), true).tab, 'profile');
  assert.equal(readShellRoute(authenticationDestination('/', 'admin', true), true).tab, 'admin');
  assert.equal(authenticationDestination('/?view=search&app=31', 'admin', false), '/?view=search&app=31');
});
test('ordinary login preserves the full return destination', () => {
  const destination = '/?view=search&app=31&mode=detail';
  assert.equal(authenticationDestination(destination, null, true), destination);
});

test('direct installed-app visits load source metadata for update matching', () => {
  assert.equal(needsClientCatalog('profile'), true);
  assert.equal(needsClientCatalog('history'), true);
  assert.equal(needsClientCatalog('search'), true);
  assert.equal(needsClientCatalog('settings'), false);
});
