import type { FormEvent, RefObject } from 'react';
import { AlertCircle, Check, ChevronRight, Link, ShieldCheck, Upload, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import { ArtifactModeOption } from '../../shared/components/ArtifactModeOption';
import { FilePicker } from '../../shared/components/FilePicker';
import type { Category } from '../../shared/types';
import { cx, formatBytes, localizedName } from '../../shared/utils';

export type AppSubmissionDraft = {
  name: string;
  version: string;
  summary: string;
  description: string;
  categoryId: string;
  tags: string;
  allowUnreviewedUpdates: boolean;
  emailNotificationsEnabled: boolean;
  sourceType: string;
  downloadUrl: string;
  sha256: string;
  installPassword: string;
};

export type SubmissionArtifactMode = 'local' | 'external';

export function AppSubmissionForm({
  draft,
  onDraftChange,
  categories,
  storageOptions,
  storageKey,
  onStorageKeyChange,
  artifactMode,
  onArtifactModeChange,
  file,
  onFileChange,
  fileInputRef,
  desktopScreenshotFiles,
  desktopScreenshotCaptions,
  onDesktopScreenshotFilesChange,
  onDesktopScreenshotCaptionChange,
  mobileScreenshotFiles,
  mobileScreenshotCaptions,
  onMobileScreenshotFilesChange,
  onMobileScreenshotCaptionChange,
  recentSubmission,
  isDirectPublishUser,
  isSubmitting,
  onSubmit,
  onCancel,
}: {
  draft: AppSubmissionDraft;
  onDraftChange: (draft: AppSubmissionDraft) => void;
  categories: Category[];
  storageOptions: Array<{ value: string; label: string }>;
  storageKey: string;
  onStorageKeyChange: (key: string) => void;
  artifactMode: SubmissionArtifactMode;
  onArtifactModeChange: (mode: SubmissionArtifactMode) => void;
  file: File | null;
  onFileChange: (file: File | null) => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
  desktopScreenshotFiles: File[];
  desktopScreenshotCaptions: Record<string, string>;
  onDesktopScreenshotFilesChange: (files: File[]) => void;
  onDesktopScreenshotCaptionChange: (file: File, caption: string) => void;
  mobileScreenshotFiles: File[];
  mobileScreenshotCaptions: Record<string, string>;
  onMobileScreenshotFilesChange: (files: File[]) => void;
  onMobileScreenshotCaptionChange: (file: File, caption: string) => void;
  recentSubmission: { name: string; status: string } | null;
  isDirectPublishUser: boolean;
  isSubmitting: boolean;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const appInfoReady = Boolean(draft.name.trim());
  const appInfoDetailed = Boolean(draft.summary.trim() && draft.description.trim());
  const appInfoComplete = appInfoReady && appInfoDetailed;
  const externalDownloadReady = Boolean(draft.downloadUrl.trim());
  const externalChecksumReady = Boolean(draft.sha256.trim());
  const artifactReady = artifactMode === 'local' ? Boolean(file) : externalDownloadReady;
  const appIdentityCanAutofill = artifactMode === 'local' ? Boolean(file) : externalDownloadReady;
  const canSubmitUpload = (appInfoReady || appIdentityCanAutofill) && artifactReady;
  const screenshotFileKey = (screenshot: File) => `${screenshot.name}:${screenshot.size}:${screenshot.lastModified}`;
  const renderScreenshotCaptionList = (
    files: File[],
    captions: Record<string, string>,
    onCaptionChange: (file: File, caption: string) => void,
  ) => files.length > 0 ? (
    <div className="selected-screenshot-list">
      {files.map((screenshot, index) => (
        <div className="selected-screenshot-row" key={screenshotFileKey(screenshot)}>
          <div className="selected-screenshot-meta">
            <strong>{screenshot.name}</strong>
            <span>{formatBytes(screenshot.size)}</span>
          </div>
          <XTextInput
            label={t('submitApp.screenshotCaptionFor', { index: index + 1 })}
            value={captions[screenshotFileKey(screenshot)] || ''}
            placeholder={t('submitApp.screenshotCaption')}
            onChange={(value) => onCaptionChange(screenshot, value)}
          />
        </div>
      ))}
    </div>
  ) : null;

  return (
    <section className="workspace-pane">
      <form className="panel form-panel" onSubmit={onSubmit}>
        <div className="section-title with-action">
          <div>
            <Upload size={19} />
            <h2>{t('submitApp.title')}</h2>
          </div>
          <XButton type="button" variant="secondary" size="sm" label={t('common.cancel')} icon={<X size={17} />} onClick={onCancel} />
        </div>
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
                : externalDownloadReady
                  ? t('submitApp.readinessArtifactExternalReady')
                  : externalChecksumReady
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
        <XTextInput label={t('submitApp.appName')} value={draft.name} onChange={(value) => onDraftChange({ ...draft, name: value })} />
        <XTextInput label={t('common.version')} value={draft.version} onChange={(value) => onDraftChange({ ...draft, version: value })} />
        <XTextInput label={t('common.summary')} value={draft.summary} onChange={(value) => onDraftChange({ ...draft, summary: value })} />
        <XTextArea label={t('common.description')} value={draft.description} rows={4} onChange={(value) => onDraftChange({ ...draft, description: value })} />
        <XSelector
          label={t('common.category')}
          value={draft.categoryId}
          options={[
            { value: '', label: t('common.uncategorized') },
            ...categories.map((category) => ({ value: String(category.id), label: localizedName(category) })),
          ]}
          onChange={(value) => onDraftChange({ ...draft, categoryId: value })}
        />
        <XTextInput label={t('common.tags')} value={draft.tags} onChange={(value) => onDraftChange({ ...draft, tags: value })} />
        {storageOptions.length > 0 && (
          <XSelector
            label={t('common.storage')}
            description={t('submitApp.storageHelp')}
            value={storageKey}
            options={storageOptions}
            onChange={onStorageKeyChange}
          />
        )}
        <div className="artifact-section">
          <div className="artifact-section-head">
            <strong>{t('submitApp.artifactMode')}</strong>
            <span>{artifactMode === 'local' ? t('submitApp.localArtifactHint') : t('submitApp.externalArtifactHint')}</span>
          </div>
          <div className="artifact-mode" aria-label={t('submitApp.artifactMode')}>
            <ArtifactModeOption
              icon={<Upload size={17} />}
              title={t('submitApp.localArtifact')}
              hint={t('submitApp.localArtifactHint')}
              isSelected={artifactMode === 'local'}
              onSelect={() => onArtifactModeChange('local')}
            />
            <ArtifactModeOption
              icon={<Link size={17} />}
              title={t('submitApp.externalArtifact')}
              hint={t('submitApp.externalArtifactHint')}
              isSelected={artifactMode === 'external'}
              onSelect={() => onArtifactModeChange('external')}
            />
          </div>
          {artifactMode === 'local' ? (
            <FilePicker
              label={t('common.lpkFile')}
              help={t('submitApp.localFileHelp')}
              value={file}
              inputRef={fileInputRef}
              accept=".lpk"
              required
              onChange={(nextFile) => onFileChange(Array.isArray(nextFile) ? nextFile[0] || null : nextFile)}
            />
          ) : (
            <div className="artifact-fields">
              <p className="field-help">{t('submitApp.externalFieldsHelp')}</p>
              <XSelector
                label={t('submitApp.externalSource')}
                value={draft.sourceType}
                options={[
                  { value: 'GITHUB', label: 'GitHub Release' },
                  { value: 'WEBDAV', label: 'WebDAV URL' },
                  { value: 'S3', label: 'S3 URL' },
                ]}
                onChange={(value) => onDraftChange({ ...draft, sourceType: value })}
              />
              <XTextInput
                label={t('submitApp.externalDownloadUrl')}
                description={t('submitApp.externalDownloadHelp')}
                value={draft.downloadUrl}
                onChange={(value) => onDraftChange({ ...draft, downloadUrl: value })}
              />
              <XTextInput
                label={t('common.sha256')}
                description={t('submitApp.sha256Help')}
                value={draft.sha256}
                onChange={(value) => onDraftChange({ ...draft, sha256: value })}
              />
            </div>
          )}
        </div>
        <div className="artifact-section">
          <div className="artifact-section-head">
            <strong>{t('submitApp.screenshots')}</strong>
            <span>{t('submitApp.screenshotsHelp')}</span>
          </div>
          <div className="screenshot-upload-grid">
            <div className="screenshot-upload-stack">
              <FilePicker
                label={t('submitApp.desktopScreenshots')}
                help={t('submitApp.desktopScreenshotsHelp', { count: desktopScreenshotFiles.length })}
                value={desktopScreenshotFiles}
                accept=".png,.jpg,.jpeg,.webp"
                multiple
                maxFiles={8}
                onChange={(nextFiles) => onDesktopScreenshotFilesChange(Array.isArray(nextFiles) ? nextFiles : nextFiles ? [nextFiles] : [])}
              />
              {renderScreenshotCaptionList(desktopScreenshotFiles, desktopScreenshotCaptions, onDesktopScreenshotCaptionChange)}
            </div>
            <div className="screenshot-upload-stack">
              <FilePicker
                label={t('submitApp.mobileScreenshots')}
                help={t('submitApp.mobileScreenshotsHelp', { count: mobileScreenshotFiles.length })}
                value={mobileScreenshotFiles}
                accept=".png,.jpg,.jpeg,.webp"
                multiple
                maxFiles={8}
                onChange={(nextFiles) => onMobileScreenshotFilesChange(Array.isArray(nextFiles) ? nextFiles : nextFiles ? [nextFiles] : [])}
              />
              {renderScreenshotCaptionList(mobileScreenshotFiles, mobileScreenshotCaptions, onMobileScreenshotCaptionChange)}
            </div>
          </div>
        </div>
        <XTextInput
          type="password"
          label={t('submitApp.installPassword')}
          description={t('submitApp.installPasswordHelp')}
          value={draft.installPassword}
          onChange={(value) => onDraftChange({ ...draft, installPassword: value })}
        />
        <XSwitch
          label={t('submitApp.emailNotificationsEnabled')}
          value={draft.emailNotificationsEnabled}
          labelSpacing="spread"
          width="100%"
          onChange={(checked) => onDraftChange({ ...draft, emailNotificationsEnabled: checked })}
        />
        <XSwitch
          label={t('submitApp.allowUnreviewedUpdates')}
          value={draft.allowUnreviewedUpdates}
          labelSpacing="spread"
          width="100%"
          onChange={(checked) => onDraftChange({ ...draft, allowUnreviewedUpdates: checked })}
        />
        {!canSubmitUpload && <p className="field-help">{t('submitApp.submitBlocked')}</p>}
        <XButton
          type="submit"
          variant="primary"
          label={isSubmitting ? t('common.submitting') : t('common.submit')}
          icon={<Upload size={18} />}
          isDisabled={!canSubmitUpload || isSubmitting}
        />
      </form>
    </section>
  );
}
