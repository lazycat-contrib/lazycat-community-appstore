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
  Pencil,
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
import { Theme } from '@astryxdesign/core/theme';
import { neutralTheme } from '@astryxdesign/theme-neutral/built';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import i18n from './i18n';
import { API_BASE, DEFAULT_SOURCE_NAME, DEFAULT_SOURCE_URL, HAS_API } from './config';
import { api, clientApi } from './shared/api';
import {
  ANNOUNCEMENT_DISMISS_STORAGE_KEY,
  ANNOUNCEMENT_NOTIFY_STORAGE_KEY,
  RECOMMENDED_DOWNLOAD_MIRRORS,
  RECOMMENDED_RAW_MIRRORS,
  THEME_STORAGE_KEY,
  mirrorPresetText,
} from './shared/constants';
import { readSystemTheme, readThemeMode, ThemeToggle } from './shared/theme';
import {
  applicableMirrorsForVersion,
  arrayOrEmpty,
  belongsToSource,
  cx,
  defaultMirrorIDForVersion,
  defaultSiteProfile,
  errorMessage,
  findInstalledApplication,
  formatBytes,
  formatDate,
  githubMirrorKindForURL,
  hasInstallableVersion,
  isSourceAppUpdateAvailable,
  isSourceStale,
  localizedName,
  runAction,
  selectedSourceVersion,
  shortSHA,
  sourceActionLabel,
  sourceForApp,
  sourceInstallAction,
  statusKey,
  reviewKindKey,
  stripTrailingSlash,
  withInstallPassword,
} from './shared/utils';
import { AppIcon, AvatarIcon } from './components/AppIcon';
import { FilePicker } from './shared/components/FilePicker';
import { ClientHistoryView } from './modules/client/ClientHistoryView';
import { ClientCatalog } from './modules/client/ClientCatalog';
import { InstalledAppsView } from './modules/client/InstalledAppsView';
import { ClientSettingsView } from './modules/client/ClientSettingsView';
import { SourcesView as ClientSourcesView } from './modules/client/SourcesView';
import { CollectionAppPicker } from './modules/admin/CollectionAppPicker';
import { AppGrid } from './modules/storefront/AppGrid';
import { StorefrontHome } from './modules/storefront/StorefrontHome';
import { StorefrontSearch } from './modules/storefront/StorefrontSearch';

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
  packageId?: string;
  categoryId?: number;
  name: string;
  slug: string;
  summary: string;
  description: string;
  iconUrl?: string;
  status: string;
  category?: string;
  tags: string[];
  visibleGroupIds: number[];
  allowUnreviewedUpdates: boolean;
  commentsEnabled: boolean;
  emailNotificationsEnabled: boolean;
  installProtected: boolean;
  downloadCount: number;
  latestVersion?: Version;
  versions?: Version[];
  screenshots?: Screenshot[];
  comments?: Comment[];
  favorites?: number;
  outdatedMarks?: number;
  outdatedMarked?: boolean;
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
  parentId?: number;
  authorType?: 'USER' | 'CLIENT' | string;
  clientUserId?: string;
  username: string;
  body: string;
  canDelete?: boolean;
  replies?: Comment[];
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
  nameI18n?: Record<string, string>;
  slug: string;
  sortOrder?: number;
};

