import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { normalizeSourceIdentity, sourceColor, sourceColorMap, sourceDisplayURL } from './sourceIdentity.ts';

test('identity normalizes host case, default ports, trailing slashes, fragments and surrounding whitespace', () => {
  for (const value of [
    'https://example.com/source',
    ' HTTPS://EXAMPLE.COM:443/source/ ',
    'https://example.com/source///#section',
    'https://user:secret@example.com/source#token=secret',
  ]) {
    assert.equal(normalizeSourceIdentity(value), 'https://example.com/source');
    assert.equal(sourceColor(value), 'hsl(265 48% 44%)');
  }
  assert.equal(normalizeSourceIdentity('http://EXAMPLE.COM:80/'), 'http://example.com/');
  assert.equal(normalizeSourceIdentity('https://example.com'), 'https://example.com/');
  assert.equal(normalizeSourceIdentity('https://[2001:DB8::1]:443/source/'), 'https://[2001:db8::1]/source');
  assert.equal(normalizeSourceIdentity('https://example.com/a/../source'), 'https://example.com/source');
});

test('scheme, non-default ports, path case and query values remain part of source identity', () => {
  const base = 'https://example.com/source';
  for (const value of ['http://example.com/source', 'https://example.com:8443/source', 'https://example.com/Source', `${base}?channel=beta`]) {
    assert.notEqual(normalizeSourceIdentity(value), normalizeSourceIdentity(base));
    assert.notEqual(sourceColor(value), sourceColor(base));
  }
  assert.equal(normalizeSourceIdentity(`${base}?b=2&a=1#fragment`), `${base}?a=1&b=2`);
  assert.equal(sourceColor(`${base}?b=2&a=1`), sourceColor(`${base}?a=1&b=2`));
  assert.notEqual(normalizeSourceIdentity(`${base}?a=1&a=2`), normalizeSourceIdentity(`${base}?a=2&a=1`));
});

test('display URL never exposes credentials, query parameters or fragments', () => {
  const secretURL = 'https://alice:password@EXAMPLE.com:443/source/?token=private&password=secret#access-key';
  assert.equal(sourceDisplayURL(secretURL), 'https://example.com/source/');
  assert.doesNotMatch(sourceColor(secretURL), /alice|password|private|secret|access-key/);
});

test('invalid and unsafe URLs produce no color or display text', () => {
  for (const value of ['', ' ', 'not a url?token=secret', '/relative', 'javascript:alert(1)', 'file:///tmp/source', 'https://', 'ftp://example.com']) {
    assert.equal(normalizeSourceIdentity(value), '');
    assert.equal(sourceColor(value), undefined);
    assert.equal(sourceDisplayURL(value), '');
  }
});

test('source ID mapping survives rename, sync, reordering, reload and equivalent URLs', () => {
  const sources = [
    { id: 7, name: 'Same name', url: 'https://example.com/source' },
    { id: 'opaque-id', name: 'Same name', url: 'https://another.example/source' },
  ];
  const colors = sourceColorMap(sources);
  const reloaded = JSON.parse(JSON.stringify(sources)).reverse().map((source) => ({ ...source, name: 'Renamed', lastSync: '2026-10-22' }));
  for (const source of reloaded) assert.equal(sourceColorMap(reloaded).get(String(source.id)), colors.get(String(source.id)));
  assert.equal(sourceColorMap([{ id: 7, url: 'https://EXAMPLE.COM:443/source/#sync' }]).get('7'), colors.get('7'));
  assert.notEqual(colors.get('7'), colors.get('opaque-id'));
  assert.equal(colors.get('Same name'), undefined);
  assert.equal(colors.get('deleted-id'), undefined);
  assert.equal(sourceColorMap([{ id: 0, url: sources[0].url }]).get('0'), colors.get('7'));
  assert.equal(sourceColorMap([{ id: 7, url: 'invalid' }]).size, 0);
});

test('cards and source rows share URL-derived colors while preserving source names and safe tooltips', async () => {
  const read = (path) => readFile(new URL(path, import.meta.url), 'utf8');
  const [grid, catalog, installed, row, sources] = await Promise.all([
    read('./SourceAppGrid.tsx'), read('./ClientCatalog.tsx'), read('./InstalledAppsView.tsx'), read('./SourceStatusRow.tsx'), read('./SourcesView.tsx'),
  ]);
  assert.match(catalog, /<SourceAppGrid\s+apps=\{pagedSourceApps\}\s+sources=\{sources\}/);
  for (const component of [grid, installed]) {
    assert.match(component, /sourceColorMap\(sources\)/);
    assert.match(component, /colors\.get\(String\(/);
    assert.match(component, /source-identity-card/);
    assert.match(component, /source-identity-dot" aria-hidden="true"/);
    assert.match(component, /sourceName/);
  }
  assert.match(row, /sourceColor\(source.url\)/);
  assert.match(row, /<strong>\{color && <span className="source-identity-dot"[^\n]+\{source.name\}<\/strong>/);
  assert.match(row, /title=\{displayURL\}>\{displayURL\}/);
  assert.doesNotMatch(row, /title=\{source.url\}|>\{source.url\}</);
  assert.match(sources, /sourceColor\(normalizedDraftURL\)/);
  assert.match(sources, /sourceColor\(editDraft.url\)/);
  assert.match(sources, /sourceDisplayURL\(normalizedDraftURL\)/);
});

test('source accents stay short, thin and decorative with gated hover and reduced-motion support', async () => {
  const css = await readFile(new URL('../../styles/source-colors.css', import.meta.url), 'utf8');
  const styles = await readFile(new URL('../../styles.css', import.meta.url), 'utf8');
  assert.match(styles, /@import '.\/styles\/source-colors.css'/);
  const accent = css.match(/\.source-identity-card::before\s*\{([^}]+)\}/)?.[1] || '';
  assert.match(accent, /width: 2px/);
  assert.match(accent, /height: 1\.5rem/);
  assert.match(accent, /pointer-events: none/);
  assert.match(css, /@media \(hover: hover\) and \(pointer: fine\)/);
  assert.match(css, /:has\(:focus-visible\)::before/);
  assert.match(css, /\.installed-app-identity \.source-identity-dot\s*\{\s*display: inline-block;\s*margin-top: 0;/);
  assert.match(css, /@media \(prefers-reduced-motion: no-preference\)\s*\{\s*\.source-identity-card::before\s*\{\s*transition: opacity/);
  assert.match(css, /html\[data-input-method='keyboard'\] \.source-identity-card::before\s*\{\s*transition: none/);
  assert.doesNotMatch(css, /transition: all|animation:|height: 100%/);
});
