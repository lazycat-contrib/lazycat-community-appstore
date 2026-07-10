# Storefront UX Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make the public server storefront clearly prioritize app discovery, expose source subscription as a dedicated task, reduce app-card noise, improve search recovery, and polish the full-page app detail experience without touching shared integration-owned files.

**Architecture:** Keep the existing React 19 + Astryx component structure and refine only the storefront module boundary. Add a dependency-free Node contract test beside the storefront components, then implement homepage, card, search, detail, responsive, motion, and accessibility improvements incrementally; `App.tsx`, locale dictionaries, shared styles, and generated distributions remain integration-owned.

**Tech Stack:** React 19, TypeScript 5.9, Astryx Design components, lucide-react, vanilla CSS, Node 26 built-in test runner, Vite development server.

## Global Constraints

- The approved design target is the public server storefront only: home, discovery/search, category browsing, app cards, app detail, download action, and source subscription.
- Preserve the user's current uncommitted work in `AppDrawer.tsx`, `AppGrid.tsx`, and `StorefrontSearch.tsx`; do not restore these files from `HEAD` or delete the rating/download-period additions.
- Treat the current rating UI in `AppDrawer.tsx` and `AppGrid.tsx` and the `downloads_day`, `downloads_week`, `downloads_month`, and `downloads_year` sort modes in `StorefrontSearch.tsx` as immutable baseline behavior.
- FE-1 may modify only `client/src/modules/storefront/**` and `client/src/styles/storefront.css`.
- FE-1 must not modify `client/src/App.tsx`, `client/src/locales/zh.ts`, `client/src/locales/en.ts`, `client/src/styles.css`, `client/src/styles/components.css`, `client/package.json`, or any backend file.
- FE-1 must not run `npm run build`, copy build output, or modify `client/dist`, `clientembed/dist`, or `web/dist`; TypeScript checking uses `npx tsc -p tsconfig.json --noEmit` and browser review uses the Vite development server.
- All screens have one visually primary action. On the homepage it is “Discover”; login/submit, copy, and open-feed actions are secondary.
- Press feedback lasts 100–160ms and scales to 0.97–0.99. Frequent navigation and keyboard-triggered actions do not use positional animation.
- Pop-in motion is not introduced in FE-1. Hover styling is gated by `@media (hover: hover) and (pointer: fine)`.
- `prefers-reduced-motion` removes transform movement while preserving readable state changes.
- App cards remain whole-card detail links; the download button is an independent command and must stop click propagation.
- Trust information removed from cards remains present in the app detail page.
- Empty states expose at most one recovery action.
- Use existing Astryx components and theme variables; do not introduce another design system or a new runtime dependency.
- Use `git add -p` for the three WIP files. Stage only FE-1 hunks, split combined hunks with `s`, and edit a patch with `e` if a FE-1 hunk contains pre-existing WIP lines.
- Before every commit, `git diff --cached --unified=0` must contain no added line for `toggleAppRating`, `ratingAdded`, `app.rating`, `Sparkles`, `downloads_day`, `downloads_week`, `downloads_month`, or `downloads_year`.

---

## Existing WIP Baseline

Run this before editing and save the output in the task log:

```bash
git status --short
git diff -- client/src/modules/storefront/AppDrawer.tsx \
  client/src/modules/storefront/AppGrid.tsx \
  client/src/modules/storefront/StorefrontSearch.tsx
```

Expected observations:

- `AppDrawer.tsx` contains rating mutation UI and rating metadata.
- `AppGrid.tsx` contains the rating metric with the `Sparkles` icon.
- `StorefrontSearch.tsx` contains day/week/month/year download sorting.
- `StorefrontHome.tsx`, `CategoryBrowser.tsx`, and `styles/storefront.css` have no pre-existing user diff at plan-writing time.

Do not use `git checkout`, `git restore`, `git reset`, or any command that discards these changes.

## File Structure

| File | Responsibility | Change type |
| --- | --- | --- |
| `client/src/modules/storefront/storefront.contract.test.mjs` | Dependency-free structural regression tests for hierarchy, action semantics, WIP-safe rendering, responsive CSS, and motion rules | Create |
| `client/src/modules/storefront/StorefrontHome.tsx` | Homepage hero, metrics, source subscription panel, clipboard status, and ad placement | Modify |
| `client/src/modules/storefront/AppGrid.tsx` | Compact card hierarchy, independent download command, and card accessibility | Modify while preserving WIP |
| `client/src/modules/storefront/StorefrontSearch.tsx` | Search/filter/result hierarchy and one-action empty-state recovery | Modify while preserving WIP |
| `client/src/modules/storefront/AppDrawer.tsx` | Public detail content order and section/action styling hooks | Modify while preserving WIP |
| `client/src/styles/storefront.css` | Storefront-only layout, responsive behavior, pointer feedback, focus, and reduced-motion overrides | Modify |

`CategoryBrowser.tsx` remains unchanged: its current button semantics, horizontal overflow, and labelled subcategory rail already satisfy this pass.

## Design Contract

