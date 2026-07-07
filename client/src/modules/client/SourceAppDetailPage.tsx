import { type FormEvent, useEffect, useRef, useState } from 'react';
import { AlertCircle, Archive, ArrowLeft, Check, Cloud, Download, Gauge, History, MessageSquare, MessageSquareOff, RefreshCw, ShieldCheck, Star, Tag, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { AppIcon } from '../../components/AppIcon';
import { CommentList } from '../../components/CommentList';
import { clientApi } from '../../shared/api';
import { EmptyState } from '../../shared/components/Feedback';
import { orderedScreenshots, screenshotDeviceLabel, usePreferredScreenshotDevice } from '../../shared/screenshotHelpers';
import type { Comment, InstalledApplication, SourceApp, Toast } from '../../shared/types';
import { arrayOrEmpty, cx, errorMessage, formatBytes, hasInstallableVersion, localizedCategory, shortSHA, sourceActionLabel, sourceInstallAction } from '../../shared/utils';

export function SourceAppDetailPage({
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
