import {
  AlertCircle,
  Archive,
  ArrowDown,
  ArrowUp,
  Check,
  ChevronRight,
  Cloud,
  Copy,
  Download,
  Gauge,
  Heart,
  History,
  Home,
  KeyRound,
  Layers3,
  Link,
  LogIn,
  LogOut,
  MessageSquare,
  Monitor,
  Moon,
  PackagePlus,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Save,
  Star,
  Sun,
  Tag,
  Trash2,
  Upload,
  Users,
  X,
} from 'lucide-react';
import { Avatar } from '@humation/react';
import { humation1 } from '@humation/assets-humation-1';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import i18n from './i18n';
import { API_BASE, DEFAULT_SOURCE_NAME, DEFAULT_SOURCE_URL, HAS_API } from './config';

type User = {
  id: number;
  username: string;
  email?: string;
  role: 'USER' | 'SOFTWARE_ADMIN' | 'SITE_ADMIN';
  emailVerified?: boolean;
};

type Version = {
  id: number;
  appId: number;
  version: string;
  changelog: string;
  status: string;
  sourceType: string;
  downloadUrl: string;
  fileSize: number;
  sha256: string;
  createdAt: string;
  publishedAt?: string;
};

type StoreApp = {
  id: number;
  ownerId: number;
  owner: string;
  categoryId?: number;
  name: string;
  slug: string;
  summary: string;
  description: string;
  status: string;
  category?: string;
  tags: string[];
  visibleGroupIds: number[];
  allowUnreviewedUpdates: boolean;
  commentsEnabled: boolean;
  installProtected: boolean;
  downloadCount: number;
  latestVersion?: Version;
  versions?: Version[];
  screenshots?: Screenshot[];
  comments?: Comment[];
  favorites?: number;
  outdatedMarks?: number;
  canManageApp?: boolean;
  canUploadVersion?: boolean;
  updatedAt: string;
};

type Screenshot = {
  id: number;
  appId: number;
  imageUrl: string;
  caption: string;
  sortOrder: number;
};

type Comment = {
  id: number;
  userId: number;
  username: string;
  body: string;
  createdAt: string;
};

type Review = {
  id: number;
  kind: string;
  status: string;
  appId?: number;
  versionId?: number;
  requesterId: number;
  note: string;
  reviewNote?: string;
  createdAt: string;
};

type Category = {
  id: number;
  name: string;
  slug: string;
  sortOrder?: number;
};

type TagRecord = {
  id: number;
  name: string;
  slug: string;
};

type Collection = {
  id: number;
  name: string;
  slug: string;
  description: string;
  kind: string;
  apps: StoreApp[];
};

type CollaboratorRequest = {
  id: number;
  app_id?: number;
  appId?: number;
  user_id?: number;
  userId?: number;
  username?: string;
  email?: string;
  status: string;
  message: string;
  created_at?: string;
  createdAt?: string;
};

type Group = {
  id: number;
  owner_id?: number;
  ownerId?: number;
  name: string;
  slug: string;
  description: string;
};

type APITokenRecord = {
  id: number;
  name: string;
  prefix: string;
  created_at?: string;
  createdAt?: string;
};

type SourceID = number | string;

type SourceSubscription = {
  id: SourceID;
  name: string;
  url: string;
  password: string;
  mirror: string;
  lastSync?: string;
  lastError?: string;
  lastErrorCode?: SourceErrorCode;
  lastAppCount?: number;
  lastInstallableCount?: number;
};

type SourceInput = Pick<SourceSubscription, 'name' | 'url' | 'password' | 'mirror'>;

type SourceApp = {
  id: number;
  sourceId?: SourceID;
  sourceName: string;
  name: string;
  slug: string;
  summary: string;
  category?: string;
  installProtected?: boolean;
  latestVersion?: {
    version: string;
    downloadUrl: string;
    upstreamDownloadUrl?: string;
    sourceType?: string;
    sha256: string;
    size: number;
  };
};

type FavoriteData = {
  apps: StoreApp[];
  submitters: User[];
};

type SetupStatus = {
  needsSetup: boolean;
};

type Toast = {
  tone: 'success' | 'error' | 'neutral';
  message: string;
};

type InstallActivity = {
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

type InstallPasswordRequest = {
  app: StoreApp | SourceApp;
};

type ClientSourceStats = {
  sourceCount: number;
  syncedSourceCount: number;
  staleSourceCount: number;
  authSourceCount: number;
  failedSourceCount: number;
  sourceAppCount: number;
  installableSourceAppCount: number;
};

const SOURCE_STALE_MS = 24 * 60 * 60 * 1000;

type SourceErrorCode = 'auth' | 'format' | 'http' | 'network';

type InstalledApplication = {
  appid?: string;
  title?: string;
  version?: string;
  status?: string;
  instanceStatus?: string;
  icon?: string;
};

type ClientInstallResult = {
  mode: string;
  taskId?: string;
  status?: string;
  detail?: string;
};

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

async function runAction<T>(setToast: (toast: Toast) => void, fallback: string, action: () => Promise<T>): Promise<T | undefined> {
  try {
    return await action();
  } catch (error) {
    setToast({ tone: 'error', message: errorMessage(error, fallback) });
    return undefined;
  }
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(' ');
}

function AvatarIcon({ seed, title, size = 46, className }: { seed: string; title?: string; size?: number; className?: string }) {
  return <Avatar assets={humation1} seed={seed || 'lazycat-app'} title={title} size={size} className={cx('humation-avatar', className)} />;
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  if (!HAS_API) {
    throw new Error(i18n.t('toast.apiMissing'));
  }
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data?.error?.message || `HTTP ${response.status}`);
  }
  return data as T;
}

const CLIENT_API_BASE = '/api/client/v1';