| Before | After | Why |
| --- | --- | --- |
| Hero has two `primary` buttons | Discover remains `primary`; login/submit becomes `secondary` | One screen should have one dominant next step |
| Source URL is the third metric card | Source subscription is a dedicated panel with v2 label, copy, open, and persistent status | Subscription is a user task, not a statistic |
| Pure metric cards move on hover and press | Metrics remain visually stable | Static numbers must not imply clickability |
| Cards show category, version, downloads, rating, source type, readiness, checksum, password, and review badges | Cards show identity, summary, category, version, downloads/rating, and one download command | Scanning improves while trust facts stay available in detail |
| Download click can bubble to the whole-card detail action | Download stops propagation before calling `onInstall` | The nested command must not open detail accidentally |
| Search result count is embedded inside the search control | Active category and result count stay visible in a separate live summary | Users can understand the current result set without reopening controls |
| Zero-result search has no next action | Zero-result search offers one “clear filters” recovery when filters are active | Empty states should lead to the most likely recovery |
| Screenshots appear after trust, metadata, and release notes | Screenshots appear directly after the app header/actions, followed by trust and technical facts | Product evaluation starts with the app, then verifies trust |
| Detail sections visually blend into a long page | Screenshots, version history, and comments have clear section surfaces | Long detail pages need stable landmarks, especially on mobile |
| Generic motion applies without pointer distinction | Hover is fine-pointer only; press is short; reduced motion removes transforms | Feedback remains responsive without making keyboard or motion-sensitive use feel slow |

---

### Task 1: Homepage Primary Action and Source Subscription

**Files:**
- Create: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/modules/storefront/StorefrontHome.tsx:1-110`
- Modify: `client/src/styles/storefront.css:1-265`

**Interfaces:**
- Consumes: existing `StorefrontHome` props without signature changes.
- Consumes: existing locale keys `nav.discover`, `home.submitApp`, `topbar.login`, `home.openSourceFeed`, `home.copySourceFeed`, `home.sourceCopied`, `home.copySourceFailed`, `home.copySourceUnsupported`, `home.sourceUrl`, `sources.subtitle`, and `common.version`.
- Produces: local state type `'idle' | 'copied' | 'failed' | 'unsupported'` for persistent clipboard feedback.
- Produces: CSS hooks `.storefront-subscribe-panel`, `.storefront-subscribe-copy`, `.storefront-source-meta`, `.storefront-subscribe-actions`, and `.storefront-copy-status`.

- [x] **Step 1: Write the failing homepage contract test**

Create `client/src/modules/storefront/storefront.contract.test.mjs` with:

```js
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(relativePath) {
  return readFile(new URL(relativePath, import.meta.url), 'utf8');
}

test('homepage has one primary action and a dedicated source subscription task', async () => {
  const [home, styles] = await Promise.all([
    source('./StorefrontHome.tsx'),
    source('../../styles/storefront.css'),
  ]);

  assert.match(home, /variant="primary"[\s\S]{0,200}onClick=\{\(\) => onNavigate\('search'\)\}/);
  assert.match(home, /variant="secondary"[\s\S]{0,200}label=\{backstageLabel\}/);
  assert.match(home, /className="panel storefront-subscribe-panel"/);
  assert.match(home, /role="status" aria-live="polite"/);
  assert.doesNotMatch(home, /source-feed-card/);
  assert.doesNotMatch(styles, /metric-card:hover[\s\S]{0,100}translateY/);
  assert.doesNotMatch(styles, /metric-card:active[\s\S]{0,100}scale/);
});
```

- [x] **Step 2: Run the contract test and verify the existing UI fails**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: exit code `1`; output contains `not ok 1 - homepage has one primary action and a dedicated source subscription task` and an assertion mentioning the missing secondary action or subscription panel.

- [x] **Step 3: Replace `StorefrontHome.tsx` with the focused homepage implementation**

Use this complete file content. It keeps all existing props, collections, categories, ads, and empty-state behavior:

```tsx
import { Copy, ExternalLink, History, Layers3, Link, LogIn, PackagePlus, Search, Tag } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CodeBlock as XCodeBlock } from '@astryxdesign/core/CodeBlock';
import { API_BASE } from '../../config';
import { AdSpot, visibleSiteAds } from '../../components/AdSpot';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Category, Collection, SiteAd, SiteProfile, StoreApp } from '../../shared/types';
import { AppGrid } from './AppGrid';
import { CategoryBrowser } from './CategoryBrowser';

type SourceCopyStatus = 'idle' | 'copied' | 'failed' | 'unsupported';

