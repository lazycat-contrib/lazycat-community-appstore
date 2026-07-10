# Standalone Client UX Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让独立客户端在已有软件源时直接进入应用发现，无源时提供单一明确的首次引导，并把软件源、安装、已安装应用、历史和设置流程改造成状态清楚、反馈持久、移动端可用的体验。

**Architecture:** FE-2 只修改 `client/src/modules/client/**` 与 `client/src/styles/client.css`，用小型纯函数模块统一安装时间线和设置脏状态，以可测试的派生状态驱动页面。`client/src/App.tsx`、导航、共享类型、语言包、共享全局样式和所有生成目录由 INT 单线程集成；FE-2 通过兼容的可选 props 和明确的集成契约避免并行冲突。

**Tech Stack:** React 19、TypeScript 5.9、Vite 7、Astryx Design、lucide-react、react-i18next、Node 24+ 内置 test runner。

## Global Constraints

- 目标版本：React 19、Vite 7、Astryx Design；不新增第二套设计系统，不新增前端运行时依赖。
- FE-2 独占 `client/src/modules/client/**`、`client/src/styles/client.css`；不得编辑 `client/src/App.tsx`、`client/src/modules/shell/navigation.ts`、`client/src/shared/**`、`client/src/locales/zh.ts`、`client/src/locales/en.ts`、`client/src/styles.css`、商店或后台模块。
- 当前 `client/src/locales/{zh,en}.ts`、`clientembed/dist/**` 已有未提交 WIP；不得暂存、覆盖或重建这些改动。
- FE-2 不运行 `npm run build`，因为它会生成 `client/dist/**`；每个任务使用 `npm exec --prefix client -- tsc -b client/tsconfig.json` 做无产物类型检查。Vite build 与三处 dist 同步只由 INT 最后执行一次。
- 每屏只有一个视觉主操作；高频导航和设置标签不做位移动画。
- 可按压控件的反馈为 100–160ms、`scale(0.97–0.99)`；hover 仅在 `@media (hover: hover) and (pointer: fine)` 下启用。
- 弹窗进入只使用 180–240ms 强 `ease-out`、`scale(0.96–0.98)` 和透明度；不得从 `scale(0)` 出现。
- `prefers-reduced-motion` 移除位移和缩放，仅保留必要颜色或透明度反馈。
- Toast 只用于短暂成功确认；软件源同步失败、安装失败、历史刷新失败和设置保存失败必须有持续的就地错误与恢复操作。
- 安装 API 当前在系统 SDK 完成或失败后同步返回，不能观测真实字节下载百分比；运行中使用 indeterminate progress，不制造虚假 12%/42% 数字。
- 所有新增交互必须防止重复提交，并在触屏设备上无需 hover 即可到达。

---

## File Structure

| File | Responsibility |
| --- | --- |
| `client/src/modules/client/clientUxState.ts` | 纯函数：安装阶段时间线、可编辑设置归一化与脏状态比较；不依赖 DOM 或 React。 |
| `client/src/modules/client/clientUxState.test.mjs` | 使用 Node 24+ 内置 test runner 验证纯函数，不引入 Vitest/Jest。 |
| `client/src/modules/client/ClientCatalog.tsx` | 独立客户端发现页的信息层级、结果摘要、筛选与安装命令。 |
| `client/src/modules/client/SourceAppGrid.tsx` | 紧凑应用卡片、安装/更新/重装动作、防重复点击和事件冒泡。 |
| `client/src/modules/client/SourceOnboarding.tsx` | 无软件源时唯一主操作的首次使用引导。 |
| `client/src/modules/client/SourceStatusRow.tsx` | 紧凑软件源状态行、错误恢复、群组/应用数与操作。 |
| `client/src/modules/client/SourcesView.tsx` | 软件源增删改同步编排、持久错误、确认删除；不再重复承载应用目录。 |
| `client/src/modules/client/InstallActivityPanel.tsx` | 持久安装状态、真实不确定进度、阶段时间线和失败恢复。 |
| `client/src/modules/client/InstallOptionsDialog.tsx` | 密码/镜像提交 pending、防重复提交和就地错误。 |
| `client/src/modules/client/InstalledAppsView.tsx` | 按可更新、软件源托管、本地未知来源分组展示已安装应用。 |
| `client/src/modules/client/ClientHistoryView.tsx` | 历史刷新 pending、当前页统计和持久刷新错误。 |
| `client/src/modules/client/SourceAppDetailPage.tsx` | 保持全页详情；强化唯一安装主操作、回滚语义和移动端动作区。 |
| `client/src/modules/client/ClientSettingsView.tsx` | 脏状态、保存中、已保存、失败四态及固定可见保存区。 |
| `client/src/styles/client.css` | 上述客户端页面的局部布局、状态、响应式、触屏与 reduced-motion 覆盖。 |

## UX Change Summary

| Before | After | Why |
| --- | --- | --- |
| 客户端固定从软件源管理开始 | 有源进入发现，无源进入单一添加源引导 | 普通用户的默认任务是发现和安装，源管理是安静配置 |
| 发现页 6 个摘要指标、应用卡片堆叠多枚 readiness 徽章 | 3 个任务相关指标；卡片只保留名称、摘要、来源、分类、版本、安装状态和一个命令 | 减少浏览时的认知负担，信任详情留在详情页 |
| 软件源页同时包含源管理和完整应用目录 | 只保留源状态、同步、编辑和删除；应用浏览回到发现页 | 一个页面只服务一个主要任务 |
| 删除源依赖“点两次 + Toast” | 显示对象名和后果的确认对话框 | 危险操作必须明确且可撤销退出 |
| 安装用模拟 12%/42% 的 determinate 进度 | SDK 等待期间为 indeterminate，结束后显示真实结果与任务信息 | 不向用户展示虚假精度 |
| 已安装应用是无分组平铺列表 | 更新可用、来源已知、本地未知来源分组 | 用户先看到需要处理的应用，并理解来源边界 |
| 设置页上下两个保存按钮，只有 saving 状态 | 单一稳定保存区，显示未保存/保存中/已保存/失败 | 避免重复主操作并持续表达表单状态 |
| 设置标签每次进入都有位移动画 | 标签即时切换，仅保留颜色/焦点反馈 | 高频操作应感觉立即响应 |

---

### Task 1: Add a tested client UX state model

**Files:**
- Create: `client/src/modules/client/clientUxState.test.mjs`
- Create: `client/src/modules/client/clientUxState.ts`

**Interfaces:**
- Consumes: structural objects with `status`, `stageKey`, and editable client-setting fields; no import from shared files.
- Produces: `buildInstallTimeline(activity: InstallActivitySnapshot): InstallTimelineItem[]`, `normalizeEditableClientSettings(settings: EditableClientSettings): EditableClientSettings`, and `sameEditableClientSettings(left, right): boolean`.

- [x] **Step 1: Write the failing pure-function tests**

Create `client/src/modules/client/clientUxState.test.mjs`:

```js
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildInstallTimeline,
  normalizeEditableClientSettings,
  sameEditableClientSettings,
} from './clientUxState.ts';

test('install timeline exposes the system step without inventing determinate progress', () => {
  assert.deepEqual(
    buildInstallTimeline({ status: 'running', stageKey: 'installActivity.stageSystem' }),
    [
      { key: 'queued', state: 'complete' },
      { key: 'prepare', state: 'complete' },
      { key: 'system', state: 'current' },
      { key: 'result', state: 'pending' },
    ],
  );
});

test('failed installs leave the result stage in error state', () => {
  assert.deepEqual(buildInstallTimeline({ status: 'error', stageKey: 'installActivity.stageFailed' }).at(-1), {
    key: 'result',
    state: 'error',
  });
});

test('editable settings comparison ignores server-owned sync result fields', () => {
  const base = {
    clientTitle: 'MiaoMiao',
    commentDisplayName: 'Cat',
    defaultPageSize: 24,
    autoSyncEnabled: true,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
    installSuccessDismissSeconds: 3,
    lastAutoSyncAt: '2026-07-10T00:00:00Z',
    lastAutoSyncStatus: 'success',
  };
  assert.equal(
    sameEditableClientSettings(base, { ...base, lastAutoSyncAt: '2026-07-11T00:00:00Z', lastAutoSyncStatus: 'partial' }),
    true,
  );
});

test('editable settings normalization trims strings and applies numeric defaults', () => {
  assert.deepEqual(
    normalizeEditableClientSettings({
      clientTitle: '  MiaoMiao  ',
      commentDisplayName: '  Cat  ',
      defaultPageSize: 0,
      autoSyncEnabled: true,
      autoSyncIntervalMinutes: 0,
      syncOnStartup: true,
      installSuccessDismissSeconds: Number.NaN,
    }),
    {
      clientTitle: 'MiaoMiao',
      commentDisplayName: 'Cat',
      defaultPageSize: 24,
      autoSyncEnabled: true,
      autoSyncIntervalMinutes: 60,
      syncOnStartup: true,
      installSuccessDismissSeconds: 3,
    },
  );
});
```

