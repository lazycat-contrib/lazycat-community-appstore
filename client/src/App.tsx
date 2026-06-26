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
import { FormEvent, useEffect, useMemo, useState } from 'react';
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

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  if (!HAS_API) {
    throw new Error('未配置服务端 API');
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
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}

const tabs = [
  { key: 'home', label: '首页', icon: Home },
  { key: 'categories', label: '分类', icon: Layers3 },
  { key: 'search', label: '搜索', icon: Search },
  { key: 'sources', label: '软件源', icon: Cloud },
  { key: 'profile', label: '我的', icon: UserRound },
] as const;

type TabKey = (typeof tabs)[number]['key'];
type SortMode = 'recent' | 'downloads' | 'name';

function verificationTokenFromURL() {
  return new URLSearchParams(window.location.search).get('token') || '';
}

export function App() {
  const [tab, setTab] = useState<TabKey>(() => (verificationTokenFromURL() ? 'profile' : 'home'));
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
    await runAction(setToast, '审核列表加载失败', async () => {
      const data = await api<{ reviews: Review[] }>('/api/v1/admin/reviews?status=PENDING');
      setReviews(data.reviews);
    });
  }

  async function loadGroups() {
    await runAction(setToast, '群组加载失败', async () => {
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
    await runAction(setToast, '应用详情加载失败', async () => {
      const data = await api<{ app: StoreApp }>(`/api/v1/apps/${app.id}`);
      setSelectedApp(data.app);
    });
  }

  async function installApp(app: StoreApp | SourceApp) {
    await runAction(setToast, '安装失败', async () => {
      const version = app.latestVersion;
      if (!version) {
        setToast({ tone: 'error', message: '没有可安装版本' });
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
        setToast({ tone: result.mode === 'lazycat-sdk' || result.mode === 'download' ? 'success' : 'error', message: result.message });
      } finally {
        window.setTimeout(() => setInstalling(null), 700);
      }
    });
  }

  async function approveReview(review: Review, approve: boolean) {
    await runAction(setToast, '审核操作失败', async () => {
      await api(`/api/v1/admin/reviews/${review.id}/${approve ? 'approve' : 'reject'}`, {
        method: 'POST',
        body: JSON.stringify({ note: approve ? 'Approved from client' : 'Rejected from client' }),
      });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? '已通过审核' : '已拒绝审核' });
      await refreshAll();
    });
  }

  async function syncSource(source: SourceSubscription) {
    const url = new URL(source.url);
    if (source.password) url.searchParams.set('password', source.password);
    const response = await fetch(url.toString(), { headers: source.password ? { 'X-Source-Password': source.password } : undefined });
    if (response.status === 401) throw new Error('软件源密码无效，请更新访问密码');
    if (!response.ok) throw new Error('软件源同步失败');
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
    setToast({ tone: 'success', message: '软件源已同步' });
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Archive size={22} />
          </div>
          <div>
            <strong>懒猫软件商店</strong>
            <span>LPK distribution</span>
          </div>
        </div>
        <nav className="nav">
          {tabs.map((item) => {
            const Icon = item.icon;
            return (
              <button key={item.key} className={cx('nav-item', tab === item.key && 'active')} onClick={() => setTab(item.key)}>
                <Icon size={19} />
                <span>{item.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="server-card">
          <Server size={18} />
          <div>
            <span>Server</span>
            <strong>{HAS_API ? API_BASE.replace(/^https?:\/\//, '') : '未配置'}</strong>
          </div>
        </div>
      </aside>

      <main className="main">
        <header className="topbar">
          <div className="searchbox">
            <Search size={18} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索应用、标签、提交者" />
          </div>
          <div className="top-actions">
            <button className="icon-button" aria-label="刷新" onClick={() => void refreshAll()}>
              <RefreshCw size={18} />
            </button>
            {user ? (
              <button
                className="user-pill"
                onClick={() =>
                  void runAction(setToast, '退出失败', async () => {
                    await api('/api/v1/auth/logout', { method: 'POST' });
                    setUser(null);
                  })
                }
              >
                <LogOut size={16} />
                <span>{user.username}</span>
              </button>
            ) : (
              <button className="user-pill" onClick={() => setTab('profile')}>
                <LogIn size={16} />
                <span>登录</span>
              </button>
            )}
          </div>
        </header>

        {loading ? (
          <div className="loading-state">
            <Gauge size={28} />
            <span>加载中</span>
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
                onSubmitter={setActiveSubmitter}
                onSortMode={setSortMode}
                onOpen={openApp}
                onInstall={installApp}
              />
            )}
            {tab === 'sources' && <SourcesView sources={sources} setSources={setSources} sourceApps={sourceApps} onSync={syncSource} onInstall={installApp} setToast={setToast} />}
            {tab === 'profile' && <ProfileView user={user} setUser={setUser} groups={groups} setGroups={setGroups} categories={categories} refreshAll={refreshAll} setToast={setToast} hasAPI={HAS_API} />}
          </>
        )}
      </main>

      <MobileTabs tab={tab} setTab={setTab} />

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
        <div className="install-panel">
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
}: {
  apps: StoreApp[];
  collections: Collection[];
  reviews: Review[];
  canReview: boolean;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp) => void;
  onApprove: (review: Review, approve: boolean) => void;
}) {
  const latest = [...apps].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, 6);
  return (
    <section className="page-grid">
      <div className="hero-band">
        <div>
          <span className="eyebrow">Community LPK</span>
          <h1>从审核到安装的一条发布线</h1>
          <p>已上架 {apps.filter((app) => app.status === 'APPROVED').length} 个应用，待处理 {reviews.length} 个审核。</p>
        </div>
        <div className="hero-stack">
          <div><PackagePlus size={18} /> 上传</div>
          <div><ShieldCheck size={18} /> 审核</div>
          <div><Download size={18} /> 安装</div>
        </div>
      </div>

      {canReview && reviews.length > 0 && (
        <section className="panel">
          <SectionTitle icon={ShieldCheck} title="待审核" />
          <div className="review-list">
            {reviews.slice(0, 4).map((review) => (
              <div className="review-row" key={review.id}>
                <div>
                  <strong>{review.kind.replaceAll('_', ' ')}</strong>
                  <span>#{review.appId || review.versionId} · {formatDate(review.createdAt)}</span>
                </div>
                <div className="row-actions">
                  <button className="icon-button ok" aria-label="通过" onClick={() => void onApprove(review, true)}>
                    <Check size={17} />
                  </button>
                  <button className="icon-button danger" aria-label="拒绝" onClick={() => void onApprove(review, false)}>
                    <X size={17} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="panel">
        <SectionTitle icon={History} title="最近更新" />
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
  return (
    <section className="page-grid">
      <div className="segmented">
        <button className={cx(activeCategory === 'all' && 'active')} onClick={() => onCategory('all')}>全部</button>
        {categories.map((category) => (
          <button key={category.id} className={cx(activeCategory === category.name && 'active')} onClick={() => onCategory(category.name)}>
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
  onSubmitter,
  onSortMode,
  onOpen,
  onInstall,
}: {
  apps: StoreApp[];
  sourceApps: SourceApp[];
  submitters: string[];
  activeSubmitter: string;
  sortMode: SortMode;
  onSubmitter: (submitter: string) => void;
  onSortMode: (mode: SortMode) => void;
  onOpen: (app: StoreApp) => void;
  onInstall: (app: StoreApp | SourceApp) => void;
}) {
  return (
    <section className="page-grid">
      <section className="panel">
        <SectionTitle icon={Search} title="本地商店" />
        <div className="filter-bar">
          <label>
            <span>排序</span>
            <select value={sortMode} onChange={(event) => onSortMode(event.target.value as SortMode)}>
              <option value="recent">最近更新</option>
              <option value="downloads">下载最多</option>
              <option value="name">名称</option>
            </select>
          </label>
          <label>
            <span>提交者</span>
            <select value={activeSubmitter} onChange={(event) => onSubmitter(event.target.value)}>
              <option value="all">全部提交者</option>
              {submitters.map((submitter) => (
                <option key={submitter} value={submitter}>{submitter}</option>
              ))}
            </select>
          </label>
        </div>
        <AppGrid apps={apps} onOpen={onOpen} onInstall={onInstall} />
      </section>
      <section className="panel">
        <SectionTitle icon={Cloud} title="订阅来源" />
        <div className="source-apps">
          {sourceApps.length === 0 ? (
            <EmptyState icon={Cloud} title="没有同步的软件源应用" />
          ) : (
            sourceApps.map((app) => (
              <button className="source-app-row" key={`${app.sourceName}-${app.id}`} onClick={() => void onInstall(app)}>
                <div>
                  <strong>{app.name}</strong>
                  <span>{app.sourceName} · {app.latestVersion?.version || '-'}</span>
                </div>
                <Download size={18} />
              </button>
            ))
          )}
        </div>
      </section>
    </section>
  );
}

function SourcesView({
  sources,
  setSources,
  sourceApps,
  onSync,
  onInstall,
  setToast,
}: {
  sources: SourceSubscription[];
  setSources: (update: SourceSubscription[] | ((current: SourceSubscription[]) => SourceSubscription[])) => void;
  sourceApps: SourceApp[];
  onSync: (source: SourceSubscription) => Promise<void>;
  onInstall: (app: SourceApp) => void;
  setToast: (toast: Toast) => void;
}) {
  const emptyDraft = { name: '', url: DEFAULT_SOURCE_URL, password: '', mirror: '' };
  const [draft, setDraft] = useState(emptyDraft);

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
    <section className="split">
      <form className="panel form-panel" onSubmit={addSource}>
        <SectionTitle icon={Cloud} title="软件源" />
        <label>
          <span>名称</span>
          <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        </label>
        <label>
          <span>URL</span>
          <input value={draft.url} onChange={(event) => setDraft({ ...draft, url: event.target.value })} />
        </label>
        <label>
          <span>访问密码</span>
          <input type="password" value={draft.password} onChange={(event) => setDraft({ ...draft, password: event.target.value })} />
        </label>
        <label>
          <span>GitHub 镜像</span>
          <input value={draft.mirror} onChange={(event) => setDraft({ ...draft, mirror: event.target.value })} />
        </label>
        <button className="primary-button">
          <Cloud size={18} />
          <span>添加软件源</span>
        </button>
      </form>

      <section className="panel">
        <SectionTitle icon={Server} title="订阅" />
        <div className="source-list">
          {sources.length === 0 ? (
            <EmptyState icon={Cloud} title="还没有软件源" />
          ) : (
            sources.map((source) => (
              <div className="source-row" key={source.id}>
                <div>
                  <strong>{source.name}</strong>
                  <span>{source.url}</span>
                  {source.lastSync && <small>{formatDate(source.lastSync)}</small>}
                  <div className="source-edit-grid">
                    <input value={source.password} type="password" placeholder="访问密码" onChange={(event) => updateSource(source.id, { password: event.target.value })} />
                    <input value={source.mirror} placeholder="GitHub 镜像" onChange={(event) => updateSource(source.id, { mirror: event.target.value })} />
                  </div>
                </div>
                <div className="row-actions">
                  <button
                    className="icon-button"
                    aria-label="同步"
                    onClick={() =>
                      void onSync(source).catch((error) =>
                        setToast({ tone: 'error', message: error instanceof Error ? error.message : '软件源同步失败' }),
                      )
                    }
                  >
                    <RefreshCw size={17} />
                  </button>
                  <button className="icon-button danger" aria-label="删除" onClick={() => setSources((current) => current.filter((item) => item.id !== source.id))}>
                    <X size={17} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
        {sourceApps.length > 0 && (
          <div className="source-apps compact">
            {sourceApps.map((app) => (
              <button className="source-app-row" key={`${app.sourceName}-${app.id}`} onClick={() => onInstall(app)}>
                <div>
                  <strong>{app.name}</strong>
                  <span>{app.sourceName} · {app.latestVersion?.version}</span>
                </div>
                <Download size={18} />
              </button>
            ))}
          </div>
        )}
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
    await runAction(setToast, '邮箱验证失败', async () => {
      const data = await api<{ user: User }>('/api/v1/auth/verify-email', {
        method: 'POST',
        body: JSON.stringify({ token: verifyToken }),
      });
      setUser(data.user);
      setToast({ tone: 'success', message: '邮箱已验证' });
      await refreshAll();
    });
  }

  async function submitAuth(event: FormEvent) {
    event.preventDefault();
    if (mode === 'verify') {
      await submitVerification(event);
      return;
    }
    await runAction(setToast, mode === 'login' ? '登录失败' : '注册失败', async () => {
      const data = await api<{ user: User }>(`/api/v1/auth/${mode}`, {
        method: 'POST',
        body: JSON.stringify(authForm),
      });
      setUser(data.user);
      if (data.user.emailVerified === false) {
        setMode('verify');
        setToast({ tone: 'neutral', message: '请完成邮箱验证后继续' });
      } else {
        setToast({ tone: 'success', message: mode === 'login' ? '已登录' : '已注册' });
      }
      await refreshAll();
    });
  }

  async function submitUpload(event: FormEvent) {
    event.preventDefault();
    if (!file && !uploadForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: '请选择 LPK 文件或填写外部下载链接' });
      return;
    }
    if (!file && !uploadForm.sha256.trim()) {
      setToast({ tone: 'error', message: '外部下载链接需要填写 SHA256' });
      return;
    }
    await runAction(setToast, '应用提交失败', async () => {
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
      setToast({ tone: 'success', message: '应用已提交' });
      setUploadForm({ name: '', version: '0.1.0', summary: '', description: '', categoryId: '', tags: '', allowUnreviewedUpdates: false, sourceType: 'GITHUB', downloadUrl: '', sha256: '' });
      setFile(null);
      await refreshAll();
    });
  }

  async function createToken() {
    await runAction(setToast, 'Token 生成失败', async () => {
      const data = await api<{ token: string; record: APITokenRecord }>('/api/v1/me/tokens', {
        method: 'POST',
        body: JSON.stringify({ name: 'CI publish token' }),
      });
      setTokens((current) => [data.record, ...current]);
      setNewToken(data.token);
    });
  }

  async function loadFavorites() {
    await runAction(setToast, '收藏加载失败', async () => {
      const data = await api<FavoriteData>('/api/v1/me/favorites');
      setFavorites({ apps: data.apps || [], submitters: data.submitters || [] });
    });
  }

  async function loadInstalledApps() {
    try {
      const result = await queryInstalledApplications();
      setInstalledApps(result?.infoList || []);
      setToast({ tone: 'success', message: '已刷新安装列表' });
    } catch {
      setToast({ tone: 'error', message: '当前环境无法访问 LazyCat SDK' });
    }
  }

  if (!hasAPI) {
    return (
      <section className="page-grid">
        <div className="panel profile-card">
          <div className="avatar"><Cloud size={28} /></div>
          <h2>客户端模式</h2>
          <p>当前未配置服务端 API，可在软件源页面添加任意 LazyCat App Store source。</p>
        </div>
      </section>
    );
  }

  if (!user) {
    return (
      <form className="panel form-panel profile-panel" onSubmit={submitAuth}>
        <SectionTitle icon={KeyRound} title={mode === 'login' ? '登录' : '注册'} />
        <div className="segmented compact">
          <button type="button" className={cx(mode === 'login' && 'active')} onClick={() => setMode('login')}>登录</button>
          <button type="button" className={cx(mode === 'register' && 'active')} onClick={() => setMode('register')}>注册</button>
          <button type="button" className={cx(mode === 'verify' && 'active')} onClick={() => setMode('verify')}>验证</button>
        </div>
        {mode === 'verify' ? (
          <label>
            <span>验证 Token</span>
            <input value={verifyToken} onChange={(event) => setVerifyToken(event.target.value)} />
          </label>
        ) : (
          <>
            <label>
              <span>用户名</span>
              <input value={authForm.username} onChange={(event) => setAuthForm({ ...authForm, username: event.target.value })} />
            </label>
            {mode === 'register' && (
              <label>
                <span>邮箱</span>
                <input type="email" value={authForm.email} onChange={(event) => setAuthForm({ ...authForm, email: event.target.value })} />
              </label>
            )}
            <label>
              <span>密码</span>
              <input type="password" value={authForm.password} onChange={(event) => setAuthForm({ ...authForm, password: event.target.value })} />
            </label>
          </>
        )}
        <button className="primary-button">
          <LogIn size={18} />
          <span>{mode === 'login' ? '登录' : mode === 'register' ? '注册' : '验证邮箱'}</span>
        </button>
      </form>
    );
  }

  if (user.emailVerified === false) {
    return (
      <section className="page-grid">
        <div className="split">
          <div className="panel profile-card">
            <div className="avatar"><UserRound size={28} /></div>
            <h2>{user.username}</h2>
            <p>邮箱待验证</p>
            <button
              className="secondary-button"
              onClick={() =>
                void runAction(setToast, '退出失败', async () => {
                  await api('/api/v1/auth/logout', { method: 'POST' });
                  setUser(null);
                })
              }
            >
              <LogOut size={18} />
              <span>退出</span>
            </button>
          </div>
          <form className="panel form-panel" onSubmit={submitVerification}>
            <SectionTitle icon={AlertCircle} title="验证邮箱" />
            <p className="inline-note">管理员开启邮箱验证后，需要完成验证才能提交应用、生成 Token 或管理群组。</p>
            <label>
              <span>验证 Token</span>
              <input value={verifyToken} onChange={(event) => setVerifyToken(event.target.value)} />
            </label>
            <button className="primary-button">
              <Check size={18} />
              <span>完成验证</span>
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
        <div className="avatar"><UserRound size={28} /></div>
        <h2>{user.username}</h2>
        <p>{user.role}</p>
        <button
          className="secondary-button"
          onClick={() =>
            void runAction(setToast, '退出失败', async () => {
              await api('/api/v1/auth/logout', { method: 'POST' });
              setUser(null);
            })
          }
        >
          <LogOut size={18} />
          <span>退出</span>
        </button>
        </div>

        <form className="panel form-panel" onSubmit={submitUpload}>
        <SectionTitle icon={Upload} title="提交应用" />
        <label>
          <span>应用名称</span>
          <input value={uploadForm.name} onChange={(event) => setUploadForm({ ...uploadForm, name: event.target.value })} />
        </label>
        <label>
          <span>版本</span>
          <input value={uploadForm.version} onChange={(event) => setUploadForm({ ...uploadForm, version: event.target.value })} />
        </label>
        <label>
          <span>摘要</span>
          <input value={uploadForm.summary} onChange={(event) => setUploadForm({ ...uploadForm, summary: event.target.value })} />
        </label>
        <label>
          <span>描述</span>
          <textarea value={uploadForm.description} onChange={(event) => setUploadForm({ ...uploadForm, description: event.target.value })} />
        </label>
        <label>
          <span>分类</span>
          <select value={uploadForm.categoryId} onChange={(event) => setUploadForm({ ...uploadForm, categoryId: event.target.value })}>
            <option value="">未分类</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>{category.name}</option>
            ))}
          </select>
        </label>
        <label>
          <span>标签</span>
          <input value={uploadForm.tags} onChange={(event) => setUploadForm({ ...uploadForm, tags: event.target.value })} />
        </label>
        <label>
          <span>外部来源</span>
          <select value={uploadForm.sourceType} onChange={(event) => setUploadForm({ ...uploadForm, sourceType: event.target.value })}>
            <option value="GITHUB">GitHub Release</option>
            <option value="WEBDAV">WebDAV URL</option>
            <option value="S3">S3 URL</option>
          </select>
        </label>
        <label>
          <span>外部下载链接</span>
          <input value={uploadForm.downloadUrl} onChange={(event) => setUploadForm({ ...uploadForm, downloadUrl: event.target.value })} />
        </label>
        <label>
          <span>外部文件 SHA256</span>
          <input value={uploadForm.sha256} onChange={(event) => setUploadForm({ ...uploadForm, sha256: event.target.value })} />
        </label>
        <label className="toggle-line">
          <input
            type="checkbox"
            checked={uploadForm.allowUnreviewedUpdates}
            onChange={(event) => setUploadForm({ ...uploadForm, allowUnreviewedUpdates: event.target.checked })}
          />
          <span>免审批更新</span>
        </label>
        <label>
          <span>LPK 文件</span>
          <input type="file" accept=".lpk" onChange={(event) => setFile(event.target.files?.[0] || null)} />
        </label>
        <button className="primary-button">
          <Upload size={18} />
          <span>提交</span>
        </button>
        </form>
      </div>

      <section className="split">
        <GroupPanel groups={groups} setGroups={setGroups} setToast={setToast} />
        <section className="panel">
          <SectionTitle icon={KeyRound} title="API Token" />
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
          <button className="secondary-button" onClick={() => void createToken()}>
            <KeyRound size={18} />
            <span>生成 Token</span>
          </button>
        </section>
      </section>

      <section className="split">
        <section className="panel">
          <SectionTitle icon={Heart} title="收藏" />
          <div className="review-list">
            {favorites.apps.length === 0 && favorites.submitters.length === 0 ? (
              <EmptyState icon={Heart} title="还没有收藏" />
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
                      <span>{item.email || '提交者'}</span>
                    </div>
                  </div>
                ))}
              </>
            )}
          </div>
          <button className="secondary-button" onClick={() => void loadFavorites()}>
            <RefreshCw size={18} />
            <span>刷新收藏</span>
          </button>
        </section>
        <section className="panel">
          <SectionTitle icon={Download} title="已安装" />
          <div className="review-list">
            {installedApps.length === 0 ? (
              <EmptyState icon={Download} title="未读取安装列表" />
            ) : (
              installedApps.map((item) => (
                <div className="review-row" key={item.appid || item.title}>
                  <div>
                    <strong>{item.title || item.appid}</strong>
                    <span>{item.version || '-'} · status {item.status ?? '-'}</span>
                  </div>
                </div>
              ))
            )}
          </div>
          <button className="secondary-button" onClick={() => void loadInstalledApps()}>
            <RefreshCw size={18} />
            <span>读取已安装</span>
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
  const [draft, setDraft] = useState({ name: '', description: '' });
  const [memberDrafts, setMemberDrafts] = useState<Record<number, string>>({});

  async function reload() {
    await runAction(setToast, '群组加载失败', async () => {
      const data = await api<{ groups: Group[] }>('/api/v1/groups');
      setGroups(data.groups);
    });
  }

  async function createGroup(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '群组创建失败', async () => {
      await api('/api/v1/groups', { method: 'POST', body: JSON.stringify(draft) });
      setDraft({ name: '', description: '' });
      setToast({ tone: 'success', message: '用户群组已创建' });
      await reload();
    });
  }

  async function addMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, '添加成员失败', async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'POST' });
      setToast({ tone: 'success', message: '成员已添加' });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  async function removeMember(groupID: number) {
    const userID = memberDrafts[groupID];
    if (!userID) return;
    await runAction(setToast, '移除成员失败', async () => {
      await api(`/api/v1/groups/${groupID}/members/${userID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '成员已移除' });
      setMemberDrafts((current) => ({ ...current, [groupID]: '' }));
    });
  }

  return (
    <section className="panel form-panel">
      <SectionTitle icon={Users} title="用户群组" />
      <form className="inline-form" onSubmit={createGroup}>
        <input placeholder="群组名称" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        <button className="icon-button" aria-label="创建"><Plus size={17} /></button>
      </form>
      <div className="review-list">
        {groups.length === 0 ? <EmptyState icon={Users} title="没有群组" /> : groups.map((group) => (
          <div className="review-row" key={group.id}>
            <div>
              <strong>{group.name}</strong>
              <span>{group.slug}</span>
            </div>
            <div className="inline-form compact-line group-member-actions">
              <input
                placeholder="用户 ID"
                value={memberDrafts[group.id] || ''}
                onChange={(event) => setMemberDrafts((current) => ({ ...current, [group.id]: event.target.value }))}
              />
              <button className="icon-button" aria-label="添加成员" onClick={() => void addMember(group.id)}><Plus size={17} /></button>
              <button className="icon-button danger" aria-label="移除成员" onClick={() => void removeMember(group.id)}><Trash2 size={17} /></button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function AdminPanel({ user, setToast }: { user: User; setToast: (toast: Toast) => void }) {
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

  useEffect(() => {
    void reload();
  }, []);

  async function reload() {
    await runAction(setToast, '管理员数据加载失败', async () => {
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
    await runAction(setToast, '用户角色更新失败', async () => {
      await api(`/api/v1/admin/users/${userID}`, { method: 'PATCH', body: JSON.stringify({ role }) });
      setToast({ tone: 'success', message: '用户角色已更新' });
      await reload();
    });
  }

  async function saveSettings(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '站点配置保存失败', async () => {
      await api('/api/v1/admin/settings', { method: 'PATCH', body: JSON.stringify(settings) });
      setToast({ tone: 'success', message: '站点配置已保存' });
      await reload();
    });
  }

  async function createCategory(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '分类创建失败', async () => {
      await api('/api/v1/admin/categories', { method: 'POST', body: JSON.stringify(categoryForm) });
      setCategoryForm({ name: '', slug: '' });
      setToast({ tone: 'success', message: '分类已创建' });
      await reload();
    });
  }

  async function updateCategory(item: Category) {
    const draft = categoryDrafts[item.id] || { name: item.name, slug: item.slug };
    await runAction(setToast, '分类更新失败', async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: '分类已更新' });
      await reload();
    });
  }

  async function deleteCategory(item: Category) {
    await runAction(setToast, '分类删除失败', async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '分类已删除' });
      await reload();
    });
  }

  async function createTag(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '标签创建失败', async () => {
      await api('/api/v1/admin/tags', { method: 'POST', body: JSON.stringify(tagForm) });
      setTagForm({ name: '', slug: '' });
      setToast({ tone: 'success', message: '标签已创建' });
      await reload();
    });
  }

  async function updateTag(item: TagRecord) {
    const draft = tagDrafts[item.id] || { name: item.name, slug: item.slug };
    await runAction(setToast, '标签更新失败', async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'PATCH', body: JSON.stringify(draft) });
      setToast({ tone: 'success', message: '标签已更新' });
      await reload();
    });
  }

  async function deleteTag(item: TagRecord) {
    await runAction(setToast, '标签删除失败', async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '标签已删除' });
      await reload();
    });
  }

  async function createCollection(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '聚合分类创建失败', async () => {
      await api('/api/v1/admin/collections', {
        method: 'POST',
        body: JSON.stringify({
          name: collectionForm.name,
          kind: collectionForm.kind,
          appIds: collectionForm.appIds.split(',').map((id) => Number(id.trim())).filter(Boolean),
        }),
      });
      setCollectionForm({ name: '', kind: 'MANUAL', appIds: '' });
      setToast({ tone: 'success', message: '聚合分类已创建' });
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
    await runAction(setToast, '聚合分类更新失败', async () => {
      await api(`/api/v1/admin/collections/${item.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          name: draft.name,
          slug: draft.slug,
          kind: draft.kind,
          appIds: draft.appIds.split(',').map((id) => Number(id.trim())).filter(Boolean),
        }),
      });
      setToast({ tone: 'success', message: '聚合分类已更新' });
      await reload();
    });
  }

  async function deleteCollection(item: Collection) {
    await runAction(setToast, '聚合分类删除失败', async () => {
      await api(`/api/v1/admin/collections/${item.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '聚合分类已删除' });
      await reload();
    });
  }

  return (
    <section className="page-grid">
      {isSiteAdmin && (
        <section className="split">
          <form className="panel form-panel" onSubmit={saveSettings}>
            <SectionTitle icon={Settings} title="站点配置" />
            {['max_lpk_size', 'max_versions', 'source_password', 'source_password_rotation', 'github_mirror', 'require_email_verify'].map((key) => (
              <label key={key}>
                <span>{key}</span>
                <input value={settings[key] || ''} onChange={(event) => setSettings({ ...settings, [key]: event.target.value })} />
              </label>
            ))}
            <button className="primary-button">
              <Settings size={18} />
              <span>保存配置</span>
            </button>
          </form>
          <section className="panel">
            <SectionTitle icon={Users} title="用户管理" />
            <div className="review-list">
              {users.map((item) => (
                <div className="review-row" key={item.id}>
                  <div>
                    <strong>#{item.id} {item.username}</strong>
                    <span>{item.email || 'no email'}</span>
                  </div>
                  <select value={item.role} onChange={(event) => void updateUserRole(item.id, event.target.value as User['role'])}>
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
          <SectionTitle icon={Layers3} title="分类与标签" />
          <form className="inline-stack" onSubmit={createCategory}>
            <input placeholder="分类名称" value={categoryForm.name} onChange={(event) => setCategoryForm({ ...categoryForm, name: event.target.value })} />
            <input placeholder="slug" value={categoryForm.slug} onChange={(event) => setCategoryForm({ ...categoryForm, slug: event.target.value })} />
            <button className="secondary-button"><Plus size={17} /><span>分类</span></button>
          </form>
          <form className="inline-stack" onSubmit={createTag}>
            <input placeholder="标签名称" value={tagForm.name} onChange={(event) => setTagForm({ ...tagForm, name: event.target.value })} />
            <input placeholder="slug" value={tagForm.slug} onChange={(event) => setTagForm({ ...tagForm, slug: event.target.value })} />
            <button className="secondary-button"><Plus size={17} /><span>标签</span></button>
          </form>
        </div>
        <section className="panel">
          <SectionTitle icon={Tag} title="分类列表" />
          <div className="review-list">
            {adminCategories.map((item) => {
              const draft = categoryDrafts[item.id] || { name: item.name, slug: item.slug };
              return (
                <div className="edit-row" key={item.id}>
                  <input value={draft.name} onChange={(event) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input value={draft.slug} onChange={(event) => setCategoryDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <div className="row-actions">
                    <button className="icon-button" aria-label="保存分类" onClick={() => void updateCategory(item)}><Save size={16} /></button>
                    <button className="icon-button danger" aria-label="删除分类" onClick={() => void deleteCategory(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
          <SectionTitle icon={Tag} title="标签列表" />
          <div className="review-list">
            {adminTags.map((item) => {
              const draft = tagDrafts[item.id] || { name: item.name, slug: item.slug };
              return (
                <div className="edit-row" key={item.id}>
                  <input value={draft.name} onChange={(event) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input value={draft.slug} onChange={(event) => setTagDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <div className="row-actions">
                    <button className="icon-button" aria-label="保存标签" onClick={() => void updateTag(item)}><Save size={16} /></button>
                    <button className="icon-button danger" aria-label="删除标签" onClick={() => void deleteTag(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      <section className="split">
        <form className="panel form-panel" onSubmit={createCollection}>
          <SectionTitle icon={Layers3} title="聚合分类" />
          <label>
            <span>名称</span>
            <input value={collectionForm.name} onChange={(event) => setCollectionForm({ ...collectionForm, name: event.target.value })} />
          </label>
          <label>
            <span>类型</span>
            <select value={collectionForm.kind} onChange={(event) => setCollectionForm({ ...collectionForm, kind: event.target.value })}>
              <option value="MANUAL">手动</option>
              <option value="RECENT_UPDATED">最近更新</option>
              <option value="MOST_DOWNLOADED">下载最多</option>
            </select>
          </label>
          <label>
            <span>应用 ID</span>
            <input value={collectionForm.appIds} onChange={(event) => setCollectionForm({ ...collectionForm, appIds: event.target.value })} />
          </label>
          <button className="primary-button">
            <Layers3 size={18} />
            <span>创建聚合</span>
          </button>
        </form>
        <section className="panel">
          <SectionTitle icon={Layers3} title="聚合列表" />
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
                  <input value={draft.name} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, name: event.target.value } }))} />
                  <input value={draft.slug} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, slug: event.target.value } }))} />
                  <select value={draft.kind} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, kind: event.target.value } }))}>
                    <option value="MANUAL">手动</option>
                    <option value="RECENT_UPDATED">最近更新</option>
                    <option value="MOST_DOWNLOADED">下载最多</option>
                  </select>
                  <input value={draft.appIds} onChange={(event) => setCollectionDrafts((current) => ({ ...current, [item.id]: { ...draft, appIds: event.target.value } }))} />
                  <div className="row-actions">
                    <button className="icon-button" aria-label="保存聚合" onClick={() => void updateCollection(item)}><Save size={16} /></button>
                    <button className="icon-button danger" aria-label="删除聚合" onClick={() => void deleteCollection(item)}><Trash2 size={16} /></button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      <section className="panel">
        <SectionTitle icon={PackagePlus} title="可选应用" />
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

  async function loadCollaboratorRequests() {
    await runAction(setToast, '协作者申请加载失败', async () => {
      const data = await api<{ requests: CollaboratorRequest[] }>(`/api/v1/apps/${app.id}/collaborator-requests`);
      setCollaboratorRequests(data.requests);
    });
  }

  async function submitComment(event: FormEvent) {
    event.preventDefault();
    if (!commentText.trim()) return;
    await runAction(setToast, '评论发布失败', async () => {
      await api(`/api/v1/apps/${app.id}/comments`, { method: 'POST', body: JSON.stringify({ body: commentText }) });
      setCommentText('');
      setToast({ tone: 'success', message: '评论已发布' });
      await onRefresh();
    });
  }

  async function markOutdated() {
    await runAction(setToast, '过期标记失败', async () => {
      await api(`/api/v1/apps/${app.id}/outdated-marks`, { method: 'POST', body: JSON.stringify({ note: '客户端标记' }) });
      setToast({ tone: 'neutral', message: '已标记过期' });
      await onRefresh();
    });
  }

  async function clearOutdated() {
    await runAction(setToast, '取消过期标记失败', async () => {
      await api(`/api/v1/apps/${app.id}/outdated-marks`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '已取消过期标记' });
      await onRefresh();
    });
  }

  async function submitAppInfo(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, '应用信息保存失败', async () => {
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
      setToast({ tone: 'success', message: data.review ? '应用信息已提交审核' : '应用信息已保存' });
      await onRefresh();
    });
  }

  async function submitExternalVersion(event: FormEvent) {
    event.preventDefault();
    if (!versionFile && !versionForm.downloadUrl.trim()) {
      setToast({ tone: 'error', message: '请选择 LPK 文件或填写外部下载链接' });
      return;
    }
    if (!versionFile && !versionForm.sha256.trim()) {
      setToast({ tone: 'error', message: '外部下载链接需要填写 SHA256' });
      return;
    }
    await runAction(setToast, '版本提交失败', async () => {
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
      setToast({ tone: 'success', message: '版本已提交' });
      await onRefresh();
    });
  }

  async function unlistApp() {
    await runAction(setToast, '应用下架失败', async () => {
      await api(`/api/v1/apps/${app.id}/unlist`, { method: 'POST' });
      setToast({ tone: 'neutral', message: '应用已下架' });
      await onRefresh();
    });
  }

  async function deleteApp() {
    await runAction(setToast, '应用删除失败', async () => {
      await api(`/api/v1/apps/${app.id}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '应用已删除' });
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
    await runAction(setToast, '截图上传失败', async () => {
      await api(`/api/v1/apps/${app.id}/screenshots`, { method: 'POST', body: form });
      setScreenshotFile(null);
      setScreenshotCaption('');
      setToast({ tone: 'success', message: '截图已上传' });
      await onRefresh();
    });
  }

  async function moveScreenshot(screenshotID: number, direction: -1 | 1) {
    const shots = [...(app.screenshots || [])].sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
    const index = shots.findIndex((shot) => shot.id === screenshotID);
    const nextIndex = index + direction;
    if (index < 0 || nextIndex < 0 || nextIndex >= shots.length) return;
    [shots[index], shots[nextIndex]] = [shots[nextIndex], shots[index]];
    await runAction(setToast, '截图排序失败', async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/reorder`, {
        method: 'PATCH',
        body: JSON.stringify({ items: shots.map((shot, sortOrder) => ({ id: shot.id, sortOrder })) }),
      });
      setToast({ tone: 'success', message: '截图顺序已更新' });
      await onRefresh();
    });
  }

  async function deleteScreenshot(screenshotID: number) {
    await runAction(setToast, '截图删除失败', async () => {
      await api(`/api/v1/apps/${app.id}/screenshots/${screenshotID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '截图已删除' });
      await onRefresh();
    });
  }

  async function deleteComment(commentID: number) {
    await runAction(setToast, '评论删除失败', async () => {
      await api(`/api/v1/comments/${commentID}`, { method: 'DELETE' });
      setToast({ tone: 'neutral', message: '评论已删除' });
      await onRefresh();
    });
  }

  async function saveVisibility() {
    await runAction(setToast, '可见性保存失败', async () => {
      await api(`/api/v1/apps/${app.id}/visibility`, {
        method: 'PATCH',
        body: JSON.stringify({ groupIds: visibility }),
      });
      setToast({ tone: 'success', message: visibility.length === 0 ? '应用已设为公开' : '可见群组已更新' });
      await onRefresh();
    });
  }

  async function requestCollaborator() {
    await runAction(setToast, '协作者申请失败', async () => {
      await api(`/api/v1/apps/${app.id}/collaborator-requests`, {
        method: 'POST',
        body: JSON.stringify({ message: '我想协助维护这个应用' }),
      });
      setToast({ tone: 'success', message: '协作者申请已提交' });
    });
  }

  async function decideCollaboratorRequest(requestID: number, approve: boolean) {
    await runAction(setToast, '协作者申请处理失败', async () => {
      await api(`/api/v1/collaborator-requests/${requestID}/${approve ? 'approve' : 'reject'}`, { method: 'POST' });
      setToast({ tone: approve ? 'success' : 'neutral', message: approve ? '协作者已通过' : '协作者申请已拒绝' });
      await loadCollaboratorRequests();
    });
  }

  async function toggleAppFavorite() {
    await runAction(setToast, '收藏更新失败', async () => {
      await api(`/api/v1/apps/${app.id}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: '收藏已更新' });
    });
  }

  async function toggleSubmitterFavorite() {
    await runAction(setToast, '提交者收藏更新失败', async () => {
      await api(`/api/v1/submitters/${app.ownerId}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: '提交者收藏已更新' });
    });
  }

  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <article className="drawer" onClick={(event) => event.stopPropagation()}>
        <button className="icon-button close" aria-label="关闭" onClick={onClose}><X size={18} /></button>
        <div className="detail-head">
          <div className="app-icon">{app.name.slice(0, 1)}</div>
          <div>
            <h2>{app.name}</h2>
            <p>{app.summary || app.description}</p>
            <div className="meta-line">
              <span>{app.owner}</span>
              <span>{app.category || '未分类'}</span>
              <span>{app.latestVersion?.version || '-'}</span>
            </div>
          </div>
        </div>
        <div className="detail-actions">
          <button className="primary-button" onClick={() => onInstall(app)}>
            <Download size={18} />
            <span>安装</span>
          </button>
          {user && (
            <>
              <button className="secondary-button" onClick={() => void toggleAppFavorite()}>
                <Heart size={18} />
                <span>收藏</span>
              </button>
              <button className="secondary-button" onClick={() => void toggleSubmitterFavorite()}>
                <Star size={18} />
                <span>提交者</span>
              </button>
              <button className="secondary-button" onClick={() => void markOutdated()}>
                <AlertCircle size={18} />
                <span>过期</span>
              </button>
              <button className="secondary-button" onClick={() => void clearOutdated()}>
                <Check size={18} />
                <span>取消过期</span>
              </button>
            </>
          )}
          {user && user.id !== app.ownerId && (
            <button className="secondary-button" onClick={() => void requestCollaborator()}>
              <Users size={18} />
              <span>协作</span>
            </button>
          )}
          {canMaintain && (
            <>
              <button className="secondary-button" onClick={() => void unlistApp()}>
                <Archive size={18} />
                <span>下架</span>
              </button>
              <button className="secondary-button danger-button" onClick={() => void deleteApp()}>
                <Trash2 size={18} />
                <span>删除</span>
              </button>
            </>
          )}
        </div>
        {(canMaintain || canUploadVersion) && (
          <section className="maintenance-grid">
            {canMaintain && (
              <form className="panel form-panel nested-panel" onSubmit={submitAppInfo}>
                <SectionTitle icon={Settings} title="应用信息" />
                <label>
                  <span>名称</span>
                  <input value={appForm.name} onChange={(event) => setAppForm({ ...appForm, name: event.target.value })} />
                </label>
                <label>
                  <span>摘要</span>
                  <input value={appForm.summary} onChange={(event) => setAppForm({ ...appForm, summary: event.target.value })} />
                </label>
                <label>
                  <span>描述</span>
                  <textarea value={appForm.description} onChange={(event) => setAppForm({ ...appForm, description: event.target.value })} />
                </label>
                <label>
                  <span>分类</span>
                  <select value={appForm.categoryId} onChange={(event) => setAppForm({ ...appForm, categoryId: event.target.value })}>
                    <option value="">未分类</option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>{category.name}</option>
                    ))}
                  </select>
                </label>
                <label>
                  <span>标签</span>
                  <input value={appForm.tags} onChange={(event) => setAppForm({ ...appForm, tags: event.target.value })} />
                </label>
                <label className="toggle-line">
                  <input
                    type="checkbox"
                    checked={appForm.commentsEnabled}
                    onChange={(event) => setAppForm({ ...appForm, commentsEnabled: event.target.checked })}
                  />
                  <span>允许评论反馈</span>
                </label>
                <label className="toggle-line">
                  <input
                    type="checkbox"
                    checked={appForm.allowUnreviewedUpdates}
                    onChange={(event) => setAppForm({ ...appForm, allowUnreviewedUpdates: event.target.checked })}
                  />
                  <span>免审批更新</span>
                </label>
                <button className="secondary-button">
                  <Save size={18} />
                  <span>保存信息</span>
                </button>
              </form>
            )}
            {canUploadVersion && (
              <form className="panel form-panel nested-panel" onSubmit={submitExternalVersion}>
                <SectionTitle icon={Link} title="发布版本" />
                <label>
                  <span>版本</span>
                  <input value={versionForm.version} onChange={(event) => setVersionForm({ ...versionForm, version: event.target.value })} />
                </label>
                <label>
                  <span>来源</span>
                  <select value={versionForm.sourceType} onChange={(event) => setVersionForm({ ...versionForm, sourceType: event.target.value })}>
                    <option value="GITHUB">GitHub Release</option>
                    <option value="WEBDAV">WebDAV URL</option>
                    <option value="S3">S3 URL</option>
                  </select>
                </label>
                <label>
                  <span>下载链接</span>
                  <input value={versionForm.downloadUrl} disabled={!!versionFile} onChange={(event) => setVersionForm({ ...versionForm, downloadUrl: event.target.value })} />
                </label>
                <label>
                  <span>外部文件 SHA256</span>
                  <input value={versionForm.sha256} disabled={!!versionFile} onChange={(event) => setVersionForm({ ...versionForm, sha256: event.target.value })} />
                </label>
                <label>
                  <span>更新日志</span>
                  <input value={versionForm.changelog} onChange={(event) => setVersionForm({ ...versionForm, changelog: event.target.value })} />
                </label>
                <label>
                  <span>LPK 文件</span>
                  <input type="file" accept=".lpk" onChange={(event) => setVersionFile(event.target.files?.[0] || null)} />
                </label>
                <button className="secondary-button">
                  <Upload size={18} />
                  <span>提交版本</span>
                </button>
              </form>
            )}
            {canMaintain && (
              <section className="panel form-panel nested-panel">
                <SectionTitle icon={Users} title="可见群组" />
                <div className="checkbox-list">
                  {groups.length === 0 ? (
                    <span className="muted-text">没有可用群组，当前为公开应用</span>
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
                <button className="secondary-button" onClick={() => void saveVisibility()}>
                  <Users size={18} />
                  <span>保存可见性</span>
                </button>
              </section>
            )}
            {canMaintain && (
              <section className="panel nested-panel">
                <SectionTitle icon={Users} title="协作者申请" />
                <div className="review-list">
                  {collaboratorRequests.length === 0 ? (
                    <EmptyState icon={Users} title="没有协作者申请" />
                  ) : (
                    collaboratorRequests.map((request) => (
                      <div className="review-row" key={request.id}>
                        <div>
                          <strong>{request.username || `用户 #${request.user_id || request.userId}`}</strong>
                          <span>{request.status} · {request.message || request.email || '无留言'}</span>
                        </div>
                        {request.status === 'PENDING' && (
                          <div className="row-actions">
                            <button className="icon-button ok" aria-label="通过协作者" onClick={() => void decideCollaboratorRequest(request.id, true)}>
                              <Check size={17} />
                            </button>
                            <button className="icon-button danger" aria-label="拒绝协作者" onClick={() => void decideCollaboratorRequest(request.id, false)}>
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
          <h3>截图</h3>
          {(app.screenshots || []).length > 0 ? (
            <div className="screenshot-grid">
              {(app.screenshots || []).map((shot, index, shots) => (
                <figure className="screenshot-item" key={shot.id}>
                  <img src={shot.imageUrl} alt={shot.caption || app.name} />
                  {shot.caption && <figcaption>{shot.caption}</figcaption>}
                  {canMaintain && (
                    <div className="screenshot-actions">
                      <button className="icon-button" aria-label="上移截图" disabled={index === 0} onClick={() => void moveScreenshot(shot.id, -1)}>
                        <ArrowUp size={15} />
                      </button>
                      <button className="icon-button" aria-label="下移截图" disabled={index === shots.length - 1} onClick={() => void moveScreenshot(shot.id, 1)}>
                        <ArrowDown size={15} />
                      </button>
                      <button className="icon-button danger" aria-label="删除截图" onClick={() => void deleteScreenshot(shot.id)}>
                        <Trash2 size={15} />
                      </button>
                    </div>
                  )}
                </figure>
              ))}
            </div>
          ) : (
            <EmptyState icon={Archive} title="没有截图" />
          )}
          {canMaintain && (
            <form className="comment-form screenshot-form" onSubmit={uploadScreenshot}>
              <input value={screenshotCaption} onChange={(event) => setScreenshotCaption(event.target.value)} placeholder="截图说明" />
              <input type="file" accept=".png,.jpg,.jpeg,.webp" onChange={(event) => setScreenshotFile(event.target.files?.[0] || null)} />
              <button className="icon-button" aria-label="上传截图"><Upload size={17} /></button>
            </form>
          )}
        </section>
        <section>
          <h3>版本历史</h3>
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
          <h3>评论</h3>
          {user && (
            <form className="comment-form" onSubmit={submitComment}>
              <input value={commentText} onChange={(event) => setCommentText(event.target.value)} placeholder="发布反馈" />
              <button className="icon-button" aria-label="发布"><MessageSquare size={17} /></button>
            </form>
          )}
          <div className="comments">
            {(app.comments || []).map((comment) => (
              <div className="comment" key={comment.id}>
                <div className="comment-head">
                  <strong>{comment.username}</strong>
                  {(canMaintain || user?.id === comment.userId) && (
                    <button className="icon-button danger" aria-label="删除评论" onClick={() => void deleteComment(comment.id)}>
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
  if (apps.length === 0) return <EmptyState icon={PackagePlus} title="没有应用" />;
  return (
    <div className="app-grid">
      {apps.map((app) => (
        <article className="app-card" key={app.id}>
          <button className="app-open" onClick={() => void onOpen(app)}>
            <div className="app-icon">{app.name.slice(0, 1)}</div>
            <div>
              <h3>{app.name}</h3>
              <p>{app.summary || app.description || 'LPK 应用'}</p>
            </div>
            <ChevronRight size={18} />
          </button>
          <div className="app-meta">
            <span><Tag size={14} /> {app.category || '未分类'}</span>
            <span><Star size={14} /> {app.latestVersion?.version || app.status}</span>
          </div>
          <button className="install-button" onClick={() => void onInstall(app)}>
            <Download size={17} />
            <span>安装</span>
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

function MobileTabs({ tab, setTab }: { tab: TabKey; setTab: (tab: TabKey) => void }) {
  return (
    <nav className="mobile-tabs">
      {tabs.map((item) => {
        const Icon = item.icon;
        return (
          <button key={item.key} className={cx(tab === item.key && 'active')} onClick={() => setTab(item.key)} aria-label={item.label}>
            <Icon size={20} />
            <span>{item.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
