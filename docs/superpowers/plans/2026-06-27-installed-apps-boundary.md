# Installed Apps Boundary Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the app-store "device" concept from the standalone client and make installed-app lookup explicitly use LazyCat SDK application management.

**Architecture:** Keep the existing single React client surface and LazyCat SDK adapter. The SDK adapter returns installed application records from `pkgm.QueryApplication({ appidList: [] })`; UI components render those records as an installed-app list, not as device state. This is a narrow terminology and contract correction with no server API changes.

**Tech Stack:** Vite, React, TypeScript, i18next, lucide-react, `@lazycatcloud/sdk`, CSS modules by project convention in `client/src/styles.css`.

---

## File Map

- Modify `client/src/lazycatSdk.ts`: expose a small installed application response type and call `QueryApplication` with `appidList: []`.
- Modify `client/src/App.tsx`: rename standalone installed-app variables, nav label key, readiness labels, and CSS class usage away from "device".
- Modify `client/src/i18n.ts`: replace visible device strings with installed-app strings in Chinese and English.
- Modify `client/src/styles.css`: rename `.device-state` to `.installed-state`.
- No server files change in this batch.

## Task 1: Correct LazyCat SDK Installed-App Query

**Files:**
- Modify: `client/src/lazycatSdk.ts`

- [ ] **Step 1: Add installed application response types**

In `client/src/lazycatSdk.ts`, add these types above `type InstallTarget`:

```ts
export type InstalledApplication = {
  appid?: string;
  title?: string;
  version?: string;
  status?: number;
};

export type InstalledApplicationsResponse = {
  infoList?: InstalledApplication[];
};
```

- [ ] **Step 2: Change `queryInstalledApplications` to the SDK documented request shape**

Replace the current function:

```ts
export async function queryInstalledApplications() {
  const gateway = await getGateway();
  return gateway.pkgm.QueryApplication({ deployIds: [] });
}
```

with:

```ts
export async function queryInstalledApplications(): Promise<InstalledApplicationsResponse> {
  const gateway = await getGateway();
  return gateway.pkgm.QueryApplication({ appidList: [] });
}
```

- [ ] **Step 3: Verify TypeScript still sees the adapter**

Run:

```bash
cd client && npm run build
```

Expected: the build reaches Vite production build without TypeScript errors.

## Task 2: Rename Standalone Client UI Concepts

**Files:**
- Modify: `client/src/App.tsx`

- [ ] **Step 1: Import the installed application type**

Replace:

```ts
import { installWithLazyCat, queryInstalledApplications } from './lazycatSdk';
```

with:

```ts
import { installWithLazyCat, queryInstalledApplications, type InstalledApplication } from './lazycatSdk';
```

- [ ] **Step 2: Rename standalone client nav to Installed**

Replace this `clientTabs` entry:

```ts
{ key: 'profile', labelKey: 'nav.device', icon: Server },
```

with:

```ts
{ key: 'profile', labelKey: 'nav.installed', icon: Archive },
```

- [ ] **Step 3: Use the shared installed application type**

Replace:

```ts
const [installedApps, setInstalledApps] = useState<Array<{ appid?: string; title?: string; version?: string; status?: number }>>([]);
```

with:

```ts
const [installedApps, setInstalledApps] = useState<InstalledApplication[]>([]);
```

- [ ] **Step 4: Rename readiness variables away from device**

Replace:

```ts
const deviceQueryReady = installedState === 'loaded';
```

with:

```ts
const installedLookupReady = installedState === 'loaded';
```

Replace:

```ts
const deviceReadinessBody =
  installedState === 'loaded'
    ? t('profile.clientDeviceLoaded', { count: installedApps.length })
    : installedState === 'loading'
      ? t('profile.clientDeviceLoading')
      : installedState === 'error'
        ? installedError || t('profile.clientDeviceError')
        : t('profile.clientDeviceIdle');
```

with:

```ts
const installedReadinessBody =
  installedState === 'loaded'
    ? t('profile.clientInstalledLoaded', { count: installedApps.length })
    : installedState === 'loading'
      ? t('profile.clientInstalledLoading')
      : installedState === 'error'
        ? installedError || t('profile.clientInstalledError')
        : t('profile.clientInstalledIdle');
```

- [ ] **Step 5: Rename readiness markup keys**

In the standalone client readiness card, replace:

```tsx
<div className={cx('readiness-step', deviceQueryReady && 'ready')}>
  <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loading' ? 'pending' : deviceQueryReady ? 'synced' : 'unsynced')}>
    {installedState === 'error' ? <AlertCircle size={14} /> : deviceQueryReady ? <Check size={14} /> : <Gauge size={14} />}
    {t(`profile.deviceState.${installedState}`)}
  </span>
  <strong>{t('profile.clientDeviceTitle')}</strong>
  <small>{deviceReadinessBody}</small>
</div>
```

with:

```tsx
<div className={cx('readiness-step', installedLookupReady && 'ready')}>
  <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loading' ? 'pending' : installedLookupReady ? 'synced' : 'unsynced')}>
    {installedState === 'error' ? <AlertCircle size={14} /> : installedLookupReady ? <Check size={14} /> : <Gauge size={14} />}
    {t(`profile.installedState.${installedState}`)}
  </span>
  <strong>{t('profile.clientInstalledTitle')}</strong>
  <small>{installedReadinessBody}</small>
</div>
```

- [ ] **Step 6: Rename installed-app panel title, help, state class, and state key**

Replace:

```tsx
<h2>{t('profile.clientDeviceTitle')}</h2>
<p>{t('profile.clientDeviceHelp')}</p>
<div className={cx('device-state', installedState)}>
  <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loaded' ? 'synced' : 'unsynced')}>
    {t(`profile.deviceState.${installedState}`)}
  </span>
  {installedState === 'error' && <small>{installedError}</small>}
</div>
```

with:

```tsx
<h2>{t('profile.clientInstalledTitle')}</h2>
<p>{t('profile.clientInstalledHelp')}</p>
<div className={cx('installed-state', installedState)}>
  <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loaded' ? 'synced' : 'unsynced')}>
    {t(`profile.installedState.${installedState}`)}
  </span>
  {installedState === 'error' && <small>{installedError}</small>}
</div>
```

- [ ] **Step 7: Verify no stale App.tsx device identifiers remain in the installed-app flow**

Run:

```bash
rg -n "clientDevice|deviceState|deviceQueryReady|deviceReadinessBody|device-state|nav\\.device" client/src/App.tsx
```

Expected: no output.

## Task 3: Update Bilingual Copy And CSS Class

**Files:**
- Modify: `client/src/i18n.ts`
- Modify: `client/src/styles.css`

- [ ] **Step 1: Replace nav key in Chinese and English**

In `client/src/i18n.ts`, replace both `nav.device` entries with `nav.installed`.

Chinese nav block should include:

```ts
installed: '已安装',
```

English nav block should include:

```ts
installed: 'Installed',
```

- [ ] **Step 2: Replace Chinese profile installed-app keys**

Replace Chinese `clientDevice*` keys and `deviceState` with:

```ts
clientInstalledTitle: '已安装应用',
clientInstalledHelp: '读取 LazyCat SDK 返回的已安装应用列表，安装前可对照当前应用状态。',
clientInstalledIdle: '在 LazyCat 客户端内读取后，会显示已安装应用。',
clientInstalledLoading: '正在读取 LazyCat 已安装应用列表。',
clientInstalledLoaded: '已读取到 {{count}} 个已安装应用。',
clientInstalledError: '当前环境无法访问 LazyCat SDK。',
```

Replace:

```ts
installedEmptyLoaded: '这台设备还没有返回已安装应用',
installedIdleBody: '在 LazyCat 客户端内读取后，这里会显示设备上已经安装的应用。',
```

with:

```ts
installedEmptyLoaded: '当前客户端没有返回已安装应用',
installedIdleBody: '读取后，这里会显示已安装应用列表。',
```

Replace the Chinese `deviceState` object with:

```ts
installedState: {
  idle: '等待读取',
  loading: '读取中',
  loaded: '已读取',
  error: 'SDK 不可用',
},
```

- [ ] **Step 3: Replace English profile installed-app keys**

Replace English `clientDevice*` keys and `deviceState` with:

```ts
clientInstalledTitle: 'Installed apps',
clientInstalledHelp: 'Read the installed app list returned by the LazyCat SDK before installing new apps.',
clientInstalledIdle: 'Read inside the LazyCat client to show installed apps.',
clientInstalledLoading: 'Reading the LazyCat installed app list.',
clientInstalledLoaded: '{{count}} installed apps were returned.',
clientInstalledError: 'LazyCat SDK is not available in this environment.',
```

Replace:

```ts
installedEmptyLoaded: 'This device did not return installed apps',
installedIdleBody: 'Read from inside the LazyCat client to list apps already installed on this device.',
```

with:

```ts
installedEmptyLoaded: 'The current client did not return installed apps',
installedIdleBody: 'After reading, installed apps will appear here.',
```