- [x] **Step 2: Run the tests to verify the module is missing**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
```

Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `clientUxState.ts`.

- [x] **Step 3: Implement the state model**

Create `client/src/modules/client/clientUxState.ts`:

```ts
export type InstallActivitySnapshot = {
  status: 'running' | 'success' | 'error';
  stageKey: string;
};

export type InstallTimelineKey = 'queued' | 'prepare' | 'system' | 'result';
export type InstallTimelineState = 'pending' | 'current' | 'complete' | 'error';
export type InstallTimelineItem = { key: InstallTimelineKey; state: InstallTimelineState };

const installTimelineOrder: InstallTimelineKey[] = ['queued', 'prepare', 'system', 'result'];

function activeInstallStage(stageKey: string): InstallTimelineKey {
  if (stageKey.endsWith('stageDone') || stageKey.endsWith('stageFailed')) return 'result';
  if (stageKey.endsWith('stageSystem') || stageKey.endsWith('stageHandoff') || stageKey.endsWith('stageVerify')) return 'system';
  if (stageKey.endsWith('stagePrepare')) return 'prepare';
  return 'queued';
}

export function buildInstallTimeline(activity: InstallActivitySnapshot): InstallTimelineItem[] {
  const active = activeInstallStage(activity.stageKey);
  const activeIndex = installTimelineOrder.indexOf(active);
  return installTimelineOrder.map((key, index) => {
    if (index < activeIndex) return { key, state: 'complete' };
    if (index > activeIndex) return { key, state: 'pending' };
    if (key === 'result' && activity.status === 'error') return { key, state: 'error' };
    if (activity.status === 'success') return { key, state: 'complete' };
    return { key, state: 'current' };
  });
}

export type EditableClientSettings = {
  clientTitle: string;
  commentDisplayName: string;
  defaultPageSize: number;
  autoSyncEnabled: boolean;
  autoSyncIntervalMinutes: number;
  syncOnStartup: boolean;
  installSuccessDismissSeconds: number;
  lastAutoSyncAt?: string;
  lastAutoSyncStatus?: string;
  lastAutoSyncError?: string;
};

export function normalizeEditableClientSettings(settings: EditableClientSettings): EditableClientSettings {
  return {
    clientTitle: settings.clientTitle.trim(),
    commentDisplayName: settings.commentDisplayName.trim(),
    defaultPageSize: Number(settings.defaultPageSize) || 24,
    autoSyncEnabled: Boolean(settings.autoSyncEnabled),
    autoSyncIntervalMinutes: Number(settings.autoSyncIntervalMinutes) || 60,
    syncOnStartup: Boolean(settings.syncOnStartup),
    installSuccessDismissSeconds: Number.isFinite(Number(settings.installSuccessDismissSeconds))
      ? Number(settings.installSuccessDismissSeconds)
      : 3,
  };
}

export function sameEditableClientSettings(left: EditableClientSettings, right: EditableClientSettings) {
  return JSON.stringify(normalizeEditableClientSettings(left)) === JSON.stringify(normalizeEditableClientSettings(right));
}
```

- [x] **Step 4: Run unit tests and type checking**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
```

Expected: 4 tests PASS; TypeScript exits 0 without creating `client/dist`.

- [~] **Step 5: Commit the state model**

```bash
git add client/src/modules/client/clientUxState.ts client/src/modules/client/clientUxState.test.mjs
git commit -m "test: define standalone client ux state"
```

---

### Task 2: Make discovery the focused client task

**Files:**
- Modify: `client/src/modules/client/ClientCatalog.tsx`
- Modify: `client/src/modules/client/SourceAppGrid.tsx`
- Modify: `client/src/styles/client.css`

**Interfaces:**
- Consumes: existing `SourceApp`, `SourceSubscription`, `InstalledApplication`, `sourceInstallAction`, `sourceActionLabel`, and `onInstall(app, options?)`.
- Produces: `SourceAppGrid` keeps its existing required props and adds optional `activeInstallKey?: string`; app identity is `${app.sourceId ?? app.sourceName}:${app.id}`.

- [x] **Step 1: Record the pre-change source contract**

Run:

```bash
rg -n "app-readiness|checksumReady|sizeReady|updateAllSourceApps|client-summary-grid" client/src/modules/client/ClientCatalog.tsx client/src/modules/client/SourceAppGrid.tsx
```

Expected: matches show six summary metrics, batch update, and multiple readiness badges. Save this output in the task log; it is the failing UX contract this task removes.

- [x] **Step 2: Reduce the discovery header and summary to task-relevant information**

In `ClientCatalog.tsx`:

1. Remove `updateAllSourceApps`, the `updateAll` button, and unused `RefreshCw` import.
2. Keep “管理软件源” as `variant="secondary"` in the page heading.
3. Replace the six-card summary with exactly three non-interactive cells:

```tsx
<div className="client-summary-grid client-discovery-summary" aria-label={t('search.installReadiness')}>
  <div>
    <span>{t('search.sourcesTotal')}</span>
    <strong>{sourceStats.sourceCount}</strong>
  </div>
  <div>
    <span>{t('search.syncedAppsTotal')}</span>
    <strong>{sourceStats.sourceAppCount}</strong>
  </div>
  <div className={cx(updateSourceApps.length > 0 && 'warning')}>
    <span>{t('search.updatesAvailable')}</span>
    <strong>{updateSourceApps.length}</strong>
  </div>
</div>
```

4. Keep search, sort, category and result count before the grid. If `sources.length === 0`, the existing empty action must remain the only primary action in the result area.

- [x] **Step 3: Simplify each app card and make install an independent command**

In `SourceAppGrid.tsx`:

1. Add `activeInstallKey` to destructuring and add `activeInstallKey?: string` to the prop object type; callers that do not pass it remain source-compatible.
2. Replace the readiness badge cluster with a single derived status badge only when installed or updateable.
3. Keep source, category, version, name and two-line summary.
4. Add local pending state and stop the install click from opening the detail page:

```tsx
const [pendingAppKey, setPendingAppKey] = useState('');

async function runInstall(app: SourceApp, appKey: string) {
  if (pendingAppKey) return;
  setPendingAppKey(appKey);
  try {
    await Promise.resolve(onInstall(app));
  } finally {
    setPendingAppKey('');
  }
}
```

Use this card body inside `apps.map`:

```tsx
const appKey = `${app.sourceId ?? app.sourceName}:${app.id}`;
const isPending = pendingAppKey === appKey || activeInstallKey === appKey;

<XClickableCard
  className="source-app-card client-source-app-card"
  key={appKey}
  label={t('app.open', { name: appName })}
  onClick={() => onOpen(app)}
  padding={3}
>
  <div className="app-open">
    <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={appName} />
    <div>
      <h3>{appName}</h3>
      <p>{appSummary}</p>
    </div>
    <ChevronRight size={18} />
  </div>
  <div className="client-app-facts">
    <span><Cloud size={14} /> {app.sourceName}</span>
    <span><Tag size={14} /> {localizedCategory(app, t('common.uncategorized'))}</span>
    <span><Archive size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
  </div>
  {installedMatch && (
    <StatusBadge
      tone={isUpdateAvailable ? 'stale' : 'synced'}
      label={isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
    />
  )}
  <XButton
    type="button"
    variant="primary"
    label={isPending ? t('installActivity.status.running') : sourceActionLabel(t, installAction)}
    icon={isPending ? <RefreshCw size={17} className="spin" /> : isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
    isDisabled={!installable || Boolean(pendingAppKey) || Boolean(activeInstallKey)}
    onClick={(event) => {
      event.stopPropagation();
      void runInstall(app, appKey);
    }}
    aria-label={installable ? t('app.install', { name: appName }) : t('app.installUnavailable', { name: appName })}
  />
</XClickableCard>
```

Update imports to include `useState`, `StatusBadge`, and remove unused trust-detail icons and `XBadge`.

- [x] **Step 4: Add discovery-local styles**

Append to `client/src/styles/client.css`:

```css
.client-discovery-summary {
  grid-template-columns: repeat(3, minmax(0, 180px));
}

.client-source-app-card {
  min-height: 220px;
  grid-template-rows: auto minmax(0, 1fr) auto auto;
}

.client-app-facts {
  min-width: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  color: var(--muted);
  font-size: 13px;
}

.client-app-facts span {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

@media (max-width: 640px) {
  .client-discovery-summary {
    grid-template-columns: repeat(3, minmax(118px, 1fr));
    overflow-x: auto;
    overscroll-behavior-inline: contain;
    padding-bottom: 2px;
  }
}
```

