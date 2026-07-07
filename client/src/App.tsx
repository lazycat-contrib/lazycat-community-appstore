import {
  AlertCircle,
  Archive,
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Check,
  ChevronDown,
  ChevronRight,
  Cloud,
  Copy,
  Download,
  Gauge,
  Heart,
  HelpCircle,
  History,
  Home,
  KeyRound,
  Layers3,
  Link,
  LogIn,
  LogOut,
  MessageSquare,
  MessageSquareOff,
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
  UserPlus,
  UserRound,
  Users,
  X,
} from 'lucide-react';
import { Theme } from '@astryxdesign/core/theme';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import i18n from './i18n';
import { API_BASE, DEFAULT_SOURCE_NAME, DEFAULT_SOURCE_URL, HAS_API } from './config';
import { api, apiWithUploadProgress, clientApi } from './shared/api';
import {
  ANNOUNCEMENT_DISMISS_STORAGE_KEY,
  ANNOUNCEMENT_NOTIFY_STORAGE_KEY,
  ASTRYX_THEME_STORAGE_KEY,
  RECOMMENDED_DOWNLOAD_MIRRORS,
  RECOMMENDED_RAW_MIRRORS,
  THEME_STORAGE_KEY,
  mirrorPresetText,
} from './shared/constants';
import { getAstryxTheme, type AstryxThemeName } from './shared/astryxThemes';
import { AstryxThemeSelector, LanguageSelector, readAstryxThemeName, readSystemTheme, readThemeMode, ThemeToggle } from './shared/theme';
import type {
  APITokenRecord,
  Category,
  ClientInstallResult,
  CollaborationData,
  ClientSettings,
  ClientSourceStats,
  CollaboratorRequest,
  Collection,
  CollectionDraft,
  Comment,
  CommentNotification,
  FavoriteData,
  GitHubMirror,
  Group,
  InstallActivity,
  InstalledApplication,
  InstallHistoryEntry,
  InstallOptions,
  InstallPasswordRequest,
  RegistrationInvite,
  ResolvedTheme,
  Review,
  Screenshot,
  SetupStatus,
  SiteAnnouncement,
  SiteProfile,
  SortMode,
  SourceApp,
  SourceID,
  SourceInput,
  SourceSubscription,
  SourceVersion,
  StorageOption,
  StoreApp,
  TagRecord,
  ThemeMode,
  Toast,
  User,
  Version,
  OwnedCollaboration,
  CollaboratorInvite,
} from './shared/types';
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
  localizedCategory,
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
import { AppIcon, AvatarIcon, UserAvatar } from './components/AppIcon';
import { ArtifactModeOption } from './shared/components/ArtifactModeOption';
import { FilePicker } from './shared/components/FilePicker';
import { TagTokenizer } from './shared/components/TagTokenizer';
import { ClientHistoryView } from './modules/client/ClientHistoryView';
import { ClientCatalog } from './modules/client/ClientCatalog';
import { InstalledAppsView } from './modules/client/InstalledAppsView';
import { ClientSettingsView } from './modules/client/ClientSettingsView';
import { SourcesView as ClientSourcesView } from './modules/client/SourcesView';
import { CollectionAppPicker } from './modules/admin/CollectionAppPicker';
import { AppSubmissionForm, type SubmissionArtifactMode, type SubmissionProgress } from './modules/profile/AppSubmissionForm';
import { StorageSettingsPanel, defaultStorageSettings, type StorageSettings } from './modules/admin/StorageSettingsPanel';
import { AppGrid } from './modules/storefront/AppGrid';
import { StorefrontHome } from './modules/storefront/StorefrontHome';
import { StorefrontSearch } from './modules/storefront/StorefrontSearch';

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

type TaxonomyDraft = { name: string; nameI18n: Record<string, string>; slug: string };
type AppDetailMode = 'detail' | 'manage';
type ProfileWorkspaceTab = 'overview' | 'apps' | 'collaboration' | 'manage' | 'tokens' | 'groups' | 'favorites';
type ManagedUserDraft = {
  id?: number;
  username: string;
  nickname: string;
  email: string;
  password: string;
  role: User['role'];
  emailVerified: boolean;
  disabled: boolean;
};

function verificationTokenFromURL() {
  if (!window.location.pathname.includes('verify')) return '';
  return new URLSearchParams(window.location.search).get('token') || '';
}

function collaborationInviteTokenFromURL() {
  if (!window.location.pathname.includes('collaboration-invite')) return '';
  return new URLSearchParams(window.location.search).get('token') || '';
}

