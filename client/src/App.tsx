import {
  AlertCircle,
  Archive,
  ArrowDown,
  ArrowUp,
  Check,
  ChevronRight,
  Cloud,
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
  PackagePlus,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Save,
  Star,
  Tag,
  Trash2,
  Upload,
  UserRound,
  Users,
  X,
} from 'lucide-react';
import { Avatar } from '@humation/react';
import { humation1 } from '@humation/assets-humation-1';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import i18n from './i18n';
import { API_BASE, DEFAULT_SOURCE_NAME, DEFAULT_SOURCE_URL, HAS_API } from './config';
import { installWithLazyCat, queryInstalledApplications } from './lazycatSdk';

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

type SourceSubscription = {
  id: string;
  name: string;
  url: string;
  password: string;
  mirror: string;
  lastSync?: string;
};

type SourceApp = {
  id: number;
  sourceName: string;
  name: string;
  slug: string;
  summary: string;
  category?: string;
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

type Toast = {
  tone: 'success' | 'error' | 'neutral';
  message: string;
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

type TabKey = 'home' | 'categories' | 'search' | 'sources' | 'profile';
type NavItem = { key: TabKey; labelKey: string; icon: typeof Home };

const serverTabs: NavItem[] = [
  { key: 'home', labelKey: 'nav.store', icon: Home },
  { key: 'categories', labelKey: 'nav.categories', icon: Layers3 },
  { key: 'search', labelKey: 'nav.discover', icon: Search },
  { key: 'profile', labelKey: 'nav.submitAdmin', icon: UserRound },
];

const clientTabs: NavItem[] = [
  { key: 'sources', labelKey: 'nav.sources', icon: Cloud },
  { key: 'search', labelKey: 'nav.install', icon: Download },
  { key: 'profile', labelKey: 'nav.device', icon: Server },
];

type SortMode = 'recent' | 'downloads' | 'name';

function verificationTokenFromURL() {
  return new URLSearchParams(window.location.search).get('token') || '';
}

export function App() {
  const { t } = useTranslation();
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
  const [installing, setInstalling] = useState<string | null>(null);
  const [installProgress, setInstallProgress] = useState(0);
  const [toast, setToast] = useState<Toast | null>(null);
  const [loading, setLoading] = useState(true);
  const navItems = HAS_API ? serverTabs : clientTabs;
  const modeLabel = HAS_API ? t('mode.serverStore') : t('mode.standaloneClient');
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';

  const [sources, setSources] = useState<SourceSubscription[]>(() => {
    const saved = localStorage.getItem('lazycat.sources');
    if (!saved) {
      return DEFAULT_SOURCE_URL
        ? [{ id: 'default-source', name: DEFAULT_SOURCE_NAME, url: DEFAULT_SOURCE_URL, password: '', mirror: '' }]
        : [];
    }
    try {
      return JSON.parse(saved) as SourceSubscription[];
    } catch {
      return [];
    }
  });

  const canReview = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';

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
    localStorage.setItem('lazycat.sources', JSON.stringify(sources));
  }, [sources]);

  useEffect(() => {
    void refreshAll();
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  async function refreshAll() {
    if (!HAS_API) {
      setApps([]);
      setCategories([]);
      setCollections([]);
      setGroups([]);
      setReviews([]);
      setUser(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
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

  const submitters = useMemo(() => {
    return Array.from(new Set(apps.map((app) => app.owner).filter(Boolean))).sort((a, b) => a.localeCompare(b));
  }, [apps]);

  const filteredApps = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const filtered = apps.filter((app) => {
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
  }, [apps, activeCategory, activeSubmitter, query, sortMode]);

  async function openApp(app: StoreApp) {
    await runAction(setToast, t('toast.loadAppDetailFailed'), async () => {
      const data = await api<{ app: StoreApp }>(`/api/v1/apps/${app.id}`);
      setSelectedApp(data.app);
    });
  }

  async function installApp(app: StoreApp | SourceApp) {
    await runAction(setToast, t('toast.installFailed'), async () => {
      const version = app.latestVersion;
      if (!version) {
        setToast({ tone: 'error', message: t('toast.noInstallableVersion') });
        return;
      }
      setInstalling(`${app.name} ${version.version}`);
      setInstallProgress(0);
      try {
        for (const point of [18, 41, 68]) {
          await new Promise((resolve) => window.setTimeout(resolve, 260));
          setInstallProgress(point);
        }
        const downloadUrl =
          'sourceName' in app ? version.downloadUrl : `${API_BASE}/api/v1/apps/${app.id}/versions/${(version as Version).id}/download`;
        const result = await installWithLazyCat({
          name: app.name,
          appId: app.slug,
          pkgId: app.slug,
          downloadUrl,
          sha256: version.sha256,
        });
        setInstallProgress(100);
        setToast({
          tone: result.mode === 'lazycat-sdk' || result.mode === 'download' ? 'success' : 'error',
          message: t(result.messageKey, result.messageParams),
        });
      } finally {
        window.setTimeout(() => setInstalling(null), 700);
      }
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
    const url = new URL(source.url);
    if (source.password) url.searchParams.set('password', source.password);
    const response = await fetch(url.toString(), { headers: source.password ? { 'X-Source-Password': source.password } : undefined });
    if (response.status === 401) throw new Error(t('toast.sourcePasswordInvalid'));
    if (!response.ok) throw new Error(t('toast.sourceSyncFailed'));
    const data = await response.json();
    const mirroredDownloadURL = (version: any) => {
      const upstream = String(version.upstreamDownloadUrl || '');
      const direct = String(version.downloadUrl || '');
      const githubURL = upstream || direct;
      if (source.mirror && (githubURL.includes('github.com/') || githubURL.includes('githubusercontent.com/'))) {
        return `${source.mirror.replace(/\/$/, '')}/${githubURL}`;
      }
      return direct;
    };
    const imported = (data.apps || []).map((app: any) => ({
      id: app.id,
      sourceName: source.name,
      name: app.name,
      slug: app.slug,
      summary: app.summary,
      category: app.category,
      latestVersion: app.latestVersion
        ? {
            ...app.latestVersion,
            downloadUrl: mirroredDownloadURL(app.latestVersion),
          }
        : undefined,
    }));
    setSourceApps((current) => [...current.filter((app) => app.sourceName !== source.name), ...imported]);
    setSources((current) => current.map((item) => (item.id === source.id ? { ...item, lastSync: new Date().toISOString() } : item)));
    if (!options.quiet) setToast({ tone: 'success', message: t('toast.sourceSynced') });
  }

  async function syncAllSources() {
    if (sources.length === 0) {
      setTab('sources');
      setToast({ tone: 'neutral', message: t('toast.addSourceFirst') });
      return;
    }
    await runAction(setToast, t('toast.sourceSyncFailed'), async () => {
      for (const source of sources) {
        await syncSource(source, { quiet: true });
      }
      setToast({ tone: 'success', message: t('toast.allSourcesSynced', { count: sources.length }) });
      setTab('search');
    });
  }

  return (
    <div className="shell">
      <a className="skip-link" href="#main-content" inert={!!selectedApp} aria-hidden={selectedApp ? true : undefined}>{t('common.skipToMain')}</a>
      <aside className="sidebar" inert={!!selectedApp} aria-hidden={selectedApp ? true : undefined}>
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

      <main className="main" id="main-content" tabIndex={-1} inert={!!selectedApp} aria-hidden={selectedApp ? true : undefined}>
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
          <div className="loading-state">
            <Gauge size={28} />
            <span>{t('common.loading')}</span>
          </div>
        ) : (
          <>
            {tab === 'home' && (
              <HomeView
                apps={filteredApps}
                collections={collections}
                reviews={reviews}
                canReview={canReview}
                onOpen={openApp}
                onInstall={installApp}
                onApprove={approveReview}
                onNavigate={setTab}
              />
            )}
            {tab === 'categories' && (
              <CategoryView
                apps={filteredApps}
                categories={categories}
                activeCategory={activeCategory}
                onCategory={setActiveCategory}
                onOpen={openApp}
                onInstall={installApp}
              />
            )}
            {tab === 'search' && (
              <SearchView
                apps={filteredApps}
                sourceApps={sourceApps}
                submitters={submitters}
                activeSubmitter={activeSubmitter}
                sortMode={sortMode}
                query={query}
                mode={HAS_API ? 'server' : 'client'}
                sourceCount={sources.length}
                onSubmitter={setActiveSubmitter}
                onSortMode={setSortMode}
                onOpen={openApp}
                onInstall={installApp}
                onGoSources={() => setTab('sources')}
              />
            )}
            {tab === 'sources' && (
              <SourcesView
                sources={sources}
                setSources={setSources}
                sourceApps={sourceApps}
                onSync={syncSource}
                onSyncAll={syncAllSources}
                onInstall={installApp}
                setToast={setToast}
              />
            )}
            {tab === 'profile' && <ProfileView user={user} setUser={setUser} groups={groups} setGroups={setGroups} categories={categories} refreshAll={refreshAll} setToast={setToast} hasAPI={HAS_API} />}
          </>
        )}
      </main>

      <MobileTabs tab={tab} setTab={setTab} items={navItems} inert={!!selectedApp} />

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

      {installing && (
        <div className="install-panel" inert={!!selectedApp} aria-hidden={selectedApp ? true : undefined}>
          <Download size={20} />
          <div>
            <strong>{installing}</strong>
            <div className="progress">
              <span style={{ width: `${installProgress}%` }} />
            </div>
          </div>
          <span>{installProgress}%</span>
        </div>
      )}

      {toast && <div className={cx('toast', toast.tone)}>{toast.message}</div>}
    </div>
  );
}

function HomeView({
  apps,
  collections,
  reviews,
  canReview,
  onOpen,
  onInstall,
  onApprove,
  onNavigate,
}: {
  apps: StoreApp[];
  collections: Collection[];
  reviews: Review[];
  canReview: boolean;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onApprove: (review: Review, approve: boolean) => void;
  onNavigate: (tab: TabKey) => void;
}) {
  const { t } = useTranslation();
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  const approvedCount = apps.filter((app) => app.status === 'APPROVED').length;
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
          <div><ShieldCheck size={18} /> {t('home.review')}</div>
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
          <span>{t('home.pendingReviews')}</span>
          <strong>{reviews.length}</strong>
          <small>{t('home.pendingCount', { count: reviews.length })}</small>
        </div>
        <div className="metric-card source-feed-card">
          <span>{t('home.sourceUrl')}</span>
          <strong>/source/v1/index.json</strong>
          <small>{t('home.openSourceFeed')}</small>
        </div>
      </section>

      {canReview && reviews.length > 0 && (
        <section className="panel">
          <SectionTitle icon={ShieldCheck} title={t('home.pendingReviews')} />
          <div className="review-list">
            {reviews.slice(0, 4).map((review) => (
              <div className="review-row" key={review.id}>
                <div>
                  <strong>{review.kind.replaceAll('_', ' ')}</strong>
                  <span>#{review.appId || review.versionId} · {formatDate(review.createdAt)}</span>
                </div>
                <div className="row-actions">
                  <button
                    type="button"
                    className="icon-button ok"
                    aria-label={t('home.approveReview', { id: review.id, kind: review.kind.replaceAll('_', ' ') })}
                    onClick={() => void onApprove(review, true)}
                  >
                    <Check size={17} />
                  </button>
                  <button
                    type="button"
                    className="icon-button danger"
                    aria-label={t('home.rejectReview', { id: review.id, kind: review.kind.replaceAll('_', ' ') })}
                    onClick={() => void onApprove(review, false)}
                  >
                    <X size={17} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="panel">
        <SectionTitle icon={History} title={t('home.latest')} />
        <AppGrid apps={latest} onOpen={onOpen} onInstall={onInstall} />
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

function CategoryView({
  apps,
  categories,
  activeCategory,
  onCategory,
  onOpen,
  onInstall,
}: {
  apps: StoreApp[];
  categories: Category[];
  activeCategory: string;
  onCategory: (category: string) => void;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
}) {
  const { t } = useTranslation();
  return (
    <section className="page-grid">
      <div className="page-heading">
        <h1>{t('categories.title')}</h1>
        <p>{t('search.serverDescription')}</p>
      </div>
      <div className="segmented">
        <button type="button" className={cx(activeCategory === 'all' && 'active')} onClick={() => onCategory('all')}>{t('common.all')}</button>
        {categories.map((category) => (
          <button type="button" key={category.id} className={cx(activeCategory === category.name && 'active')} onClick={() => onCategory(category.name)}>
            {category.name}
          </button>
        ))}
      </div>
      <AppGrid apps={apps} onOpen={onOpen} onInstall={onInstall} />
    </section>
  );
}

function SearchView({
  apps,
  sourceApps,
  submitters,
  activeSubmitter,
  sortMode,
  query,
  mode,
  sourceCount,
  onSubmitter,
  onSortMode,
  onOpen,
  onInstall,
  onGoSources,
}: {
  apps: StoreApp[];
  sourceApps: SourceApp[];
  submitters: string[];
  activeSubmitter: string;
  sortMode: SortMode;
  query: string;
  mode: 'server' | 'client';
  sourceCount: number;
  onSubmitter: (submitter: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp | SourceApp) => void;
  onGoSources: () => void;
}) {
  const { t } = useTranslation();
  const sourceNeedle = query.trim().toLowerCase();
  const filteredSourceApps = sourceApps.filter((app) => {
    if (!sourceNeedle) return true;
    return [app.name, app.summary, app.category, app.sourceName].filter(Boolean).join(' ').toLowerCase().includes(sourceNeedle);
  });

  if (mode === 'client') {
    return (
      <section className="page-grid">
        <div className="page-heading with-action">
          <div>
            <span className="eyebrow subtle">{t('search.sourceCount', { count: sourceCount })}</span>
            <h1>{t('search.clientTitle')}</h1>
            <p>{t('search.clientDescription')}</p>
          </div>
          <button type="button" className="secondary-button" onClick={onGoSources}>
            <Cloud size={18} />
            <span>{t('search.noSyncedAppsAction')}</span>
          </button>
        </div>
        <section className="panel">
          <SectionTitle icon={Download} title={t('search.subscribedApps')} />
          <SourceAppGrid apps={filteredSourceApps} onInstall={onInstall} onGoSources={onGoSources} />
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
        <AppGrid apps={apps} onOpen={onOpen} onInstall={onInstall} />
      </section>
      <section className="panel">
        <SectionTitle icon={Cloud} title={t('search.subscribedApps')} />
        <SourceAppGrid apps={filteredSourceApps} onInstall={onInstall} onGoSources={onGoSources} />
      </section>
    </section>
  );
}

function SourceAppGrid({
  apps,
  onInstall,
  onGoSources,
  showEmptyAction = true,
}: {
  apps: SourceApp[];
  onInstall: (app: SourceApp) => void;
  onGoSources: () => void;
  showEmptyAction?: boolean;
}) {
  const { t } = useTranslation();
  if (apps.length === 0) {
    return (
      <div className="empty-state action-empty">
        <Cloud size={28} />
        <strong>{t('search.noSyncedApps')}</strong>
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
      {apps.map((app) => (
        <article className="source-app-card" key={`${app.sourceName}-${app.id}`}>
          <div className="app-open static">
            <AvatarIcon seed={`${app.sourceName}:${app.slug || app.name}`} title={app.name} />
            <div>
              <h3>{app.name}</h3>
              <p>{app.summary || t('common.lpkApp')}</p>
            </div>
          </div>
          <div className="app-meta">
            <span><Cloud size={14} /> {app.sourceName}</span>
            <span><Tag size={14} /> {app.category || t('common.uncategorized')}</span>
            <span><Star size={14} /> {app.latestVersion?.version || '-'}</span>
          </div>
          <button type="button" className="install-button" onClick={() => void onInstall(app)} aria-label={t('app.install', { name: app.name })}>
            <Download size={17} />
            <span>{t('common.install')}</span>
          </button>
        </article>
      ))}
    </div>
  );
}

function SourcesView({
  sources,
  setSources,
  sourceApps,
  onSync,
  onSyncAll,
  onInstall,
  setToast,
}: {
  sources: SourceSubscription[];
  setSources: (update: SourceSubscription[] | ((current: SourceSubscription[]) => SourceSubscription[])) => void;
  sourceApps: SourceApp[];
  onSync: (source: SourceSubscription) => Promise<void>;
  onSyncAll: () => Promise<void>;
  onInstall: (app: SourceApp) => void;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const emptyDraft = { name: '', url: DEFAULT_SOURCE_URL, password: '', mirror: '' };
  const [draft, setDraft] = useState(emptyDraft);
  const [syncingID, setSyncingID] = useState<string | null>(null);

  function addSource(event: FormEvent) {
    event.preventDefault();
    if (!draft.name.trim() || !draft.url.trim()) return;
    setSources((current) => [...current, { id: crypto.randomUUID(), ...draft }]);
    setDraft(emptyDraft);
  }

  function updateSource(id: string, patch: Partial<SourceSubscription>) {
    setSources((current) => current.map((source) => (source.id === id ? { ...source, ...patch } : source)));
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

      <section className="split">
      <form className="panel form-panel" onSubmit={addSource}>
        <SectionTitle icon={Cloud} title={t('sources.addTitle')} />
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
        <button type="submit" className="primary-button">
          <Cloud size={18} />
          <span>{t('sources.add')}</span>
        </button>
      </form>

      <section className="panel">
        <SectionTitle icon={Server} title={t('sources.subscriptions')} />
        <div className="source-list">
          {sources.length === 0 ? (
            <EmptyState icon={Cloud} title={t('sources.empty')} />
          ) : (
            sources.map((source) => (
              <div className="source-row" key={source.id}>
                <div>
                  <strong>{source.name}</strong>
                  <span>{source.url}</span>
                  {source.lastSync && <small>{t('sources.lastSync', { time: formatDate(source.lastSync) })}</small>}
                  <div className="source-edit-grid">
                    <input
                      aria-label={t('sources.passwordFor', { name: source.name })}
                      value={source.password}
                      type="password"
                      placeholder={t('sources.passwordPlaceholder')}
                      onChange={(event) => updateSource(source.id, { password: event.target.value })}
                    />
                    <input
                      aria-label={t('sources.mirrorFor', { name: source.name })}
                      value={source.mirror}
                      placeholder={t('sources.mirrorPlaceholder')}
                      onChange={(event) => updateSource(source.id, { mirror: event.target.value })}
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
                  <button type="button" className="icon-button danger" aria-label={t('sources.deleteSource', { name: source.name })} onClick={() => setSources((current) => current.filter((item) => item.id !== source.id))}>
                    <X size={17} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      </section>
      </section>

      <section className="panel">
        <SectionTitle icon={Download} title={t('sources.syncedApps')} />
        <SourceAppGrid apps={sourceApps} onInstall={onInstall} onGoSources={() => undefined} showEmptyAction={false} />
      </section>
    </section>
  );
}

function ProfileView({
  user,
  setUser,
  groups,
  setGroups,
  categories,
  refreshAll,
  setToast,
  hasAPI,
}: {
  user: User | null;
  setUser: (user: User | null) => void;
  groups: Group[];
  setGroups: (groups: Group[]) => void;
  categories: Category[];
  refreshAll: () => Promise<void>;
  setToast: (toast: Toast) => void;
  hasAPI: boolean;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'login' | 'register' | 'verify'>('login');
  const [authForm, setAuthForm] = useState({ username: 'admin', password: 'changeme', email: '' });
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
  });
  const [file, setFile] = useState<File | null>(null);
  const [tokens, setTokens] = useState<APITokenRecord[]>([]);
  const [newToken, setNewToken] = useState('');
  const [favorites, setFavorites] = useState<FavoriteData>({ apps: [], submitters: [] });
  const [installedApps, setInstalledApps] = useState<Array<{ appid?: string; title?: string; version?: string; status?: number }>>([]);
  const authModeLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verify');
  const authSubmitLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verifyEmail');

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

  async function submitUpload(event: FormEvent) {
    event.preventDefault();
    if (!file && !uploadForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (!file && !uploadForm.sha256.trim()) {
      setToast({ tone: 'error', message: t('submitApp.sha256Required') });
      return;
    }
    await runAction(setToast, t('submitApp.failed'), async () => {
      if (file) {
        const form = new FormData();
        Object.entries(uploadForm).forEach(([key, value]) => form.set(key, String(value)));
        form.set('file', file);
        await api('/api/v1/apps', { method: 'POST', body: form });
      } else {
        await api('/api/v1/apps', {
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
            downloadUrl: uploadForm.downloadUrl,
            sha256: uploadForm.sha256,
          }),
        });
      }
      setToast({ tone: 'success', message: t('submitApp.submitted') });
      setUploadForm({ name: '', version: '0.1.0', summary: '', description: '', categoryId: '', tags: '', allowUnreviewedUpdates: false, sourceType: 'GITHUB', downloadUrl: '', sha256: '' });
      setFile(null);
      await refreshAll();
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

  async function loadInstalledApps() {
    try {
      const result = await queryInstalledApplications();
      setInstalledApps(result?.infoList || []);
      setToast({ tone: 'success', message: t('profile.installedRefreshed') });
    } catch {
      setToast({ tone: 'error', message: t('profile.lazycatSdkUnavailable') });
    }
  }

  if (!hasAPI) {
    return (
      <section className="page-grid">
        <div className="split">
          <div className="panel profile-card">
            <AvatarIcon seed="lazycat-standalone-client" title={t('profile.clientTitle')} size={74} className="avatar-large" />
            <h2>{t('profile.clientTitle')}</h2>
            <p>{t('profile.clientBody')}</p>
            <button type="button" className="primary-button" onClick={() => void loadInstalledApps()}>
              <RefreshCw size={18} />
              <span>{t('profile.readInstalled')}</span>
            </button>
          </div>
          <section className="panel">
            <SectionTitle icon={Download} title={t('profile.installed')} />
            <div className="review-list">
              {installedApps.length === 0 ? (
                <EmptyState icon={Download} title={t('profile.installedEmpty')} />
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
      <form className="panel form-panel profile-panel" onSubmit={submitAuth}>
        <SectionTitle icon={KeyRound} title={mode === 'verify' ? t('auth.verifyEmail') : authModeLabel} />
        <div className="segmented compact">
          <button type="button" className={cx(mode === 'login' && 'active')} onClick={() => setMode('login')}>{t('auth.login')}</button>
          <button type="button" className={cx(mode === 'register' && 'active')} onClick={() => setMode('register')}>{t('auth.register')}</button>
          <button type="button" className={cx(mode === 'verify' && 'active')} onClick={() => setMode('verify')}>{t('auth.verify')}</button>
        </div>
        {mode === 'verify' ? (
          <label>
            <span>{t('auth.verifyToken')}</span>
            <input value={verifyToken} onChange={(event) => setVerifyToken(event.target.value)} />
          </label>
        ) : (
          <>
            <label>
              <span>{t('common.username')}</span>
              <input value={authForm.username} onChange={(event) => setAuthForm({ ...authForm, username: event.target.value })} />
            </label>
            {mode === 'register' && (
              <label>
                <span>{t('common.email')}</span>
                <input type="email" value={authForm.email} onChange={(event) => setAuthForm({ ...authForm, email: event.target.value })} />
              </label>
            )}
            <label>
              <span>{t('common.password')}</span>
              <input type="password" value={authForm.password} onChange={(event) => setAuthForm({ ...authForm, password: event.target.value })} />
            </label>
          </>
        )}
        <button type="submit" className="primary-button" aria-label={authSubmitLabel}>
          <LogIn size={18} />
          <span>{authSubmitLabel}</span>
        </button>
      </form>
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

        <form className="panel form-panel" onSubmit={submitUpload}>
        <SectionTitle icon={Upload} title={t('submitApp.title')} />
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
          <input value={uploadForm.downloadUrl} onChange={(event) => setUploadForm({ ...uploadForm, downloadUrl: event.target.value })} />
        </label>
        <label>
          <span>{t('common.sha256')}</span>
          <input value={uploadForm.sha256} onChange={(event) => setUploadForm({ ...uploadForm, sha256: event.target.value })} />
        </label>
        <label className="toggle-line">
          <input
            type="checkbox"
            checked={uploadForm.allowUnreviewedUpdates}
            onChange={(event) => setUploadForm({ ...uploadForm, allowUnreviewedUpdates: event.target.checked })}
          />
          <span>{t('submitApp.allowUnreviewedUpdates')}</span>
        </label>
        <label>
          <span>{t('common.lpkFile')}</span>
          <input type="file" accept=".lpk" onChange={(event) => setFile(event.target.files?.[0] || null)} />
        </label>
        <button type="submit" className="primary-button">
          <Upload size={18} />
          <span>{t('common.submit')}</span>
        </button>
        </form>
      </div>

      <section className="split">
        <GroupPanel groups={groups} setGroups={setGroups} setToast={setToast} />
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
          <div className="review-list">
            {installedApps.length === 0 ? (
              <EmptyState icon={Download} title={t('profile.installedEmpty')} />
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
          <button type="button" className="secondary-button" onClick={() => void loadInstalledApps()}>
            <RefreshCw size={18} />
            <span>{t('profile.readInstalled')}</span>
          </button>
        </section>
      </section>

      {(user.role === 'SITE_ADMIN' || user.role === 'SOFTWARE_ADMIN') && <AdminPanel user={user} setToast={setToast} />}
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

function AdminPanel({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [apps, setApps] = useState<StoreApp[]>([]);
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
  const isSiteAdmin = user.role === 'SITE_ADMIN';
  const collectionKindOptions = [
    { value: 'MANUAL', label: t('admin.collectionKinds.manual') },
    { value: 'RECENT_UPDATED', label: t('admin.collectionKinds.recentUpdated') },
    { value: 'MOST_DOWNLOADED', label: t('admin.collectionKinds.mostDownloaded') },
  ];

  useEffect(() => {
    void reload();
  }, []);

  async function reload() {
    await runAction(setToast, t('admin.loadFailed'), async () => {
      const [categoryData, tagData, collectionData, appData] = await Promise.all([
        api<{ categories: Category[] }>('/api/v1/admin/categories'),
        api<{ tags: TagRecord[] }>('/api/v1/admin/tags'),
        api<{ collections: Collection[] }>('/api/v1/admin/collections'),
        api<{ apps: StoreApp[] }>('/api/v1/apps?status=APPROVED'),
      ]);
      setAdminCategories(categoryData.categories);
      setAdminTags(tagData.tags);
      setAdminCollections(collectionData.collections);
      setApps(appData.apps);
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
    await runAction(setToast, t('admin.categoryDeleteFailed'), async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.categoryDeleted') });
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
    await runAction(setToast, t('admin.tagDeleteFailed'), async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.tagDeleted') });
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
    await runAction(setToast, t('admin.collectionDeleteFailed'), async () => {
      await api(`/api/v1/admin/collections/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('admin.collectionDeleted') });
      await reload();
    });
  }

  return (
    <section className="page-grid">
      {isSiteAdmin && (
        <section className="split">
          <form className="panel form-panel" onSubmit={saveSettings}>
            <SectionTitle icon={Settings} title={t('admin.siteSettings')} />
            {['max_lpk_size', 'max_versions', 'source_password', 'source_password_rotation', 'github_mirror', 'require_email_verify'].map((key) => (
              <label key={key}>
                <span>{key}</span>
                <input value={settings[key] || ''} onChange={(event) => setSettings({ ...settings, [key]: event.target.value })} />
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
            {apps.map((item) => (
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
  const [versionFile, setVersionFile] = useState<File | null>(null);
  const [collaboratorRequests, setCollaboratorRequests] = useState<CollaboratorRequest[]>([]);
  const [appForm, setAppForm] = useState({
    name: app.name,
    summary: app.summary,
    description: app.description,
    categoryId: app.categoryId ? String(app.categoryId) : '',
    tags: (app.tags || []).join(', '),
    allowUnreviewedUpdates: app.allowUnreviewedUpdates,
    commentsEnabled: app.commentsEnabled,
  });
  const [visibility, setVisibility] = useState<number[]>(app.visibleGroupIds || []);
  const canMaintain = !!user && (app.canManageApp ?? (user.role === 'SITE_ADMIN' || user.role === 'SOFTWARE_ADMIN' || user.id === app.ownerId));
  const canUploadVersion = !!user && (app.canUploadVersion || canMaintain);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const drawerTitleId = `app-drawer-title-${app.id}`;

  useEffect(() => {
    setAppForm({
      name: app.name,
      summary: app.summary,
      description: app.description,
      categoryId: app.categoryId ? String(app.categoryId) : '',
      tags: (app.tags || []).join(', '),
      allowUnreviewedUpdates: app.allowUnreviewedUpdates,
      commentsEnabled: app.commentsEnabled,
    });
    setVisibility(app.visibleGroupIds || []);
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
        }),
      });
      setToast({ tone: 'success', message: data.review ? t('drawer.appInfoSubmittedReview') : t('drawer.appInfoSaved') });
      await onRefresh();
    });
  }

  async function submitExternalVersion(event: FormEvent) {
    event.preventDefault();
    if (!versionFile && !versionForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: t('submitApp.selectFileOrUrl') });
      return;
    }
    if (!versionFile && !versionForm.sha256.trim()) {
      setToast({ tone: 'error', message: t('submitApp.sha256Required') });
      return;
    }
    await runAction(setToast, t('drawer.versionSubmitFailed'), async () => {
      if (versionFile) {
        const form = new FormData();
        form.set('file', versionFile);
        form.set('version', versionForm.version);
        form.set('changelog', versionForm.changelog);
        await api(`/api/v1/apps/${app.id}/versions`, { method: 'POST', body: form });
      } else {
        await api(`/api/v1/apps/${app.id}/versions`, {
          method: 'POST',
          body: JSON.stringify(versionForm),
        });
      }
      setVersionForm({ version: '', sourceType: 'GITHUB', downloadUrl: '', sha256: '', changelog: '' });
      setVersionFile(null);
      setToast({ tone: 'success', message: t('drawer.versionSubmitted') });
      await onRefresh();
    });
  }

  async function unlistApp() {
    await runAction(setToast, t('drawer.unlistFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/unlist`, { method: 'POST' });
      setToast({ tone: 'neutral', message: t('drawer.unlisted') });
      await onRefresh();
    });
  }

  async function deleteApp() {
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
    await runAction(setToast, t('drawer.screenshotDeleteFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/${screenshotID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: t('drawer.screenshotDeleted') });
      await onRefresh();
    });
  }

  async function deleteComment(commentID: number) {
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
          <button type="button" className="primary-button" onClick={() => onInstall(app)}>
            <Download size={18} />
            <span>{t('common.install')}</span>
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
                <label>
                  <span>{t('common.version')}</span>
                  <input value={versionForm.version} onChange={(event) => setVersionForm({ ...versionForm, version: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.source')}</span>
                  <select value={versionForm.sourceType} onChange={(event) => setVersionForm({ ...versionForm, sourceType: event.target.value })}>
                    <option value="GITHUB">GitHub Release</option>
                    <option value="WEBDAV">WebDAV URL</option>
                    <option value="S3">S3 URL</option>
                  </select>
                </label>
                <label>
                  <span>{t('common.downloadUrl')}</span>
                  <input value={versionForm.downloadUrl} disabled={!!versionFile} onChange={(event) => setVersionForm({ ...versionForm, downloadUrl: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.sha256')}</span>
                  <input value={versionForm.sha256} disabled={!!versionFile} onChange={(event) => setVersionForm({ ...versionForm, sha256: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.changelog')}</span>
                  <input value={versionForm.changelog} onChange={(event) => setVersionForm({ ...versionForm, changelog: event.target.value })} />
                </label>
                <label>
                  <span>{t('common.lpkFile')}</span>
                  <input type="file" accept=".lpk" onChange={(event) => setVersionFile(event.target.files?.[0] || null)} />
                </label>
                <button type="submit" className="secondary-button">
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
          <div className="version-list">
            {(app.versions || []).map((version) => (
              <div className="version-row" key={version.id}>
                <div>
                  <strong>{version.version}</strong>
                  <span>{version.sourceType} · {formatBytes(version.fileSize)} · {formatDate(version.createdAt)}</span>
                </div>
                <code>{version.sha256 ? version.sha256.slice(0, 16) : '-'}</code>
              </div>
            ))}
          </div>
        </section>
        <section>
          <h3>{t('drawer.comments')}</h3>
          {user && (
            <form className="comment-form" onSubmit={submitComment}>
              <input value={commentText} onChange={(event) => setCommentText(event.target.value)} placeholder={t('drawer.commentPlaceholder')} />
              <button type="submit" className="icon-button" aria-label={t('drawer.postComment')}><MessageSquare size={17} /></button>
            </form>
          )}
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

function AppGrid({ apps, onOpen, onInstall }: { apps: StoreApp[]; onOpen: (app: StoreApp) => void; onInstall: (app: StoreApp) => void }) {
  const { t } = useTranslation();
  if (apps.length === 0) return <EmptyState icon={PackagePlus} title={t('common.noApps')} />;
  return (
    <div className="app-grid">
      {apps.map((app) => (
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
            <span><Star size={14} /> {app.latestVersion?.version || app.status}</span>
          </div>
          <button type="button" className="install-button" onClick={() => void onInstall(app)} aria-label={t('app.install', { name: app.name })}>
            <Download size={17} />
            <span>{t('common.install')}</span>
          </button>
        </article>
      ))}
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

function EmptyState({ icon: Icon, title }: { icon: typeof Home; title: string }) {
  return (
    <div className="empty-state">
      <Icon size={28} />
      <strong>{title}</strong>
    </div>
  );
}

function MobileTabs({ tab, setTab, items, inert }: { tab: TabKey; setTab: (tab: TabKey) => void; items: readonly NavItem[]; inert?: boolean }) {
  const { t } = useTranslation();
  return (
    <nav className="mobile-tabs" inert={inert} aria-hidden={inert ? true : undefined}>
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
