import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(relativePath) {
  return readFile(new URL(relativePath, import.meta.url), 'utf8');
}

function directXButtons(text) {
  const declarations = [...text.matchAll(/<XButton\b(?:(?!<XButton\b)[\s\S])*?\bonClick=\{(?:(?!<XButton\b)[\s\S])*?\}\s*\/>/g)]
    .map(([declaration]) => declaration);
  const buttonCount = [...text.matchAll(/<XButton\b/g)].length;

  assert.equal(declarations.length, buttonCount, 'contract test must inspect every direct XButton declaration');
  return declarations;
}

function leafCssRules(text) {
  return [...text.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map(([, selector, declarations]) => ({
    selector: selector.trim(),
    declarations,
  }));
}

function atRuleBody(text, headerPattern) {
  const header = headerPattern.exec(text);
  assert.ok(header, `missing CSS at-rule ${headerPattern}`);

  const openingBrace = text.indexOf('{', header.index);
  let depth = 0;
  for (let index = openingBrace; index < text.length; index += 1) {
    if (text[index] === '{') depth += 1;
    if (text[index] === '}') depth -= 1;
    if (depth === 0) return text.slice(openingBrace + 1, index);
  }

  assert.fail(`unclosed CSS at-rule ${headerPattern}`);
}

test('homepage has one primary action and a dedicated source subscription task', async () => {
  const [home, styles] = await Promise.all([
    source('./StorefrontHome.tsx'),
    source('../../styles/storefront.css'),
  ]);

  const buttons = directXButtons(home);
  const primaryButtons = buttons.filter((button) => /\bvariant="primary"/.test(button));
  assert.equal(primaryButtons.length, 1, 'StorefrontHome must declare exactly one primary XButton');
  assert.match(primaryButtons[0], /label=\{t\('nav\.discover'\)\}/);
  assert.match(primaryButtons[0], /onClick=\{\(\) => onNavigate\('search'\)\}/);

  const backstageButton = buttons.find((button) => /label=\{backstageLabel\}/.test(button));
  assert.ok(backstageButton, 'missing login or submit button');
  assert.match(backstageButton, /\bvariant="secondary"/);

  const copySourceButton = buttons.find((button) => /label=\{t\('home\.copySourceFeed'\)\}/.test(button));
  assert.ok(copySourceButton, 'missing source feed copy button');
  assert.match(copySourceButton, /\bvariant="secondary"/);

  const openSourceButton = buttons.find((button) => /label=\{t\('home\.openSourceFeed'\)\}/.test(button));
  assert.ok(openSourceButton, 'missing source feed open button');
  assert.match(openSourceButton, /\bvariant="secondary"/);

  assert.match(home, /className="panel storefront-subscribe-panel"/);
  assert.match(home, /role="status" aria-live="polite"/);
  assert.doesNotMatch(home, /source-feed-card/);

  const metricCardRules = leafCssRules(styles).filter(({ selector }) => selector.includes('.metric-card'));
  assert.ok(metricCardRules.length > 0, 'missing metric card styles');
  for (const { selector, declarations } of metricCardRules) {
    assert.doesNotMatch(selector, /\.metric-card\b[^,{]*(?::hover|:active)\b/);
    assert.doesNotMatch(declarations, /(?:^|[;\s])(?:-webkit-)?(?:transform|transition(?:-[a-z-]+)?)\s*:/i);
  }

  const reducedMotion = atRuleBody(styles, /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{/);
  assert.doesNotMatch(reducedMotion, /\.metric-card\b/);
});

test('app cards scan quickly and keep download independent from detail navigation', async () => {
  const grid = await source('./AppGrid.tsx');

  assert.doesNotMatch(grid, /className="app-readiness"/);
  assert.doesNotMatch(grid, /latestVersion\?\.sourceType/);
  assert.match(grid, /event\.stopPropagation\(\)/);
  assert.match(grid, /if \(pendingAppID !== null\) return/);
  assert.match(grid, /await onInstall\(app\)/);
  assert.match(grid, /isDisabled=\{!installable \|\| pendingAppID !== null\}/);
  assert.match(grid, /isLoading=\{isInstalling\}/);
  assert.match(grid, /className="app-card-primary-action"/);
});

test('app card layout keeps storefront specificity over global catalog rules', async () => {
  const styles = await source('../../styles/storefront.css');
  const expectedRules = new Map([
    [':is(.storefront-page, .storefront-search-page) .app-grid', [
      /grid-template-columns:\s*repeat\(auto-fill, minmax\(min\(100%, 240px\), var\(--catalog-card-max-width\)\)\)/,
      /gap:\s*14px/,
    ]],
    [':is(.storefront-page, .storefront-search-page) .app-card', [
      /min-height:\s*230px/,
      /max-height:\s*none/,
      /grid-template-rows:\s*auto minmax\(0, 1fr\) auto/,
    ]],
    [':is(.storefront-page, .storefront-search-page) .app-open', [
      /grid-template-columns:\s*58px minmax\(0, 1fr\) 18px/,
    ]],
    [':is(.storefront-page, .storefront-search-page) .app-meta', [
      /align-content:\s*start/,
      /font-size:\s*12px/,
      /max-height:\s*none/,
    ]],
  ]);
  const rules = leafCssRules(styles);

  for (const [selector, declarations] of expectedRules) {
    const rule = rules.find((candidate) => candidate.selector === selector);
    assert.ok(rule, `missing higher-specificity storefront rule: ${selector}`);
    for (const declaration of declarations) assert.match(rule.declarations, declaration);
  }

  assert.doesNotMatch(styles, /:where\(\.storefront-page, \.storefront-search-page\) \.app-(?:grid|card|open|meta)\b/);
});

test('app card action switches between LazyCat install and browser download', async () => {
  const grid = await source('./AppGrid.tsx');
  const button = /<XButton\b(?:(?!<XButton\b)[\s\S])*?className="app-card-primary-action"(?:(?!<XButton\b)[\s\S])*?<\/XButton>/
    .exec(grid)?.[0];

  assert.ok(button, 'storefront action must expose separate visible and accessible labels');
  assert.match(grid, /lazycatInstall: boolean/);
  assert.match(grid, /const actionLabel = lazycatInstall \? t\('common\.install'\) : t\('common\.download'\)/);
  assert.match(button, /label=\{installable \? `\$\{actionLabel\} \$\{appName\}` : t\('app\.installUnavailable', \{ name: appName \}\)\}/);
  assert.match(button, /isLoading=\{isInstalling\}/);
  assert.match(button, />\s*\{installable \? actionLabel : t\('common\.unavailable'\)\}\s*<\/XButton>/);
  assert.doesNotMatch(button, /\baria-label=/);
});

test('server app loads LazyCat capability and keeps install payload server-derived', async () => {
  const app = await source('../../App.tsx');

  assert.match(app, /useState<RuntimeCapabilities>\(\{ lazycatInstall: false, githubMirrors: \[\] \}\)/);
  assert.match(app, /api<RuntimeCapabilities>\('\/api\/v1\/runtime\/capabilities'\)/);
  assert.match(app, /const serverInstallMirrorConfig = useMemo<InstallMirrorConfig>/);
  assert.match(app, /applicableMirrorsForVersion\(serverInstallMirrorConfig, version\)/);
  assert.match(app, /serverMirrorOptions\.length > 0/);
  assert.match(app, /`\/api\/v1\/apps\/\$\{app\.id\}\/versions\/\$\{\(version as Version\)\.id\}\/install`/);
  assert.match(app, /body: JSON\.stringify\(\{[\s\S]*installPassword: options\.installPassword \|\| '',[\s\S]*mirrorId: options\.mirrorId \|\| '',[\s\S]*\}\)/);
  assert.match(app, /window\.open\(withInstallPassword\(downloadUrl, options\.installPassword\)/);
  assert.doesNotMatch(app, /body: JSON\.stringify\(\{[^}]*downloadUrl/s);
  assert.doesNotMatch(app, /body: JSON\.stringify\(\{[^}]*mirrorUrl/s);
});

test('search keeps current conditions visible and offers one empty-state recovery', async () => {
  const search = await source('./StorefrontSearch.tsx');

  assert.match(search, /className="page-grid storefront-search-page"/);
  assert.match(search, /className="catalog-result-summary" role="status" aria-live="polite"/);
  assert.match(search, /function clearSearch\(\)/);
  assert.match(search, /action: hasActiveFilters/);
  assert.match(search, /updateViewState\(\{ activeCategory: 'all', filters: \[\], page: 1 \}\)/);
});

test('server storefront browsing state survives opening and closing app details', async () => {
  const [app, searchView, search] = await Promise.all([
    source('../../App.tsx'),
    source('../search/SearchView.tsx'),
    source('./StorefrontSearch.tsx'),
  ]);

  assert.match(app, /useState<StorefrontSearchViewState>/);
  assert.match(app, /storefrontSearchState=\{storefrontSearchState\}/);
  assert.match(app, /onStorefrontSearchStateChange=\{setStorefrontSearchState\}/);
  assert.match(searchView, /storefrontSearchState: StorefrontSearchViewState/);
  assert.match(searchView, /onStorefrontSearchStateChange: Dispatch<SetStateAction<StorefrontSearchViewState>>/);
  assert.match(search, /viewState: StorefrontSearchViewState/);
  assert.match(search, /onViewStateChange: Dispatch<SetStateAction<StorefrontSearchViewState>>/);
  assert.doesNotMatch(search, /const \[filters, setFilters\] = useState/);
  assert.doesNotMatch(search, /const \[page, setPage\] = useState\(1\)/);
  assert.doesNotMatch(search, /const \[pageSize, setPageSize\] = useState/);
});

test('category browser renders child navigation in the default all state', async () => {
  const browser = await source('./CategoryBrowser.tsx');

  assert.match(browser, /categoryBrowserState\(categories, activeCategory, localizedName\)/);
  assert.match(browser, /railItems\.length > 0/);
  assert.match(browser, /railItems\.map\(\(item\) =>/);
  assert.doesNotMatch(browser, /selectedParent && childCategories\.length > 0/);
});

test('public app detail leads with product evidence and keeps trust facts below it', async () => {
  const drawer = await source('./AppDrawer.tsx');
  const screenshotIndex = drawer.indexOf('storefront-screenshot-section');
  const trustIndex = drawer.indexOf("cx('install-trust'");

  assert.match(drawer, /detail-actions storefront-detail-actions/);
  assert.ok(screenshotIndex >= 0, 'screenshot section hook is missing');
  assert.ok(trustIndex >= 0, 'trust section is missing');
  assert.ok(screenshotIndex < trustIndex, 'screenshots must appear before trust facts');
  assert.match(drawer, /storefront-version-section/);
  assert.match(drawer, /storefront-comment-section/);
  assert.match(drawer, /const primaryActionLabel = lazycatInstall \? t\('common\.install'\) : t\('common\.download'\)/);
  assert.match(drawer, /isLatest \? primaryActionLabel : lazycatInstall \? t\('common\.install'\) : t\('drawer\.downloadHistoricalVersion'\)/);
});

test('version management distinguishes policy, download, delete, and cleanup warning states', async () => {
  const drawer = await source('./AppDrawer.tsx');

  assert.match(drawer, /version-retention-summary/);
  assert.match(drawer, /VersionRetentionDialog/);
  assert.match(drawer, /VersionDeleteDialog/);
  assert.match(drawer, /method:\s*'PATCH'/);
  assert.match(drawer, /method:\s*'DELETE'/);
  assert.match(drawer, /cleanupWarning/);
  assert.match(drawer, /stopPropagation\(\)/);
});

test('storefront motion is pointer-aware, brief, and reduced-motion safe', async () => {
  const styles = await source('../../styles/storefront.css');
  const rules = leafCssRules(styles);
  const scopedPages = ':is(.storefront-page, .storefront-search-page, .server-detail-page)';

  assert.match(styles, /@media \(hover: hover\) and \(pointer: fine\)/);
  assert.match(styles, /:active:not\(:focus-visible\)/);
  assert.match(styles, /140ms var\(--ease-out\)/);
  assert.match(styles, /@media \(prefers-reduced-motion: reduce\)[\s\S]*transform: none/);
  assert.match(styles, /:focus-visible/);
  assert.doesNotMatch(styles, /transition:\s*all/);

  const focusRule = rules.find(({ selector }) => selector === `${scopedPages}\n  :is(button, [role='button'], a):focus-visible`);
  assert.ok(focusRule, 'storefront focus must have non-zero scoped specificity');
  assert.match(focusRule.declarations, /outline:\s*2px solid var\(--color-accent\)/);

  const pressRule = rules.find(({ selector }) => selector === `${scopedPages}\n  :is(button, [role='button'], .app-card):active:not(:focus-visible)`);
  assert.ok(pressRule, 'storefront press feedback must outrank the global app-card active rule');
  assert.match(pressRule.declarations, /transform:\s*scale\(0\.98\)/);

  const keyboardRule = rules.find(({ selector }) => selector === `${scopedPages}\n  :is(button, [role='button'], .app-card):active:focus-visible`);
  assert.ok(keyboardRule, 'keyboard activation must neutralize the global app-card active transform');
  assert.match(keyboardRule.declarations, /transform:\s*none/);

  const heroAdRules = rules.filter(({ selector }) => selector === '.storefront-hero-ad');
  assert.ok(heroAdRules.length > 0, 'storefront hero ad must keep its responsive hook');
  for (const { declarations } of heroAdRules) {
    assert.doesNotMatch(declarations, /max-height\s*:/, 'the complete ad card must remain content-sized');
  }
});