function usePreferredScreenshotDevice() {
  const readDevice = () => (window.matchMedia('(max-width: 720px)').matches ? 'MOBILE' : 'DESKTOP');
  const [device, setDevice] = useState<'DESKTOP' | 'MOBILE'>(readDevice);
  useEffect(() => {
    const query = window.matchMedia('(max-width: 720px)');
    const update = () => setDevice(query.matches ? 'MOBILE' : 'DESKTOP');
    update();
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  return device;
}

function orderedScreenshots(screenshots: Screenshot[] | undefined, preferredDevice: 'DESKTOP' | 'MOBILE') {
  return [...(screenshots || [])].sort((left, right) => {
    const leftPreferred = (left.deviceType || 'DESKTOP') === preferredDevice ? 0 : 1;
    const rightPreferred = (right.deviceType || 'DESKTOP') === preferredDevice ? 0 : 1;
    if (leftPreferred !== rightPreferred) return leftPreferred - rightPreferred;
    return (left.sortOrder || 0) - (right.sortOrder || 0) || left.id - right.id;
  });
}

function screenshotDeviceLabel(t: (key: string, options?: any) => string, deviceType?: string) {
  return deviceType === 'MOBILE' ? t('drawer.screenshotDeviceMobile') : t('drawer.screenshotDeviceDesktop');
}

function canUserManageApp(user: User | null | undefined, app: StoreApp) {
  if (!user) return false;
  return app.canManageApp ?? (user.role === 'SITE_ADMIN' || user.role === 'SOFTWARE_ADMIN' || user.id === app.ownerId);
}

function canUserUploadVersion(user: User | null | undefined, app: StoreApp) {
  return !!user && (app.canUploadVersion || canUserManageApp(user, app));
}

function screenshotFileKey(file: File) {
  return `${file.name}:${file.size}:${file.lastModified}`;
}

function reconcileScreenshotCaptions(files: File[], current: Record<string, string>) {
  return Object.fromEntries(files.map((file) => {
    const key = screenshotFileKey(file);
    return [key, current[key] || ''];
  }));
}

function displayUserName(user: User | null | undefined) {
  return user?.nickname?.trim() || user?.username || '';
}

function normalizeStorageRecord(storage?: Partial<StorageSettings>): StorageSettings {
  return {
    ...defaultStorageSettings,
    ...(storage || {}),
    key: storage?.key || defaultStorageSettings.key,
    name: storage?.name || storage?.key || defaultStorageSettings.name,
  };
}

function storageOptionsFromRecords(storages: StorageSettings[], defaultKey: string): StorageOption[] {
  return storages.map((storage) => ({
    key: storage.key,
    name: storage.name || storage.key,
    isDefault: storage.key === defaultKey || Boolean(storage.isDefault),
    provider: storage.provider,
    deliveryMode: storage.deliveryMode,
  }));
}

function storageSelectOptions(storages: StorageOption[]) {
  return storages.map((storage) => ({
    value: storage.key,
    label: storage.name || storage.key,
  }));
}

function defaultUploadStorageKey(storages: StorageOption[]) {
  return storages.find((storage) => storage.isDefault)?.key || storages[0]?.key || 'primary';
}

function knownAppTags(apps: StoreApp[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  apps.forEach((app) => {
    (app.tags || []).forEach((tag) => {
      const normalized = tag.trim();
      const key = normalized.toLowerCase();
      if (!normalized || seen.has(key)) return;
      seen.add(key);
      out.push(normalized);
    });
  });
  return out.sort((a, b) => a.localeCompare(b));
}

function emptyUserDraft(): ManagedUserDraft {
  return {
    username: '',
    nickname: '',
    email: '',
    password: '',
    role: 'USER',
    emailVerified: true,
    disabled: false,
  };
}

function draftFromUser(user: User): ManagedUserDraft {
  return {
    id: user.id,
    username: user.username,
    nickname: user.nickname || '',
    email: user.email || '',
    password: '',
    role: user.role,
    emailVerified: Boolean(user.emailVerified),
    disabled: Boolean(user.disabled),
  };
}

export function App() {
  const { t } = useTranslation();
  const [themeMode, setThemeMode] = useState<ThemeMode>(readThemeMode);
  const [astryxThemeName, setAstryxThemeName] = useState(readAstryxThemeName);
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(readSystemTheme);
  const [tab, setTab] = useState<TabKey>(() => (verificationTokenFromURL() || collaborationInviteTokenFromURL() ? 'profile' : HAS_API ? 'home' : 'sources'));
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [managedApps, setManagedApps] = useState<StoreApp[]>([]);
  const [collaborationData, setCollaborationData] = useState<CollaborationData>({ owned: [], collaborating: [], outgoingRequests: [] });
  const [sourceApps, setSourceApps] = useState<SourceApp[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [user, setUser] = useState<User | null>(null);
  const [storageOptions, setStorageOptions] = useState<StorageOption[]>([]);
  const [query, setQuery] = useState('');
  const [activeCategory, setActiveCategory] = useState<string>('all');
  const [activeSubmitter, setActiveSubmitter] = useState<string>('all');
  const [sortMode, setSortMode] = useState<SortMode>('recent');
  const [selectedApp, setSelectedApp] = useState<StoreApp | null>(null);
  const [selectedAppMode, setSelectedAppMode] = useState<AppDetailMode>('detail');
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
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const [isProfileDialogOpen, setIsProfileDialogOpen] = useState(false);
  const [openSubmitSignal, setOpenSubmitSignal] = useState(0);
  const acceptedCollaborationInviteRef = useRef('');
  const defaultSourceCheckedRef = useRef(false);
  const canReview = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const serverNavItems = user ? [...serverBaseTabs, ...(canReview ? [serverAdminTab] : [])] : serverBaseTabs.filter((item) => item.key !== 'profile');
  const navItems = HAS_API ? serverNavItems : clientTabs;
  const siteTitle = HAS_API ? siteProfile.title : t('appName');
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';
  const drawerOpen = false;
  const resolvedTheme: ResolvedTheme = themeMode === 'system' ? systemTheme : themeMode;
  const selectedAstryxTheme = useMemo(() => getAstryxTheme(astryxThemeName), [astryxThemeName]);
  const tagOptions = useMemo(() => knownAppTags(apps), [apps]);
  const announcementKey =
    siteProfile.announcement.updatedAt ||
    `${siteProfile.announcement.level}:${siteProfile.announcement.title || ''}:${siteProfile.announcement.body || ''}`;
  const showAnnouncement =
    HAS_API &&
    siteProfile.announcement.enabled &&
    Boolean(siteProfile.announcement.title || siteProfile.announcement.body) &&
    announcementKey !== dismissedAnnouncement;

  const [sources, setSources] = useState<SourceSubscription[]>([]);

  function navigateTo(nextTab: TabKey) {
    setSelectedApp(null);
    setSelectedAppMode('detail');
    setSelectedSourceApp(null);
    setTab(nextTab);
  }

  function openSubmitApp() {
    setOpenSubmitSignal((signal) => signal + 1);
    navigateTo('profile');
  }

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
    try {
      localStorage.setItem(ASTRYX_THEME_STORAGE_KEY, astryxThemeName);
    } catch {
      // Storage can be blocked by privacy settings; the active theme still applies for this session.
    }
  }, [astryxThemeName]);

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-color-scheme: dark)');
    if (!media) return;
    const updateSystemTheme = () => setSystemTheme(media.matches ? 'dark' : 'light');
    updateSystemTheme();
    media.addEventListener('change', updateSystemTheme);
    return () => media.removeEventListener('change', updateSystemTheme);
  }, []);

  useEffect(() => {
    document.documentElement.dataset.appTheme = resolvedTheme;
    document.documentElement.dataset.themePreference = themeMode;
  }, [resolvedTheme, themeMode]);

  useEffect(() => {
    void runAction(setToast, t('toast.refreshFailed'), async () => {
      await (HAS_API ? refreshAll({ silent: true }) : refreshClientData({ silent: true }));
    });
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    const token = collaborationInviteTokenFromURL();
    if (!HAS_API || !user || !token || acceptedCollaborationInviteRef.current === token) return;
    acceptedCollaborationInviteRef.current = token;
    void runAction(setToast, t('profile.collaborationInviteAcceptFailed'), async () => {
      await api('/api/v1/collaborator-invites/accept', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      setToast({ tone: 'success', message: t('profile.collaborationInviteAccepted') });
      await loadCollaborationData();
      await refreshAll({ silent: true });
    });
  }, [user, t]);

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
        setManagedApps([]);
        setCollaborationData({ owned: [], collaborating: [], outgoingRequests: [] });
        setCategories([]);
        setCollections([]);
        setGroups([]);
        setReviews([]);
        setUser(null);
        setStorageOptions([]);
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
        await loadStorageOptions();
        await loadGroups();
        await loadCollaborationData();
        if (me.value.user.role === 'SOFTWARE_ADMIN' || me.value.user.role === 'SITE_ADMIN') {
          await loadManagedApps();
          await loadReviews();
        } else {
          setManagedApps([]);
        }
      } else {
        setStorageOptions([]);
        setManagedApps([]);
        setCollaborationData({ owned: [], collaborating: [], outgoingRequests: [] });
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

  async function applySiteProfile(site?: SiteProfile) {
    if (site) {
      setSiteProfile(site);
      return;
    }
    await loadSiteProfile();
  }

  async function loadStorageOptions() {
    if (!HAS_API) return;
    try {
      const data = await api<{ storages: StorageOption[]; defaultKey: string }>('/api/v1/storage-options');
      const options = arrayOrEmpty(data.storages).map((storage) => ({ ...storage, isDefault: storage.isDefault || storage.key === data.defaultKey }));
      setStorageOptions(options);
    } catch {
      setStorageOptions([]);
    }
  }

  async function loadManagedApps() {
    if (!HAS_API) return;
    try {
      const data = await api<{ apps: StoreApp[] }>('/api/v1/apps?managed=1');
      setManagedApps(arrayOrEmpty(data.apps));
    } catch {
      setManagedApps([]);
    }
  }

  async function loadCollaborationData() {
    if (!HAS_API) return;
    try {
      const data = await api<CollaborationData>('/api/v1/me/collaboration');
      setCollaborationData({
        owned: arrayOrEmpty(data.owned),
        collaborating: arrayOrEmpty(data.collaborating),
        outgoingRequests: arrayOrEmpty(data.outgoingRequests),
      });
    } catch {
      setCollaborationData({ owned: [], collaborating: [], outgoingRequests: [] });
    }
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
    setManagedApps([]);
    setCollaborationData({ owned: [], collaborating: [], outgoingRequests: [] });
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
      const categoryMatch = activeCategory === 'all' || String(app.categoryId || '') === activeCategory;
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

  async function openApp(app: StoreApp, mode: AppDetailMode = 'detail') {
    await runAction(setToast, t('toast.loadAppDetailFailed'), async () => {
      const data = await api<{ app: StoreApp }>(`/api/v1/apps/${app.id}`);
      setSelectedAppMode(mode);
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
      navigateTo('sources');
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
      navigateTo('search');
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
      <Theme theme={selectedAstryxTheme.theme} mode={themeMode}>
        <SetupWizard
          onComplete={async (nextUser) => {
            setUser(nextUser);
            setSetupRequired(false);
            navigateTo('profile');
            await refreshAll();
          }}
          setToast={setToast}
          themeMode={themeMode}
          onThemeModeChange={setThemeMode}
          astryxThemeName={astryxThemeName}
          onAstryxThemeChange={setAstryxThemeName}
        />
        {toast && <div className={cx('toast', toast.tone)}>{toast.message}</div>}
      </Theme>
    );
  }

  return (
    <Theme theme={selectedAstryxTheme.theme} mode={themeMode}>
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
        <XTabList className="nav" value={tab} onChange={(value) => navigateTo(value as TabKey)} orientation="vertical" aria-label={t('common.navigation')}>
          {navItems.map((item) => {
            const Icon = item.icon;
            return <XTab key={item.key} value={item.key} label={t(item.labelKey)} icon={<Icon size={19} />} />;
          })}
        </XTabList>
        {HAS_API && siteProfile.publicUrl && (
          <footer className="app-version" title={siteProfile.version ? t('site.serverVersion', { version: siteProfile.version }) : undefined} aria-label={siteProfile.version ? t('site.serverVersion', { version: siteProfile.version }) : undefined}>
            {t('site.footer', { url: siteProfile.publicUrl })}
          </footer>
        )}
      </aside>

      <main className="main" id="main-content" tabIndex={-1} inert={drawerOpen} aria-hidden={drawerOpen ? true : undefined}>
        <header className="topbar">
          <div className="topbar-search">
            <XTextInput
              label={HAS_API ? t('topbar.searchStore') : t('topbar.searchSources')}
              isLabelHidden
              startIcon={<Search size={16} />}
              value={query}
              onChange={setQuery}
              placeholder={HAS_API ? t('topbar.searchStore') : t('topbar.searchSources')}
              hasClear
              width="100%"
            />
          </div>
          <div className="top-actions">
            <LanguageSelector value={currentLanguage} onChange={(language) => void i18n.changeLanguage(language)} />
            <ThemeToggle mode={themeMode} onChange={setThemeMode} />
            <AstryxThemeSelector value={astryxThemeName} onChange={setAstryxThemeName} />
            <XIconButton
              type="button"
              variant="ghost"
              label={HAS_API ? t('topbar.refreshStore') : t('topbar.syncAllSources')}
              icon={<RefreshCw size={18} />}
              onClick={() => void (HAS_API ? refreshAll() : syncAllSources())}
            />
            {HAS_API && user ? (
              <div className="account-menu">
                <button type="button" className="account-trigger" onClick={() => setIsAccountMenuOpen((open) => !open)} aria-haspopup="menu" aria-expanded={isAccountMenuOpen}>
                  <UserAvatar user={user} size={32} />
                  <span>{displayUserName(user)}</span>
                  <ChevronDown size={15} aria-hidden="true" />
                </button>
                {isAccountMenuOpen && (
                  <div className="account-menu-popover" role="menu">
                    <button
                      type="button"
                      role="menuitem"
                      onClick={() => {
                        setIsAccountMenuOpen(false);
                        setIsProfileDialogOpen(true);
                      }}
                    >
                      <UserRound size={16} />
                      <span>{t('profile.personalProfile')}</span>
                    </button>
                    <button
                      type="button"
                      role="menuitem"
                      onClick={() =>
                        void runAction(setToast, t('toast.logoutFailed'), async () => {
                          await api('/api/v1/auth/logout', { method: 'POST' });
                          setUser(null);
                          setStorageOptions([]);
                          setIsAccountMenuOpen(false);
                        })
                      }
                    >
                      <LogOut size={16} />
                      <span>{t('auth.logout')}</span>
                    </button>
                  </div>
                )}
              </div>
            ) : HAS_API ? (
              <XButton type="button" variant="secondary" label={t('topbar.login')} icon={<LogIn size={16} />} onClick={() => navigateTo('profile')} />
            ) : null}
          </div>
        </header>

        {HAS_API && user && isProfileDialogOpen && (
          <ProfileSettingsDialog
            user={user}
            storageOptions={storageOptions}
            onClose={() => setIsProfileDialogOpen(false)}
            onSaved={(nextUser) => {
              setUser(nextUser);
              setIsProfileDialogOpen(false);
              void refreshAll({ silent: true });
            }}
            setToast={setToast}
          />
        )}

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
            {HAS_API && selectedApp ? (
              <AppDrawer
                app={selectedApp}
                mode={selectedAppMode}
                onModeChange={setSelectedAppMode}
                user={user}
                groups={groups}
                categories={categories}
                tagOptions={tagOptions}
                storageOptions={storageOptions}
                onClose={() => {
                  setSelectedApp(null);
                  setSelectedAppMode('detail');
                }}
                onInstall={installApp}
                onRefresh={async () => {
                  await openApp(selectedApp, selectedAppMode);
                  await refreshAll();
                }}
                onListRefresh={refreshAll}
                setToast={setToast}
              />
            ) : !HAS_API && selectedSourceApp ? (
              <SourceAppDetailPage
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
                setToast={setToast}
              />
            ) : (
            <>
            {tab === 'home' && (
              <StorefrontHome
                apps={filteredApps}
                categories={categories}
                collections={collections}
                siteProfile={siteProfile}
                onOpen={openApp}
                onInstall={installApp}
                onNavigate={navigateTo}
                onSubmitApp={openSubmitApp}
                onCategory={(category) => {
                  setActiveCategory(category);
                  navigateTo('search');
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
                onGoSources={() => navigateTo('sources')}
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
                managedApps={managedApps}
                groups={groups}
                setGroups={setGroups}
                categories={categories}
                tagOptions={tagOptions}
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
                siteProfile={siteProfile}
                storageOptions={storageOptions}
                collaborationData={collaborationData}
                onCollaborationRefresh={loadCollaborationData}
                openSubmitSignal={openSubmitSignal}
                onNavigate={navigateTo}
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
                <AdminPanel user={user} reviews={reviews} onApprove={approveReview} onSiteProfileSaved={applySiteProfile} onStorageOptionsChanged={loadStorageOptions} setToast={setToast} />
              ) : (
                <EmptyState
                  icon={ShieldCheck}
                  title={user ? t('admin.noPermission') : t('auth.loginRequired')}
                  body={user ? t('admin.noPermissionBody') : t('auth.loginRequiredBody')}
                  action={!user ? { label: t('auth.login'), icon: LogIn, onClick: () => navigateTo('profile') } : undefined}
                />
              )
            )}
            </>
            )}
          </>
        )}
      </main>

      <MobileTabs tab={tab} setTab={navigateTo} items={navItems} inert={drawerOpen} />

      {HAS_API && siteProfile.publicUrl && (
        <footer className="mobile-app-version" title={siteProfile.version ? t('site.serverVersion', { version: siteProfile.version }) : undefined} aria-label={siteProfile.version ? t('site.serverVersion', { version: siteProfile.version }) : undefined}>
          {t('site.footer', { url: siteProfile.publicUrl })}
        </footer>
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
          <XIconButton type="button" variant="ghost" label={t('installActivity.dismiss')} icon={<X size={17} />} onClick={() => setInstallActivity(null)} />
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
        <XIconButton type="button" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onCancel} />
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
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onCancel} />
          <XButton type="submit" variant="primary" label={t('installPassword.confirm')} icon={<Download size={17} />} />
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
  astryxThemeName,
  onAstryxThemeChange,
}: {
  onComplete: (user: User) => Promise<void>;
  setToast: (toast: Toast) => void;
  themeMode: ThemeMode;
  onThemeModeChange: (mode: ThemeMode) => void;
  astryxThemeName: AstryxThemeName;
  onAstryxThemeChange: (theme: AstryxThemeName) => void;
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
            <LanguageSelector value={currentLanguage} onChange={(language) => void i18n.changeLanguage(language)} />
            <ThemeToggle mode={themeMode} onChange={onThemeModeChange} />
            <AstryxThemeSelector value={astryxThemeName} onChange={onAstryxThemeChange} />
          </div>
          <XTextInput label={t('common.username')} value={form.username} onChange={(value) => setForm({ ...form, username: value })} />
          <XTextInput type="email" label={t('common.email')} value={form.email} onChange={(value) => setForm({ ...form, email: value })} />
          <XTextInput type="password" label={t('common.password')} value={form.password} onChange={(value) => setForm({ ...form, password: value })} />
          <XTextInput type="password" label={t('setup.confirmPassword')} value={form.confirmPassword} onChange={(value) => setForm({ ...form, confirmPassword: value })} />
          <XSwitch
            label={t('setup.protectSource')}
            value={form.sourcePasswordEnabled}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => setForm({ ...form, sourcePasswordEnabled: checked })}
          />
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
          <XSwitch
            label={t('setup.requireEmailVerify')}
            value={form.requireEmailVerify}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => setForm({ ...form, requireEmailVerify: checked })}
          />
          <XButton
            type="submit"
            variant="primary"
            label={submitting ? t('setup.submitting') : t('setup.finish')}
            icon={<ShieldCheck size={18} />}
            isDisabled={submitting}
          />
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
          <XButton
            variant="secondary"
            size="sm"
            label={announcement.linkLabel || t('site.announcementLink')}
            icon={<Link size={16} />}
            href={announcement.linkUrl}
            target="_blank"
            rel="noreferrer"
          />
        )}
        {onDismiss && (
          <XIconButton type="button" variant="ghost" label={t('site.dismissAnnouncement')} icon={<X size={17} />} onClick={onDismiss} />
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
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
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


function SourceAppDetailPage({
  app,
  installedMatch,
  installedState,
  onClose,
  onInstall,
  onLoadInstalled,
  onRefreshSourceApp,
  setToast,
}: {
  app: SourceApp;
  installedMatch?: InstalledApplication;
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  onClose: () => void;
  onInstall: (app: SourceApp, options?: { version?: string }) => void;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onRefreshSourceApp: () => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const requiredLabel = (label: string) => `${label} · ${t('common.required')}`;
  const backButtonRef = useRef<HTMLButtonElement>(null);
  const detailTitleId = `source-app-detail-title-${app.sourceId || app.sourceName}-${app.id}`;
  const [comments, setComments] = useState<Comment[]>([]);
  const [commentsState, setCommentsState] = useState<'idle' | 'loading' | 'loaded' | 'error'>('idle');
  const [commentText, setCommentText] = useState('');
  const [replyTarget, setReplyTarget] = useState<number | null>(null);
  const [replyText, setReplyText] = useState('');
  const [isOutdatedFormOpen, setIsOutdatedFormOpen] = useState(false);
  const [outdatedCount, setOutdatedCount] = useState(app.outdatedMarks ?? 0);
  const [outdatedForm, setOutdatedForm] = useState({
    note: '',
    installedVersion: installedMatch?.version || app.latestVersion?.version || '',
    expectedVersion: '',
  });
  const preferredScreenshotDevice = usePreferredScreenshotDevice();
  const latestVersion = app.latestVersion;
  const sourceVersions = app.versions && app.versions.length > 0 ? app.versions : latestVersion ? [latestVersion] : [];
  const sourceScreenshots = orderedScreenshots(app.screenshots, preferredScreenshotDevice);
  const installable = hasInstallableVersion(app);
  const installAction = sourceInstallAction(app, installedMatch);
  const isUpdateAvailable = installAction === 'update';
  const hasChecksum = Boolean(latestVersion?.sha256);
  const hasSize = Boolean(latestVersion?.size && latestVersion.size > 0);
  const hasOutdatedMarks = outdatedCount > 0;
  const sourceCommentsEnabled = app.commentsEnabled !== false;
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
    backButtonRef.current?.focus();
  }, [app.id]);

  useEffect(() => {
    void loadSourceComments();
    setCommentText('');
    setReplyTarget(null);
    setReplyText('');
    setIsOutdatedFormOpen(false);
    setOutdatedCount(app.outdatedMarks ?? 0);
    setOutdatedForm({
      note: '',
      installedVersion: installedMatch?.version || app.latestVersion?.version || '',
      expectedVersion: '',
    });
  }, [app.id]);

  useEffect(() => {
    setOutdatedCount(app.outdatedMarks ?? 0);
  }, [app.outdatedMarks]);

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
    if (!sourceCommentsEnabled) {
      setToast({ tone: 'neutral', message: t('sourceDetail.commentsDisabled') });
      return;
    }
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

  async function submitSourceOutdated(event: FormEvent) {
    event.preventDefault();
    const note = outdatedForm.note.trim();
    if (!note) return;
    try {
      const data = await clientApi<{ outdatedMarks?: number }>(`/apps/${app.id}/outdated-marks`, {
        method: 'POST',
        body: JSON.stringify({
          note,
          installedVersion: outdatedForm.installedVersion.trim(),
          expectedVersion: outdatedForm.expectedVersion.trim(),
        }),
      });
      setOutdatedCount(typeof data.outdatedMarks === 'number' ? data.outdatedMarks : Math.max(outdatedCount, 1));
      setOutdatedForm((current) => ({ ...current, note: '', expectedVersion: '' }));
      setIsOutdatedFormOpen(false);
      setToast({ tone: 'success', message: t('sourceDetail.outdatedSubmitted') });
      await onRefreshSourceApp();
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('sourceDetail.outdatedSubmitFailed')) });
    }
  }

  return (
    <section className="detail-page-shell">
      <article className="detail-page" aria-labelledby={detailTitleId}>
        <XButton ref={backButtonRef} type="button" variant="secondary" size="sm" label={t('common.back')} icon={<ArrowLeft size={17} />} onClick={onClose} />
        <header className="detail-head">
          <AppIcon src={app.iconUrl} seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} className="detail-avatar" />
          <div>
            <span className="eyebrow subtle">{t('sourceDetail.eyebrow')}</span>
            <h2 id={detailTitleId}>{app.name}</h2>
            <p>{app.summary || t('common.lpkApp')}</p>
            <div className="app-meta">
              <span><Cloud size={14} /> {app.sourceName}</span>
              <span><Tag size={14} /> {localizedCategory(app, t('common.uncategorized'))}</span>
              <span><Star size={14} /> {latestVersion?.version || t('app.noPublishedVersion')}</span>
              {installedMatch && (
                <span className={cx('status-badge', isUpdateAvailable ? 'pending' : 'synced')}>
                  {isUpdateAvailable ? <RefreshCw size={13} /> : <Check size={13} />}
                  {isUpdateAvailable ? t('app.updateAvailable') : t('app.installed')}
                </span>
              )}
              {hasOutdatedMarks && (
                <span className="status-badge stale">
                  <AlertCircle size={13} />
                  {t('sourceDetail.outdatedBadge', { count: outdatedCount })}
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
            <XButton
              type="button"
              variant="primary"
              label={sourceActionLabel(t, installAction)}
              icon={isUpdateAvailable ? <RefreshCw size={17} /> : <Download size={17} />}
              isDisabled={!installable}
              onClick={() => void onInstall(app)}
            />
            <XButton
              type="button"
              variant="secondary"
              label={installedState === 'loading' ? t('profile.readingInstalled') : t('profile.readInstalled')}
              icon={<RefreshCw size={17} />}
              isDisabled={installedState === 'loading'}
              onClick={() => void onLoadInstalled()}
            />
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

        <section className={cx('outdated-state', hasOutdatedMarks && 'active')} aria-label={t('sourceDetail.outdatedTitle')}>
          <div className="outdated-state-head">
            <AlertCircle size={19} />
            <div>
              <strong>{hasOutdatedMarks ? t('sourceDetail.outdatedActiveTitle', { count: outdatedCount }) : t('sourceDetail.outdatedTitle')}</strong>
              <span>{hasOutdatedMarks ? t('sourceDetail.outdatedActiveBody') : t('sourceDetail.outdatedBody')}</span>
            </div>
            <XButton
              type="button"
              variant="secondary"
              size="sm"
              label={hasOutdatedMarks ? t('sourceDetail.updateOutdatedInfo') : t('sourceDetail.markOutdated')}
              icon={<AlertCircle size={17} />}
              onClick={() => setIsOutdatedFormOpen((open) => !open)}
            />
          </div>
          {isOutdatedFormOpen && (
            <form className="outdated-form" onSubmit={(event) => void submitSourceOutdated(event)}>
              <XTextArea
                label={requiredLabel(t('sourceDetail.outdatedReason'))}
                value={outdatedForm.note}
                rows={3}
                isRequired
                onChange={(value) => setOutdatedForm({ ...outdatedForm, note: value })}
              />
              <div className="outdated-form-grid">
                <XTextInput
                  label={t('sourceDetail.currentVersion')}
                  value={outdatedForm.installedVersion}
                  onChange={(value) => setOutdatedForm({ ...outdatedForm, installedVersion: value })}
                />
                <XTextInput
                  label={t('sourceDetail.expectedVersion')}
                  value={outdatedForm.expectedVersion}
                  onChange={(value) => setOutdatedForm({ ...outdatedForm, expectedVersion: value })}
                />
              </div>
              <p className="field-help">{t('sourceDetail.outdatedHelp')}</p>
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setIsOutdatedFormOpen(false)} />
                <XButton type="submit" variant="primary" label={t('sourceDetail.submitOutdated')} icon={<AlertCircle size={18} />} isDisabled={!outdatedForm.note.trim()} />
              </div>
            </form>
          )}
        </section>

        <section className="detail-summary" aria-label={t('drawer.metadata')}>
          <div>
            <span>{t('sourceDetail.source')}</span>
            <strong>{app.sourceName}</strong>
          </div>
          <div>
            <span>{t('common.category')}</span>
            <strong>{localizedCategory(app, t('common.uncategorized'))}</strong>
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

        <section>
          <div className="section-title">
            <Archive size={19} />
            <h3>{t('drawer.screenshots')}</h3>
          </div>
          {sourceScreenshots.length > 0 ? (
            <div className="screenshot-grid">
              {sourceScreenshots.map((shot) => (
                <figure className="screenshot-item" key={`${shot.deviceType || 'DESKTOP'}-${shot.imageUrl}`}>
                  <img src={shot.imageUrl} alt={shot.caption || app.name} />
                  <figcaption>
                    <span>{shot.caption || app.name}</span>
                    <small>{screenshotDeviceLabel(t, shot.deviceType)}</small>
                  </figcaption>
                </figure>
              ))}
            </div>
          ) : (
            <EmptyState icon={Archive} title={t('drawer.noScreenshots')} />
          )}
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
                      <XButton
                        type="button"
                        variant="secondary"
                        size="sm"
                        label={isLatest ? t('common.install') : t('sourceDetail.rollback')}
                        icon={<Download size={17} />}
                        isDisabled={!canInstallVersion}
                        onClick={() => void onInstall(app, { version: version.version })}
                      />
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
            <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={17} />} onClick={() => void loadSourceComments()} />
          </div>
          {commentsState === 'error' && <p className="inline-warning"><AlertCircle size={15} /><span>{t('sourceDetail.commentsUnavailable')}</span></p>}
          {sourceCommentsEnabled ? (
            <form className="comment-form rich-comment-form" onSubmit={(event) => void submitSourceComment(event)}>
              <XTextInput
                label={t('drawer.commentPlaceholder')}
                isLabelHidden
                value={commentText}
                placeholder={t('drawer.commentPlaceholder')}
                onChange={setCommentText}
              />
              <XIconButton type="submit" variant="ghost" label={t('drawer.postComment')} icon={<MessageSquare size={17} />} isDisabled={!commentText.trim()} />
            </form>
          ) : (
            <div className="comment-disabled-note" role="note">
              <MessageSquareOff size={17} />
              <span>{t('sourceDetail.commentsDisabled')}</span>
            </div>
          )}
          <CommentList
            comments={comments}
            commentsState={commentsState}
            canReply={sourceCommentsEnabled}
            replyTarget={replyTarget}
            replyText={replyText}
            onReplyTarget={setReplyTarget}
            onReplyText={setReplyText}
            onReply={(event, parentId) => void submitSourceComment(event, parentId)}
            onDelete={(commentId) => void deleteSourceComment(commentId)}
          />
        </section>
      </article>
    </section>
  );
}

