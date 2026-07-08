import {
  Archive,
  Cloud,
  Download,
  History,
  Home,
  LogIn,
  LogOut,
  PackagePlus,
  RefreshCw,
  Search,
  Settings,
  ShieldCheck,
  UserRound,
  X,
} from 'lucide-react';
import { AppShell as XAppShell } from '@astryxdesign/core/AppShell';
import { Theme } from '@astryxdesign/core/theme';
import { Button as XButton } from '@astryxdesign/core/Button';
import { DropdownMenu as XDropdownMenu } from '@astryxdesign/core/DropdownMenu';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Link as XLink } from '@astryxdesign/core/Link';
import { MobileNavToggle as XMobileNavToggle } from '@astryxdesign/core/MobileNav';
import { NavIcon as XNavIcon } from '@astryxdesign/core/NavIcon';
import { SideNav as XSideNav, SideNavHeading as XSideNavHeading, SideNavItem as XSideNavItem } from '@astryxdesign/core/SideNav';
import { Skeleton as XSkeleton } from '@astryxdesign/core/Skeleton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { Toast as XToast } from '@astryxdesign/core/Toast';
import { TopNav as XTopNav } from '@astryxdesign/core/TopNav';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import i18n from './i18n';
import { API_BASE, DEFAULT_SOURCE_NAME, DEFAULT_SOURCE_URL, HAS_API } from './config';
import { api, clientApi, fetchAllPaginated } from './shared/api';
import {
  ANNOUNCEMENT_DISMISS_STORAGE_KEY,
  ANNOUNCEMENT_NOTIFY_STORAGE_KEY,
  ASTRYX_THEME_STORAGE_KEY,
  THEME_STORAGE_KEY,
} from './shared/constants';
import { getAstryxTheme } from './shared/astryxThemes';
import { AstryxThemeSelector, LanguageSelector, readAstryxThemeName, readSystemTheme, readThemeMode, ThemeToggle, type LanguageCode } from './shared/theme';
import { displayUserName } from './shared/appHelpers';
import type {
  Category,
  ClientInstallResult,
  CollaborationData,
  ClientSettings,
  ClientSourceStats,
  Collection,
  Group,
  InstallActivity,
  InstalledApplication,
  InstallHistoryEntry,
  InstallOptions,
  InstallPasswordRequest,
  PaginatedResponse,
  Pagination as PaginationMeta,
  ResolvedTheme,
  Review,
  SetupStatus,
  SiteProfile,
  SortMode,
  SourceApp,
  SourceInput,
  SourceSubscription,
  StorageOption,
  StoreApp,
  TagRecord,
  ThemeMode,
  Toast,
  User,
  Version,
} from './shared/types';
import {
  applicableMirrorsForVersion,
  arrayOrEmpty,
  belongsToSource,
  cx,
  defaultSiteProfile,
  errorMessage,
  findInstalledApplication,
  hasInstallableVersion,
  isSourceStale,
  localizedAppDescription,
  localizedAppName,
  localizedAppSummary,
  runAction,
  selectedSourceVersion,
  shortSHA,
  sourceForApp,
  withInstallPassword,
} from './shared/utils';
import { AnnouncementBanner } from './components/AnnouncementBanner';
import { UserAvatar } from './components/AppIcon';
import { EmptyState } from './shared/components/Feedback';
import { StatusBadge } from './shared/components/StatusBadge';
import { ClientHistoryView } from './modules/client/ClientHistoryView';
import { InstallOptionsDialog } from './modules/client/InstallOptionsDialog';
import { ClientSettingsView } from './modules/client/ClientSettingsView';
import { SourceAppDetailPage } from './modules/client/SourceAppDetailPage';
import { SourcesView as ClientSourcesView } from './modules/client/SourcesView';
import { AdminPanel } from './modules/admin/AdminPanel';
import { LoginPage } from './modules/auth/LoginPage';
import { SetupWizard } from './modules/auth/SetupWizard';
import { ProfileSettingsDialog } from './modules/profile/ProfileSettingsDialog';
import { ProfileView } from './modules/profile/ProfileView';
import { SearchView } from './modules/search/SearchView';
import { AppDrawer, type AppDetailMode } from './modules/storefront/AppDrawer';
import { StorefrontHome } from './modules/storefront/StorefrontHome';

