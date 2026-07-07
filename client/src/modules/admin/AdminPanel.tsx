import { type FormEvent, useEffect, useMemo, useState } from 'react';
import { Archive, Check, Copy, Download, Gauge, KeyRound, Layers3, MessageSquare, Pencil, Plus, Save, Server, Settings, ShieldCheck, Tag, Trash2, Upload, UserPlus, UserRound, Users, X } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard } from '@astryxdesign/core/Card';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Tab as XTab, TabList as XTabList } from '@astryxdesign/core/TabList';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { AnnouncementBanner } from '../../components/AnnouncementBanner';
import { UserAvatar } from '../../components/AppIcon';
import { api } from '../../shared/api';
import { CollectionAppPicker } from './CollectionAppPicker';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { FilePicker } from '../../shared/components/FilePicker';
import { RECOMMENDED_DOWNLOAD_MIRRORS, RECOMMENDED_RAW_MIRRORS, mirrorPresetText } from '../../shared/constants';
import type { Category, Collection, CollectionDraft, RegistrationInvite, Review, SiteAnnouncement, SiteProfile, StorageOption, StoreApp, TagRecord, Toast, User } from '../../shared/types';
import { cx, errorMessage, formatBytes, formatDate, localizedName, reviewKindKey, runAction, shortSHA, statusKey, stripTrailingSlash } from '../../shared/utils';
import { StorageSettingsPanel, defaultStorageSettings, type StorageSettings } from './StorageSettingsPanel';

type TaxonomyDraft = { name: string; nameI18n: Record<string, string>; slug: string };
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

function defaultUploadStorageKey(storages: StorageOption[]) {
  return storages.find((storage) => storage.isDefault)?.key || storages[0]?.key || 'primary';
}

function displayUserName(user: User | null | undefined) {
  return user?.nickname?.trim() || user?.username || '';
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
    emailVerified: user.emailVerified !== false,
    disabled: Boolean(user.disabled),
  };
}

export function AdminPanel({
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
