import {
  AlertCircle,
  Archive,
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Check,
  Download,
  Gauge,
  Heart,
  History,
  Link,
  MessageSquare,
  MessageSquareOff,
  Save,
  Settings,
  ShieldCheck,
  Star,
  Trash2,
  Upload,
  Users,
  X,
} from 'lucide-react';
import { BreadcrumbItem, Breadcrumbs } from '@astryxdesign/core/Breadcrumbs';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Card as XCard, type CardVariant } from '@astryxdesign/core/Card';
import { CheckboxInput as XCheckboxInput } from '@astryxdesign/core/CheckboxInput';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { List as XList, ListItem as XListItem } from '@astryxdesign/core/List';
import { MetadataList as XMetadataList, MetadataListItem as XMetadataListItem } from '@astryxdesign/core/MetadataList';
import { ProgressBar as XProgressBar } from '@astryxdesign/core/ProgressBar';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { type FormEvent, type ReactNode, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, apiWithUploadProgress } from '../../shared/api';
import { canUserManageApp, canUserUploadVersion, defaultUploadStorageKey, storageSelectOptions } from '../../shared/appHelpers';
import { EmptyState, SectionTitle } from '../../shared/components/Feedback';
import { ArtifactModeOption } from '../../shared/components/ArtifactModeOption';
import { FilePicker } from '../../shared/components/FilePicker';
import { ModalLayer } from '../../shared/components/ModalLayer';
import { StatusBadge } from '../../shared/components/StatusBadge';
import { TagTokenizer } from '../../shared/components/TagTokenizer';
import { VersionHistoryTable } from '../../shared/components/VersionHistoryTable';
import { AppIcon } from '../../components/AppIcon';
import { CommentList } from '../../components/CommentList';
import type { Category, CollaboratorRequest, Group, InstallOptions, Review, StorageOption, StoreApp, Toast, User } from '../../shared/types';
import { flattenCategoryTree } from '../../shared/categoryTree';
import { orderedScreenshots, screenshotDeviceLabel, usePreferredScreenshotDevice } from '../../shared/screenshotHelpers';
import {
  cx,
  formatBytes,
  formatDate,
  githubMirrorKindForURL,
  hasInstallableVersion,
  localizedAppDescription,
  localizedAppName,
  localizedAppSummary,
  localizedCategory,
  runAction,
  shortSHA,
} from '../../shared/utils';
import type { SubmissionProgress } from '../profile/AppSubmissionForm';

export type AppDetailMode = 'detail' | 'manage';
type ManagementDialog = 'app-info' | 'publish-version' | 'screenshots' | 'visibility' | 'collaborators';

