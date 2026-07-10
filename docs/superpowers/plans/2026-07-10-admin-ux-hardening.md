# Server Admin UX Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Reorganize the server administration interface around clear operational tasks, persistent save/run feedback, touch-reachable actions, and explicit destructive confirmations without changing backend APIs or broadening the existing `AdminPanel` refactor.

**Architecture:** Keep `AdminPanel` as the data-owning container and make only surgical extractions for reusable admin-only feedback components and pure state comparison helpers. Promote storage, backup, and migration to first-level admin tasks; keep site settings, users/groups, taxonomy, and collections on their existing API contracts; put all new styling in `client/src/styles/admin.css`. Shared application assembly, language packs, global styles, and generated bundles remain an INT-stage responsibility.

**Tech Stack:** React 19.2, TypeScript 5.9, Vite 7.3, Astryx Design 0.1.4, lucide-react, i18next, CSS custom properties, Node 26 built-in test runner.

## Global Constraints

- Target the approved design in `docs/superpowers/specs/2026-07-10-frontend-backend-hardening-design.md`.
- FE-3 may modify only `client/src/modules/admin/**` and `client/src/styles/admin.css`.
- Do not modify `client/src/App.tsx`, `client/src/locales/en.ts`, `client/src/locales/zh.ts`, `client/src/styles.css`, other shared/global styles, or generated `client/dist` / `clientembed/dist` assets; those belong to INT.
- Preserve the current uncommitted `AdminPanel.tsx` time-zone work: `timeZoneOptions` and the `site_timezone` field must survive every edit.
- Preserve every unrelated uncommitted file and do not use reset, checkout, or generated-bundle rebuilds.
- Keep React, Vite, Astryx, Ent, routes, payloads, and public API semantics unchanged; add no frontend dependencies.
- Each admin task page has a clear title, a concise status summary, and one dominant page-level operation.
- Saving, testing, running, importing, and deleting must expose pending state and prevent duplicate submission.
- Site and backup forms show unsaved, saving, saved, and failed state persistently; mobile save controls account for `env(safe-area-inset-bottom)`.
- Storage tests, backup runs, and migration operations leave a persistent result panel; Toast remains a secondary confirmation only.
- Destructive dialogs name the object and explain the consequence; two-click Toast confirmation is removed.
- High-frequency admin tab switching has no positional animation. Press feedback stays 100–160ms, hover behavior is gated by `hover: hover` and `pointer: fine`, and reduced motion removes transforms.
- Validate at 1440, 1024, 768, 375, and 320 CSS pixels in light, dark, and one non-default Astryx theme.

---

## Current Review Baseline

| Before | After | Why |
| --- | --- | --- |
| Storage, backup, and migration are buried inside the Site tab | Promote them to first-level admin tasks | These are operational workflows, not ordinary site fields |
| Every settings sub-tab re-enters with `translateY(6px)` | Switch admin task and settings tabs instantly, with no positional animation | Administrators repeat these switches frequently; movement makes the interface feel slower |
| Site and backup save actions do not show dirty state | Add persistent save bars with unsaved/saving/saved/failed states | Users must know whether their edits are durable |
| Storage tests and migration failures primarily report through Toast | Keep the latest operation result inline with time, target, and retry | Operational outcomes must remain inspectable after the Toast disappears |
| User, category, tag, collection, invite, and storage deletion uses “click again” | Open a named confirmation dialog with consequence copy and a pending destructive button | Accidental double-clicks and transient Toasts are poor safeguards |
| `AdminPanel.tsx` owns every visual primitive | Extract only admin task header, save bar, result panel, confirmation dialog, and pure state helpers | Reuse feedback patterns without an unrelated container rewrite |

## File Map

- Create `client/src/modules/admin/adminState.ts`: pure structural comparison and admin async-state types.
- Create `client/src/modules/admin/adminState.test.mjs`: Node 26 tests for dirty-state comparison without adding a test dependency.
- Create `client/src/modules/admin/AdminTaskHeader.tsx`: consistent task title/status header.
- Create `client/src/modules/admin/AdminSaveBar.tsx`: persistent form save state and submit action.
- Create `client/src/modules/admin/AdminOperationResult.tsx`: persistent latest-result panel with optional retry.
- Create `client/src/modules/admin/AdminDeleteDialog.tsx`: reusable object-specific destructive confirmation.
- Modify `client/src/modules/admin/AdminPanel.tsx`: first-level task navigation, site save state, storage results, and explicit delete orchestration while preserving current time-zone WIP.
- Modify `client/src/modules/admin/StorageSettingsPanel.tsx`: dirty/pending/result props and stable touch actions.
- Modify `client/src/modules/admin/AdminBackupPanel.tsx`: dirty tracking, save bar, and persistent save/run results.
- Modify `client/src/modules/admin/migration/AdminMigrationPanel.tsx`: persistent export/preview/import activity result.
- Modify `client/src/modules/admin/AdminUsersWorkspace.tsx`: forward pending/delete request state only; do not move data fetching into this component.
- Modify `client/src/modules/admin/AdminUsersPanel.tsx`: disable duplicate row actions and request deletion through the parent dialog.
- Modify `client/src/modules/admin/AdminGroupsPanel.tsx`: add pending state to the existing named delete/rotate dialogs.
- Modify `client/src/modules/admin/AdminAnnouncementsPanel.tsx`: add pending deletion to its existing named confirmation.
- Modify `client/src/styles/admin.css`: admin-only layout, status, save bar, result, dialog, touch, mobile, and reduced-motion rules.

## Task 1: Add Admin-Only State and Feedback Primitives

**Files:**
- Create: `client/src/modules/admin/adminState.ts`
- Create: `client/src/modules/admin/adminState.test.mjs`
- Create: `client/src/modules/admin/AdminTaskHeader.tsx`
- Create: `client/src/modules/admin/AdminSaveBar.tsx`
- Create: `client/src/modules/admin/AdminOperationResult.tsx`
- Create: `client/src/modules/admin/AdminDeleteDialog.tsx`

**Interfaces:**
- Produces: `type AdminSaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'`.
- Produces: `type AdminStorageAction = 'create' | 'save' | 'test' | 'default' | 'delete' | null`.
- Produces: `type AdminOperationResult = { variant; title; message; occurredAt; target? }`.
- Produces: `areAdminDraftsEqual(left: unknown, right: unknown): boolean`.
- Produces: `<AdminTaskHeader icon title body statusLabel statusVariant action />`.
- Produces: `<AdminSaveBar status isDirty saveLabel onSave isDisabled />`.
- Produces: `<AdminOperationResultPanel result retryLabel? onRetry? isRetrying? />`.
- Produces: `<AdminDeleteDialog title subject consequence confirmLabel isDeleting onCancel onConfirm />`.

- [x] **Step 1: Write failing state-comparison tests**

Create `client/src/modules/admin/adminState.test.mjs`:

```js
import assert from 'node:assert/strict';
import test from 'node:test';
import { areAdminDraftsEqual } from './adminState.ts';

test('areAdminDraftsEqual ignores object key insertion order', () => {
  assert.equal(
    areAdminDraftsEqual(
      { site_title: 'Store', nested: { enabled: true, count: 2 } },
      { nested: { count: 2, enabled: true }, site_title: 'Store' },
    ),
    true,
  );
});

test('areAdminDraftsEqual preserves array order', () => {
  assert.equal(
    areAdminDraftsEqual(
      { storageKeys: ['primary', 'archive'] },
      { storageKeys: ['archive', 'primary'] },
    ),
    false,
  );
});

test('areAdminDraftsEqual detects nested edits', () => {
  assert.equal(
    areAdminDraftsEqual(
      { targets: [{ storageKey: 'primary', directory: 'backups/appstore' }] },
      { targets: [{ storageKey: 'primary', directory: 'backups/nightly' }] },
    ),
    false,
  );
});
```

- [x] **Step 2: Run the test and confirm the missing module failure**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
```

Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `adminState.ts`.

- [x] **Step 3: Implement the pure state contract**

Create `client/src/modules/admin/adminState.ts`:

```ts
export type AdminSaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error';

export type AdminStorageAction = 'create' | 'save' | 'test' | 'default' | 'delete' | null;

export type AdminOperationResult = {
  variant: 'neutral' | 'success' | 'warning' | 'error' | 'info';
  title: string;
  message: string;
  occurredAt: string;
  target?: string;
};

function canonicalize(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalize);
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, item]) => [key, canonicalize(item)]),
    );
  }
  return value;
}

export function areAdminDraftsEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(canonicalize(left)) === JSON.stringify(canonicalize(right));
}
```

- [x] **Step 4: Run the state tests**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
```

Expected: PASS, 3 tests and 0 failures.

- [x] **Step 5: Add the task header component**

Create `client/src/modules/admin/AdminTaskHeader.tsx`:

```tsx
import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';

export type AdminTaskHeaderProps = {
  icon: LucideIcon;
  title: string;
  body: string;
  statusLabel?: string;
  statusVariant?: 'neutral' | 'success' | 'warning' | 'error' | 'info';
  action?: ReactNode;
};

export function AdminTaskHeader({
  icon: Icon,
  title,
  body,
  statusLabel,
  statusVariant = 'neutral',
  action,
}: AdminTaskHeaderProps) {
  return (
    <header className="admin-task-header">
      <div className="admin-task-heading">
        <span className="admin-task-icon" aria-hidden="true"><Icon size={20} /></span>
        <div>
          <h2>{title}</h2>
          <p>{body}</p>
        </div>
      </div>
      <div className="admin-task-header-actions">
        {statusLabel && <XBadge label={statusLabel} variant={statusVariant} />}
        {action}
      </div>
    </header>
  );
}
```

- [x] **Step 6: Add the save bar**

Create `client/src/modules/admin/AdminSaveBar.tsx`:

```tsx
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Check, Save, TriangleAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AdminSaveStatus } from './adminState';

export function AdminSaveBar({
  status,
  isDirty,
  saveLabel,
  onSave,
  isDisabled = false,
}: {
  status: AdminSaveStatus;
  isDirty: boolean;
  saveLabel: string;
  onSave: () => void;
  isDisabled?: boolean;
}) {
  const { t } = useTranslation();
  const label = status === 'saving'
    ? t('admin.saveState.saving')
    : status === 'error'
      ? t('admin.saveState.failed')
      : isDirty
        ? t('admin.saveState.unsaved')
        : status === 'saved'
          ? t('admin.saveState.saved')
          : t('admin.saveState.noChanges');
  const variant = status === 'error' ? 'error' : isDirty ? 'warning' : status === 'saved' ? 'success' : 'neutral';
  const icon = status === 'error' ? <TriangleAlert size={14} /> : status === 'saved' && !isDirty ? <Check size={14} /> : undefined;

  return (
    <div className="admin-save-bar" role="status" aria-live="polite">
      <XBadge label={label} variant={variant} icon={icon} />
      <XButton
        type="button"
        variant="primary"
        label={saveLabel}
        icon={<Save size={17} />}
        isDisabled={isDisabled || !isDirty || status === 'saving'}
        isLoading={status === 'saving'}
        onClick={onSave}
      />
    </div>
  );
}
```

- [x] **Step 7: Add the persistent operation-result panel**

Create `client/src/modules/admin/AdminOperationResult.tsx`:

```tsx
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckCircle2, RefreshCw, TriangleAlert, XCircle } from 'lucide-react';
import { formatDate } from '../../shared/utils';
import type { AdminOperationResult } from './adminState';

function resultIcon(variant: AdminOperationResult['variant']) {
  if (variant === 'success') return <CheckCircle2 size={16} />;
  if (variant === 'error') return <XCircle size={16} />;
  return <TriangleAlert size={16} />;
}

export function AdminOperationResultPanel({
  result,
  retryLabel,
  onRetry,
  isRetrying = false,
}: {
  result: AdminOperationResult | null;
  retryLabel?: string;
  onRetry?: () => void;
  isRetrying?: boolean;
}) {
  if (!result) return null;
  return (
    <section className="admin-operation-result" role={result.variant === 'error' ? 'alert' : 'status'} aria-live="polite">
      <div className="admin-operation-result-head">
        <div>
          <strong>{result.title}</strong>
          <time dateTime={result.occurredAt}>{formatDate(result.occurredAt)}</time>
        </div>
        <XBadge label={result.message} variant={result.variant} icon={resultIcon(result.variant)} />
      </div>
      {result.target && <code>{result.target}</code>}
      {onRetry && retryLabel && (
        <XButton type="button" variant="secondary" size="sm" label={retryLabel} icon={<RefreshCw size={16} />} isLoading={isRetrying} isDisabled={isRetrying} onClick={onRetry} />
      )}
    </section>
  );
}
```

- [x] **Step 8: Add the destructive confirmation dialog**

Create `client/src/modules/admin/AdminDeleteDialog.tsx`:

```tsx
import { Button as XButton } from '@astryxdesign/core/Button';
import { Trash2, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { SectionTitle } from '../../shared/components/Feedback';

export function AdminDeleteDialog({
  title,
  subject,
  consequence,
  confirmLabel,
  isDeleting,
  onCancel,
  onConfirm,
}: {
  title: string;
  subject: string;
  consequence: string;
  confirmLabel: string;
  isDeleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const { t } = useTranslation();
  return (
    <ModalLayer onClose={() => { if (!isDeleting) onCancel(); }} purpose="required">
      <div className="modal-panel form-panel admin-delete-dialog" role="alertdialog" aria-modal="true" aria-labelledby="admin-delete-dialog-title">
        <SectionTitle icon={Trash2} title={title} />
        <div className="admin-delete-copy">
          <strong id="admin-delete-dialog-title">{subject}</strong>
          <p>{consequence}</p>
        </div>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={isDeleting} onClick={onCancel} />
          <XButton type="button" variant="destructive" label={confirmLabel} icon={<Trash2 size={17} />} isDisabled={isDeleting} isLoading={isDeleting} onClick={onConfirm} />
        </div>
      </div>
    </ModalLayer>
  );
}
```

- [x] **Step 9: Type-check the new primitives without generating dist**

Run:

```bash
cd client && npx tsc -b
```

Expected: PASS with no TypeScript diagnostics and no `client/dist` write.

- [x] **Step 10: Commit the primitives**

```bash
git add client/src/modules/admin/adminState.ts client/src/modules/admin/adminState.test.mjs client/src/modules/admin/AdminTaskHeader.tsx client/src/modules/admin/AdminSaveBar.tsx client/src/modules/admin/AdminOperationResult.tsx client/src/modules/admin/AdminDeleteDialog.tsx
git commit -m "feat(admin): add persistent feedback primitives"
```

## Task 2: Promote Operational Workflows to First-Level Admin Tasks

**Files:**
- Modify: `client/src/modules/admin/AdminPanel.tsx:1-220,1046-1490`
- Modify: `client/src/styles/admin.css`

**Interfaces:**
- Consumes: `AdminTaskHeader` from Task 1.
- Produces: `type AdminTask = 'reviews' | 'site' | 'users' | 'taxonomy' | 'collections' | 'storage' | 'backup' | 'migration'`.
- Keeps: all current `AdminPanel` props and backend routes unchanged.
- Keeps: current `timeZoneOptions` and `site_timezone` field unchanged.

- [x] **Step 1: Capture the existing time-zone WIP before editing**

Run:

```bash
git diff -- client/src/modules/admin/AdminPanel.tsx
```

Expected: the diff includes `timeZoneOptions` and `{ key: 'site_timezone', ... }`. Do not remove or reformat those blocks as part of this task.

- [x] **Step 2: Add the first-level task type and task-header import**

In `client/src/modules/admin/AdminPanel.tsx`, add:

```tsx
import { AdminTaskHeader, type AdminTaskHeaderProps } from './AdminTaskHeader';

type AdminTask = 'reviews' | 'site' | 'users' | 'taxonomy' | 'collections' | 'storage' | 'backup' | 'migration';
type SiteSettingsTask = 'identity' | 'announcement' | 'ads' | 'registration' | 'policy' | 'mail';
```

Change the two state declarations to:

```tsx
const [adminTab, setAdminTab] = useState<AdminTask>('reviews');
const [siteSettingsTab, setSiteSettingsTab] = useState<SiteSettingsTask>('identity');
```

- [x] **Step 3: Promote storage, backup, and migration in `adminTabs`**

Replace `adminTabs` and `siteSettingsTabs` with:

```tsx
const adminTabs = [
  { key: 'reviews', label: t('admin.tabs.reviews'), icon: ShieldCheck },
  ...(isSiteAdmin ? [{ key: 'site' as const, label: t('admin.tabs.site'), icon: Settings }] : []),
  ...(isSiteAdmin ? [{ key: 'users' as const, label: t('admin.tabs.users'), icon: Users }] : []),
  { key: 'taxonomy', label: t('admin.tabs.taxonomy'), icon: Tag },
  { key: 'collections', label: t('admin.tabs.collections'), icon: Layers3 },
  ...(isSiteAdmin ? [{ key: 'storage' as const, label: t('admin.siteSettingTabs.storage'), icon: Server }] : []),
  ...(isSiteAdmin ? [{ key: 'backup' as const, label: t('admin.siteSettingTabs.backup'), icon: CloudUpload }] : []),
  ...(isSiteAdmin ? [{ key: 'migration' as const, label: t('admin.siteSettingTabs.migration'), icon: DatabaseBackup }] : []),
] as const;

const siteSettingsTabs = [
  { key: 'identity', label: t('admin.siteSettingTabs.identity'), icon: Archive },
  { key: 'announcement', label: t('admin.siteSettingTabs.announcement'), icon: MessageSquare },
  { key: 'ads', label: t('admin.siteSettingTabs.ads'), icon: Megaphone },
  { key: 'registration', label: t('admin.siteSettingTabs.registration'), icon: KeyRound },
  { key: 'policy', label: t('admin.siteSettingTabs.policy'), icon: ShieldCheck },
  { key: 'mail', label: t('admin.siteSettingTabs.mail'), icon: MessageSquare },
] as const;
```

- [x] **Step 4: Add a task metadata function without moving business state**

Immediately before `return`, add:

```tsx
function activeTaskHeader(): AdminTaskHeaderProps {
  switch (adminTab) {
    case 'reviews':
      return { icon: ShieldCheck, title: t('admin.reviewQueue'), body: reviewOpsBody, statusLabel: reviewSummary.total === 0 ? t('admin.opsReady') : t('admin.opsNeedsAction'), statusVariant: reviewSummary.total === 0 ? 'success' as const : 'warning' as const };
    case 'site':
      return { icon: Settings, title: t('admin.siteSettings'), body: t('admin.siteIdentityBody'), statusLabel: sourceProtected ? t('admin.opsReady') : t('admin.opsNeedsAction'), statusVariant: sourceProtected ? 'success' as const : 'warning' as const };
    case 'users':
      return { icon: Users, title: t('admin.userManagement'), body: t('admin.body'), statusLabel: String(userPagination.totalItems || users.length), statusVariant: 'neutral' as const };
    case 'taxonomy':
      return { icon: Tag, title: t('admin.categoriesAndTags'), body: t('admin.taxonomyHelp'), statusLabel: `${adminCategories.length + adminTags.length}`, statusVariant: adminCategories.length > 0 ? 'success' as const : 'warning' as const };
    case 'collections':
      return { icon: Layers3, title: t('admin.collectionList'), body: t('admin.collectionAppsHelp'), statusLabel: `${adminCollections.length}`, statusVariant: adminCollections.length > 0 ? 'success' as const : 'neutral' as const };
    case 'storage':
      return { icon: Server, title: t('admin.storageSettings'), body: t('admin.storageSettingsBody'), statusLabel: `${storageRecords.length}`, statusVariant: storageRecords.length > 0 ? 'success' as const : 'warning' as const };
    case 'backup':
      return { icon: CloudUpload, title: t('admin.backup.title'), body: t('admin.backup.body'), statusLabel: t('admin.siteSettingTabs.backup'), statusVariant: 'neutral' as const };
    case 'migration':
      return { icon: DatabaseBackup, title: t('admin.migration.title'), body: t('admin.migration.body'), statusLabel: t('admin.siteSettingTabs.migration'), statusVariant: 'warning' as const };
  }
}

const taskHeader = activeTaskHeader();
```

- [x] **Step 5: Render the task header and add an admin root scope**

Change the root opening tag from:

```tsx
<section className="page-grid">
```

to:

```tsx
<section className="page-grid admin-shell">
```

Change the main-tab wrapper from:

```tsx
<div className="horizontal-control-scroll">
```

to:

```tsx
<div className="horizontal-control-scroll admin-primary-tabs">
```

Immediately after that wrapper's closing `</div>`, insert:

```tsx
<AdminTaskHeader {...taskHeader} />
```

Do not add page-level action buttons to the header yet. Existing create/run/save controls remain the domain actions until their task is updated below.

- [x] **Step 6: Simplify the Site content branch**

Inside the Site panel, delete the two leading branches that render `AdminMigrationPanel` and `AdminBackupPanel`. Replace this condition:

```tsx
siteSettingsTab !== 'storage' && siteSettingsTab !== 'announcement' && siteSettingsTab !== 'ads'
```

with:

```tsx
siteSettingsTab !== 'announcement' && siteSettingsTab !== 'ads'
```

Keep the complete existing identity, registration, policy, and mail form body, including the time-zone selector. In the final ternary tail, replace:

```tsx
) : siteSettingsTab === 'ads' ? (
```

with:

```tsx
) : (
```

Then delete the final fallback branch that renders `StorageSettingsPanel` inside the Site panel. The remaining final branch is the complete existing `AdminAdsPanel` block.

- [x] **Step 7: Render the promoted tasks as standalone workspaces**

Add these branches after the Site branch and before Users:

```tsx
{isSiteAdmin && adminTab === 'storage' && (
  <section className="workspace-pane admin-task-workspace">
    <StorageSettingsPanel
      storages={storageRecords}
      defaultKey={defaultStorageKey}
      selectedKey={selectedStorageKey}
      draft={storageDraft}
      createDraft={storageCreateDraft}
      isCreateOpen={isStorageCreateOpen}
      onSelect={setSelectedStorageKey}
      onDraftChange={setStorageDraft}
      onCreateDraftChange={setStorageCreateDraft}
      onOpenCreate={() => setIsStorageCreateOpen(true)}
      onCloseCreate={() => setIsStorageCreateOpen(false)}
      onCreate={createStorage}
      onSave={saveStorageSettings}
      onTestDraft={testStorageSettings}
      onTestSaved={testSavedStorage}
      onSetDefault={setDefaultStorage}
      onDelete={deleteStorage}
    />
  </section>
)}
{isSiteAdmin && adminTab === 'backup' && (
  <section className="panel admin-task-workspace">
    <AdminBackupPanel storages={storageRecords} setToast={setToast} />
  </section>
)}
{isSiteAdmin && adminTab === 'migration' && (
  <section className="panel admin-task-workspace">
    <AdminMigrationPanel api={api} setToast={setToast} />
  </section>
)}
```

- [x] **Step 8: Add only the structural task CSS**

Append to `client/src/styles/admin.css`:

```css
.admin-shell,
.admin-task-workspace {
  min-width: 0;
}

.admin-task-workspace {
  display: grid;
  gap: 14px;
  max-width: 1100px;
}

.admin-task-header {
  align-items: flex-start;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  display: flex;
  gap: 16px;
  justify-content: space-between;
  min-width: 0;
  padding: 14px;
}

.admin-task-heading {
  align-items: flex-start;
  display: grid;
  gap: 10px;
  grid-template-columns: 36px minmax(0, 1fr);
  min-width: 0;
}

.admin-task-icon {
  align-items: center;
  background: var(--field);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--green-strong);
  display: inline-flex;
  height: 36px;
  justify-content: center;
  width: 36px;
}

.admin-task-heading h2,
.admin-task-heading p {
  margin: 0;
}

.admin-task-heading h2 {
  font-size: 18px;
  line-height: 1.25;
}

.admin-task-heading p {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
  margin-top: 4px;
}

.admin-task-header-actions {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.admin-shell .settings-tab-panel {
  transform: none;
  transition: none;
}

@starting-style {
  .admin-shell .settings-tab-panel {
    opacity: 1;
    transform: none;
  }
}
```

- [x] **Step 9: Type-check and verify the time-zone WIP remains**

Run:

```bash
cd client && npx tsc -b
cd .. && rg -n "timeZoneOptions|site_timezone|adminTab === 'storage'|adminTab === 'backup'|adminTab === 'migration'" client/src/modules/admin/AdminPanel.tsx
```

Expected: type-check PASS; all five search terms are present; storage/backup/migration no longer appear in `siteSettingsTabs`.

- [x] **Step 10: Commit the task navigation**

```bash
git add client/src/modules/admin/AdminPanel.tsx client/src/styles/admin.css
git commit -m "feat(admin): promote operational task navigation"
```

## Task 3: Add Persistent Site Settings Save State

**Files:**
- Modify: `client/src/modules/admin/AdminPanel.tsx:130-175,381-439,643-645,1219-1400`
- Modify: `client/src/styles/admin.css`

**Interfaces:**
- Consumes: `AdminSaveBar`, `AdminSaveStatus`, and `areAdminDraftsEqual` from Task 1.
- Produces: site settings dirty detection against the last server-confirmed snapshot.
- Keeps: `PATCH /api/v1/admin/settings` payload exactly `Record<string, string>`.

- [x] **Step 1: Import save-state primitives and add state**

Add imports:

```tsx
import { AdminSaveBar } from './AdminSaveBar';
import { areAdminDraftsEqual, type AdminSaveStatus } from './adminState';
```

Beside `settings`, add:

```tsx
const [savedSettings, setSavedSettings] = useState<Record<string, string>>({});
const [settingsSaveStatus, setSettingsSaveStatus] = useState<AdminSaveStatus>('idle');
const settingsDirty = !areAdminDraftsEqual(settings, savedSettings);
```

- [x] **Step 2: Make reload preserve a dirty site draft and establish the server baseline only when requested**

Add above `reload`:

```tsx
type ReloadOptions = { refreshSettings?: boolean };
```

Change the signature and Site-admin portion of `reload` to:

```tsx
async function reload({ refreshSettings = !settingsDirty }: ReloadOptions = {}) {
  await runAction(setToast, t('admin.loadFailed'), async () => {
    const [categoryData, tagData, collectionData, appCountData] = await Promise.all([
      api<{ categories: Category[] }>('/api/v1/admin/categories'),
      api<{ tags: TagRecord[] }>('/api/v1/admin/tags'),
      api<{ collections: Collection[] }>('/api/v1/admin/collections'),
      api<PaginatedResponse<StoreApp, 'apps'>>('/api/v1/apps?managed=1&status=APPROVED&pageSize=1'),
    ]);
    setAdminCategories(categoryData.categories);
    setAdminTags(tagData.tags);
    setAdminCollections(collectionData.collections);
    setApprovedAppCount(appCountData.pagination?.totalItems || appCountData.apps?.length || 0);
    setCategoryDrafts({});
    setTagDrafts({});
    setCollectionDrafts({});
    if (isSiteAdmin) {
      const settingsPromise = refreshSettings
        ? api<{ settings: Record<string, string> }>('/api/v1/admin/settings')
        : Promise.resolve(null);
      const [settingData, storageData, announcementData, adData] = await Promise.all([
        settingsPromise,
        api<{ storages: StorageSettings[]; defaultKey: string }>('/api/v1/admin/storage'),
        fetchAllPaginated<SiteAnnouncement, 'announcements'>(api, '/api/v1/admin/announcements', 'announcements'),
        fetchAllPaginated<SiteAd, 'ads'>(api, '/api/v1/admin/ads', 'ads'),
      ]);
      if (settingData) {
        const nextSettings = settingData.settings || {};
        setSettings(nextSettings);
        setSavedSettings(nextSettings);
        setSettingsSaveStatus('idle');
      }
      setLoadedStorageRecords(storageData.storages || [], storageData.defaultKey);
      setAnnouncements(announcementData || []);
      setAds(adData || []);
      await Promise.all([fetchUsersPage(), fetchInvitesPage()]);
    }
  });
}
```

