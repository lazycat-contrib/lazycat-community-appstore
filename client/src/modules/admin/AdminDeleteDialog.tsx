import { Button as XButton } from '@astryxdesign/core/Button';
import { Dialog as XDialog } from '@astryxdesign/core/Dialog';
import { Trash2, X } from 'lucide-react';
import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import { SectionTitle } from '../../shared/components/Feedback';

export function AdminDeleteDialog({
  title,
  subject,
  consequence,
  confirmLabel,
  isDeleting,
  onCancel,
  onConfirm,
}: {
  title: string;
  subject: string;
  consequence: string;
  confirmLabel: string;
  isDeleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
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
      <div className="modal-panel form-panel admin-delete-dialog">
        <div id={titleId}><SectionTitle icon={Trash2} title={title} /></div>
        <div className="admin-delete-copy">
          <strong id={subjectId}>{subject}</strong>
          <p id={consequenceId}>{consequence}</p>
        </div>
        <div className="dialog-actions">
          <XButton data-autofocus="true" type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={isDeleting} onClick={onCancel} />
          <XButton type="button" variant="destructive" label={confirmLabel} icon={<Trash2 size={17} />} isDisabled={isDeleting} isLoading={isDeleting} onClick={onConfirm} />
        </div>
      </div>
    </XDialog>
  );
}
