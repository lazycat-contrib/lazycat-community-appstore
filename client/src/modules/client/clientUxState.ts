import type { ClientInstallTask, InstallActivity } from '../../shared/types';

export type InstallActivitySnapshot = {
  status: InstallActivity['status'];
  stageKey: string;
};

export type InstallTimelineKey = 'queued' | 'prepare' | 'system' | 'result';
export type InstallTimelineState = 'pending' | 'current' | 'complete' | 'error';
export type InstallTimelineItem = { key: InstallTimelineKey; state: InstallTimelineState };

const installTimelineOrder: InstallTimelineKey[] = ['queued', 'prepare', 'system', 'result'];

function activeInstallStage(stageKey: string): InstallTimelineKey {
  if (stageKey.endsWith('stageDone') || stageKey.endsWith('stageFailed') || stageKey.endsWith('stageCancelled')) return 'result';
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
    if (key === 'result' && (activity.status === 'error' || activity.status === 'cancelled')) return { key, state: 'error' };
    if (activity.status === 'success') return { key, state: 'complete' };
    return { key, state: 'current' };
  });
}

export function installTaskState(task: ClientInstallTask): Pick<InstallActivity, 'status' | 'stageKey'> & { isTerminal: boolean } {
  const status = task.status.trim().toUpperCase();
  if (status === 'INSTALL_OK' || status === 'SUCCEEDED' || status === 'SUCCESS') {
    return { status: 'success', stageKey: 'installActivity.stageDone', isTerminal: true };
  }
  if (status === 'CANCELLED' || status === 'CANCELED') {
    return { status: 'cancelled', stageKey: 'installActivity.stageCancelled', isTerminal: true };
  }
  if (status.endsWith('_ERR') || status === 'FAILED' || status === 'ERROR') {
    return { status: 'error', stageKey: 'installActivity.stageFailed', isTerminal: true };
  }
  if (status === 'CREATING' || status === 'QUEUED') {
    return { status: 'running', stageKey: 'installActivity.stageQueued', isTerminal: false };
  }
  return { status: 'running', stageKey: 'installActivity.stageSystem', isTerminal: false };
}

export function installTaskProgress(task: ClientInstallTask) {
  const total = Number(task.totalSize);
  const downloaded = Number(task.downloadedSize || 0);
  if (!Number.isFinite(total) || !Number.isFinite(downloaded) || total <= 0) {
    return { progress: 0, progressKnown: false };
  }
  return { progress: Math.max(0, Math.min(100, Math.round((downloaded / total) * 100))), progressKnown: true };
}

export function inspectionPresentation(state?: string) {
	switch ((state || '').trim().toUpperCase()) {
		case 'PENDING':
			return { stateKey: 'pending', isActive: true, statusVariant: 'accent' as const };
		case 'RUNNING':
			return { stateKey: 'running', isActive: true, statusVariant: 'accent' as const };
		case 'SUCCEEDED':
			return { stateKey: 'succeeded', isActive: false, statusVariant: 'success' as const };
		case 'FAILED':
			return { stateKey: 'failed', isActive: false, statusVariant: 'error' as const };
		case 'TIMED_OUT':
			return { stateKey: 'timedOut', isActive: false, statusVariant: 'error' as const };
		case 'CANCELLED':
			return { stateKey: 'cancelled', isActive: false, statusVariant: 'neutral' as const };
		default:
			return { stateKey: 'unknown', isActive: false, statusVariant: 'neutral' as const };
	}
}

export function buildUpdateConfirmation<T extends {
  item: { appid?: string };
  source?: { packageId?: string; installProtected?: boolean };
}>(rows: T[]) {
  const eligible: string[] = [];
  const skipped: string[] = [];
	for (const row of rows) {
	  const packageID = row.source?.packageId || row.item.appid || '';
	  if (!packageID) continue;
	  if (row.source?.installProtected) {
		 skipped.push(packageID);
	  } else {
		 eligible.push(packageID);
	  }
	}
	return { eligible, skipped };
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
  autoUpdateEnabled: boolean;
  autoUpdateIntervalMinutes: number;
  lastAutoUpdateAt?: string;
  lastAutoUpdateStatus?: string;
  lastAutoUpdateError?: string;
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
    autoUpdateEnabled: Boolean(settings.autoUpdateEnabled),
    autoUpdateIntervalMinutes: Number(settings.autoUpdateIntervalMinutes) || 60,
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