type TagRecord = {
  id: number;
  name: string;
  nameI18n?: Record<string, string>;
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

type GitHubMirror = {
  id: string;
  kind: 'download' | 'raw';
  name: string;
  url: string;
};

type SourceSubscription = {
  id: SourceID;
  name: string;
  url: string;
  password: string;
  defaultDownloadMirrorId: string;
  defaultRawMirrorId: string;
  githubMirrors: GitHubMirror[];
  lastSync?: string;
  lastError?: string;
  lastErrorCode?: SourceErrorCode;
  lastAppCount?: number;
  lastInstallableCount?: number;
};

type SourceInput = Pick<SourceSubscription, 'name' | 'url' | 'password' | 'defaultDownloadMirrorId' | 'defaultRawMirrorId'>;

type ClientSettings = {
  commentDisplayName: string;
  autoSyncEnabled: boolean;
  autoSyncIntervalMinutes: number;
  syncOnStartup: boolean;
  lastAutoSyncAt?: string;
  lastAutoSyncStatus?: string;
  lastAutoSyncError?: string;
};

type CommentNotification = {
  id: number;
  appId: number;
  commentId: number;
  appName: string;
  actorName: string;
  body: string;
  read: boolean;
  createdAt: string;
};

type SourceVersion = {
  version: string;
  downloadUrl: string;
  upstreamDownloadUrl?: string;
  sourceType?: string;
  sha256: string;
  size: number;
};

type SourceApp = {
  id: number;
  sourceId?: SourceID;
  sourceName: string;
  externalId?: string;
  packageId?: string;
  name: string;
  slug: string;
  summary: string;
  category?: string;
  iconUrl?: string;
  installProtected?: boolean;
  latestVersion?: SourceVersion;
  versions?: SourceVersion[];
};

type FavoriteData = {
  apps: StoreApp[];
  submitters: User[];
};

type SetupStatus = {
  needsSetup: boolean;
};

type SiteAnnouncement = {
  enabled: boolean;
  level: 'info' | 'warning' | 'success';
  title?: string;
  body?: string;
  linkLabel?: string;
  linkUrl?: string;
  updatedAt?: string;
};

type SiteProfile = {
  title: string;
  iconUrl?: string;
  publicUrl: string;
  sourceUrl: string;
  announcement: SiteAnnouncement;
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
  version?: string;
};

type InstallOptions = {
  installPassword?: string;
  version?: string;
  mirrorId?: string;
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

type InstallHistoryEntry = {
  id: number;
  sourceId?: SourceID;
  sourceAppId?: number;
  sourceName?: string;
  packageId: string;
  appName: string;
  version?: string;
  result: 'SUCCESS' | 'FAILED';
  downloadUrl?: string;
  sha256?: string;
  error?: string;
  createdAt: string;
};

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

type TabKey = 'home' | 'search' | 'sources' | 'profile' | 'history' | 'settings' | 'admin';
type NavItem = { key: TabKey; labelKey: string; icon: typeof Home };
type ThemeMode = 'system' | 'light' | 'dark';
type ResolvedTheme = Exclude<ThemeMode, 'system'>;

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
  { key: 'history', labelKey: 'nav.history', icon: History },
  { key: 'settings', labelKey: 'nav.settings', icon: Settings },
];

type SortMode = 'recent' | 'downloads' | 'name';
type CollectionDraft = { name: string; slug: string; kind: string; appIds: number[] };

function verificationTokenFromURL() {
  return new URLSearchParams(window.location.search).get('token') || '';
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
  const [clientSettings, setClientSettings] = useState<ClientSettings>({
    commentDisplayName: '',
    autoSyncEnabled: false,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
  });
  const [installedApps, setInstalledApps] = useState<InstalledApplication[]>([]);
  const [installHistory, setInstallHistory] = useState<InstallHistoryEntry[]>([]);
  const [installedState, setInstalledState] = useState<'idle' | 'loading' | 'loaded' | 'error'>('idle');
  const [installedError, setInstalledError] = useState('');
  const [installActivity, setInstallActivity] = useState<InstallActivity | null>(null);
  const [installPasswordRequest, setInstallPasswordRequest] = useState<InstallPasswordRequest | null>(null);
  const [siteProfile, setSiteProfile] = useState<SiteProfile>(() => defaultSiteProfile(t('appName')));
  const [dismissedAnnouncement, setDismissedAnnouncement] = useState(() => {
    try {
      return localStorage.getItem(ANNOUNCEMENT_DISMISS_STORAGE_KEY) || '';
    } catch {
      return '';
    }
  });
  const [toast, setToast] = useState<Toast | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);
  const defaultSourceCheckedRef = useRef(false);
  const canReview = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const serverNavItems = user ? [...serverBaseTabs, ...(canReview ? [serverAdminTab] : [])] : serverBaseTabs.filter((item) => item.key !== 'profile');
  const navItems = HAS_API ? serverNavItems : clientTabs;
  const siteTitle = HAS_API ? siteProfile.title : t('appName');
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';
  const drawerOpen = Boolean(selectedApp || (!HAS_API && selectedSourceApp));
  const resolvedTheme: ResolvedTheme = themeMode === 'system' ? systemTheme : themeMode;
  const announcementKey =
    siteProfile.announcement.updatedAt ||
    `${siteProfile.announcement.level}:${siteProfile.announcement.title || ''}:${siteProfile.announcement.body || ''}`;
  const showAnnouncement =
    HAS_API &&
    siteProfile.announcement.enabled &&
    Boolean(siteProfile.announcement.title || siteProfile.announcement.body) &&
    announcementKey !== dismissedAnnouncement;

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
    if (HAS_API && !user && tab === 'profile') {
      return;
    }
    if (!navItems.some((item) => item.key === tab)) {
      setTab(navItems[0].key);
    }
  }, [navItems, tab, user]);

  useEffect(() => {
    document.getElementById('main-content')?.focus({ preventScroll: true });
  }, [tab]);

  useEffect(() => {
    document.documentElement.lang = currentLanguage === 'en' ? 'en' : 'zh-CN';
    document.title = siteTitle;
  }, [currentLanguage, siteTitle]);

  useEffect(() => {
    if (!siteProfile.iconUrl) return;
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!link) {
      link = document.createElement('link');
      link.rel = 'icon';
      document.head.appendChild(link);
    }
    link.href = siteProfile.iconUrl;
  }, [siteProfile.iconUrl]);

  useEffect(() => {
    if (!showAnnouncement || !announcementKey) return;
    try {
      if (localStorage.getItem(ANNOUNCEMENT_NOTIFY_STORAGE_KEY) === announcementKey) return;
      localStorage.setItem(ANNOUNCEMENT_NOTIFY_STORAGE_KEY, announcementKey);
    } catch {
      // Notification still works for this session when storage is blocked.
    }
    setToast({ tone: 'neutral', message: siteProfile.announcement.title || t('site.newAnnouncement') });
  }, [announcementKey, showAnnouncement, siteProfile.announcement.title, t]);

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
      const [siteData, me, appData, categoryData, collectionData] = await Promise.allSettled([
        api<{ site: SiteProfile }>('/api/v1/site/profile'),
        api<{ user: User }>('/api/v1/auth/me'),
        api<{ apps: StoreApp[] }>('/api/v1/apps'),
        api<{ categories: Category[] }>('/api/v1/categories'),
        api<{ collections: Collection[] }>('/api/v1/collections'),
      ]);
      if (siteData.status === 'fulfilled') setSiteProfile(siteData.value.site);
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

  async function loadSiteProfile() {
    if (!HAS_API) return;
    const data = await api<{ site: SiteProfile }>('/api/v1/site/profile');
    setSiteProfile(data.site);
  }

  async function loadClientSources() {
    const data = await clientApi<{ sources: SourceSubscription[] }>('/sources');
    const nextSources = arrayOrEmpty(data.sources);
    if (!defaultSourceCheckedRef.current && nextSources.length === 0 && DEFAULT_SOURCE_URL) {
      defaultSourceCheckedRef.current = true;
      const created = await clientApi<{ source: SourceSubscription }>('/sources', {
        method: 'POST',
        body: JSON.stringify({ name: DEFAULT_SOURCE_NAME, url: DEFAULT_SOURCE_URL, password: '', defaultDownloadMirrorId: '', defaultRawMirrorId: '' }),
      });
      setSources([created.source]);
      return [created.source];
    }
    defaultSourceCheckedRef.current = true;
    setSources(nextSources);
    return nextSources;
  }

  async function loadClientApps() {
    const data = await clientApi<{ apps: SourceApp[] }>('/apps');
    const nextApps = arrayOrEmpty(data.apps);
    setSourceApps(nextApps);
    return nextApps;
  }

  async function loadClientSettings() {
    const data = await clientApi<{ settings: ClientSettings }>('/settings');
    const nextSettings = data.settings || { commentDisplayName: '', autoSyncEnabled: false, autoSyncIntervalMinutes: 60, syncOnStartup: false };
    setClientSettings(nextSettings);
    return nextSettings;
  }

  async function loadInstallHistory() {
    const data = await clientApi<{ history: InstallHistoryEntry[] }>('/history');
    const nextHistory = arrayOrEmpty(data.history);
    setInstallHistory(nextHistory);
    return nextHistory;
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
      await Promise.all([loadClientSources(), loadClientApps(), loadClientSettings(), loadInstallHistory()]);
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

  async function installApp(app: StoreApp | SourceApp, options: InstallOptions = {}) {
    const isSourceApp = 'sourceName' in app;
    const version = isSourceApp ? selectedSourceVersion(app, options.version) : app.latestVersion;
    if (!version) {
      setToast({ tone: 'error', message: t('toast.noInstallableVersion') });
      return;
    }
    const source = isSourceApp ? sourceForApp(app, sources) : undefined;
    const availableMirrors = isSourceApp ? applicableMirrorsForVersion(source, version) : [];
    const needsPassword = app.installProtected && !options.installPassword;
    const needsMirrorChoice = isSourceApp && availableMirrors.length > 0 && !Object.prototype.hasOwnProperty.call(options, 'mirrorId');
    if (needsPassword || needsMirrorChoice) {
      setInstallPasswordRequest({ app, version: version.version });
      return;
    }
    await runAction(setToast, t('toast.installFailed'), async () => {
      const source = isSourceApp ? app.sourceName : t('search.localStore');
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
        isSourceApp
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
      if (result.mode === 'lazycat-go-sdk') {
        void loadInstalledApps({ quiet: true });
        void loadInstallHistory();
      }
      setToast({
        tone: success ? 'success' : 'error',
        message: t(result.messageKey, result.messageParams),
      });
    });
    if (isSourceApp) void loadInstallHistory();
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
        defaultDownloadMirrorId: source.defaultDownloadMirrorId || '',
        defaultRawMirrorId: source.defaultRawMirrorId || '',
      }),
    });
    await refreshClientData({ silent: true });
  }

  async function deleteClientSource(source: SourceSubscription) {
    await clientApi(`/sources/${source.id}`, { method: 'DELETE' });
    setSelectedSourceApp((current) => (current && belongsToSource(current, source) ? null : current));
    await refreshClientData({ silent: true });
  }

  async function saveClientSettings(nextSettings: ClientSettings) {
    const data = await clientApi<{ settings: ClientSettings }>('/settings', {
      method: 'PATCH',
      body: JSON.stringify(nextSettings),
    });
    setClientSettings(data.settings || nextSettings);
  }

  if (HAS_API && setupRequired) {
    return (
      <Theme theme={neutralTheme} mode={resolvedTheme}>
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
      </Theme>
    );
  }

  return (
    <Theme theme={neutralTheme} mode={resolvedTheme}>
    <div className="shell">
      <a className="skip-link" href="#main-content" inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>{t('common.skipToMain')}</a>
      <aside className="sidebar" inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>
        <div className="brand">
          <div className="brand-mark">
            {HAS_API && siteProfile.iconUrl ? <img src={siteProfile.iconUrl} alt="" /> : <Archive size={22} />}
          </div>
          <div>
            <strong>{siteTitle}</strong>
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
            {showAnnouncement && (
              <AnnouncementBanner
                announcement={siteProfile.announcement}
                onDismiss={() => {
                  setDismissedAnnouncement(announcementKey);
                  try {
                    localStorage.setItem(ANNOUNCEMENT_DISMISS_STORAGE_KEY, announcementKey);
                  } catch {
                    // Dismissal still applies for this session when storage is blocked.
                  }
                }}
              />
            )}
            {tab === 'home' && (
              <StorefrontHome
                apps={filteredApps}
                categories={categories}
                collections={collections}
                siteProfile={siteProfile}
                onOpen={openApp}
                onInstall={installApp}
                onNavigate={setTab}
                onCategory={(category) => {
                  setActiveCategory(category);
                  setTab('search');
                }}
                setToast={setToast}
                isAuthenticated={Boolean(user)}
              />
            )}
            {tab === 'search' && (
              <SearchView
                apps={filteredApps}
                sourceApps={sourceApps}
                sources={sources}
                categories={categories}
                submitters={submitters}
                activeCategory={activeCategory}
                activeSubmitter={activeSubmitter}
                sortMode={sortMode}
                query={query}
                mode={HAS_API ? 'server' : 'client'}
                sourceStats={sourceStats}
                installedApps={HAS_API ? [] : installedApps}
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
              <ClientSourcesView
                sources={sources}
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
                sourceApps={sourceApps}
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
            {tab === 'history' && !HAS_API && (
              <ClientHistoryView
                history={installHistory}
                sourceApps={sourceApps}
                onRefresh={() => void loadInstallHistory()}
                onOpenSource={setSelectedSourceApp}
              />
            )}
            {tab === 'settings' && !HAS_API && (
              <ClientSettingsView
                settings={clientSettings}
                sourceStats={sourceStats}
                onSave={saveClientSettings}
                setToast={setToast}
              />
            )}
            {tab === 'admin' && (
              user && canReview ? (
                <AdminPanel user={user} reviews={reviews} onApprove={approveReview} onSiteProfileSaved={loadSiteProfile} setToast={setToast} />
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

      {!HAS_API && selectedSourceApp && (
        <SourceAppDrawer
          app={selectedSourceApp}
          installedMatch={findInstalledApplication(selectedSourceApp, installedApps)}
          installedState={installedState}
          onClose={() => setSelectedSourceApp(null)}
          onInstall={installApp}
          onLoadInstalled={loadInstalledApps}
          onRefreshSourceApp={async () => {
            const data = await clientApi<{ app: SourceApp }>(`/apps/${selectedSourceApp.id}`);
            setSelectedSourceApp(data.app);
          }}
        />
      )}

      {installPasswordRequest && (
        <InstallOptionsDialog
          app={installPasswordRequest.app}
          source={'sourceName' in installPasswordRequest.app ? sourceForApp(installPasswordRequest.app, sources) : undefined}
          version={
            'sourceName' in installPasswordRequest.app
              ? selectedSourceVersion(installPasswordRequest.app, installPasswordRequest.version)
              : installPasswordRequest.app.latestVersion
          }
          onCancel={() => setInstallPasswordRequest(null)}
          onSubmit={(options) => {
            const target = installPasswordRequest.app;
            const targetVersion = installPasswordRequest.version;
            setInstallPasswordRequest(null);
            void installApp(target, { ...options, version: targetVersion });
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
    </Theme>
  );
}

function InstallOptionsDialog({
  app,
  source,
  version,
  onCancel,
  onSubmit,
}: {
  app: StoreApp | SourceApp;
  source?: SourceSubscription;
  version?: Version | SourceVersion;
  onCancel: () => void;
  onSubmit: (options: { installPassword?: string; mirrorId?: string }) => void;
}) {
  const { t } = useTranslation();
  const [password, setPassword] = useState('');
  const [mirrorId, setMirrorId] = useState(() => defaultMirrorIDForVersion(source, version) || '');
  const [error, setError] = useState('');
  const dialogTitleId = `install-password-title-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const dialogBodyId = `install-password-body-${'sourceName' in app ? 'source' : 'store'}-${app.id}`;
  const requiresPassword = app.installProtected;
  const mirrorOptions = applicableMirrorsForVersion(source, version);
  const mirrorKind = githubMirrorKindForURL(version && 'upstreamDownloadUrl' in version ? version.upstreamDownloadUrl || version.downloadUrl : version?.downloadUrl);

  useEffect(() => {
    setMirrorId(defaultMirrorIDForVersion(source, version) || '');
  }, [source?.id, version?.version]);

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
    if (requiresPassword && !value) {
      setError(t('installPassword.required'));
      return;
    }
    onSubmit({
      installPassword: requiresPassword ? value : undefined,
      mirrorId: mirrorOptions.length > 0 ? mirrorId : undefined,
    });
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
            {requiresPassword ? <KeyRound size={21} /> : <Download size={21} />}
          </span>
          <div>
            <h2 id={dialogTitleId}>{t(mirrorOptions.length > 0 ? 'installOptions.title' : 'installPassword.title')}</h2>
            <p id={dialogBodyId}>
              {requiresPassword
                ? t('installPassword.body', { name: app.name })
                : t('installOptions.body', { name: app.name })}
            </p>
          </div>
        </div>
        {requiresPassword && (
          <XTextInput
            type="password"
            label={t('installPassword.label')}
            value={password}
            hasAutoFocus
            onChange={(value) => {
              setPassword(value);
              if (error) setError('');
            }}
          />
        )}
        {mirrorOptions.length > 0 && (
          <XSelector
            label={t('installOptions.mirror')}
            description={t(mirrorKind === 'raw' ? 'installOptions.rawMirrorHelp' : 'installOptions.downloadMirrorHelp')}
            value={mirrorId}
            options={[
              { value: '', label: t('installOptions.direct') },
              ...mirrorOptions.map((entry) => ({ value: entry.id, label: entry.name })),
            ]}
            onChange={setMirrorId}
          />
        )}
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
    githubDownloadMirrors: '',
    githubRawMirrors: '',
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
          githubDownloadMirrors: form.githubDownloadMirrors,
          githubRawMirrors: form.githubRawMirrors,
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
          <XTextInput label={t('common.username')} value={form.username} onChange={(value) => setForm({ ...form, username: value })} />
          <XTextInput type="email" label={t('common.email')} value={form.email} onChange={(value) => setForm({ ...form, email: value })} />
          <XTextInput type="password" label={t('common.password')} value={form.password} onChange={(value) => setForm({ ...form, password: value })} />
          <XTextInput type="password" label={t('setup.confirmPassword')} value={form.confirmPassword} onChange={(value) => setForm({ ...form, confirmPassword: value })} />
          <label className="toggle-line">
            <input
              type="checkbox"
              checked={form.sourcePasswordEnabled}
              onChange={(event) => setForm({ ...form, sourcePasswordEnabled: event.target.checked })}
            />
            <span>{t('setup.protectSource')}</span>
          </label>
          {form.sourcePasswordEnabled && (
            <XTextInput type="password" label={t('sources.password')} value={form.sourcePassword} onChange={(value) => setForm({ ...form, sourcePassword: value })} />
          )}
          <XTextArea
            label={t('admin.settings.githubDownloadMirrors')}
            description={t('admin.settingsHelp.githubDownloadMirrors')}
            value={form.githubDownloadMirrors}
            rows={3}
            onChange={(value) => setForm({ ...form, githubDownloadMirrors: value })}
          />
          <XTextArea
            label={t('admin.settings.githubRawMirrors')}
            description={t('admin.settingsHelp.githubRawMirrors')}
            value={form.githubRawMirrors}
            rows={3}
            onChange={(value) => setForm({ ...form, githubRawMirrors: value })}
          />
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

function AnnouncementBanner({ announcement, onDismiss }: { announcement: SiteAnnouncement; onDismiss?: () => void }) {
  const { t } = useTranslation();
  const tone = announcement.level || 'info';
  return (
    <section className={cx('announcement-banner', tone)} aria-live="polite">
      <div>
        <span className="status-badge synced">{t(`site.announcementLevels.${tone}`)}</span>
        {announcement.title && <strong>{announcement.title}</strong>}
        {announcement.body && <p>{announcement.body}</p>}
      </div>
      <div className="announcement-actions">
        {announcement.linkUrl && (
          <a className="secondary-button compact-button" href={announcement.linkUrl} target="_blank" rel="noreferrer">
            <Link size={16} />
            <span>{announcement.linkLabel || t('site.announcementLink')}</span>
          </a>
        )}
        {onDismiss && (
          <button type="button" className="icon-button" aria-label={t('site.dismissAnnouncement')} onClick={onDismiss}>
            <X size={17} />
          </button>
        )}
      </div>
    </section>
  );
}

function SearchView({
  apps,
  sourceApps,
  sources,
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
  sources: SourceSubscription[];
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
  onInstall: (app: StoreApp | SourceApp, options?: InstallOptions) => void | Promise<void>;
  onGoSources: () => void;
}) {
  const { t } = useTranslation();

  if (mode === 'client') {
    return (
      <ClientCatalog
        sourceApps={sourceApps}
        sources={sources}
        query={query}
        sourceStats={sourceStats}
        installedApps={installedApps}
        onOpenSource={onOpenSource}
        onInstall={onInstall}
        onGoSources={onGoSources}
      />
    );
  }

  return (
    <StorefrontSearch
      apps={apps}
      categories={categories}
      submitters={submitters}
      activeCategory={activeCategory}
      activeSubmitter={activeSubmitter}
      sortMode={sortMode}
      onCategory={onCategory}
      onSubmitter={onSubmitter}
      onSortMode={onSortMode}
      onOpen={onOpen}
      onInstall={onInstall}
    />
  );
}


function SourceAppDrawer({
  app,
  installedMatch,
  installedState,
  onClose,
  onInstall,
  onLoadInstalled,
  onRefreshSourceApp,
}: {
  app: SourceApp;
  installedMatch?: InstalledApplication;
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  onClose: () => void;
  onInstall: (app: SourceApp, options?: { version?: string }) => void;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onRefreshSourceApp: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const drawerTitleId = `source-app-drawer-title-${app.sourceId || app.sourceName}-${app.id}`;
  const [comments, setComments] = useState<Comment[]>([]);
  const [commentsState, setCommentsState] = useState<'idle' | 'loading' | 'loaded' | 'error'>('idle');
  const [commentText, setCommentText] = useState('');
  const [replyTarget, setReplyTarget] = useState<number | null>(null);
  const [replyText, setReplyText] = useState('');
  const latestVersion = app.latestVersion;
  const sourceVersions = app.versions && app.versions.length > 0 ? app.versions : latestVersion ? [latestVersion] : [];
  const installable = hasInstallableVersion(app);
  const installAction = sourceInstallAction(app, installedMatch);
  const isUpdateAvailable = installAction === 'update';
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
    void loadSourceComments();
    setCommentText('');
    setReplyTarget(null);
    setReplyText('');
  }, [app.id]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  async function loadSourceComments() {
    setCommentsState('loading');
    try {
      const data = await clientApi<{ comments: Comment[] }>(`/apps/${app.id}/comments`);
      setComments(arrayOrEmpty(data.comments));
      setCommentsState('loaded');
    } catch {
      setComments([]);
      setCommentsState('error');
    }
  }

  async function submitSourceComment(event: FormEvent, parentId?: number) {
    event.preventDefault();
    const body = (parentId ? replyText : commentText).trim();
    if (!body) return;
    try {
      await clientApi(`/apps/${app.id}/comments`, {
        method: 'POST',
        body: JSON.stringify({ body, parentId }),
      });
      if (parentId) {
        setReplyText('');
        setReplyTarget(null);
      } else {
        setCommentText('');
      }
      await loadSourceComments();
      await onRefreshSourceApp();
    } catch (error) {
      console.error(error);
      setCommentsState('error');
    }
  }

  async function deleteSourceComment(commentId: number) {
    await clientApi(`/apps/${app.id}/comments/${commentId}`, { method: 'DELETE' });
    await loadSourceComments();
  }

  return (
    <div className="detail-page-backdrop" onClick={onClose}>
      <article
        className="detail-page"
        role="dialog"
        aria-modal="true"
        aria-labelledby={drawerTitleId}
        onClick={(event) => event.stopPropagation()}
      >
        <button ref={closeButtonRef} type="button" className="icon-button close" aria-label={t('common.close')} onClick={onClose}>
          <X size={17} />
        </button>
        <header className="detail-head">
          <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} className="detail-avatar" />
          <div>
            <span className="eyebrow subtle">{t('sourceDetail.eyebrow')}</span>
            <h2 id={drawerTitleId}>{app.name}</h2>
            <p>{app.summary || t('common.lpkApp')}</p>
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
              <span><Star size={14} /> {latestVersion?.version || t('app.noPublishedVersion')}</span>
              {installedMatch && (
                <span className={cx('status-badge', isUpdateAvailable ? 'pending' : 'synced')}>
                  {isUpdateAvailable ? <RefreshCw size={13} /> : <Check size={13} />}
                  {isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
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
            <button type="button" className={cx('install-button', isUpdateAvailable && 'update-available')} disabled={!installable} onClick={() => void onInstall(app)}>
              {isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
              <span>{sourceActionLabel(t, installAction)}</span>
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

        <section className="source-version-panel">
          <div className="section-title">
            <History size={19} />
            <h3>{t('sourceDetail.availableVersions')}</h3>
          </div>
          {sourceVersions.length === 0 ? (
            <EmptyState icon={History} title={t('sourceDetail.noVersions')} body={t('sourceDetail.noVersionsBody')} />
          ) : (
            <div className="source-version-list">
              {sourceVersions.map((version) => {
                const isLatest = version.version === latestVersion?.version;
                const canInstallVersion = Boolean(version.downloadUrl);
                return (
                  <div className="source-version-row" key={`${version.version}-${version.downloadUrl}`}>
                    <div>
                      <strong>{version.version}</strong>
                      <span>{version.sourceType || t('drawer.sourceMissing')} · {version.size ? formatBytes(version.size) : t('drawer.sizeMissing')} · {shortSHA(version.sha256)}</span>
                    </div>
                    <div className="row-actions">
                      <span className={cx('status-badge', isLatest ? 'approved' : 'pending')}>
                        {isLatest ? t('sourceDetail.latest') : t('sourceDetail.rollbackCandidate')}
                      </span>
                      <button
                        type="button"
                        className="secondary-button compact-button"
                        disabled={!canInstallVersion}
                        onClick={() => void onInstall(app, { version: version.version })}
                      >
                        <Download size={17} />
                        <span>{isLatest ? t('common.install') : t('sourceDetail.rollback')}</span>
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
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

        <section className="comment-section">
          <div className="section-title with-action">
            <div>
              <MessageSquare size={19} />
              <h3>{t('drawer.comments')}</h3>
            </div>
            <button type="button" className="secondary-button compact-button" onClick={() => void loadSourceComments()}>
              <RefreshCw size={17} />
              <span>{t('common.refresh')}</span>
            </button>
          </div>
          {commentsState === 'error' && <p className="inline-warning"><AlertCircle size={15} /><span>{t('sourceDetail.commentsUnavailable')}</span></p>}
          <form className="comment-form rich-comment-form" onSubmit={(event) => void submitSourceComment(event)}>
            <XTextInput
              label={t('drawer.commentPlaceholder')}
              isLabelHidden
              value={commentText}
              placeholder={t('drawer.commentPlaceholder')}
              onChange={setCommentText}
            />
            <button type="submit" className="icon-button" aria-label={t('drawer.postComment')} disabled={!commentText.trim()}>
              <MessageSquare size={17} />
            </button>
          </form>
          <CommentList
            comments={comments}
            commentsState={commentsState}
            replyTarget={replyTarget}
            replyText={replyText}
            onReplyTarget={setReplyTarget}
            onReplyText={setReplyText}
            onReply={(event, parentId) => void submitSourceComment(event, parentId)}
            onDelete={(commentId) => void deleteSourceComment(commentId)}
          />
        </section>
      </article>
    </div>
  );
}

function CommentList({
  comments,
  commentsState = 'loaded',
  replyTarget,
  replyText,
  onReplyTarget,
  onReplyText,
  onReply,
  onDelete,
}: {
  comments: Comment[];
  commentsState?: 'idle' | 'loading' | 'loaded' | 'error';
  replyTarget: number | null;
  replyText: string;
  onReplyTarget: (id: number | null) => void;
  onReplyText: (value: string) => void;
  onReply: (event: FormEvent, parentId: number) => void;
  onDelete: (id: number) => void;
}) {
  const { t } = useTranslation();
  if (commentsState === 'loading') {
    return (
      <div className="comments">
        <div className="comment skeleton-comment" aria-label={t('common.loading')} />
      </div>
    );
  }
  if (comments.length === 0) {
    return <EmptyState icon={MessageSquare} title={t('drawer.noComments')} body={t('drawer.noCommentsBody')} />;
  }
  return (
    <div className="comments">
      {comments.map((comment) => (
        <article className="comment" key={comment.id}>
          <CommentBody comment={comment} onDelete={onDelete} />
          <div className="comment-actions">
            <button type="button" className="secondary-button compact-button" onClick={() => onReplyTarget(replyTarget === comment.id ? null : comment.id)}>
              <MessageSquare size={15} />
              <span>{t('drawer.reply')}</span>
            </button>
          </div>
          {replyTarget === comment.id && (
            <form className="comment-form rich-comment-form reply-form" onSubmit={(event) => onReply(event, comment.id)}>
              <XTextInput
                label={t('drawer.replyPlaceholder')}
                isLabelHidden
                value={replyText}
                placeholder={t('drawer.replyPlaceholder')}
                onChange={onReplyText}
              />
              <button type="submit" className="icon-button" aria-label={t('drawer.postReply')} disabled={!replyText.trim()}>
                <MessageSquare size={17} />
              </button>
            </form>
          )}
          {comment.replies && comment.replies.length > 0 && (
            <div className="comment-replies">
              {comment.replies.map((reply) => (
                <article className="comment reply" key={reply.id}>
                  <CommentBody comment={reply} onDelete={onDelete} />
                </article>
              ))}
            </div>
          )}
        </article>
      ))}
    </div>
  );
}

function CommentBody({ comment, onDelete }: { comment: Comment; onDelete: (id: number) => void }) {
  const { t } = useTranslation();
  return (
    <>
      <div className="comment-head">
        <div>
          <strong>{comment.username}</strong>
          <span>{formatDate(comment.createdAt)}</span>
        </div>
        {comment.canDelete && (
          <button type="button" className="icon-button danger" aria-label={t('drawer.deleteComment')} onClick={() => onDelete(comment.id)}>
            <Trash2 size={15} />
          </button>
        )}
      </div>
      <p>{comment.body}</p>
    </>
  );
}

function ProfileView({
  user,
  setUser,
  apps,
  groups,
  setGroups,
  categories,
  sourceApps,
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
  sourceApps: SourceApp[];
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
  const [workspaceTab, setWorkspaceTab] = useState<'overview' | 'apps' | 'submit' | 'tokens' | 'groups' | 'favorites'>('overview');
  const [authForm, setAuthForm] = useState({ username: '', password: '', email: '' });
  const [verifyToken, setVerifyToken] = useState(verificationTokenFromURL);
  const [uploadForm, setUploadForm] = useState({
    name: '',
    version: '',
    summary: '',
    description: '',
    categoryId: '',
    tags: '',
    allowUnreviewedUpdates: false,
    emailNotificationsEnabled: true,
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
  const [commentNotifications, setCommentNotifications] = useState<CommentNotification[]>([]);
  const authModeLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verify');
  const authSubmitLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verifyEmail');
  const authHint = mode === 'login' ? t('auth.loginHint') : mode === 'register' ? t('auth.registerHint') : t('auth.verifyHint');
  const AuthSubmitIcon = mode === 'verify' ? Check : mode === 'register' ? Plus : LogIn;
  const workspaceTabs = [
    { key: 'overview', label: t('profile.tabs.overview'), icon: Gauge },
    { key: 'apps', label: t('profile.tabs.apps'), icon: PackagePlus },
    { key: 'submit', label: t('profile.tabs.submit'), icon: Upload },
    { key: 'tokens', label: t('profile.tabs.tokens'), icon: KeyRound },
    { key: 'groups', label: t('profile.tabs.groups'), icon: Users },
    { key: 'favorites', label: t('profile.tabs.favorites'), icon: Heart },
  ] as const;
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
  const externalArtifactReady = externalDownloadReady;
  const artifactReady = artifactMode === 'local' ? Boolean(file) : externalArtifactReady;
  const appIdentityCanAutofill = artifactMode === 'local' ? Boolean(file) : externalDownloadReady;
  const canSubmitUpload = (appInfoReady || appIdentityCanAutofill) && artifactReady;
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
  const installedSummary = useMemo(() => {
    const versioned = installedApps.filter((item) => item.version).length;
    const statusKnown = installedApps.filter((item) => item.status || item.instanceStatus).length;
    const active = installedApps.filter((item) => /running|active|started/i.test(`${item.status || ''} ${item.instanceStatus || ''}`)).length;
    return { total: installedApps.length, versioned, statusKnown, active };
  }, [installedApps]);

  useEffect(() => {
    if (!user) return;
    void api<{ tokens: APITokenRecord[] }>('/api/v1/me/tokens').then((data) => setTokens(data.tokens)).catch(() => setTokens([]));
    void loadCommentNotifications();
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
            emailNotificationsEnabled: uploadForm.emailNotificationsEnabled,
            sourceType: uploadForm.sourceType,
            downloadUrl: uploadForm.downloadUrl.trim(),
            sha256: uploadForm.sha256.trim(),
            ...(uploadForm.installPassword.trim() ? { installPassword: uploadForm.installPassword.trim() } : {}),
          }),
        });
      }
      setRecentSubmission({ name: created.app?.name || uploadForm.name, status: created.app?.status || 'PENDING' });
      setToast({ tone: 'success', message: t('submitApp.submitted') });
      setUploadForm({ name: '', version: '', summary: '', description: '', categoryId: '', tags: '', allowUnreviewedUpdates: false, emailNotificationsEnabled: true, sourceType: 'GITHUB', downloadUrl: '', sha256: '', installPassword: '' });
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

  async function loadCommentNotifications() {
    if (!HAS_API || !user) return;
    await runAction(setToast, t('profile.notificationsLoadFailed'), async () => {
      const data = await api<{ notifications: CommentNotification[] }>('/api/v1/me/comment-notifications');
      setCommentNotifications(arrayOrEmpty(data.notifications));
    });
  }

  async function markAllCommentNotificationsRead() {
    await runAction(setToast, t('profile.notificationsReadFailed'), async () => {
      await api('/api/v1/me/comment-notifications/read', { method: 'POST' });
      await loadCommentNotifications();
      setToast({ tone: 'success', message: t('profile.notificationsRead') });
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
        <InstalledAppsView
          installedApps={installedApps}
          sourceApps={sourceApps}
          installedState={installedState}
          installedError={installedError}
          installedReadinessBody={installedReadinessBody}
          onLoadInstalled={onLoadInstalled}
        />
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
              <XTextInput label={t('auth.verifyToken')} value={verifyToken} isRequired onChange={setVerifyToken} />
            ) : (
              <>
                <XTextInput label={t('common.username')} value={authForm.username} isRequired onChange={(value) => setAuthForm({ ...authForm, username: value })} />
                {mode === 'register' && (
                  <XTextInput type="email" label={t('common.email')} value={authForm.email} onChange={(value) => setAuthForm({ ...authForm, email: value })} />
                )}
                <XTextInput
                  type="password"
                  label={t('common.password')}
                  description={mode === 'register' ? t('auth.passwordHelp') : undefined}
                  value={authForm.password}
                  isRequired
                  onChange={(value) => setAuthForm({ ...authForm, password: value })}
                />
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
            <XTextInput label={t('auth.verifyToken')} value={verifyToken} onChange={setVerifyToken} />
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
      <div className="segmented workspace-tabs" aria-label={t('profile.tabs.label')}>
        {workspaceTabs.map((item) => {
          const Icon = item.icon;
          return (
            <button
              type="button"
              key={item.key}
              className={cx(workspaceTab === item.key && 'active')}
              onClick={() => setWorkspaceTab(item.key)}
            >
              <Icon size={17} />
              <span>{item.label}</span>
            </button>
          );
        })}
      </div>
      {workspaceTab === 'overview' && (
      <div className="split">
        <div className="panel profile-card">
          <AvatarIcon seed={user.email || user.username} title={user.username} size={74} className="avatar-large" />
          <h2>{user.username}</h2>
          <p>{t(`admin.roles.${user.role === 'SITE_ADMIN' ? 'siteAdmin' : user.role === 'SOFTWARE_ADMIN' ? 'softwareAdmin' : 'user'}`)}</p>
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
          <SectionTitle icon={Gauge} title={t('profile.publishOverview')} />
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
        </section>
      </div>
      )}

      {workspaceTab === 'apps' && (
      <section className="panel">
        <SectionTitle icon={PackagePlus} title={t('profile.mySubmissions')} />
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
      )}

      {workspaceTab === 'submit' && (
      <section className="workspace-pane">
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
          <XTextInput label={t('submitApp.appName')} value={uploadForm.name} onChange={(value) => setUploadForm({ ...uploadForm, name: value })} />
          <XTextInput label={t('common.version')} value={uploadForm.version} onChange={(value) => setUploadForm({ ...uploadForm, version: value })} />
          <XTextInput label={t('common.summary')} value={uploadForm.summary} onChange={(value) => setUploadForm({ ...uploadForm, summary: value })} />
          <XTextArea label={t('common.description')} value={uploadForm.description} rows={4} onChange={(value) => setUploadForm({ ...uploadForm, description: value })} />
          <XSelector
            label={t('common.category')}
            value={uploadForm.categoryId}
            options={[
              { value: '', label: t('common.uncategorized') },
              ...categories.map((category) => ({ value: String(category.id), label: localizedName(category) })),
            ]}
            onChange={(value) => setUploadForm({ ...uploadForm, categoryId: value })}
          />
          <XTextInput label={t('common.tags')} value={uploadForm.tags} onChange={(value) => setUploadForm({ ...uploadForm, tags: value })} />
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
              <FilePicker
                label={t('common.lpkFile')}
                help={t('submitApp.localFileHelp')}
                fileName={file?.name}
                inputRef={fileInputRef}
                accept=".lpk"
                required
                onChange={(nextFile) => setFile(nextFile)}
              />
            ) : (
              <div className="artifact-fields">
                <p className="field-help">{t('submitApp.externalFieldsHelp')}</p>
                <XSelector
                  label={t('submitApp.externalSource')}
                  value={uploadForm.sourceType}
                  options={[
                    { value: 'GITHUB', label: 'GitHub Release' },
                    { value: 'WEBDAV', label: 'WebDAV URL' },
                    { value: 'S3', label: 'S3 URL' },
                  ]}
                  onChange={(value) => setUploadForm({ ...uploadForm, sourceType: value })}
                />
                <XTextInput
                  label={t('submitApp.externalDownloadUrl')}
                  description={t('submitApp.externalDownloadHelp')}
                  value={uploadForm.downloadUrl}
                  onChange={(value) => setUploadForm({ ...uploadForm, downloadUrl: value })}
                />
                <XTextInput
                  label={t('common.sha256')}
                  description={t('submitApp.sha256Help')}
                  value={uploadForm.sha256}
                  onChange={(value) => setUploadForm({ ...uploadForm, sha256: value })}
                />
              </div>
            )}
          </div>
          <XTextInput
            type="password"
            label={t('submitApp.installPassword')}
            description={t('submitApp.installPasswordHelp')}
            value={uploadForm.installPassword}
            onChange={(value) => setUploadForm({ ...uploadForm, installPassword: value })}
          />
          <label className="toggle-line">
            <input
              type="checkbox"
              checked={uploadForm.emailNotificationsEnabled}
              onChange={(event) => setUploadForm({ ...uploadForm, emailNotificationsEnabled: event.target.checked })}
            />
            <span>{t('submitApp.emailNotificationsEnabled')}</span>
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
      </section>
      )}
      {workspaceTab === 'tokens' && (
      <section className="workspace-pane">
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
      )}

      {workspaceTab === 'groups' && (
      <section className="workspace-pane">
        <GroupPanel groups={groups} setGroups={setGroups} setToast={setToast} />
      </section>
      )}
      {workspaceTab === 'favorites' && (
      <section className="workspace-pane">
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
      </section>
      )}
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
        <XTextInput label={t('groups.name')} isLabelHidden placeholder={t('groups.name')} value={draft.name} onChange={(value) => setDraft({ ...draft, name: value })} />
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
              <XTextInput
                label={t('groups.userId')}
                isLabelHidden
                placeholder={t('groups.userId')}
                value={memberDrafts[group.id] || ''}
                onChange={(value) => setMemberDrafts((current) => ({ ...current, [group.id]: value }))}
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
  onSiteProfileSaved,
  setToast,
}: {
  user: User;
  reviews: Review[];
  onApprove: (review: Review, approve: boolean) => void;
  onSiteProfileSaved: () => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [adminTab, setAdminTab] = useState<'reviews' | 'site' | 'users' | 'taxonomy' | 'collections' | 'inventory'>('reviews');
  const [users, setUsers] = useState<User[]>([]);
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [reviewApps, setReviewApps] = useState<StoreApp[]>([]);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [adminCategories, setAdminCategories] = useState<Category[]>([]);
  const [adminTags, setAdminTags] = useState<TagRecord[]>([]);
  const [adminCollections, setAdminCollections] = useState<Collection[]>([]);
  const [categoryForm, setCategoryForm] = useState({ name: '', slug: '' });
  const [tagForm, setTagForm] = useState({ name: '', slug: '' });
  const [collectionForm, setCollectionForm] = useState<{ name: string; kind: string; appIds: number[] }>({ name: '', kind: 'MANUAL', appIds: [] });
  const [categoryDrafts, setCategoryDrafts] = useState<Record<number, { name: string; slug: string }>>({});
  const [tagDrafts, setTagDrafts] = useState<Record<number, { name: string; slug: string }>>({});
  const [collectionDrafts, setCollectionDrafts] = useState<Record<number, CollectionDraft>>({});
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [testEmailTo, setTestEmailTo] = useState(user.email || '');
  const isSiteAdmin = user.role === 'SITE_ADMIN';
  const adminTabs = [
    { key: 'reviews', label: t('admin.tabs.reviews'), icon: ShieldCheck },
    ...(isSiteAdmin ? [{ key: 'site' as const, label: t('admin.tabs.site'), icon: Settings }] : []),
    ...(isSiteAdmin ? [{ key: 'users' as const, label: t('admin.tabs.users'), icon: Users }] : []),
    { key: 'taxonomy', label: t('admin.tabs.taxonomy'), icon: Tag },
    { key: 'collections', label: t('admin.tabs.collections'), icon: Layers3 },
    { key: 'inventory', label: t('admin.tabs.inventory'), icon: PackagePlus },
  ] as const;
  const collectionKindOptions = [
    { value: 'MANUAL', label: t('admin.collectionKinds.manual') },
    { value: 'RECENT_UPDATED', label: t('admin.collectionKinds.recentUpdated') },
    { value: 'MOST_DOWNLOADED', label: t('admin.collectionKinds.mostDownloaded') },
  ];
  const userRoleOptions: Array<{ value: User['role']; label: string }> = [
    { value: 'USER', label: t('admin.roles.user') },
    { value: 'SOFTWARE_ADMIN', label: t('admin.roles.softwareAdmin') },
    { value: 'SITE_ADMIN', label: t('admin.roles.siteAdmin') },
  ];
  const siteIdentityFields = [
    { key: 'site_title', label: t('admin.settings.siteTitle'), help: t('admin.settingsHelp.siteTitle') },
    { key: 'site_icon_url', label: t('admin.settings.siteIconURL'), help: t('admin.settingsHelp.siteIconURL'), type: 'url' },
    { key: 'site_public_url', label: t('admin.settings.sitePublicURL'), help: t('admin.settingsHelp.sitePublicURL'), type: 'url' },
  ];
  const announcementFields = [
    { key: 'announcement_enabled', label: t('admin.settings.announcementEnabled'), help: t('admin.settingsHelp.announcementEnabled'), type: 'boolean' },
    {
      key: 'announcement_level',
      label: t('admin.settings.announcementLevel'),
      help: t('admin.settingsHelp.announcementLevel'),
      type: 'select',
      options: [
        { value: 'info', label: t('site.announcementLevels.info') },
        { value: 'warning', label: t('site.announcementLevels.warning') },
        { value: 'success', label: t('site.announcementLevels.success') },
      ],
    },
    { key: 'announcement_title', label: t('admin.settings.announcementTitle'), help: t('admin.settingsHelp.announcementTitle') },
    { key: 'announcement_body', label: t('admin.settings.announcementBody'), help: t('admin.settingsHelp.announcementBody'), type: 'textarea' },
    { key: 'announcement_link_label', label: t('admin.settings.announcementLinkLabel'), help: t('admin.settingsHelp.announcementLinkLabel') },
    { key: 'announcement_link_url', label: t('admin.settings.announcementLinkURL'), help: t('admin.settingsHelp.announcementLinkURL'), type: 'url' },
  ];
  const policySettingFields = [
    { key: 'max_lpk_size', label: t('admin.settings.maxLPKSize'), help: t('admin.settingsHelp.maxLPKSize'), inputMode: 'numeric' },
    { key: 'max_versions', label: t('admin.settings.maxVersions'), help: t('admin.settingsHelp.maxVersions'), inputMode: 'numeric' },
    { key: 'source_password', label: t('admin.settings.sourcePassword'), help: t('admin.settingsHelp.sourcePassword'), type: 'password' },
    { key: 'source_password_rotation', label: t('admin.settings.sourcePasswordRotation'), help: t('admin.settingsHelp.sourcePasswordRotation'), inputMode: 'numeric' },
    { key: 'github_download_mirrors', label: t('admin.settings.githubDownloadMirrors'), help: t('admin.settingsHelp.githubDownloadMirrors'), type: 'textarea' },
    { key: 'github_raw_mirrors', label: t('admin.settings.githubRawMirrors'), help: t('admin.settingsHelp.githubRawMirrors'), type: 'textarea' },
    { key: 'require_email_verify', label: t('admin.settings.requireEmailVerify'), help: t('admin.settingsHelp.requireEmailVerify'), type: 'boolean' },
  ];
  const smtpSettingFields = [
    { key: 'smtp_host', label: t('admin.settings.smtpHost'), help: t('admin.settingsHelp.smtpHost') },
    { key: 'smtp_port', label: t('admin.settings.smtpPort'), help: t('admin.settingsHelp.smtpPort') },
    { key: 'smtp_user', label: t('admin.settings.smtpUser'), help: t('admin.settingsHelp.smtpUser') },
    { key: 'smtp_pass', label: t('admin.settings.smtpPass'), help: t('admin.settingsHelp.smtpPass'), type: 'password' },
    { key: 'smtp_from', label: t('admin.settings.smtpFrom'), help: t('admin.settingsHelp.smtpFrom') },
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
  const adminPublicURL = stripTrailingSlash(settings.site_public_url || window.location.origin);
  const adminSourceURL = adminPublicURL ? `${adminPublicURL}/source/v1/index.json` : '';
  const announcementPreview: SiteAnnouncement = {
    enabled: settings.announcement_enabled === 'true',
    level: settings.announcement_level === 'warning' || settings.announcement_level === 'success' ? settings.announcement_level : 'info',
    title: settings.announcement_title,
    body: settings.announcement_body,
    linkLabel: settings.announcement_link_label,
    linkUrl: settings.announcement_link_url,
    updatedAt: settings.announcement_updated_at,
  };

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
        setSettings(settingData.settings || {});
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
      await onSiteProfileSaved();
    });
  }

  async function sendTestEmail() {
    await runAction(setToast, t('admin.testEmailFailed'), async () => {
      await api('/api/v1/admin/settings/test-email', {
        method: 'POST',
        body: JSON.stringify({
          to: testEmailTo,
          settings,
        }),
      });
      setToast({ tone: 'success', message: t('admin.testEmailSent') });
    });
  }

  function updateSetting(key: string, value: string) {
    setSettings((current) => ({ ...current, [key]: value }));
  }

  function recommendedMirrorsForSetting(key: string) {
    if (key === 'github_download_mirrors') return mirrorPresetText(RECOMMENDED_DOWNLOAD_MIRRORS);
    if (key === 'github_raw_mirrors') return mirrorPresetText(RECOMMENDED_RAW_MIRRORS);
    return '';
  }

  function renderSettingField(field: {
    key: string;
    label: string;
    help: string;
    type?: string;
    inputMode?: string;
    options?: Array<{ value: string; label: string }>;
  }) {
    if (field.type === 'boolean') {
      return (
        <XSelector
          key={field.key}
          label={field.label}
          description={field.help}
          value={settings[field.key] || 'false'}
          options={[
            { value: 'false', label: t('common.off') },
            { value: 'true', label: t('common.on') },
          ]}
          onChange={(value) => updateSetting(field.key, value)}
        />
      );
    }
    if (field.type === 'select') {
      return (
        <XSelector
          key={field.key}
          label={field.label}
          description={field.help}
          value={settings[field.key] || field.options?.[0]?.value || ''}
          options={field.options || []}
          onChange={(value) => updateSetting(field.key, value)}
        />
      );
    }
    if (field.type === 'textarea') {
      const preset = recommendedMirrorsForSetting(field.key);
      if (preset) {
        return (
          <div className="settings-mirror-field" key={field.key}>
            <XTextArea
              label={field.label}
              description={field.help}
              value={settings[field.key] || ''}
              rows={5}
              onChange={(value) => updateSetting(field.key, value)}
            />
            <button type="button" className="secondary-button compact-button" onClick={() => updateSetting(field.key, preset)}>
              <Download size={16} />
              <span>{t('admin.useRecommendedMirrors')}</span>
            </button>
          </div>
        );
      }
      return (
        <XTextArea
          key={field.key}
          label={field.label}
          description={field.help}
          value={settings[field.key] || ''}
          rows={4}
          onChange={(value) => updateSetting(field.key, value)}
        />
      );
    }
    return (
      <XTextInput
        key={field.key}
        type={field.type === 'password' ? 'password' : 'text'}
        label={field.label}
        description={field.help}
        value={settings[field.key] || ''}
        onChange={(value) => updateSetting(field.key, value)}
      />
    );
  }

  async function copyAdminSourceURL() {
    try {
      if (!navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(adminSourceURL);
      setToast({ tone: 'success', message: t('home.sourceCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('home.copySourceFailed')) });
    }
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
          appIds: collectionForm.appIds,
        }),
      });
      setCollectionForm({ name: '', kind: 'MANUAL', appIds: [] });
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
        appIds: (item.apps || []).map((app) => app.id),
      };
    await runAction(setToast, t('admin.collectionUpdateFailed'), async () => {
      await api(`/api/v1/admin/collections/${item.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          name: draft.name,
          slug: draft.slug,
          kind: draft.kind,
          appIds: draft.appIds,
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

  function statusBadgeVariant(value?: string): 'neutral' | 'success' | 'warning' | 'error' | 'info' {
    const key = statusKey(value);
    if (key === 'approved' || key === 'synced') return 'success';
    if (key === 'pending') return 'warning';
    if (key === 'rejected' || key === 'failed' || key === 'blocked') return 'error';
    return 'neutral';
  }

  function renderAdminMetric({
    label,
    value,
    body,
    status,
    variant,
  }: {
    label: string;
    value: string | number;
    body: string;
    status: string;
    variant: 'neutral' | 'success' | 'warning' | 'error' | 'info';
  }) {
    return (
      <XCard className="admin-metric-card" padding={4}>
        <div className="admin-metric-card-head">
          <span>{label}</span>
          <XBadge label={status} variant={variant} />
        </div>
        <strong>{value}</strong>
        <p>{body}</p>
      </XCard>
    );
  }

  return (
    <section className="page-grid">
      <div className="page-heading">
        <span className="eyebrow subtle">{t('admin.eyebrow')}</span>
        <h1>{t('admin.title')}</h1>
        <p>{t('admin.body')}</p>
      </div>
      <XTabList value={adminTab} onChange={(value) => setAdminTab(value as typeof adminTab)} hasDivider size="md">
        {adminTabs.map((item) => {
          const Icon = item.icon;
          return <XTab key={item.key} value={item.key} label={item.label} icon={<Icon size={17} />} />;
        })}
      </XTabList>
      {adminTab === 'reviews' && (
      <>
      <section className="panel">
        <SectionTitle icon={Gauge} title={t('admin.operationsOverview')} />
        <div className="admin-metric-grid" aria-label={t('admin.operationsOverview')}>
          {renderAdminMetric({
            label: t('admin.opsReviewTitle'),
            value: reviewSummary.total,
            body: reviewOpsBody,
            status: reviewSummary.total === 0 ? t('admin.opsReady') : t('admin.opsNeedsAction'),
            variant: reviewSummary.total === 0 ? 'success' : 'warning',
          })}
          {renderAdminMetric({
            label: t('admin.opsCatalogTitle'),
            value: apps.length,
            body: catalogOpsBody,
            status: catalogReady ? t('admin.opsReady') : t('admin.opsNeedsAction'),
            variant: catalogReady ? 'success' : 'warning',
          })}
          {renderAdminMetric({
            label: t('admin.opsSourceTitle'),
            value: sourceProtected ? t('common.on') : t('common.off'),
            body: sourceOpsBody,
            status: sourceProtected ? t('admin.opsReady') : t('admin.opsNeedsAction'),
            variant: sourceProtected ? 'success' : 'warning',
          })}
        </div>
      </section>
      <section className="panel">
        <SectionTitle icon={ShieldCheck} title={t('admin.reviewQueue')} />
        <div className="admin-metric-grid compact" aria-label={t('admin.reviewSummary')}>
          {renderAdminMetric({
            label: t('admin.pendingTotal'),
            value: reviewSummary.total,
            body: t('admin.reviewSummaryPendingBody'),
            status: reviewSummary.total > 0 ? t('admin.opsNeedsAction') : t('admin.opsReady'),
            variant: reviewSummary.total > 0 ? 'warning' : 'success',
          })}
          {renderAdminMetric({
            label: t('admin.appSubmissions'),
            value: reviewSummary.appSubmissions,
            body: t('admin.reviewSummaryAppBody'),
            status: t('reviewKinds.appsubmission'),
            variant: 'neutral',
          })}
          {renderAdminMetric({
            label: t('admin.versionUploads'),
            value: reviewSummary.versionUploads,
            body: t('admin.reviewSummaryVersionBody'),
            status: t('reviewKinds.versionupload'),
            variant: 'neutral',
          })}
          {renderAdminMetric({
            label: t('admin.infoUpdates'),
            value: reviewSummary.infoUpdates,
            body: t('admin.reviewSummaryInfoBody'),
            status: t('reviewKinds.appinfoupdate'),
            variant: 'neutral',
          })}
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
                    <XBadge label={t(`statusLabels.${statusKey(review.status)}`)} variant={statusBadgeVariant(review.status)} />
                    <XButton
                      label={t('admin.approveReview', { id: review.id })}
                      icon={<Check size={16} />}
                      isIconOnly
                      size="sm"
                      variant="secondary"
                      onClick={() => void onApprove(review, true)}
                    />
                    <XButton
                      label={t('admin.rejectReview', { id: review.id })}
                      icon={<X size={16} />}
                      isIconOnly
                      size="sm"
                      variant="destructive"
                      onClick={() => void onApprove(review, false)}
                    />
                  </div>
                </div>
              );
            })
          )}
        </div>
      </section>
      </>
      )}
      {isSiteAdmin && adminTab === 'site' && (
        <section className="settings-layout">
          <form className="panel form-panel site-settings-panel" onSubmit={saveSettings}>
            <SectionTitle icon={Settings} title={t('admin.siteSettings')} />
            <div className="settings-section">
              <div className="settings-section-head">
                <strong>{t('admin.siteIdentity')}</strong>
                <span>{t('admin.siteIdentityBody')}</span>
              </div>
              <XFormLayout direction="horizontal">
                {siteIdentityFields.map(renderSettingField)}
              </XFormLayout>
            </div>
            <div className="settings-section">
              <div className="settings-section-head">
                <strong>{t('admin.announcementCenter')}</strong>
                <span>{t('admin.announcementCenterBody')}</span>
              </div>
              <XFormLayout>
                {announcementFields.map(renderSettingField)}
              </XFormLayout>
            </div>
            <div className="settings-section">
              <div className="settings-section-head">
                <strong>{t('admin.policySettings')}</strong>
                <span>{t('admin.policySettingsBody')}</span>
              </div>
              <XFormLayout direction="horizontal">
                {policySettingFields.map(renderSettingField)}
              </XFormLayout>
            </div>
            <div className="settings-section">
              <div className="settings-section-head">
                <strong>{t('admin.smtpSettings')}</strong>
                <span>{t('admin.smtpSettingsBody')}</span>
              </div>
              <XFormLayout direction="horizontal">
                {smtpSettingFields.map(renderSettingField)}
              </XFormLayout>
              <div className="test-email-form">
                <XTextInput
                  type="email"
                  label={t('admin.testEmailTo')}
                  description={t('admin.testEmailHelp')}
                  value={testEmailTo}
                  onChange={setTestEmailTo}
                />
                <button type="button" className="secondary-button compact-button" onClick={() => void sendTestEmail()}>
                  <MessageSquare size={17} />
                  <span>{t('admin.sendTestEmail')}</span>
                </button>
              </div>
            </div>
            <XButton type="submit" variant="primary" label={t('admin.saveSettings')} icon={<Settings size={18} />} />
          </form>
          <section className="panel site-preview-panel">
            <SectionTitle icon={Archive} title={t('admin.sitePreview')} />
            <div className="site-preview-brand">
              <div className="brand-mark">
                {settings.site_icon_url ? <img src={settings.site_icon_url} alt="" /> : <Archive size={22} />}
              </div>
              <div>
                <strong>{settings.site_title || t('appName')}</strong>
                <span>{adminPublicURL}</span>
              </div>
            </div>
            <div className="source-url-preview">
              <span>{t('admin.subscriptionURL')}</span>
              <code>{adminSourceURL}</code>
              <button type="button" className="secondary-button compact-button" onClick={() => void copyAdminSourceURL()}>
                <Copy size={16} />
                <span>{t('home.copySourceFeed')}</span>
              </button>
            </div>
            {announcementPreview.enabled ? (
              <AnnouncementBanner announcement={announcementPreview} />
            ) : (
              <div className="announcement-preview-empty">
                <MessageSquare size={18} />
                <span>{t('admin.announcementDisabled')}</span>
              </div>
            )}
          </section>
        </section>
      )}
      {isSiteAdmin && adminTab === 'users' && (
        <section className="workspace-pane">
          <section className="panel">
            <SectionTitle icon={Users} title={t('admin.userManagement')} />
            <div className="review-list">
              {users.map((item) => (
                <div className="review-row" key={item.id}>
                <div>
                  <strong>#{item.id} {item.username}</strong>
                  <span>{item.email || t('admin.noEmail')}</span>
                </div>
                  <XSelector
                    label={t('admin.userRoleFor', { username: item.username })}
                    isLabelHidden
                    value={item.role}
                    options={userRoleOptions}
                    width={220}
                    onChange={(value) => void updateUserRole(item.id, value as User['role'])}
                  />
                </div>
              ))}
            </div>
          </section>
        </section>
      )}
      {adminTab === 'taxonomy' && (
      <section className="split">
        <div className="panel form-panel">
          <SectionTitle icon={Layers3} title={t('admin.categoriesAndTags')} />
          <form className="inline-stack" onSubmit={createCategory}>
            <XTextInput label={t('admin.categoryName')} value={categoryForm.name} onChange={(value) => setCategoryForm({ ...categoryForm, name: value })} />
            <XTextInput label={t('admin.categorySlug')} value={categoryForm.slug} onChange={(value) => setCategoryForm({ ...categoryForm, slug: value })} />
            <XButton type="submit" variant="secondary" label={t('admin.category')} icon={<Plus size={17} />} />
          </form>
          <form className="inline-stack" onSubmit={createTag}>
            <XTextInput label={t('admin.tagName')} value={tagForm.name} onChange={(value) => setTagForm({ ...tagForm, name: value })} />
            <XTextInput label={t('admin.tagSlug')} value={tagForm.slug} onChange={(value) => setTagForm({ ...tagForm, slug: value })} />
            <XButton type="submit" variant="secondary" label={t('admin.tag')} icon={<Plus size={17} />} />
          </form>
        </div>
        <section className="panel">
          <SectionTitle icon={Tag} title={t('admin.categoryList')} />
          <div className="review-list">
            {adminCategories.map((item) => {
              const draft = categoryDrafts[item.id] || { name: item.name, slug: item.slug };
              return (
                <div className="edit-row" key={item.id}>
                  <XTextInput label={t('admin.categoryNameFor', { name: item.name })} isLabelHidden value={draft.name} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, name: value } }))} />
                  <XTextInput label={t('admin.categorySlugFor', { name: item.name })} isLabelHidden value={draft.slug} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: value } }))} />
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
                  <XTextInput label={t('admin.tagNameFor', { name: item.name })} isLabelHidden value={draft.name} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, name: value } }))} />
                  <XTextInput label={t('admin.tagSlugFor', { name: item.name })} isLabelHidden value={draft.slug} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: value } }))} />
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
      )}
      {adminTab === 'collections' && (
      <section className="split">
        <form className="panel form-panel" onSubmit={createCollection}>
          <SectionTitle icon={Layers3} title={t('admin.collection')} />
          <XTextInput label={t('common.name')} value={collectionForm.name} onChange={(value) => setCollectionForm({ ...collectionForm, name: value })} />
          <XSelector
            label={t('admin.type')}
            value={collectionForm.kind}
            options={collectionKindOptions}
            onChange={(value) => setCollectionForm({ ...collectionForm, kind: value })}
          />
          <CollectionAppPicker
            apps={apps}
            appIds={collectionForm.appIds}
            labels={{
              title: t('admin.collectionApps'),
              selectedCount: t('admin.selectedAppsCount', { count: collectionForm.appIds.length }),
              empty: t('admin.noApprovedAppsForCollection'),
            }}
            onChange={(appIds) => setCollectionForm({ ...collectionForm, appIds })}
          />
          <p className="field-help">{t('admin.collectionAppsHelp')}</p>
          <XButton type="submit" variant="primary" label={t('admin.createCollection')} icon={<Layers3 size={18} />} />
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
                  appIds: (item.apps || []).map((app) => app.id),
                };
              return (
                <div className="collection-edit-row" key={item.id}>
                  <XTextInput label={t('admin.collectionNameFor', { name: item.name })} isLabelHidden value={draft.name} onChange={(value) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, name: value } }))} />
                  <XTextInput label={t('admin.collectionSlugFor', { name: item.name })} isLabelHidden value={draft.slug} onChange={(value) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: value } }))} />
                  <XSelector
                    label={t('admin.collectionTypeFor', { name: item.name })}
                    isLabelHidden
                    value={draft.kind}
                    options={collectionKindOptions}
                    onChange={(value) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, kind: value } }))}
                  />
                  <CollectionAppPicker
                    apps={apps}
                    appIds={draft.appIds}
                    labels={{
                      title: t('admin.collectionAppsFor', { name: item.name }),
                      selectedCount: t('admin.selectedAppsCount', { count: draft.appIds.length }),
                      empty: t('admin.noApprovedAppsForCollection'),
                    }}
                    onChange={(appIds) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, appIds } }))}
                  />
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
      )}
      {adminTab === 'inventory' && (
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
      )}
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
  const [replyTarget, setReplyTarget] = useState<number | null>(null);
  const [replyText, setReplyText] = useState('');
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
    emailNotificationsEnabled: app.emailNotificationsEnabled,
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
  const versionExternalArtifactReady = versionExternalDownloadReady;
  const versionArtifactReady = versionArtifactMode === 'local' ? Boolean(versionFile) : versionExternalArtifactReady;
  const canSubmitVersion = versionArtifactReady;
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
      emailNotificationsEnabled: app.emailNotificationsEnabled,
      installPassword: '',
      clearInstallPassword: false,
    });
    setVisibility(app.visibleGroupIds || []);
    setConfirmAction(null);
    setCommentText('');
    setReplyTarget(null);
    setReplyText('');
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

  async function submitComment(event: FormEvent, parentId?: number) {
    event.preventDefault();
    const body = (parentId ? replyText : commentText).trim();
    if (!body) return;
    await runAction(setToast, t('drawer.commentPostFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/comments`, { method: 'POST', body: JSON.stringify({ body, parentId }) });
      if (parentId) {
        setReplyText('');
        setReplyTarget(null);
      } else {
        setCommentText('');
      }
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
          emailNotificationsEnabled: appForm.emailNotificationsEnabled,
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
    if (versionArtifactMode === 'local' && !versionFile) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (versionArtifactMode === 'external' && !versionForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
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
            aria-label={installable ? `${t('common.download')} ${app.name}` : t('app.installUnavailable', { name: app.name })}
          >
            <Download size={18} />
            <span>{installable ? t('common.download') : t('common.unavailable')}</span>
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
                <XTextInput label={t('common.name')} value={appForm.name} onChange={(value) => setAppForm({ ...appForm, name: value })} />
                <XTextInput label={t('common.summary')} value={appForm.summary} onChange={(value) => setAppForm({ ...appForm, summary: value })} />
                <XTextArea label={t('common.description')} value={appForm.description} rows={4} onChange={(value) => setAppForm({ ...appForm, description: value })} />
                <XSelector
                  label={t('common.category')}
                  value={appForm.categoryId}
                  options={[
                    { value: '', label: t('common.uncategorized') },
                    ...categories.map((category) => ({ value: String(category.id), label: localizedName(category) })),
                  ]}
                  onChange={(value) => setAppForm({ ...appForm, categoryId: value })}
                />
                <XTextInput label={t('common.tags')} value={appForm.tags} onChange={(value) => setAppForm({ ...appForm, tags: value })} />
                <XTextInput
                  type="password"
                  label={t('drawer.installPassword')}
                  description={app.installProtected ? t('drawer.installPasswordUpdateHelp') : t('drawer.installPasswordHelp')}
                  value={appForm.installPassword}
                  onChange={(value) => setAppForm({ ...appForm, installPassword: value, clearInstallPassword: false })}
                />
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
                    checked={appForm.emailNotificationsEnabled}
                    onChange={(event) => setAppForm({ ...appForm, emailNotificationsEnabled: event.target.checked })}
                  />
                  <span>{t('drawer.emailNotificationsEnabled')}</span>
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
                <XTextInput label={t('common.version')} value={versionForm.version} onChange={(value) => setVersionForm({ ...versionForm, version: value })} />
                <XTextInput label={t('common.changelog')} value={versionForm.changelog} onChange={(value) => setVersionForm({ ...versionForm, changelog: value })} />
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
                    <FilePicker
                      label={t('common.lpkFile')}
                      help={t('drawer.versionLocalFileHelp')}
                      fileName={versionFile?.name}
                      inputRef={versionFileInputRef}
                      accept=".lpk"
                      required
                      onChange={(nextFile) => setVersionFile(nextFile)}
                    />
                  ) : (
                    <div className="artifact-fields">
                      <p className="field-help">{t('submitApp.externalFieldsHelp')}</p>
                      <XSelector
                        label={t('submitApp.externalSource')}
                        value={versionForm.sourceType}
                        options={[
                          { value: 'GITHUB', label: 'GitHub Release' },
                          { value: 'WEBDAV', label: 'WebDAV URL' },
                          { value: 'S3', label: 'S3 URL' },
                        ]}
                        onChange={(value) => setVersionForm({ ...versionForm, sourceType: value })}
                      />
                      <XTextInput
                        label={t('submitApp.externalDownloadUrl')}
                        description={t('submitApp.externalDownloadHelp')}
                        value={versionForm.downloadUrl}
                        onChange={(value) => setVersionForm({ ...versionForm, downloadUrl: value })}
                      />
                      <XTextInput
                        label={t('common.sha256')}
                        description={t('submitApp.sha256Help')}
                        value={versionForm.sha256}
                        onChange={(value) => setVersionForm({ ...versionForm, sha256: value })}
                      />
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
              <XTextInput label={t('drawer.screenshotCaption')} isLabelHidden value={screenshotCaption} placeholder={t('drawer.screenshotCaption')} onChange={setScreenshotCaption} />
              <FilePicker
                label={t('drawer.uploadScreenshot')}
                fileName={screenshotFile?.name}
                accept=".png,.jpg,.jpeg,.webp"
                onChange={(nextFile) => setScreenshotFile(nextFile)}
              />
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
            <form className="comment-form rich-comment-form" onSubmit={(event) => void submitComment(event)}>
              <XTextInput
                label={t('drawer.commentPlaceholder')}
                isLabelHidden
                value={commentText}
                placeholder={t('drawer.commentPlaceholder')}
                onChange={setCommentText}
              />
              <button type="submit" className="icon-button" aria-label={t('drawer.postComment')} disabled={!commentText.trim()}>
                <MessageSquare size={17} />
              </button>
            </form>
          )}
          {!app.commentsEnabled && <p className="inline-note">{t('drawer.commentsDisabled')}</p>}
          <CommentList
            comments={app.comments || []}
            replyTarget={replyTarget}
            replyText={replyText}
            onReplyTarget={setReplyTarget}
            onReplyText={setReplyText}
            onReply={(event, parentId) => void submitComment(event, parentId)}
            onDelete={(commentID) => void deleteComment(commentID)}
          />
        </section>
      </article>
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