- [x] **Step 5: Verify discovery compiles and the removed density does not return**

Run:

```bash
npm exec --prefix client -- tsc -b client/tsconfig.json
! rg -n "app-readiness|checksumReady|sizeReady|updateAllSourceApps" client/src/modules/client/ClientCatalog.tsx client/src/modules/client/SourceAppGrid.tsx
```

Expected: TypeScript exits 0; the second command returns no matches.

- [~] **Step 6: Commit focused discovery**

```bash
git add client/src/modules/client/ClientCatalog.tsx client/src/modules/client/SourceAppGrid.tsx client/src/styles/client.css
git commit -m "feat: focus standalone client discovery"
```

---

### Task 3: Turn software sources into a compact configuration workflow

**Files:**
- Create: `client/src/modules/client/SourceOnboarding.tsx`
- Create: `client/src/modules/client/SourceStatusRow.tsx`
- Modify: `client/src/modules/client/SourcesView.tsx`
- Modify: `client/src/styles/client.css`

**Interfaces:**
- Consumes: existing `SourceSubscription`, `SourceHealth`, `formatDate`, `SourceInput`, and source CRUD/sync callbacks.
- Produces: `SourceOnboarding({ onAdd })`; `SourceStatusRow({ source, health, appCount, installableCount, isSyncing, error, onSync, onEdit, onDelete })`; `SourcesView.onSyncAll` accepts `Promise<void | { success: number; failed: number }>` for backward-compatible INT threading.

- [x] **Step 1: Add the no-source onboarding component**

Create `client/src/modules/client/SourceOnboarding.tsx`:

```tsx
import { Cloud, Plus } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { useTranslation } from 'react-i18next';

export function SourceOnboarding({ onAdd }: { onAdd: () => void }) {
  const { t } = useTranslation();
  return (
    <section className="client-source-onboarding" aria-labelledby="client-source-onboarding-title">
      <span className="client-source-onboarding-icon" aria-hidden="true"><Cloud size={28} /></span>
      <div>
        <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
        <h1 id="client-source-onboarding-title">{t('sources.onboardingTitle')}</h1>
        <p>{t('sources.onboardingBody')}</p>
      </div>
      <XButton type="button" variant="primary" label={t('sources.add')} icon={<Plus size={18} />} onClick={onAdd} />
    </section>
  );
}
```

- [x] **Step 2: Add the compact source status row**

Create `client/src/modules/client/SourceStatusRow.tsx`:

```tsx
import { AlertCircle, KeyRound, Pencil, RefreshCw, Server, Trash2, UsersRound } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { useTranslation } from 'react-i18next';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { SourceHealth, SourceSubscription } from '../../shared/types';
import { cx, formatDate } from '../../shared/utils';

export function SourceStatusRow({
  source,
  health,
  appCount,
  installableCount,
  isSyncing,
  error,
  onSync,
  onEdit,
  onDelete,
}: {
  source: SourceSubscription;
  health: SourceHealth;
  appCount: number;
  installableCount: number;
  isSyncing: boolean;
  error?: string;
  onSync: () => void | Promise<void>;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  const groupCount = source.groups?.length || source.groupCodes?.length || 0;
  const issue = error || source.lastError || '';
  return (
    <article className={cx('client-source-row', issue && 'has-error')} aria-busy={isSyncing}>
      <div className="client-source-row-main">
        <span className="client-source-row-icon" aria-hidden="true"><Server size={19} /></span>
        <div>
          <div className="client-source-row-title">
            <strong>{source.name}</strong>
            <StatusBadge tone={health} label={t(`sources.health.${health}`)} />
          </div>
          <span className="client-source-url" title={source.url}>{source.url}</span>
          <div className="client-source-facts">
            <span>{source.lastSync ? t('sources.lastSync', { time: formatDate(source.lastSync) }) : t('sources.neverSynced')}</span>
            <span>{t('sources.syncedAppCount', { count: appCount })}</span>
            <span>{t('sources.installableAppCount', { count: installableCount })}</span>
            <span><UsersRound size={14} /> {t('sources.groupCount', { count: groupCount })}</span>
          </div>
        </div>
      </div>
      {issue && (
        <div className="client-source-error" role="alert">
          {source.lastErrorCode === 'auth' ? <KeyRound size={16} /> : <AlertCircle size={16} />}
          <span>{issue}</span>
          <XButton
            type="button"
            variant="secondary"
            size="sm"
            label={t('common.retry')}
            icon={<RefreshCw size={16} />}
            isDisabled={isSyncing}
            onClick={() => void onSync()}
          />
        </div>
      )}
      <div className="client-source-row-actions">
        <XButton
          type="button"
          variant="secondary"
          size="sm"
          label={isSyncing ? t('sources.health.syncing') : t('common.sync')}
          icon={<RefreshCw size={16} className={isSyncing ? 'spin' : undefined} />}
          isDisabled={isSyncing}
          onClick={() => void onSync()}
        />
        <XIconButton type="button" variant="ghost" label={t('sources.editTitle')} icon={<Pencil size={17} />} onClick={onEdit} />
        <XIconButton type="button" variant="destructive" label={t('sources.deleteSource', { name: source.name })} icon={<Trash2 size={17} />} onClick={onDelete} />
      </div>
    </article>
  );
}
```

- [x] **Step 3: Refactor SourcesView around source operations only**

In `SourcesView.tsx`:

1. Remove imports and state used only by the duplicated app catalog or selectable-card filter: `Download`, `Pagination`, `SelectableCard`, `Selector`, `Skeleton`, `ToggleButton`, `AdSpot`, `CategoryBrowser`, `SourceAppGrid`, `SourceHealthFilter`, page-size constants, `confirmDeleteSource`, `sourceHealthFilter`, `selectedSourceID`, selected synced source/category and synced pagination.
2. Keep `sourceApps` only to calculate per-source live counts.
3. Add imports for `SourceOnboarding` and `SourceStatusRow`.
4. Replace operation state with:

```tsx
const [syncingID, setSyncingID] = useState<SourceID | null>(null);
const [syncingAll, setSyncingAll] = useState(false);
const [savingSource, setSavingSource] = useState(false);
const [formError, setFormError] = useState('');
const [sourceErrors, setSourceErrors] = useState<Record<string, string>>({});
const [deleteTarget, setDeleteTarget] = useState<SourceSubscription | null>(null);
const [syncAllResult, setSyncAllResult] = useState<{ success: number; failed: number } | null>(null);
```

5. Widen the prop type to `onSyncAll: () => Promise<void | { success: number; failed: number }>`.
6. At the start of `addSource` and `saveEditedSource`, return if `savingSource`, clear `formError`, set pending, and always reset pending in `finally`. In `addSource`, use `setFormError(errorMessage(error, t('sources.invalid')))`; in `saveEditedSource`, use `setFormError(errorMessage(error, t('toast.sourceSaveFailed')))`. Retain success Toasts.
6. Replace the two-click delete function with:

```tsx
async function confirmDelete() {
  if (!deleteTarget || savingSource) return;
  setSavingSource(true);
  setFormError('');
  try {
    await onDeleteSource(deleteTarget);
    setDeleteTarget(null);
    setToast({ tone: 'success', message: t('sources.deleted') });
  } catch (error) {
    setFormError(errorMessage(error, t('toast.sourceSaveFailed')));
  } finally {
    setSavingSource(false);
  }
}

async function runSourceSync(source: SourceSubscription) {
  const key = String(source.id);
  if (syncingID !== null || syncingAll) return;
  setSyncingID(source.id);
  setSourceErrors((current) => ({ ...current, [key]: '' }));
  try {
    await onSync(source);
  } catch (error) {
    setSourceErrors((current) => ({ ...current, [key]: errorMessage(error, t('toast.sourceSyncFailed')) }));
  } finally {
    setSyncingID(null);
  }
}

async function runSyncAll() {
  if (syncingAll || syncingID !== null || sources.length === 0) return;
  setSyncingAll(true);
  setFormError('');
  setSyncAllResult(null);
  try {
    const result = await onSyncAll();
    if (result) setSyncAllResult(result);
  } catch (error) {
    setFormError(errorMessage(error, t('toast.sourceSyncFailed')));
  } finally {
    setSyncingAll(false);
  }
}
```

- [x] **Step 4: Render the focused no-source and managed-source states**

Use this top-level branching inside `SourcesView` while keeping the add/edit dialogs after it:

```tsx
{sources.length === 0 ? (
  <SourceOnboarding onAdd={() => {
    setFormError('');
    setIsAddSourceOpen(true);
  }} />
) : (
  <section className="page-grid client-sources-page">
    <div className="page-heading with-action">
      <div>
        <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
        <h1>{t('sources.title')}</h1>
        <p>{t('sources.managementSubtitle')}</p>
      </div>
      <div className="row-actions">
        <XButton type="button" variant="primary" label={t('sources.add')} icon={<Plus size={18} />} onClick={() => setIsAddSourceOpen(true)} />
        <XButton
          type="button"
          variant="secondary"
          label={syncingAll ? t('sources.syncingAll') : t('sources.syncAll')}
          icon={<RefreshCw size={18} className={syncingAll ? 'spin' : undefined} />
          isDisabled={syncingAll || syncingID !== null}
          onClick={() => void runSyncAll()}
        />
      </div>
    </div>
    {formError && <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>}
    {syncAllResult && (
      <div className={cx('client-sync-result', syncAllResult.failed > 0 && 'partial')} role="status">
        <strong>{syncAllResult.failed > 0 ? t('sources.syncResultPartial') : t('sources.syncResultSuccess')}</strong>
        <span>{t('sources.syncResultCounts', syncAllResult)}</span>
      </div>
    )}
    <div className="client-source-list">
      {sources.map((source) => {
        const scopedApps = sourceApps.filter((app) => belongsToSource(app, source));
        return (
          <SourceStatusRow
            key={source.id}
            source={source}
            health={healthFor(source)}
            appCount={source.lastAppCount ?? scopedApps.length}
            installableCount={source.lastInstallableCount ?? scopedApps.filter(hasInstallableVersion).length}
            isSyncing={syncingID === source.id}
            error={sourceErrors[String(source.id)]}
            onSync={() => runSourceSync(source)}
            onEdit={() => openEditSource(source)}
            onDelete={() => {
              setFormError('');
              setDeleteTarget(source);
            }}
          />
        );
      })}
    </div>
  </section>
)}
```

Add `aria-busy={savingSource}` to add/edit forms, render `formError` as an inline alert above `.dialog-actions`, and disable submit/cancel actions while saving.

- [x] **Step 5: Add an explicit destructive confirmation dialog**

Render after the edit dialog:

```tsx
{deleteTarget && (
  <ModalLayer onClose={() => !savingSource && setDeleteTarget(null)} purpose="form">
    <div className="modal-panel form-panel client-source-delete-dialog" role="alertdialog" aria-labelledby="client-source-delete-title">
      <SectionTitle icon={AlertCircle} title={t('sources.deleteTitle')} />
      <p id="client-source-delete-title">{t('sources.deleteBody', { name: deleteTarget.name })}</p>
      {formError && <p className="inline-alert" role="alert"><AlertCircle size={15} /><span>{formError}</span></p>}
      <div className="dialog-actions">
        <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} isDisabled={savingSource} onClick={() => setDeleteTarget(null)} />
        <XButton type="button" variant="destructive" label={savingSource ? t('common.deleting') : t('sources.deleteConfirm')} icon={<Trash2 size={18} />} isDisabled={savingSource} onClick={() => void confirmDelete()} />
      </div>
    </div>
  </ModalLayer>
)}
```

- [x] **Step 6: Add source-management styles**

Append to `client/src/styles/client.css`:

```css
.client-source-onboarding {
  min-height: min(620px, calc(100dvh - 180px));
  display: grid;
  place-items: center;
  align-content: center;
  justify-items: center;
  gap: 18px;
  padding: clamp(24px, 7vw, 72px);
  text-align: center;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.client-source-onboarding > div {
  max-width: 620px;
}

.client-source-onboarding h1,
.client-source-onboarding p {
  margin: 0;
}

.client-source-onboarding p {
  margin-top: 10px;
  color: var(--muted);
  line-height: 1.6;
}

.client-source-onboarding-icon,
.client-source-row-icon {
  display: grid;
  place-items: center;
  color: var(--green-strong);
  background: var(--mint);
  border: 1px solid color-mix(in srgb, var(--green) 30%, var(--line));
  border-radius: 8px;
}

.client-source-onboarding-icon {
  width: 54px;
  height: 54px;
}

.client-source-list {
  display: grid;
  gap: 10px;
}

.client-source-row {
  min-width: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px 18px;
  align-items: center;
  padding: 14px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.client-source-row.has-error {
  border-color: var(--status-error-line);
}

.client-source-row-main {
  min-width: 0;
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.client-source-row-icon {
  width: 38px;
  height: 38px;
}

.client-source-row-title,
.client-source-facts,
.client-source-row-actions,
.client-source-error {
  display: flex;
  align-items: center;
}

.client-source-row-title {
  min-width: 0;
  justify-content: space-between;
  gap: 10px;
}

.client-source-row-title strong,
.client-source-url {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-source-url,
.client-source-facts {
  color: var(--muted);
  font-size: 13px;
}

.client-source-facts {
  flex-wrap: wrap;
  gap: 5px 14px;
  margin-top: 6px;
}

.client-source-facts span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.client-source-row-actions {
  gap: 6px;
}

.client-source-error {
  grid-column: 1 / -1;
  gap: 8px;
  padding: 10px;
  color: var(--status-error-ink);
  background: var(--status-error-bg);
  border: 1px solid var(--status-error-line);
  border-radius: 8px;
}

.client-source-error span {
  min-width: 0;
  flex: 1;
  overflow-wrap: anywhere;
}

.client-sync-result {
  display: grid;
  gap: 4px;
  padding: 12px;
  color: var(--status-success-ink);
  background: var(--status-success-bg);
  border: 1px solid var(--status-success-line);
  border-radius: 8px;
}

.client-sync-result.partial {
  color: var(--status-warning-ink);
  background: var(--status-warning-bg);
  border-color: var(--status-warning-line);
}

@media (max-width: 760px) {
  .client-source-row {
    grid-template-columns: 1fr;
  }

  .client-source-row-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
```

- [x] **Step 7: Verify the source page no longer duplicates discovery**

Run:

```bash
npm exec --prefix client -- tsc -b client/tsconfig.json
! rg -n "SourceAppGrid|syncedApps|XPagination|selectedSyncedSource|selectedSyncedCategory" client/src/modules/client/SourcesView.tsx
```

Expected: TypeScript exits 0; the second command returns no matches.

- [~] **Step 8: Commit source workflow changes**

```bash
git add client/src/modules/client/SourceOnboarding.tsx client/src/modules/client/SourceStatusRow.tsx client/src/modules/client/SourcesView.tsx client/src/styles/client.css
git commit -m "feat: streamline standalone source management"
```

---

### Task 4: Replace simulated install percentages with an honest, durable activity panel

**Files:**
- Modify: `client/src/modules/client/InstallActivityPanel.tsx`
- Modify: `client/src/modules/client/InstallOptionsDialog.tsx`
- Modify: `client/src/modules/client/SourceAppDetailPage.tsx`
- Modify: `client/src/styles/client.css`
- Test: `client/src/modules/client/clientUxState.test.mjs`

**Interfaces:**
- Consumes: `buildInstallTimeline(activity)` from Task 1 and current `InstallActivity` fields.
- Produces: `InstallActivityPanel` keeps `activity` and `onDismiss`, adds optional `onRetry?: () => void` and `onOpenHistory?: () => void`; `SourceAppDetailPage` adds optional `isInstallPending?: boolean` without making INT changes mandatory for FE-2 compilation.

- [x] **Step 1: Extend the timeline test for legacy stage compatibility**

Append to `clientUxState.test.mjs`:

```js
test('legacy handoff and verify keys map to the real system-owned phase', () => {
  for (const stageKey of ['installActivity.stageHandoff', 'installActivity.stageVerify']) {
    assert.equal(
      buildInstallTimeline({ status: 'running', stageKey }).find((item) => item.state === 'current')?.key,
      'system',
    );
  }
});
```

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
```

Expected: 5 tests PASS.

- [x] **Step 2: Replace InstallActivityPanel with a real indeterminate workflow**

Update the component signature:

```tsx
export function InstallActivityPanel({
  activity,
  onDismiss,
  onRetry,
  onOpenHistory,
}: {
  activity: InstallActivity;
  onDismiss: () => void;
  onRetry?: () => void;
  onOpenHistory?: () => void;
})
```

Import `buildInstallTimeline`, `Button as XButton`, `History`, `RefreshCw`, and render the body as:

```tsx
const timeline = buildInstallTimeline(activity);

