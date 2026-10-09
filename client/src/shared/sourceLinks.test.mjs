import assert from 'node:assert/strict';
import test from 'node:test';
import { publicSourceURL, repositoryURL, sourceLinks } from './sourceLinks.ts';

test('source links preserve full GitHub release and raw LPK URLs', () => {
  for (const url of ['https://github.com/acme/lpk/releases/download/v1/app.lpk?download=1', 'https://raw.githubusercontent.com/acme/lpk/main/app.lpk']) {
    assert.equal(publicSourceURL(url), url);
    assert.equal(repositoryURL(url), 'https://github.com/acme/lpk');
  }
});
test('software repository and LPK repository remain distinct', () => {
  assert.deepEqual(sourceLinks({ homepage: 'https://github.com/vendor/software', latestVersion: { downloadUrl: 'https://store.test/download', upstreamDownloadUrl: 'https://github.com/packager/lpk/releases/download/v1/app.lpk' } }), {
    softwareRepository: 'https://github.com/vendor/software', lpkRepository: 'https://github.com/packager/lpk',
  });
});
test('non-GitHub online upstream addresses stay complete; protected addresses are not invented', () => {
  assert.equal(publicSourceURL('https://cdn.test/full/app.lpk?version=1'), 'https://cdn.test/full/app.lpk?version=1');
  assert.equal(sourceLinks({ latestVersion: { downloadUrl: 'https://store.test/download' } }).lpkRepository, '');
  assert.equal(repositoryURL('https://gitlab.com/group/subgroup/project/-/releases/v1/downloads/app.lpk'), 'https://gitlab.com/group/subgroup/project');
});
test('unsafe and credential-bearing links are excluded', () => {
  for (const url of ['javascript:alert(1)', 'https://user:secret@github.com/acme/lpk', 'invalid']) assert.equal(publicSourceURL(url), '');
  assert.equal(repositoryURL('https://github.com.evil.test/acme/lpk'), '');
});

test('password protection hides the LPK repository while keeping software homepage metadata', () => {
  assert.deepEqual(sourceLinks({ installProtected: true, homepage: 'https://github.com/vendor/software', latestVersion: { downloadUrl: 'https://github.com/private/lpk/releases/download/v1/app.lpk' } }), { softwareRepository: 'https://github.com/vendor/software', lpkRepository: '' });
});
