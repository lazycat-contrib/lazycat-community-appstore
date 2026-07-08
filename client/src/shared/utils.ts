import i18n from '../i18n';
import { API_BASE } from '../config';
import { ApiRequestError } from './api';
import { SOURCE_STALE_MS } from './constants';
import type {
  GitHubMirror,
  InstalledApplication,
  SiteProfile,
  SourceActionKey,
  SourceApp,
  SourceSubscription,
  SourceVersion,
  StoreApp,
  Toast,
} from './types';

export function errorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiRequestError && error.code) {
    const localized = i18n.t(`errors.${error.code}`, { defaultValue: '' });
    if (localized) return localized;
  }
  return error instanceof Error && error.message ? error.message : fallback;
}

export async function runAction<T>(setToast: (toast: Toast) => void, fallback: string, action: () => Promise<T>): Promise<T | undefined> {
  try {
    return await action();
  } catch (error) {
    setToast({ tone: 'error', message: errorMessage(error, fallback) });
    return undefined;
  }
}

export function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

export function arrayOrEmpty<T>(value?: T[]) {
  return Array.isArray(value) ? value : [];
}

export function formatBytes(size?: number) {
  if (!size) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function formatDate(value?: string) {
  if (!value) return '-';
  const locale = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en-US' : 'zh-CN';
  return new Intl.DateTimeFormat(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}

export function hasInstallableVersion(app: StoreApp | SourceApp) {
  return Boolean(app.latestVersion?.downloadUrl);
}

export function localizedName(record: { name: string; nameI18n?: Record<string, string> }) {
  return localizedText(record.nameI18n, record.name);
}

export function localizedAppName(app: StoreApp | SourceApp) {
  return localizedText(app.nameI18n, app.name);
}

export function localizedAppSummary(app: StoreApp | SourceApp, fallback = '') {
  return localizedText(app.summaryI18n, app.summary || fallback);
}

export function localizedAppDescription(app: StoreApp | SourceApp, fallback = '') {
  const description = 'description' in app ? app.description : '';
  return localizedText(app.descriptionI18n, description || fallback);
}

export function localizedText(values?: Record<string, string>, fallback = '') {
  const language = (i18n.resolvedLanguage || i18n.language || '').toLowerCase();
  const candidates = language.startsWith('zh') ? ['zh-CN', 'zh_Hans', 'zh', 'en'] : ['en', 'zh-CN', 'zh_Hans', 'zh'];
  for (const key of candidates) {
    const value = values?.[key]?.trim();
    if (value) return value;
  }
  return fallback;
}

export function localizedCategory(record: { category?: string; categoryI18n?: Record<string, string> }, fallback = '') {
  return localizedText(record.categoryI18n, record.category || fallback);
}

export function selectedSourceVersion(app: SourceApp, version?: string) {
  const wanted = version?.trim();
  if (wanted) {
    return app.versions?.find((item) => item.version === wanted) || (app.latestVersion?.version === wanted ? app.latestVersion : undefined);
  }
  return app.latestVersion;
}

function normalizeVersionParts(value?: string) {
  const raw = (value || '').trim().replace(/^[vV]/, '');
  if (!raw) return null;
  const main = raw.split(/[+-]/)[0];
  const parts = main.split('.');
  if (parts.some((part) => !/^\d+$/.test(part))) return null;
  return parts.map((part) => Number(part));
}

export function compareVersions(a?: string, b?: string) {
  const aParts = normalizeVersionParts(a);
  const bParts = normalizeVersionParts(b);
  if (!aParts || !bParts) {
    const left = (a || '').trim();
    const right = (b || '').trim();
    if (!left || !right) return 0;
    return left === right ? 0 : left.localeCompare(right, undefined, { numeric: true, sensitivity: 'base' });
  }
  const length = Math.max(aParts.length, bParts.length);
  for (let index = 0; index < length; index += 1) {
    const left = aParts[index] || 0;
    const right = bParts[index] || 0;
    if (left !== right) return left > right ? 1 : -1;
  }
  return 0;
}

export function sourceInstallAction(app: SourceApp, installedMatch?: InstalledApplication): SourceActionKey {
  if (!hasInstallableVersion(app)) return 'unavailable';
  if (!installedMatch?.version) return installedMatch ? 'reinstall' : 'install';
  const comparison = compareVersions(installedMatch.version, app.latestVersion?.version);
  if (comparison < 0) return 'update';
  return 'reinstall';
}

export function isSourceAppUpdateAvailable(app: SourceApp, installedApps: InstalledApplication[]) {
  const installedMatch = findInstalledApplication(app, installedApps);
  return sourceInstallAction(app, installedMatch) === 'update';
}

export function sourceActionLabel(t: (key: string, options?: any) => string, action: SourceActionKey) {
  return action === 'update'
    ? t('common.update')
    : action === 'reinstall'
      ? t('common.reinstall')
      : action === 'install'
        ? t('common.install')
        : t('common.unavailable');
}

export function withInstallPassword(rawUrl: string, password?: string) {
  if (!password) return rawUrl;
  try {
    const url = new URL(rawUrl, window.location.href);
    url.searchParams.set('installPassword', password);
    return url.toString();
  } catch {
    const separator = rawUrl.includes('?') ? '&' : '?';
    return `${rawUrl}${separator}installPassword=${encodeURIComponent(password)}`;
  }
}

export function shortSHA(value?: string) {
  return value ? value.slice(0, 16) : '-';
}

export function stripTrailingSlash(value: string) {
  return value.trim().replace(/\/+$/, '');
}

export function defaultSiteProfile(title: string): SiteProfile {
  const publicUrl = stripTrailingSlash(API_BASE || window.location.origin);
  return {
    title,
    subtitle: '',
    publicUrl,
    sourceUrl: `${publicUrl}/source/v1/index.json`,
    defaultPageSize: 24,
    announcement: { enabled: false, level: 'info' },
    registration: { mode: 'open' },
  };
}

export function normalizeAppIdentity(value?: string) {
  return (value || '').trim().toLowerCase();
}

export function findInstalledApplication(app: StoreApp | SourceApp, installedApps: InstalledApplication[]) {
  const packageID = normalizeAppIdentity('packageId' in app ? app.packageId : undefined);
  const appID = normalizeAppIdentity(app.slug);
  const appName = normalizeAppIdentity(app.name);
  return installedApps.find((item) => {
    const installedID = normalizeAppIdentity(item.appid);
    if (packageID && installedID && packageID === installedID) return true;
    if (appID && installedID) return appID === installedID;
    return appName !== '' && normalizeAppIdentity(item.title) === appName;
  });
}

export function findSourceForInstalled(item: InstalledApplication, sourceApps: SourceApp[]) {
  const installedID = normalizeAppIdentity(item.appid);
  const installedTitle = normalizeAppIdentity(item.title);
  return sourceApps.find((app) => {
    const packageID = normalizeAppIdentity(app.packageId);
    const slug = normalizeAppIdentity(app.slug);
    const name = normalizeAppIdentity(app.name);
    return (
      (installedID && (installedID === packageID || installedID === slug)) ||
      (installedTitle && installedTitle === name)
    );
  });
}

export function belongsToSource(app: SourceApp, source: SourceSubscription) {
  return app.sourceId !== undefined ? String(app.sourceId) === String(source.id) : app.sourceName === source.name;
}

export function sourceForApp(app: SourceApp, sources: SourceSubscription[]) {
  return sources.find((source) => belongsToSource(app, source));
}

export function githubMirrorKindForURL(rawURL?: string): GitHubMirror['kind'] | '' {
  const value = (rawURL || '').trim();
  if (value.includes('raw.githubusercontent.com/')) return 'raw';
  if (value.includes('github.com/')) return 'download';
  return '';
}

export function applicableMirrorsForVersion(source: SourceSubscription | undefined, version?: Pick<SourceVersion, 'downloadUrl' | 'upstreamDownloadUrl'>) {
  if (!source || !version) return [];
  const kind = githubMirrorKindForURL(version.upstreamDownloadUrl || version.downloadUrl);
  if (!kind) return [];
  return arrayOrEmpty(source.githubMirrors).filter((entry) => entry.kind === kind);
}

export function defaultMirrorIDForVersion(source: SourceSubscription | undefined, version?: Pick<SourceVersion, 'downloadUrl' | 'upstreamDownloadUrl'>) {
  const kind = githubMirrorKindForURL(version?.upstreamDownloadUrl || version?.downloadUrl);
  if (!source || !kind) return '';
  return kind === 'raw' ? source.defaultRawMirrorId || '' : source.defaultDownloadMirrorId || '';
}

export function sourceMirrorOptions(source: SourceSubscription | null | undefined, kind: GitHubMirror['kind'], directLabel: string) {
  return [
    { value: '', label: directLabel },
    ...arrayOrEmpty(source?.githubMirrors).filter((entry) => entry.kind === kind).map((entry) => ({ value: entry.id, label: entry.name })),
  ];
}

export function sourceMirrorSummary(source: SourceSubscription, kind: GitHubMirror['kind'], fallback: string) {
  const mirrors = arrayOrEmpty(source.githubMirrors).filter((entry) => entry.kind === kind);
  const defaultID = kind === 'raw' ? source.defaultRawMirrorId : source.defaultDownloadMirrorId;
  const selected = mirrors.find((entry) => entry.id === defaultID);
  return selected ? selected.name : fallback;
}

export function isSourceStale(source: SourceSubscription) {
  if (!source.lastSync || source.lastError) return false;
  const syncedAt = Date.parse(source.lastSync);
  return Number.isFinite(syncedAt) && Date.now() - syncedAt > SOURCE_STALE_MS;
}

export function statusKey(value?: string) {
  return (value || 'UNKNOWN').toLowerCase().replaceAll('_', '');
}

export function reviewKindKey(value?: string) {
  return (value || 'APP_SUBMISSION').toLowerCase().replaceAll('_', '');
}