Replace the English `deviceState` object with:

```ts
installedState: {
  idle: 'Waiting',
  loading: 'Reading',
  loaded: 'Loaded',
  error: 'SDK unavailable',
},
```

- [ ] **Step 4: Rename CSS class**

In `client/src/styles.css`, replace:

```css
.device-state {
```

with:

```css
.installed-state {
```

Replace:

```css
.device-state small {
```

with:

```css
.installed-state small {
```

- [ ] **Step 5: Verify user-facing Chinese only lives in i18n**

Run:

```bash
rg -n "[\\p{Han}]" client/src --glob '!i18n.ts' || true
```

Expected: no output.

- [ ] **Step 6: Verify no app-store device wording remains outside correction docs**

Run:

```bash
rg -n "Device|This device|current device|clientDevice|deviceState|device-state|设备|当前设备|这台设备" client/src
```

Expected: no output except unrelated `Server API` strings are not matched by this command.

## Task 4: Build, Browser Smoke, Commit, And Push

**Files:**
- Verify: `client/src/lazycatSdk.ts`
- Verify: `client/src/App.tsx`
- Verify: `client/src/i18n.ts`
- Verify: `client/src/styles.css`

- [ ] **Step 1: Run full static verification**

Run:

```bash
cd client && npm run build
cd .. && go test ./...
rg -n "[\\p{Han}]" client/src --glob '!i18n.ts' || true
rg -n "Device|This device|current device|clientDevice|deviceState|device-state|设备|当前设备|这台设备" client/src
git diff --check
```

Expected:

- `npm run build` exits 0.
- `go test ./...` exits 0.
- Chinese literal scan prints no output.
- Device wording scan prints no output.
- `git diff --check` exits 0.

- [ ] **Step 2: Start standalone client dev server**

Run:

```bash
cd client
VITE_API_BASE_URL= npm run dev -- --host 127.0.0.1 --port 5215
```

Expected: Vite serves the app at `http://127.0.0.1:5215/`.

- [ ] **Step 3: Seed local source data in the browser**

Run with agent-browser:

```bash
agent-browser open http://127.0.0.1:5215
cat <<'JS' | agent-browser eval --stdin
localStorage.setItem('i18nextLng', 'en');
localStorage.setItem('lazycat.sources', JSON.stringify([{
  id: 'source-1',
  name: 'Community',
  url: 'https://store.example/source/v1/index.json',
  password: '',
  mirror: '',
  lastSync: new Date().toISOString(),
  lastAppCount: 1,
  lastInstallableCount: 1
}]));
localStorage.setItem('lazycat.sourceApps', JSON.stringify([{
  id: 1,
  sourceId: 'source-1',
  sourceName: 'Community',
  name: 'NAS Notes',
  slug: 'nas-notes',
  summary: 'Private notes on NAS',
  category: 'Productivity',
  latestVersion: {
    version: '1.0.0',
    downloadUrl: 'https://example.com/nas-notes.lpk',
    sha256: 'dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd',
    size: 1024
  }
}]));
location.reload();
JS
agent-browser wait --load networkidle
agent-browser snapshot -i
```

Expected: the standalone nav includes `Sources`, `Install`, and `Installed`; it does not include `Device`.

- [ ] **Step 4: Verify installed page and mobile overflow**

Run:

```bash
agent-browser find text "Installed" click
agent-browser wait --text "Installed apps"
cat <<'JS' | agent-browser eval --stdin
(() => {
  const bodyText = document.body.innerText;
  return {
    hasInstalledApps: bodyText.includes('Installed apps'),
    hasDevice: bodyText.includes('Device') || bodyText.includes('This device'),
    overflow: document.documentElement.scrollWidth > window.innerWidth
  };
})()
JS
agent-browser viewport 390 844
cat <<'JS' | agent-browser eval --stdin
(() => ({
  width: window.innerWidth,
  scrollWidth: document.documentElement.scrollWidth,
  overflow: document.documentElement.scrollWidth > window.innerWidth
}))()
JS
agent-browser close
```

Expected:

- `hasInstalledApps` is `true`.
- `hasDevice` is `false`.
- `overflow` is `false` at the default and 390px viewport.

- [ ] **Step 5: Commit and push**

Run:

```bash
git status --short
git add client/src/lazycatSdk.ts client/src/App.tsx client/src/i18n.ts client/src/styles.css docs/superpowers/plans/2026-06-27-installed-apps-boundary.md
git commit -m "feat: align installed apps boundary"
git push
```

Expected:

- Commit includes the code changes and this implementation plan.
- Push updates `origin/main`.