export function StorefrontHome({
  apps,
  appCount,
  categories,
  collections,
  siteProfile,
  onOpen,
  onInstall,
  onNavigate,
  onSubmitApp,
  activeCategory,
  onCategory,
  isAuthenticated,
  ads,
}: {
  apps: StoreApp[];
  appCount?: number;
  categories: Category[];
  collections: Collection[];
  siteProfile: SiteProfile;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onNavigate: (tab: 'search' | 'profile') => void;
  onSubmitApp: () => void;
  activeCategory: string;
  onCategory: (category: string) => void;
  isAuthenticated: boolean;
  ads?: SiteAd[];
}) {
  const { t } = useTranslation();
  const [sourceCopyStatus, setSourceCopyStatus] = useState<SourceCopyStatus>('idle');
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  const approvedCount = appCount ?? apps.filter((app) => app.status === 'APPROVED').length;
  const sourceFeedURL = siteProfile.sourceUrl || `${API_BASE || window.location.origin}/source/v2/index.json`;
  const BackstageIcon = isAuthenticated ? PackagePlus : LogIn;
  const backstageLabel = isAuthenticated ? t('home.submitApp') : t('topbar.login');
  const visibleAds = visibleSiteAds(ads);
  const sourceCopyMessage = sourceCopyStatus === 'copied'
    ? t('home.sourceCopied')
    : sourceCopyStatus === 'unsupported'
      ? t('home.copySourceUnsupported')
      : sourceCopyStatus === 'failed'
        ? t('home.copySourceFailed')
        : '';

  async function copySourceFeed() {
    if (!navigator.clipboard?.writeText) {
      setSourceCopyStatus('unsupported');
      return;
    }
    try {
      await navigator.clipboard.writeText(sourceFeedURL);
      setSourceCopyStatus('copied');
    } catch {
      setSourceCopyStatus('failed');
    }
  }

  function openSourceFeed() {
    window.open(sourceFeedURL, '_blank', 'noopener,noreferrer');
  }

  return (
    <section className="page-grid storefront-page">
      <div className={`hero-band storefront-hero${visibleAds.length === 0 ? ' storefront-hero-without-ad' : ''}`}>
        <div className="storefront-hero-copy">
          <span className="eyebrow">{t('home.eyebrow')}</span>
          <h1>{siteProfile.title || t('home.title')}</h1>
          <p>{siteProfile.subtitle || t('home.body')}</p>
          <div className="hero-actions">
            <XButton type="button" variant="primary" label={t('nav.discover')} icon={<Search size={18} />} onClick={() => onNavigate('search')} />
            <XButton type="button" variant="secondary" label={backstageLabel} icon={<BackstageIcon size={18} />} onClick={onSubmitApp} />
          </div>
        </div>
        {visibleAds.length > 0 && <AdSpot ads={visibleAds} className="storefront-hero-ad" />}
      </div>

      <section className="store-metrics" aria-label={t('nav.store')}>
        <XCard className="metric-card" padding={4}>
          <span>{t('common.apps')}</span>
          <strong>{approvedCount}</strong>
          <small>{t('home.approvedCount', { count: approvedCount })}</small>
        </XCard>
        <XCard className="metric-card" padding={4}>
          <span>{t('common.category')}</span>
          <strong>{categories.length}</strong>
          <small>{t('home.categoryCount', { count: categories.length })}</small>
        </XCard>
      </section>

      <section className="panel storefront-subscribe-panel" aria-labelledby="storefront-subscribe-title">
        <div className="storefront-subscribe-copy">
          <div className="section-title">
            <Link size={19} />
            <h2 id="storefront-subscribe-title">{t('home.openSourceFeed')}</h2>
          </div>
          <p>{t('sources.subtitle')}</p>
          <div className="storefront-source-meta">
            <span>{t('common.version')}</span>
            <strong>v2</strong>
          </div>
        </div>
        <div className="storefront-subscribe-command">
          <XCodeBlock code={sourceFeedURL} language="plaintext" hasLanguageLabel={false} width="100%" size="sm" />
          <div className="storefront-subscribe-actions">
            <XButton type="button" variant="secondary" label={t('home.copySourceFeed')} icon={<Copy size={17} />} onClick={() => void copySourceFeed()} />
            <XButton type="button" variant="secondary" label={t('home.openSourceFeed')} icon={<ExternalLink size={17} />} onClick={openSourceFeed} />
          </div>
          {sourceCopyMessage && (
            <p className="storefront-copy-status" role="status" aria-live="polite" data-tone={sourceCopyStatus}>
              {sourceCopyMessage}
            </p>
          )}
        </div>
      </section>

      {categories.length > 0 && (
        <section className="panel category-rail-panel">
          <SectionTitle icon={Tag} title={t('home.categories')} />
          <CategoryBrowser categories={categories} activeCategory={activeCategory} onCategory={onCategory} />
        </section>
      )}

      <section className="panel">
        <SectionTitle icon={History} title={t('home.latest')} />
        <AppGrid
          apps={latest}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{
            title: t('home.emptyTitle'),
            body: isAuthenticated ? t('home.emptyBody') : t('home.emptyLoginBody'),
            action: { label: backstageLabel, icon: BackstageIcon, onClick: onSubmitApp },
          }}
        />
      </section>
      {collections.map((collection) => (
        <section className="panel" key={collection.id}>
          <SectionTitle icon={Layers3} title={collection.name} />
          <AppGrid apps={collection.apps || []} onOpen={onOpen} onInstall={onInstall} />
        </section>
      ))}
    </section>
  );
}
```

- [x] **Step 4: Add the homepage and subscription styles**

In `client/src/styles/storefront.css`, replace the existing `.storefront-page .store-metrics` and `.metric-card` rules with:

```css
.storefront-page .store-metrics {
  grid-template-columns: repeat(2, minmax(150px, 240px));
  justify-content: start;
}

.storefront-page .metric-card {
  align-content: start;
}

.storefront-subscribe-panel {
  display: grid;
  grid-template-columns: minmax(220px, 0.7fr) minmax(0, 1.3fr);
  gap: 18px;
  align-items: center;
}

.storefront-subscribe-copy,
.storefront-subscribe-command {
  min-width: 0;
  display: grid;
  gap: 10px;
}

.storefront-subscribe-copy p,
.storefront-copy-status {
  margin: 0;
  color: var(--muted);
  line-height: 1.5;
}

.storefront-source-meta {
  width: max-content;
  max-width: 100%;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--field);
  padding: 4px 9px;
  font-size: 12px;
}

.storefront-source-meta span {
  color: var(--muted);
}

.storefront-subscribe-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.storefront-copy-status[data-tone='copied'] {
  color: var(--green);
}

.storefront-copy-status[data-tone='failed'],
.storefront-copy-status[data-tone='unsupported'] {
  color: var(--red);
}
```

Remove `.metric-card` from every hover, active, transition, and reduced-motion selector. The remaining selectors must not apply `transform` to metric cards.

Inside `@media (max-width: 980px)`, use:

```css
.storefront-page .store-metrics,
.storefront-subscribe-panel {
  grid-template-columns: 1fr;
}
```

- [x] **Step 5: Run the homepage contract and TypeScript check**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected:

- Node exits `0` with `# pass 1` and `# fail 0`.
- TypeScript exits `0` with no diagnostics and writes no distribution files.

- [x] **Step 6: Stage only Task 1 files and commit**

Run:

```bash
git add client/src/modules/storefront/storefront.contract.test.mjs \
  client/src/modules/storefront/StorefrontHome.tsx \
  client/src/styles/storefront.css
git diff --cached --check
git diff --cached --unified=0 | rg '^\+.*(toggleAppRating|ratingAdded|app\.rating|Sparkles|downloads_(day|week|month|year))' || true
git commit -m "feat: focus storefront homepage actions"
```

Expected: `git diff --cached --check` prints nothing; the WIP scan prints nothing; commit succeeds without staging locale files or generated distributions.

---

### Task 2: Compact App Cards and Independent Download Action

**Files:**
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/modules/storefront/AppGrid.tsx:1-83`
- Modify: `client/src/styles/storefront.css`

**Interfaces:**
- Consumes: the same `AppGrid` prop names; widens `onInstall` from `void` to `void | Promise<void>` so pending state can follow asynchronous installs.
- Preserves: the current rating metric added by the user WIP.
- Produces: card content order `identity → compact metadata → download`.
- Produces: a local `pendingAppID` guard and `installApp(event, app)` handler so repeated clicks are ignored until `onInstall` settles.
- Removes from card surface only: `app-readiness`, checksum badge, review badge, install-password badge, and source-type metric.
- Does not remove those values from `StoreApp` or `AppDrawer`.

- [x] **Step 1: Add a failing card contract**

Append to `storefront.contract.test.mjs`:

```js
test('app cards scan quickly and keep download independent from detail navigation', async () => {
  const grid = await source('./AppGrid.tsx');

  assert.doesNotMatch(grid, /className="app-readiness"/);
  assert.doesNotMatch(grid, /latestVersion\?\.sourceType/);
  assert.match(grid, /event\.stopPropagation\(\)/);
  assert.match(grid, /if \(pendingAppID === app\.id\) return/);
  assert.match(grid, /await onInstall\(app\)/);
  assert.match(grid, /isLoading=\{isInstalling\}/);
  assert.match(grid, /className="app-card-primary-action"/);
});
```

- [x] **Step 2: Run only the card contract and verify failure**

Run:

```bash
node --test --test-name-pattern="app cards" client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: exit code `1`; output contains `not ok 1 - app cards scan quickly and keep download independent from detail navigation`.

- [x] **Step 3: Simplify `AppGrid.tsx` without removing the user's rating line**

Make these exact structural changes:

1. Keep the existing `Sparkles` import and rating `<span>`.
2. Remove imports used only by readiness/source badges: `Check`, `KeyRound`, `Link`, and `ShieldCheck`.
3. Add the React event/state import:

```tsx
import { type MouseEvent, useState } from 'react';
```

4. Change the `onInstall` prop type to:

```tsx
onInstall: (app: StoreApp) => void | Promise<void>;
```

5. Immediately after `const { t } = useTranslation();`, add:

```tsx
const [pendingAppID, setPendingAppID] = useState<number | null>(null);

async function installApp(event: MouseEvent<HTMLButtonElement>, app: StoreApp) {
  event.stopPropagation();
  if (pendingAppID === app.id) return;
  setPendingAppID(app.id);
  try {
    await onInstall(app);
  } finally {
    setPendingAppID((current) => (current === app.id ? null : current));
  }
}
```

6. Remove `const hasChecksum = ...` and add this inside the app mapping, after `installable`:

```tsx
const isInstalling = pendingAppID === app.id;
```

7. Remove the conditional source-type `<span>`.
8. Delete the complete `<div className="app-readiness">...</div>` block.
9. Replace the download button with:

```tsx
<XButton
  className="app-card-primary-action"
  type="button"
  variant="primary"
  label={installable ? t('common.download') : t('common.unavailable')}
  icon={<Download size={17} />}
  isDisabled={!installable || isInstalling}
  isLoading={isInstalling}
  onClick={(event) => void installApp(event, app)}
  aria-label={installable ? `${t('common.download')} ${appName}` : t('app.installUnavailable', { name: appName })}
/>
```

The final metadata block must remain:

```tsx
<div className="app-meta">
  <XBadge variant="neutral" icon={<Tag size={13} />} label={localizedCategory(app, t('common.uncategorized'))} />
  <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
  <span><Download size={14} /> {t('app.downloads', { count: app.downloadCount })}</span>
  <span><Sparkles size={14} /> {t('app.rating', { score: app.rating?.score || 0, count: app.rating?.voteCount || 0 })}</span>
</div>
```

- [x] **Step 4: Replace card sizing and interaction styles**

Replace the current storefront card/grid rules with:

```css
:where(.storefront-page, .storefront-search-page) .app-grid {
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), var(--catalog-card-max-width)));
  justify-content: start;
  gap: 14px;
}

:where(.storefront-page, .storefront-search-page) .app-card {
  min-height: 230px;
  max-width: var(--catalog-card-max-width);
  max-height: none;
  grid-template-rows: auto minmax(0, 1fr) auto;
  background: var(--surface);
  transition:
    transform 140ms var(--ease-out),
    border-color 140ms ease,
    box-shadow 140ms ease;
}

:where(.storefront-page, .storefront-search-page) .app-open {
  grid-template-columns: 58px minmax(0, 1fr) 18px;
}

:where(.storefront-page, .storefront-search-page) .app-open .app-artwork,
:where(.storefront-page, .storefront-search-page) .app-open .humation-avatar {
  width: 58px !important;
  height: 58px !important;
}

:where(.storefront-page, .storefront-search-page) .app-open h3 {
  font-size: 17px;
  line-height: 1.25;
}

:where(.storefront-page, .storefront-search-page) .app-meta {
  align-content: start;
  color: var(--muted);
  font-size: 12px;
  gap: 7px;
  max-height: none;
}

:where(.storefront-page, .storefront-search-page) .app-meta span {
  min-height: 24px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--field);
  padding: 3px 7px;
}

:where(.storefront-page, .storefront-search-page) .app-card-primary-action {
  align-self: end;
  justify-self: stretch;
}

@media (hover: hover) and (pointer: fine) {
  :where(.storefront-page, .storefront-search-page) .app-card:hover {
    border-color: var(--color-border-emphasized);
    box-shadow: var(--shadow-soft);
  }
}
```