function CommentList({
  comments,
  commentsState = 'loaded',
  canReply = true,
  replyTarget,
  replyText,
  onReplyTarget,
  onReplyText,
  onReply,
  onDelete,
}: {
  comments: Comment[];
  commentsState?: 'idle' | 'loading' | 'loaded' | 'error';
  canReply?: boolean;
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
          {canReply && (
            <div className="comment-actions">
              <XButton type="button" variant="secondary" size="sm" label={t('drawer.reply')} icon={<MessageSquare size={15} />} onClick={() => onReplyTarget(replyTarget === comment.id ? null : comment.id)} />
            </div>
          )}
          {canReply && replyTarget === comment.id && (
            <form className="comment-form rich-comment-form reply-form" onSubmit={(event) => onReply(event, comment.id)}>
              <XTextInput
                label={t('drawer.replyPlaceholder')}
                isLabelHidden
                value={replyText}
                placeholder={t('drawer.replyPlaceholder')}
                onChange={onReplyText}
              />
              <XIconButton type="submit" variant="ghost" label={t('drawer.postReply')} icon={<MessageSquare size={17} />} isDisabled={!replyText.trim()} />
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
          <XIconButton type="button" variant="destructive" label={t('drawer.deleteComment')} icon={<Trash2 size={15} />} onClick={() => onDelete(comment.id)} />
        )}
      </div>
      <p>{comment.body}</p>
    </>
  );
}

function ProfileSettingsDialog({
  user,
  storageOptions,
  onClose,
  onSaved,
  setToast,
}: {
  user: User;
  storageOptions: StorageOption[];
  onClose: () => void;
  onSaved: (user: User) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState({
    nickname: user.nickname || '',
    email: user.email || '',
    currentPassword: '',
    newPassword: '',
  });
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarStorageKey, setAvatarStorageKey] = useState(defaultUploadStorageKey(storageOptions));
  const storageChoices = storageSelectOptions(storageOptions);

  useEffect(() => {
    setDraft({
      nickname: user.nickname || '',
      email: user.email || '',
      currentPassword: '',
      newPassword: '',
    });
  }, [user]);

  useEffect(() => {
    const fallback = defaultUploadStorageKey(storageOptions);
    setAvatarStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
  }, [storageOptions]);

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('profile.profileSaveFailed'), async () => {
      let nextUser = user;
      const profileData = await api<{ user: User }>('/api/v1/me/profile', {
        method: 'PATCH',
        body: JSON.stringify({
          nickname: draft.nickname,
          email: draft.email,
          currentPassword: draft.currentPassword,
          newPassword: draft.newPassword,
        }),
      });
      nextUser = profileData.user;
      if (avatarFile) {
        const form = new FormData();
        form.set('file', avatarFile);
        form.set('storageKey', avatarStorageKey);
        const avatarData = await api<{ user: User; url: string }>('/api/v1/me/avatar', { method: 'POST', body: form });
        nextUser = avatarData.user;
      }
      setToast({ tone: 'success', message: t('profile.profileSaved') });
      onSaved(nextUser);
    });
  }

  return (
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <form className="modal-panel form-panel profile-dialog" role="dialog" aria-modal="true" aria-label={t('profile.personalProfile')} onClick={(event) => event.stopPropagation()} onSubmit={saveProfile}>
        <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={onClose} />
        <div className="profile-dialog-head">
          <UserAvatar user={user} size={72} className="avatar-large" />
          <div>
            <h2>{t('profile.personalProfile')}</h2>
            <p>{displayUserName(user)}</p>
          </div>
        </div>
        <XFormLayout>
          <XTextInput label={t('profile.nickname')} value={draft.nickname} onChange={(value) => setDraft({ ...draft, nickname: value })} />
          <XTextInput type="email" label={t('common.email')} value={draft.email} onChange={(value) => setDraft({ ...draft, email: value })} />
          <XTextInput type="password" label={t('profile.currentPassword')} value={draft.currentPassword} onChange={(value) => setDraft({ ...draft, currentPassword: value })} />
          <XTextInput type="password" label={t('profile.newPassword')} description={t('profile.newPasswordHelp')} value={draft.newPassword} onChange={(value) => setDraft({ ...draft, newPassword: value })} />
        </XFormLayout>
        <div className="avatar-upload-grid">
          <FilePicker
            label={t('profile.avatar')}
            help={t('profile.avatarHelp')}
            value={avatarFile}
            accept=".png,.jpg,.jpeg,.webp"
            onChange={(nextFile) => setAvatarFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
          />
          {storageChoices.length > 0 && (
            <XSelector
              label={t('common.storage')}
              value={avatarStorageKey}
              options={storageChoices}
              onChange={setAvatarStorageKey}
            />
          )}
        </div>
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={onClose} />
          <XButton type="submit" variant="primary" label={t('profile.saveProfile')} icon={<Save size={17} />} />
        </div>
      </form>
    </div>
  );
}

