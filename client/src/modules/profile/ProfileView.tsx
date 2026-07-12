import { type FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { AlertCircle, Check, ChevronRight, Cloud, Gauge, Heart, KeyRound, LogIn, LogOut, PackagePlus, RefreshCw, Search, Server, Settings, Trash2, Upload, Users, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { useTranslation } from 'react-i18next';
import { UserAvatar } from '../../components/AppIcon';
import { api, apiWithUploadProgress } from '../../shared/api';
import { canUserManageApp, canUserUploadVersion, defaultUploadStorageKey, displayUserName, reconcileScreenshotCaptions, screenshotFileKey, storageSelectOptions } from '../../shared/appHelpers';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import type { Category, ClientSourceStats, CollaborationData, FavoriteData, InstalledApplication, LPKInspectionStatus, PaginatedResponse, Pagination as PaginationMeta, SiteProfile, SourceApp, SourceSubscription, StorageOption, StoreApp, Toast, UpdateQueueRequest, UpdateQueueResult, User } from '../../shared/types';
import { cx, formatDate, hasInstallableVersion, runAction, statusKey } from '../../shared/utils';
import { InstalledAppsView } from '../client/InstalledAppsView';
import type { AppDetailMode } from '../storefront/AppDrawer';
import { APITokenWorkspace } from './APITokenWorkspace';
import { AppSubmissionForm, type SubmissionArtifactMode, type SubmissionProgress } from './AppSubmissionForm';
import { CollaborationPanel } from './CollaborationPanel';
import { MCPWorkspace } from './MCPWorkspace';

type ProfileWorkspaceTab = 'overview' | 'apps' | 'collaboration' | 'manage' | 'mcp' | 'tokens' | 'favorites';
type ProfileNavigationTarget = 'sources' | 'search';

const DEFAULT_FAVORITE_PAGINATION: PaginationMeta = { page: 1, pageSize: 0, totalItems: 0, totalPages: 0 };
const FAVORITE_PAGE_SIZE_OPTIONS = [12, 24, 48, 96, 100];

type BulkLPKInspectionItem = { appId: number; appName: string; inspection: LPKInspectionStatus };
type BulkLPKInspectionResponse = {
  inspections: BulkLPKInspectionItem[];
  skipped: Array<{ appId: number; appName: string; reason: string }>;
};

const terminalInspectionStates = new Set(['SUCCEEDED', 'FAILED', 'TIMED_OUT', 'CANCELLED']);

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

export function ProfileView({
  user,
  setUser,
  apps,
  managedApps,
  categories,
  sourceApps,
  sources,
  sourceStats,
  installedApps,
  installedState,
  installedError,
  onLoadInstalled,
	  onSetAutoUpdatePolicy,
	  autoUpdatePolicySaving,
	  onRunUpdates,
	  updateQueueResult,
	  isUpdateQueueRunning,
  onOpen,
  onLogout,
  refreshAll,
  setToast,
  hasAPI,
  storageOptions,
  collaborationData,
  onCollaborationRefresh,
  tagOptions,
  siteProfile,
  openSubmitSignal,
  onNavigate,
  onLogin,
}: {
  user: User | null;
  setUser: (user: User | null) => void;
  apps: StoreApp[];
  managedApps: StoreApp[];
  categories: Category[];
  sourceApps: SourceApp[];
  sources: SourceSubscription[];
  sourceStats: ClientSourceStats;
  installedApps: InstalledApplication[];
  installedState: 'idle' | 'loading' | 'loaded' | 'error';
  installedError: string;
  onLoadInstalled: (options?: { quiet?: boolean }) => Promise<void>;
	  onSetAutoUpdatePolicy?: (packageID: string, enabled: boolean) => Promise<void>;
	  autoUpdatePolicySaving?: Set<string>;
	  onRunUpdates?: (options?: UpdateQueueRequest) => Promise<void>;
	  updateQueueResult?: UpdateQueueResult | null;
	  isUpdateQueueRunning?: boolean;
  onOpen: (app: StoreApp, mode?: AppDetailMode) => void;
  onLogout: () => Promise<void>;
  refreshAll: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
  hasAPI: boolean;
  storageOptions: StorageOption[];
  collaborationData: CollaborationData;
  onCollaborationRefresh: () => Promise<void>;
  tagOptions: string[];
  siteProfile: SiteProfile;
  openSubmitSignal: number;
  onNavigate: (tab: ProfileNavigationTarget) => void;
  onLogin: () => void;
}) {
  const { t } = useTranslation();
  const [workspaceTab, setWorkspaceTab] = useState<ProfileWorkspaceTab>(() => (collaborationInviteTokenFromURL() ? 'collaboration' : 'overview'));
  const [isSubmitOpen, setIsSubmitOpen] = useState(false);
  const [selectedOwnedAppIDs, setSelectedOwnedAppIDs] = useState<Set<number>>(() => new Set());
  const [bulkAction, setBulkAction] = useState<'refresh' | 'delete' | null>(null);
  const [bulkRefreshOverwrite, setBulkRefreshOverwrite] = useState(false);
  const [isBulkDeleting, setIsBulkDeleting] = useState(false);
  const [bulkRefreshPhase, setBulkRefreshPhase] = useState<'idle' | 'queueing' | 'running'>('idle');
  const [bulkRefreshItems, setBulkRefreshItems] = useState<BulkLPKInspectionItem[]>([]);
  const [bulkRefreshSkipped, setBulkRefreshSkipped] = useState(0);
  const bulkRefreshRunRef = useRef(0);
  const [managedSubmitter, setManagedSubmitter] = useState('all');
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
  const handledOpenSubmitSignalRef = useRef(0);
  const [favorites, setFavorites] = useState<FavoriteData>({ apps: [], submitters: [] });
  const [favoriteTab, setFavoriteTab] = useState<'apps' | 'submitters'>('apps');
  const [favoritePagination, setFavoritePagination] = useState<{ apps: PaginationMeta; submitters: PaginationMeta }>({
    apps: DEFAULT_FAVORITE_PAGINATION,
    submitters: DEFAULT_FAVORITE_PAGINATION,
  });
  const canUseManagementWorkspace = user?.role === 'SOFTWARE_ADMIN' || user?.role === 'SITE_ADMIN';
  const workspaceTabs = [
    { key: 'overview' as const, label: t('profile.tabs.overview'), icon: Gauge },
    { key: 'apps' as const, label: t('profile.tabs.apps'), icon: PackagePlus },
    { key: 'collaboration' as const, label: t('profile.tabs.collaboration'), icon: Users },
    ...(canUseManagementWorkspace ? [{ key: 'manage' as const, label: t('profile.tabs.manage'), icon: Settings }] : []),
    { key: 'mcp' as const, label: t('profile.tabs.mcp'), icon: Server },
    { key: 'tokens' as const, label: t('profile.tabs.tokens'), icon: KeyRound },
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
  const bulkRefreshCompleted = bulkRefreshItems.filter((item) => terminalInspectionStates.has(item.inspection.state)).length;
  const bulkRefreshFailed = bulkRefreshItems.filter((item) => ['FAILED', 'TIMED_OUT', 'CANCELLED'].includes(item.inspection.state)).length;
  const bulkRefreshProgress = bulkRefreshItems.length > 0 ? Math.round((bulkRefreshCompleted / bulkRefreshItems.length) * 100) : 0;

  async function refreshOwnedLPKMetadata() {
    if (bulkRefreshPhase !== 'idle' || selectedOwnedAppIDs.size === 0) return;
    const runID = ++bulkRefreshRunRef.current;
    setBulkRefreshPhase('queueing');
    setBulkRefreshItems([]);
    setBulkRefreshSkipped(0);
    try {
      const queued = await api<BulkLPKInspectionResponse>('/api/v1/me/apps/lpk-inspections', {
        method: 'POST',
        body: JSON.stringify({ appIds: Array.from(selectedOwnedAppIDs), overwriteExistingMetadata: bulkRefreshOverwrite }),
      });
      if (runID !== bulkRefreshRunRef.current) return;
      setBulkRefreshItems(queued.inspections);
      setBulkRefreshSkipped(queued.skipped.length);
      if (queued.inspections.length === 0) {
        setBulkRefreshPhase('idle');
        setToast({ tone: 'neutral', message: t('profile.bulkRefreshNone') });
        return;
      }
      setBulkRefreshPhase('running');
      const ids = queued.inspections.map((item) => item.inspection.id);
      while (runID === bulkRefreshRunRef.current) {
        const status = await api<BulkLPKInspectionResponse>('/api/v1/me/apps/lpk-inspections/status', {
          method: 'POST',
          body: JSON.stringify({ ids }),
        });
        if (runID !== bulkRefreshRunRef.current) return;
        setBulkRefreshItems(status.inspections);
        const completed = status.inspections.filter((item) => terminalInspectionStates.has(item.inspection.state));
        if (completed.length === status.inspections.length) {
          const failed = completed.filter((item) => item.inspection.state !== 'SUCCEEDED').length;
          await refreshAll({ silent: true });
          if (runID !== bulkRefreshRunRef.current) return;
          setBulkRefreshPhase('idle');
          setBulkRefreshItems([]);
          setSelectedOwnedAppIDs(new Set());
          setBulkRefreshOverwrite(false);
          setToast({
            tone: failed > 0 ? 'neutral' : 'success',
            message: t(failed > 0 ? 'profile.bulkRefreshCompletedWithFailures' : 'profile.bulkRefreshCompleted', {
              count: completed.length,
              failed,
              skipped: queued.skipped.length,
            }),
          });
          return;
        }
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
      }
    } catch {
      if (runID !== bulkRefreshRunRef.current) return;
      setBulkRefreshPhase('idle');
      setBulkRefreshItems([]);
      setToast({ tone: 'error', message: t('profile.bulkRefreshFailed') });
    }
  }

  async function deleteSelectedOwnedApps() {
    if (isBulkDeleting || selectedOwnedAppIDs.size === 0) return;
    setIsBulkDeleting(true);
    const appIDs = Array.from(selectedOwnedAppIDs);
    let failed = 0;
    try {
      for (const appID of appIDs) {
        try {
          await api(`/api/v1/apps/${appID}`, { method: 'DELETE' });
        } catch {
          failed += 1;
        }
      }
      await refreshAll({ silent: true });
      setSelectedOwnedAppIDs(new Set());
      setBulkAction(null);
      setToast({
        tone: failed > 0 ? 'neutral' : 'success',
        message: t(failed > 0 ? 'profile.bulkDeleteCompletedWithFailures' : 'profile.bulkDeleteCompleted', { count: appIDs.length - failed, failed }),
      });
    } catch {
      setToast({ tone: 'error', message: t('profile.bulkDeleteFailed') });
    } finally {
      setIsBulkDeleting(false);
    }
  }

  useEffect(() => () => {
    bulkRefreshRunRef.current += 1;
  }, []);

  useEffect(() => {
    const ownedIDs = new Set(ownedApps.map((app) => app.id));
    setSelectedOwnedAppIDs((current) => {
      const next = new Set(Array.from(current).filter((id) => ownedIDs.has(id)));
      return next.size === current.size ? current : next;
    });
  }, [ownedApps]);

  function toggleOwnedAppSelection(appID: number, selected: boolean) {
    if (bulkRefreshPhase !== 'idle') return;
    setSelectedOwnedAppIDs((current) => {
      const next = new Set(current);
      if (selected) next.add(appID);
      else next.delete(appID);
      return next;
    });
  }
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
  const storageChoices = storageSelectOptions(storageOptions);

  useEffect(() => {
    const fallback = defaultUploadStorageKey(storageOptions);
    setUploadStorageKey((current) => (storageOptions.some((storage) => storage.key === current) ? current : fallback));
  }, [storageOptions]);

  useEffect(() => {
    if (!user || openSubmitSignal === 0 || handledOpenSubmitSignalRef.current === openSubmitSignal) return;
    handledOpenSubmitSignalRef.current = openSubmitSignal;
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
    void loadFavorites();
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

  function favoritePath(targetType: 'APP' | 'SUBMITTER', page: number, pageSize?: number) {
    const params = new URLSearchParams({ targetType, page: String(page || 1) });
    if (pageSize && pageSize > 0) params.set('pageSize', String(pageSize));
    return `/api/v1/me/favorites?${params.toString()}`;
  }

  async function loadFavoriteApps(page = favoritePagination.apps.page || 1, pageSize = favoritePagination.apps.pageSize) {
    const data = await api<PaginatedResponse<StoreApp, 'apps'>>(favoritePath('APP', page, pageSize));
    setFavorites((current) => ({ ...current, apps: data.apps || [] }));
    setFavoritePagination((current) => ({
      ...current,
      apps: data.pagination || { page, pageSize, totalItems: data.apps?.length || 0, totalPages: 1 },
    }));
  }

  async function loadFavoriteSubmitters(page = favoritePagination.submitters.page || 1, pageSize = favoritePagination.submitters.pageSize) {
    const data = await api<PaginatedResponse<User, 'submitters'>>(favoritePath('SUBMITTER', page, pageSize));
    setFavorites((current) => ({ ...current, submitters: data.submitters || [] }));
    setFavoritePagination((current) => ({
      ...current,
      submitters: data.pagination || { page, pageSize, totalItems: data.submitters?.length || 0, totalPages: 1 },
    }));
  }

  async function loadFavorites() {
    await runAction(setToast, t('favorites.loadFailed'), async () => {
      await Promise.all([loadFavoriteApps(), loadFavoriteSubmitters()]);
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
              <StatusBadge
                tone={sourceCacheReady ? 'approved' : 'unlisted'}
                icon={sourceCacheReady ? <Check size={14} /> : <AlertCircle size={14} />}
                label={sourceCacheReady ? t('sources.ready') : t('sources.needsValue')}
              />
              <strong>{t('profile.clientSourceTitle')}</strong>
              <small>{sourceCacheBody}</small>
            </div>
            <div className={cx('readiness-step', installCatalogReady && 'ready')}>
              <StatusBadge
                tone={installCatalogReady ? 'approved' : 'unlisted'}
                icon={installCatalogReady ? <Check size={14} /> : <AlertCircle size={14} />}
                label={installCatalogReady ? t('sources.ready') : t('sources.needsValue')}
              />
              <strong>{t('profile.clientInstallTitle')}</strong>
              <small>{installCatalogBody}</small>
            </div>
            <div className={cx('readiness-step', installedLookupReady && 'ready')}>
              <StatusBadge
                tone={installedState === 'error' ? 'failed' : installedState === 'loading' ? 'pending' : installedLookupReady ? 'synced' : 'unsynced'}
                icon={installedState === 'error' ? <AlertCircle size={14} /> : installedLookupReady ? <Check size={14} /> : <Gauge size={14} />}
                label={t(`profile.installedState.${installedState}`)}
              />
              <strong>{t('profile.clientInstalledTitle')}</strong>
              <small>{installedReadinessBody}</small>
            </div>
          </div>
        </section>
        <InstalledAppsView
          installedApps={installedApps}
          sourceApps={sourceApps}
          sources={sources}
          installedState={installedState}
          installedError={installedError}
          installedReadinessBody={installedReadinessBody}
          onLoadInstalled={onLoadInstalled}
		  onSetAutoUpdatePolicy={onSetAutoUpdatePolicy}
		  autoUpdatePolicySaving={autoUpdatePolicySaving}
		  onRunUpdates={onRunUpdates}
		  updateQueueResult={updateQueueResult}
		  isUpdateQueueRunning={isUpdateQueueRunning}
        />
      </section>
    );
  }

  if (!user) {
    return (
      <section className="page-grid">
        <EmptyState
          icon={LogIn}
          title={t('auth.loginRequired')}
          body={t('auth.loginRequiredBody')}
          action={{ label: t('auth.login'), icon: LogIn, onClick: onLogin }}
        />
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
                  await onLogout();
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
        <XTabList value={workspaceTab} onChange={(value) => setWorkspaceTab(value as typeof workspaceTab)} aria-label={t('profile.tabs.label')} size="sm" hasDivider>
          {workspaceTabs.map((item) => {
            const Icon = item.icon;
            return (
              <XTab
                key={item.key}
                value={item.key}
                label={item.label}
                icon={<Icon size={17} />}
              />
            );
          })}
        </XTabList>
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
                await onLogout();
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
          <div className="section-title-actions">
            {selectedOwnedAppIDs.size > 0 && (
              <>
                <XIconButton
                  type="button"
                  variant="secondary"
                  size="sm"
                  label={bulkRefreshPhase === 'queueing'
                    ? t('profile.bulkRefreshQueueing')
                    : bulkRefreshPhase === 'running'
                      ? t('profile.bulkRefreshing', { completed: bulkRefreshCompleted, total: bulkRefreshItems.length })
                      : t('profile.bulkRefreshSelected', { count: selectedOwnedAppIDs.size })}
                  tooltip={bulkRefreshPhase === 'idle' ? t('profile.bulkRefreshSelected', { count: selectedOwnedAppIDs.size }) : undefined}
                  icon={<RefreshCw size={17} className={bulkRefreshPhase !== 'idle' ? 'spin' : undefined} />}
                  isDisabled={bulkRefreshPhase !== 'idle' || isBulkDeleting}
                  onClick={() => { setBulkRefreshOverwrite(false); setBulkAction('refresh'); }}
                />
                <XIconButton
                  type="button"
                  variant="destructive"
                  size="sm"
                  label={t('profile.bulkDeleteSelected', { count: selectedOwnedAppIDs.size })}
                  tooltip={t('profile.bulkDeleteSelected', { count: selectedOwnedAppIDs.size })}
                  icon={<Trash2 size={17} />}
                  isDisabled={bulkRefreshPhase !== 'idle' || isBulkDeleting}
                  onClick={() => setBulkAction('delete')}
                />
              </>
            )}
            <XButton
              type="button"
              variant="primary"
              size="sm"
              label={isSubmitOpen ? t('common.close') : t('submitApp.title')}
              icon={isSubmitOpen ? <X size={17} /> : <Upload size={17} />}
              onClick={() => setIsSubmitOpen((open) => !open)}
            />
          </div>
        </div>
        {bulkRefreshPhase === 'running' && (
          <div className="bulk-metadata-refresh" role="status" aria-live="polite">
            <div className="bulk-metadata-refresh-copy">
              <strong>{t('profile.bulkRefreshProgressTitle')}</strong>
              <span>{t('profile.bulkRefreshProgressBody', { completed: bulkRefreshCompleted, total: bulkRefreshItems.length, failed: bulkRefreshFailed, skipped: bulkRefreshSkipped })}</span>
            </div>
            <XProgressBar label={t('profile.bulkRefreshProgressLabel')} isLabelHidden value={bulkRefreshProgress} hasValueLabel variant={bulkRefreshFailed > 0 ? 'warning' : 'accent'} />
          </div>
        )}
        {ownedApps.length === 0 ? (
          <EmptyState icon={PackagePlus} title={t('profile.mySubmissionsEmpty')} body={t('profile.mySubmissionsEmptyBody')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {ownedApps.map((item) => (
              <XListItem
                key={item.id}
                isSelected={selectedOwnedAppIDs.has(item.id)}
                startContent={(
                  <XCheckboxInput
                    label={t('profile.selectForBulkAction', { name: item.name })}
                    isLabelHidden
                    value={selectedOwnedAppIDs.has(item.id)}
                    isDisabled={bulkRefreshPhase !== 'idle' || isBulkDeleting}
                    onChange={(selected) => toggleOwnedAppSelection(item.id, selected)}
                  />
                )}
                label={item.name}
                description={(
                  <span className="action-list-description">
                    <span>{item.latestVersion?.version || t('app.noPublishedVersion')} · {formatDate(item.updatedAt)}</span>
                    <small>{t(`profile.submissionStep.${submissionStep(item).key}`)}</small>
                  </span>
                )}
                endContent={(
                  <div className="row-actions">
                    <StatusBadge tone={submissionStep(item).tone} label={t(`statusLabels.${statusKey(item.status)}`)} />
                    <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                    {(canUserManageApp(user, item) || canUserUploadVersion(user, item)) && (
                      <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                    )}
                  </div>
                )}
              />
            ))}
          </XList>
        )}
      </section>
      )}

      {workspaceTab === 'apps' && isSubmitOpen && (
      <ModalLayer onClose={() => setIsSubmitOpen(false)} purpose="form" width="min(960px, calc(100vw - 36px))" maxHeight="min(90vh, 920px)">
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
      </ModalLayer>
      )}
      {workspaceTab === 'apps' && bulkAction && (
        <ModalLayer onClose={() => { if (!isBulkDeleting) { setBulkAction(null); setBulkRefreshOverwrite(false); } }} purpose="required" width="min(480px, calc(100vw - 32px))">
          <section className="bulk-action-confirmation" aria-labelledby="bulk-action-confirmation-title">
            <div className="section-title">
              {bulkAction === 'delete' ? <Trash2 size={19} /> : <RefreshCw size={19} />}
              <h2 id="bulk-action-confirmation-title">{t(bulkAction === 'delete' ? 'profile.bulkDeleteConfirmTitle' : 'profile.bulkRefreshConfirmTitle')}</h2>
            </div>
            <p>{t(bulkAction === 'delete' ? 'profile.bulkDeleteConfirmBody' : 'profile.bulkRefreshConfirmBody', { count: selectedOwnedAppIDs.size })}</p>
            {bulkAction === 'refresh' && (
              <XCheckboxInput
                label={t('profile.bulkRefreshOverwrite')}
                description={t('profile.bulkRefreshOverwriteHelp')}
                value={bulkRefreshOverwrite}
                onChange={setBulkRefreshOverwrite}
              />
            )}
            <div className="bulk-action-selection" role="list" aria-label={t('profile.selectedApps')}>
              {ownedApps.filter((app) => selectedOwnedAppIDs.has(app.id)).map((app) => <span role="listitem" key={app.id}>{app.name}</span>)}
            </div>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={isBulkDeleting} onClick={() => { setBulkAction(null); setBulkRefreshOverwrite(false); }} />
              <XButton
                type="button"
                variant={bulkAction === 'delete' ? 'destructive' : 'primary'}
                label={t(bulkAction === 'delete'
                  ? (isBulkDeleting ? 'profile.bulkDeleting' : 'profile.bulkDeleteConfirmAction')
                  : bulkRefreshOverwrite
                    ? 'profile.bulkRefreshOverwriteConfirmAction'
                    : 'profile.bulkRefreshConfirmAction')}
                icon={bulkAction === 'delete' ? <Trash2 size={17} /> : <RefreshCw size={17} />}
                isDisabled={isBulkDeleting}
                onClick={() => {
                  if (bulkAction === 'delete') void deleteSelectedOwnedApps();
                  else {
                    setBulkAction(null);
                    void refreshOwnedLPKMetadata();
                  }
                }}
              />
            </div>
          </section>
        </ModalLayer>
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
          <div className="toolbar-row">
            <XSelector
              label={t('profile.maintainerFilter')}
              value={managedSubmitter}
              options={managedSubmitterOptions}
              onChange={setManagedSubmitter}
            />
          </div>
          {manageableApps.length === 0 ? (
            <EmptyState icon={Settings} title={t('profile.appManagementEmpty')} body={t('profile.appManagementEmptyBody')} />
          ) : (
            <XList className="action-list management-app-list" density="compact" hasDividers>
              {manageableApps.map((item) => (
                <XListItem
                  className="management-app-row"
                  key={item.id}
                  label={item.name}
                  description={(
                    <span className="action-list-description">
                      <span>{item.owner} · {item.latestVersion?.version || t('app.noPublishedVersion')} · {formatDate(item.updatedAt)}</span>
                      <small>{t(`profile.submissionStep.${submissionStep(item).key}`)}</small>
                    </span>
                  )}
                  endContent={(
                    <div className="row-actions management-row-actions">
                      <StatusBadge tone={submissionStep(item).tone} label={t(`statusLabels.${statusKey(item.status)}`)} />
                      <XIconButton className="fixed-row-icon-button" type="button" variant="ghost" size="sm" label={t('profile.openSubmission')} tooltip={t('profile.openSubmission')} icon={<ChevronRight size={17} />} onClick={() => void onOpen(item)} />
                      <XIconButton className="fixed-row-icon-button" type="button" variant="secondary" size="sm" label={t('profile.manageApp')} tooltip={t('profile.manageApp')} icon={<Settings size={17} />} onClick={() => void onOpen(item, 'manage')} />
                    </div>
                  )}
                />
              ))}
            </XList>
          )}
        </section>
      </section>
      )}
      {workspaceTab === 'mcp' && (
        <MCPWorkspace user={user} siteSourceUrl={siteProfile.sourceUrl} setToast={setToast} />
      )}
      {workspaceTab === 'tokens' && (
        <APITokenWorkspace user={user} setToast={setToast} />
      )}

      {workspaceTab === 'favorites' && (
      <section className="workspace-pane">
        <section className="panel">
          <SectionTitle icon={Heart} title={t('favorites.title')} />
          <div className="section-toolbar">
            <XTabList value={favoriteTab} onChange={(value) => setFavoriteTab(value as 'apps' | 'submitters')} aria-label={t('favorites.tabsLabel')} size="sm">
              <XTab value="apps" label={t('favorites.apps')} icon={<PackagePlus size={17} />} />
              <XTab value="submitters" label={t('favorites.submitters')} icon={<Users size={17} />} />
            </XTabList>
            <XButton type="button" variant="secondary" size="sm" label={t('favorites.refresh')} icon={<RefreshCw size={18} />} onClick={() => void loadFavorites()} />
          </div>
          {favoriteTab === 'apps' ? (
            <>
              {favorites.apps.length === 0 ? (
                <EmptyState icon={Heart} title={t('favorites.emptyApps')} />
              ) : (
                <XList className="action-list" density="compact" hasDividers>
                  {favorites.apps.map((item) => (
                    <XListItem
                      key={`app-${item.id}`}
                      label={item.name}
                      description={`${item.owner} · ${item.latestVersion?.version || item.status}`}
                      onClick={() => void onOpen(item)}
                      endContent={<ChevronRight size={17} aria-hidden="true" />}
                    />
                  ))}
                </XList>
              )}
              {favoritePagination.apps.pageSize > 0 && favoritePagination.apps.totalItems > favoritePagination.apps.pageSize && (
                <XPagination
                  className="list-pagination"
                  page={favoritePagination.apps.page}
                  onChange={(page) => void runAction(setToast, t('favorites.loadFailed'), () => loadFavoriteApps(page, favoritePagination.apps.pageSize))}
                  totalItems={favoritePagination.apps.totalItems}
                  pageSize={favoritePagination.apps.pageSize}
                  pageSizeOptions={FAVORITE_PAGE_SIZE_OPTIONS}
                  onPageSizeChange={(pageSize) => void runAction(setToast, t('favorites.loadFailed'), () => loadFavoriteApps(1, pageSize))}
                  variant="pages"
                  size="sm"
                  label={t('pagination.label')}
                />
              )}
            </>
          ) : (
            favorites.submitters.length === 0 ? (
              <EmptyState icon={Users} title={t('favorites.emptySubmitters')} />
            ) : (
              <XList className="action-list" density="compact" hasDividers>
                {favorites.submitters.map((item) => (
                  <XListItem
                    key={`submitter-${item.id}`}
                    label={displayUserName(item)}
                    description={item.email || t('favorites.submitter')}
                  />
                ))}
              </XList>
            )
          )}
          {favoriteTab === 'submitters' && favoritePagination.submitters.pageSize > 0 && favoritePagination.submitters.totalItems > favoritePagination.submitters.pageSize && (
            <XPagination
              className="list-pagination"
              page={favoritePagination.submitters.page}
              onChange={(page) => void runAction(setToast, t('favorites.loadFailed'), () => loadFavoriteSubmitters(page, favoritePagination.submitters.pageSize))}
              totalItems={favoritePagination.submitters.totalItems}
              pageSize={favoritePagination.submitters.pageSize}
              pageSizeOptions={FAVORITE_PAGE_SIZE_OPTIONS}
              onPageSizeChange={(pageSize) => void runAction(setToast, t('favorites.loadFailed'), () => loadFavoriteSubmitters(1, pageSize))}
              variant="pages"
              size="sm"
              label={t('pagination.label')}
            />
          )}
        </section>
      </section>
      )}
    </section>
  );
}