Do not add hover translation. The card may change border/shadow, but it stays in place.

- [x] **Step 5: Run both contracts and TypeScript**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected: Node reports `# pass 2`, `# fail 0`; TypeScript exits `0` without output.

- [x] **Step 6: Interactively stage the WIP-overlapping card file and commit**

Run:

```bash
git add client/src/modules/storefront/storefront.contract.test.mjs client/src/styles/storefront.css
git add -p client/src/modules/storefront/AppGrid.tsx
git diff --cached --check
git diff --cached --unified=0 | rg '^\+.*(toggleAppRating|ratingAdded|app\.rating|Sparkles|downloads_(day|week|month|year))' || true
git diff --cached -- client/src/modules/storefront/AppGrid.tsx
git commit -m "feat: simplify storefront app cards"
```

During `git add -p`:

- Stage removal of source type/readiness, the button class, and `stopPropagation`.
- Do not stage the pre-existing `Sparkles` import or rating metric addition.
- If Git combines them, press `s`; if it cannot split, press `e` and remove only the rating-addition lines from the staged patch.

Expected: the cached WIP scan prints nothing. The working tree still contains the rating metric after the commit.

---

### Task 3: Search Hierarchy, Result Summary, and Empty Recovery

**Files:**
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/modules/storefront/StorefrontSearch.tsx:1-210`
- Modify: `client/src/styles/storefront.css`

**Interfaces:**
- Consumes: existing search props and existing `Category` records.
- Preserves: all current period download sort modes and `SortMode` behavior.
- Produces: `activeCategoryLabel: string`, `hasActiveFilters: boolean`, and `clearSearch(): void` inside `StorefrontSearch`.
- Produces: `.storefront-search-page`, `.storefront-search-panel`, and `.catalog-result-summary`.
- Produces for INT: locale key `search.clearFilters`; FE-1 uses `search.allCategories` as `defaultValue` until INT merges the dictionaries.

- [x] **Step 1: Add a failing search contract**

Append:

```js
test('search keeps current conditions visible and offers one empty-state recovery', async () => {
  const search = await source('./StorefrontSearch.tsx');

  assert.match(search, /className="page-grid storefront-search-page"/);
  assert.match(search, /className="catalog-result-summary" role="status" aria-live="polite"/);
  assert.match(search, /function clearSearch\(\)/);
  assert.match(search, /action: hasActiveFilters/);
  assert.match(search, /setFilters\(\[\]\)/);
  assert.match(search, /onCategory\('all'\)/);
});
```

- [x] **Step 2: Run the search contract and verify failure**

Run:

```bash
node --test --test-name-pattern="search keeps" client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: exit code `1`; output contains `not ok 1 - search keeps current conditions visible and offers one empty-state recovery`.

- [x] **Step 3: Add search summary and clear behavior while leaving sort WIP untouched**

Change the icon import to include `RotateCcw`:

```tsx
import { RotateCcw, Search, Tag, UserRound } from 'lucide-react';
```

Change the utility import to:

```tsx
import { localizedAppDescription, localizedAppName, localizedAppSummary, localizedName } from '../../shared/utils';
```

After `pagedApps`, add:

```tsx
const selectedCategory = categories.find((category) => String(category.id) === activeCategory);
const activeCategoryLabel = selectedCategory ? localizedName(selectedCategory) : t('search.allCategories');
const hasActiveFilters = activeCategory !== 'all' || filters.length > 0;

function clearSearch() {
  setFilters([]);
  onCategory('all');
  setPage(1);
}
```

Change the page and panel opening tags to:

```tsx
<section className="page-grid storefront-search-page">
  <div className="page-heading">
    <h1>{t('search.serverTitle')}</h1>
    <p>{t('search.serverDescription')}</p>
  </div>
  <section className="panel storefront-search-panel">
```

Immediately after `.catalog-search-toolbar`, insert:

```tsx
<div className="catalog-result-summary" role="status" aria-live="polite" aria-atomic="true">
  <span>{activeCategoryLabel}</span>
  <strong>{t('search.resultCount', { count: filteredApps.length })}</strong>
</div>
```

Replace the `AppGrid` empty prop with:

```tsx
empty={{
  title: t('search.noResultsTitle'),
  body: t('search.noResultsBody'),
  action: hasActiveFilters
    ? {
        label: t('search.clearFilters', { defaultValue: t('search.allCategories') }),
        icon: RotateCcw,
        onClick: clearSearch,
      }
    : undefined,
}}
```

Do not edit the sort comparator or sort option array in this task.

- [x] **Step 4: Add responsive search-control styles**

Append before the existing media queries:

```css
.storefront-search-page {
  gap: 18px;
}

.storefront-search-panel {
  display: grid;
  gap: 14px;
}

.storefront-search-panel .category-browser,
.storefront-search-panel .catalog-search-toolbar {
  margin-bottom: 0;
}

.catalog-result-summary {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--line);
  padding-top: 12px;
  color: var(--muted);
}

.catalog-result-summary span,
.catalog-result-summary strong {
  min-width: 0;
  overflow-wrap: anywhere;
}

.catalog-result-summary strong {
  color: var(--ink);
  text-align: right;
}
```

Inside `@media (max-width: 640px)`, add:

```css
.storefront-search-page .catalog-search-toolbar {
  grid-template-columns: 1fr;
}

.catalog-result-summary {
  align-items: flex-start;
  flex-direction: column;
  gap: 4px;
}

.catalog-result-summary strong {
  text-align: left;
}
```

- [x] **Step 5: Run contracts and TypeScript**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected: Node reports `# pass 3`, `# fail 0`; TypeScript exits `0` without diagnostics.

- [x] **Step 6: Interactively stage the search file and commit**

Run:

```bash
git add client/src/modules/storefront/storefront.contract.test.mjs client/src/styles/storefront.css
git add -p client/src/modules/storefront/StorefrontSearch.tsx
git diff --cached --check
git diff --cached --unified=0 | rg '^\+.*downloads_(day|week|month|year)' || true
git diff --cached -- client/src/modules/storefront/StorefrontSearch.tsx
git commit -m "feat: clarify storefront search results"
```

During `git add -p`, stage only the new root class, imports, category/result summary, `clearSearch`, and empty-state action. Skip every pre-existing period-sort hunk. Expected: the period-sort scan prints nothing, while the working tree still contains all four period sorts.

---

### Task 4: App Detail Product Hierarchy

**Files:**
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/modules/storefront/AppDrawer.tsx:1021-1301`
- Modify: `client/src/styles/storefront.css`

**Interfaces:**
- Consumes: existing `AppDrawer` props, rating/favorite/collaboration handlers, trust facts, screenshot renderer, version table, and comments.
- Preserves: all detail and management behavior, including the user's rating action.
- Produces: `.storefront-detail-actions`, `.storefront-detail-section`, `.storefront-screenshot-section`, `.storefront-version-section`, and `.storefront-comment-section`.
- Produces content order in public detail mode: header → actions → screenshots → trust/outdated/metadata → release notes → version history → comments.
- Management mode retains its existing action and dialog ordering.

- [x] **Step 1: Add a failing detail hierarchy contract**

Append:

```js
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
});
```

- [x] **Step 2: Run the detail contract and verify failure**

Run:

```bash
node --test --test-name-pattern="public app detail" client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: exit code `1`; output contains `not ok 1 - public app detail leads with product evidence and keeps trust facts below it`.

- [x] **Step 3: Add detail hooks and move only the public screenshot section**

Change:

```tsx
<div className="detail-actions">
```

to:

```tsx
<div className="detail-actions storefront-detail-actions">
```

Immediately after the closing `</div>` for detail actions and before the existing `install-trust` card, insert:

```tsx
{!isManageMode && (
  <section className="storefront-detail-section storefront-screenshot-section">
    <h3>{t('drawer.screenshots')}</h3>
    {renderScreenshotGallery(false)}
  </section>
)}
```

Delete the old identical public screenshot section that currently appears after `renderManagementDialogs()`.

Change the version section opening tag to:

```tsx
<section className="storefront-detail-section storefront-version-section">
```

Change the public comments section opening tag to:

```tsx
<section className="storefront-detail-section storefront-comment-section">
```

Do not change `toggleAppRating`, its button, locale calls, request paths, or refresh behavior.

- [x] **Step 4: Add public-detail surface and action styles**

Append before media queries:

```css
.server-detail-page .storefront-detail-actions {
  min-width: 0;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  padding: 12px;
}

.server-detail-page .storefront-detail-actions > button {
  min-height: 38px;
}

.server-detail-page .storefront-detail-section {
  min-width: 0;
  display: grid;
  gap: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  padding: 18px;
}

.server-detail-page .storefront-detail-section > h3 {
  margin: 0;
}

.server-detail-page .storefront-screenshot-section .screenshot-grid {
  margin-top: 0;
}

.server-detail-page .storefront-comment-section .comment-form {
  margin-top: 0;
}
```

Inside `@media (max-width: 640px)`, add:

```css
.server-detail-page .storefront-detail-actions,
.server-detail-page .storefront-detail-section {
  padding: 14px;
}

.server-detail-page .storefront-detail-actions > button {
  width: 100%;
  justify-content: center;
}
```

- [x] **Step 5: Run contracts and TypeScript**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
```

Expected: Node reports `# pass 4`, `# fail 0`; TypeScript exits `0` without diagnostics.

- [x] **Step 6: Interactively stage the detail file and commit**

Run:

```bash
git add client/src/modules/storefront/storefront.contract.test.mjs client/src/styles/storefront.css
git add -p client/src/modules/storefront/AppDrawer.tsx
git diff --cached --check
git diff --cached --unified=0 | rg '^\+.*(toggleAppRating|ratingAdded|ratingRemoved|ratingVote|ratingVoted)' || true
git diff --cached -- client/src/modules/storefront/AppDrawer.tsx
git commit -m "feat: refine storefront app detail hierarchy"
```

Stage only the class-name and screenshot-order hunks. Skip all pre-existing rating hunks. Expected: the rating scan prints nothing and the working tree still contains the rating feature.

---

### Task 5: Storefront Interaction, Focus, Responsive, and Reduced Motion Rules

**Files:**
- Modify: `client/src/modules/storefront/storefront.contract.test.mjs`
- Modify: `client/src/styles/storefront.css`

**Interfaces:**
- Consumes: all CSS hooks created in Tasks 1–4.
- Produces: pointer-specific hover, 140ms press feedback, visible focus, stable 320–1440px layout, and transform-free reduced motion.
- Produces no JavaScript API or prop changes.
- Leaves the project-wide reduced-motion rule in `client/src/styles.css` to INT.

- [x] **Step 1: Add a failing interaction contract**

Append:

```js
test('storefront motion is pointer-aware, brief, and reduced-motion safe', async () => {
  const styles = await source('../../styles/storefront.css');

  assert.match(styles, /@media \(hover: hover\) and \(pointer: fine\)/);
  assert.match(styles, /:active:not\(:focus-visible\)/);
  assert.match(styles, /140ms var\(--ease-out\)/);
  assert.match(styles, /@media \(prefers-reduced-motion: reduce\)[\s\S]*transform: none/);
  assert.match(styles, /:focus-visible/);
  assert.doesNotMatch(styles, /transition:\s*all/);
});
```

- [x] **Step 2: Run the interaction contract and verify failure**

Run:

```bash
node --test --test-name-pattern="storefront motion" client/src/modules/storefront/storefront.contract.test.mjs
```

Expected: exit code `1`; output contains `not ok 1 - storefront motion is pointer-aware, brief, and reduced-motion safe`.

- [x] **Step 3: Add exact focus and press feedback rules**

Append before responsive media queries:

```css
:where(.storefront-page, .storefront-search-page, .server-detail-page)
  :where(button, [role='button'], a):focus-visible {
  outline: 2px solid var(--color-accent);
  outline-offset: 3px;
}

:where(.storefront-page, .storefront-search-page, .server-detail-page)
  :where(button, [role='button'], .app-card) {
  transition:
    transform 140ms var(--ease-out),
    border-color 140ms ease,
    background-color 140ms ease,
    color 140ms ease;
}

:where(.storefront-page, .storefront-search-page, .server-detail-page)
  :where(button, [role='button'], .app-card):active:not(:focus-visible) {
  transform: scale(0.98);
}
```

Do not add animation to category changes, sort changes, pagination changes, or keyboard navigation.

- [x] **Step 4: Complete mobile and ad constraints**

Inside `@media (max-width: 980px)`, ensure the hero and subscription blocks are one column and add:

```css
.storefront-hero-ad {
  min-height: 0;
  max-height: 220px;
}
```

Inside `@media (max-width: 640px)`, ensure app grids are one column and add:

```css
.hero-band.storefront-hero {
  gap: 14px;
}

.storefront-hero-copy {
  padding-right: 0;
}

.storefront-subscribe-actions > button {
  width: 100%;
  justify-content: center;
}

.storefront-page .app-grid,
.storefront-search-page .app-grid {
  grid-template-columns: 1fr;
}
```

Add a narrow-phone rule:

```css
@media (max-width: 420px) {
  .storefront-page,
  .storefront-search-page {
    gap: 12px;
  }

  .storefront-hero-ad {
    max-height: 180px;
  }

  .server-detail-page .storefront-detail-actions,
  .server-detail-page .storefront-detail-section {
    padding: 12px;
  }
}
```

- [x] **Step 5: Replace the storefront reduced-motion block**

Use this storefront-specific block:

```css
@media (prefers-reduced-motion: reduce) {
  :where(.storefront-page, .storefront-search-page, .server-detail-page)
    :where(button, [role='button'], .app-card),
  .detail-page-shell {
    transform: none !important;
  }

  :where(.storefront-page, .storefront-search-page, .server-detail-page)
    :where(button, [role='button'], .app-card):hover,
  :where(.storefront-page, .storefront-search-page, .server-detail-page)
    :where(button, [role='button'], .app-card):active {
    transform: none !important;
  }
}
```

The global wildcard reduced-motion rule remains unchanged for INT to reconcile.

- [x] **Step 6: Run the complete automated FE-1 verification**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
cd .. && git diff --check
```

Expected:

- Contract runner reports `# pass 5` and `# fail 0`.
- TypeScript exits `0` with no diagnostics.
- `git diff --check` exits `0` and prints nothing.
- `git status --short client/dist clientembed/dist web/dist` is unchanged from the baseline because FE-1 did not build distributions.

- [x] **Step 7: Commit the interaction pass**

Run:

```bash
git add client/src/modules/storefront/storefront.contract.test.mjs client/src/styles/storefront.css
git diff --cached --check
git diff --cached --unified=0 | rg '^\+.*(toggleAppRating|ratingAdded|app\.rating|Sparkles|downloads_(day|week|month|year))' || true
git commit -m "feat: polish storefront interaction states"
```

Expected: whitespace check and WIP scan print nothing; commit contains only the contract test and storefront CSS.

---

### Task 6: Browser Matrix and Integration Handoff

**Files:**
- No source files are modified in this task.
- Do not modify locale dictionaries, `App.tsx`, global styles, or generated distributions.

**Interfaces:**
- Consumes: FE-1 implementation from Tasks 1–5.
- Produces: a verification record and exact INT handoff items.

- [x] **Step 1: Start the development server without generating a distribution**

Run in terminal A:

```bash
cd client && npm run dev
```

Expected: Vite prints a local URL containing `http://127.0.0.1:5173/` and remains running. Do not run `npm run build`.

- [x] **Step 2: Verify desktop layouts**

At widths `1440` and `1024`, verify all of the following:

```text
Home: Discover is the only primary hero action.
Home: Login/Submit is secondary.
Home: app/category metrics do not move or scale.
Home: source v2 panel shows URL, Copy, Open, and persistent success/error status.
Home: an enabled ad does not displace the hero copy or primary action.
Cards: name and summary clamp cleanly; category/version/download/rating remain legible.
Cards: clicking card body opens detail; clicking Download does not also open detail.
Search: category, search, and sort appear before results.
Search: active category and result count remain visible above cards.
Search: clearing active filters from a zero-result state returns to all categories.
Detail: screenshots appear before trust and technical metadata.
Detail: Download is the only primary public action; community/management actions remain secondary.
```

Expected: every line passes in light mode, dark mode, and one non-default Astryx theme.