This keeps the initial `reload()` behavior because the initial draft is clean, but later user/group/taxonomy/announcement refreshes do not overwrite unsaved Site edits. Keep the existing `timeZoneOptions` and `site_timezone` field untouched.

- [x] **Step 3: Mark edits dirty immediately**

Replace `updateSetting` with:

```tsx
function updateSetting(key: string, value: string) {
  setSettings((current) => ({ ...current, [key]: value }));
  setSettingsSaveStatus('dirty');
}
```

- [x] **Step 4: Replace generic `runAction` save handling with explicit status handling**

Replace `saveSettings` with:

```tsx
async function saveSettings(event?: FormEvent) {
  event?.preventDefault();
  if (!settingsDirty || settingsSaveStatus === 'saving') return;
  setSettingsSaveStatus('saving');
  try {
    await api('/api/v1/admin/settings', { method: 'PATCH', body: JSON.stringify(settings) });
    setSavedSettings(settings);
    setSettingsSaveStatus('saved');
    setToast({ tone: 'success', message: t('admin.settingsSaved') });
    await onSiteProfileSaved();
  } catch (error) {
    setSettingsSaveStatus('error');
    setToast({ tone: 'error', message: errorMessage(error, t('admin.settingsSaveFailed')) });
  }
}
```

Do not call the broad `reload()` after saving; it needlessly resets taxonomy/collection drafts and can erase unrelated in-progress admin work.

- [x] **Step 5: Replace the old save button with `AdminSaveBar`**

Replace:

```tsx
<div className="settings-form-actions">
  <XButton type="submit" variant="primary" label={t('admin.saveSettings')} icon={<Settings size={18} />} />
</div>
```

with:

```tsx
<AdminSaveBar
  status={settingsSaveStatus}
  isDirty={settingsDirty}
  saveLabel={t('admin.saveSettings')}
  onSave={() => void saveSettings()}
/>
```

Keep `onSubmit={saveSettings}` on the form so keyboard submission still works; keyboard submission must not introduce an animation.

- [x] **Step 6: Add the desktop/mobile save bar layout**

Append to `client/src/styles/admin.css`:

```css
.admin-save-bar {
  align-items: center;
  background: color-mix(in srgb, var(--surface) 96%, transparent);
  border: 1px solid var(--line);
  border-radius: 8px;
  bottom: 10px;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  padding: 10px;
  position: sticky;
  z-index: 4;
}

@media (max-width: 720px) {
  .admin-save-bar {
    bottom: 0;
    margin: 0 -14px -14px;
    padding: 10px 14px calc(10px + env(safe-area-inset-bottom));
    border-bottom: 0;
    border-inline: 0;
    border-radius: 0 0 8px 8px;
  }

  .admin-save-bar > button {
    min-width: 132px;
  }
}

@media (max-width: 360px) {
  .admin-save-bar {
    align-items: stretch;
    display: grid;
  }

  .admin-save-bar > button {
    width: 100%;
  }
}
```

- [x] **Step 7: Run state tests and type-check**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
cd client && npx tsc -b
```

Expected: 3 tests PASS; TypeScript PASS. Changing any identity/policy/mail/registration field enables the save button; a successful save disables it and shows the saved status.

- [x] **Step 8: Commit site save feedback**

```bash
git add client/src/modules/admin/AdminPanel.tsx client/src/styles/admin.css
git commit -m "feat(admin): show persistent site save state"
```

## Task 4: Harden Storage Editing, Tests, and Deletion Feedback

**Files:**
- Modify: `client/src/modules/admin/AdminPanel.tsx:145-153,321-327,454-524,storage task render`
- Modify: `client/src/modules/admin/StorageSettingsPanel.tsx:62-211`
- Modify: `client/src/styles/admin.css`

**Interfaces:**
- Consumes: `areAdminDraftsEqual`, `AdminOperationResult`, `AdminOperationResultPanel`, `AdminSaveBar`, and `AdminDeleteDialog`.
- Extends `StorageSettingsPanelProps` with `isDirty`, `saveStatus`, `activeAction`, and `result`.
- Keeps all storage endpoints and `storageSettingsPayload` unchanged.

- [x] **Step 1: Add storage action, save, result, and delete state in `AdminPanel`**

Add imports:

```tsx
import { AdminDeleteDialog } from './AdminDeleteDialog';
import { AdminOperationResultPanel } from './AdminOperationResult';
import type { AdminOperationResult, AdminStorageAction } from './adminState';
```

Add state beside the existing storage state:

```tsx
const [storageSaveStatus, setStorageSaveStatus] = useState<AdminSaveStatus>('idle');
const [storageAction, setStorageAction] = useState<AdminStorageAction>(null);
const [storageResult, setStorageResult] = useState<AdminOperationResult | null>(null);
const [storageToDelete, setStorageToDelete] = useState<StorageSettings | null>(null);
const selectedStorageRecord = storageRecords.find((storage) => storage.key === selectedStorageKey) || storageRecords[0] || defaultStorageSettings;
const storageDirty = !areAdminDraftsEqual(storageSettingsPayload(storageDraft), storageSettingsPayload(selectedStorageRecord));
```

- [x] **Step 2: Reset save status when changing storage selection and mark edits dirty**

In the `selectedStorageKey`/`storageRecords` effect, add:

```tsx
setStorageSaveStatus('idle');
```

Pass this exact draft handler to `StorageSettingsPanel`:

```tsx
onDraftChange={(nextDraft) => {
  setStorageDraft(nextDraft);
  setStorageSaveStatus('dirty');
}}
```

- [x] **Step 3: Make save and test operations explicit and persistent**

Replace `saveStorageSettings` with:

```tsx
async function saveStorageSettings() {
  if (!storageDirty || storageAction) return;
  setStorageAction('save');
  setStorageSaveStatus('saving');
  try {
    const data = await api<{ storage: StorageSettings }>(`/api/v1/admin/storage/${encodeURIComponent(storageDraft.key)}`, {
      method: 'PATCH',
      body: JSON.stringify(storageSettingsPayload(storageDraft)),
    });
    const saved = normalizeStorageRecord(data.storage);
    setStorageRecords((current) => current.map((item) => (item.key === saved.key ? saved : item)));
    setStorageDraft(saved);
    setStorageSaveStatus('saved');
    setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: saved.name || saved.key });
    setToast({ tone: 'success', message: t('admin.storageSaved') });
    await onStorageOptionsChanged();
  } catch (error) {
    const message = errorMessage(error, t('admin.storageSaveFailed'));
    setStorageSaveStatus('error');
    setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: storageDraft.name || storageDraft.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}