export function AppDrawer({
  app,
  mode,
  onModeChange,
  user,
  groups,
  categories,
  tagOptions,
  storageOptions,
  chatEnabled,
  onClose,
  onInstall,
  onContactPublisher,
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
  chatEnabled: boolean;
  onClose: () => void;
  onInstall: (app: StoreApp, options?: InstallOptions) => void | Promise<void>;
  onContactPublisher?: (app: StoreApp) => void | Promise<void>;
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
  const [managementDialog, setManagementDialog] = useState<ManagementDialog | null>(null);
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
  const canContactPublisher = Boolean(chatEnabled && user && user.id !== app.ownerId);
  const isManageMode = mode === 'manage' && canOpenManagement;
  const canEditScreenshots = isManageMode && canMaintain;
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const versionFileInputRef = useRef<HTMLInputElement>(null);
  const drawerTitleId = `app-drawer-title-${app.id}`;
  const latestVersion = app.latestVersion;
  const appName = localizedAppName(app);
  const appSummary = localizedAppSummary(app, localizedAppDescription(app, t('common.lpkApp')));
  const installable = hasInstallableVersion(app);
  const hasChecksum = Boolean(latestVersion?.sha256);
  const hasFileSize = Boolean(latestVersion && latestVersion.fileSize > 0);
  const trustState: 'ready' | 'caution' | 'blocked' = !installable ? 'blocked' : hasChecksum && hasFileSize ? 'ready' : 'caution';
  const trustCardVariant: CardVariant = trustState === 'ready' ? 'green' : trustState === 'caution' ? 'yellow' : 'red';
  const categoryOptions = flattenCategoryTree(categories).map((item) => ({ value: String(item.category.id), label: item.path }));
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
  const appFavorited = Boolean(app.appFavorited);
  const submitterFavorited = Boolean(app.submitterFavorited);
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
  const canSubmitVersion = versionNumberReady && versionArtifactReady;
  const versionPublishesDirectly = user?.role === 'SITE_ADMIN' || user?.role === 'SOFTWARE_ADMIN' || app.allowUnreviewedUpdates;
  const storageChoices = storageSelectOptions(storageOptions);
  const appNeedsResubmission = app.status === 'REJECTED';

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
    setManagementDialog(null);
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
    if (!isManageMode) setManagementDialog(null);
  }, [isManageMode]);

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
          submitForReview: appNeedsResubmission,
          ...(installPassword || appForm.clearInstallPassword ? { installPassword } : {}),
        }),
      });
      setToast({
        tone: 'success',
        message: appNeedsResubmission
          ? data.review
            ? t('drawer.appResubmittedReview')
            : t('drawer.appResubmitted')
          : data.review
            ? t('drawer.appInfoSubmittedReview')
            : t('drawer.appInfoSaved'),
      });
      await onRefresh();
      setManagementDialog(null);
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
        setManagementDialog(null);
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
    if (!confirmDanger('unlist-app', t('drawer.confirmUnlist', { name: appName }))) return;
    await runAction(setToast, t('drawer.unlistFailed'), async () => {
      await api(`/api/v1/apps/${app.id}/unlist`, { method: 'POST' });
      setToast({ tone: 'neutral', message: t('drawer.unlisted') });
      await onRefresh();
    });
  }

  async function deleteApp() {
    if (!confirmDanger('delete-app', t('drawer.confirmDeleteApp', { name: appName }))) return;
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
      setManagementDialog(null);
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
      const data = await api<{ favorited: boolean; favorites?: number }>(`/api/v1/apps/${app.id}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: data.favorited ? t('drawer.appFavoriteAdded') : t('drawer.appFavoriteRemoved') });
      await onRefresh();
    });
  }

  async function toggleSubmitterFavorite() {
    await runAction(setToast, t('drawer.submitterFavoriteUpdateFailed'), async () => {
      const data = await api<{ favorited: boolean }>(`/api/v1/submitters/${app.ownerId}/favorites`, { method: 'POST' });
      setToast({ tone: 'success', message: data.favorited ? t('drawer.submitterFavoriteAdded') : t('drawer.submitterFavoriteRemoved') });
      await onRefresh();
    });
  }

  function renderScreenshotGallery(canEdit: boolean) {
    return (
      <>
        {displayScreenshots.length > 0 ? (
          <div className="screenshot-grid">
            {displayScreenshots.map((shot, index, shots) => (
              <figure className="screenshot-item" key={shot.id}>
                <img src={shot.imageUrl} alt={shot.caption || appName} />
                {canEdit ? (
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
                    <span>{shot.caption || appName}</span>
                    <small>{screenshotDeviceLabel(t, shot.deviceType)}</small>
                  </figcaption>
                )}
                {canEdit && (
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
        {canEdit && (
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
      </>
    );
  }

  function renderMetadataCard() {
    return (
      <XCard className="detail-metadata-card" padding={4}>
        <XMetadataList columns="multi">
          <XMetadataListItem label={t('drawer.latestVersion')}>
            {latestVersion?.version || t('app.noPublishedVersion')}
          </XMetadataListItem>
          <XMetadataListItem label={t('common.download')}>
            {t('app.downloads', { count: app.downloadCount })}
          </XMetadataListItem>
          <XMetadataListItem label={t('common.source')}>
            {latestVersion?.sourceType || '-'}
          </XMetadataListItem>
          <XMetadataListItem label={t('drawer.artifactSize')}>
            {latestVersion ? formatBytes(latestVersion.fileSize) : '-'}
          </XMetadataListItem>
          <XMetadataListItem label={t('common.checksum')}>
            {t('drawer.sha256', { hash: shortSHA(latestVersion?.sha256) })}
          </XMetadataListItem>
        </XMetadataList>
      </XCard>
    );
  }

  function renderManagementMetadataCard() {
    return (
      <XCard className="detail-metadata-card management-summary-card" padding={4}>
        <XMetadataList columns="multi">
          <XMetadataListItem label={t('drawer.latestVersion')}>
            {latestVersion?.version || t('app.noPublishedVersion')}
          </XMetadataListItem>
          <XMetadataListItem label={t('drawer.installStatus')}>
            {installable ? t('app.installReady') : t('app.installMissingVersion')}
          </XMetadataListItem>
          <XMetadataListItem label={t('drawer.outdatedStatus')}>
            {hasOutdatedMarks ? t('drawer.outdatedBadge', { count: app.outdatedMarks ?? 0 }) : t('drawer.outdatedInactiveTitle')}
          </XMetadataListItem>
          <XMetadataListItem label={t('common.source')}>
            {latestVersion?.sourceType || '-'}
          </XMetadataListItem>
        </XMetadataList>
      </XCard>
    );
  }

  function renderManagementDialogs() {
    if (!managementDialog) return null;

    if (managementDialog === 'app-info') {
      return (
        <ModalLayer purpose="form" width="min(760px, calc(100vw - 32px))" onClose={() => setManagementDialog(null)}>
          <form className="modal-panel app-info-panel" onSubmit={(event) => void submitAppInfo(event)}>
            <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
            <SectionTitle icon={Settings} title={t('drawer.appInfo')} />
            <XFormLayout>
              <XTextInput label={t('common.name')} value={appForm.name} onChange={(name) => setAppForm((current) => ({ ...current, name }))} />
              <XTextInput label={t('common.summary')} value={appForm.summary} onChange={(summary) => setAppForm((current) => ({ ...current, summary }))} />
              <XTextArea label={t('common.description')} value={appForm.description} rows={5} onChange={(description) => setAppForm((current) => ({ ...current, description }))} />
              <XSelector
                label={t('common.category')}
                value={appForm.categoryId}
                options={[
                  { value: '', label: t('common.uncategorized') },
                  ...categoryOptions,
                ]}
                onChange={(categoryId) => setAppForm((current) => ({ ...current, categoryId }))}
              />
              <TagTokenizer label={t('common.tags')} value={appForm.tags} knownTags={tagOptions} onChange={(tags) => setAppForm((current) => ({ ...current, tags }))} />
              <XTextInput
                type="password"
                label={t('drawer.installPassword')}
                description={app.installProtected ? t('drawer.installPasswordUpdateHelp') : t('drawer.installPasswordHelp')}
                value={appForm.installPassword}
                onChange={(installPassword) => setAppForm((current) => ({ ...current, installPassword }))}
              />
              {app.installProtected && (
                <XCheckboxInput
                  label={t('drawer.clearInstallPassword')}
                  value={appForm.clearInstallPassword}
                  onChange={(clearInstallPassword) => setAppForm((current) => ({ ...current, clearInstallPassword }))}
                />
              )}
              <XCheckboxInput
                label={t('drawer.commentsEnabled')}
                value={appForm.commentsEnabled}
                onChange={(commentsEnabled) => setAppForm((current) => ({ ...current, commentsEnabled }))}
              />
              <XCheckboxInput
                label={t('drawer.emailNotificationsEnabled')}
                value={appForm.emailNotificationsEnabled}
                onChange={(emailNotificationsEnabled) => setAppForm((current) => ({ ...current, emailNotificationsEnabled }))}
              />
              <XCheckboxInput
                label={t('submitApp.allowUnreviewedUpdates')}
                description={t('submitApp.allowUnreviewedUpdatesHelp')}
                value={appForm.allowUnreviewedUpdates}
                onChange={(allowUnreviewedUpdates) => setAppForm((current) => ({ ...current, allowUnreviewedUpdates }))}
              />
            </XFormLayout>
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
              <XButton type="submit" variant="primary" label={appNeedsResubmission ? t('drawer.resubmitApp') : t('drawer.saveInfo')} icon={<Save size={17} />} />
            </div>
          </form>
        </ModalLayer>
      );
    }

    if (managementDialog === 'publish-version') {
      return (
        <ModalLayer purpose="form" width="min(820px, calc(100vw - 32px))" onClose={() => setManagementDialog(null)}>
          <form className="modal-panel version-publish-panel" onSubmit={(event) => void submitExternalVersion(event)}>
            <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
            <SectionTitle icon={Upload} title={t('drawer.publishVersion')} />
            <XCard variant={versionPublishesDirectly ? 'green' : 'yellow'} padding={3} className="version-route-card">
              <strong>{t('drawer.versionPublishPath')}</strong>
              <span>{versionPublishesDirectly ? t('drawer.versionDirectHint') : t('drawer.versionReviewHint')}</span>
            </XCard>
            <div className="submission-readiness" aria-label={t('drawer.versionReadiness')}>
              <div className={cx('readiness-step', versionNumberReady && 'ready')}>
                <StatusBadge
                  tone={versionNumberReady ? 'approved' : 'unlisted'}
                  icon={versionNumberReady ? <Check size={14} /> : <AlertCircle size={14} />}
                  label={versionNumberReady ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
                />
                <strong>{t('drawer.readinessVersion')}</strong>
                <small>{versionNumberReady ? t('drawer.readinessVersionReady') : t('drawer.readinessVersionMissing')}</small>
              </div>
              <div className={cx('readiness-step', versionArtifactReady && 'ready')}>
                <StatusBadge
                  tone={versionArtifactReady ? 'approved' : 'unlisted'}
                  icon={versionArtifactReady ? <Check size={14} /> : <AlertCircle size={14} />}
                  label={versionArtifactReady ? t('submitApp.readinessReady') : t('submitApp.readinessNeedsAction')}
                />
                <strong>{t('submitApp.readinessArtifact')}</strong>
                <small>
                  {versionArtifactMode === 'local'
                    ? versionFile
                      ? t('submitApp.readinessArtifactLocalReady', { name: versionFile.name, size: formatBytes(versionFile.size) })
                      : t('submitApp.readinessArtifactLocalMissing')
                    : versionExternalDownloadReady
                      ? versionExternalChecksumReady
                        ? t('submitApp.readinessArtifactExternalReady')
                        : t('submitApp.readinessArtifactExternalPartial')
                      : t('submitApp.readinessArtifactExternalMissing')}
                </small>
              </div>
              <div className="readiness-step ready">
                <StatusBadge tone="synced" icon={<ShieldCheck size={14} />} label={versionPublishesDirectly ? t('submitApp.readinessDirect') : t('submitApp.readinessQueued')} />
                <strong>{t('submitApp.readinessReview')}</strong>
                <small>{versionPublishesDirectly ? t('drawer.readinessVersionDirect') : t('drawer.readinessVersionQueued')}</small>
              </div>
            </div>
            <XFormLayout>
              <XTextInput
                label={t('common.version')}
                description={t('drawer.versionRequired')}
                value={versionForm.version}
                onChange={(version) => setVersionForm((current) => ({ ...current, version }))}
              />
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
                <>
                  {storageChoices.length > 0 && (
                    <XSelector
                      label={t('common.storage')}
                      description={t('drawer.versionStorageHelp')}
                      value={versionStorageKey}
                      options={storageChoices}
                      onChange={setVersionStorageKey}
                    />
                  )}
                  <FilePicker
                    label={t('common.lpkFile')}
                    help={t('drawer.versionLocalFileHelp')}
                    value={versionFile}
                    inputRef={versionFileInputRef}
                    accept=".lpk"
                    required
                    onChange={(nextFile) => setVersionFile(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
                  />
                </>
              ) : (
                <>
                  <XSelector
                    label={t('submitApp.externalSource')}
                    value={versionForm.sourceType}
                    options={[
                      { value: 'GITHUB', label: 'GitHub Release' },
                      { value: 'WEBDAV', label: 'WebDAV URL' },
                      { value: 'S3', label: 'S3 URL' },
                    ]}
                    onChange={(sourceType) => setVersionForm((current) => ({ ...current, sourceType }))}
                  />
                  <XTextInput
                    label={t('common.downloadUrl')}
                    description={t('submitApp.externalDownloadHelp')}
                    value={versionForm.downloadUrl}
                    onChange={(downloadUrl) => setVersionForm((current) => ({ ...current, downloadUrl }))}
                  />
                  <XTextInput
                    label={t('common.sha256')}
                    description={t('submitApp.sha256Help')}
                    value={versionForm.sha256}
                    onChange={(sha256) => setVersionForm((current) => ({ ...current, sha256 }))}
                  />
                  {versionForm.sourceType === 'GITHUB' && (
                    <XCheckboxInput
                      label={t('submitApp.useMirrorDownload')}
                      description={versionMirrorDownloadHelp}
                      value={versionForm.useMirrorDownload}
                      onChange={(useMirrorDownload) => setVersionForm((current) => ({ ...current, useMirrorDownload }))}
                    />
                  )}
                </>
              )}
              <XTextArea
                label={t('common.changelog')}
                value={versionForm.changelog}
                rows={4}
                onChange={(changelog) => setVersionForm((current) => ({ ...current, changelog }))}
              />
            </XFormLayout>
            {versionProgress && (
              <XProgressBar
                label={versionProgress.label}
                value={versionProgress.percent}
                hasValueLabel
                variant={versionProgress.percent >= 100 ? 'success' : 'accent'}
              />
            )}
            {!canSubmitVersion && <p className="field-help">{t('drawer.versionSubmitBlocked')}</p>}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
              <XButton
                type="submit"
                variant="primary"
                label={isSubmittingVersion ? t('common.submitting') : t('drawer.publishVersion')}
                icon={<Upload size={17} />}
                isDisabled={!canSubmitVersion || isSubmittingVersion}
              />
            </div>
          </form>
        </ModalLayer>
      );
    }

    if (managementDialog === 'screenshots') {
      return (
        <ModalLayer purpose="form" width="min(920px, calc(100vw - 32px))" onClose={() => setManagementDialog(null)}>
          <div className="modal-panel screenshots-panel">
            <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
            <SectionTitle icon={Archive} title={t('drawer.screenshots')} />
            {renderScreenshotGallery(canEditScreenshots)}
          </div>
        </ModalLayer>
      );
    }

    if (managementDialog === 'visibility') {
      return (
        <ModalLayer purpose="form" width="min(620px, calc(100vw - 32px))" onClose={() => setManagementDialog(null)}>
          <form
            className="modal-panel visibility-panel"
            onSubmit={(event) => {
              event.preventDefault();
              void saveVisibility();
            }}
          >
            <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
            <SectionTitle icon={Users} title={t('drawer.visibilityGroups')} />
            {groups.length === 0 ? (
              <EmptyState icon={Users} title={t('drawer.noGroupsPublic')} />
            ) : (
              <div className="checkbox-list">
                {groups.map((group) => (
                  <XCheckboxInput
                    key={group.id}
                    label={group.name}
                    description={group.description}
                    value={visibility.includes(group.id)}
                    onChange={(checked) => {
                      setVisibility((current) => (
                        checked
                          ? Array.from(new Set([...current, group.id]))
                          : current.filter((id) => id !== group.id)
                      ));
                    }}
                  />
                ))}
              </div>
            )}
            <div className="dialog-actions">
              <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
              <XButton type="submit" variant="primary" label={t('drawer.saveVisibility')} icon={<Save size={17} />} />
            </div>
          </form>
        </ModalLayer>
      );
    }

    return (
      <ModalLayer purpose="form" width="min(700px, calc(100vw - 32px))" onClose={() => setManagementDialog(null)}>
        <div className="modal-panel collaborator-panel">
          <XIconButton type="button" className="close" variant="ghost" label={t('common.close')} icon={<X size={17} />} onClick={() => setManagementDialog(null)} />
          <SectionTitle icon={Users} title={t('drawer.collaboratorRequests')} />
          {collaboratorRequests.length === 0 ? (
            <EmptyState icon={Users} title={t('drawer.noCollaboratorRequests')} />
          ) : (
            <XList density="compact" hasDividers>
              {collaboratorRequests.map((request) => {
                const requesterName = request.username || t('drawer.userLabel', { id: request.userId || request.user_id || '-' });
                return (
                  <XListItem
                    key={request.id}
                    label={requesterName}
                    description={request.message || request.email || t('drawer.noMessage')}
                    endContent={(
                      <div className="row-actions">
                        <XIconButton
                          className="fixed-row-icon-button"
                          type="button"
                          variant="secondary"
                          size="sm"
                          label={t('drawer.approveCollaboratorFor', { name: requesterName })}
                          tooltip={t('drawer.approveCollaborator')}
                          icon={<Check size={17} />}
                          onClick={() => void decideCollaboratorRequest(request.id, true)}
                        />
                        <XIconButton
                          className="fixed-row-icon-button"
                          type="button"
                          variant="destructive"
                          size="sm"
                          label={t('drawer.rejectCollaboratorFor', { name: requesterName })}
                          tooltip={t('drawer.rejectCollaborator')}
                          icon={<X size={17} />}
                          onClick={() => void decideCollaboratorRequest(request.id, false)}
                        />
                      </div>
                    )}
                  />
                );
              })}
            </XList>
          )}
        </div>
      </ModalLayer>
    );
  }

  return (
    <section className="detail-page-shell">
      <article className={cx('detail-page server-detail-page', isManageMode && 'manage-mode')} aria-labelledby={drawerTitleId}>
        <Breadcrumbs className="detail-breadcrumbs" variant="supporting" label={t('common.navigation')}>
          <BreadcrumbItem onClick={onClose}>{t('nav.store')}</BreadcrumbItem>
          <BreadcrumbItem isCurrent={!isManageMode} onClick={isManageMode ? () => onModeChange('detail') : undefined}>
            {appName}
          </BreadcrumbItem>
          {isManageMode && <BreadcrumbItem isCurrent>{t('drawer.management')}</BreadcrumbItem>}
        </Breadcrumbs>
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
          <AppIcon src={app.iconUrl} seed={app.slug || app.name} title={appName} size={58} className="detail-avatar" />
          <div>
            <h2 id={drawerTitleId}>{isManageMode ? t('drawer.manageTitle', { name: appName }) : appName}</h2>
            <p>{isManageMode ? t('drawer.manageBody') : appSummary}</p>
            <div className="meta-line">
              <span>{app.owner}</span>
              <span>{localizedCategory(app, t('common.uncategorized'))}</span>
              <span>{app.latestVersion?.version || '-'}</span>
              {hasOutdatedMarks && (
                <StatusBadge tone="stale" icon={<AlertCircle size={13} />} label={t('drawer.outdatedBadge', { count: app.outdatedMarks ?? 0 })} />
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
                aria-label={installable ? `${t('common.download')} ${appName}` : t('app.installUnavailable', { name: appName })}
              />
              {canOpenManagement && (
                <XButton type="button" variant="secondary" label={t('drawer.manageApp')} icon={<Settings size={18} />} onClick={() => onModeChange('manage')} />
              )}
              {canContactPublisher && (
                <XButton
                  type="button"
                  variant="secondary"
                  label={t('drawer.contactPublisher')}
                  icon={<MessageSquare size={18} />}
                  onClick={() => void onContactPublisher?.(app)}
                />
              )}
              {user && (
                <>
                  <XButton
                    type="button"
                    variant="secondary"
                    label={appFavorited ? t('drawer.appFavorited') : t('drawer.favoriteApp')}
                    icon={<Heart size={18} fill={appFavorited ? 'currentColor' : 'none'} />}
                    onClick={() => void toggleAppFavorite()}
                  />
                  <XButton
                    type="button"
                    variant="secondary"
                    label={submitterFavorited ? t('drawer.submitterFavorited') : t('drawer.favoriteSubmitter')}
                    icon={<Star size={18} fill={submitterFavorited ? 'currentColor' : 'none'} />}
                    onClick={() => void toggleSubmitterFavorite()}
                  />
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
            <XCard className={cx('install-trust', trustState)} variant={trustCardVariant} padding={4} aria-label={t('drawer.installReadiness')}>
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
            </XCard>
            <XCard className={cx('outdated-state', hasOutdatedMarks && 'active')} variant={hasOutdatedMarks ? 'yellow' : 'muted'} padding={4} aria-label={t('drawer.outdatedStatus')}>
              <div className="outdated-state-head">
                <AlertCircle size={19} />
                <div>
                  <strong>{hasOutdatedMarks ? t('drawer.outdatedActiveTitle', { count: app.outdatedMarks ?? 0 }) : t('drawer.outdatedInactiveTitle')}</strong>
                  <span>{hasOutdatedMarks ? t('drawer.outdatedActiveBody') : t('drawer.outdatedInactiveBody')}</span>
                </div>
              </div>
            </XCard>
            {renderMetadataCard()}
          </>
        )}
        {isManageMode && (
          <XCard className="management-overview" padding={4}>
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
            {renderManagementMetadataCard()}
          </XCard>
        )}
        {isManageMode && (canMaintain || canUploadVersion) && (
          <section className="management-actions">
            <SectionTitle icon={Settings} title={t('drawer.managementActions')} />
            <div className="management-action-grid">
              {canMaintain && (
                <ManagementActionCard
                  icon={<Settings size={19} />}
                  title={t('drawer.appInfo')}
                  body={t('drawer.appInfoActionBody')}
                  action={<XButton type="button" variant="secondary" size="sm" label={t('common.edit')} icon={<Settings size={17} />} onClick={() => setManagementDialog('app-info')} />}
                />
              )}
              {canUploadVersion && (
                <ManagementActionCard
                  icon={<Upload size={19} />}
                  title={t('drawer.publishVersion')}
                  body={t('drawer.publishVersionActionBody')}
                  action={<XButton type="button" variant="secondary" size="sm" label={t('drawer.publishVersion')} icon={<Upload size={17} />} onClick={() => setManagementDialog('publish-version')} />}
                />
              )}
              {canMaintain && (
                <ManagementActionCard
                  icon={<Archive size={19} />}
                  title={t('drawer.screenshots')}
                  body={t('drawer.screenshotsActionBody')}
                  action={<XButton type="button" variant="secondary" size="sm" label={t('drawer.manageScreenshots')} icon={<Archive size={17} />} onClick={() => setManagementDialog('screenshots')} />}
                />
              )}
              {canMaintain && (
                <ManagementActionCard
                  icon={<Users size={19} />}
                  title={t('drawer.visibilityGroups')}
                  body={t('drawer.visibilityActionBody')}
                  action={<XButton type="button" variant="secondary" size="sm" label={t('drawer.manageVisibility')} icon={<Users size={17} />} onClick={() => setManagementDialog('visibility')} />}
                />
              )}
              {canMaintain && (
                <ManagementActionCard
                  icon={<Users size={19} />}
                  title={t('drawer.collaboratorRequests')}
                  body={t('drawer.collaboratorActionBody', { count: collaboratorRequests.length })}
                  action={<XButton type="button" variant="secondary" size="sm" label={t('drawer.reviewCollaborators')} icon={<Users size={17} />} onClick={() => setManagementDialog('collaborators')} />}
                />
              )}
            </div>
          </section>
        )}
        {renderManagementDialogs()}
        {!isManageMode && (
          <section>
            <h3>{t('drawer.screenshots')}</h3>
            {renderScreenshotGallery(false)}
          </section>
        )}
        <section>
          <h3>{t('drawer.versionHistory')}</h3>
          {(app.versions || []).length === 0 ? (
            <EmptyState icon={History} title={t('drawer.noVersions')} body={t('drawer.installBlocked')} />
          ) : (
            <VersionHistoryTable
              rows={(app.versions || []).map((version) => {
                const isLatest = version.id === latestVersion?.id || version.version === latestVersion?.version;
                return {
                  id: version.id,
                  version: version.version,
                  sourceType: version.sourceType,
                  sizeBytes: version.fileSize,
                  sha256: version.sha256,
                  publishedAt: version.publishedAt || version.createdAt,
                  isLatest,
                  statusLabel: isLatest ? t('sourceDetail.latest') : t('drawer.historicalVersion'),
                  statusVariant: isLatest ? 'success' : 'neutral',
                  action: (
                    <XButton
                      type="button"
                      variant="secondary"
                      size="sm"
                      label={isLatest ? t('common.download') : t('drawer.downloadHistoricalVersion')}
                      icon={<Download size={17} />}
                      isDisabled={!version.downloadUrl}
                      onClick={() => void onInstall(app, { version: version.version })}
                    />
                  ),
                };
              })}
            />
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

function ManagementActionCard({
  icon,
  title,
  body,
  action,
}: {
  icon: ReactNode;
  title: string;
  body: string;
  action: ReactNode;
}) {
  return (
    <XCard className="management-action-card" variant="muted" padding={4}>
      <div className="management-action-card-head">
        <span className="management-action-card-icon" aria-hidden="true">
          {icon}
        </span>
        <div className="management-action-card-copy">
          <strong>{title}</strong>
          <span>{body}</span>
        </div>
      </div>
      <div className="management-action-card-action">
        {action}
      </div>
    </XCard>
  );
}