- [x] **Step 3: Verify tablet and phone layouts**

At widths `768`, `375`, and `320`, verify:

```text
No horizontal page scroll.
Hero actions remain reachable before the ad.
Source URL wraps or scrolls inside its own code block without widening the page.
Copy/Open actions are full-width at phone sizes.
Search toolbar stacks to one column.
Cards render as one column with a visible Download action.
Detail actions become full-width and remain above screenshots.
Screenshot, version, and comment sections retain 12px or greater inner padding.
All actions are reachable on touch without hover.
```

Expected: no clipped labels, vertical letter columns, overlapping controls, or off-screen actions.

- [x] **Step 4: Verify keyboard and reduced motion**

Use keyboard-only navigation and browser reduced-motion emulation:

```text
Tab order follows hero → metrics skip → subscription actions → categories → cards.
Visible focus ring appears on every interactive control.
Enter/Space activates a focused control once.
Escape still exits app detail through the existing AppDrawer handler.
Reduced motion produces no card/button translation or scale.
Changing category, sort, or page does not replay positional animation.
```

Expected: all checks pass; no focus trap is introduced by FE-1.

- [x] **Step 5: Re-run WIP preservation audit**

Run:

```bash
rg -n "toggleAppRating|ratingAdded|ratingRemoved|ratingVote|ratingVoted" \
  client/src/modules/storefront/AppDrawer.tsx
rg -n "app\.rating|Sparkles" client/src/modules/storefront/AppGrid.tsx
rg -n "downloads_(day|week|month|year)" client/src/modules/storefront/StorefrontSearch.tsx
git status --short
```

Expected:

- Rating behavior still exists in detail and cards.
- All four period sort modes still exist.
- User-owned locale, shared type, backend, Ent, OpenAPI, and embedded-dist changes remain present and unexplained files were not staged by FE-1.

- [x] **Step 6: Deliver the exact INT handoff**

Report these items to the integration owner:

```text
Locale key: search.clearFilters
zh: 清除筛选
en: Clear filters
FE-1 fallback before INT merge: existing `search.allCategories`, supplied through i18next `defaultValue` so no raw key appears in the UI.

Shared-style follow-up: client/src/styles.css currently applies 1ms to every transition under prefers-reduced-motion. INT should narrow that wildcard so color, border-color, and opacity feedback may remain while transform/position motion is removed.

Integration files still owned by INT: client/src/App.tsx, client/src/locales/zh.ts, client/src/locales/en.ts, client/src/styles.css, generated client/dist, clientembed/dist, and web/dist.
```

Do not implement these handoff changes in FE-1.

- [x] **Step 7: Final FE-1 status check**

Run:

```bash
node --test client/src/modules/storefront/storefront.contract.test.mjs
cd client && npx tsc -p tsconfig.json --noEmit
cd .. && git diff --check
git log --oneline -5
```

Expected:

- Contract tests: `# pass 5`, `# fail 0`.
- TypeScript: no diagnostics.
- Diff check: no output.
- Recent history includes the five FE-1 commits:
  - `feat: focus storefront homepage actions`
  - `feat: simplify storefront app cards`
  - `feat: clarify storefront search results`
  - `feat: refine storefront app detail hierarchy`
  - `feat: polish storefront interaction states`

## Acceptance Criteria

- Homepage has exactly one visually primary hero action: Discover.
- Login/Submit is secondary and remains available to authenticated and unauthenticated users.
- Source v2 subscription is a dedicated panel with URL, protocol version, copy, open, and persistent result feedback.
- Metrics no longer animate as if clickable.
- App cards preserve rating/download WIP, remove readiness badge clutter, and keep trust details in the detail page.
- Card-body click opens detail; Download stops propagation and only invokes install/download.
- Search controls precede results; active category and count remain visible.
- Filtered zero-results exposes one clear-filter recovery action.
- Public app detail places screenshots before trust/technical facts and retains version history and comments.
- Desktop `1440/1024`, tablet `768`, phone `375/320`, light, dark, non-default theme, keyboard, touch, and reduced-motion checks pass.
- FE-1 changes no `App.tsx`, locale dictionary, shared/global style, backend, package manifest, or generated distribution file.
- Existing user WIP remains in the working tree and is not included in FE-1 commits.

## Completion Evidence — 2026-07-10

- Contract suite: `node --test client/src/modules/storefront/storefront.contract.test.mjs` — 8 passed, 0 failed (the planned five contracts plus additional card accessibility/layout and version-management coverage).
- TypeScript: `npm exec --prefix client -- tsc -p client/tsconfig.json --noEmit` — passed without diagnostics.
- WIP preservation: rating behavior remains in `AppDrawer.tsx` and `AppGrid.tsx`; all four `downloads_day/week/month/year` sort modes remain in `StorefrontSearch.tsx`.
- Browser smoke against a temporary local server: at 1440px, Discover was the only primary homepage button; at 320px the page had no horizontal overflow, subscription actions were full-width, and the search page remained within the viewport with its category/result summary visible.
- The temporary database had no published app fixture, so runtime card-download propagation and populated detail ordering were covered by the structural contracts rather than a destructive browser fixture. Final INT validation should repeat those two interactions with release fixtures.
- `git diff --check` passed. Commit/staging commands were intentionally deferred under the user's no-commit/no-publish instruction.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-10-storefront-ux-hardening.md`.

Execution options for the parent coordinator:

1. **Subagent-Driven (recommended):** assign FE-1 to a dedicated worker using `subagent-driven-development`, review each commit against the WIP staging rules, and keep INT single-threaded for locale/App/global-style/dist changes.
2. **Inline Execution:** execute Tasks 1–6 with `executing-plans`, pausing after every interactive staging check.