function ProfileView({
  user,
  setUser,
  apps,
  managedApps,
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
  siteProfile,
  storageOptions,
  collaborationData,
  onCollaborationRefresh,
  tagOptions,
  openSubmitSignal,
  onNavigate,
}: {
  user: User | null;
  setUser: (user: User | null) => void;
  apps: StoreApp[];
  managedApps: StoreApp[];
  groups: Group[];
  setGroups: (groups: Group[]) => void;
  categories: Category[];
  sourceApps: SourceApp[];
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  refreshAll: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
  hasAPI: boolean;
  siteProfile: SiteProfile;
  storageOptions: StorageOption[];
  collaborationData: CollaborationData;
  onCollaborationRefresh: () => Promise<void>;
  tagOptions: string[];
  openSubmitSignal: number;
  onNavigate: (tab: TabKey) => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'login' | 'register' | 'verify'>('login');
  const [workspaceTab, setWorkspaceTab] = useState<ProfileWorkspaceTab>(() => (collaborationInviteTokenFromURL() ? 'collaboration' : 'overview'));
  const [isSubmitOpen, setIsSubmitOpen] = useState(false);
  const [managedSubmitter, setManagedSubmitter] = useState('all');
  const [authForm, setAuthForm] = useState({ username: '', password: '', email: '', inviteCode: '' });
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
    useMirrorDownload: true,
    installPassword: '',
  });
  const [recentSubmission, setRecentSubmission] = useState<{ name: string; status: string } | null>(null);
  const [isSubmittingApp, setIsSubmittingApp] = useState(false);
  const [submissionProgress, setSubmissionProgress] = useState<SubmissionProgress | null>(null);
  const [artifactMode, setArtifactMode] = useState<SubmissionArtifactMode>('local');
  const [uploadStorageKey, setUploadStorageKey] = useState(defaultUploadStorageKey(storageOptions));
  const [file, setFile] = useState<File | null>(null);
  const [desktopScreenshotFiles, setDesktopScreenshotFiles] = useState<File[]>([]);
  const [mobileScreenshotFiles, setMobileScreenshotFiles] = useState<File[]>([]);
  const [desktopScreenshotCaptions, setDesktopScreenshotCaptions] = useState<Record<string, string>>({});
  const [mobileScreenshotCaptions, setMobileScreenshotCaptions] = useState<Record<string, string>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [newToken, setNewToken] = useState('');
  const [isTokenHelpOpen, setIsTokenHelpOpen] = useState(false);
  const [favorites, setFavorites] = useState<FavoriteData>({ apps: [], submitters: [] });
  const [commentNotifications, setCommentNotifications] = useState<CommentNotification[]>([]);
  const authModeLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verify');
  const authSubmitLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verifyEmail');
  const registrationMode = siteProfile.registration?.mode || 'open';
  const registrationOpen = registrationMode !== 'closed';
  const inviteRegistration = registrationMode === 'invite';
  const authHint = mode === 'login'
    ? t('auth.loginHint')
    : mode === 'register'
      ? t(inviteRegistration ? 'auth.registerInviteHint' : 'auth.registerHint')
      : t('auth.verifyHint');
  const AuthSubmitIcon = mode === 'verify' ? Check : mode === 'register' ? Plus : LogIn;
  const requiredLabel = (label: string) => `${label} · ${t('common.required')}`;
  const canUseManagementWorkspace = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const workspaceTabs = [
    { key: 'overview' as const, label: t('profile.tabs.overview'), icon: Gauge },
    { key: 'apps' as const, label: t('profile.tabs.apps'), icon: PackagePlus },
    { key: 'collaboration' as const, label: t('profile.tabs.collaboration'), icon: Users },
    ...(canUseManagementWorkspace ? [{ key: 'manage' as const, label: t('profile.tabs.manage'), icon: Settings }] : []),
    { key: 'tokens' as const, label: t('profile.tabs.tokens'), icon: KeyRound },
    { key: 'groups' as const, label: t('profile.tabs.groups'), icon: Users },
    { key: 'favorites' as const, label: t('profile.tabs.favorites'), icon: Heart },
  ];
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
  const managedSubmitterOptions = useMemo(() => {
    const users = new Map<number, string>();
    managedApps.forEach((app) => {
      users.set(app.ownerId, app.owner || String(app.ownerId));
    });
    return [
      { value: 'all', label: t('profile.allMaintainers') },
      ...Array.from(users, ([id, label]) => ({ value: String(id), label })).sort((a, b) => a.label.localeCompare(b.label)),
    ];
  }, [managedApps, t]);
  const manageableApps = useMemo(() => {
    const filtered = managedSubmitter === 'all'
      ? managedApps
      : managedApps.filter((app) => String(app.ownerId) === managedSubmitter);
    return [...filtered].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
  }, [managedApps, managedSubmitter]);
  const publishSummary = useMemo(() => {
    return {
      total: ownedApps.length,
      approved: ownedApps.filter((app) => app.status === 'APPROVED').length,
      pending: ownedApps.filter((app) => app.status === 'PENDING').length,
      needsVersion: ownedApps.filter((app) => app.status === 'APPROVED' && !hasInstallableVersion(app)).length,
    };
  }, [ownedApps]);
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
  const storageChoices = storageSelectOptions(storageOptions);

  useEffect(() => {
    const fallback = defaultUploadStorageKey(storageOptions);
    setUploadStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
  }, [storageOptions]);

  useEffect(() => {
    if (!user || openSubmitSignal === 0) return;
    setWorkspaceTab('apps');
    setIsSubmitOpen(true);
  }, [openSubmitSignal, user]);

  useEffect(() => {
    if (workspaceTab === 'manage' && !canUseManagementWorkspace) {
      setWorkspaceTab('overview');
    }
  }, [canUseManagementWorkspace, workspaceTab]);

  useEffect(() => {
    if (managedSubmitter === 'all') return;
    if (!managedSubmitterOptions.some((option) => option.value === managedSubmitter)) {
      setManagedSubmitter('all');
    }
  }, [managedSubmitter, managedSubmitterOptions]);

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
    if (!registrationOpen && mode === 'register') {
      setMode('login');
    }
  }, [mode, registrationOpen]);

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

  function updateDesktopScreenshotFiles(files: File[]) {
    setDesktopScreenshotFiles(files);
    setDesktopScreenshotCaptions((current) => reconcileScreenshotCaptions(files, current));
  }

  function updateMobileScreenshotFiles(files: File[]) {
    setMobileScreenshotFiles(files);
    setMobileScreenshotCaptions((current) => reconcileScreenshotCaptions(files, current));
  }

  function updateDesktopScreenshotCaption(file: File, caption: string) {
    setDesktopScreenshotCaptions((current) => ({ ...current, [screenshotFileKey(file)]: caption }));
  }

  function updateMobileScreenshotCaption(file: File, caption: string) {
    setMobileScreenshotCaptions((current) => ({ ...current, [screenshotFileKey(file)]: caption }));
  }

  function startSubmissionStageProgress(label: string, initialPercent = 12, maxPercent = 86) {
    setSubmissionProgress({ percent: initialPercent, label });
    const timer = window.setInterval(() => {
      setSubmissionProgress((current) => {
        if (!current) return current;
        const remaining = maxPercent - current.percent;
        if (remaining <= 0) return current;
        return { ...current, percent: Math.min(maxPercent, current.percent + Math.max(1, remaining * 0.16)) };
      });
    }, 700);
    return () => window.clearInterval(timer);
  }

  async function submitUpload(event: FormEvent) {
    event.preventDefault();
    if (isSubmittingApp) return;
    if (artifactMode === 'local' && !file) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (artifactMode === 'external' && !uploadForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    setIsSubmittingApp(true);
    setSubmissionProgress({ percent: 6, label: t('submitApp.progressPreparing') });
    let stopStageProgress: (() => void) | undefined;
    try {
      await runAction(setToast, t('submitApp.failed'), async () => {
        let created: { app?: StoreApp };
        if (artifactMode === 'local' && file) {
          const form = new FormData();
          Object.entries(uploadForm).forEach(([key, value]) => form.set(key, String(value)));
          form.set('file', file);
          form.set('storageKey', uploadStorageKey);
          created = await apiWithUploadProgress<{ app: StoreApp }>('/api/v1/apps', {
            method: 'POST',
            body: form,
            onUploadProgress: (percent) => {
              setSubmissionProgress({
                percent: Math.min(82, Math.max(8, Math.round(percent * 0.76))),
                label: percent >= 100 ? t('submitApp.progressVerifying') : t('submitApp.progressUploading'),
              });
            },
          });
        } else {
          stopStageProgress = startSubmissionStageProgress(
            uploadForm.sourceType === 'GITHUB' && uploadForm.useMirrorDownload
              ? t('submitApp.progressMirrorFetching')
              : t('submitApp.progressRemoteFetching'),
          );
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
              useMirrorDownload: uploadForm.useMirrorDownload,
              ...(uploadForm.installPassword.trim() ? { installPassword: uploadForm.installPassword.trim() } : {}),
            }),
          });
        }
        stopStageProgress?.();
        stopStageProgress = undefined;
        if (created.app?.id) {
          setSubmissionProgress({ percent: 88, label: t('submitApp.progressScreenshots') });
          await uploadInitialScreenshots(created.app.id, desktopScreenshotFiles, desktopScreenshotCaptions, 'DESKTOP');
          await uploadInitialScreenshots(created.app.id, mobileScreenshotFiles, mobileScreenshotCaptions, 'MOBILE');
        }
        setSubmissionProgress({ percent: 100, label: t('submitApp.progressDone') });
        setRecentSubmission({ name: created.app?.name || uploadForm.name, status: created.app?.status || 'PENDING' });
        setToast({ tone: 'success', message: t('submitApp.submitted') });
        setUploadForm({ name: '', version: '', summary: '', description: '', categoryId: '', tags: '', allowUnreviewedUpdates: false, emailNotificationsEnabled: true, sourceType: 'GITHUB', downloadUrl: '', sha256: '', useMirrorDownload: true, installPassword: '' });
        setArtifactMode('local');
        setFile(null);
        setDesktopScreenshotFiles([]);
        setMobileScreenshotFiles([]);
        setDesktopScreenshotCaptions({});
        setMobileScreenshotCaptions({});
        if (fileInputRef.current) fileInputRef.current.value = '';
        setIsSubmitOpen(false);
        setWorkspaceTab('apps');
        await refreshAll({ silent: true });
      });
    } finally {
      stopStageProgress?.();
      setSubmissionProgress(null);
      setIsSubmittingApp(false);
    }
  }

  async function uploadInitialScreenshots(appID: number, files: File[], captions: Record<string, string>, deviceType: 'DESKTOP' | 'MOBILE') {
    for (const screenshot of files) {
      const form = new FormData();
      form.set('file', screenshot);
      form.set('caption', (captions[screenshotFileKey(screenshot)] || '').trim());
      form.set('deviceType', deviceType);
      form.set('storageKey', uploadStorageKey);
      await api(`/api/v1/apps/${appID}/screenshots`, { method: 'POST', body: form });
    }
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
            <XButton type="button" variant="primary" label={t('profile.openSources')} icon={<Cloud size={18} />} onClick={() => onNavigate('sources')} />
            <XButton type="button" variant="secondary" label={t('profile.browseInstallable')} icon={<Search size={18} />} onClick={() => onNavigate('search')} />
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
            <XToggleButtonGroup value={mode} onChange={(value) => value && setMode(value as typeof mode)} label={t('auth.modeSwitch')} size="sm">
              <XToggleButton value="login" label={t('auth.login')} />
              {registrationOpen && <XToggleButton value="register" label={t('auth.register')} />}
              <XToggleButton value="verify" label={t('auth.verify')} />
            </XToggleButtonGroup>
            {mode === 'verify' ? (
              <XTextInput label={requiredLabel(t('auth.verifyToken'))} value={verifyToken} isRequired onChange={setVerifyToken} />
            ) : (
              <>
                <XTextInput label={requiredLabel(t('common.username'))} value={authForm.username} isRequired onChange={(value) => setAuthForm({ ...authForm, username: value })} />
                {mode === 'register' && (
                  <XTextInput type="email" label={t('common.email')} value={authForm.email} onChange={(value) => setAuthForm({ ...authForm, email: value })} />
                )}
                {mode === 'register' && inviteRegistration && (
                  <XTextInput label={requiredLabel(t('auth.inviteCode'))} value={authForm.inviteCode} isRequired onChange={(value) => setAuthForm({ ...authForm, inviteCode: value })} />
                )}
                <XTextInput
                  type="password"
                  label={requiredLabel(t('common.password'))}
                  description={mode === 'register' ? t('auth.passwordHelp') : undefined}
                  value={authForm.password}
                  isRequired
                  onChange={(value) => setAuthForm({ ...authForm, password: value })}
                />
              </>
            )}
            <XButton type="submit" variant="primary" label={authSubmitLabel} icon={<AuthSubmitIcon size={18} />} />
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
                <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathBrowseAction')} icon={<Home size={17} />} onClick={() => onNavigate('home')} />
              </div>
              {registrationOpen && (
                <div className="auth-path-row">
                  <PackagePlus size={19} />
                  <div>
                    <strong>{t('auth.pathSubmitTitle')}</strong>
                    <span>{t(inviteRegistration ? 'auth.pathSubmitInviteBody' : 'auth.pathSubmitBody')}</span>
                  </div>
                  <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathSubmitAction')} icon={<Plus size={17} />} onClick={() => setMode('register')} />
                </div>
              )}
              <div className="auth-path-row">
                <ShieldCheck size={19} />
                <div>
                  <strong>{t('auth.pathAdminTitle')}</strong>
                  <span>{t('auth.pathAdminBody')}</span>
                </div>
                <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathAdminAction')} icon={<LogIn size={17} />} onClick={() => setMode('login')} />
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
            <UserAvatar user={user} size={74} className="avatar-large" />
            <h2>{displayUserName(user)}</h2>
            <p>{t('auth.emailPending')}</p>
            <XButton
              type="button"
              variant="secondary"
              label={t('auth.logout')}
              icon={<LogOut size={18} />}
              onClick={() =>
                void runAction(setToast, t('toast.logoutFailed'), async () => {
                  await api('/api/v1/auth/logout', { method: 'POST' });
                  setUser(null);
                })
              }
            />
          </div>
          <form className="panel form-panel" onSubmit={submitVerification}>
            <SectionTitle icon={AlertCircle} title={t('auth.verifyEmail')} />
            <p className="inline-note">{t('auth.verificationHelp')}</p>
            <XTextInput label={t('auth.verifyToken')} value={verifyToken} onChange={setVerifyToken} />
            <XButton type="submit" variant="primary" label={t('auth.completeVerification')} icon={<Check size={18} />} />
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
      <div className="horizontal-control-scroll">
        <XToggleButtonGroup value={workspaceTab} onChange={(value) => value && setWorkspaceTab(value as typeof workspaceTab)} label={t('profile.tabs.label')} size="sm">
          {workspaceTabs.map((item) => {
            const Icon = item.icon;
            return (
              <XToggleButton
                key={item.key}
                value={item.key}
                label={item.label}
                icon={<Icon size={17} />}
              />
            );
          })}
        </XToggleButtonGroup>
      </div>
      {workspaceTab === 'overview' && (
      <div className="split">
        <div className="panel profile-card">
          <UserAvatar user={user} size={74} className="avatar-large" />
          <h2>{displayUserName(user)}</h2>
          <p>{t(`admin.roles.${user.role === 'SITE_ADMIN' ? 'siteAdmin' : user.role === 'SOFTWARE_ADMIN' ? 'softwareAdmin' : 'user'}`)}</p>
          <XButton
            type="button"
            variant="secondary"
            label={t('auth.logout')}
            icon={<LogOut size={18} />}
            onClick={() =>
              void runAction(setToast, t('toast.logoutFailed'), async () => {
                await api('/api/v1/auth/logout', { method: 'POST' });
                setUser(null);
              })
            }
          />
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
        <div className="section-title with-action">
          <div>
            <PackagePlus size={19} />
            <h2>{t('profile.mySubmissions')}</h2>
          </div>
          <XButton
            type="button"
            variant="primary"
            size="sm"
            label={isSubmitOpen ? t('common.close') : t('submitApp.title')}
            icon={isSubmitOpen ? <X size={17} /> : <Upload size={17} />}
            onClick={() => setIsSubmitOpen((open) => !open)}
          />
        </div>
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
                  <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                  {(canUserManageApp(user, item) || canUserUploadVersion(user, item)) && (
                    <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </section>
      )}

      {workspaceTab === 'apps' && isSubmitOpen && (
      <div className="modal-backdrop" role="presentation" onClick={() => setIsSubmitOpen(false)}>
        <AppSubmissionForm
          draft={uploadForm}
          onDraftChange={setUploadForm}
          categories={categories}
          knownTags={tagOptions}
          storageOptions={storageChoices}
          storageKey={uploadStorageKey}
          onStorageKeyChange={setUploadStorageKey}
          artifactMode={artifactMode}
          onArtifactModeChange={selectArtifactMode}
          file={file}
          onFileChange={setFile}
          fileInputRef={fileInputRef}
          desktopScreenshotFiles={desktopScreenshotFiles}
          desktopScreenshotCaptions={desktopScreenshotCaptions}
          onDesktopScreenshotFilesChange={updateDesktopScreenshotFiles}
          onDesktopScreenshotCaptionChange={updateDesktopScreenshotCaption}
          mobileScreenshotFiles={mobileScreenshotFiles}
          mobileScreenshotCaptions={mobileScreenshotCaptions}
          onMobileScreenshotFilesChange={updateMobileScreenshotFiles}
          onMobileScreenshotCaptionChange={updateMobileScreenshotCaption}
          recentSubmission={recentSubmission}
          isDirectPublishUser={user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN'}
          isSubmitting={isSubmittingApp}
          submissionProgress={submissionProgress}
          presentation="modal"
          onSubmit={submitUpload}
          onCancel={() => setIsSubmitOpen(false)}
        />
      </div>
      )}
      {workspaceTab === 'collaboration' && (
      <section className="workspace-pane">
        <CollaborationPanel
          data={collaborationData}
          currentUser={user}
          onOpen={onOpen}
          onRefresh={onCollaborationRefresh}
          onListRefresh={refreshAll}
          setToast={setToast}
        />
      </section>
      )}
      {workspaceTab === 'manage' && (
      <section className="workspace-pane">
        <section className="panel">
          <div className="section-title with-action">
            <div>
              <Settings size={19} />
              <h2>{t('profile.appManagement')}</h2>
            </div>
            <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={17} />} onClick={() => void refreshAll({ silent: true })} />
          </div>
          <div className="review-list management-app-list">
            {manageableApps.length === 0 ? (
              <EmptyState icon={Settings} title={t('profile.appManagementEmpty')} body={t('profile.appManagementEmptyBody')} />
            ) : (
              manageableApps.map((item) => (
                <div className="review-row management-app-row" key={item.id}>
                  <div>
                    <strong>{item.name}</strong>
                    <span>{item.owner} · {item.latestVersion?.version || t('app.noPublishedVersion')} · {formatDate(item.updatedAt)}</span>
                    <small className="workflow-hint">{t(`profile.submissionStep.${submissionStep(item).key}`)}</small>
                  </div>
                  <div className="row-actions management-row-actions">
                    <span className={cx('status-badge', submissionStep(item).tone)}>{t(`statusLabels.${statusKey(item.status)}`)}</span>
                    <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                    <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                  </div>
                </div>
              ))
            )}
          </div>
        </section>
      </section>
      )}
      {workspaceTab === 'tokens' && (
      <section className="workspace-pane">
        <section className="panel">
          <div className="section-title with-action">
            <div>
              <KeyRound size={19} />
              <h2>{t('token.title')}</h2>
            </div>
            <XIconButton type="button" variant="ghost" label={t('token.help')} icon={<HelpCircle size={17} />} onClick={() => setIsTokenHelpOpen(true)} />
          </div>
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
          <XButton type="button" variant="secondary" label={t('token.generate')} icon={<KeyRound size={18} />} onClick={() => void createToken()} />
        </section>
        {isTokenHelpOpen && <TokenHelpDialog onClose={() => setIsTokenHelpOpen(false)} />}
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
          <XButton type="button" variant="secondary" label={t('favorites.refresh')} icon={<RefreshCw size={18} />} onClick={() => void loadFavorites()} />
        </section>
      </section>
      )}
    </section>
  );
}

function CollaborationPanel({
  data,
  currentUser,
  onOpen,
  onRefresh,
  onListRefresh,
  setToast,
}: {
  data: CollaborationData;
  currentUser: User;
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  onRefresh: () => Promise<void>;
  onListRefresh: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [collaboratorDrafts, setCollaboratorDrafts] = useState<Record<number, string>>({});
  const [inviteDrafts, setInviteDrafts] = useState<Record<number, { email: string; sendEmail: boolean }>>({});

  const refreshCollaboration = async () => {
    await onRefresh();
    await onListRefresh({ silent: true });
  };

  async function copyInvite(invite: CollaboratorInvite) {
    await runAction(setToast, t('profile.inviteCopyFailed'), async () => {
      if (!invite.inviteUrl || !navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(invite.inviteUrl);
      setToast({ tone: 'success', message: t('profile.inviteCopied') });
    });
  }

  async function addCollaborator(item: OwnedCollaboration) {
    const identity = (collaboratorDrafts[item.app.id] || '').trim();
    if (!identity) return;
    const body = identity.includes('@') ? { email: identity } : { username: identity };
    await runAction(setToast, t('profile.addCollaboratorFailed'), async () => {
      await api(`/api/v1/apps/${item.app.id}/collaborators`, {
        method: 'POST',
        body: JSON.stringify(body),
      });
      setCollaboratorDrafts((current) => ({ ...current, [item.app.id]: '' }));
      setToast({ tone: 'success', message: t('profile.collaboratorAdded') });
      await refreshCollaboration();
    });
  }

  async function removeCollaborator(appID: number, userID: number, self = false) {
    await runAction(setToast, self ? t('profile.leaveCollaborationFailed') : t('profile.removeCollaboratorFailed'), async () => {
      await api(`/api/v1/apps/${appID}/collaborators/${userID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: self ? t('profile.collaborationLeft') : t('profile.collaboratorRemoved') });
      await refreshCollaboration();
    });
  }

  async function createInvite(item: OwnedCollaboration) {
    const draft = inviteDrafts[item.app.id] || { email: '', sendEmail: false };
    await runAction(setToast, t('profile.createInviteFailed'), async () => {
      const data = await api<{ invite: CollaboratorInvite; inviteUrl: string }>(`/api/v1/apps/${item.app.id}/collaborator-invites`, {
        method: 'POST',
        body: JSON.stringify({ email: draft.email.trim(), sendEmail: draft.sendEmail }),
      });
      setInviteDrafts((current) => ({ ...current, [item.app.id]: { email: '', sendEmail: false } }));
      setToast({ tone: 'success', message: t('profile.inviteCreated') });
      if (data.inviteUrl && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(data.inviteUrl);
        setToast({ tone: 'success', message: t('profile.inviteCopied') });
      }
      await refreshCollaboration();
    });
  }

  async function decideRequest(request: CollaboratorRequest, approve: boolean) {
    await runAction(setToast, t('profile.requestDecisionFailed'), async () => {
      await api(`/api/v1/collaborator-requests/${request.id}/${approve ? 'approve' : 'reject'}`, { method: 'POST' });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? t('profile.requestApproved') : t('profile.requestRejected') });
      await refreshCollaboration();
    });
  }

  return (
    <section className="collaboration-workspace">
      <div className="page-heading compact-heading">
        <span className="eyebrow subtle">{t('profile.tabs.collaboration')}</span>
        <h2>{t('profile.collaborationTitle')}</h2>
        <p>{t('profile.collaborationBody')}</p>
      </div>

      <section className="panel">
        <div className="section-title with-action">
          <div>
            <Users size={19} />
            <h2>{t('profile.collaboratingApps')}</h2>
          </div>
          <XButton type="button" variant="secondary" size="sm" label={t('common.refresh')} icon={<RefreshCw size={17} />} onClick={() => void refreshCollaboration()} />
        </div>
        <div className="review-list">
          {data.collaborating.length === 0 ? (
            <EmptyState icon={Users} title={t('profile.noCollaboratingApps')} />
          ) : (
            data.collaborating.map((item) => (
              <div className="review-row collaboration-row" key={item.id}>
                <div>
                  <strong>{item.name}</strong>
                  <span>{item.owner} · {item.latestVersion?.version || t('app.noPublishedVersion')}</span>
                </div>
                <div className="row-actions">
                  <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                  <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                  <XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('profile.leaveCollaboration')} tooltip={t('profile.leaveCollaboration')} icon={<LogOut size={17} />} onClick={() => void removeCollaborator(item.id, currentUser.id, true)} />
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      <section className="panel">
        <SectionTitle icon={UserPlus} title={t('profile.ownedCollaboration')} />
        <div className="collaboration-owned-list">
          {data.owned.length === 0 ? (
            <EmptyState icon={UserPlus} title={t('profile.noOwnedCollaboration')} />
          ) : (
            data.owned.map((item) => {
              const inviteDraft = inviteDrafts[item.app.id] || { email: '', sendEmail: false };
              return (
                <article className="nested-panel collaboration-app-panel" key={item.app.id}>
                  <div className="section-title with-action">
                    <div>
                      <AppIcon src={item.app.iconUrl} seed={item.app.packageId || item.app.slug || item.app.name} title={item.app.name} size={36} />
                      <div>
                        <h3>{item.app.name}</h3>
                        <span>{item.app.latestVersion?.version || t('app.noPublishedVersion')}</span>
                      </div>
                    </div>
                    <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item.app, 'manage')} />
                  </div>

                  <div className="collaboration-inline-form">
                    <XTextInput
                      label={t('profile.collaboratorIdentity')}
                      isLabelHidden
                      placeholder={t('profile.collaboratorIdentity')}
                      value={collaboratorDrafts[item.app.id] || ''}
                      onChange={(value) => setCollaboratorDrafts((current) => ({ ...current, [item.app.id]: value }))}
                    />
                    <XButton type="button" variant="secondary" size="sm" label={t('profile.addCollaborator')} icon={<UserPlus size={17} />} onClick={() => void addCollaborator(item)} />
                  </div>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorMembers')}</h4>
                    <div className="review-list compact-review-list">
                      {item.collaborators.length === 0 ? (
                        <span className="muted-text">{t('profile.noCollaborators')}</span>
                      ) : item.collaborators.map((collaborator) => (
                        <div className="review-row compact-row" key={collaborator.id}>
                          <div>
                            <strong>{collaborator.username || t('drawer.userLabel', { id: collaborator.userId })}</strong>
                            <span>{collaborator.email || formatDate(collaborator.createdAt)}</span>
                          </div>
                          <XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('profile.removeCollaborator')} tooltip={t('profile.removeCollaborator')} icon={<Trash2 size={17} />} onClick={() => void removeCollaborator(item.app.id, collaborator.userId)} />
                        </div>
                      ))}
                    </div>
                  </section>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorInvites')}</h4>
                    <div className="collaboration-inline-form invite-form">
                      <XTextInput
                        type="email"
                        label={t('profile.inviteEmail')}
                        description={t('profile.inviteEmailHelp')}
                        placeholder={t('profile.inviteEmail')}
                        value={inviteDraft.email}
                        onChange={(value) => setInviteDrafts((current) => ({ ...current, [item.app.id]: { ...inviteDraft, email: value } }))}
                      />
                      <XSwitch
                        label={t('profile.sendInviteEmail')}
                        value={inviteDraft.sendEmail}
                        width="100%"
                        onChange={(checked) => setInviteDrafts((current) => ({ ...current, [item.app.id]: { ...inviteDraft, sendEmail: checked } }))}
                      />
                      <XButton type="button" variant="secondary" size="sm" label={t('profile.createInvite')} icon={<Link size={17} />} onClick={() => void createInvite(item)} />
                    </div>
                    <div className="review-list compact-review-list">
                      {item.invites.length === 0 ? (
                        <span className="muted-text">{t('profile.noActiveInvites')}</span>
                      ) : item.invites.map((invite) => (
                        <div className="review-row compact-row" key={invite.id}>
                          <div>
                            <strong>{invite.email || invite.tokenPrefix}</strong>
                            <span>{t('profile.inviteExpires', { date: formatDate(invite.expiresAt) })}</span>
                          </div>
                          <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.copyInvite')} tooltip={t('profile.copyInvite')} icon={<Copy size={17} />} onClick={() => void copyInvite(invite)} />
                        </div>
                      ))}
                    </div>
                  </section>

                  <section className="collaboration-block">
                    <h4>{t('profile.collaboratorRequests')}</h4>
                    <div className="review-list compact-review-list">
                      {item.requests.length === 0 ? (
                        <span className="muted-text">{t('drawer.noCollaboratorRequests')}</span>
                      ) : item.requests.map((request) => (
                        <div className="review-row compact-row" key={request.id}>
                          <div>
                            <strong>{request.username || t('drawer.userLabel', { id: request.userId || request.user_id || '-' })}</strong>
                            <span>{request.message || request.email || t('drawer.noMessage')}</span>
                          </div>
                          <div className="row-actions">
                            <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('drawer.approveCollaborator')} tooltip={t('drawer.approveCollaborator')} icon={<Check size={17} />} onClick={() => void decideRequest(request, true)} />
                            <XIconButton className="fixed-row-icon-button" type="button" variant="destructive" size="sm" label={t('drawer.rejectCollaborator')} tooltip={t('drawer.rejectCollaborator')} icon={<X size={17} />} onClick={() => void decideRequest(request, false)} />
                          </div>
                        </div>
                      ))}
                    </div>
                  </section>
                </article>
              );
            })
          )}
        </div>
      </section>

      <section className="panel">
        <SectionTitle icon={History} title={t('profile.outgoingRequests')} />
        <div className="review-list">
          {data.outgoingRequests.length === 0 ? (
            <EmptyState icon={History} title={t('profile.noOutgoingRequests')} />
          ) : (
            data.outgoingRequests.map((request) => (
              <div className="review-row" key={request.id}>
                <div>
                  <strong>{request.appName || t('common.app')}</strong>
                  <span>{request.message || t('drawer.noMessage')}</span>
                </div>
                <span className={cx('status-badge', statusKey(request.status))}>{t(`statusLabels.${statusKey(request.status)}`)}</span>
              </div>
            ))
          )}
        </div>
      </section>
    </section>
  );
}

const tokenCreateAppCurlExample = [
  'export APPSTORE_URL="https://store.example.com"',
  'export APPSTORE_TOKEN="lcst_..."',
  '',
  'curl -fsS -X POST "$APPSTORE_URL/api/v1/apps" \\',
  '  -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '  -H "Content-Type: application/json" \\',
  "  -d '{",
  '    "packageId": "cloud.lazycat.example.app",',
  '    "name": "Example App",',
  '    "summary": "Published from CI",',
  '    "version": "1.2.3",',
  '    "sourceType": "GITHUB",',
  '    "downloadUrl": "https://github.com/acme/app/releases/download/v1.2.3/app.lpk",',
  '    "sha256": "REPLACE_WITH_64_CHAR_SHA256"',
  "  }'",
].join('\n');

const tokenPublishVersionCurlExample = [
  'export APPSTORE_URL="https://store.example.com"',
  'export APPSTORE_TOKEN="lcst_..."',
  'export APP_ID="123"',
  '',
  'curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \\',
  '  -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '  -F "version=1.2.4" \\',
  '  -F "changelog=Automated release" \\',
  '  -F "file=@dist/app.lpk"',
].join('\n');

const tokenGithubActionsExample = [
  'name: Publish LazyCat LPK',
  '',
  'on:',
  '  release:',
  '    types: [published]',
  '',
  'jobs:',
  '  publish:',
  '    runs-on: ubuntu-latest',
  '    steps:',
  '      - uses: actions/checkout@v4',
  '      - name: Build LPK',
  '        run: lzc-cli project release -o dist/app.lpk',
  '      - name: Publish version',
  '        env:',
  '          APPSTORE_URL: ${{ secrets.APPSTORE_URL }}',
  '          APPSTORE_TOKEN: ${{ secrets.APPSTORE_TOKEN }}',
  '          APP_ID: ${{ secrets.APP_ID }}',
  '        run: |',
  '          curl -fsS -X POST "$APPSTORE_URL/api/v1/apps/$APP_ID/versions" \\',
  '            -H "Authorization: Bearer $APPSTORE_TOKEN" \\',
  '            -F "version=${GITHUB_REF_NAME#v}" \\',
  '            -F "changelog=${{ github.event.release.body }}" \\',
  '            -F "file=@dist/app.lpk"',
].join('\n');

function TokenHelpDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const titleId = 'token-help-title';

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  return (
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <section
        className="modal-panel token-help-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onClick={(event) => event.stopPropagation()}
      >
        <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={onClose} />
        <div className="token-help-head">
          <span className="install-password-icon">
            <KeyRound size={21} />
          </span>
          <div>
            <h2 id={titleId}>{t('token.helpTitle')}</h2>
            <p>{t('token.helpBody')}</p>
          </div>
        </div>
        <div className="token-help-content">
          <TokenHelpExample title={t('token.helpCreateAppTitle')} body={t('token.helpCreateAppBody')} code={tokenCreateAppCurlExample} />
          <TokenHelpExample title={t('token.helpPublishVersionTitle')} body={t('token.helpPublishVersionBody')} code={tokenPublishVersionCurlExample} />
          <TokenHelpExample title={t('token.helpGithubActionsTitle')} body={t('token.helpGithubActionsBody')} code={tokenGithubActionsExample} />
        </div>
      </section>
    </div>
  );
}

function TokenHelpExample({ title, body, code }: { title: string; body: string; code: string }) {
  return (
    <section className="token-help-section">
      <div>
        <strong>{title}</strong>
        <span>{body}</span>
      </div>
      <pre className="code-example"><code>{code}</code></pre>
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
        <XIconButton type="submit" variant="secondary" label={t('groups.create')} icon={<Plus size={17} />} />
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
              <XIconButton type="button" variant="secondary" label={t('groups.addMember')} icon={<Plus size={17} />} onClick={() => void addMember(group.id)} />
              <XIconButton type="button" variant="destructive" label={t('groups.removeMember')} icon={<Trash2 size={17} />} onClick={() => void removeMember(group.id)} />
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
  onStorageOptionsChanged,
  setToast,
}: {
  user: User;
  reviews: Review[];
  onApprove: (review: Review, approve: boolean) => void;
  onSiteProfileSaved: (site?: SiteProfile) => Promise<void>;
  onStorageOptionsChanged: () => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [adminTab, setAdminTab] = useState<'reviews' | 'site' | 'users' | 'taxonomy' | 'collections'>('reviews');
  const [users, setUsers] = useState<User[]>([]);
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [reviewApps, setReviewApps] = useState<StoreApp[]>([]);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [registrationInvites, setRegistrationInvites] = useState<RegistrationInvite[]>([]);
  const [inviteDraft, setInviteDraft] = useState({ note: '', maxUses: '1' });
  const [isInviteCreateOpen, setIsInviteCreateOpen] = useState(false);
  const [storageRecords, setStorageRecords] = useState<StorageSettings[]>([defaultStorageSettings]);
  const [defaultStorageKey, setDefaultStorageKey] = useState(defaultStorageSettings.key);
  const [selectedStorageKey, setSelectedStorageKey] = useState(defaultStorageSettings.key);
  const [storageDraft, setStorageDraft] = useState<StorageSettings>(defaultStorageSettings);
  const [storageCreateDraft, setStorageCreateDraft] = useState<StorageSettings>({ ...defaultStorageSettings, key: '', name: '' });
  const [isStorageCreateOpen, setIsStorageCreateOpen] = useState(false);
  const [siteIconFile, setSiteIconFile] = useState<File | null>(null);
  const [siteIconStorageKey, setSiteIconStorageKey] = useState(defaultStorageSettings.key);
  const [isUploadingSiteIcon, setIsUploadingSiteIcon] = useState(false);
  const [adminCategories, setAdminCategories] = useState<Category[]>([]);
  const [adminTags, setAdminTags] = useState<TagRecord[]>([]);
  const [adminCollections, setAdminCollections] = useState<Collection[]>([]);
  const [categoryForm, setCategoryForm] = useState<TaxonomyDraft>({ name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '' });
  const [tagForm, setTagForm] = useState<TaxonomyDraft>({ name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '' });
  const [collectionForm, setCollectionForm] = useState<{ name: string; kind: string; appIds: number[] }>({ name: '', kind: 'MANUAL', appIds: [] });
  const [siteSettingsTab, setSiteSettingsTab] = useState<'identity' | 'announcement' | 'registration' | 'policy' | 'storage' | 'mail'>('identity');
  const [userDialogMode, setUserDialogMode] = useState<'create' | 'edit' | null>(null);
  const [userDraft, setUserDraft] = useState<ManagedUserDraft>(emptyUserDraft);
  const [isCollectionCreateOpen, setIsCollectionCreateOpen] = useState(false);
  const [taxonomyCreateMode, setTaxonomyCreateMode] = useState<'category' | 'tag' | null>(null);
  const [editingCategoryID, setEditingCategoryID] = useState<number | null>(null);
  const [editingTagID, setEditingTagID] = useState<number | null>(null);
  const [categoryDrafts, setCategoryDrafts] = useState<Record<number, TaxonomyDraft>>({});
  const [tagDrafts, setTagDrafts] = useState<Record<number, TaxonomyDraft>>({});
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
  const siteSettingsTabs = [
    { key: 'identity', label: t('admin.siteSettingTabs.identity'), icon: Archive },
    { key: 'announcement', label: t('admin.siteSettingTabs.announcement'), icon: MessageSquare },
    { key: 'registration', label: t('admin.siteSettingTabs.registration'), icon: KeyRound },
    { key: 'policy', label: t('admin.siteSettingTabs.policy'), icon: ShieldCheck },
    { key: 'storage', label: t('admin.siteSettingTabs.storage'), icon: Server },
    { key: 'mail', label: t('admin.siteSettingTabs.mail'), icon: MessageSquare },
  ] as const;
  const siteIdentityFields = [
    { key: 'site_title', label: t('admin.settings.siteTitle'), help: t('admin.settingsHelp.siteTitle') },
    { key: 'site_subtitle', label: t('admin.settings.siteSubtitle'), help: t('admin.settingsHelp.siteSubtitle'), type: 'textarea' },
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
  const registrationSettingFields = [
    {
      key: 'registration_mode',
      label: t('admin.settings.registrationMode'),
      help: t('admin.settingsHelp.registrationMode'),
      type: 'select',
      options: [
        { value: 'open', label: t('admin.registrationModes.open') },
        { value: 'invite', label: t('admin.registrationModes.invite') },
        { value: 'closed', label: t('admin.registrationModes.closed') },
      ],
    },
  ];
  const policySettingFields = [
    { key: 'max_lpk_size', label: t('admin.settings.maxLPKSize'), help: t('admin.settingsHelp.maxLPKSize'), inputMode: 'numeric' },
    { key: 'max_versions', label: t('admin.settings.maxVersions'), help: t('admin.settingsHelp.maxVersions'), inputMode: 'numeric' },
    { key: 'comments_enabled', label: t('admin.settings.commentsEnabled'), help: t('admin.settingsHelp.commentsEnabled'), type: 'boolean' },
    { key: 'allow_manual_outdated_clear', label: t('admin.settings.allowManualOutdatedClear'), help: t('admin.settingsHelp.allowManualOutdatedClear'), type: 'boolean' },
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
  const adminStorageOptions = useMemo(() => storageOptionsFromRecords(storageRecords, defaultStorageKey), [storageRecords, defaultStorageKey]);
  const adminStorageChoices = useMemo(() => storageSelectOptions(adminStorageOptions), [adminStorageOptions]);
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

  useEffect(() => {
    const selected = storageRecords.find((storage) => storage.key === selectedStorageKey) || storageRecords[0] || defaultStorageSettings;
    setStorageDraft({ ...defaultStorageSettings, ...selected });
  }, [selectedStorageKey, storageRecords]);

  useEffect(() => {
    setSiteIconStorageKey((current) => (adminStorageOptions.some((storage) => storage.key === current) ? current : defaultUploadStorageKey(adminStorageOptions)));
  }, [adminStorageOptions]);

  function setLoadedStorageRecords(storages: StorageSettings[], defaultKey: string) {
    const normalized = (storages.length > 0 ? storages : [defaultStorageSettings]).map(normalizeStorageRecord);
    const nextDefaultKey = defaultKey || normalized.find((storage) => storage.isDefault)?.key || normalized[0]?.key || defaultStorageSettings.key;
    setStorageRecords(normalized.map((storage) => ({ ...storage, isDefault: storage.key === nextDefaultKey })));
    setDefaultStorageKey(nextDefaultKey);
    setSelectedStorageKey((current) => (normalized.some((storage) => storage.key === current) ? current : nextDefaultKey));
  }

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
        const [userData, settingData, storageData, inviteData] = await Promise.all([
          api<{ users: User[] }>('/api/v1/admin/users'),
          api<{ settings: Record<string, string> }>('/api/v1/admin/settings'),
          api<{ storages: StorageSettings[]; defaultKey: string }>('/api/v1/admin/storage'),
          api<{ invites: RegistrationInvite[] }>('/api/v1/admin/registration-invites'),
        ]);
        setUsers(userData.users);
        setSettings(settingData.settings || {});
        setLoadedStorageRecords(storageData.storages || [], storageData.defaultKey);
        setRegistrationInvites(inviteData.invites || []);
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

  async function saveStorageSettings() {
    await runAction(setToast, t('admin.storageSaveFailed'), async () => {
      const data = await api<{ storage: StorageSettings }>(`/api/v1/admin/storage/${encodeURIComponent(storageDraft.key)}`, {
        method: 'PATCH',
        body: JSON.stringify(storageDraft),
      });
      setStorageRecords((current) => current.map((item) => (item.key === data.storage.key ? normalizeStorageRecord(data.storage) : item)));
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await onStorageOptionsChanged();
    });
  }

  async function testStorageSettings() {
    await runAction(setToast, t('admin.storageTestFailed'), async () => {
      await api('/api/v1/admin/storage/test', {
        method: 'POST',
        body: JSON.stringify(storageDraft),
      });
      setToast({ tone: 'success', message: t('admin.storageTested') });
    });
  }

  async function createStorage() {
    await runAction(setToast, t('admin.storageSaveFailed'), async () => {
      const data = await api<{ storage: StorageSettings }>('/api/v1/admin/storage', {
        method: 'POST',
        body: JSON.stringify(storageCreateDraft),
      });
      const next = normalizeStorageRecord(data.storage);
      setStorageRecords((current) => [next, ...current.filter((item) => item.key !== next.key)]);
      setSelectedStorageKey(next.key);
      setStorageCreateDraft({ ...defaultStorageSettings, key: '', name: '' });
      setIsStorageCreateOpen(false);
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await onStorageOptionsChanged();
    });
  }

  async function testSavedStorage(storage: StorageSettings) {
    await runAction(setToast, t('admin.storageTestFailed'), async () => {
      await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/test`, { method: 'POST' });
      setToast({ tone: 'success', message: t('admin.storageTested') });
    });
  }

  async function setDefaultStorage(storage: StorageSettings) {
    await runAction(setToast, t('admin.storageSaveFailed'), async () => {
      await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/default`, { method: 'POST' });
      setDefaultStorageKey(storage.key);
      setStorageRecords((current) => current.map((item) => ({ ...item, isDefault: item.key === storage.key })));
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await onStorageOptionsChanged();
    });
  }

  async function deleteStorage(storage: StorageSettings) {
    const confirmKey = `storage:${storage.key}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteStorage', { name: storage.name || storage.key }) });
      return;
    }
    await runAction(setToast, t('admin.storageDeleteFailed'), async () => {
      await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}`, { method: 'DELETE' });
      setStorageRecords((current) => current.filter((item) => item.key !== storage.key));
      setConfirmDelete(null);
      setToast({ tone: 'neutral', message: t('admin.storageDeleted') });
      await onStorageOptionsChanged();
    });
  }

  async function uploadSiteIcon() {
    if (!siteIconFile) return;
    setIsUploadingSiteIcon(true);
    try {
      await runAction(setToast, t('admin.siteIconUploadFailed'), async () => {
        const form = new FormData();
        form.set('file', siteIconFile);
        form.set('storageKey', siteIconStorageKey);
        const data = await api<{ url: string; site?: SiteProfile }>('/api/v1/admin/settings/site-icon', { method: 'POST', body: form });
        updateSetting('site_icon_url', data.url);
        setSiteIconFile(null);
        setToast({ tone: 'success', message: t('admin.siteIconUploaded') });
        await onSiteProfileSaved(data.site);
      });
    } finally {
      setIsUploadingSiteIcon(false);
    }
  }

  async function createRegistrationInvite() {
    const maxUses = Number.parseInt(inviteDraft.maxUses, 10);
    await runAction(setToast, t('admin.inviteCreateFailed'), async () => {
      const data = await api<{ invite: RegistrationInvite; code: string }>('/api/v1/admin/registration-invites', {
        method: 'POST',
        body: JSON.stringify({
          note: inviteDraft.note,
          maxUses: Number.isFinite(maxUses) ? maxUses : 1,
        }),
      });
      setRegistrationInvites((current) => [data.invite, ...current]);
      setInviteDraft({ note: '', maxUses: '1' });
      setIsInviteCreateOpen(false);
      setToast({ tone: 'success', message: t('admin.inviteCreated') });
    });
  }

  async function deleteRegistrationInvite(invite: RegistrationInvite) {
    const confirmKey = `invite:${invite.id}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteInvite', { code: invite.note || invite.codePrefix }) });
      return;
    }
    await runAction(setToast, t('admin.inviteDeleteFailed'), async () => {
      await api(`/api/v1/admin/registration-invites/${invite.id}`, { method: 'DELETE' });
      setRegistrationInvites((current) => current.filter((item) => item.id !== invite.id));
      setConfirmDelete(null);
      setToast({ tone: 'neutral', message: t('admin.inviteDeleted') });
    });
  }

  async function copyInviteCode(invite: RegistrationInvite) {
    try {
      if (!invite.code || !navigator.clipboard?.writeText) throw new Error(t('home.copySourceUnsupported'));
      await navigator.clipboard.writeText(invite.code);
      setToast({ tone: 'success', message: t('admin.inviteCopied') });
    } catch (error) {
      setToast({ tone: 'error', message: errorMessage(error, t('admin.inviteCopyFailed')) });
    }
  }

  function openCreateUserDialog() {
    setUserDraft(emptyUserDraft());
    setUserDialogMode('create');
  }

  function openEditUserDialog(item: User) {
    setUserDraft(draftFromUser(item));
    setUserDialogMode('edit');
  }

  async function saveManagedUser(event: FormEvent) {
    event.preventDefault();
    const body = {
      username: userDraft.username,
      nickname: userDraft.nickname,
      email: userDraft.email,
      ...(userDraft.password.trim() ? { password: userDraft.password } : userDialogMode === 'create' ? { password: userDraft.password } : {}),
      role: userDraft.role,
      emailVerified: userDraft.emailVerified,
      disabled: userDraft.disabled,
    };
    await runAction(setToast, userDialogMode === 'create' ? t('admin.userCreateFailed') : t('admin.userUpdateFailed'), async () => {
      if (userDialogMode === 'create') {
        await api('/api/v1/admin/users', { method: 'POST', body: JSON.stringify(body) });
        setToast({ tone: 'success', message: t('admin.userCreated') });
      } else if (userDraft.id) {
        await api(`/api/v1/admin/users/${userDraft.id}`, { method: 'PATCH', body: JSON.stringify(body) });
        setToast({ tone: 'success', message: t('admin.userUpdated') });
      }
      setUserDialogMode(null);
      await reload();
    });
  }

  async function toggleUserDisabled(item: User) {
    await runAction(setToast, t('admin.userUpdateFailed'), async () => {
      await api(`/api/v1/admin/users/${item.id}`, { method: 'PATCH', body: JSON.stringify({ disabled: !item.disabled }) });
      setToast({ tone: 'success', message: item.disabled ? t('admin.userEnabled') : t('admin.userDisabled') });
      await reload();
    });
  }

  async function deleteManagedUser(item: User) {
    const confirmKey = `user:${item.id}`;
    if (confirmDelete !== confirmKey) {
      setConfirmDelete(confirmKey);
      setToast({ tone: 'neutral', message: t('admin.confirmDeleteUser', { name: displayUserName(item) }) });
      return;
    }
    await runAction(setToast, t('admin.userDeleteFailed'), async () => {
      await api(`/api/v1/admin/users/${item.id}`, { method: 'DELETE' });
      setConfirmDelete(null);
      setToast({ tone: 'neutral', message: t('admin.userDeleted') });
      await reload();
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
            <XButton
              type="button"
              variant="secondary"
              size="sm"
              label={t('admin.useRecommendedMirrors')}
              icon={<Download size={16} />}
              onClick={() => updateSetting(field.key, preset)}
            />
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

  function emptyTaxonomyDraft(): TaxonomyDraft {
    return { name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '' };
  }

  function taxonomyDraft(item: Category | TagRecord): TaxonomyDraft {
    return {
      name: item.name,
      nameI18n: {
        'zh-CN': item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || '',
        en: item.nameI18n?.en || '',
      },
      slug: item.slug,
    };
  }

  function updateTaxonomyI18n(draft: TaxonomyDraft, language: string, value: string): TaxonomyDraft {
    return { ...draft, nameI18n: { ...draft.nameI18n, [language]: value } };
  }

  async function createCategory(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.categoryCreateFailed'), async () => {
      await api('/api/v1/admin/categories', { method: 'POST', body: JSON.stringify(categoryForm) });
      setCategoryForm(emptyTaxonomyDraft());
      setTaxonomyCreateMode(null);
      setToast({ tone: 'success', message: t('admin.categoryCreated') });
      await reload();
    });
  }

  async function updateCategory(item: Category) {
    const draft = categoryDrafts[item.id] || taxonomyDraft(item);
    await runAction(setToast, t('admin.categoryUpdateFailed'), async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: t('admin.categoryUpdated') });
      setEditingCategoryID(null);
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
      setTagForm(emptyTaxonomyDraft());
      setTaxonomyCreateMode(null);
      setToast({ tone: 'success', message: t('admin.tagCreated') });
      await reload();
    });
  }

  async function updateTag(item: TagRecord) {
    const draft = tagDrafts[item.id] || taxonomyDraft(item);
    await runAction(setToast, t('admin.tagUpdateFailed'), async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: t('admin.tagUpdated') });
      setEditingTagID(null);
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
      setIsCollectionCreateOpen(false);
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
      <div className="horizontal-control-scroll">
        <XTabList value={adminTab} onChange={(value) => setAdminTab(value as typeof adminTab)} hasDivider size="md">
          {adminTabs.map((item) => {
            const Icon = item.icon;
            return <XTab key={item.key} value={item.key} label={item.label} icon={<Icon size={17} />} />;
          })}
        </XTabList>
      </div>
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
          <section className="panel form-panel site-settings-panel">
            <SectionTitle icon={Settings} title={t('admin.siteSettings')} />
            <div className="horizontal-control-scroll">
              <XTabList value={siteSettingsTab} onChange={(value) => setSiteSettingsTab(value as typeof siteSettingsTab)} hasDivider size="sm">
                {siteSettingsTabs.map((item) => {
                  const Icon = item.icon;
                  return <XTab key={item.key} value={item.key} label={item.label} icon={<Icon size={16} />} />;
                })}
              </XTabList>
            </div>

            {siteSettingsTab !== 'storage' ? (
              <form className="settings-tab-panel" onSubmit={saveSettings}>
                {siteSettingsTab === 'identity' && (
                  <div className="settings-section">
                    <div className="settings-section-head">
                      <strong>{t('admin.siteIdentity')}</strong>
                      <span>{t('admin.siteIdentityBody')}</span>
                    </div>
                    <div className="site-icon-upload">
                      <div className="brand-mark preview">
                        {settings.site_icon_url ? <img src={settings.site_icon_url} alt="" /> : <Archive size={22} />}
                      </div>
                      <FilePicker
                        label={t('admin.uploadSiteIcon')}
                        help={t('admin.uploadSiteIconHelp')}
                        value={siteIconFile}
                        accept=".png,.jpg,.jpeg,.webp"
                        onChange={(nextFile) => setSiteIconFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
                      />
                      {adminStorageChoices.length > 0 && (
                        <XSelector
                          label={t('common.storage')}
                          value={siteIconStorageKey}
                          options={adminStorageChoices}
                          onChange={setSiteIconStorageKey}
                        />
                      )}
                      <XButton type="button" variant="secondary" size="sm" label={t('admin.uploadSiteIcon')} icon={<Upload size={17} />} isDisabled={!siteIconFile || isUploadingSiteIcon} isLoading={isUploadingSiteIcon} onClick={() => void uploadSiteIcon()} />
                    </div>
                    <XFormLayout>
                      {siteIdentityFields.map(renderSettingField)}
                    </XFormLayout>
                  </div>
                )}

                {siteSettingsTab === 'announcement' && (
                  <div className="settings-section">
                    <div className="settings-section-head">
                      <strong>{t('admin.announcementCenter')}</strong>
                      <span>{t('admin.announcementCenterBody')}</span>
                    </div>
                    <XFormLayout>
                      {announcementFields.map(renderSettingField)}
                    </XFormLayout>
                  </div>
                )}

                {siteSettingsTab === 'registration' && (
                  <div className="settings-section">
                    <div className="settings-section-head">
                      <strong>{t('admin.registrationSettings')}</strong>
                      <span>{t('admin.registrationSettingsBody')}</span>
                    </div>
                    <XFormLayout>
                      {registrationSettingFields.map(renderSettingField)}
                    </XFormLayout>
                    <div className="invite-manager">
                      <div className="settings-section-head">
                        <div>
                          <strong>{t('admin.inviteManagement')}</strong>
                          <span>{t('admin.inviteManagementBody')}</span>
                        </div>
                        <XButton type="button" variant="secondary" size="sm" label={t('admin.createInvite')} icon={<KeyRound size={17} />} onClick={() => setIsInviteCreateOpen(true)} />
                      </div>
                      <div className="review-list invite-list">
                        {registrationInvites.length === 0 ? (
                          <EmptyState icon={KeyRound} title={t('admin.noInvites')} body={t('admin.noInvitesBody')} />
                        ) : (
                          registrationInvites.map((invite) => (
                            <div className="review-row invite-row" key={invite.id}>
                              <div>
                                <strong>{invite.note || t('admin.inviteUntitled')}</strong>
                                <span>
                                  {t('admin.inviteUsage', { remaining: invite.remainingUses, max: invite.maxUses })} · {formatDate(invite.createdAt)}
                                </span>
                                <code className="invite-code-inline">{invite.code}</code>
                              </div>
                              <div className="row-actions">
                                <XBadge label={invite.remainingUses > 0 ? t('admin.inviteUsable') : t('admin.inviteExhausted')} variant={invite.remainingUses > 0 ? 'success' : 'neutral'} />
                                <XIconButton
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  label={t('admin.copyInviteCode')}
                                  icon={<Copy size={16} />}
                                  onClick={() => void copyInviteCode(invite)}
                                />
                                <XIconButton
                                  type="button"
                                  variant="destructive"
                                  size="sm"
                                  label={t('admin.deleteInvite')}
                                  icon={<Trash2 size={16} />}
                                  onClick={() => void deleteRegistrationInvite(invite)}
                                />
                              </div>
                            </div>
                          ))
                        )}
                      </div>
                    </div>
                    {isInviteCreateOpen && (
                      <div className="modal-backdrop" role="presentation" onClick={() => setIsInviteCreateOpen(false)}>
                        <div
                          className="modal-panel form-panel invite-dialog"
                          role="dialog"
                          aria-modal="true"
                          aria-label={t('admin.createInvite')}
                          onClick={(event) => event.stopPropagation()}
                        >
                          <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setIsInviteCreateOpen(false)} />
                          <SectionTitle icon={KeyRound} title={t('admin.createInvite')} />
                          <XFormLayout>
                            <XTextInput
                              label={t('admin.inviteNote')}
                              description={t('admin.inviteNoteHelp')}
                              value={inviteDraft.note}
                              onChange={(value) => setInviteDraft((current) => ({ ...current, note: value }))}
                            />
                            <XTextInput
                              label={t('admin.inviteMaxUses')}
                              description={t('admin.inviteMaxUsesHelp')}
                              value={inviteDraft.maxUses}
                              onChange={(value) => setInviteDraft((current) => ({ ...current, maxUses: value }))}
                            />
                          </XFormLayout>
                          <div className="dialog-actions">
                            <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setIsInviteCreateOpen(false)} />
                            <XButton type="button" variant="primary" label={t('admin.createInvite')} icon={<KeyRound size={17} />} onClick={() => void createRegistrationInvite()} />
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {siteSettingsTab === 'policy' && (
                  <div className="settings-section">
                    <div className="settings-section-head">
                      <strong>{t('admin.policySettings')}</strong>
                      <span>{t('admin.policySettingsBody')}</span>
                    </div>
                    <XFormLayout>
                      {policySettingFields.map(renderSettingField)}
                    </XFormLayout>
                  </div>
                )}

                {siteSettingsTab === 'mail' && (
                  <div className="settings-section">
                    <div className="settings-section-head">
                      <strong>{t('admin.smtpSettings')}</strong>
                      <span>{t('admin.smtpSettingsBody')}</span>
                    </div>
                    <XFormLayout>
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
                      <XButton type="button" variant="secondary" size="sm" label={t('admin.sendTestEmail')} icon={<MessageSquare size={17} />} onClick={() => void sendTestEmail()} />
                    </div>
                  </div>
                )}

                <div className="settings-form-actions">
                  <XButton type="submit" variant="primary" label={t('admin.saveSettings')} icon={<Settings size={18} />} />
                </div>
              </form>
            ) : (
              <div className="settings-tab-panel">
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
              </div>
            )}
          </section>
          <section className="panel site-preview-panel">
            <SectionTitle icon={Archive} title={t('admin.sitePreview')} />
            <div className="site-preview-brand">
              <div className="brand-mark">
                {settings.site_icon_url ? <img src={settings.site_icon_url} alt="" /> : <Archive size={22} />}
              </div>
              <div>
                <strong>{settings.site_title || t('appName')}</strong>
                {settings.site_subtitle && <span className="site-preview-subtitle">{settings.site_subtitle}</span>}
                <span>{adminPublicURL}</span>
              </div>
            </div>
            <div className="source-url-preview">
              <span>{t('admin.subscriptionURL')}</span>
              <code>{adminSourceURL}</code>
              <XButton type="button" variant="secondary" size="sm" label={t('home.copySourceFeed')} icon={<Copy size={16} />} onClick={() => void copyAdminSourceURL()} />
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
            <div className="section-title with-action">
              <div>
                <Users size={19} />
                <h2>{t('admin.userManagement')}</h2>
              </div>
              <XButton type="button" variant="primary" size="sm" label={t('admin.createUser')} icon={<UserPlus size={17} />} onClick={openCreateUserDialog} />
            </div>
            <div className="review-list user-management-list">
              {users.length === 0 ? (
                <EmptyState icon={Users} title={t('admin.noUsers')} />
              ) : users.map((item) => (
                <div className="review-row user-row" key={item.id}>
                  <UserAvatar user={item} size={42} />
                  <div>
                    <strong>{displayUserName(item)}</strong>
                    <span>{item.username} · {item.email || t('admin.noEmail')}</span>
                  </div>
                  <div className="row-actions">
                    <XBadge label={t(`admin.roles.${item.role === 'SITE_ADMIN' ? 'siteAdmin' : item.role === 'SOFTWARE_ADMIN' ? 'softwareAdmin' : 'user'}`)} variant={item.role === 'SITE_ADMIN' ? 'info' : item.role === 'SOFTWARE_ADMIN' ? 'success' : 'neutral'} />
                    {item.disabled && <XBadge label={t('admin.userDisabledBadge')} variant="error" />}
                    <XIconButton type="button" variant="ghost" size="sm" label={t('admin.editUserNamed', { name: displayUserName(item) })} icon={<Pencil size={16} />} onClick={() => openEditUserDialog(item)} />
                    <XIconButton type="button" variant="ghost" size="sm" label={item.disabled ? t('admin.enableUserNamed', { name: displayUserName(item) }) : t('admin.disableUserNamed', { name: displayUserName(item) })} icon={<UserRound size={16} />} onClick={() => void toggleUserDisabled(item)} />
                    <XIconButton type="button" variant="destructive" size="sm" label={t('admin.deleteUserNamed', { name: displayUserName(item) })} icon={<Trash2 size={16} />} onClick={() => void deleteManagedUser(item)} />
                  </div>
                </div>
              ))}
            </div>
          </section>
          {userDialogMode && (
            <div className="modal-backdrop" role="presentation" onClick={() => setUserDialogMode(null)}>
              <form
                className="modal-panel form-panel user-dialog"
                role="dialog"
                aria-modal="true"
                aria-label={userDialogMode === 'create' ? t('admin.createUser') : t('admin.editUser')}
                onClick={(event) => event.stopPropagation()}
                onSubmit={saveManagedUser}
              >
                <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setUserDialogMode(null)} />
                <SectionTitle icon={userDialogMode === 'create' ? UserPlus : UserRound} title={userDialogMode === 'create' ? t('admin.createUser') : t('admin.editUser')} />
                <XFormLayout>
                  <XTextInput label={t('common.username')} value={userDraft.username} onChange={(value) => setUserDraft({ ...userDraft, username: value })} />
                  <XTextInput label={t('profile.nickname')} value={userDraft.nickname} onChange={(value) => setUserDraft({ ...userDraft, nickname: value })} />
                  <XTextInput type="email" label={t('common.email')} value={userDraft.email} onChange={(value) => setUserDraft({ ...userDraft, email: value })} />
                  <XTextInput type="password" label={userDialogMode === 'create' ? t('common.password') : t('admin.newPasswordOptional')} value={userDraft.password} onChange={(value) => setUserDraft({ ...userDraft, password: value })} />
                  <XSelector label={t('common.role')} value={userDraft.role} options={userRoleOptions} onChange={(value) => setUserDraft({ ...userDraft, role: value as User['role'] })} />
                  <XSelector
                    label={t('admin.emailVerified')}
                    value={String(userDraft.emailVerified)}
                    options={[
                      { value: 'true', label: t('common.on') },
                      { value: 'false', label: t('common.off') },
                    ]}
                    onChange={(value) => setUserDraft({ ...userDraft, emailVerified: value === 'true' })}
                  />
                  <XSelector
                    label={t('admin.userDisabledField')}
                    value={String(userDraft.disabled)}
                    options={[
                      { value: 'false', label: t('common.off') },
                      { value: 'true', label: t('common.on') },
                    ]}
                    onChange={(value) => setUserDraft({ ...userDraft, disabled: value === 'true' })}
                  />
                </XFormLayout>
                <div className="dialog-actions">
                  <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setUserDialogMode(null)} />
                  <XButton type="submit" variant="primary" label={userDialogMode === 'create' ? t('admin.createUser') : t('common.save')} icon={<Save size={17} />} />
                </div>
              </form>
            </div>
          )}
        </section>
      )}
      {adminTab === 'taxonomy' && (
      <section className="workspace-pane taxonomy-workspace">
        {taxonomyCreateMode && (
          <div className="modal-backdrop" role="presentation" onClick={() => setTaxonomyCreateMode(null)}>
            <form
              className="modal-panel form-panel taxonomy-dialog"
              role="dialog"
              aria-modal="true"
              aria-label={taxonomyCreateMode === 'category' ? t('admin.createCategory') : t('admin.createTag')}
              onClick={(event) => event.stopPropagation()}
              onSubmit={taxonomyCreateMode === 'category' ? createCategory : createTag}
            >
              <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setTaxonomyCreateMode(null)} />
              <SectionTitle icon={Tag} title={taxonomyCreateMode === 'category' ? t('admin.createCategory') : t('admin.createTag')} />
              {taxonomyCreateMode === 'category' ? (
                <>
                  <XTextInput label={t('admin.categoryNameZh')} value={categoryForm.nameI18n['zh-CN'] || ''} onChange={(value) => setCategoryForm(updateTaxonomyI18n(categoryForm, 'zh-CN', value))} />
                  <XTextInput label={t('admin.categoryNameEn')} value={categoryForm.nameI18n.en || ''} onChange={(value) => setCategoryForm(updateTaxonomyI18n(categoryForm, 'en', value))} />
                  <XTextInput label={t('admin.categoryName')} value={categoryForm.name} onChange={(value) => setCategoryForm({ ...categoryForm, name: value })} />
                  <XTextInput label={t('admin.categorySlug')} value={categoryForm.slug} onChange={(value) => setCategoryForm({ ...categoryForm, slug: value })} />
                </>
              ) : (
                <>
                  <XTextInput label={t('admin.tagNameZh')} value={tagForm.nameI18n['zh-CN'] || ''} onChange={(value) => setTagForm(updateTaxonomyI18n(tagForm, 'zh-CN', value))} />
                  <XTextInput label={t('admin.tagNameEn')} value={tagForm.nameI18n.en || ''} onChange={(value) => setTagForm(updateTaxonomyI18n(tagForm, 'en', value))} />
                  <XTextInput label={t('admin.tagName')} value={tagForm.name} onChange={(value) => setTagForm({ ...tagForm, name: value })} />
                  <XTextInput label={t('admin.tagSlug')} value={tagForm.slug} onChange={(value) => setTagForm({ ...tagForm, slug: value })} />
                </>
              )}
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setTaxonomyCreateMode(null)} />
                <XButton type="submit" variant="primary" label={taxonomyCreateMode === 'category' ? t('admin.createCategory') : t('admin.createTag')} icon={<Plus size={18} />} />
              </div>
            </form>
          </div>
        )}

        <section className="panel">
          <div className="section-title with-action">
            <div>
              <Tag size={19} />
              <h2>{t('admin.categoriesAndTags')}</h2>
            </div>
            <div className="row-actions">
              <XButton type="button" variant="secondary" size="sm" label={t('admin.createCategory')} icon={<Plus size={17} />} onClick={() => setTaxonomyCreateMode('category')} />
              <XButton type="button" variant="secondary" size="sm" label={t('admin.createTag')} icon={<Plus size={17} />} onClick={() => setTaxonomyCreateMode('tag')} />
            </div>
          </div>
          <p className="inline-note">{t('admin.taxonomyHelp')}</p>
        </section>

        <div className="taxonomy-list-grid">
          <section className="panel">
            <SectionTitle icon={Layers3} title={t('admin.categoryList')} />
            <div className="review-list">
              {adminCategories.length === 0 ? (
                <EmptyState icon={Layers3} title={t('admin.noCategories')} body={t('admin.noCategoriesBody')} />
              ) : adminCategories.map((item) => {
                const draft = categoryDrafts[item.id] || taxonomyDraft(item);
                const isEditing = editingCategoryID === item.id;
                return (
                  <div className={cx('taxonomy-row', isEditing && 'editing')} key={item.id}>
                    {isEditing ? (
                      <>
                        <div className="taxonomy-edit-fields">
                          <XTextInput label={t('admin.categoryNameZhFor', { name: localizedName(item) })} value={draft.nameI18n['zh-CN'] || ''} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: updateTaxonomyI18n(draft, 'zh-CN', value) }))} />
                          <XTextInput label={t('admin.categoryNameEnFor', { name: localizedName(item) })} value={draft.nameI18n.en || ''} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: updateTaxonomyI18n(draft, 'en', value) }))} />
                          <XTextInput label={t('admin.categoryNameFor', { name: localizedName(item) })} value={draft.name} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, name: value } }))} />
                          <XTextInput label={t('admin.categorySlugFor', { name: item.name })} value={draft.slug} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: value } }))} />
                        </div>
                        <div className="dialog-actions">
                          <XButton type="button" variant="secondary" size="sm" label={t('common.cancel')} icon={<X size={16} />} onClick={() => setEditingCategoryID(null)} />
                          <XButton type="button" variant="primary" size="sm" label={t('admin.saveCategory')} icon={<Save size={16} />} onClick={() => void updateCategory(item)} />
                        </div>
                      </>
                    ) : (
                      <>
                        <div className="taxonomy-summary">
                          <strong>{localizedName(item)}</strong>
                          <span>{item.slug}</span>
                          <small>{t('admin.taxonomyLanguages', { zh: item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || item.name || '-', en: item.nameI18n?.en || '-' })}</small>
                        </div>
                        <div className="row-actions">
                          <XIconButton type="button" variant="ghost" label={t('admin.editCategoryNamed', { name: item.name })} icon={<Pencil size={16} />} onClick={() => setEditingCategoryID(item.id)} />
                          <XIconButton type="button" variant="destructive" label={t('admin.deleteCategoryNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => void deleteCategory(item)} />
                        </div>
                      </>
                    )}
                  </div>
                );
              })}
            </div>
          </section>

          <section className="panel">
            <SectionTitle icon={Tag} title={t('admin.tagList')} />
            <div className="review-list">
              {adminTags.length === 0 ? (
                <EmptyState icon={Tag} title={t('admin.noTags')} body={t('admin.noTagsBody')} />
              ) : adminTags.map((item) => {
                const draft = tagDrafts[item.id] || taxonomyDraft(item);
                const isEditing = editingTagID === item.id;
                return (
                  <div className={cx('taxonomy-row', isEditing && 'editing')} key={item.id}>
                    {isEditing ? (
                      <>
                        <div className="taxonomy-edit-fields">
                          <XTextInput label={t('admin.tagNameZhFor', { name: localizedName(item) })} value={draft.nameI18n['zh-CN'] || ''} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: updateTaxonomyI18n(draft, 'zh-CN', value) }))} />
                          <XTextInput label={t('admin.tagNameEnFor', { name: localizedName(item) })} value={draft.nameI18n.en || ''} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: updateTaxonomyI18n(draft, 'en', value) }))} />
                          <XTextInput label={t('admin.tagNameFor', { name: localizedName(item) })} value={draft.name} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, name: value } }))} />
                          <XTextInput label={t('admin.tagSlugFor', { name: item.name })} value={draft.slug} onChange={(value) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: value } }))} />
                        </div>
                        <div className="dialog-actions">
                          <XButton type="button" variant="secondary" size="sm" label={t('common.cancel')} icon={<X size={16} />} onClick={() => setEditingTagID(null)} />
                          <XButton type="button" variant="primary" size="sm" label={t('admin.saveTag')} icon={<Save size={16} />} onClick={() => void updateTag(item)} />
                        </div>
                      </>
                    ) : (
                      <>
                        <div className="taxonomy-summary">
                          <strong>{localizedName(item)}</strong>
                          <span>{item.slug}</span>
                          <small>{t('admin.taxonomyLanguages', { zh: item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || item.name || '-', en: item.nameI18n?.en || '-' })}</small>
                        </div>
                        <div className="row-actions">
                          <XIconButton type="button" variant="ghost" label={t('admin.editTagNamed', { name: item.name })} icon={<Pencil size={16} />} onClick={() => setEditingTagID(item.id)} />
                          <XIconButton type="button" variant="destructive" label={t('admin.deleteTagNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => void deleteTag(item)} />
                        </div>
                      </>
                    )}
                  </div>
                );
              })}
            </div>
          </section>
        </div>
      </section>
      )}
      {adminTab === 'collections' && (
      <section className="workspace-pane">
        {isCollectionCreateOpen && (
          <div className="modal-backdrop" role="presentation" onClick={() => setIsCollectionCreateOpen(false)}>
            <form
              className="modal-panel form-panel collection-dialog"
              role="dialog"
              aria-modal="true"
              aria-label={t('admin.createCollection')}
              onClick={(event) => event.stopPropagation()}
              onSubmit={createCollection}
            >
              <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={() => setIsCollectionCreateOpen(false)} />
              <SectionTitle icon={Layers3} title={t('admin.createCollection')} />
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
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={() => setIsCollectionCreateOpen(false)} />
                <XButton type="submit" variant="primary" label={t('admin.createCollection')} icon={<Layers3 size={18} />} />
              </div>
            </form>
          </div>
        )}

        <section className="panel">
          <div className="section-title with-action">
            <div>
              <Layers3 size={19} />
              <h2>{t('admin.collectionList')}</h2>
            </div>
            <XButton type="button" variant="primary" size="sm" label={t('admin.createCollection')} icon={<Plus size={17} />} onClick={() => setIsCollectionCreateOpen(true)} />
          </div>
          <div className="review-list">
            {adminCollections.length === 0 ? (
              <EmptyState icon={Layers3} title={t('admin.noCollections')} body={t('admin.noCollectionsBody')} />
            ) : adminCollections.map((item) => {
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
                    <XIconButton type="button" variant="ghost" label={t('admin.saveCollectionNamed', { name: item.name })} icon={<Save size={16} />} onClick={() => void updateCollection(item)} />
                    <XIconButton type="button" variant="destructive" label={t('admin.deleteCollectionNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => void deleteCollection(item)} />
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      )}
    </section>
  );
}

function AppDrawer({
  app,
  mode,
  onModeChange,
  user,
  groups,
  categories,
  tagOptions,
  storageOptions,
  onClose,
  onInstall,
  onRefresh,
  onListRefresh,
  setToast,
}: {
  app: StoreApp;
  mode: AppDetailMode;
  onModeChange: (mode: AppDetailMode) => void;
  user: User | null;
  groups: Group[];
  categories: Category[];
  tagOptions: string[];
  storageOptions: StorageOption[];
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
  const [screenshotDeviceType, setScreenshotDeviceType] = useState<'DESKTOP' | 'MOBILE'>('DESKTOP');
  const [screenshotStorageKey, setScreenshotStorageKey] = useState(defaultUploadStorageKey(storageOptions));
  const [screenshotCaptionDrafts, setScreenshotCaptionDrafts] = useState<Record<number, string>>({});
  const preferredScreenshotDevice = usePreferredScreenshotDevice();
  const [versionForm, setVersionForm] = useState({ version: '', sourceType: 'GITHUB', downloadUrl: '', sha256: '', useMirrorDownload: true, changelog: '' });
  const [versionArtifactMode, setVersionArtifactMode] = useState<'local' | 'external'>('local');
  const [versionFile, setVersionFile] = useState<File | null>(null);
  const [isSubmittingVersion, setIsSubmittingVersion] = useState(false);
  const [versionProgress, setVersionProgress] = useState<SubmissionProgress | null>(null);
  const [versionStorageKey, setVersionStorageKey] = useState(defaultUploadStorageKey(storageOptions));
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
  const canMaintain = canUserManageApp(user, app);
  const canUploadVersion = canUserUploadVersion(user, app);
  const canOpenManagement = canMaintain || canUploadVersion;
  const isManageMode = mode === 'manage' && canOpenManagement;
  const canEditScreenshots = isManageMode && canMaintain;
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
  const installNextStep = canUploadVersion ? t('drawer.installBlockedManage') : t('drawer.installBlockedUser');
  const displayScreenshots = orderedScreenshots(app.screenshots, preferredScreenshotDevice);
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
  const hasOutdatedMarks = (app.outdatedMarks ?? 0) > 0;
  const commentsAllowed = app.commentsAllowed ?? app.commentsEnabled;
  const canComment = !!user && commentsAllowed;
  const versionNumberReady = Boolean(versionForm.version.trim());
  const versionExternalDownloadReady = Boolean(versionForm.downloadUrl.trim());
  const versionExternalChecksumReady = Boolean(versionForm.sha256.trim());
  const versionGithubMirrorKind = githubMirrorKindForURL(versionForm.downloadUrl);
  const versionMirrorDownloadHelp =
    versionGithubMirrorKind === 'raw'
      ? t('submitApp.mirrorDownloadRawHelp')
      : versionGithubMirrorKind === 'download'
        ? t('submitApp.mirrorDownloadReleaseHelp')
        : t('submitApp.mirrorDownloadHelp');
  const versionExternalArtifactReady = versionExternalDownloadReady;
  const versionArtifactReady = versionArtifactMode === 'local' ? Boolean(versionFile) : versionExternalArtifactReady;
  const canSubmitVersion = versionArtifactReady;
  const versionPublishesDirectly = user?.role === 'SITE_ADMIN' || user?.role === 'SOFTWARE_ADMIN' || app.allowUnreviewedUpdates;
  const storageChoices = storageSelectOptions(storageOptions);

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
    setIsSubmittingVersion(false);
    setVersionProgress(null);
    setScreenshotCaptionDrafts(Object.fromEntries((app.screenshots || []).map((shot) => [shot.id, shot.caption || ''])));
    if (versionFileInputRef.current) versionFileInputRef.current.value = '';
  }, [app]);

  useEffect(() => {
    const fallback = defaultUploadStorageKey(storageOptions);
    setVersionStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
    setScreenshotStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
  }, [storageOptions]);

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
    if (!canComment) {
      setToast({ tone: 'neutral', message: t('drawer.commentsDisabled') });
      return;
    }
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

  function startVersionStageProgress(label: string, initialPercent = 12, maxPercent = 86) {
    setVersionProgress({ percent: initialPercent, label });
    const timer = window.setInterval(() => {
      setVersionProgress((current) => {
        if (!current) return current;
        const remaining = maxPercent - current.percent;
        if (remaining <= 0) return current;
        return { ...current, percent: Math.min(maxPercent, current.percent + Math.max(1, remaining * 0.16)) };
      });
    }, 700);
    return () => window.clearInterval(timer);
  }

  async function submitExternalVersion(event: FormEvent) {
    event.preventDefault();
    if (isSubmittingVersion) return;
    if (versionArtifactMode === 'local' && !versionFile) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (versionArtifactMode === 'external' && !versionForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    setIsSubmittingVersion(true);
    setVersionProgress({ percent: 6, label: t('submitApp.progressPreparing') });
    let stopStageProgress: (() => void) | undefined;
    try {
      await runAction(setToast, t('drawer.versionSubmitFailed'), async () => {
        if (versionArtifactMode === 'local' && versionFile) {
          const form = new FormData();
          form.set('file', versionFile);
          form.set('version', versionForm.version.trim());
          form.set('changelog', versionForm.changelog);
          form.set('storageKey', versionStorageKey);
          await apiWithUploadProgress(`/api/v1/apps/${app.id}/versions`, {
            method: 'POST',
            body: form,
            onUploadProgress: (percent) => {
              setVersionProgress({
                percent: Math.min(88, Math.max(8, Math.round(percent * 0.82))),
                label: percent >= 100 ? t('submitApp.progressVerifying') : t('submitApp.progressUploading'),
              });
            },
          });
        } else {
          stopStageProgress = startVersionStageProgress(
            versionForm.sourceType === 'GITHUB' && versionForm.useMirrorDownload
              ? t('submitApp.progressMirrorFetching')
              : t('submitApp.progressRemoteFetching'),
          );
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
        stopStageProgress?.();
        stopStageProgress = undefined;
        setVersionProgress({ percent: 100, label: t('submitApp.progressDone') });
        setVersionForm({ version: '', sourceType: 'GITHUB', downloadUrl: '', sha256: '', useMirrorDownload: true, changelog: '' });
        setVersionArtifactMode('local');
        setVersionFile(null);
        if (versionFileInputRef.current) versionFileInputRef.current.value = '';
        setToast({ tone: 'success', message: t('drawer.versionSubmitted') });
        await onRefresh();
      });
    } finally {
      stopStageProgress?.();
      setVersionProgress(null);
      setIsSubmittingVersion(false);
    }
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
    form.set('caption', '');
    form.set('deviceType', screenshotDeviceType);
    form.set('storageKey', screenshotStorageKey);
    await runAction(setToast, t('drawer.screenshotUploadFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots`, { method: 'POST', body: form });
      setScreenshotFile(null);
      setScreenshotDeviceType('DESKTOP');
      setToast({ tone: 'success', message: t('drawer.screenshotUploaded') });
      await onRefresh();
    });
  }

  async function saveScreenshotCaption(screenshotID: number) {
    await runAction(setToast, t('drawer.screenshotCaptionSaveFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/${screenshotID}`, {
        method: 'PATCH',
        body: JSON.stringify({ caption: screenshotCaptionDrafts[screenshotID] || '' }),
      });
      setToast({ tone: 'success', message: t('drawer.screenshotCaptionSaved') });
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

  async function clearOutdatedMarks() {
    if (!confirmDanger('clear-outdated-marks', t('drawer.confirmClearOutdated'))) return;
    await runAction(setToast, t('drawer.clearOutdatedFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/outdated-marks`, { method: 'DELETE' });
      setToast({ tone: 'success', message: t('drawer.outdatedCleared') });
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
    <section className="detail-page-shell">
      <article className={cx('detail-page server-detail-page', isManageMode && 'manage-mode')} aria-labelledby={drawerTitleId}>
        <XButton
          ref={closeButtonRef}
          className="detail-back-button"
          type="button"
          variant="secondary"
          size="sm"
          label={isManageMode ? t('drawer.backToDetail') : t('common.back')}
          icon={<ArrowLeft size={17} />}
          onClick={isManageMode ? () => onModeChange('detail') : onClose}
        />
        <div className="detail-head">
          <AppIcon src={app.iconUrl} seed={app.slug || app.name} title={app.name} size={58} className="detail-avatar" />
          <div>
            <h2 id={drawerTitleId}>{isManageMode ? t('drawer.manageTitle', { name: app.name }) : app.name}</h2>
            <p>{isManageMode ? t('drawer.manageBody') : app.summary || app.description}</p>
            <div className="meta-line">
              <span>{app.owner}</span>
              <span>{localizedCategory(app, t('common.uncategorized'))}</span>
              <span>{app.latestVersion?.version || '-'}</span>
              {hasOutdatedMarks && (
                <span className="status-badge stale">
                  <AlertCircle size={13} />
                  {t('drawer.outdatedBadge', { count: app.outdatedMarks ?? 0 })}
                </span>
              )}
            </div>
          </div>
        </div>
        <div className="detail-actions">
          {isManageMode ? (
            <>
              <XButton type="button" variant="secondary" label={t('drawer.backToDetail')} icon={<ArrowLeft size={18} />} onClick={() => onModeChange('detail')} />
              {canMaintain && (
                <>
                  <XButton type="button" variant="secondary" label={t('drawer.unlist')} icon={<Archive size={18} />} onClick={() => void unlistApp()} />
                  <XButton type="button" variant="destructive" label={t('common.delete')} icon={<Trash2 size={18} />} onClick={() => void deleteApp()} />
                </>
              )}
            </>
          ) : (
            <>
              <XButton
                type="button"
                variant="primary"
                label={installable ? t('common.download') : t('common.unavailable')}
                icon={<Download size={18} />}
                isDisabled={!installable}
                onClick={() => onInstall(app)}
                aria-label={installable ? `${t('common.download')} ${app.name}` : t('app.installUnavailable', { name: app.name })}
              />
              {canOpenManagement && (
                <XButton type="button" variant="secondary" label={t('drawer.manageApp')} icon={<Settings size={18} />} onClick={() => onModeChange('manage')} />
              )}
              {user && (
                <>
                  <XButton type="button" variant="secondary" label={t('drawer.favorite')} icon={<Heart size={18} />} onClick={() => void toggleAppFavorite()} />
                  <XButton type="button" variant="secondary" label={t('drawer.submitter')} icon={<Star size={18} />} onClick={() => void toggleSubmitterFavorite()} />
                </>
              )}
              {user && user.id !== app.ownerId && (
                <XButton type="button" variant="secondary" label={t('drawer.collaborate')} icon={<Users size={18} />} onClick={() => void requestCollaborator()} />
              )}
            </>
          )}
        </div>
        {!isManageMode && (
          <>
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
            <section className={cx('outdated-state', hasOutdatedMarks && 'active')} aria-label={t('drawer.outdatedStatus')}>
              <div className="outdated-state-head">
                <AlertCircle size={19} />
                <div>
                  <strong>{hasOutdatedMarks ? t('drawer.outdatedActiveTitle', { count: app.outdatedMarks ?? 0 }) : t('drawer.outdatedInactiveTitle')}</strong>
                  <span>{hasOutdatedMarks ? t('drawer.outdatedActiveBody') : t('drawer.outdatedInactiveBody')}</span>
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
          </>
        )}
        {isManageMode && (
          <section className="panel nested-panel management-overview">
            <div className="section-title with-action">
              <div>
                <Gauge size={19} />
                <h2>{t('drawer.managementOverview')}</h2>
              </div>
              {hasOutdatedMarks && app.canClearOutdatedMarks && (
                <XButton
                  type="button"
                  variant="secondary"
                  size="sm"
                  label={t('drawer.clearOutdated')}
                  icon={<Check size={17} />}
                  onClick={() => void clearOutdatedMarks()}
                />
              )}
            </div>
            <div className="detail-summary management-summary" aria-label={t('drawer.metadata')}>
              <div>
                <span>{t('drawer.latestVersion')}</span>
                <strong>{latestVersion?.version || t('app.noPublishedVersion')}</strong>
              </div>
              <div>
                <span>{t('drawer.installStatus')}</span>
                <strong>{installable ? t('app.installReady') : t('app.installMissingVersion')}</strong>
              </div>
              <div>
                <span>{t('drawer.outdatedStatus')}</span>
                <strong>{hasOutdatedMarks ? t('drawer.outdatedBadge', { count: app.outdatedMarks ?? 0 }) : t('drawer.outdatedInactiveTitle')}</strong>
              </div>
              <div>
                <span>{t('common.source')}</span>
                <strong>{latestVersion?.sourceType || '-'}</strong>
              </div>
            </div>
          </section>
        )}
        {isManageMode && (canMaintain || canUploadVersion) && (
          <section className={cx('maintenance-grid', !canMaintain && 'single-column')}>
            {canMaintain && (
              <div className="maintenance-main">
              <form className="panel form-panel nested-panel app-info-panel" onSubmit={submitAppInfo}>
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
                <TagTokenizer label={t('common.tags')} value={appForm.tags} knownTags={tagOptions} onChange={(value) => setAppForm({ ...appForm, tags: value })} />
                <XTextInput
                  type="password"
                  label={t('drawer.installPassword')}
                  description={app.installProtected ? t('drawer.installPasswordUpdateHelp') : t('drawer.installPasswordHelp')}
                  value={appForm.installPassword}
                  onChange={(value) => setAppForm({ ...appForm, installPassword: value, clearInstallPassword: false })}
                />
                {app.installProtected && (
                  <XSwitch
                    label={t('drawer.clearInstallPassword')}
                    value={appForm.clearInstallPassword}
                    labelSpacing="spread"
                    width="100%"
                    onChange={(checked) => setAppForm({ ...appForm, clearInstallPassword: checked, installPassword: checked ? '' : appForm.installPassword })}
                  />
                )}
                <XSwitch
                  label={t('drawer.commentsEnabled')}
                  value={appForm.commentsEnabled}
                  labelSpacing="spread"
                  width="100%"
                  onChange={(checked) => setAppForm({ ...appForm, commentsEnabled: checked })}
                />
                <XSwitch
                  label={t('drawer.emailNotificationsEnabled')}
                  value={appForm.emailNotificationsEnabled}
                  labelSpacing="spread"
                  width="100%"
                  onChange={(checked) => setAppForm({ ...appForm, emailNotificationsEnabled: checked })}
                />
                <XSwitch
                  label={t('submitApp.allowUnreviewedUpdates')}
                  value={appForm.allowUnreviewedUpdates}
                  labelSpacing="spread"
                  width="100%"
                  onChange={(checked) => setAppForm({ ...appForm, allowUnreviewedUpdates: checked })}
                />
                <XButton type="submit" variant="secondary" label={t('drawer.saveInfo')} icon={<Save size={18} />} />
              </form>
              </div>
            )}
            <div className="maintenance-side">
            {canUploadVersion && (
              <form className="panel form-panel nested-panel version-publish-panel" onSubmit={submitExternalVersion}>
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
                    <ArtifactModeOption
                      icon={<Upload size={17} />}
                      title={t('submitApp.localArtifact')}
                      hint={t('drawer.versionLocalArtifactHint')}
                      isSelected={versionArtifactMode === 'local'}
                      onSelect={() => selectVersionArtifactMode('local')}
                    />
                    <ArtifactModeOption
                      icon={<Link size={17} />}
                      title={t('submitApp.externalArtifact')}
                      hint={t('drawer.versionExternalArtifactHint')}
                      isSelected={versionArtifactMode === 'external'}
                      onSelect={() => selectVersionArtifactMode('external')}
                    />
                  </div>
                  {versionArtifactMode === 'local' ? (
                    <div className="artifact-fields">
                      <FilePicker
                        label={t('common.lpkFile')}
                        help={t('drawer.versionLocalFileHelp')}
                        value={versionFile}
                        inputRef={versionFileInputRef}
                        accept=".lpk"
                        required
                        onChange={(nextFile) => setVersionFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
                      />
                      {storageChoices.length > 0 && (
                        <XSelector
                          label={t('common.storage')}
                          description={t('drawer.versionStorageHelp')}
                          value={versionStorageKey}
                          options={storageChoices}
                          onChange={setVersionStorageKey}
                        />
                      )}
                    </div>
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
                      {versionForm.sourceType === 'GITHUB' && (
                        <div className="mirror-download-option">
                          <XSwitch
                            label={t('submitApp.useMirrorDownload')}
                            value={versionForm.useMirrorDownload}
                            labelSpacing="spread"
                            width="100%"
                            onChange={(checked) => setVersionForm({ ...versionForm, useMirrorDownload: checked })}
                          />
                          <p className="field-help">{versionMirrorDownloadHelp}</p>
                        </div>
                      )}
                    </div>
                  )}
                </div>
                {!canSubmitVersion && <p className="field-help">{t('drawer.versionSubmitBlocked')}</p>}
                {versionProgress && (
                  <div className="submit-progress" role="status" aria-live="polite">
                    <div className="submit-progress-row">
                      <strong>{versionProgress.label}</strong>
                      <span>{Math.round(versionProgress.percent)}%</span>
                    </div>
                    <div className="progress">
                      <span style={{ width: `${Math.max(4, Math.min(100, versionProgress.percent))}%` }} />
                    </div>
                  </div>
                )}
                <XButton
                  type="submit"
                  variant="secondary"
                  label={isSubmittingVersion ? t('common.submitting') : t('drawer.publishVersion')}
                  icon={<Upload size={18} />}
                  isDisabled={!canSubmitVersion || isSubmittingVersion}
                />
              </form>
            )}
            {canMaintain && (
              <section className="panel form-panel nested-panel visibility-panel">
                <SectionTitle icon={Users} title={t('drawer.visibilityGroups')} />
                <div className="checkbox-list">
                  {groups.length === 0 ? (
                    <span className="muted-text">{t('drawer.noGroupsPublic')}</span>
                  ) : (
                    groups.map((group) => (
                      <XCheckboxInput
                        key={group.id}
                        label={group.name}
                        value={visibility.includes(group.id)}
                        onChange={(checked) =>
                          setVisibility((current) => (checked ? [...current, group.id] : current.filter((id) => id !== group.id)))
                        }
                      />
                    ))
                  )}
                </div>
                <XButton type="button" variant="secondary" label={t('drawer.saveVisibility')} icon={<Users size={18} />} onClick={() => void saveVisibility()} />
              </section>
            )}
            {canMaintain && (
              <section className="panel nested-panel collaborator-panel">
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
                            <XIconButton
                              type="button"
                              variant="ghost"
                              label={t('drawer.approveCollaboratorFor', { name: request.username || request.email || request.id })}
                              icon={<Check size={17} />}
                              onClick={() => void decideCollaboratorRequest(request.id, true)}
                            />
                            <XIconButton
                              type="button"
                              variant="destructive"
                              label={t('drawer.rejectCollaboratorFor', { name: request.username || request.email || request.id })}
                              icon={<X size={17} />}
                              onClick={() => void decideCollaboratorRequest(request.id, false)}
                            />
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </section>
            )}
            </div>
          </section>
        )}
        <section>
          <h3>{t('drawer.screenshots')}</h3>
          {displayScreenshots.length > 0 ? (
            <div className="screenshot-grid">
              {displayScreenshots.map((shot, index, shots) => (
                <figure className="screenshot-item" key={shot.id}>
                  <img src={shot.imageUrl} alt={shot.caption || app.name} />
                  {canEditScreenshots ? (
                    <figcaption className="screenshot-caption-editor">
                      <XTextInput
                        label={t('drawer.screenshotCaptionFor', { index: index + 1 })}
                        isLabelHidden
                        value={screenshotCaptionDrafts[shot.id] ?? shot.caption ?? ''}
                        placeholder={t('drawer.screenshotCaption')}
                        onChange={(value) => setScreenshotCaptionDrafts((current) => ({ ...current, [shot.id]: value }))}
                      />
                      <XIconButton
                        type="button"
                        variant="ghost"
                        label={t('drawer.saveScreenshotCaption')}
                        icon={<Save size={16} />}
                        isDisabled={(screenshotCaptionDrafts[shot.id] ?? '') === (shot.caption || '')}
                        onClick={() => void saveScreenshotCaption(shot.id)}
                      />
                      <small>{screenshotDeviceLabel(t, shot.deviceType)}</small>
                    </figcaption>
                  ) : (
                    <figcaption>
                      <span>{shot.caption || app.name}</span>
                      <small>{screenshotDeviceLabel(t, shot.deviceType)}</small>
                    </figcaption>
                  )}
                  {canEditScreenshots && (
                    <div className="screenshot-actions">
                      <XIconButton type="button" variant="ghost" label={t('drawer.moveScreenshotUp')} icon={<ArrowUp size={15} />} isDisabled={index === 0} onClick={() => void moveScreenshot(shot.id, -1)} />
                      <XIconButton type="button" variant="ghost" label={t('drawer.moveScreenshotDown')} icon={<ArrowDown size={15} />} isDisabled={index === shots.length - 1} onClick={() => void moveScreenshot(shot.id, 1)} />
                      <XIconButton type="button" variant="destructive" label={t('drawer.deleteScreenshot')} icon={<Trash2 size={15} />} onClick={() => void deleteScreenshot(shot.id)} />
                    </div>
                  )}
                </figure>
              ))}
            </div>
          ) : (
            <EmptyState icon={Archive} title={t('drawer.noScreenshots')} />
          )}
          {canEditScreenshots && (
            <form className="comment-form screenshot-form" onSubmit={uploadScreenshot}>
              <FilePicker
                label={t('drawer.uploadScreenshot')}
                value={screenshotFile}
                accept=".png,.jpg,.jpeg,.webp"
                onChange={(nextFile) => setScreenshotFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
              />
              <XSelector
                label={t('drawer.screenshotDevice')}
                isLabelHidden
                value={screenshotDeviceType}
                options={[
                  { value: 'DESKTOP', label: t('drawer.screenshotDeviceDesktop') },
                  { value: 'MOBILE', label: t('drawer.screenshotDeviceMobile') },
                ]}
                onChange={(value) => setScreenshotDeviceType(value as 'DESKTOP' | 'MOBILE')}
              />
              {storageChoices.length > 0 && (
                <XSelector
                  label={t('common.storage')}
                  isLabelHidden
                  value={screenshotStorageKey}
                  options={storageChoices}
                  onChange={setScreenshotStorageKey}
                />
              )}
              <XIconButton type="submit" variant="ghost" label={t('drawer.uploadScreenshot')} icon={<Upload size={17} />} />
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
        {!isManageMode && (
          <section>
            <h3>{t('drawer.comments')}</h3>
            {canComment ? (
              <form className="comment-form rich-comment-form" onSubmit={(event) => void submitComment(event)}>
                <XTextInput
                  label={t('drawer.commentPlaceholder')}
                  isLabelHidden
                  value={commentText}
                  placeholder={t('drawer.commentPlaceholder')}
                  onChange={setCommentText}
                />
                <XIconButton type="submit" variant="ghost" label={t('drawer.postComment')} icon={<MessageSquare size={17} />} isDisabled={!commentText.trim()} />
              </form>
            ) : !commentsAllowed ? (
              <div className="comment-disabled-note" role="note">
                <MessageSquareOff size={17} />
                <span>{t('drawer.commentsDisabled')}</span>
              </div>
            ) : null}
            <CommentList
              comments={app.comments || []}
              canReply={canComment}
              replyTarget={replyTarget}
              replyText={replyText}
              onReplyTarget={setReplyTarget}
              onReplyText={setReplyText}
              onReply={(event, parentId) => void submitComment(event, parentId)}
              onDelete={(commentID) => void deleteComment(commentID)}
            />
          </section>
        )}
      </article>
    </section>
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
        <XButton type="button" variant="secondary" label={action.label} icon={ActionIcon ? <ActionIcon size={18} /> : undefined} onClick={action.onClick} />
      )}
    </div>
  );
}

function MobileTabs({ tab, setTab, items, inert }: { tab: TabKey; setTab: (tab: TabKey) => void; items: readonly NavItem[]; inert?: boolean }) {
  const { t } = useTranslation();
  return (
    <XTabList
      className="mobile-tab-list"
      value={tab}
      onChange={(value) => setTab(value as TabKey)}
      layout="fill"
      size="sm"
      inert={inert}
      aria-hidden={inert ? true : undefined}
      aria-label={t('common.navigation')}
    >
      {items.map((item) => {
        const Icon = item.icon;
        return <XTab key={item.key} value={item.key} label={t(item.labelKey)} icon={<Icon size={18} />} />;
      })}
    </XTabList>
  );
}
