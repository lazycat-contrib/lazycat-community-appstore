import { type FormEvent, useEffect, useId, useState } from 'react';
import { Archive, Save, Trash2, X } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Dialog as XDialog } from '@astryxdesign/core/Dialog';
import { FormLayout as XFormLayout } from '@astryxdesign/core/FormLayout';
import { NumberInput as XNumberInput } from '@astryxdesign/core/NumberInput';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { Text as XText } from '@astryxdesign/core/Text';
import { useTranslation } from 'react-i18next';
import { SectionTitle } from '../../shared/components/Feedback';
import type { Version, VersionRetentionPolicy } from '../../shared/types';
import { retentionPruneCount } from './versionManagementState';

const wrappingTextStyle = {
  minWidth: 0,
  overflowWrap: 'anywhere',
} as const;

export type VersionRetentionDialogProps = {
  policy: VersionRetentionPolicy;
  versions: Version[];
  isSaving: boolean;
  onCancel: () => void;
  onSave: (
    input:
      | { mode: 'INHERIT' }
      | { mode: 'CUSTOM'; maxVersions: number },
  ) => Promise<void>;
};

export type VersionDeleteDialogProps = {
  appName: string;
  version: Version;
  consequence: string;
  isDeleting: boolean;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
};

export function VersionRetentionDialog({
  policy,
  versions,
  isSaving,
  onCancel,
  onSave,
}: VersionRetentionDialogProps) {
  const { t } = useTranslation();
  const titleId = useId();
  const summaryId = useId();
  const [mode, setMode] = useState<VersionRetentionPolicy['mode']>(policy.mode);
  const [customMaxVersions, setCustomMaxVersions] = useState(
    policy.appMaxVersions ?? policy.effectiveMaxVersions,
  );

  useEffect(() => {
    setMode(policy.mode);
    setCustomMaxVersions(policy.appMaxVersions ?? policy.effectiveMaxVersions);
  }, [policy.appMaxVersions, policy.effectiveMaxVersions, policy.mode]);

  const effectiveMaxVersions = mode === 'INHERIT'
    ? policy.siteMaxVersions
    : customMaxVersions;
  const pruneCount = retentionPruneCount(versions, effectiveMaxVersions);
  const approvedCount = versions.filter((version) => version.status === 'APPROVED').length;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isSaving) return;
    if (mode === 'INHERIT') {
      await onSave({ mode: 'INHERIT' });
      return;
    }
    await onSave({ mode: 'CUSTOM', maxVersions: customMaxVersions });
  }

  return (
    <XDialog
      isOpen
      onOpenChange={(open) => {
        if (!open && !isSaving) onCancel();
      }}
      purpose="form"
      aria-labelledby={titleId}
      aria-describedby={summaryId}
      width="min(560px, calc(100vw - 36px))"
      maxHeight="calc(100vh - 36px)"
      padding={0}
      className="modal-dialog-shell"
    >
      <form
        className="modal-panel form-panel"
        aria-busy={isSaving}
        onSubmit={(event) => void submit(event)}
      >
        <div id={titleId} style={wrappingTextStyle}>
          <SectionTitle icon={Archive} title={t('drawer.versionRetentionTitle')} />
        </div>

        <XFormLayout>
          <XSelector
            label={t('drawer.versionRetentionSettings')}
            value={mode}
            options={[
              {
                value: 'INHERIT',
                label: t('drawer.versionRetentionInherited', { count: policy.siteMaxVersions }),
              },
              {
                value: 'CUSTOM',
                label: t('drawer.versionRetentionCustom'),
              },
            ]}
            isDisabled={isSaving}
            onChange={(value) => setMode(value as VersionRetentionPolicy['mode'])}
          />

          {mode === 'CUSTOM' && (
            <XNumberInput
              label={t('drawer.versionRetentionEffectiveCount')}
              description={t('drawer.versionRetentionUnlimited')}
              value={customMaxVersions}
              min={0}
              step={1}
              isIntegerOnly
              isDisabled={isSaving}
              onChange={(value) => setCustomMaxVersions(Math.max(0, value))}
            />
          )}
        </XFormLayout>

        <div id={summaryId} style={wrappingTextStyle}>
          <XText as="p" display="block" style={wrappingTextStyle}>
            {t('drawer.versionRetentionApprovedCount', { count: approvedCount })}
          </XText>
          <XText as="p" type="supporting" display="block" style={wrappingTextStyle}>
            {effectiveMaxVersions === 0
              ? t('drawer.versionRetentionUnlimited')
              : t('drawer.versionRetentionEffectiveCount', { count: effectiveMaxVersions })}
          </XText>
          {pruneCount > 0 && (
            <XText as="p" type="supporting" display="block" style={wrappingTextStyle}>
              {t('drawer.versionRetentionSaveAndPrune', { count: pruneCount })}
            </XText>
          )}
        </div>

        <div className="dialog-actions">
          <XButton
            data-autofocus="true"
            type="button"
            variant="secondary"
            label={t('common.cancel')}
            icon={<X size={17} />}
            isDisabled={isSaving}
            onClick={onCancel}
          />
          <XButton
            type="submit"
            variant={pruneCount > 0 ? 'destructive' : 'primary'}
            label={pruneCount > 0
              ? t('drawer.versionRetentionSaveAndPrune', { count: pruneCount })
              : t('drawer.versionRetentionSave')}
            icon={pruneCount > 0 ? <Trash2 size={17} /> : <Save size={17} />}
            isDisabled={isSaving}
            isLoading={isSaving}
          />
        </div>
      </form>
    </XDialog>
  );
}

export function VersionDeleteDialog({
  appName,
  version,
  consequence,
  isDeleting,
  onCancel,
  onConfirm,
}: VersionDeleteDialogProps) {
  const { t } = useTranslation();
  const titleId = useId();
  const subjectId = useId();
  const consequenceId = useId();

  return (
    <XDialog
      isOpen
      onOpenChange={(open) => {
        if (!open && !isDeleting) onCancel();
      }}
      purpose="form"
      role="alertdialog"
      aria-labelledby={titleId}
      aria-describedby={`${subjectId} ${consequenceId}`}
      width="min(560px, calc(100vw - 36px))"
      maxHeight="calc(100vh - 36px)"
      padding={0}
      className="modal-dialog-shell"
    >
      <div className="modal-panel form-panel" aria-busy={isDeleting}>
        <div id={titleId} style={wrappingTextStyle}>
          <SectionTitle
            icon={Trash2}
            title={t('drawer.versionDeleteTitle', { appName, version: version.version })}
          />
        </div>

        <XText id={subjectId} as="p" display="block" weight="semibold" style={wrappingTextStyle}>
          {appName} · {version.version}
        </XText>
        <XText id={consequenceId} as="p" type="supporting" display="block" style={wrappingTextStyle}>
          {consequence}
        </XText>

        <div className="dialog-actions">
          <XButton
            data-autofocus="true"
            type="button"
            variant="secondary"
            label={t('common.cancel')}
            icon={<X size={17} />}
            isDisabled={isDeleting}
            onClick={onCancel}
          />
          <XButton
            type="button"
            variant="destructive"
            label={t('drawer.versionDeleteConfirm')}
            icon={<Trash2 size={17} />}
            isDisabled={isDeleting}
            isLoading={isDeleting}
            clickAction={onConfirm}
          />
        </div>
      </div>
    </XDialog>
  );
}
