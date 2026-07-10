import { Badge as XBadge } from '@astryxdesign/core/Badge';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Check, Save, TriangleAlert } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import type { AdminSaveStatus } from './adminState';

export function AdminSaveBar({
  status,
  isDirty,
  saveLabel,
  onSave,
  isDisabled = false,
}: {
  status: AdminSaveStatus;
  isDirty: boolean;
  saveLabel: string;
  onSave: () => void;
  isDisabled?: boolean;
}) {
  const { t } = useTranslation();
  const label = status === 'saving'
    ? t('admin.saveState.saving')
    : status === 'error'
      ? t('admin.saveState.failed')
      : isDirty
        ? t('admin.saveState.unsaved')
        : status === 'saved'
          ? t('admin.saveState.saved')
          : t('admin.saveState.noChanges');
  const variant = status === 'error' ? 'error' : isDirty ? 'warning' : status === 'saved' ? 'success' : 'neutral';
  const icon = status === 'error' ? <TriangleAlert size={14} /> : status === 'saved' && !isDirty ? <Check size={14} /> : undefined;

  return (
    <div className="admin-save-bar">
      <span className="admin-save-status" role="status" aria-live="polite" aria-atomic="true">
        <XBadge label={label} variant={variant} icon={icon} />
      </span>
      <XButton
        type="button"
        variant="primary"
        label={saveLabel}
        icon={<Save size={17} />}
        isDisabled={isDisabled || !isDirty || status === 'saving'}
        isLoading={status === 'saving'}
        onClick={onSave}
      />
    </div>
  );
}