type TabKey = 'home' | 'search' | 'sources' | 'profile' | 'history' | 'settings' | 'admin';
type NavItem = { key: TabKey; labelKey: string; icon: typeof Home };
type AuthMode = 'login' | 'register' | 'verify';

const DEFAULT_REVIEW_PAGINATION: PaginationMeta = { page: 1, pageSize: 0, totalItems: 0, totalPages: 0 };
const DEFAULT_HISTORY_PAGINATION: PaginationMeta = { page: 1, pageSize: 0, totalItems: 0, totalPages: 0 };
const DEFAULT_CLIENT_PAGE_SIZE = 24;

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

function verificationTokenFromURL() {
  const params = new URLSearchParams(window.location.search);
  if (window.location.pathname.includes('verify')) return params.get('token') || '';
  if (window.location.pathname === '/login' && params.get('mode') === 'verify') return params.get('token') || '';
  return '';
}

function collaborationInviteTokenFromURL() {
  if (!window.location.pathname.includes('collaboration-invite')) return '';
  return new URLSearchParams(window.location.search).get('token') || '';
}

function currentRoute() {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

function returnToFromURL() {
  const value = new URLSearchParams(window.location.search).get('returnTo') || '/';
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/login')) return '/';
  return value;
}

function knownAppTags(apps: StoreApp[], catalogTags: TagRecord[] = []) {
  const seen = new Set<string>();
  const out: string[] = [];
  catalogTags.forEach((tag) => {
    const normalized = tag.name.trim();
    const key = normalized.toLowerCase();
    if (!normalized || seen.has(key)) return;
    seen.add(key);
    out.push(normalized);
  });
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

export function App() {
  const { t } = useTranslation();
  const [routeLocation, setRouteLocation] = useState(currentRoute);
  const [themeMode, setThemeMode] = useState<ThemeMode>(readThemeMode);
  const [astryxThemeName, setAstryxThemeName] = useState(readAstryxThemeName);
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(readSystemTheme);
  const [tab, setTab] = useState<TabKey>(() => (verificationTokenFromURL() || collaborationInviteTokenFromURL() ? 'profile' : HAS_API ? 'home' : 'sources'));
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [managedApps, setManagedApps] = useState<StoreApp[]>([]);
  const [collaborationData, setCollaborationData] = useState<CollaborationData>({ owned: [], collaborating: [], outgoingRequests: [] });
  const [sourceApps, setSourceApps] = useState<SourceApp[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [catalogTags, setCatalogTags] = useState<TagRecord[]>([]);
  const [collections, setCollections] = useState<Collection[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [reviews, setReviews] = useState<Review[]>([]);
  const [reviewPagination, setReviewPagination] = useState<PaginationMeta>(DEFAULT_REVIEW_PAGINATION);
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
    defaultPageSize: DEFAULT_CLIENT_PAGE_SIZE,
    autoSyncEnabled: false,
    autoSyncIntervalMinutes: 60,
    syncOnStartup: false,
  });
  const [installedApps, setInstalledApps] = useState<InstalledApplication[]>([]);
  const [installHistory, setInstallHistory] = useState<InstallHistoryEntry[]>([]);
  const [installHistoryPagination, setInstallHistoryPagination] = useState<PaginationMeta>(DEFAULT_HISTORY_PAGINATION);
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
  const [isProfileDialogOpen, setIsProfileDialogOpen] = useState(false);
  const [openSubmitSignal, setOpenSubmitSignal] = useState(0);
  const acceptedCollaborationInviteRef = useRef('');
  const defaultSourceCheckedRef = useRef(false);
  const canReview = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const serverNavItems = user ? [...serverBaseTabs, ...(canReview ? [serverAdminTab] : [])] : serverBaseTabs.filter((item) => item.key !== 'profile');
  const navItems = HAS_API ? serverNavItems : clientTabs;
  const siteTitle = HAS_API ? siteProfile.title : t('appName');
  const footerYear = new Date().getFullYear();
  const siteFooterName = siteProfile.title || siteTitle;
  const siteVersionTip = siteProfile.version ? t('site.serverVersion', { version: siteProfile.version }) : undefined;
  const currentLanguage: LanguageCode = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';
  const routeURL = useMemo(() => new URL(routeLocation, window.location.origin), [routeLocation]);
  const isLoginRoute = HAS_API && routeURL.pathname === '/login';
  const resolvedTheme: ResolvedTheme = themeMode === 'system' ? systemTheme : themeMode;
  const selectedAstryxTheme = useMemo(() => getAstryxTheme(astryxThemeName), [astryxThemeName]);
  const tagOptions = useMemo(() => knownAppTags(apps, catalogTags), [apps, catalogTags]);
  const announcementKey =
    siteProfile.announcement.updatedAt ||
    `${siteProfile.announcement.level}:${siteProfile.announcement.title || ''}:${siteProfile.announcement.body || ''}`;
  const showAnnouncement =
    HAS_API &&
    siteProfile.announcement.enabled &&
    Boolean(siteProfile.announcement.title || siteProfile.announcement.body) &&
    announcementKey !== dismissedAnnouncement;

  const [sources, setSources] = useState<SourceSubscription[]>([]);

  function navigateRoute(path: string) {
    window.history.pushState(null, '', path);
    setRouteLocation(currentRoute());
  }

  function openLogin(returnTo = '/', options: { mode?: AuthMode; next?: 'submit' | 'admin' } = {}) {
    const params = new URLSearchParams();
    if (options.mode && options.mode !== 'login') params.set('mode', options.mode);
    if (options.next) params.set('next', options.next);
    if (returnTo && returnTo !== '/' && !returnTo.startsWith('/login')) {
      params.set('returnTo', returnTo);
    }
    navigateRoute(`/login${params.size ? `?${params.toString()}` : ''}`);
  }

  function completeLogin(nextUser: User) {
    const params = new URLSearchParams(window.location.search);
    const next = params.get('next');
    const returnTo = returnToFromURL();
    navigateRoute(returnTo);
    setSelectedApp(null);
    setSelectedAppMode('detail');
    setSelectedSourceApp(null);
    if (next === 'submit') {
      setTab('profile');
      setOpenSubmitSignal((signal) => signal + 1);
      return;
    }
    if (next === 'admin' && (nextUser.role === 'SOFTWARE_ADMIN' || nextUser.role === 'SITE_ADMIN')) {
      setTab('admin');
      return;
    }
    if (returnTo.startsWith('/collaboration-invite')) {
      setTab('profile');
      return;
    }
    setTab(returnTo.startsWith('/profile') ? 'profile' : returnTo.startsWith('/admin') ? 'admin' : 'home');
  }

  function navigateTo(nextTab: TabKey) {
    if (isLoginRoute) {
      navigateRoute('/');
    }
    setSelectedApp(null);
    setSelectedAppMode('detail');
    setSelectedSourceApp(null);
    setTab(nextTab);
  }

  function openSubmitApp() {
    if (HAS_API && !user) {
      openLogin('/', { next: 'submit' });
      return;
    }
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
    const handlePopState = () => setRouteLocation(currentRoute());
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

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
    if (!HAS_API || isLoginRoute) return;
    const verifyToken = verificationTokenFromURL();
    if (verifyToken && window.location.pathname.includes('verify')) {
      navigateRoute(`/login?mode=verify&token=${encodeURIComponent(verifyToken)}`);
      return;
    }
    if (!user && collaborationInviteTokenFromURL()) {
      openLogin(currentRoute());
    }
  }, [isLoginRoute, routeLocation, user]);

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
  }, [routeLocation, user, t]);

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
        setCatalogTags([]);
        setCollections([]);
        setGroups([]);
        setReviews([]);
        setUser(null);
        setStorageOptions([]);
        return;
      }
      const [siteData, me, appData, categoryData, tagData, collectionData] = await Promise.allSettled([
        api<{ site: SiteProfile }>('/api/v1/site/profile'),
        api<{ user: User }>('/api/v1/auth/me'),
        fetchAllPaginated<StoreApp, 'apps'>(api, '/api/v1/apps', 'apps'),
        api<{ categories: Category[] }>('/api/v1/categories'),
        api<{ tags: TagRecord[] }>('/api/v1/tags'),
        api<{ collections: Collection[] }>('/api/v1/collections'),
      ]);
      if (siteData.status === 'fulfilled') setSiteProfile(siteData.value.site);
      if (me.status === 'fulfilled') setUser(me.value.user);
      if (appData.status === 'fulfilled') setApps(appData.value);
      if (categoryData.status === 'fulfilled') setCategories(categoryData.value.categories);
      if (tagData.status === 'fulfilled') setCatalogTags(tagData.value.tags);
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

  async function refreshCatalogMetadata() {
    if (!HAS_API) return;
    const [categoryData, tagData, collectionData] = await Promise.allSettled([
      api<{ categories: Category[] }>('/api/v1/categories'),
      api<{ tags: TagRecord[] }>('/api/v1/tags'),
      api<{ collections: Collection[] }>('/api/v1/collections'),
    ]);
    if (categoryData.status === 'fulfilled') setCategories(categoryData.value.categories);
    if (tagData.status === 'fulfilled') setCatalogTags(tagData.value.tags);
    if (collectionData.status === 'fulfilled') setCollections(collectionData.value.collections);
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
      const data = await fetchAllPaginated<StoreApp, 'apps'>(api, '/api/v1/apps?managed=1', 'apps');
      setManagedApps(arrayOrEmpty(data));
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
    const data = await fetchAllPaginated<SourceApp, 'apps'>(clientApi, '/apps', 'apps');
    const nextApps = arrayOrEmpty(data);
    setSourceApps(nextApps);
    return nextApps;
  }

  async function loadClientSettings() {
    const data = await clientApi<{ settings: ClientSettings }>('/settings');
    const defaultSettings: ClientSettings = {
      commentDisplayName: '',
      defaultPageSize: DEFAULT_CLIENT_PAGE_SIZE,
      autoSyncEnabled: false,
      autoSyncIntervalMinutes: 60,
      syncOnStartup: false,
    };
    const nextSettings = { ...defaultSettings, ...(data.settings || {}) };
    setClientSettings(nextSettings);
    return nextSettings;
  }

  async function loadInstallHistory(page = installHistoryPagination.page || 1, pageSize = installHistoryPagination.pageSize) {
    const params = new URLSearchParams({ page: String(page) });
    if (pageSize > 0) params.set('pageSize', String(pageSize));
    const data = await clientApi<PaginatedResponse<InstallHistoryEntry, 'history'>>(`/history?${params.toString()}`);
    const nextHistory = arrayOrEmpty(data.history);
    setInstallHistory(nextHistory);
    setInstallHistoryPagination(data.pagination || { page, pageSize, totalItems: nextHistory.length, totalPages: 1 });
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

  async function loadReviews(page = reviewPagination.page, pageSize = reviewPagination.pageSize) {
    await runAction(setToast, t('toast.loadReviewsFailed'), async () => {
      const params = new URLSearchParams({ status: 'PENDING', page: String(page || 1) });
      if (pageSize > 0) params.set('pageSize', String(pageSize));
      const data = await api<PaginatedResponse<Review, 'reviews'>>(`/api/v1/admin/reviews?${params.toString()}`);
      setReviews(data.reviews || []);
      setReviewPagination(data.pagination || { page, pageSize, totalItems: data.reviews?.length || 0, totalPages: 1 });
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
        localizedAppName(app).toLowerCase().includes(needle) ||
        app.summary.toLowerCase().includes(needle) ||
        localizedAppSummary(app).toLowerCase().includes(needle) ||
        localizedAppDescription(app).toLowerCase().includes(needle) ||
        app.owner.toLowerCase().includes(needle) ||
        app.tags.join(' ').toLowerCase().includes(needle);
      return categoryMatch && submitterMatch && queryMatch;
    });
    return [...filtered].sort((a, b) => {
      if (sortMode === 'downloads') return b.downloadCount - a.downloadCount;
      if (sortMode === 'name') return localizedAppName(a).localeCompare(localizedAppName(b));
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
    const appName = localizedAppName(app);
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
        title: `${appName} ${version.version}`,
        source,
        checksum,
        status: 'running',
        progress: 12,
        stageKey: 'installActivity.stagePrepare',
      });
      await new Promise((resolve) => window.setTimeout(resolve, 180));
      setInstallActivity((current) =>
        current && current.title === `${appName} ${version.version}`
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
        title: `${appName} ${version.version}`,
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
        groupCodes: source.groupCodes || [],
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
        <AppToast toast={toast} onDismiss={() => setToast(null)} />
      </Theme>
    );
  }

  if (HAS_API && isLoginRoute) {
    return (
      <Theme theme={selectedAstryxTheme.theme} mode={themeMode}>
        <LoginPage
          siteTitle={siteTitle}
          siteProfile={siteProfile}
          currentLanguage={currentLanguage}
          themeMode={themeMode}
          astryxThemeName={astryxThemeName}
          onLanguageChange={(language) => void i18n.changeLanguage(language)}
          onThemeModeChange={setThemeMode}
          onAstryxThemeChange={setAstryxThemeName}
          onBrowse={() => {
            navigateRoute('/');
            setTab('home');
          }}
          onAuthenticated={completeLogin}
          setUser={setUser}
          refreshAll={refreshAll}
          setToast={setToast}
        />
        <AppToast toast={toast} onDismiss={() => setToast(null)} />
      </Theme>
    );
  }

  return (
    <Theme theme={selectedAstryxTheme.theme} mode={themeMode}>
      <>
        <a className="skip-link" href="#main-content">{t('common.skipToMain')}</a>
        <XAppShell
          className="app-shell"
          variant="section"
          height="fill"
          contentPadding={0}
          mobileNav={{ breakpoint: 'md', hasToggle: false }}
          sideNav={(
            <XSideNav
              className="app-side-nav"
              header={(
                <XSideNavHeading
                  icon={(
                    <XNavIcon
                      icon={
                        HAS_API && siteProfile.iconUrl
                          ? <img className="side-nav-logo" src={siteProfile.iconUrl} alt="" />
                          : <Archive size={18} />
                      }
                    />
                  )}
                  heading={siteTitle}
                />
              )}
              footer={HAS_API && siteProfile.publicUrl ? (
                <footer className="app-version" title={siteVersionTip} aria-label={siteVersionTip}>
                  <XLink href={siteProfile.publicUrl} isExternalLink isStandalone>
                    {t('site.footer', { year: footerYear, name: siteFooterName })}
                  </XLink>
                </footer>
              ) : undefined}
            >
              {navItems.map((item) => {
                const Icon = item.icon;
                return (
                  <XSideNavItem
                    key={item.key}
                    label={t(item.labelKey)}
                    icon={<Icon size={19} />}
                    isSelected={tab === item.key}
                    onClick={() => navigateTo(item.key)}
                  />
                );
              })}
            </XSideNav>
          )}
          topNav={(
            <XTopNav
              className="topbar"
              label={t('common.navigation')}
              heading={<XMobileNavToggle label={t('common.navigation')} />}
              startContent={(
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
              )}
              endContent={(
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
                      <XDropdownMenu
                        button={{
                          label: displayUserName(user),
                          variant: 'secondary',
                          className: 'account-trigger',
                          children: (
                            <span className="account-trigger-content">
                              <UserAvatar user={user} size={32} className="account-avatar" />
                              <span className="account-trigger-name">{displayUserName(user)}</span>
                            </span>
                          ),
                        }}
                        menuWidth={192}
                        items={[
                          {
                            label: t('profile.personalProfile'),
                            icon: <UserRound size={16} />,
                            onClick: () => setIsProfileDialogOpen(true),
                          },
                          {
                            label: t('auth.logout'),
                            icon: <LogOut size={16} />,
                            onClick: () =>
                              void runAction(setToast, t('toast.logoutFailed'), async () => {
                                await api('/api/v1/auth/logout', { method: 'POST' });
                                setUser(null);
                                setStorageOptions([]);
                              }),
                          },
                        ]}
                      />
                    </div>
                  ) : HAS_API ? (
                    <XButton type="button" variant="secondary" label={t('topbar.login')} icon={<LogIn size={16} />} onClick={() => openLogin('/')} />
                  ) : null}
                </div>
              )}
            />
          )}
        >
          <div className="main" id="main-content" tabIndex={-1}>

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
              <XSkeleton height={74} radius={2} index={0} />
              <XSkeleton height={74} radius={2} index={1} />
              <XSkeleton height={74} radius={2} index={2} />
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
                defaultPageSize={HAS_API ? siteProfile.defaultPageSize || DEFAULT_CLIENT_PAGE_SIZE : clientSettings.defaultPageSize || DEFAULT_CLIENT_PAGE_SIZE}
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
                storageOptions={storageOptions}
                collaborationData={collaborationData}
                onCollaborationRefresh={loadCollaborationData}
                siteProfile={siteProfile}
                openSubmitSignal={openSubmitSignal}
                onNavigate={navigateTo}
                onLogin={() => openLogin('/profile')}
              />
            )}
            {tab === 'history' && !HAS_API && (
              <ClientHistoryView
                history={installHistory}
                pagination={installHistoryPagination}
                sourceApps={sourceApps}
                onRefresh={() => void loadInstallHistory(installHistoryPagination.page || 1, installHistoryPagination.pageSize)}
                onPageChange={(page, pageSize) => void loadInstallHistory(page, pageSize)}
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
                <AdminPanel
                  user={user}
                  reviews={reviews}
                  reviewPagination={reviewPagination}
                  onReviewPageChange={loadReviews}
                  onApprove={approveReview}
                  onSiteProfileSaved={applySiteProfile}
                  onCatalogMetadataChanged={refreshCatalogMetadata}
                  onStorageOptionsChanged={loadStorageOptions}
                  setToast={setToast}
                />
              ) : (
                <EmptyState
                  icon={ShieldCheck}
                  title={user ? t('admin.noPermission') : t('auth.loginRequired')}
                  body={user ? t('admin.noPermissionBody') : t('auth.loginRequiredBody')}
                  action={!user ? { label: t('auth.login'), icon: LogIn, onClick: () => openLogin('/admin', { next: 'admin' }) } : undefined}
                />
              )
            )}
            </>
            )}
          </>
        )}
          </div>
        </XAppShell>

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
              <StatusBadge
                tone={installActivity.status === 'running' ? 'syncing' : installActivity.status === 'success' ? 'approved' : 'failed'}
                label={t(`installActivity.status.${installActivity.status}`)}
              />
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

      <AppToast toast={toast} onDismiss={() => setToast(null)} />
      </>
    </Theme>
  );
}

function AppToast({ toast, onDismiss }: { toast: Toast | null; onDismiss: () => void }) {
  if (!toast) return null;
  return (
    <div className="toast-host">
      <XToast
        body={toast.message}
        type={toast.tone === 'error' ? 'error' : 'info'}
        isAutoHide={toast.tone !== 'error'}
        autoHideDuration={3200}
        onDismiss={onDismiss}
      />
    </div>
  );
}
