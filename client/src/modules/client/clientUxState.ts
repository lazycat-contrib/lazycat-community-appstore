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

function normalizeStableAppIdentity(value?: string) {
  return (value || '').trim().toLowerCase();
}

export function findStableSourceApp<T extends { packageId?: string; slug?: string }>(
  installed: { appid?: string },
  sourceApps: T[],
) {
  const installedID = normalizeStableAppIdentity(installed.appid);
  if (!installedID) return undefined;
  return sourceApps.find((app) => {
    const packageID = normalizeStableAppIdentity(app.packageId);
    const slug = normalizeStableAppIdentity(app.slug);
    return installedID === packageID || installedID === slug;
  });
}