async function clientApi<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${CLIENT_API_BASE}${path}`, {
    credentials: 'include',
    headers: options.body instanceof FormData ? options.headers : { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data?.error?.message || `HTTP ${response.status}`);
  }
  return data as T;
}

function formatBytes(size?: number) {
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

function formatDate(value?: string) {
  if (!value) return '-';
  const locale = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en-US' : 'zh-CN';
  return new Intl.DateTimeFormat(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}

function hasInstallableVersion(app: StoreApp | SourceApp) {
  return Boolean(app.latestVersion?.downloadUrl);
}

function withInstallPassword(rawUrl: string, password?: string) {
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

function shortSHA(value?: string) {
  return value ? value.slice(0, 16) : '-';
}

function normalizeAppIdentity(value?: string) {
  return (value || '').trim().toLowerCase();
}

function findInstalledApplication(app: StoreApp | SourceApp, installedApps: InstalledApplication[]) {
  const appID = normalizeAppIdentity(app.slug);
  const appName = normalizeAppIdentity(app.name);
  return installedApps.find((item) => {
    const installedID = normalizeAppIdentity(item.appid);
    if (appID && installedID) return appID === installedID;
    return appName !== '' && normalizeAppIdentity(item.title) === appName;
  });
}

function belongsToSource(app: SourceApp, source: SourceSubscription) {
  return app.sourceId !== undefined ? String(app.sourceId) === String(source.id) : app.sourceName === source.name;
}

function isSourceStale(source: SourceSubscription) {
  if (!source.lastSync || source.lastError) return false;
  const syncedAt = Date.parse(source.lastSync);
  return Number.isFinite(syncedAt) && Date.now() - syncedAt > SOURCE_STALE_MS;
}

function reviewFieldLabel(key: string, t: (key: string, options?: any) => string) {
  const labels: Record<string, string> = {
    name: t('common.name'),
    summary: t('common.summary'),
    description: t('common.description'),
    categoryId: t('common.category'),
    tags: t('common.tags'),
    allowUnreviewedUpdates: t('submitApp.allowUnreviewedUpdates'),
    commentsEnabled: t('drawer.commentsEnabled'),
    installPassword: t('drawer.installPassword'),
  };
  return labels[key] || key;
}

type TabKey = 'home' | 'search' | 'sources' | 'profile' | 'admin';
type NavItem = { key: TabKey; labelKey: string; icon: typeof Home };
type ThemeMode = 'system' | 'light' | 'dark';
type ResolvedTheme = Exclude<ThemeMode, 'system'>;

const THEME_STORAGE_KEY = 'lazycat.theme';

const serverBaseTabs: NavItem[] = [
  { key: 'home', labelKey: 'nav.store', icon: Home },
  { key: 'search', labelKey: 'nav.discover', icon: Search },
  { key: 'profile', labelKey: 'nav.myApps', icon: PackagePlus },
];

const serverAdminTab: NavItem = { key: 'admin', labelKey: 'nav.admin', icon: ShieldCheck };

const clientTabs: NavItem[] = [
  { key: 'sources', labelKey: 'nav.sources', icon: Cloud },
  { key: 'search', labelKey: 'nav.install', icon: Download },
  { key: 'profile', labelKey: 'nav.installed', icon: Archive },
];

function readThemeMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    return saved === 'light' || saved === 'dark' || saved === 'system' ? saved : 'system';
  } catch {
    return 'system';
  }
}

function readSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined' || !window.matchMedia) return 'light';
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function nextThemeMode(mode: ThemeMode): ThemeMode {
  if (mode === 'system') return 'light';
  if (mode === 'light') return 'dark';
  return 'system';
}

function ThemeToggle({ mode, onChange }: { mode: ThemeMode; onChange: (mode: ThemeMode) => void }) {
  const { t } = useTranslation();
  const Icon = mode === 'system' ? Monitor : mode === 'dark' ? Moon : Sun;
  const label = t('theme.toggle', { mode: t(`theme.modes.${mode}`) });
  return (
    <button type="button" className="icon-button" aria-label={label} title={label} onClick={() => onChange(nextThemeMode(mode))}>
      <Icon size={18} />
    </button>
  );
}

type SortMode = 'recent' | 'downloads' | 'name';
type SourceAppFilter = 'all' | 'installable' | 'installed' | 'incomplete';
type SourceHealth = 'syncing' | 'auth' | 'failed' | 'stale' | 'synced' | 'unsynced';
type SourceHealthFilter = 'all' | Exclude<SourceHealth, 'syncing'>;

function verificationTokenFromURL() {
  return new URLSearchParams(window.location.search).get('token') || '';
}

function statusKey(value?: string) {
  return (value || 'UNKNOWN').toLowerCase().replaceAll('_', '');
}

function reviewKindKey(value?: string) {
  return (value || 'APP_SUBMISSION').toLowerCase().replaceAll('_', '');
}

export function App() {
  const { t } = useTranslation();
  const [themeMode, setThemeMode] = useState<ThemeMode>(readThemeMode);
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(readSystemTheme);
  const [tab, setTab] = useState<TabKey>(() => (verificationTokenFromURL() ? 'profile' : HAS_API ? 'home' : 'sources'));
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [sourceApps, setSourceApps] = useState<SourceApp[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [user, setUser] = useState<User | null>(null);
  const [query, setQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState<string>('all');
  const [activeSubmitter, setActiveSubmitter] = useState<string>('all');
  const [sortMode, setSortMode] = useState<SortMode>('recent');
  const [selectedApp, setSelectedApp] = useState<StoreApp | null>(null);
  const [selectedSourceApp, setSelectedSourceApp] = useState<SourceApp | null>(null);
  const [installedApps, setInstalledApps] = useState<InstalledApplication[]>([]);
  const [installedState, setInstalledState] = useState<'idle' | 'loading' | 'loaded' | 'error'>('idle');
  const [installedError, setInstalledError] = useState('');
  const [installActivity, setInstallActivity] = useState<InstallActivity | null>(null);
  const [installPasswordRequest, setInstallPasswordRequest] = useState<InstallPasswordRequest | null>(null);
  const [toast, setToast] = useState<Toast | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);
  const defaultSourceCheckedRef = useRef(false);
  const canReview = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const navItems = HAS_API ? [...serverBaseTabs, ...(canReview ? [serverAdminTab] : [])] : clientTabs;
  const modeLabel = HAS_API ? t('mode.serverStore') : t('mode.standaloneClient');
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';
  const drawerOpen = Boolean(selectedApp || selectedSourceApp);
  const resolvedTheme: ResolvedTheme = themeMode === 'system' ? systemTheme : themeMode;

  const [sources, setSources] = useState<SourceSubscription[]>([]);

  const sourceStats = useMemo<ClientSourceStats>(() => {
    return {
      sourceCount: sources.length,
      syncedSourceCount: sources.filter((source) => source.lastSync && !source.lastError && !isSourceStale(source)).length,
      staleSourceCount: sources.filter(isSourceStale).length,
      authSourceCount: sources.filter((source) => source.lastErrorCode === 'auth').length,
      failedSourceCount: sources.filter((source) => source.lastError && source.lastErrorCode !== 'auth').length,
      sourceAppCount: sourceApps.length,
      installableSourceAppCount: sourceApps.filter(hasInstallableVersion).length,
    };
  }, [sources, sourceApps]);

  useEffect(() => {
    if (!navItems.some((item) => item.key === tab)) {
      setTab(navItems[0].key);
    }
  }, [navItems, tab]);

  useEffect(() => {
    document.getElementById('main-content')?.focus({ preventScroll: true });
  }, [tab]);

  useEffect(() => {
    document.documentElement.lang = currentLanguage === 'en' ? 'en' : 'zh-CN';
    document.title = t('appName');
  }, [currentLanguage, t]);

  useEffect(() => {
    try {
      localStorage.setItem(THEME_STORAGE_KEY, themeMode);
    } catch {
      // Storage can be blocked by privacy settings; the active theme still applies for this session.
    }
  }, [themeMode]);

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: dark)');
    if (!media) return;
    const updateSystemTheme = () => setSystemTheme(media.matches ? 'dark' : 'light');
    updateSystemTheme();
    media.addEventListener('change', updateSystemTheme);
    return () => media.removeEventListener('change', updateSystemTheme);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = resolvedTheme;
    document.documentElement.dataset.themePreference = themeMode;
    document.documentElement.style.colorScheme = resolvedTheme;
  }, [resolvedTheme, themeMode]);

  useEffect(() => {
    void (HAS_API ? refreshAll() : refreshClientData());
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  async function refreshAll(options: { silent?: boolean } = {}) {
    if (!HAS_API) {
      await refreshClientData(options);
      return;
    }
    if (!options.silent) setLoading(true);
    try {
      const setup = await api<SetupStatus>('/api/v1/setup/status');
      setSetupRequired(setup.needsSetup);
      if (setup.needsSetup) {
        setApps([]);
        setCategories([]);
        setCollections([]);
        setGroups([]);
        setReviews([]);
        setUser(null);
        return;
      }
      const [me, appData, categoryData, collectionData] = await Promise.allSettled([
        api<{ user: User }>('/api/v1/auth/me'),
        api<{ apps: StoreApp[] }>('/api/v1/apps'),
        api<{ categories: Category[] }>('/api/v1/categories'),
        api<{ collections: Collection[] }>('/api/v1/collections'),
      ]);
      if (me.status === 'fulfilled') setUser(me.value.user);
      if (appData.status === 'fulfilled') setApps(appData.value.apps);
      if (categoryData.status === 'fulfilled') setCategories(categoryData.value.categories);
      if (collectionData.status === 'fulfilled') setCollections(collectionData.value.collections);
      if (me.status === 'fulfilled') {
        await loadGroups();
        if (me.value.user.role === 'SOFTWARE_ADMIN' || me.value.user.role === 'SITE_ADMIN') {
          await loadReviews();
        }
      }
    } finally {
      setLoading(false);
    }
  }

  async function loadClientSources() {
    const data = await clientApi<{ sources: SourceSubscription[] }>('/sources');
    if (!defaultSourceCheckedRef.current && data.sources.length === 0 && DEFAULT_SOURCE_URL) {
      defaultSourceCheckedRef.current = true;
      const created = await clientApi<{ source: SourceSubscription }>('/sources', {
        method: 'POST',
        body: JSON.stringify({ name: DEFAULT_SOURCE_NAME, url: DEFAULT_SOURCE_URL, password: '', mirror: '' }),
      });
      setSources([created.source]);
      return [created.source];
    }
    defaultSourceCheckedRef.current = true;
    setSources(data.sources);
    return data.sources;
  }

  async function loadClientApps() {
    const data = await clientApi<{ apps: SourceApp[] }>('/apps');
    setSourceApps(data.apps);
    return data.apps;
  }

  async function refreshClientData(options: { silent?: boolean } = {}) {
    if (!options.silent) setLoading(true);
    setApps([]);
    setCategories([]);
    setCollections([]);
    setGroups([]);
    setReviews([]);
    setUser(null);
    try {
      await Promise.all([loadClientSources(), loadClientApps()]);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.clientDataLoadFailed')) });
    } finally {
      setLoading(false);
    }
  }

  async function loadReviews() {
    await runAction(setToast, t('toast.loadReviewsFailed'), async () => {
      const data = await api<{ reviews: Review[] }>('/api/v1/admin/reviews?status=PENDING');
      setReviews(data.reviews);
    });
  }

  async function loadGroups() {
    await runAction(setToast, t('toast.loadGroupsFailed'), async () => {
      const data = await api<{ groups: Group[] }>('/api/v1/groups');
      setGroups(data.groups);
    });
  }

  const storeApps = useMemo(() => apps.filter((app) => app.status === 'APPROVED'), [apps]);

  const submitters = useMemo(() => {
    return Array.from(new Set(storeApps.map((app) => app.owner).filter(Boolean))).sort((a, b) => a.localeCompare(b));
  }, [storeApps]);

  const filteredApps = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const filtered = storeApps.filter((app) => {
      const categoryMatch = activeCategory === 'all' || app.category === activeCategory;
      const submitterMatch = activeSubmitter === 'all' || app.owner === activeSubmitter;
      const queryMatch =
        needle === '' ||
        app.name.toLowerCase().includes(needle) ||
        app.summary.toLowerCase().includes(needle) ||
        app.owner.toLowerCase().includes(needle) ||
        app.tags.join(' ').toLowerCase().includes(needle);
      return categoryMatch && submitterMatch && queryMatch;
    });
    return [...filtered].sort((a, b) => {
      if (sortMode === 'downloads') return b.downloadCount - a.downloadCount;
      if (sortMode === 'name') return a.name.localeCompare(b.name);
      return Date.parse(b.updatedAt) - Date.parse(a.updatedAt);
    });
  }, [storeApps, activeCategory, activeSubmitter, query, sortMode]);

  async function openApp(app: StoreApp) {
    await runAction(setToast, t('toast.loadAppDetailFailed'), async () => {
      const data = await api<{ app: StoreApp }>(`/api/v1/apps/${app.id}`);
      setSelectedApp(data.app);
    });
  }

  async function loadInstalledApps(options: { quiet?: boolean } = {}) {
    setInstalledState('loading');
    setInstalledError('');
    try {
      const result = await clientApi<{ apps: InstalledApplication[] }>('/installed');
      setInstalledApps(result.apps || []);
      setInstalledState('loaded');
      if (!options.quiet) setToast({ tone: 'success', message: t('profile.installedRefreshed') });
    } catch (error) {
      const message = errorMessage(error, t('profile.clientInstallApiUnavailable'));
      setInstalledState('error');
      setInstalledError(message);
      if (!options.quiet) setToast({ tone: 'error', message });
    }
  }

  async function installApp(app: StoreApp | SourceApp, options: { installPassword?: string } = {}) {
    if (app.installProtected && !options.installPassword) {
      setInstallPasswordRequest({ app });
      return;
    }
    await runAction(setToast, t('toast.installFailed'), async () => {
      const version = app.latestVersion;
      if (!version) {
        setToast({ tone: 'error', message: t('toast.noInstallableVersion') });
        return;
      }
      const source = 'sourceName' in app ? app.sourceName : t('search.localStore');
      const checksum = version.sha256 ? shortSHA(version.sha256) : t('app.checksumMissing');
      setInstallActivity({
        title: `${app.name} ${version.version}`,
        source,
        checksum,
        status: 'running',
        progress: 12,
        stageKey: 'installActivity.stagePrepare',
      });
      await new Promise((resolve) => window.setTimeout(resolve, 180));
      setInstallActivity((current) =>
        current && current.title === `${app.name} ${version.version}`
          ? { ...current, progress: 42, stageKey: version.sha256 ? 'installActivity.stageVerify' : 'installActivity.stageHandoff' }
          : current,
      );
      const result =
        'sourceName' in app
          ? await clientApi<ClientInstallResult>('/install', {
              method: 'POST',
              body: JSON.stringify({
                appId: app.id,
                installPassword: options.installPassword,
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
      const success = result.mode === 'lazycat-go-sdk' || result.mode === 'download';
      setInstallActivity({
        title: `${app.name} ${version.version}`,
        source,
        checksum,
        status: success ? 'success' : 'error',
        progress: 100,
        stageKey: success ? 'installActivity.stageDone' : 'installActivity.stageFailed',
        resultMode: result.mode,
        messageKey: result.messageKey,
        messageParams: result.messageParams,
      });
      if (result.mode === 'lazycat-go-sdk') void loadInstalledApps({ quiet: true });
      setToast({
        tone: success ? 'success' : 'error',
        message: t(result.messageKey, result.messageParams),
      });
    });
  }

  async function approveReview(review: Review, approve: boolean) {
    await runAction(setToast, t('toast.reviewActionFailed'), async () => {
      await api(`/api/v1/admin/reviews/${review.id}/${approve ? 'approve' : 'reject'}`, {
        method: 'POST',
        body: JSON.stringify({ note: approve ? 'Approved from client' : 'Rejected from client' }),
      });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? t('toast.reviewApproved') : t('toast.reviewRejected') });
      await refreshAll();
    });
  }

  async function syncSource(source: SourceSubscription, options: { quiet?: boolean } = {}) {
    try {
      await clientApi<{ source: SourceSubscription }>(`/sources/${source.id}/sync`, { method: 'POST' });
      await refreshClientData({ silent: true });
      if (!options.quiet) setToast({ tone: 'success', message: t('toast.sourceSynced') });
    } catch (error) {
      const message = errorMessage(error, t('toast.sourceSyncFailed'));
      await loadClientSources().catch(() => undefined);
      throw new Error(message);
    }
  }

  async function syncAllSources() {
    if (sources.length === 0) {
      setTab('sources');
      setToast({ tone: 'neutral', message: t('toast.addSourceFirst') });
      return;
    }
    let success = 0;
    let failed = 0;
    try {
      const data = await clientApi<{ result: { success: number; failed: number } }>('/sources/sync', { method: 'POST' });
      await refreshClientData({ silent: true });
      success = data.result.success;
      failed = data.result.failed;
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.sourceSyncFailed')) });
      return;
    }
    if (failed > 0) {
      setToast({ tone: success > 0 ? 'neutral' : 'error', message: t('toast.sourcesSyncPartial', { success, failed }) });
    } else {
      setToast({ tone: 'success', message: t('toast.allSourcesSynced', { count: sources.length }) });
    }
    if (success > 0) {
      setTab('search');
    }
  }

  async function addClientSource(input: SourceInput) {
    await clientApi<{ source: SourceSubscription }>('/sources', {
      method: 'POST',
      body: JSON.stringify(input),
    });
    await refreshClientData({ silent: true });
  }

  async function updateClientSource(source: SourceSubscription) {
    await clientApi<{ source: SourceSubscription }>(`/sources/${source.id}`, {
      method: 'PATCH',
      body: JSON.stringify({
        name: source.name,
        url: source.url,
        password: source.password,
        mirror: source.mirror,
      }),
    });
    await refreshClientData({ silent: true });
  }

  async function deleteClientSource(source: SourceSubscription) {
    await clientApi(`/sources/${source.id}`, { method: 'DELETE' });
    setSelectedSourceApp((current) => (current && belongsToSource(current, source) ? null : current));
    await refreshClientData({ silent: true });
  }

  if (HAS_API && setupRequired) {
    return (
      <>
        <SetupWizard
          onComplete={async (nextUser) => {
            setUser(nextUser);
            setSetupRequired(false);
            setTab('profile');
            await refreshAll();
          }}
          setToast={setToast}
          themeMode={themeMode}
          onThemeModeChange={setThemeMode}
        />
        {toast && <div className={cx('toast', toast.tone)}>{toast.message}</div>}
      </>
    );
  }

  return (
    <div className="shell">
      <a className="skip-link" href="#main-content" inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>{t('common.skipToMain')}</a>
      <aside className="sidebar" inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>
        <div className="brand">
          <div className="brand-mark">
            <Archive size={22} />
          </div>
          <div>
            <strong>{t('appName')}</strong>
            <span>{modeLabel}</span>
          </div>
        </div>
        <nav className="nav">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <button type="button" key={item.key} className={cx('nav-item', tab === item.key && 'active')} onClick={() => setTab(item.key)}>
                <Icon size={19} />
                <span>{t(item.labelKey)}</span>
              </button>
            );
          })}
        </nav>
        <div className="server-card">
          <Server size={18} />
          <div>
            <span>{HAS_API ? t('mode.serverApi') : t('mode.sourceClient')}</span>
            <strong>{HAS_API ? API_BASE.replace(/^https?:\/\//, '') : t('mode.notConfigured')}</strong>
          </div>
        </div>
      </aside>

      <main className="main" id="main-content" tabIndex={-1} inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>
        <header className="topbar">
          <div className="searchbox">
            <Search size={18} />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={HAS_API ? t('topbar.searchStore') : t('topbar.searchSources')}
              aria-label={HAS_API ? t('topbar.searchStore') : t('topbar.searchSources')}
            />
          </div>
          <div className="top-actions">
            <span className="mode-pill">{modeLabel}</span>
            <label className="language-select">
              <span>{t('language.label')}</span>
              <select aria-label={t('language.label')} value={currentLanguage} onChange={(event) => void i18n.changeLanguage(event.target.value)}>
                <option value="zh">{t('language.zh')}</option>
                <option value="en">{t('language.en')}</option>
              </select>
            </label>
            <ThemeToggle mode={themeMode} onChange={setThemeMode} />
            <button type="button" className="icon-button" aria-label={HAS_API ? t('topbar.refreshStore') : t('topbar.syncAllSources')} onClick={() => void (HAS_API ? refreshAll() : syncAllSources())}>
              <RefreshCw size={18} />
            </button>
            {HAS_API && user ? (
              <button
                type="button"
                className="user-pill"
                aria-label={user.username}
                onClick={() =>
                  void runAction(setToast, t('toast.logoutFailed'), async () => {
                    await api('/api/v1/auth/logout', { method: 'POST' });
                    setUser(null);
                  })
                }
              >
                <LogOut size={16} />
                <span>{user.username}</span>
              </button>
            ) : HAS_API ? (
              <button type="button" className="user-pill" aria-label={t('topbar.login')} onClick={() => setTab('profile')}>
                <LogIn size={16} />
                <span>{t('topbar.login')}</span>
              </button>
            ) : null}
          </div>
        </header>

        {loading ? (
          <div className="loading-state skeleton-state" aria-label={t('common.loading')} aria-live="polite">
            <div className="skeleton-list" aria-hidden="true">
              <span className="skeleton-line" />
              <span className="skeleton-line" />
              <span className="skeleton-line" />
            </div>
          </div>
        ) : (
          <>
            {tab === 'home' && (
              <HomeView
                apps={filteredApps}
                categories={categories}
                collections={collections}
                onOpen={openApp}
                onInstall={installApp}
                onNavigate={setTab}
                setToast={setToast}
              />
            )}
            {tab === 'search' && (
              <SearchView
                apps={filteredApps}
                sourceApps={sourceApps}
                categories={categories}
                submitters={submitters}
                activeCategory={activeCategory}
                activeSubmitter={activeSubmitter}
                sortMode={sortMode}
                query={query}
                mode={HAS_API ? 'server' : 'client'}
                sourceStats={sourceStats}
                installedApps={installedApps}
                onCategory={setActiveCategory}
                onSubmitter={setActiveSubmitter}
                onSortMode={setSortMode}
                onOpen={openApp}
                onOpenSource={setSelectedSourceApp}
                onInstall={installApp}
                onGoSources={() => setTab('sources')}
              />
            )}
            {tab === 'sources' && (
              <SourcesView
                sources={sources}
                setSources={setSources}
                sourceApps={sourceApps}
                onAddSource={addClientSource}
                onUpdateSource={updateClientSource}
                onDeleteSource={deleteClientSource}
                onSync={syncSource}
                onSyncAll={syncAllSources}
                onOpenSource={setSelectedSourceApp}
                onInstall={installApp}
                installedApps={installedApps}
                sourceStats={sourceStats}
                setToast={setToast}
              />
            )}
            {tab === 'profile' && (
              <ProfileView
                user={user}
                setUser={setUser}
                apps={apps}
                groups={groups}
                setGroups={setGroups}
                categories={categories}
                sourceStats={sourceStats}
                installedApps={installedApps}
                installedState={installedState}
                installedError={installedError}
                onLoadInstalled={loadInstalledApps}
                onOpen={openApp}
                refreshAll={refreshAll}
                setToast={setToast}
                hasAPI={HAS_API}
                onNavigate={setTab}
              />
            )}
            {tab === 'admin' && (
              user && canReview ? (
                <AdminPanel user={user} reviews={reviews} onApprove={approveReview} setToast={setToast} />
              ) : (
                <EmptyState
                  icon={ShieldCheck}
                  title={user ? t('admin.noPermission') : t('auth.loginRequired')}
                  body={user ? t('admin.noPermissionBody') : t('auth.loginRequiredBody')}
                  action={!user ? { label: t('auth.login'), icon: LogIn, onClick: () => setTab('profile') } : undefined}
                />
              )
            )}
          </>
        )}
      </main>

      <MobileTabs tab={tab} setTab={setTab} items={navItems} inert={drawerOpen} />

      {selectedApp && (
        <AppDrawer
          app={selectedApp}
          user={user}
          groups={groups}
          categories={categories}
          onClose={() => setSelectedApp(null)}
          onInstall={installApp}
          onRefresh={async () => {
            await openApp(selectedApp);
            await refreshAll();
          }}
          onListRefresh={refreshAll}
          setToast={setToast}
        />
      )}

      {selectedSourceApp && (
        <SourceAppDrawer
          app={selectedSourceApp}
          installedMatch={findInstalledApplication(selectedSourceApp, installedApps)}
          installedState={installedState}
          onClose={() => setSelectedSourceApp(null)}
          onInstall={installApp}
          onLoadInstalled={loadInstalledApps}
        />
      )}

      {installPasswordRequest && (
        <InstallPasswordDialog
          app={installPasswordRequest.app}
          onCancel={() => setInstallPasswordRequest(null)}
          onSubmit={(password) => {
            const target = installPasswordRequest.app;
            setInstallPasswordRequest(null);
            void installApp(target, { installPassword: password });
          }}
        />
      )}

      {installActivity && (
        <aside className={cx('install-panel', installActivity.status)} aria-live="polite" aria-label={t('installActivity.title')}>
          <Download size={20} />
          <div className="install-panel-body">
            <div className="install-panel-head">
              <strong>{installActivity.title}</strong>
              <span className={cx('status-badge', installActivity.status === 'running' ? 'syncing' : installActivity.status === 'success' ? 'approved' : 'failed')}>
                {t(`installActivity.status.${installActivity.status}`)}
              </span>
            </div>
            <span>{t(installActivity.stageKey)}</span>
            <div className="progress">
              <span style={{ width: `${installActivity.progress}%` }} />
            </div>
            <div className="install-panel-meta">
              <small>{t('installActivity.source', { source: installActivity.source })}</small>
              <small>{t('installActivity.checksum', { checksum: installActivity.checksum })}</small>
              {installActivity.resultMode && (
                <small>{t('installActivity.resultMode', { mode: t(`installActivity.modes.${installActivity.resultMode}`) })}</small>
              )}
            </div>
            {installActivity.messageKey && <p>{t(installActivity.messageKey, installActivity.messageParams)}</p>}
          </div>
          <button type="button" className="icon-button" aria-label={t('installActivity.dismiss')} onClick={() => setInstallActivity(null)}>
            <X size={17} />
          </button>
        </aside>
      )}

      {toast && <div className={cx('toast', toast.tone)}>{toast.message}</div>}
    </div>
  );
}

function InstallPasswordDialog({
  app,
  onCancel,
  onSubmit,
}: {
  app: StoreApp | SourceApp;
  onCancel: () => void;
  onSubmit: (password: string) => void;
}) {
  const { t } = useTranslation();
  const inputRef = useRef<HTMLInputElement>(null);
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const dialogTitleId = `install-password-title-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const dialogBodyId = `install-password-body-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onCancel();
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onCancel]);

  function submit(event: FormEvent) {
    event.preventDefault();
    const value = password.trim();
    if (!value) {
      setError(t('installPassword.required'));
      return;
    }
    onSubmit(value);
  }

  return (
    <div className="drawer-backdrop modal-backdrop" onClick={onCancel}>
      <form
        className="install-password-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={dialogTitleId}
        aria-describedby={dialogBodyId}
        onSubmit={submit}
        onClick={(event) => event.stopPropagation()}
      >
        <button type="button" className="icon-button close" aria-label={t('common.close')} onClick={onCancel}>
          <X size={17} />
        </button>
        <div className="install-password-head">
          <span className="install-password-icon">
            <KeyRound size={21} />
          </span>
          <div>
            <h2 id={dialogTitleId}>{t('installPassword.title')}</h2>
            <p id={dialogBodyId}>{t('installPassword.body', { name: app.name })}</p>
          </div>
        </div>
        <label>
          <span>{t('installPassword.label')}</span>
          <input
            ref={inputRef}
            type="password"
            autoComplete="off"
            value={password}
            onChange={(event) => {
              setPassword(event.target.value);
              if (error) setError('');
            }}
          />
        </label>
        {error && <p className="form-error">{error}</p>}
        <div className="dialog-actions">
          <button type="button" className="secondary-button" onClick={onCancel}>
            <X size={17} />
            <span>{t('common.cancel')}</span>
          </button>
          <button type="submit" className="primary-button">
            <Download size={17} />
            <span>{t('installPassword.confirm')}</span>
          </button>
        </div>
      </form>
    </div>
  );
}

function SetupWizard({
  onComplete,
  setToast,
  themeMode,
  onThemeModeChange,
}: {
  onComplete: (user: User) => Promise<void>;
  setToast: (toast: Toast) => void;
  themeMode: ThemeMode;
  onThemeModeChange: (mode: ThemeMode) => void;
}) {
  const { t } = useTranslation();
  const [form, setForm] = useState({
    username: 'admin',
    email: '',
    password: '',
    confirmPassword: '',
    sourcePasswordEnabled: true,
    sourcePassword: '',
    githubMirror: '',
    requireEmailVerify: false,
  });
  const [submitting, setSubmitting] = useState(false);
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';

  async function submitSetup(event: FormEvent) {
    event.preventDefault();
    if (form.password !== form.confirmPassword) {
      setToast({ tone: 'error', message: t('setup.passwordMismatch') });
      return;
    }
    if (form.sourcePasswordEnabled && !form.sourcePassword.trim()) {
      setToast({ tone: 'error', message: t('setup.sourcePasswordRequired') });
      return;
    }
    setSubmitting(true);
    await runAction(setToast, t('setup.failed'), async () => {
      const data = await api<{ user: User }>('/api/v1/setup', {
        method: 'POST',
        body: JSON.stringify({
          username: form.username,
          email: form.email,
          password: form.password,
          sourcePasswordEnabled: form.sourcePasswordEnabled,
          sourcePassword: form.sourcePassword,
          githubMirror: form.githubMirror,
          requireEmailVerify: form.requireEmailVerify,
        }),
      });
      setToast({ tone: 'success', message: t('setup.completed') });
      await onComplete(data.user);
    });
    setSubmitting(false);
  }

  return (
    <main className="setup-shell">
      <div className="setup-panel">
        <section className="setup-copy">
          <span className="eyebrow subtle">{t('setup.eyebrow')}</span>
          <h1>{t('setup.title')}</h1>
          <p>{t('setup.body')}</p>
          <div className="setup-steps" aria-label={t('setup.stepsLabel')}>
            <div><ShieldCheck size={18} /> {t('setup.stepAdmin')}</div>
            <div><Settings size={18} /> {t('setup.stepPolicy')}</div>
            <div><Cloud size={18} /> {t('setup.stepSource')}</div>
          </div>
        </section>
        <form className="panel form-panel setup-form" onSubmit={submitSetup}>
          <div className="form-topline">
            <SectionTitle icon={KeyRound} title={t('setup.formTitle')} />
            <label className="language-select">
              <span>{t('language.label')}</span>
              <select aria-label={t('language.label')} value={currentLanguage} onChange={(event) => void i18n.changeLanguage(event.target.value)}>
                <option value="zh">{t('language.zh')}</option>
                <option value="en">{t('language.en')}</option>
              </select>
            </label>
            <ThemeToggle mode={themeMode} onChange={onThemeModeChange} />
          </div>
          <label>
            <span>{t('common.username')}</span>
            <input autoComplete="username" value={form.username} onChange={(event) => setForm({ ...form, username: event.target.value })} />
          </label>
          <label>
            <span>{t('common.email')}</span>
            <input type="email" autoComplete="email" value={form.email} onChange={(event) => setForm({ ...form, email: event.target.value })} />
          </label>
          <label>
            <span>{t('common.password')}</span>
            <input type="password" autoComplete="new-password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} />
          </label>
          <label>
            <span>{t('setup.confirmPassword')}</span>
            <input type="password" autoComplete="new-password" value={form.confirmPassword} onChange={(event) => setForm({ ...form, confirmPassword: event.target.value })} />
          </label>
          <label className="toggle-line">
            <input
              type="checkbox"
              checked={form.sourcePasswordEnabled}
              onChange={(event) => setForm({ ...form, sourcePasswordEnabled: event.target.checked })}
            />
            <span>{t('setup.protectSource')}</span>
          </label>
          {form.sourcePasswordEnabled && (
            <label>
              <span>{t('sources.password')}</span>
              <input type="password" value={form.sourcePassword} onChange={(event) => setForm({ ...form, sourcePassword: event.target.value })} />
            </label>
          )}
          <label>
            <span>{t('sources.mirror')}</span>
            <input type="url" value={form.githubMirror} onChange={(event) => setForm({ ...form, githubMirror: event.target.value })} />
          </label>
          <label className="toggle-line">
            <input
              type="checkbox"
              checked={form.requireEmailVerify}
              onChange={(event) => setForm({ ...form, requireEmailVerify: event.target.checked })}
            />
            <span>{t('setup.requireEmailVerify')}</span>
          </label>
          <button type="submit" className="primary-button" disabled={submitting}>
            <ShieldCheck size={18} />
            <span>{submitting ? t('setup.submitting') : t('setup.finish')}</span>
          </button>
        </form>
      </div>
    </main>
  );
}

function HomeView({
  apps,
  categories,
  collections,
  onOpen,
  onInstall,
  onNavigate,
  setToast,
}: {
  apps: StoreApp[];
  categories: Category[];
  collections: Collection[];
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onNavigate: (tab: TabKey) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  const approvedCount = apps.filter((app) => app.status === 'APPROVED').length;
  const sourceFeedURL = `${API_BASE || window.location.origin}/source/v1/index.json`;

  async function copySourceFeed() {
    try {
      if (!navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(sourceFeedURL);
      setToast({ tone: 'success', message: t('home.sourceCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('home.copySourceFailed')) });
    }
  }

  return (
    <section className="page-grid">
      <div className="hero-band">
        <div>
          <span className="eyebrow">{t('home.eyebrow')}</span>
          <h1>{t('home.title')}</h1>
          <p>{t('home.body')}</p>
          <div className="hero-actions">
            <button type="button" className="primary-button" onClick={() => onNavigate('search')}>
              <Search size={18} />
              <span>{t('nav.discover')}</span>
            </button>
            <button type="button" className="secondary-button" onClick={() => onNavigate('profile')}>
              <PackagePlus size={18} />
              <span>{t('home.submitApp')}</span>
            </button>
          </div>
        </div>
        <div className="hero-stack">
          <div><PackagePlus size={18} /> {t('home.upload')}</div>
          <div><ShieldCheck size={18} /> {t('home.trusted')}</div>
          <div><Download size={18} /> {t('home.install')}</div>
        </div>
      </div>

      <section className="store-metrics" aria-label={t('nav.store')}>
        <div className="metric-card">
          <span>{t('common.apps')}</span>
          <strong>{approvedCount}</strong>
          <small>{t('home.approvedCount', { count: approvedCount })}</small>
        </div>
        <div className="metric-card">
          <span>{t('common.category')}</span>
          <strong>{categories.length}</strong>
          <small>{t('home.categoryCount', { count: categories.length })}</small>
        </div>
        <div className="metric-card source-feed-card">
          <span>{t('home.sourceUrl')}</span>
          <strong>{sourceFeedURL}</strong>
          <small>{t('home.openSourceFeed')}</small>
          <div className="source-feed-actions">
            <button type="button" className="secondary-button compact-button" onClick={() => void copySourceFeed()}>
              <Copy size={16} />
              <span>{t('home.copySourceFeed')}</span>
            </button>
            <button type="button" className="secondary-button compact-button" onClick={() => onNavigate('search')}>
              <Download size={16} />
              <span>{t('home.browseInstallable')}</span>
            </button>
          </div>
        </div>
      </section>

      <section className="panel">
        <SectionTitle icon={History} title={t('home.latest')} />
        <AppGrid
          apps={latest}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{
            title: t('home.emptyTitle'),
            body: t('home.emptyBody'),
            action: { label: t('home.emptyAction'), icon: PackagePlus, onClick: () => onNavigate('profile') },
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

function SearchView({
  apps,
  sourceApps,
  categories,
  submitters,
  activeCategory,
  activeSubmitter,
  sortMode,
  query,
  mode,
  sourceStats,
  installedApps,
  onCategory,
  onSubmitter,
  onSortMode,
  onOpen,
  onOpenSource,
  onInstall,
  onGoSources,
}: {
  apps: StoreApp[];
  sourceApps: SourceApp[];
  categories: Category[];
  submitters: string[];
  activeCategory: string;
  activeSubmitter: string;
  sortMode: SortMode;
  query: string;
  mode: 'server' | 'client';
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  onCategory: (category: string) => void;
  onSubmitter: (submitter: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp) => void;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: StoreApp | SourceApp) => void;
  onGoSources: () => void;
}) {
  const { t } = useTranslation();
  const [sourceAppFilter, setSourceAppFilter] = useState<SourceAppFilter>('all');
  const sourceNeedle = query.trim().toLowerCase();
  const searchableSourceApps = sourceApps.filter((app) => {
    if (!sourceNeedle) return true;
    return [app.name, app.summary, app.category, app.sourceName].filter(Boolean).join(' ').toLowerCase().includes(sourceNeedle);
  });
  const sourceAppFilterItems: Array<{ key: SourceAppFilter; label: string; count: number }> = [
    { key: 'all', label: t('search.sourceFilters.all'), count: searchableSourceApps.length },
    { key: 'installable', label: t('search.sourceFilters.installable'), count: searchableSourceApps.filter(hasInstallableVersion).length },
    { key: 'installed', label: t('search.sourceFilters.installed'), count: searchableSourceApps.filter((app) => Boolean(findInstalledApplication(app, installedApps))).length },
    {
      key: 'incomplete',
      label: t('search.sourceFilters.incomplete'),
      count: searchableSourceApps.filter((app) => !hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size).length,
    },
  ];
  const filteredSourceApps = searchableSourceApps.filter((app) => {
    if (sourceAppFilter === 'installable') return hasInstallableVersion(app);
    if (sourceAppFilter === 'installed') return Boolean(findInstalledApplication(app, installedApps));
    if (sourceAppFilter === 'incomplete') return !hasInstallableVersion(app) || !app.latestVersion?.sha256 || !app.latestVersion?.size;
    return true;
  });
  const sourceEmptyTitle = sourceApps.length === 0 ? t('search.noSyncedApps') : t('search.noResultsTitle');
  const sourceEmptyBody =
    sourceApps.length === 0
      ? t('search.noSyncedAppsBody')
      : sourceAppFilter === 'all'
        ? t('search.noResultsBody')
        : t('search.noFilterResultsBody');

  if (mode === 'client') {
    return (
      <section className="page-grid">
        <div className="page-heading with-action">
          <div>
            <span className="eyebrow subtle">{t('search.sourceCount', { count: sourceStats.sourceCount })}</span>
            <h1>{t('search.clientTitle')}</h1>
            <p>{t('search.clientDescription')}</p>
          </div>
          <button type="button" className="secondary-button" onClick={onGoSources}>
            <Cloud size={18} />
            <span>{t('search.noSyncedAppsAction')}</span>
          </button>
        </div>
        <div className="client-summary-grid" aria-label={t('search.installReadiness')}>
          <div>
            <span>{t('search.sourcesTotal')}</span>
            <strong>{sourceStats.sourceCount}</strong>
          </div>
          <div>
            <span>{t('search.syncedAppsTotal')}</span>
            <strong>{sourceStats.sourceAppCount}</strong>
          </div>
          <div>
            <span>{t('search.installableApps')}</span>
            <strong>{sourceStats.installableSourceAppCount}</strong>
          </div>
          <div className={cx(sourceStats.staleSourceCount > 0 && 'warning')}>
            <span>{t('search.staleSources')}</span>
            <strong>{sourceStats.staleSourceCount}</strong>
          </div>
          <div className={cx(sourceStats.authSourceCount > 0 && 'warning')}>
            <span>{t('search.authSources')}</span>
            <strong>{sourceStats.authSourceCount}</strong>
          </div>
          <div className={cx(sourceStats.failedSourceCount > 0 && 'warning')}>
            <span>{t('search.failedSources')}</span>
            <strong>{sourceStats.failedSourceCount}</strong>
          </div>
        </div>
        <section className="panel">
          <SectionTitle icon={Download} title={t('search.subscribedApps')} />
          <div className="segmented filter-segmented" aria-label={t('search.sourceAppFilter')}>
            {sourceAppFilterItems.map((item) => (
              <button
                type="button"
                key={item.key}
                className={cx(sourceAppFilter === item.key && 'active')}
                onClick={() => setSourceAppFilter(item.key)}
              >
                {item.label} {item.count}
              </button>
            ))}
          </div>
          <SourceAppGrid
            apps={filteredSourceApps}
            installedApps={installedApps}
            onOpen={onOpenSource}
            onInstall={onInstall}
            onGoSources={onGoSources}
            emptyTitle={sourceEmptyTitle}
            emptyBody={sourceEmptyBody}
          />
        </section>
      </section>
    );
  }

  return (
    <section className="page-grid">
      <div className="page-heading">
        <h1>{t('search.serverTitle')}</h1>
        <p>{t('search.serverDescription')}</p>
      </div>
      <section className="panel">
        <SectionTitle icon={Search} title={t('search.localStore')} />
        {categories.length > 0 && (
          <div className="segmented filter-segmented" aria-label={t('search.categoryFilter')}>
            <button type="button" className={cx(activeCategory === 'all' && 'active')} onClick={() => onCategory('all')}>{t('common.all')}</button>
            {categories.map((category) => (
              <button type="button" key={category.id} className={cx(activeCategory === category.name && 'active')} onClick={() => onCategory(category.name)}>
                {category.name}
              </button>
            ))}
          </div>
        )}
        <div className="filter-bar">
          <label>
            <span>{t('search.sort')}</span>
            <select value={sortMode} onChange={(event) => onSortMode(event.target.value as SortMode)}>
              <option value="recent">{t('search.recent')}</option>
              <option value="downloads">{t('search.downloads')}</option>
              <option value="name">{t('search.name')}</option>
            </select>
          </label>
          <label>
            <span>{t('search.submitter')}</span>
            <select value={activeSubmitter} onChange={(event) => onSubmitter(event.target.value)}>
              <option value="all">{t('search.allSubmitters')}</option>
              {submitters.map((submitter) => (
                <option key={submitter} value={submitter}>{submitter}</option>
              ))}
            </select>
          </label>
        </div>
        <AppGrid
          apps={apps}
          onOpen={onOpen}
          onInstall={onInstall}
          empty={{ title: t('search.noResultsTitle'), body: t('search.noResultsBody') }}
        />
      </section>
      <section className="panel">
        <SectionTitle icon={Cloud} title={t('search.subscribedApps')} />
        <SourceAppGrid apps={filteredSourceApps} installedApps={installedApps} onOpen={onOpenSource} onInstall={onInstall} onGoSources={onGoSources} />
      </section>
    </section>
  );
}

function SourceAppGrid({
  apps,
  installedApps,
  onOpen,
  onInstall,
  onGoSources,
  showEmptyAction = true,
  emptyTitle,
  emptyBody,
}: {
  apps: SourceApp[];
  installedApps: InstalledApplication[];
  onOpen: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void;
  onGoSources: () => void;
  showEmptyAction?: boolean;
  emptyTitle?: string;
  emptyBody?: string;
}) {
  const { t } = useTranslation();
  if (apps.length === 0) {
    return (
      <div className="empty-state action-empty">
        <Cloud size={28} />
        <strong>{emptyTitle || t('search.noSyncedApps')}</strong>
        {emptyBody && <p>{emptyBody}</p>}
        {showEmptyAction && (
          <button type="button" className="secondary-button" onClick={onGoSources}>
            <Plus size={18} />
            <span>{t('search.noSyncedAppsAction')}</span>
          </button>
        )}
      </div>
    );
  }
  return (
    <div className="source-app-grid">
      {apps.map((app) => {
        const installable = hasInstallableVersion(app);
        const hasChecksum = Boolean(app.latestVersion?.sha256);
        const hasSize = Boolean(app.latestVersion?.size && app.latestVersion.size > 0);
        const installedMatch = findInstalledApplication(app, installedApps);
        return (
          <article className="source-app-card" key={`${app.sourceId || app.sourceName}-${app.id}`}>
            <button type="button" className="app-open" onClick={() => onOpen(app)} aria-label={t('app.open', { name: app.name })}>
              <AvatarIcon seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} />
              <div>
                <h3>{app.name}</h3>
                <p>{app.summary || t('common.lpkApp')}</p>
              </div>
              <ChevronRight size={18} />
            </button>
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
              {app.latestVersion?.sourceType && <span><Link size={14} /> {t('app.sourceType', { type: app.latestVersion.sourceType })}</span>}
            </div>
            <div className="app-readiness" aria-label={t('app.installSignals')}>
              <span className={cx('status-badge', installable ? 'approved' : 'blocked')}>
                <Download size={13} />
                {installable ? t('app.installReady') : t('app.installMissingVersion')}
              </span>
              <span className={cx('status-badge', hasChecksum ? 'synced' : 'unsynced')}>
                <ShieldCheck size={13} />
                {hasChecksum ? t('app.checksumReady') : t('app.checksumMissing')}
              </span>
              {app.installProtected && (
                <span className="status-badge pending">
                  <KeyRound size={13} />
                  {t('app.installPasswordRequired')}
                </span>
              )}
              <span className={cx('status-badge', hasSize ? 'synced' : 'unsynced')}>
                <Archive size={13} />
                {hasSize ? t('app.sizeReady') : t('app.sizeMissing')}
              </span>
              {installedMatch && (
                <span className="status-badge synced">
                  <Check size={13} />
                  {t('app.installed')}
                </span>
              )}
            </div>
            <button
              type="button"
              className="install-button"
              disabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? t('app.install', { name: app.name }) : t('app.installUnavailable', { name: app.name })}
            >
              <Download size={17} />
              <span>{installable ? t('common.install') : t('common.unavailable')}</span>
            </button>
          </article>
        );
      })}
    </div>
  );
}

function SourcesView({
  sources,
  setSources,
  sourceApps,
  onAddSource,
  onUpdateSource,
  onDeleteSource,
  onSync,
  onSyncAll,
  onOpenSource,
  onInstall,
  installedApps,
  sourceStats,
  setToast,
}: {
  sources: SourceSubscription[];
  setSources: (update: SourceSubscription[] | ((current: SourceSubscription[]) => SourceSubscription[])) => void;
  sourceApps: SourceApp[];
  onAddSource: (input: SourceInput) => Promise<void>;
  onUpdateSource: (source: SourceSubscription) => Promise<void>;
  onDeleteSource: (source: SourceSubscription) => Promise<void>;
  onSync: (source: SourceSubscription) => Promise<void>;
  onSyncAll: () => Promise<void>;
  onOpenSource: (app: SourceApp) => void;
  onInstall: (app: SourceApp) => void;
  installedApps: InstalledApplication[];
  sourceStats: ClientSourceStats;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const emptyDraft = { name: '', url: DEFAULT_SOURCE_URL, password: '', mirror: '' };
  const [draft, setDraft] = useState(emptyDraft);
  const [syncingID, setSyncingID] = useState<SourceID | null>(null);
  const [confirmDeleteSource, setConfirmDeleteSource] = useState<SourceID | null>(null);
  const [sourceHealthFilter, setSourceHealthFilter] = useState<SourceHealthFilter>('all');

  function normalizedSourceURL(rawURL: string) {
    try {
      const parsed = new URL(rawURL.trim());
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
      return parsed.toString();
    } catch {
      return '';
    }
  }

  const normalizedDraftURL = normalizedSourceURL(draft.url);
  const sourceNameReady = Boolean(draft.name.trim());
  const sourceURLReady = Boolean(normalizedDraftURL);
  const sourcePasswordReady = Boolean(draft.password.trim());
  const sourceMirrorReady = Boolean(draft.mirror.trim());
  const canAddSource = sourceNameReady && sourceURLReady;

  async function addSource(event: FormEvent) {
    event.preventDefault();
    const name = draft.name.trim();
    const url = normalizedDraftURL;
    if (!name) {
      setToast({ tone: 'error', message: t('sources.nameRequired') });
      return;
    }
    if (!url) {
      setToast({ tone: 'error', message: t('sources.invalid') });
      return;
    }
    if (sources.some((source) => normalizedSourceURL(source.url) === url)) {
      setToast({ tone: 'neutral', message: t('sources.duplicate') });
      return;
    }
    try {
      await onAddSource({ name, url, password: draft.password, mirror: draft.mirror });
      setDraft(emptyDraft);
      setToast({ tone: 'success', message: t('sources.addedNext') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('sources.invalid')) });
    }
  }

  function updateSource(id: SourceID, patch: Partial<SourceSubscription>) {
    setSources((current) => current.map((source) => (source.id === id ? { ...source, ...patch } : source)));
  }

  async function saveSource(source: SourceSubscription) {
    try {
      await onUpdateSource(source);
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.sourceSaveFailed')) });
    }
  }

  function healthFor(source: SourceSubscription): SourceHealth {
    if (syncingID === source.id) return 'syncing';
    if (source.lastErrorCode === 'auth') return 'auth';
    if (source.lastError) return 'failed';
    if (isSourceStale(source)) return 'stale';
    if (source.lastSync) return 'synced';
    return 'unsynced';
  }

  const sourceHealthFilterItems: Array<{ key: SourceHealthFilter; label: string; count: number }> = [
    { key: 'all', label: t('sources.filters.all'), count: sources.length },
    { key: 'synced', label: t('sources.filters.synced'), count: sources.filter((source) => healthFor(source) === 'synced').length },
    { key: 'stale', label: t('sources.filters.stale'), count: sources.filter((source) => healthFor(source) === 'stale').length },
    { key: 'auth', label: t('sources.filters.auth'), count: sources.filter((source) => healthFor(source) === 'auth').length },
    { key: 'unsynced', label: t('sources.filters.unsynced'), count: sources.filter((source) => healthFor(source) === 'unsynced').length },
    { key: 'failed', label: t('sources.filters.failed'), count: sources.filter((source) => healthFor(source) === 'failed').length },
  ];
  const filteredSources = sources.filter((source) => sourceHealthFilter === 'all' || healthFor(source) === sourceHealthFilter);

  async function deleteSource(source: SourceSubscription) {
    if (confirmDeleteSource !== source.id) {
      setConfirmDeleteSource(source.id);
      setToast({ tone: 'neutral', message: t('sources.confirmDelete', { name: source.name }) });
      return;
    }
    try {
      await onDeleteSource(source);
      setConfirmDeleteSource(null);
      setToast({ tone: 'success', message: t('sources.deleted') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('toast.sourceSaveFailed')) });
    }
  }

  return (
    <section className="page-grid">
      <div className="page-heading with-action">
        <div>
          <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
          <h1>{t('sources.title')}</h1>
          <p>{t('sources.subtitle')}</p>
        </div>
        <button type="button" className="primary-button" onClick={() => void onSyncAll()}>
          <RefreshCw size={18} />
          <span>{t('sources.syncAll')}</span>
        </button>
      </div>

      <div className="client-summary-grid source-summary" aria-label={t('sources.summary')}>
        <div>
          <span>{t('search.sourcesTotal')}</span>
          <strong>{sourceStats.sourceCount}</strong>
        </div>
        <div>
          <span>{t('search.syncedSources')}</span>
          <strong>{sourceStats.syncedSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.staleSourceCount > 0 && 'warning')}>
          <span>{t('search.staleSources')}</span>
          <strong>{sourceStats.staleSourceCount}</strong>
        </div>
        <div className={cx(sourceStats.authSourceCount > 0 && 'warning')}>
          <span>{t('search.authSources')}</span>
          <strong>{sourceStats.authSourceCount}</strong>
        </div>
        <div>
          <span>{t('search.installableApps')}</span>
          <strong>{sourceStats.installableSourceAppCount}</strong>
        </div>
        <div className={cx(sourceStats.failedSourceCount > 0 && 'warning')}>
          <span>{t('search.failedSources')}</span>
          <strong>{sourceStats.failedSourceCount}</strong>
        </div>
      </div>

      <section className="split">
      <form className="panel form-panel" onSubmit={addSource} noValidate>
        <SectionTitle icon={Cloud} title={t('sources.addTitle')} />
        <div className="source-readiness" aria-label={t('sources.addReadiness')}>
          <div className={cx('readiness-step', sourceNameReady && 'ready')}>
            <span className={cx('status-badge', sourceNameReady ? 'approved' : 'unlisted')}>
              {sourceNameReady ? <Check size={14} /> : <AlertCircle size={14} />}
              {sourceNameReady ? t('sources.ready') : t('sources.needsValue')}
            </span>
            <strong>{t('sources.readinessName')}</strong>
            <small>{sourceNameReady ? t('sources.readinessNameReady') : t('sources.readinessNameMissing')}</small>
          </div>
          <div className={cx('readiness-step', sourceURLReady && 'ready')}>
            <span className={cx('status-badge', sourceURLReady ? 'approved' : 'unlisted')}>
              {sourceURLReady ? <Check size={14} /> : <AlertCircle size={14} />}
              {sourceURLReady ? t('sources.ready') : t('sources.needsValue')}
            </span>
            <strong>{t('sources.readinessUrl')}</strong>
            <small>{sourceURLReady ? t('sources.readinessUrlReady') : t('sources.readinessUrlMissing')}</small>
          </div>
          <div className={cx('readiness-step', sourcePasswordReady && 'ready')}>
            <span className={cx('status-badge', sourcePasswordReady ? 'synced' : 'unsynced')}>
              <KeyRound size={14} />
              {sourcePasswordReady ? t('sources.filled') : t('sources.optional')}
            </span>
            <strong>{t('sources.readinessPassword')}</strong>
            <small>{sourcePasswordReady ? t('sources.readinessPasswordReady') : t('sources.readinessPasswordOptional')}</small>
          </div>
          <div className={cx('readiness-step', sourceMirrorReady && 'ready')}>
            <span className={cx('status-badge', sourceMirrorReady ? 'synced' : 'unsynced')}>
              <Link size={14} />
              {sourceMirrorReady ? t('sources.filled') : t('sources.optional')}
            </span>
            <strong>{t('sources.readinessMirror')}</strong>
            <small>{sourceMirrorReady ? t('sources.readinessMirrorReady') : t('sources.readinessMirrorOptional')}</small>
          </div>
        </div>
        <label>
          <span>{t('common.name')}</span>
          <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        </label>
        <label>
          <span>{t('sources.url')}</span>
          <input type="url" value={draft.url} onChange={(event) => setDraft({ ...draft, url: event.target.value })} />
        </label>
        <label>
          <span>{t('sources.password')}</span>
          <input type="password" value={draft.password} onChange={(event) => setDraft({ ...draft, password: event.target.value })} />
        </label>
        <label>
          <span>{t('sources.mirror')}</span>
          <input value={draft.mirror} onChange={(event) => setDraft({ ...draft, mirror: event.target.value })} />
        </label>
        {!canAddSource && <p className="field-help">{t('sources.addBlocked')}</p>}
        <button type="submit" className="primary-button" disabled={!canAddSource}>
          <Cloud size={18} />
          <span>{t('sources.add')}</span>
        </button>
      </form>

      <section className="panel">
        <SectionTitle icon={Server} title={t('sources.subscriptions')} />
        <div className="segmented filter-segmented" aria-label={t('sources.statusFilter')}>
          {sourceHealthFilterItems.map((item) => (
            <button
              type="button"
              key={item.key}
              className={cx(sourceHealthFilter === item.key && 'active')}
              onClick={() => setSourceHealthFilter(item.key)}
            >
              {item.label} {item.count}
            </button>
          ))}
        </div>
        <div className="source-list">
          {sources.length === 0 ? (
            <EmptyState icon={Cloud} title={t('sources.empty')} />
          ) : filteredSources.length === 0 ? (
            <EmptyState icon={Cloud} title={t('sources.emptyFiltered')} body={t('sources.emptyFilteredBody')} />
          ) : (
            filteredSources.map((source) => {
              const sourceScopedApps = sourceApps.filter((app) => belongsToSource(app, source));
              const syncedAppCount = source.lastAppCount ?? sourceScopedApps.length;
              const installableAppCount = source.lastInstallableCount ?? sourceScopedApps.filter(hasInstallableVersion).length;
              const health = healthFor(source);
              const healthHint =
                health === 'auth'
                  ? t('sources.healthHints.auth')
                  : health === 'failed'
                  ? t('sources.healthHints.failed')
                  : health === 'stale'
                    ? t('sources.healthHints.stale')
                    : health === 'unsynced'
                      ? t('sources.healthHints.unsynced')
                      : health === 'syncing'
                        ? t('sources.healthHints.syncing')
                        : t('sources.healthHints.synced');
              return (
                <div className="source-row" key={source.id}>
                  <div>
                    <div className="source-row-header">
                      <strong>{source.name}</strong>
                      <span className={cx('status-badge', health)} aria-live="polite">{t(`sources.health.${health}`)}</span>
                    </div>
                    <span className="source-url" title={source.url}>{source.url}</span>
                    <div className="source-facts">
                      <small>{source.lastSync ? t(health === 'stale' ? 'sources.lastSyncStale' : 'sources.lastSync', { time: formatDate(source.lastSync) }) : t('sources.neverSynced')}</small>
                      <small>{t('sources.syncedAppCount', { count: syncedAppCount })}</small>
                      <small>{t('sources.installableAppCount', { count: installableAppCount })}</small>
                    </div>
                    {source.lastError && (
                      <p className={cx(health === 'auth' ? 'inline-warning' : 'inline-alert')}>
                        {health === 'auth' ? <KeyRound size={15} /> : <AlertCircle size={15} />}
                        <span>{source.lastError}</span>
                      </p>
                    )}
                    {(health === 'auth' || !source.lastError) && (
                      <p className={cx(health === 'synced' ? 'inline-success' : 'inline-warning')}>
                        {health === 'synced' ? <Check size={15} /> : health === 'auth' ? <KeyRound size={15} /> : <AlertCircle size={15} />}
                        <span>{healthHint}</span>
                      </p>
                    )}
                    <div className="source-edit-grid">
                      <input
                        aria-label={t('sources.passwordFor', { name: source.name })}
                        value={source.password}
                        type="password"
                        placeholder={t('sources.passwordPlaceholder')}
                        onChange={(event) => {
                          updateSource(source.id, {
                            password: event.target.value,
                            ...(source.lastErrorCode === 'auth' ? { lastError: undefined, lastErrorCode: undefined } : {}),
                          });
                        }}
                        onBlur={(event) =>
                          void saveSource({
                            ...source,
                            password: event.currentTarget.value,
                            ...(source.lastErrorCode === 'auth' ? { lastError: undefined, lastErrorCode: undefined } : {}),
                          })
                        }
                      />
                      <input
                        aria-label={t('sources.mirrorFor', { name: source.name })}
                        value={source.mirror}
                        placeholder={t('sources.mirrorPlaceholder')}
                        onChange={(event) => updateSource(source.id, { mirror: event.target.value })}
                        onBlur={(event) => void saveSource({ ...source, mirror: event.currentTarget.value })}
                      />
                    </div>
                  </div>
                  <div className="row-actions">
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={t('sources.syncSource', { name: source.name })}
                      disabled={syncingID === source.id}
                      onClick={() =>
                        void (async () => {
                          setSyncingID(source.id);
                          try {
                            await onSync(source);
                          } catch (error) {
                            setToast({ tone: 'error', message: error instanceof Error ? error.message : t('toast.sourceSyncFailed') });
                          } finally {
                            setSyncingID(null);
                          }
                        })()
                      }
                    >
                      <RefreshCw size={17} />
                    </button>
                    <button type="button" className="icon-button danger" aria-label={t('sources.deleteSource', { name: source.name })} onClick={() => deleteSource(source)}>
                      <X size={17} />
                    </button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </section>
      </section>

      <section className="panel">
        <SectionTitle icon={Download} title={t('sources.syncedApps')} />
        <SourceAppGrid apps={sourceApps} installedApps={installedApps} onOpen={onOpenSource} onInstall={onInstall} onGoSources={() => undefined} showEmptyAction={false} />
      </section>
    </section>
  );
}

function SourceAppDrawer({
  app,
  installedMatch,
  installedState,
  onClose,
  onInstall,
  onLoadInstalled,
}: {
  app: SourceApp;
  installedMatch?: InstalledApplication;
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  onClose: () => void;
  onInstall: (app: SourceApp) => void;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
}) {
  const { t } = useTranslation();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const drawerTitleId = `source-app-drawer-title-${app.sourceId || app.sourceName}-${app.id}`;
  const latestVersion = app.latestVersion;
  const installable = hasInstallableVersion(app);
  const hasChecksum = Boolean(latestVersion?.sha256);
  const hasSize = Boolean(latestVersion?.size && latestVersion.size > 0);
  const trustState: 'ready' | 'caution' | 'blocked' = !installable ? 'blocked' : hasChecksum && hasSize ? 'ready' : 'caution';
  const TrustIcon = trustState === 'ready' ? ShieldCheck : trustState === 'caution' ? Gauge : AlertCircle;
  const trustTitle = trustState === 'ready' ? t('sourceDetail.trustReadyTitle') : trustState === 'caution' ? t('sourceDetail.trustCautionTitle') : t('sourceDetail.trustBlockedTitle');
  const trustBody = trustState === 'ready' ? t('sourceDetail.trustReadyBody') : trustState === 'caution' ? t('sourceDetail.trustCautionBody') : t('sourceDetail.trustBlockedBody');
  const installedBody = installedMatch
    ? t('sourceDetail.installedBody', { version: installedMatch.version || '-' })
    : installedState === 'loaded'
      ? t('sourceDetail.notInstalledLoaded')
      : t('sourceDetail.notInstalledIdle');
  const trustFacts = [
    { label: t('drawer.installStatus'), value: installable ? t('app.installReady') : t('app.installMissingVersion') },
    { label: t('drawer.installAccess'), value: app.installProtected ? t('app.installPasswordRequired') : t('app.installOpen') },
    { label: t('sourceDetail.source'), value: app.sourceName },
    { label: t('drawer.artifactChecksum'), value: hasChecksum ? t('drawer.checksumShort', { hash: shortSHA(latestVersion?.sha256) }) : t('drawer.checksumMissing') },
    { label: t('drawer.artifactSize'), value: hasSize ? formatBytes(latestVersion?.size) : t('drawer.sizeMissing') },
  ];

  useEffect(() => {
    closeButtonRef.current?.focus();
  }, [app.id]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <article
        className="drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby={drawerTitleId}
        onClick={(event) => event.stopPropagation()}
      >
        <button ref={closeButtonRef} type="button" className="icon-button close" aria-label={t('common.close')} onClick={onClose}>
          <X size={17} />
        </button>
        <header className="detail-head">
          <AvatarIcon seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} className="detail-avatar" />
          <div>
            <span className="eyebrow subtle">{t('sourceDetail.eyebrow')}</span>
            <h2 id={drawerTitleId}>{app.name}</h2>
            <p>{app.summary || t('common.lpkApp')}</p>
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
              <span><Star size={14} /> {latestVersion?.version || t('app.noPublishedVersion')}</span>
              {installedMatch && (
                <span className="status-badge synced">
                  <Check size={13} />
                  {t('app.installed')}
                </span>
              )}
            </div>
          </div>
        </header>

        <section className={cx('install-trust', trustState)} aria-label={t('drawer.installReadiness')}>
          <div className="install-trust-lead">
            <TrustIcon size={22} />
            <div>
              <strong>{trustTitle}</strong>
              <span>{trustBody}</span>
              {!installable && <small>{t('sourceDetail.installBlockedHint')}</small>}
            </div>
          </div>
          <div className="trust-facts" role="list">
            {trustFacts.map((fact) => (
              <div role="listitem" key={fact.label}>
                <span>{fact.label}</span>
                <strong>{fact.value}</strong>
              </div>
            ))}
          </div>
          <div className="source-detail-actions">
            <button type="button" className="install-button" disabled={!installable} onClick={() => void onInstall(app)}>
              <Download size={17} />
              <span>{installable ? t('common.install') : t('common.unavailable')}</span>
            </button>
            <button type="button" className="secondary-button" disabled={installedState === 'loading'} onClick={() => void onLoadInstalled()}>
              <RefreshCw size={17} />
              <span>{installedState === 'loading' ? t('profile.readingInstalled') : t('profile.readInstalled')}</span>
            </button>
          </div>
        </section>

        <section className={cx('install-trust', installedMatch ? 'ready' : 'caution')} aria-label={t('sourceDetail.installedTitle')}>
          <div className="install-trust-lead">
            {installedMatch ? <Check size={22} /> : <Gauge size={22} />}
            <div>
              <strong>{installedMatch ? t('sourceDetail.installedTitle') : t('sourceDetail.notInstalledTitle')}</strong>
              <span>{installedBody}</span>
              {installedMatch?.appid && <small>{installedMatch.appid}</small>}
            </div>
          </div>
        </section>

        <section className="detail-summary" aria-label={t('drawer.metadata')}>
          <div>
            <span>{t('sourceDetail.source')}</span>
            <strong>{app.sourceName}</strong>
          </div>
          <div>
            <span>{t('common.category')}</span>
            <strong>{app.category || t('common.uncategorized')}</strong>
          </div>
          <div>
            <span>{t('drawer.latestVersion')}</span>
            <strong>{latestVersion?.version || t('app.noPublishedVersion')}</strong>
          </div>
          <div>
            <span>{t('drawer.artifactSource')}</span>
            <strong>{latestVersion?.sourceType || t('drawer.sourceMissing')}</strong>
          </div>
        </section>

        <section className="source-detail-urls">
          <h3>{t('sourceDetail.downloadDetails')}</h3>
          <div className="detail-url-row">
            <span>{t('common.downloadUrl')}</span>
            <code>{latestVersion?.downloadUrl || '-'}</code>
          </div>
          {latestVersion?.upstreamDownloadUrl && (
            <div className="detail-url-row">
              <span>{t('sourceDetail.upstreamUrl')}</span>
              <code>{latestVersion.upstreamDownloadUrl}</code>
            </div>
          )}
          <div className="detail-url-row">
            <span>{t('common.sha256')}</span>
            <code>{latestVersion?.sha256 || '-'}</code>
          </div>
        </section>
      </article>
    </div>
  );
}

function ProfileView({
  user,
  setUser,
  apps,
  groups,
  setGroups,
  categories,
  sourceStats,
  installedApps,
  installedState,
  installedError,
  onLoadInstalled,
  onOpen,
  refreshAll,
  setToast,
  hasAPI,
  onNavigate,
}: {
  user: User | null;
  setUser: (user: User | null) => void;
  apps: StoreApp[];
  groups: Group[];
  setGroups: (groups: Group[]) => void;
  categories: Category[];
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onOpen: (app: StoreApp) => void;
  refreshAll: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
  hasAPI: boolean;
  onNavigate: (tab: TabKey) => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'login' | 'register' | 'verify'>('login');
  const [authForm, setAuthForm] = useState({ username: '', password: '', email: '' });
  const [verifyToken, setVerifyToken] = useState(verificationTokenFromURL);
  const [uploadForm, setUploadForm] = useState({
    name: '',
    version: '0.1.0',
    summary: '',
    description: '',
    categoryId: '',
    tags: '',
    allowUnreviewedUpdates: false,
    sourceType: 'GITHUB',
    downloadUrl: '',
    sha256: '',
    installPassword: '',
  });
  const [recentSubmission, setRecentSubmission] = useState<{ name: string; status: string } | null>(null);
  const [artifactMode, setArtifactMode] = useState<'local' | 'external'>('local');
  const [file, setFile] = useState<File | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [newToken, setNewToken] = useState('');
  const [favorites, setFavorites] = useState<FavoriteData>({ apps: [], submitters: [] });
  const authModeLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verify');
  const authSubmitLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verifyEmail');
  const authHint = mode === 'login' ? t('auth.loginHint') : mode === 'register' ? t('auth.registerHint') : t('auth.verifyHint');
  const AuthSubmitIcon = mode === 'verify' ? Check : mode === 'register' ? Plus : LogIn;
  const ownedApps = useMemo(() => {
    if (!user) return [];
    return apps
      .filter((app) => app.ownerId === user.id)
      .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  }, [apps, user]);
  const ownedStatusSummary = useMemo(() => {
    const order = ['APPROVED', 'PENDING', 'REJECTED', 'UNLISTED'];
    return order
      .map((status) => ({ status, count: ownedApps.filter((app) => app.status === status).length }))
      .filter((item) => item.count > 0);
  }, [ownedApps]);
  const publishSummary = useMemo(() => {
    return {
      total: ownedApps.length,
      approved: ownedApps.filter((app) => app.status === 'APPROVED').length,
      pending: ownedApps.filter((app) => app.status === 'PENDING').length,
      needsVersion: ownedApps.filter((app) => app.status === 'APPROVED' && !hasInstallableVersion(app)).length,
    };
  }, [ownedApps]);
  const appInfoReady = Boolean(uploadForm.name.trim());
  const appInfoDetailed = Boolean(uploadForm.summary.trim() && uploadForm.description.trim());
  const appInfoComplete = appInfoReady && appInfoDetailed;
  const externalDownloadReady = Boolean(uploadForm.downloadUrl.trim());
  const externalChecksumReady = Boolean(uploadForm.sha256.trim());
  const externalArtifactReady = externalDownloadReady && externalChecksumReady;
  const artifactReady = artifactMode === 'local' ? Boolean(file) : externalArtifactReady;
  const canSubmitUpload = appInfoReady && artifactReady;
  const isDirectPublishUser = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const sourceCacheReady = sourceStats.syncedSourceCount > 0;
  const installCatalogReady = sourceStats.installableSourceAppCount > 0;
  const installedLookupReady = installedState === 'loaded';
  const sourceCacheBody =
    sourceStats.sourceCount === 0
      ? t('profile.clientSourceMissing')
      : sourceStats.syncedSourceCount === 0 && sourceStats.authSourceCount > 0
        ? t('profile.clientSourceNeedsPassword', { count: sourceStats.authSourceCount })
      : sourceStats.syncedSourceCount === 0 && sourceStats.staleSourceCount > 0
        ? t('profile.clientSourceStale', { count: sourceStats.staleSourceCount })
      : sourceStats.syncedSourceCount === 0
        ? t('profile.clientSourceNeedsSync', { count: sourceStats.sourceCount })
        : sourceStats.authSourceCount > 0
          ? t('profile.clientSourcePartlyNeedsPassword', {
              synced: sourceStats.syncedSourceCount,
              auth: sourceStats.authSourceCount,
              total: sourceStats.sourceCount,
            })
        : sourceStats.staleSourceCount > 0
          ? t('profile.clientSourcePartlyStale', {
              synced: sourceStats.syncedSourceCount,
              stale: sourceStats.staleSourceCount,
              total: sourceStats.sourceCount,
            })
        : t('profile.clientSourceReady', { synced: sourceStats.syncedSourceCount, total: sourceStats.sourceCount });
  const installCatalogBody =
    sourceStats.installableSourceAppCount > 0
      ? t('profile.clientInstallReady', { count: sourceStats.installableSourceAppCount })
      : sourceStats.sourceAppCount > 0
        ? t('profile.clientInstallMissingInstallable', { count: sourceStats.sourceAppCount })
        : t('profile.clientInstallMissing');
  const installedReadinessBody =
    installedState === 'loaded'
      ? t('profile.clientInstalledLoaded', { count: installedApps.length })
      : installedState === 'loading'
        ? t('profile.clientInstalledLoading')
        : installedState === 'error'
          ? installedError || t('profile.clientInstalledError')
          : t('profile.clientInstalledIdle');

  useEffect(() => {
    if (!user) return;
    void api<{ tokens: APITokenRecord[] }>('/api/v1/me/tokens').then((data) => setTokens(data.tokens)).catch(() => setTokens([]));
    void loadFavorites();
  }, [user]);

  useEffect(() => {
    if (verifyToken) setMode('verify');
  }, [verifyToken]);

  useEffect(() => {
    if (user?.emailVerified === false) setMode('verify');
  }, [user]);

  async function submitVerification(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('auth.verifyFailed'), async () => {
      const data = await api<{ user: User }>('/api/v1/auth/verify-email', {
        method: 'POST',
        body: JSON.stringify({ token: verifyToken }),
      });
      setUser(data.user);
      setToast({ tone: 'success', message: t('auth.emailVerified') });
      await refreshAll();
    });
  }

  async function submitAuth(event: FormEvent) {
    event.preventDefault();
    if (mode === 'verify') {
      await submitVerification(event);
      return;
    }
    await runAction(setToast, mode === 'login' ? t('auth.loginFailed') : t('auth.registerFailed'), async () => {
      const body =
        mode === 'login'
          ? { username: authForm.username, password: authForm.password }
          : authForm;
      const data = await api<{ user: User }>(`/api/v1/auth/${mode}`, {
        method: 'POST',
        body: JSON.stringify(body),
      });
      setUser(data.user);
      if (data.user.emailVerified === false) {
        setMode('verify');
        setToast({ tone: 'neutral', message: t('auth.completeEmailVerification') });
      } else {
        setToast({ tone: 'success', message: mode === 'login' ? t('auth.loggedIn') : t('auth.registered') });
      }
      await refreshAll();
    });
  }

  function selectArtifactMode(nextMode: 'local' | 'external') {
    setArtifactMode(nextMode);
    if (nextMode === 'external') {
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }

  async function submitUpload(event: FormEvent) {
    event.preventDefault();
    if (artifactMode === 'local' && !file) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (artifactMode === 'external' && !uploadForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (artifactMode === 'external' && !uploadForm.sha256.trim()) {
      setToast({ tone: 'error', message: t('submitApp.sha256Required') });
      return;
    }
    await runAction(setToast, t('submitApp.failed'), async () => {
      let created: { app?: StoreApp };
      if (artifactMode === 'local' && file) {
        const form = new FormData();
        Object.entries(uploadForm).forEach(([key, value]) => form.set(key, String(value)));
        form.set('file', file);
        created = await api<{ app: StoreApp }>('/api/v1/apps', { method: 'POST', body: form });
      } else {
        created = await api<{ app: StoreApp }>('/api/v1/apps', {
          method: 'POST',
          body: JSON.stringify({
            name: uploadForm.name,
            version: uploadForm.version,
            summary: uploadForm.summary,
            description: uploadForm.description,
            categoryId: uploadForm.categoryId ? Number(uploadForm.categoryId) : undefined,
            tags: uploadForm.tags.split(',').map((tag) => tag.trim()).filter(Boolean),
            allowUnreviewedUpdates: uploadForm.allowUnreviewedUpdates,
            sourceType: uploadForm.sourceType,
            downloadUrl: uploadForm.downloadUrl.trim(),
            sha256: uploadForm.sha256.trim(),
            ...(uploadForm.installPassword.trim() ? { installPassword: uploadForm.installPassword.trim() } : {}),
          }),
        });
      }
      setRecentSubmission({ name: created.app?.name || uploadForm.name, status: created.app?.status || 'PENDING' });
      setToast({ tone: 'success', message: t('submitApp.submitted') });
      setUploadForm({ name: '', version: '0.1.0', summary: '', description: '', categoryId: '', tags: '', allowUnreviewedUpdates: false, sourceType: 'GITHUB', downloadUrl: '', sha256: '', installPassword: '' });
      setArtifactMode('local');
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      await refreshAll({ silent: true });
    });
  }

  async function createToken() {
    await runAction(setToast, t('token.createFailed'), async () => {
      const data = await api<{ token: string; record: APITokenRecord }>('/api/v1/me/tokens', {
        method: 'POST',
        body: JSON.stringify({ name: 'CI publish token' }),
      });
      setTokens((current) => [data.record, ...current]);
      setNewToken(data.token);
    });
  }

  async function loadFavorites() {
    await runAction(setToast, t('favorites.loadFailed'), async () => {
      const data = await api<FavoriteData>('/api/v1/me/favorites');
      setFavorites({ apps: data.apps || [], submitters: data.submitters || [] });
    });
  }

  function submissionStep(app: StoreApp) {
    if (app.status === 'PENDING') return { key: 'pending', tone: 'pending' };
    if (app.status === 'REJECTED') return { key: 'rejected', tone: 'rejected' };
    if (app.status === 'APPROVED' && !hasInstallableVersion(app)) return { key: 'needsVersion', tone: 'unlisted' };
    if (app.status === 'APPROVED') return { key: 'listed', tone: 'approved' };
    return { key: 'draft', tone: 'draft' };
  }

  if (!hasAPI) {
    const installedEmptyTitle = installedState === 'loaded' ? t('profile.installedEmptyLoaded') : t('profile.installedEmpty');
    const installedEmptyBody = installedState === 'idle' ? t('profile.installedIdleBody') : undefined;
    return (
      <section className="page-grid">
        <div className="page-heading with-action">
          <div>
            <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
            <h1>{t('profile.clientTitle')}</h1>
            <p>{t('profile.clientBody')}</p>
          </div>
          <div className="row-actions">
            <button type="button" className="primary-button" onClick={() => onNavigate('sources')}>
              <Cloud size={18} />
              <span>{t('profile.openSources')}</span>
            </button>
            <button type="button" className="secondary-button" onClick={() => onNavigate('search')}>
              <Search size={18} />
              <span>{t('profile.browseInstallable')}</span>
            </button>
          </div>
        </div>
        <section className="panel">
          <SectionTitle icon={Gauge} title={t('profile.clientReadiness')} />
          <div className="source-readiness" aria-label={t('profile.clientReadiness')}>
            <div className={cx('readiness-step', sourceCacheReady && 'ready')}>
              <span className={cx('status-badge', sourceCacheReady ? 'approved' : 'unlisted')}>
                {sourceCacheReady ? <Check size={14} /> : <AlertCircle size={14} />}
                {sourceCacheReady ? t('sources.ready') : t('sources.needsValue')}
              </span>
              <strong>{t('profile.clientSourceTitle')}</strong>
              <small>{sourceCacheBody}</small>
            </div>
            <div className={cx('readiness-step', installCatalogReady && 'ready')}>
              <span className={cx('status-badge', installCatalogReady ? 'approved' : 'unlisted')}>
                {installCatalogReady ? <Check size={14} /> : <AlertCircle size={14} />}
                {installCatalogReady ? t('sources.ready') : t('sources.needsValue')}
              </span>
              <strong>{t('profile.clientInstallTitle')}</strong>
              <small>{installCatalogBody}</small>
            </div>
            <div className={cx('readiness-step', installedLookupReady && 'ready')}>
              <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loading' ? 'pending' : installedLookupReady ? 'synced' : 'unsynced')}>
                {installedState === 'error' ? <AlertCircle size={14} /> : installedLookupReady ? <Check size={14} /> : <Gauge size={14} />}
                {t(`profile.installedState.${installedState}`)}
              </span>
              <strong>{t('profile.clientInstalledTitle')}</strong>
              <small>{installedReadinessBody}</small>
            </div>
          </div>
        </section>
        <div className="split">
          <div className="panel profile-card">
            <AvatarIcon seed="lazycat-standalone-client" title={t('profile.clientTitle')} size={74} className="avatar-large" />
            <h2>{t('profile.clientInstalledTitle')}</h2>
            <p>{t('profile.clientInstalledHelp')}</p>
            <div className={cx('installed-state', installedState)}>
              <span className={cx('status-badge', installedState === 'error' ? 'failed' : installedState === 'loaded' ? 'synced' : 'unsynced')}>
                {t(`profile.installedState.${installedState}`)}
              </span>
              {installedState === 'error' && <small>{installedError}</small>}
            </div>
            <button type="button" className="primary-button" disabled={installedState === 'loading'} onClick={() => void onLoadInstalled()}>
              <RefreshCw size={18} />
              <span>{installedState === 'loading' ? t('profile.readingInstalled') : t('profile.readInstalled')}</span>
            </button>
          </div>
          <section className="panel">
            <SectionTitle icon={Download} title={t('profile.installed')} />
            {installedState === 'error' && (
              <p className="inline-alert">
                <AlertCircle size={15} />
                <span>{installedError}</span>
              </p>
            )}
            <div className="review-list">
              {installedApps.length === 0 ? (
                <EmptyState icon={Download} title={installedEmptyTitle} body={installedEmptyBody} />
              ) : (
                installedApps.map((item) => (
                  <div className="review-row" key={item.appid || item.title}>
                    <div>
                      <strong>{item.title || item.appid}</strong>
                      <span>{item.version || '-'} · {t('profile.status', { status: item.status ?? '-' })}</span>
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>
        </div>
      </section>
    );
  }

  if (!user) {
    return (
      <section className="page-grid auth-gateway">
        <div className="page-heading">
          <span className="eyebrow subtle">{t('auth.entryEyebrow')}</span>
          <h1>{t('auth.entryTitle')}</h1>
          <p>{t('auth.entryBody')}</p>
        </div>
        <div className="split auth-split">
          <form className="panel form-panel profile-panel auth-panel" onSubmit={submitAuth}>
            <SectionTitle icon={KeyRound} title={mode === 'verify' ? t('auth.verifyEmail') : authModeLabel} />
            <p className="inline-note">{authHint}</p>
            <div className="segmented compact" aria-label={t('auth.modeSwitch')}>
              <button type="button" className={cx(mode === 'login' && 'active')} onClick={() => setMode('login')}>{t('auth.login')}</button>
              <button type="button" className={cx(mode === 'register' && 'active')} onClick={() => setMode('register')}>{t('auth.register')}</button>
              <button type="button" className={cx(mode === 'verify' && 'active')} onClick={() => setMode('verify')}>{t('auth.verify')}</button>
            </div>
            {mode === 'verify' ? (
              <label>
                <span>{t('auth.verifyToken')}</span>
                <input autoComplete="one-time-code" required value={verifyToken} onChange={(event) => setVerifyToken(event.target.value)} />
              </label>
            ) : (
              <>
                <label>
                  <span>{t('common.username')}</span>
                  <input autoComplete="username" required value={authForm.username} onChange={(event) => setAuthForm({ ...authForm, username: event.target.value })} />
                </label>
                {mode === 'register' && (
                  <label>
                    <span>{t('common.email')}</span>
                    <input type="email" autoComplete="email" value={authForm.email} onChange={(event) => setAuthForm({ ...authForm, email: event.target.value })} />
                  </label>
                )}
                <label>
                  <span>{t('common.password')}</span>
                  <input
                    type="password"
                    autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                    required
                    minLength={mode === 'register' ? 8 : undefined}
                    value={authForm.password}
                    onChange={(event) => setAuthForm({ ...authForm, password: event.target.value })}
                  />
                  {mode === 'register' && <small className="field-help">{t('auth.passwordHelp')}</small>}
                </label>
              </>
            )}
            <button type="submit" className="primary-button" aria-label={authSubmitLabel}>
              <AuthSubmitIcon size={18} />
              <span>{authSubmitLabel}</span>
            </button>
          </form>

          <section className="panel auth-path-panel">
            <SectionTitle icon={Users} title={t('auth.entryPaths')} />
            <div className="auth-path-list">
              <div className="auth-path-row">
                <Search size={19} />
                <div>
                  <strong>{t('auth.pathBrowseTitle')}</strong>
                  <span>{t('auth.pathBrowseBody')}</span>
                </div>
                <button type="button" className="secondary-button compact-button" onClick={() => onNavigate('home')}>
                  <Home size={17} />
                  <span>{t('auth.pathBrowseAction')}</span>
                </button>
              </div>
              <div className="auth-path-row">
                <PackagePlus size={19} />
                <div>
                  <strong>{t('auth.pathSubmitTitle')}</strong>
                  <span>{t('auth.pathSubmitBody')}</span>
                </div>
                <button type="button" className="secondary-button compact-button" onClick={() => setMode('register')}>
                  <Plus size={17} />
                  <span>{t('auth.pathSubmitAction')}</span>
                </button>
              </div>
              <div className="auth-path-row">
                <ShieldCheck size={19} />
                <div>
                  <strong>{t('auth.pathAdminTitle')}</strong>
                  <span>{t('auth.pathAdminBody')}</span>
                </div>
                <button type="button" className="secondary-button compact-button" onClick={() => setMode('login')}>
                  <LogIn size={17} />
                  <span>{t('auth.pathAdminAction')}</span>
                </button>
              </div>
            </div>
          </section>
        </div>
      </section>
    );
  }

  if (user.emailVerified === false) {
    return (
      <section className="page-grid">
        <div className="split">
          <div className="panel profile-card">
            <AvatarIcon seed={user.email || user.username} title={user.username} size={74} className="avatar-large" />
            <h2>{user.username}</h2>
            <p>{t('auth.emailPending')}</p>
            <button
              type="button"
              className="secondary-button"
              onClick={() =>
                void runAction(setToast, t('toast.logoutFailed'), async () => {
                  await api('/api/v1/auth/logout', { method: 'POST' });
                  setUser(null);
                })
              }
            >
              <LogOut size={18} />
              <span>{t('auth.logout')}</span>
            </button>
          </div>
          <form className="panel form-panel" onSubmit={submitVerification}>
            <SectionTitle icon={AlertCircle} title={t('auth.verifyEmail')} />
            <p className="inline-note">{t('auth.verificationHelp')}</p>
            <label>
              <span>{t('auth.verifyToken')}</span>
              <input value={verifyToken} onChange={(event) => setVerifyToken(event.target.value)} />
            </label>
            <button type="submit" className="primary-button">
              <Check size={18} />
              <span>{t('auth.completeVerification')}</span>
            </button>
          </form>
        </div>
      </section>
    );
  }

  return (
    <section className="page-grid">
      <div className="page-heading">
        <span className="eyebrow subtle">{t('profile.serverEyebrow')}</span>
        <h1>{t('profile.serverTitle')}</h1>
        <p>{t('profile.serverBody')}</p>
      </div>
      <div className="split">
        <div className="panel profile-card">
          <AvatarIcon seed={user.email || user.username} title={user.username} size={74} className="avatar-large" />
          <h2>{user.username}</h2>
          <p>{user.role}</p>
          <button
            type="button"
            className="secondary-button"
            onClick={() =>
              void runAction(setToast, t('toast.logoutFailed'), async () => {
                await api('/api/v1/auth/logout', { method: 'POST' });
                setUser(null);
              })
            }
          >
            <LogOut size={18} />
            <span>{t('auth.logout')}</span>
          </button>
        </div>

        <section className="panel">
          <SectionTitle icon={PackagePlus} title={t('profile.mySubmissions')} />
          <div className="workflow-summary-grid" aria-label={t('profile.publishOverview')}>
            <div>
              <span>{t('profile.totalSubmissions')}</span>
              <strong>{publishSummary.total}</strong>
            </div>
            <div>
              <span>{t('profile.listedSubmissions')}</span>
              <strong>{publishSummary.approved}</strong>
            </div>
            <div className={cx(publishSummary.pending > 0 && 'attention')}>
              <span>{t('profile.pendingSubmissions')}</span>
              <strong>{publishSummary.pending}</strong>
            </div>
            <div className={cx(publishSummary.needsVersion > 0 && 'attention')}>
              <span>{t('profile.needsVersion')}</span>
              <strong>{publishSummary.needsVersion}</strong>
            </div>
          </div>
          {ownedStatusSummary.length > 0 && (
            <div className="status-summary">
              {ownedStatusSummary.map((item) => (
                <div key={item.status}>
                  <strong>{item.count}</strong>
                  <span>{t(`statusLabels.${statusKey(item.status)}`)}</span>
                </div>
              ))}
            </div>
          )}
          <div className="review-list">
            {ownedApps.length === 0 ? (
              <EmptyState icon={PackagePlus} title={t('profile.mySubmissionsEmpty')} body={t('profile.mySubmissionsEmptyBody')} />
            ) : (
              ownedApps.map((item) => (
                <div className="review-row" key={item.id}>
                  <div>
                    <strong>{item.name}</strong>
                    <span>{item.latestVersion?.version || t('app.noPublishedVersion')} · {formatDate(item.updatedAt)}</span>
                    <small className="workflow-hint">{t(`profile.submissionStep.${submissionStep(item).key}`)}</small>
                  </div>
                  <div className="row-actions">
                    <span className={cx('status-badge', submissionStep(item).tone)}>{t(`statusLabels.${statusKey(item.status)}`)}</span>
                    <button type="button" className="secondary-button compact-button" onClick={() => void onOpen(item)}>
                      <ChevronRight size={17} />
                      <span>{t('profile.openSubmission')}</span>
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </section>
      </div>

      <section className="split">
        <form className="panel form-panel" onSubmit={submitUpload}>
          <SectionTitle icon={Upload} title={t('submitApp.title')} />
          <div className="workflow-strip">
            <div>
              <strong>{t('submitApp.publishPath')}</strong>
              <span>{t('submitApp.reviewHint')}</span>
            </div>
            <div className="workflow-steps" aria-label={t('submitApp.publishPath')}>
              <span>{t('submitApp.stepIdentity')}</span>
              <ChevronRight size={14} />
              <span>{t('submitApp.stepArtifact')}</span>
              <ChevronRight size={14} />
              <span>{t('submitApp.stepReview')}</span>
            </div>
          </div>
          <div className="submission-readiness" aria-label={t('submitApp.readiness')}>
            <div className={cx('readiness-step', appInfoComplete && 'ready')}>
              <span className={cx('status-badge', appInfoComplete ? 'approved' : appInfoReady ? 'pending' : 'unlisted')}>
                {appInfoComplete ? <Check size={14} /> : <AlertCircle size={14} />}
                {appInfoComplete ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
              </span>
              <strong>{t('submitApp.readinessAppInfo')}</strong>
              <small>
                {appInfoReady
                  ? appInfoDetailed
                    ? t('submitApp.readinessAppInfoReady')
                    : t('submitApp.readinessAppInfoNeedsDetails')
                  : t('submitApp.readinessAppInfoMissing')}
              </small>
            </div>
            <div className={cx('readiness-step', artifactReady && 'ready')}>
              <span className={cx('status-badge', artifactReady ? 'approved' : 'unlisted')}>
                {artifactReady ? <Check size={14} /> : <AlertCircle size={14} />}
                {artifactReady ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
              </span>
              <strong>{t('submitApp.readinessArtifact')}</strong>
              <small>
                {artifactMode === 'local'
                  ? file
                    ? t('submitApp.readinessArtifactLocalReady', { name: file.name, size: formatBytes(file.size) })
                    : t('submitApp.readinessArtifactLocalMissing')
                  : externalArtifactReady
                    ? t('submitApp.readinessArtifactExternalReady')
                    : externalDownloadReady || externalChecksumReady
                      ? t('submitApp.readinessArtifactExternalPartial')
                      : t('submitApp.readinessArtifactExternalMissing')}
              </small>
            </div>
            <div className="readiness-step ready">
              <span className="status-badge synced">
                <ShieldCheck size={14} />
                {isDirectPublishUser ? t('submitApp.readinessDirect') : t('submitApp.readinessQueued')}
              </span>
              <strong>{t('submitApp.readinessReview')}</strong>
              <small>{isDirectPublishUser ? t('submitApp.readinessReviewDirect') : t('submitApp.readinessReviewQueued')}</small>
            </div>
          </div>
          {recentSubmission && (
            <p className="inline-success">
              <Check size={15} />
              <span>
                {recentSubmission.status === 'APPROVED'
                  ? t('submitApp.submittedListed', { name: recentSubmission.name })
                  : t('submitApp.submittedQueued', { name: recentSubmission.name })}
              </span>
            </p>
          )}
          <label>
            <span>{t('submitApp.appName')}</span>
            <input value={uploadForm.name} onChange={(event) => setUploadForm({ ...uploadForm, name: event.target.value })} />
          </label>
          <label>
            <span>{t('common.version')}</span>
            <input value={uploadForm.version} onChange={(event) => setUploadForm({ ...uploadForm, version: event.target.value })} />
          </label>
          <label>
            <span>{t('common.summary')}</span>
            <input value={uploadForm.summary} onChange={(event) => setUploadForm({ ...uploadForm, summary: event.target.value })} />
          </label>
          <label>
            <span>{t('common.description')}</span>
            <textarea value={uploadForm.description} onChange={(event) => setUploadForm({ ...uploadForm, description: event.target.value })} />
          </label>
          <label>
            <span>{t('common.category')}</span>
            <select value={uploadForm.categoryId} onChange={(event) => setUploadForm({ ...uploadForm, categoryId: event.target.value })}>
              <option value="">{t('common.uncategorized')}</option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>{category.name}</option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('common.tags')}</span>
            <input value={uploadForm.tags} onChange={(event) => setUploadForm({ ...uploadForm, tags: event.target.value })} />
          </label>
          <div className="artifact-section">
            <div className="artifact-section-head">
              <strong>{t('submitApp.artifactMode')}</strong>
              <span>{artifactMode === 'local' ? t('submitApp.localArtifactHint') : t('submitApp.externalArtifactHint')}</span>
            </div>
            <div className="artifact-mode" aria-label={t('submitApp.artifactMode')}>
              <button type="button" className={cx(artifactMode === 'local' && 'active')} onClick={() => selectArtifactMode('local')}>
                <Upload size={17} />
                <span>
                  <strong>{t('submitApp.localArtifact')}</strong>
                  <small>{t('submitApp.localArtifactHint')}</small>
                </span>
              </button>
              <button type="button" className={cx(artifactMode === 'external' && 'active')} onClick={() => selectArtifactMode('external')}>
                <Link size={17} />
                <span>
                  <strong>{t('submitApp.externalArtifact')}</strong>
                  <small>{t('submitApp.externalArtifactHint')}</small>
                </span>
              </button>
            </div>
            {artifactMode === 'local' ? (
              <label>
                <span>{t('common.lpkFile')}</span>
                <input ref={fileInputRef} type="file" accept=".lpk" required onChange={(event) => setFile(event.target.files?.[0] || null)} />
                <small className="field-help">{t('submitApp.localFileHelp')}</small>
              </label>
            ) : (
              <div className="artifact-fields">
                <p className="field-help">{t('submitApp.externalFieldsHelp')}</p>
                <label>
                  <span>{t('submitApp.externalSource')}</span>
                  <select value={uploadForm.sourceType} onChange={(event) => setUploadForm({ ...uploadForm, sourceType: event.target.value })}>
                    <option value="GITHUB">GitHub Release</option>
                    <option value="WEBDAV">WebDAV URL</option>
                    <option value="S3">S3 URL</option>
                  </select>
                </label>
                <label>
                  <span>{t('submitApp.externalDownloadUrl')}</span>
                  <input
                    type="url"
                    required
                    value={uploadForm.downloadUrl}
                    onChange={(event) => setUploadForm({ ...uploadForm, downloadUrl: event.target.value })}
                  />
                  <small className="field-help">{t('submitApp.externalDownloadHelp')}</small>
                </label>
                <label>
                  <span>{t('common.sha256')}</span>
                  <input
                    required
                    maxLength={64}
                    pattern="[a-fA-F0-9]{64}"
                    title={t('submitApp.sha256Pattern')}
                    autoCapitalize="off"
                    autoCorrect="off"
                    spellCheck={false}
                    value={uploadForm.sha256}
                    onChange={(event) => setUploadForm({ ...uploadForm, sha256: event.target.value })}
                  />
                  <small className="field-help">{t('submitApp.sha256Help')}</small>
                </label>
              </div>
            )}
          </div>
          <label>
            <span>{t('submitApp.installPassword')}</span>
            <input
              type="password"
              autoComplete="new-password"
              minLength={4}
              maxLength={256}
              value={uploadForm.installPassword}
              onChange={(event) => setUploadForm({ ...uploadForm, installPassword: event.target.value })}
            />
            <small className="field-help">{t('submitApp.installPasswordHelp')}</small>
          </label>
          <label className="toggle-line">
            <input
              type="checkbox"
              checked={uploadForm.allowUnreviewedUpdates}
              onChange={(event) => setUploadForm({ ...uploadForm, allowUnreviewedUpdates: event.target.checked })}
            />
            <span>{t('submitApp.allowUnreviewedUpdates')}</span>
          </label>
          {!canSubmitUpload && <p className="field-help">{t('submitApp.submitBlocked')}</p>}
          <button type="submit" className="primary-button" disabled={!canSubmitUpload}>
            <Upload size={18} />
            <span>{t('common.submit')}</span>
          </button>
        </form>
        <section className="panel">
          <SectionTitle icon={KeyRound} title={t('token.title')} />
          <div className="review-list">
            {tokens.map((token) => (
              <div className="review-row" key={token.id}>
                <div>
                  <strong>{token.name}</strong>
                  <span>{token.prefix} · {formatDate(token.createdAt || token.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
          {newToken && <code className="token-output">{newToken}</code>}
          <button type="button" className="secondary-button" onClick={() => void createToken()}>
            <KeyRound size={18} />
            <span>{t('token.generate')}</span>
          </button>
        </section>
      </section>

      <section className="split">
        <GroupPanel groups={groups} setGroups={setGroups} setToast={setToast} />
        <section className="panel">
          <SectionTitle icon={Heart} title={t('favorites.title')} />
          <div className="review-list">
            {favorites.apps.length === 0 && favorites.submitters.length === 0 ? (
              <EmptyState icon={Heart} title={t('favorites.empty')} />
            ) : (
              <>
                {favorites.apps.map((item) => (
                  <div className="review-row" key={`app-${item.id}`}>
                    <div>
                      <strong>{item.name}</strong>
                      <span>{item.owner} · {item.latestVersion?.version || item.status}</span>
                    </div>
                  </div>
                ))}
                {favorites.submitters.map((item) => (
                  <div className="review-row" key={`submitter-${item.id}`}>
                    <div>
                      <strong>{item.username}</strong>
                      <span>{item.email || t('favorites.submitter')}</span>
                    </div>
                  </div>
                ))}
              </>
            )}
          </div>
          <button type="button" className="secondary-button" onClick={() => void loadFavorites()}>
            <RefreshCw size={18} />
            <span>{t('favorites.refresh')}</span>
          </button>
        </section>
        <section className="panel">
          <SectionTitle icon={Download} title={t('profile.installed')} />
          {installedState === 'error' && (
            <p className="inline-alert">
              <AlertCircle size={15} />
              <span>{installedError}</span>
            </p>
          )}
          <div className="review-list">
            {installedApps.length === 0 ? (
              <EmptyState icon={Download} title={installedState === 'loaded' ? t('profile.installedEmptyLoaded') : t('profile.installedEmpty')} body={installedState === 'idle' ? t('profile.installedIdleBody') : undefined} />
            ) : (
              installedApps.map((item) => (
                <div className="review-row" key={item.appid || item.title}>
                  <div>
                    <strong>{item.title || item.appid}</strong>
                    <span>{item.version || '-'} · {t('profile.status', { status: item.status ?? '-' })}</span>
                  </div>
                </div>
              ))
            )}
          </div>
          <button type="button" className="secondary-button" disabled={installedState === 'loading'} onClick={() => void onLoadInstalled()}>
            <RefreshCw size={18} />
            <span>{installedState === 'loading' ? t('profile.readingInstalled') : t('profile.readInstalled')}</span>
          </button>
        </section>
      </section>
    </section>
  );
}

function GroupPanel({
  groups,
  setGroups,
  setToast,
}: {
  groups: Group[];
  setGroups: (groups: Group[]) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState({ name: '', description: '' });
  const [memberDrafts, setMemberDrafts] = useState<Record<number, string>>({});

  async function reload() {
    await runAction(setToast, t('groups.loadFailed'), async () => {
      const data = await api<{ groups: Group[] }>('/api/v1/groups');
      setGroups(data.groups);
    });
  }

  async function createGroup(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('groups.createFailed'), async () => {
      await api('/api/v1/groups', { method: 'POST', body: JSON.stringify(draft) });
      setDraft({ name: '', description: '' });
      setToast({ tone: 'success', message: t('groups.created') });
      await reload();
    });
  }

  async function addMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, t('groups.addMemberFailed'), async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'POST' });
      setToast({ tone: 'success', message: t('groups.memberAdded') });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  async function removeMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, t('groups.removeMemberFailed'), async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('groups.memberRemoved') });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  return (
    <section className="panel form-panel">
      <SectionTitle icon={Users} title={t('groups.title')} />
      <form className="inline-form" onSubmit={createGroup}>
        <input aria-label={t('groups.name')} placeholder={t('groups.name')} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        <button type="submit" className="icon-button" aria-label={t('groups.create')}><Plus size={17} /></button>
      </form>
      <div className="review-list">
        {groups.length === 0 ? <EmptyState icon={Users} title={t('groups.empty')} /> : groups.map((group) => (
          <div className="review-row" key={group.id}>
            <div>
              <strong>{group.name}</strong>
              <span>{group.slug}</span>
            </div>
            <div className="inline-form compact-line group-member-actions">
              <input
                aria-label={t('groups.userId')}
                placeholder={t('groups.userId')}
                value={memberDrafts[group.id] || ''}
                onChange={(event) => setMemberDrafts((current) => ({ ...current, [group.id]: event.target.value }))}
              />
              <button type="button" className="icon-button" aria-label={t('groups.addMember')} onClick={() => void addMember(group.id)}><Plus size={17} /></button>
              <button type="button" className="icon-button danger" aria-label={t('groups.removeMember')} onClick={() => void removeMember(group.id)}><Trash2 size={17} /></button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function AdminPanel({
  user,
  reviews,
  onApprove,
  setToast,
}: {
  user: User;
  reviews: Review[];
  onApprove: (review: Review, approve: boolean) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [reviewApps, setReviewApps] = useState<StoreApp[]>([]);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [adminCategories, setAdminCategories] = useState<Category[]>([]);
  const [adminTags, setAdminTags] = useState<TagRecord[]>([]);
  const [adminCollections, setAdminCollections] = useState<Collection[]>([]);
  const [categoryForm, setCategoryForm] = useState({ name: '', slug: '' });
  const [tagForm, setTagForm] = useState({ name: '', slug: '' });
  const [collectionForm, setCollectionForm] = useState({ name: '', kind: 'MANUAL', appIds: '' });
  const [categoryDrafts, setCategoryDrafts] = useState<Record<number, { name: string; slug: string }>>({});
  const [tagDrafts, setTagDrafts] = useState<Record<number, { name: string; slug: string }>>({});
  const [collectionDrafts, setCollectionDrafts] = useState<Record<number, { name: string; slug: string; kind: string; appIds: string }>>({});
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const isSiteAdmin = user.role === 'SITE_ADMIN';
  const collectionKindOptions = [
    { value: 'MANUAL', label: t('admin.collectionKinds.manual') },
    { value: 'RECENT_UPDATED', label: t('admin.collectionKinds.recentUpdated') },
    { value: 'MOST_DOWNLOADED', label: t('admin.collectionKinds.mostDownloaded') },
  ];
  const settingFields = [
    { key: 'max_lpk_size', label: t('admin.settings.maxLPKSize'), help: t('admin.settingsHelp.maxLPKSize'), inputMode: 'numeric' },
    { key: 'max_versions', label: t('admin.settings.maxVersions'), help: t('admin.settingsHelp.maxVersions'), inputMode: 'numeric' },
    { key: 'source_password', label: t('admin.settings.sourcePassword'), help: t('admin.settingsHelp.sourcePassword'), type: 'password' },
    { key: 'source_password_rotation', label: t('admin.settings.sourcePasswordRotation'), help: t('admin.settingsHelp.sourcePasswordRotation'), inputMode: 'numeric' },
    { key: 'github_mirror', label: t('admin.settings.githubMirror'), help: t('admin.settingsHelp.githubMirror'), type: 'url' },
    { key: 'require_email_verify', label: t('admin.settings.requireEmailVerify'), help: t('admin.settingsHelp.requireEmailVerify'), type: 'boolean' },
  ];
  const reviewSummary = useMemo(() => {
    return {
      total: reviews.length,
      appSubmissions: reviews.filter((review) => review.kind === 'APP_SUBMISSION').length,
      versionUploads: reviews.filter((review) => review.kind === 'VERSION_UPLOAD').length,
      infoUpdates: reviews.filter((review) => review.kind === 'APP_INFO_UPDATE').length,
    };
  }, [reviews]);
  const reviewAppByID = useMemo(() => new Map(reviewApps.map((item) => [item.id, item])), [reviewApps]);
  const catalogReady = apps.length > 0 && adminCategories.length > 0;
  const sourceProtected = isSiteAdmin ? Boolean(settings.source_password?.trim()) : true;
  const reviewOpsBody = reviewSummary.total === 0
    ? t('admin.opsReviewClear')
    : t('admin.opsReviewPending', { count: reviewSummary.total });
  const catalogOpsBody =
    apps.length === 0
      ? t('admin.opsCatalogNeedsApps')
      : adminCategories.length === 0
        ? t('admin.opsCatalogNeedsCategories', { apps: apps.length })
        : t('admin.opsCatalogReady', { apps: apps.length, categories: adminCategories.length, collections: adminCollections.length });
  const sourceOpsBody = !isSiteAdmin
    ? t('admin.opsSourceDelegated')
    : sourceProtected
      ? t('admin.opsSourceProtected')
      : t('admin.opsSourceOpen');

  useEffect(() => {
    void reload();
  }, []);

  async function reload() {
    await runAction(setToast, t('admin.loadFailed'), async () => {
      const [categoryData, tagData, collectionData, appData] = await Promise.all([
        api<{ categories: Category[] }>('/api/v1/admin/categories'),
        api<{ tags: TagRecord[] }>('/api/v1/admin/tags'),
        api<{ collections: Collection[] }>('/api/v1/admin/collections'),
        api<{ apps: StoreApp[] }>('/api/v1/apps'),
      ]);
      setAdminCategories(categoryData.categories);
      setAdminTags(tagData.tags);
      setAdminCollections(collectionData.collections);
      setReviewApps(appData.apps);
      setApps(appData.apps.filter((item) => item.status === 'APPROVED'));
      setCategoryDrafts({});
      setTagDrafts({});
      setCollectionDrafts({});
      if (isSiteAdmin) {
        const [userData, settingData] = await Promise.all([
          api<{ users: User[] }>('/api/v1/admin/users'),
          api<{ settings: Record<string, string> }>('/api/v1/admin/settings'),
        ]);
        setUsers(userData.users);
        setSettings(settingData.settings);
      }
    });
  }

  function summarizeReviewNote(note?: string) {
    if (!note?.trim()) return '';
    try {
      const parsed = JSON.parse(note);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        const fields = Object.entries(parsed)
          .filter(([, value]) => value !== undefined && value !== null && value !== '')
          .map(([key]) => reviewFieldLabel(key, t))
          .slice(0, 4);
        if (fields.length > 0) {
          return t('admin.changedFields', { fields: fields.join(', ') });
        }
      }
    } catch {
      return note;
    }
    return note;
  }

  async function updateUserRole(userID: number, role: User['role']) {
    await runAction(setToast, t('admin.userRoleUpdateFailed'), async () => {
      await api(`/api/v1/admin/users/${userID}`, { method: 'PATCH', body: JSON.stringify({ role }) });
      setToast({ tone: 'success', message: t('admin.userRoleUpdated') });
      await reload();
    });
  }

  async function saveSettings(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.settingsSaveFailed'), async () => {
      await api('/api/v1/admin/settings', { method: 'PATCH', body: JSON.stringify(settings) });
      setToast({ tone: 'success', message: t('admin.settingsSaved') });
      await reload();
    });
  }

  async function createCategory(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.categoryCreateFailed'), async () => {
      await api('/api/v1/admin/categories', { method: 'POST', body: JSON.stringify(categoryForm) });
      setCategoryForm({ name: '', slug: '' });
      setToast({ tone: 'success', message: t('admin.categoryCreated') });
      await reload();
    });
  }

  async function updateCategory(item: Category) {
    const draft = categoryDrafts[item.id] || { name: item.name, slug: item.slug };
    await runAction(setToast, t('admin.categoryUpdateFailed'), async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: t('admin.categoryUpdated') });
      await reload();
    });
  }

  async function deleteCategory(item: Category) {
    const confirmKey = `category:${item.id}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteCategory', { name: item.name }) });
      return;
    }
    await runAction(setToast, t('admin.categoryDeleteFailed'), async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.categoryDeleted') });
      setConfirmDelete(null);
      await reload();
    });
  }

  async function createTag(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.tagCreateFailed'), async () => {
      await api('/api/v1/admin/tags', { method: 'POST', body: JSON.stringify(tagForm) });
      setTagForm({ name: '', slug: '' });
      setToast({ tone: 'success', message: t('admin.tagCreated') });
      await reload();
    });
  }

  async function updateTag(item: TagRecord) {
    const draft = tagDrafts[item.id] || { name: item.name, slug: item.slug };
    await runAction(setToast, t('admin.tagUpdateFailed'), async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: t('admin.tagUpdated') });
      await reload();
    });
  }

  async function deleteTag(item: TagRecord) {
    const confirmKey = `tag:${item.id}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteTag', { name: item.name }) });
      return;
    }
    await runAction(setToast, t('admin.tagDeleteFailed'), async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.tagDeleted') });
      setConfirmDelete(null);
      await reload();
    });
  }

  async function createCollection(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.collectionCreateFailed'), async () => {
      await api('/api/v1/admin/collections', {
        method: 'POST',
        body: JSON.stringify({
          name: collectionForm.name,
          kind: collectionForm.kind,
          appIds: collectionForm.appIds.split(',').map((id) => Number(id.trim())).filter(Boolean),
        }),
      });
      setCollectionForm({ name: '', kind: 'MANUAL', appIds: '' });
      setToast({ tone: 'success', message: t('admin.collectionCreated') });
      await reload();
    });
  }

  async function updateCollection(item: Collection) {
    const draft =
      collectionDrafts[item.id] || {
        name: item.name,
        slug: item.slug,
        kind: item.kind,
        appIds: (item.apps || []).map((app) => app.id).join(','),
      };
    await runAction(setToast, t('admin.collectionUpdateFailed'), async () => {
      await api(`/api/v1/admin/collections/${item.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          name: draft.name,
          slug: draft.slug,
          kind: draft.kind,
          appIds: draft.appIds.split(',').map((id) => Number(id.trim())).filter(Boolean),
        }),
      });
      setToast({ tone: 'success', message: t('admin.collectionUpdated') });
      await reload();
    });
  }

  async function deleteCollection(item: Collection) {
    const confirmKey = `collection:${item.id}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteCollection', { name: item.name }) });
      return;
    }
    await runAction(setToast, t('admin.collectionDeleteFailed'), async () => {
      await api(`/api/v1/admin/collections/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.collectionDeleted') });
      setConfirmDelete(null);
      await reload();
    });
  }

  return (
    <section className="page-grid">
      <div className="page-heading">
        <span className="eyebrow subtle">{t('admin.eyebrow')}</span>
        <h1>{t('admin.title')}</h1>
        <p>{t('admin.body')}</p>
      </div>
      <section className="panel">
        <SectionTitle icon={Gauge} title={t('admin.operationsOverview')} />
        <div className="source-readiness" aria-label={t('admin.operationsOverview')}>
          <div className={cx('readiness-step', reviewSummary.total === 0 && 'ready')}>
            <span className={cx('status-badge', reviewSummary.total === 0 ? 'approved' : 'pending')}>
              {reviewSummary.total === 0 ? <Check size={14} /> : <AlertCircle size={14} />}
              {reviewSummary.total === 0 ? t('admin.opsReady') : t('admin.opsNeedsAction')}
            </span>
            <strong>{t('admin.opsReviewTitle')}</strong>
            <small>{reviewOpsBody}</small>
          </div>
          <div className={cx('readiness-step', catalogReady && 'ready')}>
            <span className={cx('status-badge', catalogReady ? 'approved' : 'unlisted')}>
              {catalogReady ? <Check size={14} /> : <AlertCircle size={14} />}
              {catalogReady ? t('admin.opsReady') : t('admin.opsNeedsAction')}
            </span>
            <strong>{t('admin.opsCatalogTitle')}</strong>
            <small>{catalogOpsBody}</small>
          </div>
          <div className={cx('readiness-step', sourceProtected && 'ready')}>
            <span className={cx('status-badge', sourceProtected ? 'synced' : 'unlisted')}>
              {sourceProtected ? <ShieldCheck size={14} /> : <AlertCircle size={14} />}
              {sourceProtected ? t('admin.opsReady') : t('admin.opsNeedsAction')}
            </span>
            <strong>{t('admin.opsSourceTitle')}</strong>
            <small>{sourceOpsBody}</small>
          </div>
        </div>
      </section>
      <section className="panel">
        <SectionTitle icon={ShieldCheck} title={t('admin.reviewQueue')} />
        <div className="workflow-summary-grid" aria-label={t('admin.reviewSummary')}>
          <div className={cx(reviewSummary.total > 0 && 'attention')}>
            <span>{t('admin.pendingTotal')}</span>
            <strong>{reviewSummary.total}</strong>
          </div>
          <div>
            <span>{t('admin.appSubmissions')}</span>
            <strong>{reviewSummary.appSubmissions}</strong>
          </div>
          <div>
            <span>{t('admin.versionUploads')}</span>
            <strong>{reviewSummary.versionUploads}</strong>
          </div>
          <div>
            <span>{t('admin.infoUpdates')}</span>
            <strong>{reviewSummary.infoUpdates}</strong>
          </div>
        </div>
        <div className="review-list">
          {reviews.length === 0 ? (
            <EmptyState icon={ShieldCheck} title={t('admin.noPendingReviews')} body={t('admin.noPendingReviewsBody')} />
          ) : (
            reviews.map((review) => {
              const reviewApp = review.appId ? reviewAppByID.get(review.appId) : undefined;
              const reviewVersion = reviewApp?.latestVersion;
              const noteSummary = summarizeReviewNote(review.note);
              return (
                <div className="review-row review-workflow-row" key={review.id}>
                  <div>
                    <strong>{reviewApp ? reviewApp.name : t('admin.unknownApp')}</strong>
                    <span>
                      {t(`reviewKinds.${reviewKindKey(review.kind)}`)} · {t('admin.reviewTarget', { target: review.appId ? `#${review.appId}` : review.versionId ? `v#${review.versionId}` : '-' })} · {t('admin.requester', { id: review.requesterId })} · {formatDate(review.createdAt)}
                    </span>
                    {reviewApp && (
                      <small className="workflow-hint">
                        {reviewApp.summary || reviewApp.latestVersion?.version || t('common.lpkApp')}
                      </small>
                    )}
                    {reviewApp && (
                      <div className="review-facts">
                        {reviewVersion ? (
                          <>
                            <span>{t('admin.reviewArtifact', { source: reviewVersion.sourceType || '-', size: formatBytes(reviewVersion.fileSize) })}</span>
                            <span>{t('admin.reviewChecksum', { hash: shortSHA(reviewVersion.sha256) })}</span>
                          </>
                        ) : (
                          <span>{t('admin.reviewArtifactPending')}</span>
                        )}
                      </div>
                    )}
                    {noteSummary && (
                      <p className="review-note">{noteSummary}</p>
                    )}
                  </div>
                  <div className="row-actions">
                    <span className={cx('status-badge', statusKey(review.status))}>{t(`statusLabels.${statusKey(review.status)}`)}</span>
                    <button
                      type="button"
                      className="icon-button ok"
                      aria-label={t('admin.approveReview', { id: review.id })}
                      onClick={() => void onApprove(review, true)}
                    >
                      <Check size={17} />
                    </button>
                    <button
                      type="button"
                      className="icon-button danger"
                      aria-label={t('admin.rejectReview', { id: review.id })}
                      onClick={() => void onApprove(review, false)}
                    >
                      <X size={17} />
                    </button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </section>
      {isSiteAdmin && (
        <section className="split">
          <form className="panel form-panel" onSubmit={saveSettings}>
            <SectionTitle icon={Settings} title={t('admin.siteSettings')} />
            {settingFields.map((field) => (
              <label key={field.key}>
                <span>{field.label}</span>
                {field.type === 'boolean' ? (
                  <select value={settings[field.key] || 'false'} onChange={(event) => setSettings({ ...settings, [field.key]: event.target.value })}>
                    <option value="false">{t('common.off')}</option>
                    <option value="true">{t('common.on')}</option>
                  </select>
                ) : (
                  <input
                    type={field.type || 'text'}
                    inputMode={field.inputMode as 'numeric' | undefined}
                    value={settings[field.key] || ''}
                    onChange={(event) => setSettings({ ...settings, [field.key]: event.target.value })}
                  />
                )}
                <small className="field-help">{field.help}</small>
              </label>
            ))}
            <button type="submit" className="primary-button">
              <Settings size={18} />
              <span>{t('admin.saveSettings')}</span>
            </button>
          </form>
          <section className="panel">
            <SectionTitle icon={Users} title={t('admin.userManagement')} />
            <div className="review-list">
              {users.map((item) => (
                <div className="review-row" key={item.id}>
                  <div>
                    <strong>#{item.id} {item.username}</strong>
                    <span>{item.email || t('admin.noEmail')}</span>
                  </div>
                  <select
                    aria-label={t('admin.userRoleFor', { username: item.username })}
                    value={item.role}
                    onChange={(event) => void updateUserRole(item.id, event.target.value as User['role'])}
                  >
                    <option value="USER">USER</option>
                    <option value="SOFTWARE_ADMIN">SOFTWARE_ADMIN</option>
                    <option value="SITE_ADMIN">SITE_ADMIN</option>
                  </select>
                </div>
              ))}
            </div>
          </section>
        </section>
      )}
      <section className="split">
        <div className="panel form-panel">
          <SectionTitle icon={Layers3} title={t('admin.categoriesAndTags')} />
          <form className="inline-stack" onSubmit={createCategory}>
            <input aria-label={t('admin.categoryName')} placeholder={t('admin.categoryName')} value={categoryForm.name} onChange={(event) => setCategoryForm({ ...categoryForm, name: event.target.value })} />
            <input aria-label={t('admin.categorySlug')} placeholder={t('admin.categorySlug')} value={categoryForm.slug} onChange={(event) => setCategoryForm({ ...categoryForm, slug: event.target.value })} />
            <button type="submit" className="secondary-button"><Plus size={17} /><span>{t('admin.category')}</span></button>
          </form>
          <form className="inline-stack" onSubmit={createTag}>
            <input aria-label={t('admin.tagName')} placeholder={t('admin.tagName')} value={tagForm.name} onChange={(event) => setTagForm({ ...tagForm, name: event.target.value })} />
            <input aria-label={t('admin.tagSlug')} placeholder={t('admin.tagSlug')} value={tagForm.slug} onChange={(event) => setTagForm({ ...tagForm, slug: event.target.value })} />
            <button type="submit" className="secondary-button"><Plus size={17} /><span>{t('admin.tag')}</span></button>
          </form>
        </div>
        <section className="panel">
          <SectionTitle icon={Tag} title={t('admin.categoryList')} />
          <div className="review-list">
            {adminCategories.map((item) => {
              const draft = categoryDrafts[item.id] || { name: item.name, slug: item.slug };
              return (
                <div className="edit-row" key={item.id}>
                  <input aria-label={t('admin.categoryNameFor', { name: item.name })} value={draft.name} onChange={(event) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input aria-label={t('admin.categorySlugFor', { name: item.name })} value={draft.slug} onChange={(event) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <div className="row-actions">
                    <button type="button" className="icon-button" aria-label={t('admin.saveCategoryNamed', { name: item.name })} onClick={() => void updateCategory(item)}><Save size={16} /></button>
                    <button type="button" className="icon-button danger" aria-label={t('admin.deleteCategoryNamed', { name: item.name })} onClick={() => void deleteCategory(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
          <SectionTitle icon={Tag} title={t('admin.tagList')} />
          <div className="review-list">
            {adminTags.map((item) => {
              const draft = tagDrafts[item.id] || { name: item.name, slug: item.slug };
              return (
                <div className="edit-row" key={item.id}>
                  <input aria-label={t('admin.tagNameFor', { name: item.name })} value={draft.name} onChange={(event) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input aria-label={t('admin.tagSlugFor', { name: item.name })} value={draft.slug} onChange={(event) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <div className="row-actions">
                    <button type="button" className="icon-button" aria-label={t('admin.saveTagNamed', { name: item.name })} onClick={() => void updateTag(item)}><Save size={16} /></button>
                    <button type="button" className="icon-button danger" aria-label={t('admin.deleteTagNamed', { name: item.name })} onClick={() => void deleteTag(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      <section className="split">
        <form className="panel form-panel" onSubmit={createCollection}>
          <SectionTitle icon={Layers3} title={t('admin.collection')} />
          <label>
            <span>{t('common.name')}</span>
            <input value={collectionForm.name} onChange={(event) => setCollectionForm({ ...collectionForm, name: event.target.value })} />
          </label>
          <label>
            <span>{t('admin.type')}</span>
            <select value={collectionForm.kind} onChange={(event) => setCollectionForm({ ...collectionForm, kind: event.target.value })}>
              {collectionKindOptions.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </label>
          <label>
            <span>{t('admin.appIds')}</span>
            <input value={collectionForm.appIds} onChange={(event) => setCollectionForm({ ...collectionForm, appIds: event.target.value })} />
          </label>
          <button type="submit" className="primary-button">
            <Layers3 size={18} />
            <span>{t('admin.createCollection')}</span>
          </button>
        </form>
        <section className="panel">
          <SectionTitle icon={Layers3} title={t('admin.collectionList')} />
          <div className="review-list">
            {adminCollections.map((item) => {
              const draft =
                collectionDrafts[item.id] || {
                  name: item.name,
                  slug: item.slug,
                  kind: item.kind,
                  appIds: (item.apps || []).map((app) => app.id).join(','),
                };
              return (
                <div className="collection-edit-row" key={item.id}>
                  <input aria-label={t('admin.collectionNameFor', { name: item.name })} value={draft.name} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input aria-label={t('admin.collectionSlugFor', { name: item.name })} value={draft.slug} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <select aria-label={t('admin.collectionTypeFor', { name: item.name })} value={draft.kind} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, kind: event.target.value } }))}>
                    {collectionKindOptions.map((option) => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </select>
                  <input aria-label={t('admin.collectionAppIdsFor', { name: item.name })} value={draft.appIds} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, appIds: event.target.value } }))} />
                  <div className="row-actions">
                    <button type="button" className="icon-button" aria-label={t('admin.saveCollectionNamed', { name: item.name })} onClick={() => void updateCollection(item)}><Save size={16} /></button>
                    <button type="button" className="icon-button danger" aria-label={t('admin.deleteCollectionNamed', { name: item.name })} onClick={() => void deleteCollection(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      <section className="panel">
        <SectionTitle icon={PackagePlus} title={t('admin.optionalApps')} />
          <div className="review-list">
            {apps.length === 0 ? (
              <EmptyState icon={PackagePlus} title={t('admin.noApprovedApps')} body={t('admin.noApprovedAppsBody')} />
            ) : apps.map((item) => (
              <div className="review-row" key={item.id}>
                <div>
                  <strong>#{item.id} {item.name}</strong>
                  <span>{item.owner} · {item.latestVersion?.version || item.status}</span>
                </div>
              </div>
            ))}
          </div>
      </section>
    </section>
  );
}

function AppDrawer({
  app,
  user,
  groups,
  categories,
  onClose,
  onInstall,
  onRefresh,
  onListRefresh,
  setToast,
}: {
  app: StoreApp;
  user: User | null;
  groups: Group[];
  categories: Category[];
  onClose: () => void;
  onInstall: (app: StoreApp) => void;
  onRefresh: () => Promise<void>;
  onListRefresh: () => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [commentText, setCommentText] = useState('');
  const [screenshotFile, setScreenshotFile] = useState<File | null>(null);
  const [screenshotCaption, setScreenshotCaption] = useState('');
  const [versionForm, setVersionForm] = useState({ version: '', sourceType: 'GITHUB', downloadUrl: '', sha256: '', changelog: '' });
  const [versionArtifactMode, setVersionArtifactMode] = useState<'local' | 'external'>('local');
  const [versionFile, setVersionFile] = useState<File | null>(null);
  const [collaboratorRequests, setCollaboratorRequests] = useState<CollaboratorRequest[]>([]);
  const [confirmAction, setConfirmAction] = useState<string | null>(null);
  const [appForm, setAppForm] = useState({
    name: app.name,
    summary: app.summary,
    description: app.description,
    categoryId: app.categoryId ? String(app.categoryId) : '',
    tags: (app.tags || []).join(', '),
    allowUnreviewedUpdates: app.allowUnreviewedUpdates,
    commentsEnabled: app.commentsEnabled,
    installPassword: '',
    clearInstallPassword: false,
  });
  const [visibility, setVisibility] = useState<number[]>(app.visibleGroupIds || []);
  const canMaintain = !!user && (app.canManageApp ?? (user.role === 'SITE_ADMIN' || user.role === 'SOFTWARE_ADMIN' || user.id === app.ownerId));
  const canUploadVersion = !!user && (app.canUploadVersion || canMaintain);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const versionFileInputRef = useRef<HTMLInputElement>(null);
  const drawerTitleId = `app-drawer-title-${app.id}`;
  const latestVersion = app.latestVersion;
  const installable = hasInstallableVersion(app);
  const hasChecksum = Boolean(latestVersion?.sha256);
  const hasFileSize = Boolean(latestVersion && latestVersion.fileSize > 0);
  const trustState: 'ready' | 'caution' | 'blocked' = !installable ? 'blocked' : hasChecksum && hasFileSize ? 'ready' : 'caution';
  const TrustIcon = trustState === 'ready' ? ShieldCheck : trustState === 'caution' ? Gauge : AlertCircle;
  const trustTitle = trustState === 'ready' ? t('drawer.trustReadyTitle') : trustState === 'caution' ? t('drawer.trustCautionTitle') : t('drawer.trustBlockedTitle');
  const trustBody = trustState === 'ready' ? t('drawer.trustReadyBody') : trustState === 'caution' ? t('drawer.trustCautionBody') : t('drawer.trustBlockedBody');
  const installNextStep = canUploadVersion ? t('drawer.installBlockedMaintainer') : t('drawer.installBlockedUser');
  const trustFacts = [
    { label: t('drawer.installStatus'), value: installable ? t('app.installReady') : t('app.installMissingVersion') },
    { label: t('drawer.installAccess'), value: app.installProtected ? t('app.installPasswordRequired') : t('app.installOpen') },
    { label: t('drawer.artifactSource'), value: latestVersion?.sourceType || t('drawer.sourceMissing') },
    { label: t('drawer.artifactChecksum'), value: hasChecksum ? t('drawer.checksumShort', { hash: shortSHA(latestVersion?.sha256) }) : t('drawer.checksumMissing') },
    { label: t('drawer.artifactSize'), value: hasFileSize ? formatBytes(latestVersion?.fileSize) : t('drawer.sizeMissing') },
  ];
  const communitySummary = t('drawer.communitySummary', {
    favorites: app.favorites ?? 0,
    comments: (app.comments || []).length,
    outdated: app.outdatedMarks ?? 0,
    screenshots: (app.screenshots || []).length,
  });
  const versionNumberReady = Boolean(versionForm.version.trim());
  const versionExternalDownloadReady = Boolean(versionForm.downloadUrl.trim());
  const versionExternalChecksumReady = Boolean(versionForm.sha256.trim());
  const versionExternalArtifactReady = versionExternalDownloadReady && versionExternalChecksumReady;
  const versionArtifactReady = versionArtifactMode === 'local' ? Boolean(versionFile) : versionExternalArtifactReady;
  const canSubmitVersion = versionNumberReady && versionArtifactReady;
  const versionPublishesDirectly = user?.role === 'SITE_ADMIN' || user?.role === 'SOFTWARE_ADMIN' || app.allowUnreviewedUpdates;

  useEffect(() => {
    setAppForm({
      name: app.name,
      summary: app.summary,
      description: app.description,
      categoryId: app.categoryId ? String(app.categoryId) : '',
      tags: (app.tags || []).join(', '),
      allowUnreviewedUpdates: app.allowUnreviewedUpdates,
      commentsEnabled: app.commentsEnabled,
      installPassword: '',
      clearInstallPassword: false,
    });
    setVisibility(app.visibleGroupIds || []);
    setConfirmAction(null);
    setVersionArtifactMode('local');
    setVersionFile(null);
    if (versionFileInputRef.current) versionFileInputRef.current.value = '';
  }, [app]);

  useEffect(() => {
    if (!canMaintain) return;
    void loadCollaboratorRequests();
  }, [app.id, canMaintain]);

  useEffect(() => {
    closeButtonRef.current?.focus();
  }, [app.id]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  async function loadCollaboratorRequests() {
    await runAction(setToast, t('drawer.loadCollaboratorsFailed'), async () => {
      const data = await api<{ requests: CollaboratorRequest[] }>(`/api/v1/apps/${app.id}/collaborator-requests`);
      setCollaboratorRequests(data.requests);
    });
  }

  async function submitComment(event: FormEvent) {
    event.preventDefault();
    if (!commentText.trim()) return;
    await runAction(setToast, t('drawer.commentPostFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/comments`, { method: 'POST', body: JSON.stringify({ body: commentText }) });
      setCommentText('');
      setToast({ tone: 'success', message: t('drawer.commentPosted') });
      await onRefresh();
    });
  }

  async function markOutdated() {
    await runAction(setToast, t('drawer.markOutdatedFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/outdated-marks`, { method: 'POST', body: JSON.stringify({ note: t('drawer.defaultOutdatedNote') }) });
      setToast({ tone: 'neutral', message: t('drawer.outdatedMarked') });
      await onRefresh();
    });
  }

  async function clearOutdated() {
    await runAction(setToast, t('drawer.clearOutdatedFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/outdated-marks`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('drawer.outdatedCleared') });
      await onRefresh();
    });
  }

  async function submitAppInfo(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('drawer.appInfoSaveFailed'), async () => {
      const installPassword = appForm.installPassword.trim();
      const data = await api<{ app?: StoreApp; review?: Review }>(`/api/v1/apps/${app.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          name: appForm.name,
          summary: appForm.summary,
          description: appForm.description,
          categoryId: appForm.categoryId ? Number(appForm.categoryId) : undefined,
          tags: appForm.tags.split(',').map((tag) => tag.trim()).filter(Boolean),
          allowUnreviewedUpdates: appForm.allowUnreviewedUpdates,
          commentsEnabled: appForm.commentsEnabled,
          ...(installPassword || appForm.clearInstallPassword ? { installPassword } : {}),
        }),
      });
      setToast({ tone: 'success', message: data.review ? t('drawer.appInfoSubmittedReview') : t('drawer.appInfoSaved') });
      await onRefresh();
    });
  }

  function selectVersionArtifactMode(nextMode: 'local' | 'external') {
    setVersionArtifactMode(nextMode);
    if (nextMode === 'local') {
      setVersionForm((current) => ({ ...current, downloadUrl: '', sha256: '' }));
      return;
    }
    setVersionFile(null);
    if (versionFileInputRef.current) versionFileInputRef.current.value = '';
  }

  async function submitExternalVersion(event: FormEvent) {
    event.preventDefault();
    if (!versionForm.version.trim()) {
      setToast({ tone: 'error', message: t('drawer.versionRequired') });
      return;
    }
    if (versionArtifactMode === 'local' && !versionFile) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (versionArtifactMode === 'external' && !versionForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (versionArtifactMode === 'external' && !versionForm.sha256.trim()) {
      setToast({ tone: 'error', message: t('submitApp.sha256Required') });
      return;
    }
    await runAction(setToast, t('drawer.versionSubmitFailed'), async () => {
      if (versionArtifactMode === 'local' && versionFile) {
        const form = new FormData();
        form.set('file', versionFile);
        form.set('version', versionForm.version.trim());
        form.set('changelog', versionForm.changelog);
        await api(`/api/v1/apps/${app.id}/versions`, { method: 'POST', body: form });
      } else {
        await api(`/api/v1/apps/${app.id}/versions`, {
          method: 'POST',
          body: JSON.stringify({
            ...versionForm,
            version: versionForm.version.trim(),
            downloadUrl: versionForm.downloadUrl.trim(),
            sha256: versionForm.sha256.trim(),
          }),
        });
      }
      setVersionForm({ version: '', sourceType: 'GITHUB', downloadUrl: '', sha256: '', changelog: '' });
      setVersionArtifactMode('local');
      setVersionFile(null);
      if (versionFileInputRef.current) versionFileInputRef.current.value = '';
      setToast({ tone: 'success', message: t('drawer.versionSubmitted') });
      await onRefresh();
    });
  }

  function confirmDanger(key: string, message: string) {
    if (confirmAction !== key) {
      setConfirmAction(key);
      setToast({ tone: 'neutral', message });
      return false;
    }
    setConfirmAction(null);
    return true;
  }

  async function unlistApp() {
    if (!confirmDanger('unlist-app', t('drawer.confirmUnlist', { name: app.name }))) return;
    await runAction(setToast, t('drawer.unlistFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/unlist`, { method: 'POST' });
      setToast({ tone: 'neutral', message: t('drawer.unlisted') });
      await onRefresh();
    });
  }

  async function deleteApp() {
    if (!confirmDanger('delete-app', t('drawer.confirmDeleteApp', { name: app.name }))) return;
    await runAction(setToast, t('drawer.deleteFailed'), async () => {
      await api(`/api/v1/apps/${app.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('drawer.deleted') });
      onClose();
      await onListRefresh();
    });
  }

  async function uploadScreenshot(event: FormEvent) {
    event.preventDefault();
    if (!screenshotFile) return;
    const form = new FormData();
    form.set('file', screenshotFile);
    form.set('caption', screenshotCaption);
    await runAction(setToast, t('drawer.screenshotUploadFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots`, { method: 'POST', body: form });
      setScreenshotFile(null);
      setScreenshotCaption('');
      setToast({ tone: 'success', message: t('drawer.screenshotUploaded') });
      await onRefresh();
    });
  }

  async function moveScreenshot(screenshotID: number, direction: -1 | 1) {
    const shots = [...(app.screenshots || [])].sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
    const index = shots.findIndex((shot) => shot.id === screenshotID);
    const nextIndex = index + direction;
    if (index < 0 || nextIndex < 0 || nextIndex >= shots.length) return;
    [shots[index], shots[nextIndex]] = [shots[nextIndex], shots[index]];
    await runAction(setToast, t('drawer.screenshotReorderFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/reorder`, {
        method: 'PATCH',
        body: JSON.stringify({ items: shots.map((shot, sortOrder) => ({ id: shot.id, sortOrder })) }),
      });
      setToast({ tone: 'success', message: t('drawer.screenshotReordered') });
      await onRefresh();
    });
  }

  async function deleteScreenshot(screenshotID: number) {
    if (!confirmDanger(`delete-screenshot:${screenshotID}`, t('drawer.confirmDeleteScreenshot'))) return;
    await runAction(setToast, t('drawer.screenshotDeleteFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/${screenshotID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('drawer.screenshotDeleted') });
      await onRefresh();
    });
  }

  async function deleteComment(commentID: number) {
    if (!confirmDanger(`delete-comment:${commentID}`, t('drawer.confirmDeleteComment'))) return;
    await runAction(setToast, t('drawer.commentDeleteFailed'), async () => {
      await api(`/api/v1/comments/${commentID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('drawer.commentDeleted') });
      await onRefresh();
    });
  }

  async function saveVisibility() {
    await runAction(setToast, t('drawer.visibilitySaveFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/visibility`, {
        method: 'PATCH',
        body: JSON.stringify({ groupIds: visibility }),
      });
      setToast({ tone: 'success', message: visibility.length === 0 ? t('drawer.visibilityPublic') : t('drawer.visibilityUpdated') });
      await onRefresh();
    });
  }

  async function requestCollaborator() {
    await runAction(setToast, t('drawer.requestCollaboratorFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/collaborator-requests`, {
        method: 'POST',
        body: JSON.stringify({ message: t('drawer.collaboratorMessage') }),
      });
      setToast({ tone: 'success', message: t('drawer.collaboratorRequestSubmitted') });
    });
  }

  async function decideCollaboratorRequest(requestID: number, approve: boolean) {
    await runAction(setToast, t('drawer.collaboratorDecisionFailed'), async () => {
      await api(`/api/v1/collaborator-requests/${requestID}/${approve ? 'approve' : 'reject'}`, { method: 'POST' });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? t('drawer.collaboratorApproved') : t('drawer.collaboratorRejected') });
      await loadCollaboratorRequests();
    });
  }

  async function toggleAppFavorite() {
    await runAction(setToast, t('drawer.favoriteUpdateFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: t('drawer.favoriteUpdated') });
      await onRefresh();
    });
  }

  async function toggleSubmitterFavorite() {
    await runAction(setToast, t('drawer.submitterFavoriteUpdateFailed'), async () => {
      await api(`/api/v1/submitters/${app.ownerId}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: t('drawer.submitterFavoriteUpdated') });
    });
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <article
        className="drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby={drawerTitleId}
        onClick={(event) => event.stopPropagation()}
      >
        <button ref={closeButtonRef} type="button" className="icon-button close" aria-label={t('common.close')} onClick={onClose}><X size={18} /></button>
        <div className="detail-head">
          <AvatarIcon seed={app.slug || app.name} title={app.name} size={58} className="detail-avatar" />
          <div>
            <h2 id={drawerTitleId}>{app.name}</h2>
            <p>{app.summary || app.description}</p>
            <div className="meta-line">
              <span>{app.owner}</span>
              <span>{app.category || t('common.uncategorized')}</span>
              <span>{app.latestVersion?.version || '-'}</span>
            </div>
          </div>
        </div>
        <div className="detail-actions">
          <button
            type="button"
            className="primary-button"
            disabled={!installable}
            onClick={() => onInstall(app)}
            aria-label={installable ? t('app.install', { name: app.name }) : t('app.installUnavailable', { name: app.name })}
          >
            <Download size={18} />
            <span>{installable ? t('common.install') : t('common.unavailable')}</span>
          </button>
          {user && (
            <>
              <button type="button" className="secondary-button" onClick={() => void toggleAppFavorite()}>
                <Heart size={18} />
                <span>{t('drawer.favorite')}</span>
              </button>
              <button type="button" className="secondary-button" onClick={() => void toggleSubmitterFavorite()}>
                <Star size={18} />
                <span>{t('drawer.submitter')}</span>
              </button>
              <button type="button" className="secondary-button" onClick={() => void markOutdated()}>
                <AlertCircle size={18} />
                <span>{t('drawer.outdated')}</span>
              </button>
              <button type="button" className="secondary-button" onClick={() => void clearOutdated()}>
                <Check size={18} />
                <span>{t('drawer.clearOutdated')}</span>
              </button>
            </>
          )}
          {user && user.id !== app.ownerId && (
            <button type="button" className="secondary-button" onClick={() => void requestCollaborator()}>
              <Users size={18} />
              <span>{t('drawer.collaborate')}</span>
            </button>
          )}
          {canMaintain && (
            <>
              <button type="button" className="secondary-button" onClick={() => void unlistApp()}>
                <Archive size={18} />
                <span>{t('drawer.unlist')}</span>
              </button>
              <button type="button" className="secondary-button danger-button" onClick={() => void deleteApp()}>
                <Trash2 size={18} />
                <span>{t('common.delete')}</span>
              </button>
            </>
          )}
        </div>
        <section className={cx('install-trust', trustState)} aria-label={t('drawer.installReadiness')}>
          <div className="install-trust-lead">
            <TrustIcon size={19} />
            <div>
              <strong>{trustTitle}</strong>
              <span>{trustBody}</span>
              {!installable && <small>{installNextStep}</small>}
            </div>
          </div>
          <div className="trust-facts" role="list">
            {trustFacts.map((fact) => (
              <div role="listitem" key={fact.label}>
                <span>{fact.label}</span>
                <strong>{fact.value}</strong>
              </div>
            ))}
            <div role="listitem" className="trust-fact-wide">
              <span>{t('drawer.communitySignals')}</span>
              <strong>{communitySummary}</strong>
            </div>
          </div>
        </section>
        <section className="detail-summary" aria-label={t('drawer.metadata')}>
          <div>
            <span>{t('drawer.latestVersion')}</span>
            <strong>{latestVersion?.version || t('app.noPublishedVersion')}</strong>
          </div>
          <div>
            <span>{t('common.download')}</span>
            <strong>{t('app.downloads', { count: app.downloadCount })}</strong>
          </div>
          <div>
            <span>{t('common.source')}</span>
            <strong>{latestVersion?.sourceType || '-'}</strong>
          </div>
          <div>
            <span>{t('app.fileSize', { size: latestVersion ? formatBytes(latestVersion.fileSize) : '-' })}</span>
            <strong>{t('drawer.sha256', { hash: shortSHA(latestVersion?.sha256) })}</strong>
          </div>
        </section>
        {(canMaintain || canUploadVersion) && (
          <section className="maintenance-grid">
            {canMaintain && (
              <form className="panel form-panel nested-panel" onSubmit={submitAppInfo}>
                <SectionTitle icon={Settings} title={t('drawer.appInfo')} />
                <label>
                  <span>{t('common.name')}</span>
                  <input value={appForm.name} onChange={(event) => setAppForm({ ...appForm, name: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.summary')}</span>
                  <input value={appForm.summary} onChange={(event) => setAppForm({ ...appForm, summary: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.description')}</span>
                  <textarea value={appForm.description} onChange={(event) => setAppForm({ ...appForm, description: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.category')}</span>
                  <select value={appForm.categoryId} onChange={(event) => setAppForm({ ...appForm, categoryId: event.target.value })}>
                    <option value="">{t('common.uncategorized')}</option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>{category.name}</option>
                    ))}
                  </select>
                </label>
                <label>
                  <span>{t('common.tags')}</span>
                  <input value={appForm.tags} onChange={(event) => setAppForm({ ...appForm, tags: event.target.value })} />
                </label>
                <label>
                  <span>{t('drawer.installPassword')}</span>
                  <input
                    type="password"
                    autoComplete="new-password"
                    minLength={4}
                    maxLength={256}
                    value={appForm.installPassword}
                    onChange={(event) => setAppForm({ ...appForm, installPassword: event.target.value, clearInstallPassword: false })}
                  />
                  <small className="field-help">{app.installProtected ? t('drawer.installPasswordUpdateHelp') : t('drawer.installPasswordHelp')}</small>
                </label>
                {app.installProtected && (
                  <label className="toggle-line">
                    <input
                      type="checkbox"
                      checked={appForm.clearInstallPassword}
                      onChange={(event) => setAppForm({ ...appForm, clearInstallPassword: event.target.checked, installPassword: event.target.checked ? '' : appForm.installPassword })}
                    />
                    <span>{t('drawer.clearInstallPassword')}</span>
                  </label>
                )}
                <label className="toggle-line">
                  <input
                    type="checkbox"
                    checked={appForm.commentsEnabled}
                    onChange={(event) => setAppForm({ ...appForm, commentsEnabled: event.target.checked })}
                  />
                  <span>{t('drawer.commentsEnabled')}</span>
                </label>
                <label className="toggle-line">
                  <input
                    type="checkbox"
                    checked={appForm.allowUnreviewedUpdates}
                    onChange={(event) => setAppForm({ ...appForm, allowUnreviewedUpdates: event.target.checked })}
                  />
                  <span>{t('submitApp.allowUnreviewedUpdates')}</span>
                </label>
                <button type="submit" className="secondary-button">
                  <Save size={18} />
                  <span>{t('drawer.saveInfo')}</span>
                </button>
              </form>
            )}
            {canUploadVersion && (
              <form className="panel form-panel nested-panel" onSubmit={submitExternalVersion}>
                <SectionTitle icon={Link} title={t('drawer.publishVersion')} />
                <div className="workflow-strip">
                  <div>
                    <strong>{t('drawer.versionPublishPath')}</strong>
                    <span>{versionPublishesDirectly ? t('drawer.versionDirectHint') : t('drawer.versionReviewHint')}</span>
                  </div>
                  <div className="workflow-steps" aria-label={t('drawer.versionPublishPath')}>
                    <span>{t('common.version')}</span>
                    <ChevronRight size={14} />
                    <span>{t('submitApp.stepArtifact')}</span>
                    <ChevronRight size={14} />
                    <span>{versionPublishesDirectly ? t('submitApp.readinessDirect') : t('submitApp.stepReview')}</span>
                  </div>
                </div>
                <div className="submission-readiness" aria-label={t('drawer.versionReadiness')}>
                  <div className={cx('readiness-step', versionNumberReady && 'ready')}>
                    <span className={cx('status-badge', versionNumberReady ? 'approved' : 'unlisted')}>
                      {versionNumberReady ? <Check size={14} /> : <AlertCircle size={14} />}
                      {versionNumberReady ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
                    </span>
                    <strong>{t('drawer.readinessVersion')}</strong>
                    <small>{versionNumberReady ? t('drawer.readinessVersionReady') : t('drawer.readinessVersionMissing')}</small>
                  </div>
                  <div className={cx('readiness-step', versionArtifactReady && 'ready')}>
                    <span className={cx('status-badge', versionArtifactReady ? 'approved' : 'unlisted')}>
                      {versionArtifactReady ? <Check size={14} /> : <AlertCircle size={14} />}
                      {versionArtifactReady ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
                    </span>
                    <strong>{t('submitApp.readinessArtifact')}</strong>
                    <small>
                      {versionArtifactMode === 'local'
                        ? versionFile
                          ? t('submitApp.readinessArtifactLocalReady', { name: versionFile.name, size: formatBytes(versionFile.size) })
                          : t('submitApp.readinessArtifactLocalMissing')
                        : versionExternalArtifactReady
                          ? t('submitApp.readinessArtifactExternalReady')
                          : versionExternalDownloadReady || versionExternalChecksumReady
                            ? t('submitApp.readinessArtifactExternalPartial')
                            : t('submitApp.readinessArtifactExternalMissing')}
                    </small>
                  </div>
                  <div className="readiness-step ready">
                    <span className="status-badge synced">
                      <ShieldCheck size={14} />
                      {versionPublishesDirectly ? t('submitApp.readinessDirect') : t('submitApp.readinessQueued')}
                    </span>
                    <strong>{t('submitApp.readinessReview')}</strong>
                    <small>{versionPublishesDirectly ? t('drawer.readinessVersionDirect') : t('drawer.readinessVersionQueued')}</small>
                  </div>
                </div>
                <label>
                  <span>{t('common.version')}</span>
                  <input value={versionForm.version} onChange={(event) => setVersionForm({ ...versionForm, version: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.changelog')}</span>
                  <input value={versionForm.changelog} onChange={(event) => setVersionForm({ ...versionForm, changelog: event.target.value })} />
                </label>
                <div className="artifact-section">
                  <div className="artifact-section-head">
                    <strong>{t('submitApp.artifactMode')}</strong>
                    <span>{versionArtifactMode === 'local' ? t('drawer.versionLocalArtifactHint') : t('drawer.versionExternalArtifactHint')}</span>
                  </div>
                  <div className="artifact-mode" aria-label={t('submitApp.artifactMode')}>
                    <button type="button" className={cx(versionArtifactMode === 'local' && 'active')} onClick={() => selectVersionArtifactMode('local')}>
                      <Upload size={17} />
                      <span>
                        <strong>{t('submitApp.localArtifact')}</strong>
                        <small>{t('drawer.versionLocalArtifactHint')}</small>
                      </span>
                    </button>
                    <button type="button" className={cx(versionArtifactMode === 'external' && 'active')} onClick={() => selectVersionArtifactMode('external')}>
                      <Link size={17} />
                      <span>
                        <strong>{t('submitApp.externalArtifact')}</strong>
                        <small>{t('drawer.versionExternalArtifactHint')}</small>
                      </span>
                    </button>
                  </div>
                  {versionArtifactMode === 'local' ? (
                    <label>
                      <span>{t('common.lpkFile')}</span>
                      <input ref={versionFileInputRef} type="file" accept=".lpk" required onChange={(event) => setVersionFile(event.target.files?.[0] || null)} />
                      <small className="field-help">{t('drawer.versionLocalFileHelp')}</small>
                    </label>
                  ) : (
                    <div className="artifact-fields">
                      <p className="field-help">{t('submitApp.externalFieldsHelp')}</p>
                      <label>
                        <span>{t('submitApp.externalSource')}</span>
                        <select value={versionForm.sourceType} onChange={(event) => setVersionForm({ ...versionForm, sourceType: event.target.value })}>
                          <option value="GITHUB">GitHub Release</option>
                          <option value="WEBDAV">WebDAV URL</option>
                          <option value="S3">S3 URL</option>
                        </select>
                      </label>
                      <label>
                        <span>{t('submitApp.externalDownloadUrl')}</span>
                        <input
                          type="url"
                          required
                          value={versionForm.downloadUrl}
                          onChange={(event) => setVersionForm({ ...versionForm, downloadUrl: event.target.value })}
                        />
                        <small className="field-help">{t('submitApp.externalDownloadHelp')}</small>
                      </label>
                      <label>
                        <span>{t('common.sha256')}</span>
                        <input
                          required
                          maxLength={64}
                          pattern="[a-fA-F0-9]{64}"
                          title={t('submitApp.sha256Pattern')}
                          autoCapitalize="off"
                          autoCorrect="off"
                          spellCheck={false}
                          value={versionForm.sha256}
                          onChange={(event) => setVersionForm({ ...versionForm, sha256: event.target.value })}
                        />
                        <small className="field-help">{t('submitApp.sha256Help')}</small>
                      </label>
                    </div>
                  )}
                </div>
                {!canSubmitVersion && <p className="field-help">{t('drawer.versionSubmitBlocked')}</p>}
                <button type="submit" className="secondary-button" disabled={!canSubmitVersion}>
                  <Upload size={18} />
                  <span>{t('drawer.publishVersion')}</span>
                </button>
              </form>
            )}
            {canMaintain && (
              <section className="panel form-panel nested-panel">
                <SectionTitle icon={Users} title={t('drawer.visibilityGroups')} />
                <div className="checkbox-list">
                  {groups.length === 0 ? (
                    <span className="muted-text">{t('drawer.noGroupsPublic')}</span>
                  ) : (
                    groups.map((group) => (
                      <label className="toggle-line" key={group.id}>
                        <input
                          type="checkbox"
                          checked={visibility.includes(group.id)}
                          onChange={(event) =>
                            setVisibility((current) =>
                              event.target.checked ? [...current, group.id] : current.filter((id) => id !== group.id),
                            )
                          }
                        />
                        <span>{group.name}</span>
                      </label>
                    ))
                  )}
                </div>
                <button type="button" className="secondary-button" onClick={() => void saveVisibility()}>
                  <Users size={18} />
                  <span>{t('drawer.saveVisibility')}</span>
                </button>
              </section>
            )}
            {canMaintain && (
              <section className="panel nested-panel">
                <SectionTitle icon={Users} title={t('drawer.collaboratorRequests')} />
                <div className="review-list">
                  {collaboratorRequests.length === 0 ? (
                    <EmptyState icon={Users} title={t('drawer.noCollaboratorRequests')} />
                  ) : (
                    collaboratorRequests.map((request) => (
                      <div className="review-row" key={request.id}>
                        <div>
                          <strong>{request.username || t('drawer.userLabel', { id: request.user_id || request.userId || '-' })}</strong>
                          <span>{request.status} · {request.message || request.email || t('drawer.noMessage')}</span>
                        </div>
                        {request.status === 'PENDING' && (
                          <div className="row-actions">
                            <button
                              type="button"
                              className="icon-button ok"
                              aria-label={t('drawer.approveCollaboratorFor', { name: request.username || request.email || request.id })}
                              onClick={() => void decideCollaboratorRequest(request.id, true)}
                            >
                              <Check size={17} />
                            </button>
                            <button
                              type="button"
                              className="icon-button danger"
                              aria-label={t('drawer.rejectCollaboratorFor', { name: request.username || request.email || request.id })}
                              onClick={() => void decideCollaboratorRequest(request.id, false)}
                            >
                              <X size={17} />
                            </button>
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </section>
            )}
          </section>
        )}
        <section>
          <h3>{t('drawer.screenshots')}</h3>
          {(app.screenshots || []).length > 0 ? (
            <div className="screenshot-grid">
              {(app.screenshots || []).map((shot, index, shots) => (
                <figure className="screenshot-item" key={shot.id}>
                  <img src={shot.imageUrl} alt={shot.caption || app.name} />
                  {shot.caption && <figcaption>{shot.caption}</figcaption>}
                  {canMaintain && (
                    <div className="screenshot-actions">
                      <button type="button" className="icon-button" aria-label={t('drawer.moveScreenshotUp')} disabled={index === 0} onClick={() => void moveScreenshot(shot.id, -1)}>
                        <ArrowUp size={15} />
                      </button>
                      <button type="button" className="icon-button" aria-label={t('drawer.moveScreenshotDown')} disabled={index === shots.length - 1} onClick={() => void moveScreenshot(shot.id, 1)}>
                        <ArrowDown size={15} />
                      </button>
                      <button type="button" className="icon-button danger" aria-label={t('drawer.deleteScreenshot')} onClick={() => void deleteScreenshot(shot.id)}>
                        <Trash2 size={15} />
                      </button>
                    </div>
                  )}
                </figure>
              ))}
            </div>
          ) : (
            <EmptyState icon={Archive} title={t('drawer.noScreenshots')} />
          )}
          {canMaintain && (
            <form className="comment-form screenshot-form" onSubmit={uploadScreenshot}>
              <input value={screenshotCaption} onChange={(event) => setScreenshotCaption(event.target.value)} placeholder={t('drawer.screenshotCaption')} />
              <input type="file" accept=".png,.jpg,.jpeg,.webp" onChange={(event) => setScreenshotFile(event.target.files?.[0] || null)} />
              <button type="submit" className="icon-button" aria-label={t('drawer.uploadScreenshot')}><Upload size={17} /></button>
            </form>
          )}
        </section>
        <section>
          <h3>{t('drawer.versionHistory')}</h3>
          {(app.versions || []).length === 0 ? (
            <EmptyState icon={History} title={t('drawer.noVersions')} body={t('drawer.installBlocked')} />
          ) : (
            <div className="version-list">
              {(app.versions || []).map((version) => (
                <div className="version-row" key={version.id}>
                  <div>
                    <strong>{version.version}</strong>
                    <span>{version.sourceType} · {formatBytes(version.fileSize)} · {formatDate(version.publishedAt || version.createdAt)}</span>
                  </div>
                  <code>{shortSHA(version.sha256)}</code>
                </div>
              ))}
            </div>
          )}
        </section>
        <section>
          <h3>{t('drawer.comments')}</h3>
          {user && app.commentsEnabled && (
            <form className="comment-form" onSubmit={submitComment}>
              <input value={commentText} onChange={(event) => setCommentText(event.target.value)} placeholder={t('drawer.commentPlaceholder')} />
              <button type="submit" className="icon-button" aria-label={t('drawer.postComment')}><MessageSquare size={17} /></button>
            </form>
          )}
          {!app.commentsEnabled && <p className="inline-note">{t('drawer.commentsDisabled')}</p>}
          <div className="comments">
            {(app.comments || []).map((comment) => (
              <div className="comment" key={comment.id}>
                <div className="comment-head">
                  <strong>{comment.username}</strong>
                  {(canMaintain || user?.id === comment.userId) && (
                    <button type="button" className="icon-button danger" aria-label={t('drawer.deleteComment')} onClick={() => void deleteComment(comment.id)}>
                      <Trash2 size={15} />
                    </button>
                  )}
                </div>
                <p>{comment.body}</p>
              </div>
            ))}
          </div>
        </section>
      </article>
    </div>
  );
}

function AppGrid({
  apps,
  onOpen,
  onInstall,
  empty,
}: {
  apps: StoreApp[];
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  empty?: { title?: string; body?: string; action?: { label: string; icon?: typeof Home; onClick: () => void } };
}) {
  const { t } = useTranslation();
  if (apps.length === 0) {
    return <EmptyState icon={PackagePlus} title={empty?.title || t('common.noApps')} body={empty?.body} action={empty?.action} />;
  }
  return (
    <div className="app-grid">
      {apps.map((app) => {
        const installable = hasInstallableVersion(app);
        const hasChecksum = Boolean(app.latestVersion?.sha256);
        return (
          <article className="app-card" key={app.id}>
            <button type="button" className="app-open" onClick={() => void onOpen(app)} aria-label={t('app.open', { name: app.name })}>
              <AvatarIcon seed={app.slug || app.name} title={app.name} />
              <div>
                <h3>{app.name}</h3>
                <p>{app.summary || app.description || t('common.lpkApp')}</p>
              </div>
              <ChevronRight size={18} />
            </button>
            <div className="app-meta">
              <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
              <span><Star size={14} /> {app.latestVersion?.version || t('app.noPublishedVersion')}</span>
              <span><Download size={14} /> {t('app.downloads', { count: app.downloadCount })}</span>
              {app.latestVersion?.sourceType && <span><Link size={14} /> {t('app.sourceType', { type: app.latestVersion.sourceType })}</span>}
            </div>
            <div className="app-readiness" aria-label={t('app.installSignals')}>
              <span className={cx('status-badge', installable ? 'approved' : 'blocked')}>
                <Download size={13} />
                {installable ? t('app.installReady') : t('app.installMissingVersion')}
              </span>
              <span className={cx('status-badge', hasChecksum ? 'synced' : 'unsynced')}>
                <ShieldCheck size={13} />
                {hasChecksum ? t('app.checksumReady') : t('app.checksumMissing')}
              </span>
              {app.installProtected && (
                <span className="status-badge pending">
                  <KeyRound size={13} />
                  {t('app.installPasswordRequired')}
                </span>
              )}
              {app.status === 'APPROVED' && (
                <span className="status-badge approved">
                  <Check size={13} />
                  {t('app.reviewed')}
                </span>
              )}
            </div>
            <button
              type="button"
              className="install-button"
              disabled={!installable}
              onClick={() => void onInstall(app)}
              aria-label={installable ? t('app.install', { name: app.name }) : t('app.installUnavailable', { name: app.name })}
            >
              <Download size={17} />
              <span>{installable ? t('common.install') : t('common.unavailable')}</span>
            </button>
          </article>
        );
      })}
    </div>
  );
}

function SectionTitle({ icon: Icon, title }: { icon: typeof Home; title: string }) {
  return (
    <div className="section-title">
      <Icon size={19} />
      <h2>{title}</h2>
    </div>
  );
}

function EmptyState({
  icon: Icon,
  title,
  body,
  action,
}: {
  icon: typeof Home;
  title: string;
  body?: string;
  action?: { label: string; icon?: typeof Home; onClick: () => void };
}) {
  const ActionIcon = action?.icon;
  return (
    <div className="empty-state">
      <Icon size={28} />
      <strong>{title}</strong>
      {body && <p>{body}</p>}
      {action && (
        <button type="button" className="secondary-button" onClick={action.onClick}>
          {ActionIcon && <ActionIcon size={18} />}
          <span>{action.label}</span>
        </button>
      )}
    </div>
  );
}

function MobileTabs({ tab, setTab, items, inert }: { tab: TabKey; setTab: (tab: TabKey) => void; items: readonly NavItem[]; inert?: boolean }) {
  const { t } = useTranslation();
  return (
    <nav className="mobile-tabs" inert={inert} aria-hidden={inert ? true : undefined} style={{ gridTemplateColumns: `repeat(${items.length}, minmax(0, 1fr))` }}>
      {items.map((item) => {
        const Icon = item.icon;
        return (
          <button type="button" key={item.key} className={cx(tab === item.key && 'active')} onClick={() => setTab(item.key)} aria-label={t(item.labelKey)}>
            <Icon size={20} />
            <span>{t(item.labelKey)}</span>
          </button>
        );
      })}
    </nav>
  );
}