```

Replace `testStorageSettings` with:

```tsx
async function testStorageSettings() {
  if (storageAction) return;
  setStorageAction('test');
  try {
    await api('/api/v1/admin/storage/test', { method: 'POST', body: JSON.stringify(storageSettingsPayload(storageDraft)) });
    setStorageResult({ variant: 'success', title: t('admin.testStorage'), message: t('admin.storageTested'), occurredAt: new Date().toISOString(), target: storageDraft.name || storageDraft.key });
    setToast({ tone: 'success', message: t('admin.storageTested') });
  } catch (error) {
    const message = errorMessage(error, t('admin.storageTestFailed'));
    setStorageResult({ variant: 'error', title: t('admin.testStorage'), message, occurredAt: new Date().toISOString(), target: storageDraft.name || storageDraft.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}
```

Replace `createStorage`, `testSavedStorage`, and `setDefaultStorage` with these exact implementations:

```tsx
async function createStorage() {
  if (storageAction) return;
  setStorageAction('create');
  try {
    const data = await api<{ storage: StorageSettings }>('/api/v1/admin/storage', {
      method: 'POST',
      body: JSON.stringify(storageSettingsPayload(storageCreateDraft)),
    });
    const next = normalizeStorageRecord(data.storage);
    setStorageRecords((current) => [next, ...current.filter((item) => item.key !== next.key)]);
    setSelectedStorageKey(next.key);
    setStorageCreateDraft({ ...defaultStorageSettings, key: '', name: '' });
    setIsStorageCreateOpen(false);
    setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: next.name || next.key });
    setToast({ tone: 'success', message: t('admin.storageSaved') });
    await onStorageOptionsChanged();
  } catch (error) {
    const message = errorMessage(error, t('admin.storageSaveFailed'));
    setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: storageCreateDraft.name || storageCreateDraft.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}

async function testSavedStorage(storage: StorageSettings) {
  if (storageAction) return;
  setStorageAction('test');
  try {
    await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/test`, { method: 'POST' });
    setStorageResult({ variant: 'success', title: t('admin.testStorage'), message: t('admin.storageTested'), occurredAt: new Date().toISOString(), target: storage.name || storage.key });
    setToast({ tone: 'success', message: t('admin.storageTested') });
  } catch (error) {
    const message = errorMessage(error, t('admin.storageTestFailed'));
    setStorageResult({ variant: 'error', title: t('admin.testStorage'), message, occurredAt: new Date().toISOString(), target: storage.name || storage.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}

async function setDefaultStorage(storage: StorageSettings) {
  if (storageAction) return;
  setStorageAction('default');
  try {
    await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/default`, { method: 'POST' });
    setDefaultStorageKey(storage.key);
    setStorageRecords((current) => current.map((item) => ({ ...item, isDefault: item.key === storage.key })));
    setStorageResult({ variant: 'success', title: t('admin.defaultStoragePicker'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: storage.name || storage.key });
    setToast({ tone: 'success', message: t('admin.storageSaved') });
    await onStorageOptionsChanged();
  } catch (error) {
    const message = errorMessage(error, t('admin.storageSaveFailed'));
    setStorageResult({ variant: 'error', title: t('admin.defaultStoragePicker'), message, occurredAt: new Date().toISOString(), target: storage.name || storage.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}
```

- [x] **Step 4: Replace two-click storage deletion with a named dialog**

Rename the current deleting function to `confirmDeleteStorage` and remove its `confirmDelete`/Toast preflight:

```tsx
async function confirmDeleteStorage() {
  if (!storageToDelete || storageAction) return;
  setStorageAction('delete');
  try {
    await api(`/api/v1/admin/storage/${encodeURIComponent(storageToDelete.key)}`, { method: 'DELETE' });
    setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageDeleted'), occurredAt: new Date().toISOString(), target: storageToDelete.name || storageToDelete.key });
    setToast({ tone: 'neutral', message: t('admin.storageDeleted') });
    setStorageToDelete(null);
    setStorageRecords((current) => current.filter((item) => item.key !== storageToDelete.key));
    await onStorageOptionsChanged();
  } catch (error) {
    const message = errorMessage(error, t('admin.storageDeleteFailed'));
    setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: storageToDelete.name || storageToDelete.key });
    setToast({ tone: 'error', message });
  } finally {
    setStorageAction(null);
  }
}
```

Pass `onDelete={setStorageToDelete}` to `StorageSettingsPanel`, then render after the storage workspace:

```tsx
{storageToDelete && (
  <AdminDeleteDialog
    title={t('admin.deleteStorageNamed', { name: storageToDelete.name || storageToDelete.key })}
    subject={storageToDelete.name || storageToDelete.key}
    consequence={t('admin.confirmDeleteStorage', { name: storageToDelete.name || storageToDelete.key })}
    confirmLabel={t('common.delete')}
    isDeleting={storageAction === 'delete'}
    onCancel={() => setStorageToDelete(null)}
    onConfirm={() => void confirmDeleteStorage()}
  />
)}
```

- [x] **Step 5: Extend the storage panel interface**

In `StorageSettingsPanel.tsx`, import the new components/types and extend props:

```tsx
import { AdminOperationResultPanel } from './AdminOperationResult';
import { AdminSaveBar } from './AdminSaveBar';
import type { AdminOperationResult, AdminSaveStatus, AdminStorageAction } from './adminState';

type StorageSettingsPanelProps = {
  isDirty: boolean;
  saveStatus: AdminSaveStatus;
  activeAction: AdminStorageAction;
  result: AdminOperationResult | null;
};
```

Insert those four fields at the end of the existing `StorageSettingsPanelProps` object; retain every existing storage prop unchanged.

Destructure the four new props. Update buttons exactly as follows:

```tsx
<XButton type="button" variant="primary" size="sm" label={t('admin.createStorage')} icon={<Plus size={17} />} isDisabled={activeAction !== null} onClick={onOpenCreate} />

<XIconButton type="button" variant="ghost" size="sm" label={t('admin.testStorageNamed', { name: storage.name || storage.key })} icon={<Check size={16} />} isDisabled={activeAction !== null} onClick={() => void onTestSaved(storage)} />

<XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteStorageNamed', { name: storage.name || storage.key })} icon={<Trash2 size={16} />} isDisabled={activeAction !== null} onClick={() => void onDelete(storage)} />
```

Replace the editor save/test action block with:

```tsx
<div className="storage-editor-actions">
  <XButton className="storage-action-button" type="button" variant="secondary" size="sm" label={t('admin.testStorage')} icon={<Check size={17} />} isDisabled={activeAction !== null} isLoading={activeAction === 'test'} onClick={() => void onTestDraft()} />
</div>
```

After `StorageFields`, render:

```tsx
<AdminSaveBar status={saveStatus} isDirty={isDirty} saveLabel={t('admin.saveStorage')} isDisabled={activeAction !== null} onSave={() => void onSave()} />
<AdminOperationResultPanel result={result} retryLabel={t('admin.testStorage')} isRetrying={activeAction === 'test'} onRetry={() => void onTestDraft()} />
```

Pass the four new props from `AdminPanel`:

```tsx
isDirty={storageDirty}
saveStatus={storageSaveStatus}
activeAction={storageAction}
result={storageResult}
```

- [x] **Step 6: Add persistent result styling**

Append to `client/src/styles/admin.css`:

```css
.admin-operation-result {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 12px;
}

.admin-operation-result-head {
  align-items: flex-start;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  min-width: 0;
}

.admin-operation-result-head > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.admin-operation-result time {
  color: var(--muted);
  font-size: 12px;
}

.admin-operation-result code {
  background: var(--field);
  border: 1px solid var(--line);
  border-radius: 6px;
  overflow-wrap: anywhere;
  padding: 6px 8px;
}
```

- [x] **Step 7: Test and type-check**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
cd client && npx tsc -b
```

Expected: PASS. Changing a storage field shows unsaved; only one storage request can run at a time; test success/failure remains visible; delete opens a named modal and disables confirmation while pending.

- [x] **Step 8: Commit storage feedback**

```bash
git add client/src/modules/admin/AdminPanel.tsx client/src/modules/admin/StorageSettingsPanel.tsx client/src/styles/admin.css
git commit -m "feat(admin): harden storage operation feedback"
```

## Task 5: Persist Backup and Migration Outcomes

**Files:**
- Modify: `client/src/modules/admin/AdminBackupPanel.tsx`
- Modify: `client/src/modules/admin/migration/AdminMigrationPanel.tsx`
- Modify: `client/src/modules/admin/migration/MigrationExportCard.tsx`
- Modify: `client/src/styles/admin.css`

**Interfaces:**
- Consumes: `AdminSaveBar`, `AdminOperationResultPanel`, `areAdminDraftsEqual`, `AdminSaveStatus`, and `AdminOperationResult`.
- Keeps: backup and migration API request/response shapes unchanged.
- Produces: latest backup settings/run and migration export/preview/import results that persist until the next operation.

- [x] **Step 1: Add backup dirty and result state**

In `AdminBackupPanel.tsx`, import the Task 1 primitives and add:

```tsx
import { AdminOperationResultPanel } from './AdminOperationResult';
import { AdminSaveBar } from './AdminSaveBar';
import { areAdminDraftsEqual, type AdminOperationResult, type AdminSaveStatus } from './adminState';

const [savedDraft, setSavedDraft] = useState<BackupDraft>(draftFromSettings(defaultBackupSettings));
const [saveStatus, setSaveStatus] = useState<AdminSaveStatus>('idle');
const [operationResult, setOperationResult] = useState<AdminOperationResult | null>(null);
const isDirty = !areAdminDraftsEqual(draft, savedDraft);
```

In `loadSettings`, after computing `next`, use:

```tsx
const nextDraft = draftFromSettings(next);
setSettings(next);
setDraft(nextDraft);
setSavedDraft(nextDraft);
setSaveStatus('idle');
```

- [x] **Step 2: Make backup saving persistent**

Replace `saveSettings` with:

```tsx
async function saveSettings() {
  if (!isDirty || isSaving || isRunning) return;
  setIsSaving(true);
  setSaveStatus('saving');
  try {
    const payload = { ...draft, storageKeys: draft.targets.map((target) => target.storageKey) };
    const data = await api<{ settings: BackupSettings }>('/api/v1/admin/backups/settings', { method: 'PATCH', body: JSON.stringify(payload) });
    const next = data.settings || defaultBackupSettings;
    const nextDraft = draftFromSettings(next);
    setSettings(next);
    setDraft(nextDraft);
    setSavedDraft(nextDraft);
    setSaveStatus('saved');
    setOperationResult({ variant: 'success', title: t('admin.backup.title'), message: t('admin.backup.saved'), occurredAt: new Date().toISOString() });
    setToast({ tone: 'success', message: t('admin.backup.saved') });
  } catch (error) {
    const message = errorMessage(error, t('admin.backup.saveFailed'));
    setSaveStatus('error');
    setOperationResult({ variant: 'error', title: t('admin.backup.title'), message, occurredAt: new Date().toISOString() });
    setToast({ tone: 'error', message });
  } finally {
    setIsSaving(false);
  }
}
```

Replace the two draft helper functions with:

```tsx
function toggleStorage(key: string, checked: boolean) {
  setSaveStatus('dirty');
  setDraft((current) => {
    const keys = new Set(current.storageKeys);
    let targets = current.targets;
    if (checked) {
      keys.add(key);
      if (!targets.some((target) => target.storageKey === key)) {
        targets = [...targets, { storageKey: key, directory: defaultBackupDirectory }];
      }
    } else {
      keys.delete(key);
      targets = targets.filter((target) => target.storageKey !== key);
    }
    return { ...current, storageKeys: Array.from(keys), targets };
  });
}

function updateTargetDirectory(key: string, directory: string) {
  setSaveStatus('dirty');
  setDraft((current) => ({
    ...current,
    targets: current.targets.map((target) => (target.storageKey === key ? { ...target, directory } : target)),
  }));
}
```

Replace the enabled/time/retention `onChange` handlers with:

```tsx
onChange={(enabled) => {
  setSaveStatus('dirty');
  setDraft((current) => ({ ...current, enabled }));
}}

onChange={(scheduleTime) => {
  setSaveStatus('dirty');
  setDraft((current) => ({ ...current, scheduleTime: scheduleTime || '03:00' }));
}}

onChange={(retentionCount) => {
  setSaveStatus('dirty');
  setDraft((current) => ({ ...current, retentionCount }));
}}
```

- [x] **Step 3: Record backup run success, partial success, and failure**

Inside `runBackup`, after `data.result` is available, add:

```tsx
const variant = data.result.status === 'success' ? 'success' : data.result.status === 'partial' ? 'warning' : 'error';
const message = data.result.status === 'success'
  ? t('admin.backup.runSucceeded')
  : data.result.status === 'partial'
    ? t('admin.backup.runPartial')
    : t('admin.backup.runFailed');
setOperationResult({
  variant,
  title: t('admin.backup.lastRun'),
  message,
  occurredAt: data.result.startedAt || new Date().toISOString(),
  target: data.result.targets?.map((target) => target.storageName || target.storageKey).join(', '),
});
```

In the catch branch, set an error result with `new Date().toISOString()` and the selected storage names before showing the Toast.

- [x] **Step 4: Replace the backup save button and render the latest result**

Replace `.settings-form-actions.backup-form-actions` with:

```tsx
<AdminSaveBar status={saveStatus} isDirty={isDirty} saveLabel={t('admin.backup.save')} isDisabled={isRunning} onSave={() => void saveSettings()} />
<AdminOperationResultPanel result={operationResult} retryLabel={t('admin.backup.runNow')} isRetrying={isRunning} onRetry={() => void runBackup()} />
```

Keep `BackupLastRun` after the result panel because it contains per-target artifact details returned by the server.

- [x] **Step 5: Add migration activity state**

In `migration/AdminMigrationPanel.tsx`, add:

```tsx
import { AdminOperationResultPanel } from '../AdminOperationResult';
import type { AdminOperationResult } from '../adminState';
import { useState } from 'react';

const [activity, setActivity] = useState<AdminOperationResult | null>(null);
```

Replace `exportPackage`, `previewPackage`, and `applyImport` with explicit activity updates:

```tsx
async function exportPackage() {
  try {
    await migrationExport.exportPackage();
    const message = t('admin.migration.exportStarted');
    setActivity({ variant: 'success', title: t('admin.migration.exportTitle'), message, occurredAt: new Date().toISOString() });
    setToast({ tone: 'success', message });
  } catch (error) {
    const message = errorMessage(error, t('admin.migration.exportFailed'));
    setActivity({ variant: 'error', title: t('admin.migration.exportTitle'), message, occurredAt: new Date().toISOString() });
    setToast({ tone: 'error', message });
  }
}

async function previewPackage() {
  try {
    await migrationImport.previewPackage();
    const message = t('admin.migration.previewLoaded');
    setActivity({ variant: 'success', title: t('admin.migration.previewComplete'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name });
    setToast({ tone: 'success', message });
  } catch (error) {
    const message = errorMessage(error, t('admin.migration.previewFailed'));
    setActivity({ variant: 'error', title: t('admin.migration.previewRequired'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name });
    setToast({ tone: 'error', message });
  }
}

async function applyImport(confirmReplace?: string) {
  try {
    const result = await migrationImport.applyImport(confirmReplace);
    if (!result) return;
    const message = result.mode === 'replace' ? t('admin.migration.importFinishedRestarting') : t('admin.migration.importFinished');
    setActivity({
      variant: result.warnings?.length ? 'warning' : 'success',
      title: t('admin.migration.importComplete'),
      message: `${message} · ${t('admin.migration.importResult', { created: result.created, updated: result.updated, skipped: result.skipped })}`,
      occurredAt: new Date().toISOString(),
      target: migrationImport.file?.name,
    });
    setToast({ tone: 'success', message });
  } catch (error) {
    const message = errorMessage(error, t('admin.migration.importFailed'));
    setActivity({ variant: 'error', title: t('admin.migration.importTitle'), message, occurredAt: new Date().toISOString(), target: migrationImport.file?.name });
    setToast({ tone: 'error', message });
  }
}
```

- [x] **Step 6: Render migration activity below the two cards**

Add after `.migration-panel-grid`:

```tsx
<AdminOperationResultPanel
  result={activity}
  retryLabel={migrationImport.file ? (migrationImport.preview ? t('admin.migration.mergeApplyAction') : t('admin.migration.previewAction')) : undefined}
  isRetrying={migrationImport.isPreviewing || migrationImport.isImporting || migrationExport.isExporting}
  onRetry={migrationImport.file ? (migrationImport.preview ? requestApplyImport : () => void previewPackage()) : undefined}
/>
```

Do not make import retry available when no file is selected; in that case pass `onRetry={undefined}`.

- [x] **Step 7: Keep migration to one dominant action at a time**

In `MigrationExportCard.tsx`, change the export button from:

```tsx
variant="primary"
```

to:

```tsx
variant="secondary"
```

The import button remains `primary` for merge and `destructive` for replace only after preview. Preview and export remain secondary setup actions.

- [x] **Step 8: Test and type-check**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
cd client && npx tsc -b
```

Expected: PASS. Backup save/run and migration export/preview/import leave an inline result; all buttons prevent repeat submission while their operation is active.

- [x] **Step 9: Commit operational results**

```bash
git add client/src/modules/admin/AdminBackupPanel.tsx client/src/modules/admin/migration/AdminMigrationPanel.tsx client/src/modules/admin/migration/MigrationExportCard.tsx client/src/styles/admin.css
git commit -m "feat(admin): persist backup and migration results"
```

## Task 6: Replace Two-Click Destructive Actions and Prevent Duplicate Admin Mutations

**Files:**
- Modify: `client/src/modules/admin/AdminPanel.tsx:170,544-641,826-1011,invite/taxonomy/collection renders`
- Modify: `client/src/modules/admin/AdminUsersWorkspace.tsx`
- Modify: `client/src/modules/admin/AdminUsersPanel.tsx`
- Modify: `client/src/modules/admin/AdminGroupsPanel.tsx`
- Modify: `client/src/modules/admin/AdminAnnouncementsPanel.tsx`
- Modify: `client/src/styles/admin.css`

**Interfaces:**
- Consumes: `AdminDeleteDialog`.
- Produces: one `AdminPanel` delete target for user/category/tag/collection/invite; storage deletion remains the Task 4 dialog.
- Keeps: existing delete endpoints and success/error copy keys.

- [x] **Step 1: Add a typed delete target to `AdminPanel`**

Replace `confirmDelete` state with:

```tsx
type AdminDeleteTarget =
  | { kind: 'user'; item: User }
  | { kind: 'category'; item: Category }
  | { kind: 'tag'; item: TagRecord }
  | { kind: 'collection'; item: Collection }
  | { kind: 'invite'; item: RegistrationInvite };

const [deleteTarget, setDeleteTarget] = useState<AdminDeleteTarget | null>(null);
const [isDeleting, setIsDeleting] = useState(false);
```

- [x] **Step 2: Replace the five double-click functions with one direct dispatcher**

Remove the preflight portions from `deleteManagedUser`, `deleteCategory`, `deleteTag`, `deleteCollection`, and `deleteRegistrationInvite`. Replace them with:

```tsx
async function confirmAdminDelete() {
  if (!deleteTarget || isDeleting) return;
  setIsDeleting(true);
  try {
    switch (deleteTarget.kind) {
      case 'user':
        await api(`/api/v1/admin/users/${deleteTarget.item.id}`, { method: 'DELETE' });
        setToast({ tone: 'neutral', message: t('admin.userDeleted') });
        await fetchUsersPage(userPagination.page, userPagination.pageSize);
        break;
      case 'category':
        await api(`/api/v1/admin/categories/${deleteTarget.item.id}`, { method: 'DELETE' });
        setToast({ tone: 'neutral', message: t('admin.categoryDeleted') });
        {
          const data = await api<{ categories: Category[] }>('/api/v1/admin/categories');
          setAdminCategories(data.categories || []);
        }
        await onCatalogMetadataChanged();
        break;
      case 'tag':
        await api(`/api/v1/admin/tags/${deleteTarget.item.id}`, { method: 'DELETE' });
        setToast({ tone: 'neutral', message: t('admin.tagDeleted') });
        {
          const data = await api<{ tags: TagRecord[] }>('/api/v1/admin/tags');
          setAdminTags(data.tags || []);
        }
        await onCatalogMetadataChanged();
        break;
      case 'collection':
        await api(`/api/v1/admin/collections/${deleteTarget.item.id}`, { method: 'DELETE' });
        setToast({ tone: 'neutral', message: t('admin.collectionDeleted') });
        {
          const data = await api<{ collections: Collection[] }>('/api/v1/admin/collections');
          setAdminCollections(data.collections || []);
        }
        break;
      case 'invite':
        await api(`/api/v1/admin/registration-invites/${deleteTarget.item.id}`, { method: 'DELETE' });
        setToast({ tone: 'neutral', message: t('admin.inviteDeleted') });
        await fetchInvitesPage(invitePagination.page, invitePagination.pageSize);
        break;
    }
    setDeleteTarget(null);
  } catch (error) {
    const fallback = deleteTarget.kind === 'user'
      ? t('admin.userDeleteFailed')
      : deleteTarget.kind === 'category'
        ? t('admin.categoryDeleteFailed')
        : deleteTarget.kind === 'tag'
          ? t('admin.tagDeleteFailed')
          : deleteTarget.kind === 'collection'
            ? t('admin.collectionDeleteFailed')
            : t('admin.inviteDeleteFailed');
    setToast({ tone: 'error', message: errorMessage(error, fallback) });
  } finally {
    setIsDeleting(false);
  }
}
```

- [x] **Step 3: Turn existing delete callbacks into dialog requests**

Change the `deleteManagedUser` prop type in both `AdminUsersWorkspace.tsx` and `AdminUsersPanel.tsx` from `Promise<void>` to `void`:

```tsx
deleteManagedUser: (item: User) => void;
```

Pass to `AdminUsersWorkspace`:

```tsx
deleteManagedUser={(item) => setDeleteTarget({ kind: 'user', item })}
```

Change taxonomy, collection, and invite delete button handlers to:

```tsx
onClick={() => setDeleteTarget({ kind: 'category', item })}
onClick={() => setDeleteTarget({ kind: 'tag', item })}
onClick={() => setDeleteTarget({ kind: 'collection', item })}
onClick={() => setDeleteTarget({ kind: 'invite', item: invite })}
```

Do not change edit/create handlers.

- [x] **Step 4: Derive object-specific confirmation copy and render one dialog**

Before `return`, add:

```tsx
function deleteDialogCopy(target: AdminDeleteTarget) {
  switch (target.kind) {
    case 'user': {
      const name = displayUserName(target.item);
      return { title: t('admin.deleteUserNamed', { name }), subject: name, consequence: t('admin.confirmDeleteUser', { name }) };
    }
    case 'category': {
      const name = localizedName(target.item);
      return { title: t('admin.deleteCategoryNamed', { name }), subject: name, consequence: t('admin.confirmDeleteCategory', { name }) };
    }
    case 'tag': {
      const name = localizedName(target.item);
      return { title: t('admin.deleteTagNamed', { name }), subject: name, consequence: t('admin.confirmDeleteTag', { name }) };
    }
    case 'collection':
      return { title: t('admin.deleteCollectionNamed', { name: target.item.name }), subject: target.item.name, consequence: t('admin.confirmDeleteCollection', { name: target.item.name }) };
    case 'invite':
      return { title: t('admin.deleteInvite'), subject: target.item.note || target.item.code, consequence: t('admin.confirmDeleteInvite', { code: target.item.code }) };
  }
}

const activeDeleteCopy = deleteTarget ? deleteDialogCopy(deleteTarget) : null;
```

Render before the root closing tag:

```tsx
{deleteTarget && activeDeleteCopy && (
  <AdminDeleteDialog
    {...activeDeleteCopy}
    confirmLabel={t('common.delete')}
    isDeleting={isDeleting}
    onCancel={() => setDeleteTarget(null)}
    onConfirm={() => void confirmAdminDelete()}
  />
)}
```

- [x] **Step 5: Keep user row actions disabled while the requested delete is active**

Extend `AdminUsersWorkspace` and `AdminUsersPanel` with:

```tsx
isDeletingUserID?: number;
```

Pass `isDeletingUserID={deleteTarget?.kind === 'user' && isDeleting ? deleteTarget.item.id : undefined}` through the workspace. In `AdminUsersPanel`, apply it to edit, enable/disable, and delete controls:

```tsx
const rowBusy = isDeletingUserID === item.id;
// each row action: isDisabled={rowBusy}
// delete action additionally: isLoading={rowBusy}
```

- [x] **Step 6: Add pending state to existing group dialogs**

In `AdminGroupsPanel.tsx`, add:

```tsx
const [pendingAction, setPendingAction] = useState<'delete' | 'rotate' | null>(null);
```

Replace `deleteGroup` and `rotateGroupCode` with:

```tsx
async function deleteGroup(group: Group) {
  if (pendingAction) return;
  setPendingAction('delete');
  try {
    await runAction(setToast, t('groups.deleteFailed'), async () => {
      await api(`/api/v1/groups/${group.id}`, { method: 'DELETE' });
      setGroupToDelete(null);
      setToast({ tone: 'neutral', message: t('groups.deleted') });
      await loadGroups();
    });
  } finally {
    setPendingAction(null);
  }
}

async function rotateGroupCode(group: Group) {
  if (pendingAction) return;
  setPendingAction('rotate');
  try {
    await runAction(setToast, t('admin.groups.rotateFailed'), async () => {
      await api(`/api/v1/groups/${group.id}/code:rotate`, { method: 'POST' });
      setGroupCodeToRotate(null);
      setToast({ tone: 'success', message: t('admin.groups.rotated') });
      await loadGroups();
    });
  } finally {
    setPendingAction(null);
  }
}
```

Update the destructive buttons:

```tsx
isDisabled={pendingAction !== null}
isLoading={pendingAction === 'delete'}
```

and:

```tsx
isDisabled={pendingAction !== null}
isLoading={pendingAction === 'rotate'}
```

The existing group copy already names the group and describes the code/membership consequence; do not replace those dialogs.

- [x] **Step 7: Add pending deletion to announcements**

In `AdminAnnouncementsPanel.tsx`, add `const [isDeleting, setIsDeleting] = useState(false);` and replace `deleteAnnouncement` with:

```tsx
async function deleteAnnouncement() {
  if (!deleteTarget?.id || isDeleting) return;
  setIsDeleting(true);
  try {
    const data = await api<{ site?: SiteProfile }>(`/api/v1/admin/announcements/${deleteTarget.id}`, { method: 'DELETE' });
    setDeleteTarget(null);
    setToast({ tone: 'neutral', message: t('admin.announcementDeleted') });
    await onReload();
    await onSiteProfileSaved(data.site);
  } catch (error) {
    setToast({ tone: 'error', message: errorMessage(error, t('admin.announcementDeleteFailed')) });
  } finally {
    setIsDeleting(false);
  }
}
```

Update the modal buttons:

```tsx
<XButton type="button" variant="secondary" label={t('common.cancel')} isDisabled={isDeleting} onClick={() => setDeleteTarget(null)} />
<XButton type="button" variant="destructive" label={t('admin.deleteAnnouncement')} icon={<Trash2 size={17} />} isDisabled={isDeleting} isLoading={isDeleting} onClick={() => void deleteAnnouncement()} />
```

`AdminAdsPanel` already tracks `busyAdID`; leave it unchanged.

- [x] **Step 8: Add confirmation layout styles**

Append to `client/src/styles/admin.css`:

```css
.admin-delete-dialog {
  display: grid;
  gap: 14px;
  width: min(520px, 100%);
}

.admin-delete-copy {
  background: var(--status-danger-bg);
  border: 1px solid var(--status-danger-line);
  border-radius: 8px;
  color: var(--status-danger-ink);
  display: grid;
  gap: 6px;
  padding: 12px;
}

.admin-delete-copy p {
  line-height: 1.5;
  margin: 0;
  overflow-wrap: anywhere;
}
```

- [x] **Step 9: Test and type-check**

Run:

```bash
node --test client/src/modules/admin/adminState.test.mjs
cd client && npx tsc -b
```

Expected: PASS. No `confirmDelete` string state remains in `AdminPanel`; all destructive dialogs name the target; repeated confirm clicks issue only one request.

- [x] **Step 10: Commit destructive workflow hardening**

```bash
git add client/src/modules/admin/AdminPanel.tsx client/src/modules/admin/AdminUsersWorkspace.tsx client/src/modules/admin/AdminUsersPanel.tsx client/src/modules/admin/AdminGroupsPanel.tsx client/src/modules/admin/AdminAnnouncementsPanel.tsx client/src/styles/admin.css
git commit -m "feat(admin): make destructive workflows explicit"
```

## Task 7: Finish Admin Motion, Touch, Responsive, and Integration Handoff

**Files:**
- Modify: `client/src/styles/admin.css`
- Verify only: `client/src/modules/admin/**`
- Do not modify: `client/src/App.tsx`, locales, shared global styles, or any dist directory.

**Interfaces:**
- Consumes: all admin classes introduced by Tasks 1–6.
- Produces: no new TypeScript API; this is the admin-only quality gate.

- [x] **Step 1: Add press feedback only to admin pressable controls**

Append:

```css
.admin-shell :is(.astryx-button, .astryx-icon-button, .storage-config-main) {
  transition:
    transform 140ms var(--ease-out),
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease,
    opacity 160ms ease;
}

.admin-shell :is(.astryx-button, .astryx-icon-button, .storage-config-main):active:not(:disabled) {
  transform: scale(0.98);
}

@media (hover: hover) and (pointer: fine) {
  .admin-shell .admin-task-header-actions :is(.astryx-button, .astryx-icon-button):hover {
    border-color: color-mix(in srgb, var(--green) 38%, var(--line));
  }
}
```

- [x] **Step 2: Make admin actions touch-reachable at all supported widths**

Append:

```css
.admin-shell .row-actions,
.admin-shell .storage-config-actions {
  opacity: 1;
  transform: none;
}

.admin-shell :is(.row-actions, .storage-config-actions) > button {
  min-height: 40px;
  min-width: 40px;
}

@media (max-width: 768px) {
  .admin-task-header,
  .admin-operation-result-head {
    display: grid;
  }

  .admin-task-header-actions,
  .admin-operation-result-head {
    justify-content: flex-start;
  }

  .admin-shell :is(.row-actions, .storage-config-actions) {
    flex-wrap: wrap;
    justify-content: flex-start;
    width: 100%;
  }
}

@media (max-width: 375px) {
  .admin-task-heading {
    grid-template-columns: 1fr;
  }

  .admin-task-icon {
    height: 32px;
    width: 32px;
  }

  .admin-task-header-actions,
  .admin-task-header-actions > button,
  .admin-operation-result > button {
    width: 100%;
  }
}
```

- [x] **Step 3: Remove transform motion for reduced-motion users**

Append:

```css
@media (prefers-reduced-motion: reduce) {
  .admin-shell *,
  .admin-shell *::before,
  .admin-shell *::after {
    scroll-behavior: auto !important;
  }

  .admin-shell :is(.astryx-button, .astryx-icon-button, .storage-config-main) {
    transition:
      background-color 120ms ease,
      border-color 120ms ease,
      color 120ms ease,
      opacity 120ms ease;
  }

  .admin-shell :is(.astryx-button, .astryx-icon-button, .storage-config-main):active:not(:disabled) {
    transform: none;
  }
}
```

- [x] **Step 4: Run static tests, type-check, and a temporary production build**

Run from the repository root:

```bash
node --test client/src/modules/admin/adminState.test.mjs
cd client && npx tsc -b
npx vite build --outDir /tmp/lazycat-admin-ux-build --emptyOutDir
```

Expected: tests PASS; TypeScript PASS; Vite writes only `/tmp/lazycat-admin-ux-build`; repository dist directories are not touched.

- [x] **Step 5: Verify no forbidden FE-3 files were edited by this task series**

Run:

```bash
git diff --name-only HEAD~6..HEAD | sort
```

Expected: every source path is under `client/src/modules/admin/` or is `client/src/styles/admin.css`. If commits are squashed or the count differs, compare against the FE-3 starting commit instead. Existing unrelated WIP outside these paths must remain untouched.

- [x] **Step 6: Run the desktop/mobile browser matrix**

Start the existing development server without generating bundles:

```bash
cd client && npm run dev -- --host 127.0.0.1
```

With a site-admin session, verify each width: 1440, 1024, 768, 375, 320. Expected for every width:

1. Reviews, Site, Users, Taxonomy, Collections, Storage, Backup, and Migration are reachable from the first-level admin tabs without page overflow.
2. Switching first-level and Site sub-tabs has no vertical slide.
3. Time-zone selector remains present under Site → Identity.
4. Editing Site, Storage, and Backup shows unsaved status; save is disabled while clean and pending; mobile save bars remain above the safe area.
5. Storage test, backup run, and migration preview/import results remain visible after Toast dismissal and show target/time.
6. User, category, tag, collection, invite, storage, group, announcement, and ad actions are reachable without hover.
7. Destructive dialogs name the target, explain the consequence, close with Escape when idle, and cannot issue a duplicate request while pending.
8. Light, dark, and one non-default Astryx theme preserve readable status/error colors.
9. With `prefers-reduced-motion: reduce`, tab switching and button press do not translate or scale.

- [x] **Step 7: Prepare the INT language-pack handoff without editing locales**

Give INT this exact key set. FE-3 code may reference the keys before INT merges them; do not add them directly to `en.ts` or `zh.ts` in this task.

| Key | English | 简体中文 |
| --- | --- | --- |
| `admin.saveState.unsaved` | Unsaved changes | 有未保存的更改 |
| `admin.saveState.saving` | Saving… | 正在保存… |
| `admin.saveState.saved` | Saved | 已保存 |
| `admin.saveState.failed` | Save failed | 保存失败 |
| `admin.saveState.noChanges` | No unsaved changes | 没有未保存的更改 |

INT must also replace the existing “click again” strings with consequence copy while keeping the existing keys:

| Existing key | Required English meaning | 所需中文含义 |
| --- | --- | --- |
| `admin.confirmDeleteUser` | Deleting the named user removes access and owned session state; this cannot be undone. | 删除指定用户会移除其访问权限和会话状态，且无法撤销。 |
| `admin.confirmDeleteCategory` | Deleting the named category removes it from storefront navigation; apps must be recategorized if required. | 删除指定分类会将其从商店导航中移除，相关应用可能需要重新分类。 |
| `admin.confirmDeleteTag` | Deleting the named tag removes that classification from associated apps. | 删除指定标签会从相关应用中移除该分类。 |
| `admin.confirmDeleteCollection` | Deleting the named collection removes its storefront section but does not delete apps. | 删除指定聚合会移除对应商店区块，但不会删除应用。 |
| `admin.confirmDeleteStorage` | Deleting the named storage removes its configuration; existing remote objects are not deleted automatically. | 删除指定存储会移除其配置，远端已有对象不会被自动删除。 |
| `admin.confirmDeleteInvite` | Deleting the invite immediately prevents all remaining uses of that code. | 删除邀请码后，该码剩余次数会立即失效。 |

- [x] **Step 8: Commit final admin polish**

```bash
git add client/src/styles/admin.css
git commit -m "style(admin): polish responsive operational workflows"
```

## INT Integration Contract

INT owns and performs these changes after FE-1, FE-2, and FE-3 are merged:

1. Add the five `admin.saveState.*` keys and update the six existing destructive-confirmation keys in both `client/src/locales/en.ts` and `client/src/locales/zh.ts` using the Task 7 table.
2. Do not move the promoted admin task state into `client/src/App.tsx`; `AdminPanel` remains the feature boundary.
3. Resolve any shared global-style overlap in `client/src/styles.css` only after confirming `admin.css` scoping is insufficient; FE-3 does not edit global styles.
4. Run the single final frontend build and update `clientembed/dist` once after all frontend tasks merge.
5. Run `npm ci`, audit, TypeScript/Vite build, and the complete desktop/mobile browser matrix after generated assets match source.

## Final Self-Review Checklist

- Spec coverage: first-level tasks, task headers/status, dirty save state, sticky mobile save, explicit consequences, touch reachability, persistent storage/backup/migration results, instant tab switching, and reduced motion each map to a concrete task above.
- Scope control: no `App.tsx`, locale, shared global style, backend, API, or generated dist edit is assigned to FE-3.
- WIP preservation: the existing `timeZoneOptions` and `site_timezone` changes are called out before, during, and after `AdminPanel` edits.
- Type consistency: `AdminSaveStatus`, `AdminOperationResult`, `AdminStorageAction`, and component prop names are defined once and reused consistently.
- No broad refactor: `AdminPanel` remains the data owner; only five focused admin primitives and one pure helper module are extracted.
- No placeholders: every change step contains exact paths, signatures, code, commands, and expected outcomes.

## Completion Evidence — 2026-07-10

- Admin state/contract tests: `node --test client/src/modules/admin/adminState.test.mjs` — 14 passed, 0 failed.
- TypeScript: `npm exec --prefix client -- tsc -b client/tsconfig.json` — passed without diagnostics.
- Production build: `npx vite build --outDir /tmp/lazycat-admin-ux-build --emptyOutDir` — passed; only the existing chunk-size warning remains, and repository dist directories were not written.
- Independent code review: three rounds completed; final result was 0 Critical, 0 Important, 0 Minor, Ready.
- Browser smoke matrix against a temporary local site-admin server: 320, 375, 768, 1024, and 1440 CSS-pixel widths all reported `documentElement.scrollWidth === innerWidth`; all eight first-level admin tasks were reachable; Storage dirty state enabled Save; Site → Identity retained the time-zone selector; browser console had no application errors.
- Language-pack observation: the temporary browser run exposed the expected raw `admin.saveState.*` keys because locale ownership is intentionally assigned to INT below. The integration plan must add those translations before final release validation.
- Commit commands in the task steps were intentionally deferred because the user requested continuing in the existing dirty worktree without committing or publishing.
