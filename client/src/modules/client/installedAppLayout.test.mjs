import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const styles = await readFile(new URL('../../styles.css', import.meta.url), 'utf8');
const clientStyles = await readFile(new URL('../../styles/client.css', import.meta.url), 'utf8');

test('installed app cards reserve enough width for localized names', () => {
  const gridRule = styles.match(/\.installed-app-grid\s*\{[^}]+\}/s)?.[0] || '';
  const titleRule = styles.match(/\.installed-app-identity strong\s*\{[^}]+\}/s)?.[0] || '';
  const updateControlRule = clientStyles.match(/\.installed-auto-update-control\s*\{[^}]+\}/s)?.[0] || '';

  assert.match(gridRule, /minmax\(320px,\s*1fr\)/);
  assert.match(titleRule, /-webkit-line-clamp:\s*3/);
  assert.match(updateControlRule, /grid-column:\s*2\s*\/\s*-1/);
});
