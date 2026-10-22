import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { createInstance } from 'i18next';
import en from '../../locales/en.ts';
import zh from '../../locales/zh.ts';

const source = (path) => readFile(new URL(path, import.meta.url), 'utf8');

test('result counts use natural singular and plural labels in both locales', async () => {
  const i18n = createInstance();
  await i18n.init({ lng: 'en', resources: { en: { translation: en }, zh: { translation: zh } } });
  assert.equal(i18n.t('search.resultCount', { count: 0 }), '0 results');
  assert.equal(i18n.t('search.resultCount', { count: 1 }), '1 result');
  assert.equal(i18n.t('search.resultCount', { count: 2 }), '2 results');
  await i18n.changeLanguage('zh');
  assert.equal(i18n.t('search.resultCount', { count: 1 }), '1 个结果');
  assert.equal(i18n.t('search.resultCount', { count: 2 }), '2 个结果');
});

test('both catalogs share a live count and a persistent filter recovery action', async () => {
  const [client, server, summary] = await Promise.all([
    source('../../modules/client/ClientCatalog.tsx'),
    source('../../modules/storefront/StorefrontSearch.tsx'),
    source('./CatalogResultSummary.tsx'),
  ]);
  for (const catalog of [client, server]) {
    assert.match(catalog, /<CatalogResultSummary[^>]*onReset=\{hasActiveFilters \? clearSearch : undefined\}/);
  }
  assert.match(summary, /role="status" aria-live="polite" aria-atomic="true"/);
  assert.match(summary, /onReset &&[\s\S]*onClick=\{onReset\}/);
  assert.match(summary, /aria-hidden="true"/);
});

test('empty filtered results reset the catalog instead of adding another source', async () => {
  const [catalog, grid] = await Promise.all([
    source('../../modules/client/ClientCatalog.tsx'),
    source('../../modules/client/SourceAppGrid.tsx'),
  ]);
  assert.match(catalog, /updateViewState\(\{ filters: \[\], selectedCategoryFilter: 'all', page: 1 \}\)/);
  assert.match(catalog, /emptyAction=\{sourceApps.length > 0 && hasActiveFilters \?[^\n]*onClick: clearSearch/);
  assert.match(grid, /action=\{emptyAction \?\? \(showEmptyAction \?/);
});

test('modal motion is on the shell only and respects keyboard and reduced motion', async () => {
  const [styles, baseStyles] = await Promise.all([
    source('../../styles/apple-polish.css'),
    source('../../styles.css'),
  ]);
  assert.match(styles, /\.app-shell \.modal-dialog-shell \{[^}]*transition: opacity 200ms var\(--ease-out\), transform 200ms var\(--ease-out\);/);
  assert.match(styles, /@starting-style \{\s*opacity: 0;\s*transform: scale\(0\.97\);/);
  assert.match(styles, /\.modal-dialog-shell :is\(\.modal-panel, \.install-password-dialog\) \{[^}]*transform: none;\s*transition: none;/);
  assert.match(styles, /html\[data-input-method='keyboard'\][^{]*\.modal-dialog-shell[^}]*transition: none !important/);
  assert.match(styles, /@media \(prefers-reduced-motion: reduce\) \{\s*\.app-shell \.modal-dialog-shell \{\s*transform: none !important;\s*transition: opacity 120ms/);
  assert.doesNotMatch(baseStyles, /\.modal-dialog-shell \{[^}]*!important/);
});