<aside className={cx('install-panel', activity.status)} aria-live="polite" aria-label={t('installActivity.title')}>
  <span className="install-panel-icon" aria-hidden="true"><StatusIcon size={20} /></span>
  <div className="install-panel-body">
    <div className="install-panel-head">
      <strong>{activity.title}</strong>
      <StatusBadge tone={statusTone} label={t(`installActivity.status.${activity.status}`)} />
    </div>
    <span>{t(activity.stageKey)}</span>
    <XProgressBar
      className="install-panel-progress"
      label={t('installActivity.progressLabel')}
      isLabelHidden
      value={activity.status === 'running' ? undefined : 100}
      isIndeterminate={activity.status === 'running'}
      variant={progressVariant}
    />
    <ol className="install-timeline" aria-label={t('installActivity.timeline')}>
      {timeline.map((item) => (
        <li key={item.key} className={item.state} aria-current={item.state === 'current' ? 'step' : undefined}>
          <span aria-hidden="true" />
          <strong>{t(`installActivity.steps.${item.key}`)}</strong>
        </li>
      ))}
    </ol>
    <div className="install-panel-meta">
      <small>{t('installActivity.source', { source: activity.source })}</small>
      <small>{t('installActivity.checksum', { checksum: activity.checksum })}</small>
      {activity.resultMode && <small>{t('installActivity.resultMode', { mode: t(`installActivity.modes.${activity.resultMode}`) })}</small>}
    </div>
    {activity.messageKey && <p className={activity.status === 'error' ? 'inline-alert' : undefined}>{t(activity.messageKey, activity.messageParams)}</p>}
    {activity.status !== 'running' && (
      <div className="install-panel-actions">
        {activity.status === 'error' && onRetry && <XButton type="button" variant="primary" size="sm" label={t('common.retry')} icon={<RefreshCw size={16} />} onClick={onRetry} />}
        {onOpenHistory && <XButton type="button" variant="secondary" size="sm" label={t('history.title')} icon={<History size={16} />} onClick={onOpenHistory} />}
      </div>
    )}
  </div>
  <XIconButton type="button" variant="ghost" label={t('installActivity.dismiss')} icon={<X size={17} />} isDisabled={activity.status === 'running'} onClick={onDismiss} />
</aside>
```

The panel must not auto-dismiss errors; existing App behavior already only auto-dismisses `success`.

- [x] **Step 3: Prevent duplicate install-options submission**

In `InstallOptionsDialog.tsx`, change `onSubmit` to return `void | Promise<void>`, add `submitting`, and use:

```tsx
async function submit(event: FormEvent) {
  event.preventDefault();
  if (submitting) return;
  const value = password.trim();
  if (requiresPassword && !value) {
    setError(t('installPassword.required'));
    return;
  }
  setSubmitting(true);
  setError('');
  try {
    await Promise.resolve(onSubmit({
      installPassword: requiresPassword ? value : undefined,
      mirrorId: mirrorOptions.length > 0 ? mirrorId : undefined,
    }));
  } catch (submitError) {
    setError(submitError instanceof Error ? submitError.message : t('toast.installFailed'));
    setSubmitting(false);
  }
}
```

Set `aria-busy={submitting}` on the form; disable cancel and submit while pending; label submit with `installActivity.status.running` while pending. The successful App path closes the dialog, so no success reset is needed.

- [x] **Step 4: Keep detail full-page and make its install command stateful**

In `SourceAppDetailPage.tsx`:

1. Add optional `isInstallPending?: boolean` prop defaulting to `false`.
2. On the main install button use `isDisabled={!installable || isInstallPending}` and show spinner + running label while pending.
3. Add `className="source-detail-primary-action"` to the install button and `className="source-detail-secondary-action"` to refresh/contact buttons.
4. Keep rollback buttons in version history secondary; do not convert the page back into a narrow drawer.

- [x] **Step 5: Add install timeline and mobile action styles**

Append to `client/src/styles/client.css`:

```css
.install-timeline {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin: 2px 0;
  padding: 0;
  list-style: none;
}

.install-timeline li {
  min-width: 0;
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 6px;
  align-items: center;
  color: var(--install-panel-muted);
  font-size: 12px;
}

.install-timeline li > span {
  width: 8px;
  height: 8px;
  border: 1px solid currentColor;
  border-radius: 50%;
}

.install-timeline li.complete,
.install-timeline li.current {
  color: var(--hero-ink);
}

.install-timeline li.complete > span {
  background: var(--install-panel-success);
  border-color: var(--install-panel-success);
}

.install-timeline li.current > span {
  background: var(--green-strong);
  border-color: var(--green-strong);
}

.install-timeline li.error,
.install-timeline li.error > span {
  color: var(--install-panel-error);
  border-color: var(--install-panel-error);
}

.install-panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 520px) {
  .install-timeline {
    grid-template-columns: 1fr 1fr;
  }

  .source-detail-actions {
    position: sticky;
    bottom: calc(8px + env(safe-area-inset-bottom));
    z-index: 3;
    padding: 8px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: color-mix(in srgb, var(--surface) 94%, transparent);
    backdrop-filter: blur(10px);
  }

  .source-detail-primary-action {
    width: 100%;
    justify-content: center;
  }
}
```

- [x] **Step 6: Verify installation code**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
! rg -n "progress: 12|progress: 42" client/src/modules/client
```

Expected: 5 tests PASS; TypeScript exits 0; no simulated numeric progress exists inside FE-2 files. App cleanup remains an INT responsibility listed below.

- [~] **Step 7: Commit installation UX**

```bash
git add client/src/modules/client/clientUxState.test.mjs client/src/modules/client/InstallActivityPanel.tsx client/src/modules/client/InstallOptionsDialog.tsx client/src/modules/client/SourceAppDetailPage.tsx client/src/styles/client.css
git commit -m "feat: clarify standalone install activity"
```

---

### Task 5: Prioritize updates and source boundaries in installed apps and history

**Files:**
- Modify: `client/src/modules/client/InstalledAppsView.tsx`
- Modify: `client/src/modules/client/ClientHistoryView.tsx`
- Modify: `client/src/styles/client.css`

**Interfaces:**
- Consumes: existing `InstalledApplication[]`, `SourceApp[]`, `findSourceForInstalled`, `compareVersions`, history callbacks.
- Produces: unchanged external prop signatures; `ClientHistoryView.onRefresh` widens to `() => void | Promise<void>`.

- [x] **Step 1: Group installed applications by actionability**

In `InstalledAppsView.tsx`, import `compareVersions` and replace the summary derivation with:

```tsx
const installedGroups = useMemo(() => {
  const updates: Array<{ item: InstalledApplication; source: SourceApp }> = [];
  const managed: Array<{ item: InstalledApplication; source: SourceApp }> = [];
  const local: InstalledApplication[] = [];
  for (const item of installedApps) {
    const source = findSourceForInstalled(item, sourceApps);
    if (!source) {
      local.push(item);
      continue;
    }
    if (item.version && source.latestVersion?.version && compareVersions(item.version, source.latestVersion.version) < 0) {
      updates.push({ item, source });
      continue;
    }
    managed.push({ item, source });
  }
  return { updates, managed, local };
}, [installedApps, sourceApps]);
```

Replace four technical metrics with three task groups: updates, source-managed, local/unknown.

- [x] **Step 2: Render updates first and explain local boundaries**

Add an internal rendering helper in `InstalledAppsView.tsx`:

```tsx
function InstalledGroup({
  title,
  tone,
  rows,
}: {
  title: string;
  tone: 'stale' | 'synced' | 'unsynced';
  rows: Array<{ item: InstalledApplication; source?: SourceApp }>;
}) {
  if (rows.length === 0) return null;
  return (
    <section className="installed-group">
      <div className="installed-group-head">
        <h3>{title}</h3>
        <StatusBadge tone={tone} label={String(rows.length)} />
      </div>
      <div className="installed-app-grid">
        {rows.map(({ item, source }) => (
          <article className="installed-app-card" key={item.appid || item.title}>
            {item.icon ? <img className="installed-app-icon" src={item.icon} alt="" /> : <AvatarIcon seed={item.appid || item.title || 'installed-app'} title={item.title || item.appid} size={42} />}
            <div>
              <strong>{item.title || item.appid || t('common.app')}</strong>
              <span title={item.appid || undefined}>{item.appid || t('profile.installedAppIdMissing')}</span>
              <small>{source ? t('profile.installedFromSource', { source: source.sourceName }) : t('profile.installedLocalExisting')}</small>
            </div>
            <div className="installed-app-meta">
              <span>{item.version || '-'}</span>
              <small>{source?.latestVersion?.version && compareVersions(item.version, source.latestVersion.version) < 0 ? t('app.updateAvailable') : item.instanceStatus || item.status || t('statusLabels.unknown')}</small>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
```

Render in this order:

```tsx
<div className="installed-groups">
  <InstalledGroup title={t('profile.installedUpdates')} tone="stale" rows={installedGroups.updates} />
  <InstalledGroup title={t('profile.installedManaged')} tone="synced" rows={installedGroups.managed} />
  <InstalledGroup title={t('profile.installedLocalGroup')} tone="unsynced" rows={installedGroups.local.map((item) => ({ item }))} />
</div>
```

The local group copy must explain that the client does not force-attribution to a source.

- [x] **Step 3: Make history refresh state and failures persistent**

In `ClientHistoryView.tsx`, add:

```tsx
const [refreshing, setRefreshing] = useState(false);
const [refreshError, setRefreshError] = useState('');

async function refreshHistory() {
  if (refreshing) return;
  setRefreshing(true);
  setRefreshError('');
  try {
    await Promise.resolve(onRefresh());
  } catch (error) {
    setRefreshError(error instanceof Error ? error.message : t('history.refreshFailed'));
  } finally {
    setRefreshing(false);
  }
}
```

Use a disabled spinning refresh button, render `refreshError` with `role="alert"`, and change summary labels for success/failed to `history.currentPageSuccess` / `history.currentPageFailed` so page-local counts are not presented as global totals.

- [x] **Step 4: Add installed grouping styles**

Append to `client/src/styles/client.css`:

```css
.installed-groups,
.installed-group {
  display: grid;
  gap: 14px;
}

.installed-group + .installed-group {
  padding-top: 14px;
  border-top: 1px solid var(--line);
}

.installed-group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.installed-group-head h3 {
  margin: 0;
  font-size: 16px;
}

.installed-group:first-child .installed-app-card {
  border-color: var(--status-warning-line);
  background: var(--status-warning-bg);
}
```

- [x] **Step 5: Verify and commit installed/history changes**

Run:

```bash
npm exec --prefix client -- tsc -b client/tsconfig.json
```

Expected: TypeScript exits 0 without generating dist.

```bash
git add client/src/modules/client/InstalledAppsView.tsx client/src/modules/client/ClientHistoryView.tsx client/src/styles/client.css
git commit -m "feat: prioritize standalone installed app states"
```

---

### Task 6: Add durable dirty, saving, saved, and failed states to settings

**Files:**
- Modify: `client/src/modules/client/ClientSettingsView.tsx`
- Modify: `client/src/styles/client.css`
- Test: `client/src/modules/client/clientUxState.test.mjs`

**Interfaces:**
- Consumes: `normalizeEditableClientSettings` and `sameEditableClientSettings` from Task 1.
- Produces: unchanged `ClientSettingsView` external props; internal `SaveState = 'clean' | 'dirty' | 'saving' | 'saved' | 'error'`.

- [x] **Step 1: Add the settings state transition test**

Append to `clientUxState.test.mjs`:

```js
test('editable settings comparison detects user-owned changes', () => {
  const base = {
    clientTitle: '',
    commentDisplayName: '',
    defaultPageSize: 24,
    autoSyncEnabled: false,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
    installSuccessDismissSeconds: 3,
  };
  assert.equal(sameEditableClientSettings(base, { ...base, syncOnStartup: true }), false);
});
```

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
```

Expected: 6 tests PASS.

- [x] **Step 2: Derive settings status from baseline and draft**

In `ClientSettingsView.tsx`:

1. Import `AlertCircle`, `Check`, `normalizeEditableClientSettings`, and `sameEditableClientSettings`.
2. Replace `saving` with:

```tsx
type SaveResult = 'idle' | 'saving' | 'saved' | 'error';
type SaveState = 'clean' | 'dirty' | Exclude<SaveResult, 'idle'>;

const [baseline, setBaseline] = useState<ClientSettings>(settings);
const [draft, setDraft] = useState<ClientSettings>(settings);
const [saveResult, setSaveResult] = useState<SaveResult>('idle');
const [saveError, setSaveError] = useState('');
const isDirty = !sameEditableClientSettings(draft, baseline);
const effectiveSaveState: SaveState = saveResult === 'saving' || saveResult === 'error' || saveResult === 'saved'
  ? saveResult
  : isDirty
    ? 'dirty'
    : 'clean';

function updateDraft(next: ClientSettings) {
  setDraft(next);
  setSaveError('');
  setSaveResult('idle');
}
```

3. When incoming `settings` changes, normalize the editable fields, set both baseline and draft, clear error, and preserve `saved` when the update is the save response:

```tsx
useEffect(() => {
  const next = { ...settings, ...normalizeEditableClientSettings(settings) };
  setBaseline(next);
  setDraft(next);
  setSaveError('');
  setSaveResult((current) => current === 'saving' ? 'saved' : current);
}, [
  settings.autoSyncEnabled,
  settings.autoSyncIntervalMinutes,
  settings.clientTitle,
  settings.commentDisplayName,
  settings.defaultPageSize,
  settings.installSuccessDismissSeconds,
  settings.lastAutoSyncAt,
  settings.lastAutoSyncError,
  settings.lastAutoSyncStatus,
  settings.syncOnStartup,
]);
```

4. Replace every `setDraft({ ...draft, field: value })` field handler with `updateDraft({ ...draft, field: value })`. This clears a previous save error only after the user changes a value; an untouched failure remains visible.

- [x] **Step 3: Save the normalized payload and retain failures inline**

Replace `saveSettings` with:

```tsx
async function saveSettings(event?: Pick<FormEvent, 'preventDefault'>) {
  event?.preventDefault();
  if (!isDirty || saveResult === 'saving') return;
  const payload = normalizeEditableClientSettings(draft);
  setSaveResult('saving');
  setSaveError('');
  try {
    await onSave(payload);
    setBaseline({ ...draft, ...payload });
    setDraft((current) => ({ ...current, ...payload }));
    setSaveResult('saved');
    setToast({ tone: 'success', message: t('clientSettings.saved') });
  } catch (error) {
    setSaveError(errorMessage(error, t('clientSettings.saveFailed')));
    setSaveResult('error');
  }
}
```

- [x] **Step 4: Replace duplicate save buttons with one stable save bar**

Remove the page-heading save button and existing bottom `.settings-form-actions`. Add this after the active tab panel inside the form:

```tsx
<div className={cx('client-settings-save-bar', effectiveSaveState)} role="status" aria-live="polite">
  <div>
    {effectiveSaveState === 'error' ? <AlertCircle size={18} /> : effectiveSaveState === 'saved' ? <Check size={18} /> : <Save size={18} />}
    <span>{t(`clientSettings.saveStates.${effectiveSaveState}`)}</span>
  </div>
  {saveError && <p role="alert">{saveError}</p>}
  <XButton
    type="submit"
    variant="primary"
    label={effectiveSaveState === 'saving' ? t('clientSettings.saving') : t('clientSettings.saveSettings')}
    icon={effectiveSaveState === 'saving' ? <RefreshCw size={18} className="spin" /> : <Save size={18} />}
    isDisabled={!isDirty || saveResult === 'saving'}
  />
</div>
```

Add `cx` import. Keep tab switching synchronous; no timers and no animation state.

- [x] **Step 5: Remove repeated tab entrance motion locally and style the save bar**

Append to `client/src/styles/client.css`:

```css
.client-settings-panel.settings-tab-panel {
  transition: none;
}

.client-settings-panel.settings-tab-panel {
  @starting-style {
    opacity: 1;
    transform: none;
  }
}

.client-settings-save-bar {
  position: sticky;
  bottom: calc(10px + env(safe-area-inset-bottom));
  z-index: 4;
  width: min(760px, 100%);
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px 14px;
  align-items: center;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface) 94%, transparent);
  box-shadow: var(--shadow-soft);
  backdrop-filter: blur(10px);
}

.client-settings-save-bar > div {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.client-settings-save-bar p {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--red);
  overflow-wrap: anywhere;
}

.client-settings-save-bar.saved {
  border-color: var(--status-success-line);
}

.client-settings-save-bar.error {
  border-color: var(--status-error-line);
}

@media (max-width: 640px) {
  .client-settings-save-bar {
    grid-template-columns: 1fr;
    bottom: calc(76px + env(safe-area-inset-bottom));
  }

  .client-settings-save-bar > button {
    width: 100%;
    justify-content: center;
  }
}
```

- [x] **Step 6: Verify settings status and no tab transform**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
! rg -n "settings-hero.*XButton|settings-form-actions" client/src/modules/client/ClientSettingsView.tsx
```

Expected: 6 tests PASS; TypeScript exits 0; no duplicate settings save area remains.

- [~] **Step 7: Commit settings states**

```bash
git add client/src/modules/client/clientUxState.test.mjs client/src/modules/client/ClientSettingsView.tsx client/src/styles/client.css
git commit -m "feat: expose durable client settings state"
```

---

### Task 7: Complete client-local interaction, responsive, and reduced-motion polish

**Files:**
- Modify: `client/src/styles/client.css`

**Interfaces:**
- Consumes: class names introduced in Tasks 2–6.
- Produces: client-local responsive behavior at 1024, 768, 375, 320 and reduced-motion overrides; no shared stylesheet changes.

- [x] **Step 1: Remove decorative motion from non-interactive client metrics**

In `client.css`, replace the current `.settings-signal-card` transition and `@starting-style` with:

```css
.settings-signal-card {
  min-width: 0;
  min-height: 118px;
  max-width: 360px;
  max-height: var(--metric-card-max-height);
  overflow: hidden;
  display: grid;
  gap: 8px;
  align-content: start;
}
```

Delete `.settings-signal-card:hover { transform: translateY(-2px); }`. These cards are not clickable and must remain visually stationary.

- [x] **Step 2: Add client-local press, focus, touch, and reduced-motion rules**

Append:

```css
.client-source-row button,
.client-source-onboarding button,
.client-settings-save-bar button,
.install-panel button,
.client-source-app-card button {
  transition: transform 140ms var(--ease-out), background-color 140ms ease, border-color 140ms ease, color 140ms ease;
}

.client-source-row button:active,
.client-source-onboarding button:active,
.client-settings-save-bar button:active,
.install-panel button:active,
.client-source-app-card button:active {
  transform: scale(0.98);
}

.client-source-row :focus-visible,
.client-source-onboarding :focus-visible,
.client-settings-save-bar :focus-visible,
.install-panel :focus-visible,
.client-source-app-card :focus-visible {
  outline: 2px solid var(--green-strong);
  outline-offset: 2px;
}

@media (prefers-reduced-motion: reduce) {
  .detail-page-shell,
  .client-source-row button,
  .client-source-onboarding button,
  .client-settings-save-bar button,
  .install-panel button,
  .client-source-app-card,
  .client-source-app-card button {
    transform: none !important;
    transition-property: opacity, color, background-color, border-color !important;
    transition-duration: 120ms !important;
  }

  .spin {
    animation-duration: 1.4s;
  }
}
```

- [x] **Step 3: Verify touch actions are always visible**

Run:

```bash
rg -n "opacity: 0|visibility: hidden|:hover" client/src/styles/client.css
```

Expected: no client source/install/delete action is hidden by default. Any remaining hover rule is inside `@media (hover: hover) and (pointer: fine)` and only changes decoration.

- [x] **Step 4: Run all FE-2 automated checks**

Run:

```bash
node --test client/src/modules/client/clientUxState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
git diff --check
git status --short
```

Expected:

- 6 tests PASS.
- TypeScript exits 0.
- `git diff --check` exits 0.
- Status shows only FE-2 files plus the user's pre-existing WIP; no `client/dist`, `clientembed/dist`, `web/dist`, language file, App, shared file, storefront file, or admin file is newly modified by FE-2.

- [~] **Step 5: Commit client-local polish**

## Completion Evidence

- Implemented the seven client UX workstreams, including persistent source-operation results, shared duplicate-submit locking, indeterminate system-owned install progress, installed/history grouping, and settings save states.
- Verified 2026-07-10: `node --test client/src/modules/client/clientUxState.test.mjs` (7 passed); `npm exec --prefix client -- tsc -b client/tsconfig.json` (passed); `git diff --check` (passed).
- Review follow-up: `SourcesView` serializes add/edit/delete/sync/sync-all through one ref-backed action lock; `SourceStatusRow` disables conflicting row actions while it is busy.
- `[~]` denotes a deliberately omitted commit: the user requires this dirty worktree remain uncommitted, unpushed, and unpublished.

```bash
git add client/src/styles/client.css
git commit -m "style: polish standalone client interactions"
```

---

## INT Integration Contract — Do Not Execute in FE-2

INT owns these changes after FE-1, FE-2 and FE-3 are merged. The FE-2 implementer must report this contract verbatim in handoff and must not edit these files.

### INT-1: Default route and quieter navigation

**Files:**
- Modify: `client/src/App.tsx`
- Modify: `client/src/modules/shell/navigation.ts`

In `navigation.ts`, order standalone tabs as discovery, installed, sources, history, settings:

```ts
const clientBaseTabs: NavItem[] = [
  { key: 'search', labelKey: 'nav.install', icon: Download },
  { key: 'profile', labelKey: 'nav.installed', icon: Archive },
  { key: 'sources', labelKey: 'nav.sources', icon: Cloud },
  { key: 'history', labelKey: 'nav.history', icon: History },
  { key: 'settings', labelKey: 'nav.settings', icon: Settings },
];
```

In `App.tsx`, initialize standalone mode to `search`:

```tsx
const [tab, setTab] = useState<TabKey>(() => (
  verificationTokenFromURL() || collaborationInviteTokenFromURL()
    ? 'profile'
    : HAS_API
      ? 'home'
      : 'search'
));
```

Add this ref near `defaultSourceCheckedRef`:

```tsx
const clientLandingResolvedRef = useRef(false);
```

Inside `refreshClientData`, replace the existing `await Promise.all([loadClientSources(), loadClientSettings()]);` line with:

```tsx
const [nextSources] = await Promise.all([loadClientSources(), loadClientSettings()]);
if (!clientLandingResolvedRef.current) {
  clientLandingResolvedRef.current = true;
  setTab(nextSources.length > 0 ? 'search' : 'sources');
}
```

The line `void loadInstallHistory();` already follows the replaced statement; leave that exact line immediately after the new guarded route decision. This changes one load statement and adds one route decision without altering the remaining refresh lifecycle.

Change `syncAllSources` to return the structured result and remain on the source page so the persistent FE-2 result panel stays visible. Replace its final success/redirect block with:

```tsx
const result = { success, failed };
if (failed > 0) {
  setToast({ tone: success > 0 ? 'neutral' : 'error', message: t('toast.sourcesSyncPartial', result) });
} else {
  setToast({ tone: 'success', message: t('toast.allSourcesSynced', { count: sources.length }) });
}
return result;
```

Replace the no-source branch with:

```tsx
if (sources.length === 0) {
  navigateTo('sources');
  setToast({ tone: 'neutral', message: t('toast.addSourceFirst') });
  return { success: 0, failed: 0 };
}
```

Replace the caught-error branch with:

```tsx
} catch (error) {
  throw new Error(errorMessage(error, t('toast.sourceSyncFailed')), { cause: error });
}
```

This lets `SourcesView` keep the failure inline instead of receiving only a transient Toast.

### INT-2: Installation identity, duplicate prevention, stages, retry, and history action

**Files:**
- Modify: `client/src/shared/types.ts`
- Modify: `client/src/App.tsx`

Extend `InstallActivity` without changing existing fields:

```ts
export type InstallActivity = {
  appKey: string;
  appId: number;
  version: string;
  title: string;
  source: string;
  checksum: string;
  status: 'running' | 'success' | 'error';
  progress: number;
  stageKey: string;
  resultMode?: string;
  messageKey?: string;
  messageParams?: Record<string, string | number>;
};
```

In `App.tsx`, retain the last source install request for retry:

```tsx
const [lastInstallRequest, setLastInstallRequest] = useState<{ app: StoreApp | SourceApp; options: InstallOptions } | null>(null);
```

At the start of `installApp`, prevent duplicate execution for the active app and capture the request:

```tsx
const appKey = isSourceApp ? `${app.sourceId ?? app.sourceName}:${app.id}` : `store:${app.id}`;
if (installActivity?.status === 'running') return;
setLastInstallRequest({ app, options });
```

Replace simulated 12/42 progress and the 180ms sleep with truthful stages:

```tsx
setInstallActivity({
  appKey,
  appId: app.id,
  version: version.version,
  title: `${appName} ${version.version}`,
  source,
  checksum,
  status: 'running',
  progress: 0,
  stageKey: 'installActivity.stageQueued',
});

await Promise.resolve();
setInstallActivity((current) => current?.appKey === appKey ? { ...current, stageKey: 'installActivity.stagePrepare' } : current);

setInstallActivity((current) => current?.appKey === appKey ? { ...current, stageKey: 'installActivity.stageSystem' } : current);
const result = isSourceApp
  ? await clientApi<ClientInstallResult>('/install', {
      method: 'POST',
      body: JSON.stringify({
        appId: app.id,
        version: version.version,
        installPassword: options.installPassword,
        mirrorId: options.mirrorId || '',
      }),
    }).then((value) => ({
      mode: value.mode || 'lazycat-go-sdk',
      messageKey: 'installResult.sdkInstalled',
      messageParams: value.taskId ? { taskId: value.taskId } : undefined,
    }))
  : await Promise.resolve().then(() => {
      const downloadUrl = `${API_BASE}/api/v1/apps/${app.id}/versions/${(version as Version).id}/download`;
      const protectedDownloadUrl = withInstallPassword(downloadUrl, options.installPassword);
      window.open(protectedDownloadUrl, '_blank', 'noopener,noreferrer');
      return { mode: 'download', messageKey: 'installResult.downloadOpened', messageParams: undefined };
    });
```

Wrap the request/result section in `try/catch`. In the catch block, persist the failure before rethrowing to the existing `runAction` Toast boundary:

