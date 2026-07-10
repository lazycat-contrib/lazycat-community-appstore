import { type FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { Archive, Check, CloudUpload, Copy, DatabaseBackup, Download, Gauge, KeyRound, Layers3, Megaphone, MessageSquare, Pencil, Plus, Save, Server, Settings, ShieldCheck, Tag, Trash2, Upload, Users, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { Pagination as XPagination } from '@astryxdesign/core/Pagination';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { TreeList as XTreeList, type TreeListItemData } from '@astryxdesign/core/TreeList';
import { useTranslation } from 'react-i18next';
import { AnnouncementBanner } from '../../components/AnnouncementBanner';
import { AdSpot } from '../../components/AdSpot';
import { api, fetchAllPaginated } from '../../shared/api';
import { categoryDescendantIds, flattenCategoryTree } from '../../shared/categoryTree';
import { CollectionAppPicker } from './CollectionAppPicker';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { FilePicker } from '../../shared/components/FilePicker';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { RECOMMENDED_DOWNLOAD_MIRRORS, RECOMMENDED_RAW_MIRRORS, mirrorPresetText } from '../../shared/constants';
import type { Category, Collection, CollectionDraft, PaginatedResponse, Pagination as PaginationMeta, RegistrationInvite, Review, SiteAd, SiteAnnouncement, SiteProfile, StorageOption, StoreApp, TagRecord, Toast, User } from '../../shared/types';
import { errorMessage, formatBytes, formatDate, localizedName, reviewKindKey, runAction, shortSHA, statusKey, stripTrailingSlash } from '../../shared/utils';
import { AdminUsersWorkspace } from './AdminUsersWorkspace';
import { draftFromUser, emptyUserDraft, type ManagedUserDraft } from './AdminUsersPanel';
import { AdminAnnouncementsPanel } from './AdminAnnouncementsPanel';
import { AdminAdsPanel } from './AdminAdsPanel';
import { StorageSettingsPanel, defaultStorageSettings, type StorageSettings } from './StorageSettingsPanel';
import { AdminMigrationPanel } from './migration/AdminMigrationPanel';
import { AdminBackupPanel } from './AdminBackupPanel';
import { AdminDeleteDialog } from './AdminDeleteDialog';
import { AdminSaveBar } from './AdminSaveBar';
import { areAdminDraftsEqual, type AdminOperationResult, type AdminSaveStatus, type AdminStorageAction } from './adminState';
import { AdminTaskHeader, type AdminTaskHeaderProps } from './AdminTaskHeader';

type TaxonomyDraft = { name: string; nameI18n: Record<string, string>; slug: string; parentId?: string; sortOrder?: string };
type AdminTask = 'reviews' | 'site' | 'users' | 'taxonomy' | 'collections' | 'storage' | 'backup' | 'migration';
type SiteSettingsTask = 'identity' | 'announcement' | 'ads' | 'registration' | 'policy' | 'mail';
type AdminDeleteTarget =
  | { kind: 'user'; item: User }
  | { kind: 'category'; item: Category }
  | { kind: 'tag'; item: TagRecord }
  | { kind: 'collection'; item: Collection }
  | { kind: 'invite'; item: RegistrationInvite };

const DEFAULT_LIST_PAGINATION: PaginationMeta = { page: 1, pageSize: 0, totalItems: 0, totalPages: 0 };
const ADMIN_PAGE_SIZE_OPTIONS = [12, 24, 48, 50, 96, 100, 200];

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

function storageSettingsPayload(storage: StorageSettings) {
  return {
    key: storage.key,
    name: storage.name,
    provider: storage.provider,
    deliveryMode: storage.deliveryMode,
    localPath: storage.localPath,
    endpointUrl: storage.endpointUrl,
    bucketName: storage.bucketName,
    region: storage.region,
    pathStyle: storage.pathStyle,
    accountId: storage.accountId,
    rootPrefix: storage.rootPrefix,
    accessKeyId: storage.accessKeyId,
    secretAccessKey: storage.secretAccessKey || '',
    webdavUsername: storage.webdavUsername,
    webdavPassword: storage.webdavPassword || '',
    publicBaseUrl: storage.publicBaseUrl,
  };
}

function defaultUploadStorageKey(storages: StorageOption[]) {
  return storages.find((storage) => storage.isDefault)?.key || storages[0]?.key || 'primary';
}

function displayUserName(user: User | null | undefined) {
  return user?.nickname?.trim() || user?.username || '';
}

export function AdminPanel({
  user,
  reviews,
  reviewPagination,
  onReviewPageChange,
  onApprove,
  onSiteProfileSaved,
  onCatalogMetadataChanged,
  onStorageOptionsChanged,
  setToast,
}: {
  user: User;
  reviews: Review[];
  reviewPagination: PaginationMeta;
  onReviewPageChange: (page: number, pageSize?: number) => Promise<void>;
  onApprove: (review: Review, approve: boolean) => void;
  onSiteProfileSaved: (site?: SiteProfile) => Promise<void>;
  onCatalogMetadataChanged: () => Promise<void>;
  onStorageOptionsChanged: () => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  const { t } = useTranslation();
  const [adminTab, setAdminTab] = useState<AdminTask>('reviews');
  const [users, setUsers] = useState<User[]>([]);
  const [userPagination, setUserPagination] = useState<PaginationMeta>(DEFAULT_LIST_PAGINATION);
  const [apps, setApps] = useState<StoreApp[]>([]);
  const [approvedAppCount, setApprovedAppCount] = useState(0);
  const [reviewApps, setReviewApps] = useState<StoreApp[]>([]);
  const [settings, setSettings] = useState<Record<string, string>>({});
  const [savedSettings, setSavedSettings] = useState<Record<string, string>>({});
  const [settingsSaveStatus, setSettingsSaveStatus] = useState<AdminSaveStatus>('idle');
  const settingsRef = useRef<Record<string, string>>({});
  const savedSettingsRef = useRef<Record<string, string>>({});
  const settingsRevisionRef = useRef(0);
  const settingsRequestSequenceRef = useRef(0);
  const settingsSaveRequestSequenceRef = useRef(0);
  const settingsSaveInFlightRef = useRef(false);
  const settingsDirty = !areAdminDraftsEqual(settings, savedSettings);
  const [announcements, setAnnouncements] = useState<SiteAnnouncement[]>([]);
  const [ads, setAds] = useState<SiteAd[]>([]);
  const [registrationInvites, setRegistrationInvites] = useState<RegistrationInvite[]>([]);
  const [invitePagination, setInvitePagination] = useState<PaginationMeta>(DEFAULT_LIST_PAGINATION);
  const [inviteDraft, setInviteDraft] = useState({ note: '', maxUses: '1' });
  const [isInviteCreateOpen, setIsInviteCreateOpen] = useState(false);
  const [storageRecords, setStorageRecords] = useState<StorageSettings[]>([defaultStorageSettings]);
  const [defaultStorageKey, setDefaultStorageKey] = useState(defaultStorageSettings.key);
  const [selectedStorageKey, setSelectedStorageKey] = useState(defaultStorageSettings.key);
  const [storageDraft, setStorageDraft] = useState<StorageSettings>(defaultStorageSettings);
  const [storageCreateDraft, setStorageCreateDraft] = useState<StorageSettings>({ ...defaultStorageSettings, key: '', name: '' });
  const [isStorageCreateOpen, setIsStorageCreateOpen] = useState(false);
  const [storageSaveStatus, setStorageSaveStatus] = useState<AdminSaveStatus>('idle');
  const [storageAction, setStorageAction] = useState<AdminStorageAction>(null);
  const [storageResult, setStorageResult] = useState<AdminOperationResult | null>(null);
  const [storageToDelete, setStorageToDelete] = useState<StorageSettings | null>(null);
  const storageSelectionRef = useRef(defaultStorageSettings.key);
  const storageDraftRef = useRef<StorageSettings>(defaultStorageSettings);
  const storageRevisionRef = useRef(0);
  const storageActionRef = useRef<AdminStorageAction>(null);
  const [siteIconFile, setSiteIconFile] = useState<File | null>(null);
  const [siteIconStorageKey, setSiteIconStorageKey] = useState(defaultStorageSettings.key);
  const [isUploadingSiteIcon, setIsUploadingSiteIcon] = useState(false);
  const [adminCategories, setAdminCategories] = useState<Category[]>([]);
  const [adminTags, setAdminTags] = useState<TagRecord[]>([]);
  const [adminCollections, setAdminCollections] = useState<Collection[]>([]);
  const [categoryForm, setCategoryForm] = useState<TaxonomyDraft>({ name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '', parentId: '', sortOrder: '0' });
  const [tagForm, setTagForm] = useState<TaxonomyDraft>({ name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '' });
  const [collectionForm, setCollectionForm] = useState<{ name: string; kind: string; appIds: number[] }>({ name: '', kind: 'MANUAL', appIds: [] });
  const [siteSettingsTab, setSiteSettingsTab] = useState<SiteSettingsTask>('identity');
  const [userDialogMode, setUserDialogMode] = useState<'create' | 'edit' | null>(null);
  const [userDraft, setUserDraft] = useState<ManagedUserDraft>(emptyUserDraft);
  const [isCollectionCreateOpen, setIsCollectionCreateOpen] = useState(false);
  const [taxonomyCreateMode, setTaxonomyCreateMode] = useState<'category' | 'tag' | null>(null);
  const [editingCategoryID, setEditingCategoryID] = useState<number | null>(null);
  const [editingTagID, setEditingTagID] = useState<number | null>(null);
  const [categoryDrafts, setCategoryDrafts] = useState<Record<number, TaxonomyDraft>>({});
  const [tagDrafts, setTagDrafts] = useState<Record<number, TaxonomyDraft>>({});
  const [collectionDrafts, setCollectionDrafts] = useState<Record<number, CollectionDraft>>({});
  const [deleteTarget, setDeleteTarget] = useState<AdminDeleteTarget | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [testEmailTo, setTestEmailTo] = useState(user.email || '');
  const isSiteAdmin = user.role === 'SITE_ADMIN';
  const adminTabs = [
    { key: 'reviews', label: t('admin.tabs.reviews'), icon: ShieldCheck },
    ...(isSiteAdmin ? [{ key: 'site' as const, label: t('admin.tabs.site'), icon: Settings }] : []),
    ...(isSiteAdmin ? [{ key: 'users' as const, label: t('admin.tabs.users'), icon: Users }] : []),
    { key: 'taxonomy', label: t('admin.tabs.taxonomy'), icon: Tag },
    { key: 'collections', label: t('admin.tabs.collections'), icon: Layers3 },
    ...(isSiteAdmin ? [{ key: 'storage' as const, label: t('admin.siteSettingTabs.storage'), icon: Server }] : []),
    ...(isSiteAdmin ? [{ key: 'backup' as const, label: t('admin.siteSettingTabs.backup'), icon: CloudUpload }] : []),
    ...(isSiteAdmin ? [{ key: 'migration' as const, label: t('admin.siteSettingTabs.migration'), icon: DatabaseBackup }] : []),
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
    { key: 'ads', label: t('admin.siteSettingTabs.ads'), icon: Megaphone },
    { key: 'registration', label: t('admin.siteSettingTabs.registration'), icon: KeyRound },
    { key: 'policy', label: t('admin.siteSettingTabs.policy'), icon: ShieldCheck },
    { key: 'mail', label: t('admin.siteSettingTabs.mail'), icon: MessageSquare },
  ] as const;
  const timeZoneOptions = useMemo(() => {
    const intlWithTimeZones = Intl as typeof Intl & { supportedValuesOf?: (key: 'timeZone') => string[] };
    const supported = typeof intlWithTimeZones.supportedValuesOf === 'function' ? intlWithTimeZones.supportedValuesOf('timeZone') : [];
    const preferred = ['Asia/Shanghai', 'UTC', 'Asia/Hong_Kong', 'Asia/Taipei', 'Asia/Tokyo', 'Asia/Singapore', 'Europe/London', 'Europe/Berlin', 'America/Los_Angeles', 'America/New_York'];
    return Array.from(new Set([...preferred, ...supported])).map((value) => ({ value, label: value }));
  }, []);
  const selectedStorageRecord = storageRecords.find((storage) => storage.key === selectedStorageKey) || storageRecords[0] || defaultStorageSettings;
  const storageDirty = !areAdminDraftsEqual(storageSettingsPayload(storageDraft), storageSettingsPayload(selectedStorageRecord));
  const siteIdentityFields = [
    { key: 'site_title', label: t('admin.settings.siteTitle'), help: t('admin.settingsHelp.siteTitle') },
    { key: 'site_subtitle', label: t('admin.settings.siteSubtitle'), help: t('admin.settingsHelp.siteSubtitle'), type: 'textarea' },
    { key: 'site_icon_url', label: t('admin.settings.siteIconURL'), help: t('admin.settingsHelp.siteIconURL'), type: 'url' },
    { key: 'site_public_url', label: t('admin.settings.sitePublicURL'), help: t('admin.settingsHelp.sitePublicURL'), type: 'url' },
    { key: 'site_timezone', label: t('admin.settings.siteTimeZone'), help: t('admin.settingsHelp.siteTimeZone'), type: 'select', options: timeZoneOptions },
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
    { key: 'default_page_size', label: t('admin.settings.defaultPageSize'), help: t('admin.settingsHelp.defaultPageSize'), inputMode: 'numeric' },
    { key: 'comments_enabled', label: t('admin.settings.commentsEnabled'), help: t('admin.settingsHelp.commentsEnabled'), type: 'boolean' },
    { key: 'chat_enabled', label: t('admin.settings.chatEnabled'), help: t('admin.settingsHelp.chatEnabled'), type: 'boolean' },
    { key: 'chat_retention_days', label: t('admin.settings.chatRetentionDays'), help: t('admin.settingsHelp.chatRetentionDays'), inputMode: 'numeric' },
    { key: 'two_factor_auth_enabled', label: t('admin.settings.twoFactorAuthEnabled'), help: t('admin.settingsHelp.twoFactorAuthEnabled'), type: 'boolean' },
    { key: 'allow_manual_outdated_clear', label: t('admin.settings.allowManualOutdatedClear'), help: t('admin.settingsHelp.allowManualOutdatedClear'), type: 'boolean' },
    { key: 'min_client_version', label: t('admin.settings.minClientVersion'), help: t('admin.settingsHelp.minClientVersion') },
    { key: 'min_client_version_message', label: t('admin.settings.minClientVersionMessage'), help: t('admin.settingsHelp.minClientVersionMessage'), type: 'textarea' },
    { key: 'source_password', label: t('admin.settings.sourcePassword'), help: t('admin.settingsHelp.sourcePassword'), type: 'password' },
    { key: 'source_password_rotation', label: t('admin.settings.sourcePasswordRotation'), help: t('admin.settingsHelp.sourcePasswordRotation'), inputMode: 'numeric' },
    { key: 'source_v1_enabled', label: t('admin.settings.sourceV1Enabled'), help: t('admin.settingsHelp.sourceV1Enabled'), type: 'boolean' },
    { key: 'github_download_mirrors', label: t('admin.settings.githubDownloadMirrors'), help: t('admin.settingsHelp.githubDownloadMirrors'), type: 'textarea' },
    { key: 'github_raw_mirrors', label: t('admin.settings.githubRawMirrors'), help: t('admin.settingsHelp.githubRawMirrors'), type: 'textarea' },
    { key: 'require_email_verify', label: t('admin.settings.requireEmailVerify'), help: t('admin.settingsHelp.requireEmailVerify'), type: 'boolean' },
  ];
  const smtpSettingFields = [
    { key: 'smtp_host', label: t('admin.settings.smtpHost'), help: t('admin.settingsHelp.smtpHost') },
    { key: 'smtp_port', label: t('admin.settings.smtpPort'), help: t('admin.settingsHelp.smtpPort') },
    { key: 'smtp_user', label: t('admin.settings.smtpUser'), help: t('admin.settingsHelp.smtpUser') },
    { key: 'smtp_pass', label: t('admin.settings.smtpPass'), help: t('admin.settingsHelp.smtpPass'), type: 'password' },
    { key: 'smtp_from_name', label: t('admin.settings.smtpFromName'), help: t('admin.settingsHelp.smtpFromName') },
    { key: 'smtp_from', label: t('admin.settings.smtpFrom'), help: t('admin.settingsHelp.smtpFrom') },
  ];
  const reviewSummary = useMemo(() => {
    return {
      total: reviewPagination.totalItems || reviews.length,
      appSubmissions: reviews.filter((review) => review.kind === 'APP_SUBMISSION').length,
      versionUploads: reviews.filter((review) => review.kind === 'VERSION_UPLOAD').length,
      infoUpdates: reviews.filter((review) => review.kind === 'APP_INFO_UPDATE').length,
    };
  }, [reviews, reviewPagination.totalItems]);
  const reviewAppByID = useMemo(() => new Map(reviewApps.map((item) => [item.id, item])), [reviewApps]);
  const catalogReady = approvedAppCount > 0 && adminCategories.length > 0;
  const sourceProtected = isSiteAdmin ? Boolean(settings.source_password?.trim()) : true;
  const reviewOpsBody = reviewSummary.total === 0
    ? t('admin.opsReviewClear')
    : t('admin.opsReviewPending', { count: reviewSummary.total });
  const catalogOpsBody =
    approvedAppCount === 0
      ? t('admin.opsCatalogNeedsApps')
      : adminCategories.length === 0
        ? t('admin.opsCatalogNeedsCategories', { apps: approvedAppCount })
        : t('admin.opsCatalogReady', { apps: approvedAppCount, categories: adminCategories.length, collections: adminCollections.length });
  const sourceOpsBody = !isSiteAdmin
    ? t('admin.opsSourceDelegated')
    : sourceProtected
      ? t('admin.opsSourceProtected')
      : t('admin.opsSourceOpen');
  const adminPublicURL = stripTrailingSlash(settings.site_public_url || window.location.origin);
  const adminSourceURL = adminPublicURL ? `${adminPublicURL}/source/v2/index.json` : '';
  const adminStorageOptions = useMemo(() => storageOptionsFromRecords(storageRecords, defaultStorageKey), [storageRecords, defaultStorageKey]);
  const adminStorageChoices = useMemo(() => storageSelectOptions(adminStorageOptions), [adminStorageOptions]);
  const categoryTree = useMemo(() => flattenCategoryTree(adminCategories), [adminCategories]);
  const editingCategory = editingCategoryID === null ? null : adminCategories.find((item) => item.id === editingCategoryID) || null;
  const editingCategoryDraft = editingCategory ? categoryDrafts[editingCategory.id] || taxonomyDraft(editingCategory) : null;
  const editingTag = editingTagID === null ? null : adminTags.find((item) => item.id === editingTagID) || null;
  const editingTagDraft = editingTag ? tagDrafts[editingTag.id] || taxonomyDraft(editingTag) : null;
  const legacyAnnouncementPreview: SiteAnnouncement = {
    enabled: settings.announcement_enabled === 'true',
    level: settings.announcement_level === 'warning' || settings.announcement_level === 'success' ? settings.announcement_level : 'info',
    title: settings.announcement_title,
    body: settings.announcement_body,
    linkLabel: settings.announcement_link_label,
    linkUrl: settings.announcement_link_url,
    updatedAt: settings.announcement_updated_at,
  };
  const announcementPreview = announcements.find((item) => item.enabled && (item.title || item.body)) || legacyAnnouncementPreview;

  useEffect(() => {
    void reload();
  }, []);

  useEffect(() => {
    const selected = storageRecords.find((storage) => storage.key === selectedStorageKey) || storageRecords[0] || defaultStorageSettings;
    const selectionChanged = storageSelectionRef.current !== selectedStorageKey;
    if (selectionChanged || (storageSaveStatus !== 'dirty' && storageSaveStatus !== 'error')) {
      const nextDraft = { ...defaultStorageSettings, ...selected };
      storageDraftRef.current = nextDraft;
      setStorageDraft(nextDraft);
    }
    if (selectionChanged) setStorageSaveStatus('idle');
    storageSelectionRef.current = selectedStorageKey;
  }, [selectedStorageKey, storageRecords]);

  useEffect(() => {
    setSiteIconStorageKey((current) => (adminStorageOptions.some((storage) => storage.key === current) ? current : defaultUploadStorageKey(adminStorageOptions)));
  }, [adminStorageOptions]);

  useEffect(() => {
    void loadReviewAppsForReviews();
  }, [reviews]);

  useEffect(() => {
    if (adminTab === 'collections') {
      void loadCollectionApps();
    }
  }, [adminTab]);

  function setLoadedStorageRecords(storages: StorageSettings[], defaultKey: string) {
    const normalized = (storages.length > 0 ? storages : [defaultStorageSettings]).map(normalizeStorageRecord);
    const nextDefaultKey = defaultKey || normalized.find((storage) => storage.isDefault)?.key || normalized[0]?.key || defaultStorageSettings.key;
    setStorageRecords(normalized.map((storage) => ({ ...storage, isDefault: storage.key === nextDefaultKey })));
    setDefaultStorageKey(nextDefaultKey);
    setSelectedStorageKey((current) => (normalized.some((storage) => storage.key === current) ? current : nextDefaultKey));
  }

  function paginatedPath(path: string, page: number, pageSize?: number) {
    const params = new URLSearchParams({ page: String(page || 1) });
    if (pageSize && pageSize > 0) params.set('pageSize', String(pageSize));
    return `${path}?${params.toString()}`;
  }

  async function fetchUsersPage(page = userPagination.page || 1, pageSize = userPagination.pageSize) {
    const data = await api<PaginatedResponse<User, 'users'>>(paginatedPath('/api/v1/admin/users', page, pageSize));
    setUsers(data.users || []);
    setUserPagination(data.pagination || { page, pageSize, totalItems: data.users?.length || 0, totalPages: 1 });
  }

  async function fetchInvitesPage(page = invitePagination.page || 1, pageSize = invitePagination.pageSize) {
    const data = await api<PaginatedResponse<RegistrationInvite, 'invites'>>(paginatedPath('/api/v1/admin/registration-invites', page, pageSize));
    setRegistrationInvites(data.invites || []);
    setInvitePagination(data.pagination || { page, pageSize, totalItems: data.invites?.length || 0, totalPages: 1 });
  }

  async function loadUsersPage(page: number, pageSize?: number) {
    await runAction(setToast, t('admin.loadFailed'), async () => {
      await fetchUsersPage(page, pageSize);
    });
  }

  async function loadInvitesPage(page: number, pageSize?: number) {
    await runAction(setToast, t('admin.loadFailed'), async () => {
      await fetchInvitesPage(page, pageSize);
    });
  }

  async function loadReviewAppsForReviews() {
    const ids = Array.from(new Set(reviews.map((review) => review.appId).filter((id): id is number => Boolean(id))));
    if (ids.length === 0) {
      setReviewApps([]);
      return;
    }
    try {
      const data = await fetchAllPaginated<StoreApp, 'apps'>(api, `/api/v1/apps?managed=1&ids=${encodeURIComponent(ids.join(','))}`, 'apps');
      setReviewApps(data);
    } catch {
      setReviewApps([]);
    }
  }

  async function loadCollectionApps() {
    await runAction(setToast, t('admin.loadFailed'), async () => {
      const data = await fetchAllPaginated<StoreApp, 'apps'>(api, '/api/v1/apps?managed=1&status=APPROVED', 'apps');
      setApps(data);
      setApprovedAppCount(data.length);
    });
  }

  type ReloadOptions = { refreshSettings?: boolean };

  async function reload({ refreshSettings }: ReloadOptions = {}) {
    const settingsRevision = settingsRevisionRef.current;
    const settingsRequestID = ++settingsRequestSequenceRef.current;
    const shouldRefreshSettings = (refreshSettings ?? areAdminDraftsEqual(settingsRef.current, savedSettingsRef.current))
      && !settingsSaveInFlightRef.current;
    await runAction(setToast, t('admin.loadFailed'), async () => {
      const [categoryData, tagData, collectionData, appCountData] = await Promise.all([
        api<{ categories: Category[] }>('/api/v1/admin/categories'),
        api<{ tags: TagRecord[] }>('/api/v1/admin/tags'),
        api<{ collections: Collection[] }>('/api/v1/admin/collections'),
        api<PaginatedResponse<StoreApp, 'apps'>>('/api/v1/apps?managed=1&status=APPROVED&pageSize=1'),
      ]);
      setAdminCategories(categoryData.categories);
      setAdminTags(tagData.tags);
      setAdminCollections(collectionData.collections);
      setApprovedAppCount(appCountData.pagination?.totalItems || appCountData.apps?.length || 0);
      setCategoryDrafts({});
      setTagDrafts({});
      setCollectionDrafts({});
      if (isSiteAdmin) {
        const settingsPromise = shouldRefreshSettings
          ? api<{ settings: Record<string, string> }>('/api/v1/admin/settings')
          : Promise.resolve(null);
        const [settingData, storageData, announcementData, adData] = await Promise.all([
          settingsPromise,
          api<{ storages: StorageSettings[]; defaultKey: string }>('/api/v1/admin/storage'),
          fetchAllPaginated<SiteAnnouncement, 'announcements'>(api, '/api/v1/admin/announcements', 'announcements'),
          fetchAllPaginated<SiteAd, 'ads'>(api, '/api/v1/admin/ads', 'ads'),
        ]);
        if (
          settingData
          && settingsRequestID === settingsRequestSequenceRef.current
          && settingsRevision === settingsRevisionRef.current
          && !settingsSaveInFlightRef.current
        ) {
          const nextSettings = settingData.settings || {};
          settingsRef.current = nextSettings;
          savedSettingsRef.current = nextSettings;
          setSettings(nextSettings);
          setSavedSettings(nextSettings);
          setSettingsSaveStatus('idle');
        }
        setLoadedStorageRecords(storageData.storages || [], storageData.defaultKey);
        setAnnouncements(announcementData || []);
        setAds(adData || []);
        await Promise.all([fetchUsersPage(), fetchInvitesPage()]);
      }
    });
  }

  function summarizeReviewNote(note?: string) {
    if (!note?.trim()) return '';
    try {
      const parsed = JSON.parse(note);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
	        const fields = Object.entries(parsed)
	          .filter(([key, value]) => key !== 'submitForReview' && value !== undefined && value !== null && value !== '')
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

  async function saveSettings(event?: FormEvent) {
    event?.preventDefault();
    if (settingsSaveInFlightRef.current || areAdminDraftsEqual(settingsRef.current, savedSettingsRef.current)) return;
    settingsSaveInFlightRef.current = true;
    const settingsSaveRequestID = ++settingsSaveRequestSequenceRef.current;
    ++settingsRequestSequenceRef.current;
    const settingsSnapshot = settingsRef.current;
    const settingsRevision = settingsRevisionRef.current;
    setSettingsSaveStatus('saving');
    try {
      await api('/api/v1/admin/settings', { method: 'PATCH', body: JSON.stringify(settingsSnapshot) });
      const settingData = await api<{ settings: Record<string, string> }>('/api/v1/admin/settings', { method: 'GET' });
      if (settingsSaveRequestID !== settingsSaveRequestSequenceRef.current) return;
      const normalizedSettings = settingData.settings || {};
      const draftUnchanged = settingsRevision === settingsRevisionRef.current;
      savedSettingsRef.current = normalizedSettings;
      setSavedSettings(normalizedSettings);
      if (draftUnchanged) {
        settingsRef.current = normalizedSettings;
        setSettings(normalizedSettings);
        setSettingsSaveStatus('saved');
      } else {
        setSettingsSaveStatus('dirty');
      }
      settingsSaveInFlightRef.current = false;
      setToast({ tone: 'success', message: t('admin.settingsSaved') });
    } catch (error) {
      if (settingsSaveRequestID !== settingsSaveRequestSequenceRef.current) return;
      settingsSaveInFlightRef.current = false;
      setSettingsSaveStatus('error');
      setToast({ tone: 'error', message: errorMessage(error, t('admin.settingsSaveFailed')) });
      return;
    }
    try {
      await onSiteProfileSaved();
    } catch (error) {
      if (settingsSaveRequestID === settingsSaveRequestSequenceRef.current) {
        setToast({ tone: 'neutral', message: `${t('admin.settingsSaved')} · ${errorMessage(error, t('admin.loadFailed'))}` });
      }
    }
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

  function startStorageAction(action: Exclude<AdminStorageAction, null>) {
    if (storageActionRef.current) return false;
    storageActionRef.current = action;
    setStorageAction(action);
    return true;
  }

  function finishStorageAction() {
    storageActionRef.current = null;
    setStorageAction(null);
  }

  async function notifyStorageOptionsChanged(successMessage: string) {
    try {
      await onStorageOptionsChanged();
    } catch (error) {
      setToast({ tone: 'neutral', message: `${successMessage} · ${errorMessage(error, t('admin.loadFailed'))}` });
    }
  }

  async function saveStorageSettings() {
    if (!storageDirty || !startStorageAction('save')) return;
    const storageSnapshot = storageDraftRef.current;
    const storageRevision = storageRevisionRef.current;
    setStorageSaveStatus('saving');
    try {
      const data = await api<{ storage: StorageSettings }>(`/api/v1/admin/storage/${encodeURIComponent(storageSnapshot.key)}`, {
        method: 'PATCH',
        body: JSON.stringify(storageSettingsPayload(storageSnapshot)),
      });
      const saved = normalizeStorageRecord(data.storage);
      setStorageRecords((current) => current.map((item) => (item.key === saved.key ? saved : item)));
      if (storageRevision === storageRevisionRef.current) {
        storageDraftRef.current = saved;
        setStorageDraft(saved);
        setStorageSaveStatus('saved');
      } else {
        setStorageSaveStatus('dirty');
      }
      setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: saved.name || saved.key });
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await notifyStorageOptionsChanged(t('admin.storageSaved'));
    } catch (error) {
      const message = errorMessage(error, t('admin.storageSaveFailed'));
      setStorageSaveStatus(storageRevision === storageRevisionRef.current ? 'error' : 'dirty');
      setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: storageSnapshot.name || storageSnapshot.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
  }

  async function testStorageSettings() {
    if (!startStorageAction('test')) return;
    const storageSnapshot = storageDraftRef.current;
    try {
      await api('/api/v1/admin/storage/test', { method: 'POST', body: JSON.stringify(storageSettingsPayload(storageSnapshot)) });
      setStorageResult({ variant: 'success', title: t('admin.testStorage'), message: t('admin.storageTested'), occurredAt: new Date().toISOString(), target: storageSnapshot.name || storageSnapshot.key });
      setToast({ tone: 'success', message: t('admin.storageTested') });
    } catch (error) {
      const message = errorMessage(error, t('admin.storageTestFailed'));
      setStorageResult({ variant: 'error', title: t('admin.testStorage'), message, occurredAt: new Date().toISOString(), target: storageSnapshot.name || storageSnapshot.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
  }

  async function createStorage() {
    if (!startStorageAction('create')) return;
    try {
      const data = await api<{ storage: StorageSettings }>('/api/v1/admin/storage', {
        method: 'POST',
        body: JSON.stringify(storageSettingsPayload(storageCreateDraft)),
      });
      const next = normalizeStorageRecord(data.storage);
      setStorageRecords((current) => [next, ...current.filter((item) => item.key !== next.key)]);
      setSelectedStorageKey(next.key);
      setStorageCreateDraft({ ...defaultStorageSettings, key: '', name: '' });
      setIsStorageCreateOpen(false);
      setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: next.name || next.key });
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await notifyStorageOptionsChanged(t('admin.storageSaved'));
    } catch (error) {
      const message = errorMessage(error, t('admin.storageSaveFailed'));
      setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: storageCreateDraft.name || storageCreateDraft.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
  }

  async function testSavedStorage(storage: StorageSettings) {
    if (!startStorageAction('test')) return;
    try {
      await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/test`, { method: 'POST' });
      setStorageResult({ variant: 'success', title: t('admin.testStorage'), message: t('admin.storageTested'), occurredAt: new Date().toISOString(), target: storage.name || storage.key });
      setToast({ tone: 'success', message: t('admin.storageTested') });
    } catch (error) {
      const message = errorMessage(error, t('admin.storageTestFailed'));
      setStorageResult({ variant: 'error', title: t('admin.testStorage'), message, occurredAt: new Date().toISOString(), target: storage.name || storage.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
  }

  async function setDefaultStorage(storage: StorageSettings) {
    if (!startStorageAction('default')) return;
    try {
      await api(`/api/v1/admin/storage/${encodeURIComponent(storage.key)}/default`, { method: 'POST' });
      setDefaultStorageKey(storage.key);
      setStorageRecords((current) => current.map((item) => ({ ...item, isDefault: item.key === storage.key })));
      setStorageResult({ variant: 'success', title: t('admin.defaultStoragePicker'), message: t('admin.storageSaved'), occurredAt: new Date().toISOString(), target: storage.name || storage.key });
      setToast({ tone: 'success', message: t('admin.storageSaved') });
      await notifyStorageOptionsChanged(t('admin.storageSaved'));
    } catch (error) {
      const message = errorMessage(error, t('admin.storageSaveFailed'));
      setStorageResult({ variant: 'error', title: t('admin.defaultStoragePicker'), message, occurredAt: new Date().toISOString(), target: storage.name || storage.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
  }

  async function confirmDeleteStorage() {
    if (!storageToDelete || !startStorageAction('delete')) return;
    const deletingStorage = storageToDelete;
    try {
      await api(`/api/v1/admin/storage/${encodeURIComponent(deletingStorage.key)}`, { method: 'DELETE' });
      setStorageResult({ variant: 'success', title: t('admin.storageSettings'), message: t('admin.storageDeleted'), occurredAt: new Date().toISOString(), target: deletingStorage.name || deletingStorage.key });
      setToast({ tone: 'neutral', message: t('admin.storageDeleted') });
      setStorageToDelete(null);
      setStorageRecords((current) => current.filter((item) => item.key !== deletingStorage.key));
      await notifyStorageOptionsChanged(t('admin.storageDeleted'));
    } catch (error) {
      const message = errorMessage(error, t('admin.storageDeleteFailed'));
      setStorageResult({ variant: 'error', title: t('admin.storageSettings'), message, occurredAt: new Date().toISOString(), target: deletingStorage.name || deletingStorage.key });
      setToast({ tone: 'error', message });
    } finally {
      finishStorageAction();
    }
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
      await api<{ invite: RegistrationInvite; code: string }>('/api/v1/admin/registration-invites', {
        method: 'POST',
        body: JSON.stringify({
          note: inviteDraft.note,
          maxUses: Number.isFinite(maxUses) ? maxUses : 1,
        }),
      });
      setInviteDraft({ note: '', maxUses: '1' });
      setIsInviteCreateOpen(false);
      setToast({ tone: 'success', message: t('admin.inviteCreated') });
      await fetchInvitesPage(1, invitePagination.pageSize);
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

  function updateSetting(key: string, value: string) {
    const nextSettings = { ...settingsRef.current, [key]: value };
    settingsRef.current = nextSettings;
    settingsRevisionRef.current += 1;
    setSettings(nextSettings);
    if (!settingsSaveInFlightRef.current) {
      setSettingsSaveStatus('dirty');
    }
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
        <XSwitch
          key={field.key}
          label={field.label}
          description={field.help}
          value={settings[field.key] === 'true'}
          labelPosition="start"
          labelSpacing="spread"
          width="100%"
          onChange={(checked) => updateSetting(field.key, checked ? 'true' : 'false')}
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
    return { name: '', nameI18n: { 'zh-CN': '', en: '' }, slug: '', parentId: '', sortOrder: '0' };
  }

  function taxonomyDraft(item: Category | TagRecord): TaxonomyDraft {
    return {
      name: item.name,
      nameI18n: {
        'zh-CN': item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || '',
        en: item.nameI18n?.en || '',
      },
      slug: item.slug,
      parentId: 'parentId' in item && item.parentId ? String(item.parentId) : '',
      sortOrder: 'sortOrder' in item ? String(item.sortOrder || 0) : '0',
    };
  }

  function updateTaxonomyI18n(draft: TaxonomyDraft, language: string, value: string): TaxonomyDraft {
    return { ...draft, nameI18n: { ...draft.nameI18n, [language]: value } };
  }

  function taxonomyTextPayload(draft: TaxonomyDraft) {
    return {
      name: draft.name,
      nameI18n: draft.nameI18n,
      slug: draft.slug,
    };
  }

  function categoryPayload(draft: TaxonomyDraft) {
    const sortOrder = Number.parseInt(draft.sortOrder || '0', 10);
    return {
      ...taxonomyTextPayload(draft),
      parentId: draft.parentId ? Number(draft.parentId) : null,
      sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
    };
  }

  function categoryParentOptions(excludeID?: number) {
    const descendantIDs = excludeID ? categoryDescendantIds(adminCategories, excludeID) : new Set<number>();
    const hasChildren = excludeID ? adminCategories.some((category) => category.parentId === excludeID) : false;
    return [
      { value: '', label: t('admin.noParentCategory') },
      ...categoryTree
        .filter((item) => !hasChildren && item.depth === 0 && item.category.id !== excludeID && !descendantIDs.has(item.category.id))
        .map((item) => ({ value: String(item.category.id), label: item.path })),
    ];
  }

  function categoryPath(category: Category) {
    return categoryTree.find((item) => item.category.id === category.id)?.path || localizedName(category);
  }

  function openCategoryEditor(item: Category) {
    setCategoryDrafts((current) => ({ ...current, [item.id]: current[item.id] || taxonomyDraft(item) }));
    setEditingCategoryID(item.id);
  }

  function closeCategoryEditor() {
    setCategoryDrafts((current) => {
      if (editingCategoryID === null) return current;
      const next = { ...current };
      delete next[editingCategoryID];
      return next;
    });
    setEditingCategoryID(null);
  }

  function openTagEditor(item: TagRecord) {
    setTagDrafts((current) => ({ ...current, [item.id]: current[item.id] || taxonomyDraft(item) }));
    setEditingTagID(item.id);
  }

  function closeTagEditor() {
    setTagDrafts((current) => {
      if (editingTagID === null) return current;
      const next = { ...current };
      delete next[editingTagID];
      return next;
    });
    setEditingTagID(null);
  }

  function categoryTreeItems(): TreeListItemData[] {
    const childrenByParent = new Map<number | null, Category[]>();
    for (const item of adminCategories) {
      const parentID = item.parentId && adminCategories.some((category) => category.id === item.parentId) ? item.parentId : null;
      childrenByParent.set(parentID, [...(childrenByParent.get(parentID) || []), item]);
    }
    for (const [parentID, children] of childrenByParent.entries()) {
      childrenByParent.set(parentID, [...children].sort((left, right) => {
        const sortDelta = (left.sortOrder || 0) - (right.sortOrder || 0);
        if (sortDelta !== 0) return sortDelta;
        return localizedName(left).localeCompare(localizedName(right), undefined, { numeric: true, sensitivity: 'base' });
      }));
    }

    const build = (parentID: number | null): TreeListItemData[] => (childrenByParent.get(parentID) || []).map((item) => {
      return {
        id: String(item.id),
        isExpanded: true,
        startContent: <Tag size={16} />,
        label: (
          <span className="taxonomy-tree-label">
            <strong>{localizedName(item)}</strong>
            <small>{item.slug}</small>
          </span>
        ),
        description: t('admin.categoryMeta', {
          parent: item.parentId ? categoryPath(adminCategories.find((category) => category.id === item.parentId) || item) : t('admin.noParentCategory'),
          sort: item.sortOrder || 0,
          zh: item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || item.name || '-',
          en: item.nameI18n?.en || '-',
        }),
        endContent: (
          <div className="row-actions">
            <XIconButton type="button" variant="ghost" label={t('admin.editCategoryNamed', { name: item.name })} icon={<Pencil size={16} />} onClick={() => openCategoryEditor(item)} />
            <XIconButton type="button" variant="destructive" label={t('admin.deleteCategoryNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => setDeleteTarget({ kind: 'category', item })} />
          </div>
        ),
        children: build(item.id),
      };
    });

    return build(null);
  }

  async function refreshTaxonomyCatalog() {
    await reload();
    await onCatalogMetadataChanged();
  }

  async function createCategory(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.categoryCreateFailed'), async () => {
      await api('/api/v1/admin/categories', { method: 'POST', body: JSON.stringify(categoryPayload(categoryForm)) });
      setCategoryForm(emptyTaxonomyDraft());
      setTaxonomyCreateMode(null);
      setToast({ tone: 'success', message: t('admin.categoryCreated') });
      await refreshTaxonomyCatalog();
    });
  }

  async function updateCategory(item: Category) {
    const draft = categoryDrafts[item.id] || taxonomyDraft(item);
    await runAction(setToast, t('admin.categoryUpdateFailed'), async () => {
      await api(`/api/v1/admin/categories/${item.id}`, { method: 'PATCH', body: JSON.stringify(categoryPayload(draft)) });
      setToast({ tone: 'success', message: t('admin.categoryUpdated') });
      setEditingCategoryID(null);
      setCategoryDrafts((current) => {
        const next = { ...current };
        delete next[item.id];
        return next;
      });
      await refreshTaxonomyCatalog();
    });
  }

  async function createTag(event: FormEvent) {
    event.preventDefault();
    await runAction(setToast, t('admin.tagCreateFailed'), async () => {
      await api('/api/v1/admin/tags', { method: 'POST', body: JSON.stringify(taxonomyTextPayload(tagForm)) });
      setTagForm(emptyTaxonomyDraft());
      setTaxonomyCreateMode(null);
      setToast({ tone: 'success', message: t('admin.tagCreated') });
      await refreshTaxonomyCatalog();
    });
  }

  async function updateTag(item: TagRecord) {
    const draft = tagDrafts[item.id] || taxonomyDraft(item);
    await runAction(setToast, t('admin.tagUpdateFailed'), async () => {
      await api(`/api/v1/admin/tags/${item.id}`, { method: 'PATCH', body: JSON.stringify(taxonomyTextPayload(draft)) });
      setToast({ tone: 'success', message: t('admin.tagUpdated') });
      setEditingTagID(null);
      setTagDrafts((current) => {
        const next = { ...current };
        delete next[item.id];
        return next;
      });
      await refreshTaxonomyCatalog();
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

  async function confirmAdminDelete() {
    if (!deleteTarget || isDeleting) return;
    const target = deleteTarget;
    setIsDeleting(true);
    try {
      switch (target.kind) {
        case 'user':
          await api(`/api/v1/admin/users/${target.item.id}`, { method: 'DELETE' });
          break;
        case 'category':
          await api(`/api/v1/admin/categories/${target.item.id}`, { method: 'DELETE' });
          break;
        case 'tag':
          await api(`/api/v1/admin/tags/${target.item.id}`, { method: 'DELETE' });
          break;
        case 'collection':
          await api(`/api/v1/admin/collections/${target.item.id}`, { method: 'DELETE' });
          break;
        case 'invite':
          await api(`/api/v1/admin/registration-invites/${target.item.id}`, { method: 'DELETE' });
          break;
      }
      setDeleteTarget(null);
      const successMessage = target.kind === 'user'
        ? t('admin.userDeleted')
        : target.kind === 'category'
          ? t('admin.categoryDeleted')
          : target.kind === 'tag'
            ? t('admin.tagDeleted')
            : target.kind === 'collection'
              ? t('admin.collectionDeleted')
              : t('admin.inviteDeleted');
      setToast({ tone: 'neutral', message: successMessage });
      try {
        switch (target.kind) {
          case 'user':
            await fetchUsersPage(userPagination.page, userPagination.pageSize);
            break;
          case 'category': {
            const data = await api<{ categories: Category[] }>('/api/v1/admin/categories');
            setAdminCategories(data.categories || []);
            await onCatalogMetadataChanged();
            break;
          }
          case 'tag': {
            const data = await api<{ tags: TagRecord[] }>('/api/v1/admin/tags');
            setAdminTags(data.tags || []);
            await onCatalogMetadataChanged();
            break;
          }
          case 'collection': {
            const data = await api<{ collections: Collection[] }>('/api/v1/admin/collections');
            setAdminCollections(data.collections || []);
            break;
          }
          case 'invite':
            await fetchInvitesPage(invitePagination.page, invitePagination.pageSize);
            break;
        }
      } catch (refreshError) {
        setToast({ tone: 'neutral', message: `${successMessage} · ${errorMessage(refreshError, t('admin.loadFailed'))}` });
      }
    } catch (error) {
      const fallback = target.kind === 'user'
        ? t('admin.userDeleteFailed')
        : target.kind === 'category'
          ? t('admin.categoryDeleteFailed')
          : target.kind === 'tag'
            ? t('admin.tagDeleteFailed')
            : target.kind === 'collection'
              ? t('admin.collectionDeleteFailed')
              : t('admin.inviteDeleteFailed');
      setToast({ tone: 'error', message: errorMessage(error, fallback) });
    } finally {
      setIsDeleting(false);
    }
  }

  function deleteDialogCopy(target: AdminDeleteTarget) {
    switch (target.kind) {
      case 'user': {
        const name = displayUserName(target.item);
        return { title: t('admin.deleteUserNamed', { name }), subject: name, consequence: t('admin.confirmDeleteUser', { name }) };
      }
      case 'category': {
        const name = localizedName(target.item);
        return { title: t('admin.deleteCategoryNamed', { name }), subject: name, consequence: t('admin.confirmDeleteCategory', { name }) };
      }
      case 'tag': {
        const name = localizedName(target.item);
        return { title: t('admin.deleteTagNamed', { name }), subject: name, consequence: t('admin.confirmDeleteTag', { name }) };
      }
      case 'collection':
        return { title: t('admin.deleteCollectionNamed', { name: target.item.name }), subject: target.item.name, consequence: t('admin.confirmDeleteCollection', { name: target.item.name }) };
      case 'invite': {
        const code = target.item.code || target.item.codePrefix;
        return { title: t('admin.deleteInvite'), subject: target.item.note || code, consequence: t('admin.confirmDeleteInvite', { code }) };
      }
    }
  }

  function activeTaskHeader(): AdminTaskHeaderProps {
    switch (adminTab) {
      case 'reviews':
        return { icon: ShieldCheck, title: t('admin.reviewQueue'), body: reviewOpsBody, statusLabel: reviewSummary.total === 0 ? t('admin.opsReady') : t('admin.opsNeedsAction'), statusVariant: reviewSummary.total === 0 ? 'success' as const : 'warning' as const };
      case 'site':
        return { icon: Settings, title: t('admin.siteSettings'), body: t('admin.siteIdentityBody'), statusLabel: sourceProtected ? t('admin.opsReady') : t('admin.opsNeedsAction'), statusVariant: sourceProtected ? 'success' as const : 'warning' as const };
      case 'users':
        return { icon: Users, title: t('admin.userManagement'), body: t('admin.body'), statusLabel: String(userPagination.totalItems || users.length), statusVariant: 'neutral' as const };
      case 'taxonomy':
        return { icon: Tag, title: t('admin.categoriesAndTags'), body: t('admin.taxonomyHelp'), statusLabel: `${adminCategories.length + adminTags.length}`, statusVariant: adminCategories.length > 0 ? 'success' as const : 'warning' as const };
      case 'collections':
        return { icon: Layers3, title: t('admin.collectionList'), body: t('admin.collectionAppsHelp'), statusLabel: `${adminCollections.length}`, statusVariant: adminCollections.length > 0 ? 'success' as const : 'neutral' as const };
      case 'storage':
        return { icon: Server, title: t('admin.storageSettings'), body: t('admin.storageSettingsBody'), statusLabel: `${storageRecords.length}`, statusVariant: storageRecords.length > 0 ? 'success' as const : 'warning' as const };
      case 'backup':
        return { icon: CloudUpload, title: t('admin.backup.title'), body: t('admin.backup.body'), statusLabel: t('admin.siteSettingTabs.backup'), statusVariant: 'neutral' as const };
      case 'migration':
        return { icon: DatabaseBackup, title: t('admin.migration.title'), body: t('admin.migration.body'), statusLabel: t('admin.siteSettingTabs.migration'), statusVariant: 'warning' as const };
    }
  }

  const taskHeader = activeTaskHeader();
  const activeDeleteCopy = deleteTarget ? deleteDialogCopy(deleteTarget) : null;

  return (
    <section className="page-grid admin-shell">
      <div className="page-heading">
        <span className="eyebrow subtle">{t('admin.eyebrow')}</span>
        <h1>{t('admin.title')}</h1>
        <p>{t('admin.body')}</p>
      </div>
      <div className="horizontal-control-scroll admin-primary-tabs">
        <XTabList value={adminTab} onChange={(value) => setAdminTab(value as typeof adminTab)} hasDivider size="md">
          {adminTabs.map((item) => {
            const Icon = item.icon;
            return <XTab key={item.key} value={item.key} label={item.label} icon={<Icon size={17} />} />;
          })}
        </XTabList>
      </div>
      <AdminTaskHeader {...taskHeader} />
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
            value: approvedAppCount,
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
        {reviews.length === 0 ? (
          <EmptyState icon={ShieldCheck} title={t('admin.noPendingReviews')} body={t('admin.noPendingReviewsBody')} />
        ) : (
          <XList className="action-list" density="compact" hasDividers>
            {reviews.map((review) => {
              const reviewApp = review.appId ? reviewAppByID.get(review.appId) : undefined;
              const reviewVersion = reviewApp?.latestVersion;
              const noteSummary = summarizeReviewNote(review.note);
              return (
                <XListItem
                  className="review-workflow-row"
                  key={review.id}
                  label={reviewApp ? reviewApp.name : t('admin.unknownApp')}
                  description={(
                    <span className="action-list-description">
                      <span>
                        {t(`reviewKinds.${reviewKindKey(review.kind)}`)} · {t('admin.reviewTarget', { target: review.appId ? `#${review.appId}` : review.versionId ? `v#${review.versionId}` : '-' })} · {t('admin.requester', { id: review.requesterId })} · {formatDate(review.createdAt)}
                      </span>
                      {reviewApp && <small>{reviewApp.summary || reviewApp.latestVersion?.version || t('common.lpkApp')}</small>}
                      {reviewApp && (
                        <span className="review-facts">
                          {reviewVersion ? (
                            <>
                              <span>{t('admin.reviewArtifact', { source: reviewVersion.sourceType || '-', size: formatBytes(reviewVersion.fileSize) })}</span>
                              <span>{t('admin.reviewChecksum', { hash: shortSHA(reviewVersion.sha256) })}</span>
                            </>
                          ) : (
                            <span>{t('admin.reviewArtifactPending')}</span>
                          )}
                        </span>
                      )}
                      {noteSummary && <span className="review-note">{noteSummary}</span>}
                    </span>
                  )}
                  endContent={(
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
                  )}
                />
              );
            })}
          </XList>
        )}
        {reviewPagination.pageSize > 0 && reviewPagination.totalItems > reviewPagination.pageSize && (
          <XPagination
            className="list-pagination"
            page={reviewPagination.page}
            onChange={(page) => void onReviewPageChange(page, reviewPagination.pageSize)}
            totalItems={reviewPagination.totalItems}
            pageSize={reviewPagination.pageSize}
            pageSizeOptions={ADMIN_PAGE_SIZE_OPTIONS}
            onPageSizeChange={(pageSize) => void onReviewPageChange(1, pageSize)}
            variant="pages"
            size="sm"
            label={t('pagination.label')}
          />
        )}
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

            {siteSettingsTab !== 'announcement' && siteSettingsTab !== 'ads' ? (
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
                      <div className="site-icon-picker">
                        <FilePicker
                          label={t('admin.uploadSiteIcon')}
                          help={t('admin.uploadSiteIconHelp')}
                          value={siteIconFile}
                          accept=".png,.jpg,.jpeg,.webp"
                          onChange={(nextFile) => setSiteIconFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
                        />
                      </div>
                      <div className="site-icon-controls">
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
                    </div>
                    <XFormLayout>
                      {siteIdentityFields.map(renderSettingField)}
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
                      {registrationInvites.length === 0 ? (
                        <EmptyState icon={KeyRound} title={t('admin.noInvites')} body={t('admin.noInvitesBody')} />
                      ) : (
                        <XList className="action-list invite-list" density="compact" hasDividers>
                          {registrationInvites.map((invite) => (
                            <XListItem
                              className="invite-row"
                              key={invite.id}
                              label={invite.note || t('admin.inviteUntitled')}
                              description={(
                                <span className="action-list-description">
                                  <span>{t('admin.inviteUsage', { remaining: invite.remainingUses, max: invite.maxUses })} · {formatDate(invite.createdAt)}</span>
                                  <code className="invite-code-inline">{invite.code}</code>
                                </span>
                              )}
                              endContent={(
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
                                    onClick={() => setDeleteTarget({ kind: 'invite', item: invite })}
                                  />
                                </div>
                              )}
                            />
                          ))}
                        </XList>
                      )}
                      {invitePagination.pageSize > 0 && invitePagination.totalItems > invitePagination.pageSize && (
                        <XPagination
                          className="list-pagination"
                          page={invitePagination.page}
                          onChange={(page) => void loadInvitesPage(page, invitePagination.pageSize)}
                          totalItems={invitePagination.totalItems}
                          pageSize={invitePagination.pageSize}
                          pageSizeOptions={ADMIN_PAGE_SIZE_OPTIONS}
                          onPageSizeChange={(pageSize) => void loadInvitesPage(1, pageSize)}
                          variant="pages"
                          size="sm"
                          label={t('pagination.label')}
                        />
                      )}
                    </div>
                    {isInviteCreateOpen && (
                      <ModalLayer onClose={() => setIsInviteCreateOpen(false)} purpose="form">
                        <div
                          className="modal-panel form-panel invite-dialog"
                          aria-label={t('admin.createInvite')}
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
                      </ModalLayer>
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
                      <XButton className="test-email-action" type="button" variant="secondary" size="sm" label={t('admin.sendTestEmail')} icon={<MessageSquare size={17} />} onClick={() => void sendTestEmail()} />
                    </div>
                  </div>
                )}

                <AdminSaveBar
                  status={settingsSaveStatus}
                  isDirty={settingsDirty}
                  saveLabel={t('admin.saveSettings')}
                  isDisabled={settingsSaveInFlightRef.current}
                  onSave={() => void saveSettings()}
                />
              </form>
            ) : siteSettingsTab === 'announcement' ? (
              <div className="settings-tab-panel">
                <AdminAnnouncementsPanel
                  announcements={announcements}
                  onReload={reload}
                  onSiteProfileSaved={onSiteProfileSaved}
                  setToast={setToast}
                />
              </div>
            ) : (
              <div className="settings-tab-panel">
                <AdminAdsPanel
                  ads={ads}
                  onReload={reload}
                  onSiteProfileSaved={onSiteProfileSaved}
                  setToast={setToast}
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
            <AdSpot ads={ads} className="site-preview-ad" />
          </section>
        </section>
      )}
      {isSiteAdmin && adminTab === 'storage' && (
        <section className="workspace-pane admin-task-workspace">
          <StorageSettingsPanel
            storages={storageRecords}
            defaultKey={defaultStorageKey}
            selectedKey={selectedStorageKey}
            draft={storageDraft}
            createDraft={storageCreateDraft}
            isCreateOpen={isStorageCreateOpen}
            onSelect={setSelectedStorageKey}
            onDraftChange={(nextDraft) => {
              storageDraftRef.current = nextDraft;
              storageRevisionRef.current += 1;
              setStorageDraft(nextDraft);
              setStorageSaveStatus('dirty');
            }}
            onCreateDraftChange={setStorageCreateDraft}
            onOpenCreate={() => setIsStorageCreateOpen(true)}
            onCloseCreate={() => setIsStorageCreateOpen(false)}
            onCreate={createStorage}
            onSave={saveStorageSettings}
            onTestDraft={testStorageSettings}
            onTestSaved={testSavedStorage}
            onSetDefault={setDefaultStorage}
            onDelete={setStorageToDelete}
            isDirty={storageDirty}
            saveStatus={storageSaveStatus}
            activeAction={storageAction}
            result={storageResult}
          />
        </section>
      )}
      {storageToDelete && (
        <AdminDeleteDialog
          title={t('admin.deleteStorageNamed', { name: storageToDelete.name || storageToDelete.key })}
          subject={storageToDelete.name || storageToDelete.key}
          consequence={t('admin.confirmDeleteStorage', { name: storageToDelete.name || storageToDelete.key })}
          confirmLabel={t('common.delete')}
          isDeleting={storageAction === 'delete'}
          onCancel={() => setStorageToDelete(null)}
          onConfirm={() => void confirmDeleteStorage()}
        />
      )}
      {isSiteAdmin && adminTab === 'backup' && (
        <section className="panel admin-task-workspace">
          <AdminBackupPanel storages={storageRecords} setToast={setToast} />
        </section>
      )}
      {isSiteAdmin && adminTab === 'migration' && (
        <section className="panel admin-task-workspace">
          <AdminMigrationPanel api={api} setToast={setToast} />
        </section>
      )}
      {isSiteAdmin && adminTab === 'users' && (
        <AdminUsersWorkspace
          users={users}
          userPagination={userPagination}
          pageSizeOptions={ADMIN_PAGE_SIZE_OPTIONS}
          userDraft={userDraft}
          userDialogMode={userDialogMode}
          userRoleOptions={userRoleOptions}
          sourceURL={adminSourceURL}
          setUserDraft={setUserDraft}
          setUserDialogMode={setUserDialogMode}
          openCreateUserDialog={openCreateUserDialog}
          openEditUserDialog={openEditUserDialog}
          saveManagedUser={saveManagedUser}
          toggleUserDisabled={toggleUserDisabled}
          deleteManagedUser={(item) => setDeleteTarget({ kind: 'user', item })}
          isDeletingUserID={deleteTarget?.kind === 'user' && isDeleting ? deleteTarget.item.id : undefined}
          loadUsersPage={loadUsersPage}
          setToast={setToast}
        />
      )}
      {adminTab === 'taxonomy' && (
      <section className="workspace-pane taxonomy-workspace">
        {taxonomyCreateMode && (
          <ModalLayer onClose={() => setTaxonomyCreateMode(null)} purpose="form">
            <form
              className="modal-panel form-panel taxonomy-dialog"
              aria-label={taxonomyCreateMode === 'category' ? t('admin.createCategory') : t('admin.createTag')}
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
                  <XSelector label={t('admin.categoryParent')} value={categoryForm.parentId || ''} options={categoryParentOptions()} onChange={(value) => setCategoryForm({ ...categoryForm, parentId: value })} />
                  <XTextInput label={t('admin.categorySortOrder')} value={categoryForm.sortOrder || '0'} onChange={(value) => setCategoryForm({ ...categoryForm, sortOrder: value })} />
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
          </ModalLayer>
        )}
        {editingCategory && editingCategoryDraft && (
          <ModalLayer onClose={closeCategoryEditor} purpose="form">
            <form
              className="modal-panel form-panel taxonomy-dialog"
              aria-label={t('admin.editCategoryNamed', { name: localizedName(editingCategory) })}
              onSubmit={(event) => {
                event.preventDefault();
                void updateCategory(editingCategory);
              }}
            >
              <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={closeCategoryEditor} />
              <SectionTitle icon={Tag} title={t('admin.editCategoryNamed', { name: localizedName(editingCategory) })} />
              <XTextInput label={t('admin.categoryNameZhFor', { name: localizedName(editingCategory) })} value={editingCategoryDraft.nameI18n['zh-CN'] || ''} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: updateTaxonomyI18n(editingCategoryDraft, 'zh-CN', value) }))} />
              <XTextInput label={t('admin.categoryNameEnFor', { name: localizedName(editingCategory) })} value={editingCategoryDraft.nameI18n.en || ''} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: updateTaxonomyI18n(editingCategoryDraft, 'en', value) }))} />
              <XTextInput label={t('admin.categoryNameFor', { name: localizedName(editingCategory) })} value={editingCategoryDraft.name} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: { ...editingCategoryDraft, name: value } }))} />
              <XTextInput label={t('admin.categorySlugFor', { name: editingCategory.name })} value={editingCategoryDraft.slug} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: { ...editingCategoryDraft, slug: value } }))} />
              <XSelector label={t('admin.categoryParentFor', { name: localizedName(editingCategory) })} value={editingCategoryDraft.parentId || ''} options={categoryParentOptions(editingCategory.id)} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: { ...editingCategoryDraft, parentId: value } }))} />
              <XTextInput label={t('admin.categorySortOrderFor', { name: localizedName(editingCategory) })} value={editingCategoryDraft.sortOrder || '0'} onChange={(value) => setCategoryDrafts((current) => ({ ...current, [editingCategory.id]: { ...editingCategoryDraft, sortOrder: value } }))} />
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={closeCategoryEditor} />
                <XButton type="submit" variant="primary" label={t('admin.saveCategory')} icon={<Save size={18} />} />
              </div>
            </form>
          </ModalLayer>
        )}
        {editingTag && editingTagDraft && (
          <ModalLayer onClose={closeTagEditor} purpose="form">
            <form
              className="modal-panel form-panel taxonomy-dialog"
              aria-label={t('admin.editTagNamed', { name: localizedName(editingTag) })}
              onSubmit={(event) => {
                event.preventDefault();
                void updateTag(editingTag);
              }}
            >
              <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} onClick={closeTagEditor} />
              <SectionTitle icon={Tag} title={t('admin.editTagNamed', { name: localizedName(editingTag) })} />
              <XTextInput label={t('admin.tagNameZhFor', { name: localizedName(editingTag) })} value={editingTagDraft.nameI18n['zh-CN'] || ''} onChange={(value) => setTagDrafts((current) => ({ ...current, [editingTag.id]: updateTaxonomyI18n(editingTagDraft, 'zh-CN', value) }))} />
              <XTextInput label={t('admin.tagNameEnFor', { name: localizedName(editingTag) })} value={editingTagDraft.nameI18n.en || ''} onChange={(value) => setTagDrafts((current) => ({ ...current, [editingTag.id]: updateTaxonomyI18n(editingTagDraft, 'en', value) }))} />
              <XTextInput label={t('admin.tagNameFor', { name: localizedName(editingTag) })} value={editingTagDraft.name} onChange={(value) => setTagDrafts((current) => ({ ...current, [editingTag.id]: { ...editingTagDraft, name: value } }))} />
              <XTextInput label={t('admin.tagSlugFor', { name: editingTag.name })} value={editingTagDraft.slug} onChange={(value) => setTagDrafts((current) => ({ ...current, [editingTag.id]: { ...editingTagDraft, slug: value } }))} />
              <div className="dialog-actions">
                <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={18} />} onClick={closeTagEditor} />
                <XButton type="submit" variant="primary" label={t('admin.saveTag')} icon={<Save size={18} />} />
              </div>
            </form>
          </ModalLayer>
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
            {adminCategories.length === 0 ? (
              <EmptyState icon={Layers3} title={t('admin.noCategories')} body={t('admin.noCategoriesBody')} />
            ) : (
              <XTreeList className="taxonomy-tree" density="compact" items={categoryTreeItems()} />
            )}
          </section>

          <section className="panel">
            <SectionTitle icon={Tag} title={t('admin.tagList')} />
            <div className="review-list">
              {adminTags.length === 0 ? (
                <EmptyState icon={Tag} title={t('admin.noTags')} body={t('admin.noTagsBody')} />
              ) : adminTags.map((item) => {
                return (
                  <div className="taxonomy-row" key={item.id}>
                    <div className="taxonomy-summary">
                      <strong>{localizedName(item)}</strong>
                      <span>{item.slug}</span>
                      <small>{t('admin.taxonomyLanguages', { zh: item.nameI18n?.['zh-CN'] || item.nameI18n?.zh || item.name || '-', en: item.nameI18n?.en || '-' })}</small>
                    </div>
                    <div className="row-actions">
                      <XIconButton type="button" variant="ghost" label={t('admin.editTagNamed', { name: item.name })} icon={<Pencil size={16} />} onClick={() => openTagEditor(item)} />
                      <XIconButton type="button" variant="destructive" label={t('admin.deleteTagNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => setDeleteTarget({ kind: 'tag', item })} />
                    </div>
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
          <ModalLayer onClose={() => setIsCollectionCreateOpen(false)} purpose="form" width="min(720px, calc(100vw - 36px))" maxHeight="min(86vh, 780px)">
            <form
              className="modal-panel form-panel collection-dialog"
              aria-label={t('admin.createCollection')}
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
          </ModalLayer>
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
                    <XIconButton type="button" variant="destructive" label={t('admin.deleteCollectionNamed', { name: item.name })} icon={<Trash2 size={16} />} onClick={() => setDeleteTarget({ kind: 'collection', item })} />
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      </section>
      )}
      {deleteTarget && activeDeleteCopy && (
        <AdminDeleteDialog
          {...activeDeleteCopy}
          confirmLabel={t('common.delete')}
          isDeleting={isDeleting}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => void confirmAdminDelete()}
        />
      )}
    </section>
  );
}
