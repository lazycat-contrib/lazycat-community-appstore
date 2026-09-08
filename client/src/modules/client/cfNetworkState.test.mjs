import assert from 'node:assert/strict';
import test from 'node:test';
import { validCFEndpointSyntax } from './cfNetworkState.ts';
import { normalizeEditableClientSettings, sameEditableClientSettings } from './clientUxState.ts';

test('preferred addresses accept IPs and domains but not URLs or ports', () => {
  for (const value of ['saas.sin.fan', '  SAAS.SIN.FAN. ', '1.1.1.1', '2606:4700:4700::1111']) assert.equal(validCFEndpointSyntax(value), true, value);
  for (const value of ['', 'https://saas.sin.fan', 'saas.sin.fan:443', 'a..example', '-bad.example', 'saas.sin.fan/path', '1.2.3.999', 'localhost']) assert.equal(validCFEndpointSyntax(value), false, value);
});

test('network defaults remain optional; changes participate in dirty state and save payload', () => {
  const base = { clientTitle: '', commentDisplayName: '' };
  const normal = normalizeEditableClientSettings(base);
  assert.equal(normal.cfEnabled, false);
  assert.equal(normal.cfEndpoint, 'saas.sin.fan');
  assert.equal(sameEditableClientSettings(base, { ...base, cfEnabled: true }), false);
  assert.equal(sameEditableClientSettings(base, { ...base, cfEndpoint: '1.1.1.1' }), false);
  assert.equal(sameEditableClientSettings(base, { ...base, cfPresets: [{ endpoint: '1.1.1.1' }] }), true);
  assert.equal(normalizeEditableClientSettings({ ...base, cfEndpoint: ' SAAS.SIN.FAN. ' }).cfEndpoint, 'saas.sin.fan');
});