```tsx
} catch (error) {
  const message = errorMessage(error, t('toast.installFailed'));
  setInstallActivity((current) => current?.appKey === appKey
    ? {
        ...current,
        status: 'error',
        progress: 100,
        stageKey: 'installActivity.stageFailed',
        messageKey: 'installActivity.failureMessage',
        messageParams: { message },
      }
    : current);
  throw error;
}
```

This prevents an SDK/network error from leaving the activity panel permanently in `running`.

Keep terminal `progress: 100`, and pass FE-2 props:

```tsx
<InstallActivityPanel
  activity={installActivity}
  onDismiss={() => setInstallActivity(null)}
  onRetry={installActivity.status === 'error' && lastInstallRequest
    ? () => void installApp(lastInstallRequest.app, lastInstallRequest.options)
    : undefined}
  onOpenHistory={!HAS_API ? () => navigateTo('history') : undefined}
/>
```

Pass `activeInstallKey={installActivity?.status === 'running' ? installActivity.appKey : undefined}` through `SearchView`/`ClientCatalog` into `SourceAppGrid`, and pass `isInstallPending` to `SourceAppDetailPage`. Because `SearchView` is outside FE-2 ownership, INT performs the prop threading after all frontend branches are merged.

### INT-3: Language keys

**Files:**
- Modify: `client/src/locales/zh.ts`
- Modify: `client/src/locales/en.ts`

Merge these exact keys into existing sections; preserve the user's current language-file WIP.

| Key | 中文 | English |
| --- | --- | --- |
| `search.updatesAvailable` | 可更新 | Updates |
| `sources.onboardingTitle` | 添加第一个软件源 | Add your first software source |
| `sources.onboardingBody` | 添加并同步软件源后，即可在发现页浏览、安装和更新应用。 | Add and sync a source to discover, install, and update apps. |
| `sources.managementSubtitle` | 查看同步状态、修复错误或调整订阅；应用浏览集中在发现页。 | Check sync health, recover errors, or adjust subscriptions. Browse apps from Discover. |
| `sources.groupCount` | {{count}} 个群组 | {{count}} groups |
| `sources.syncingAll` | 正在同步全部 | Syncing all |
| `sources.syncResultSuccess` | 所有软件源已同步 | All sources synced |
| `sources.syncResultPartial` | 软件源同步部分完成 | Source sync partially completed |
| `sources.syncResultCounts` | 成功 {{success}} 个，失败 {{failed}} 个 | {{success}} succeeded, {{failed}} failed |
| `sources.deleteTitle` | 删除软件源 | Delete software source |
| `sources.deleteBody` | 将删除「{{name}}」及其本地缓存；已安装应用不会被卸载。 | This removes “{{name}}” and its local cache. Installed apps will not be uninstalled. |
| `sources.deleteConfirm` | 删除这个软件源 | Delete this source |
| `installActivity.stageQueued` | 已加入安装队列 | Added to the install queue |
| `installActivity.stageSystem` | 系统正在下载、校验并安装 | The system is downloading, verifying, and installing |
| `installActivity.timeline` | 安装阶段 | Install stages |
| `installActivity.steps.queued` | 排队 | Queued |
| `installActivity.steps.prepare` | 准备 | Prepare |
| `installActivity.steps.system` | 系统安装 | System install |
| `installActivity.steps.result` | 结果 | Result |
| `installActivity.failureMessage` | 安装失败：{{message}} | Install failed: {{message}} |
| `profile.installedUpdates` | 有可用更新 | Updates available |
| `profile.installedManaged` | 来自软件源 | From software sources |
| `profile.installedLocalGroup` | 本地或未知来源 | Local or unknown source |
| `history.currentPageSuccess` | 本页成功 | Successful on this page |
| `history.currentPageFailed` | 本页失败 | Failed on this page |
| `history.refreshFailed` | 安装历史刷新失败 | Failed to refresh install history |
| `clientSettings.saveStates.clean` | 所有更改已保存 | All changes saved |
| `clientSettings.saveStates.dirty` | 有未保存的更改 | Unsaved changes |
| `clientSettings.saveStates.saving` | 正在保存设置 | Saving settings |
| `clientSettings.saveStates.saved` | 设置已保存 | Settings saved |
| `clientSettings.saveStates.error` | 保存失败，请重试 | Save failed. Try again. |

Before adding `common.sync`, `common.retry`, `common.deleting`, check whether the keys already exist; if absent, add:

| Key | 中文 | English |
| --- | --- | --- |
| `common.sync` | 同步 | Sync |
| `common.retry` | 重试 | Retry |
| `common.deleting` | 删除中 | Deleting |

### INT-4: Shared global-style reconciliation

**File:**
- Modify: `client/src/styles.css`

After all frontend tasks merge:

1. Leave shared `.app-card`, `.source-app-card`, `.source-row`, review-row and version-row rules intact because storefront/admin still consume them; FE-2's more-specific client selectors provide its local behavior.
2. Replace the complete global `.settings-tab-panel` block, including its `transition` and nested `@starting-style`, with:

```css
.settings-tab-panel {
  display: grid;
  gap: 14px;
  min-width: 0;
}
```

3. Leave modal-specific motion in `ModalLayer` styles unchanged.
4. Leave the existing desktop-only source-card action de-emphasis inside `@media (hover: hover) and (pointer: fine)`; the new `.client-source-row-actions` selector is not part of that rule and remains visible on all input types.

### INT-5: Final build, generated assets, and cross-client verification

FE-2 must not run these commands. INT runs dependency installation and source checks after every frontend branch is merged:

```bash
npm --prefix client ci
node --test client/src/modules/client/clientUxState.test.mjs
npm exec --prefix client -- tsc -b client/tsconfig.json
```

Expected: tests PASS and TypeScript exits 0.

Build the standalone bundle, then replace its hashed assets while preserving the tracked runtime `app-config.js`:

```bash
VITE_API_BASE_URL="" npm --prefix client run build
rm -rf clientembed/dist/assets clientembed/dist/index.html
cp -R client/dist/. clientembed/dist/
```

Expected: Vite exits 0; `clientembed/dist/app-config.js` remains present; `clientembed/dist/index.html` references only files that exist under `clientembed/dist/assets`.

Build the server bundle with its API base, then refresh the ignored server embed directory:

```bash
VITE_API_BASE_URL="." npm --prefix client run build
rm -rf web/dist
mkdir -p web/dist
cp -R client/dist/. web/dist/
```

Expected: Vite exits 0; `web/dist/index.html` references only files that exist under `web/dist/assets`. `client/dist/**` and `web/dist/**` are ignored build outputs and are verified but not staged.

Run Go and frontend integration checks:

```bash
go test ./...
go vet ./...
go test -race ./...
golangci-lint run --timeout=5m
go mod tidy -diff
git diff --check
```

Expected: every command exits 0. Do not use `lazycat/client/build.sh` or `lazycat/server/build.sh` merely to refresh assets during integration because they also download external files, run `npm ci`, and build Go binaries; use them only for the actual packaging workflow.

Manual smoke matrix:

| Width / mode | Required checks |
| --- | --- |
| 1440 light | Existing source lands on Discover; cards have one command; source management is compact; install panel does not fake percentages. |
| 1024 dark | Filters and results remain above cards; settings save bar remains visible; full-page detail retains readable width. |
| 768 non-default Astryx theme | Source actions are visible without hover; source error retry and delete confirmation are reachable. |
| 375 light | No-source onboarding has one primary CTA; install panel sits above mobile nav; source actions wrap without clipping. |
| 320 dark | Summary scrolls or stacks without horizontal page overflow; sticky detail/settings actions respect safe-area inset. |
| reduced motion | Tab switches have no translate animation; press transforms and detail movement are removed; colors/focus remain visible. |
| keyboard | Logical tab order, visible focus, Enter/Space actions, Escape closes add/edit/delete/install dialogs, detail back regains focus. |

State scenarios:

1. No sources: only “添加软件源” is primary; adding shows pending and inline errors.
2. Healthy source: discover is default, source row shows last sync/app/group facts.
3. Auth failure: error persists after Toast disappears; edit and retry are visible.
4. Sync-all partial failure: page-level result persists and failed rows retain recovery actions.
5. Install: repeated clicks are blocked; running progress is indeterminate; success may auto-dismiss; failure remains and can retry/open history.
6. Installed apps: updates render first; local/unknown apps are not falsely attributed.
7. Settings: editing shows unsaved, saving blocks duplicate submit, success shows saved, failure remains inline, tab changes are immediate.

Final INT commit:

```bash
git add client/src/App.tsx client/src/modules/shell/navigation.ts client/src/shared/types.ts client/src/locales/zh.ts client/src/locales/en.ts client/src/styles.css clientembed/dist
git commit -m "feat: integrate frontend experience hardening"
```

Do not use `git add -A`; the working tree contains unrelated user WIP and generated Ent/backend changes that must not be swept into the frontend integration commit.
