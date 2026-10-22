import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const styles = await readFile(new URL('../../styles.css', import.meta.url), 'utf8');
const clientStyles = await readFile(new URL('../../styles/client.css', import.meta.url), 'utf8');
const installedAppsView = await readFile(new URL('./InstalledAppsView.tsx', import.meta.url), 'utf8');

test('installed app cards reserve enough width for localized names', () => {
  const gridRule = styles.match(/\.installed-app-grid\s*\{[^}]+\}/s)?.[0] || '';
  const titleRule = styles.match(/\.installed-app-identity strong\s*\{[^}]+\}/s)?.[0] || '';
  const updateControlRule = clientStyles.match(/\.installed-auto-update-control\s*\{[^}]+\}/s)?.[0] || '';

  assert.match(gridRule, /minmax\(320px,\s*1fr\)/);
  assert.match(titleRule, /-webkit-line-clamp:\s*3/);
  assert.match(updateControlRule, /grid-column:\s*2\s*\/\s*-1/);
});

test('password-protected updates remain visible when no automatic update is eligible', () => {
  assert.match(installedAppsView, /updateConfirmation\.skipped\.length > 0/);
  assert.match(installedAppsView, /updateQueue\.protectedPending/);
});

test('rocket loops until the update queue stops instead of timing out after launch', () => {
  assert.match(installedAppsView, /const isLaunching = isUpdateQueueRunning && queueIsActive;/);
  assert.doesNotMatch(installedAppsView, /setIsLaunching|setTimeout/);
  assert.match(installedAppsView, /<animateMotion[^>]*repeatCount="indefinite"[^>]*calcMode="linear"/);
  assert.match(clientStyles, /@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.update-rocket-flying \{ display: none; \}\s*\.update-rocket-parked\.is-hidden \{ opacity: 1; \}/);
});
