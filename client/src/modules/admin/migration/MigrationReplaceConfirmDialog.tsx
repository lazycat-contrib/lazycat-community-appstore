import { AlertTriangle, RotateCcw, X } from 'lucide-react';
import { useState } from 'react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ModalLayer } from '../../../shared/components/ModalLayer';
import { replaceConfirmationText } from './constants';

export function MigrationReplaceConfirmDialog({
  isImporting,
  t,
  onClose,
  onConfirm,
}: {
  isImporting: boolean;
  t: (key: string, options?: any) => string;
  onClose: () => void;
  onConfirm: (value: string) => void;
}) {
  const [value, setValue] = useState('');
  const canConfirm = value.trim() === replaceConfirmationText;

  return (
    <ModalLayer onClose={() => { if (!isImporting) onClose(); }} purpose="required" width="min(540px, calc(100vw - 36px))">
      <div className="modal-panel form-panel migration-confirm-dialog" aria-label={t('admin.migration.replaceConfirmTitle')}>
        <XIconButton label={t('common.close')} variant="ghost" icon={<X size={17} />} isDisabled={isImporting} onClick={onClose} />
        <div className="migration-confirm-head">
          <AlertTriangle size={22} />
          <div>
            <strong>{t('admin.migration.replaceConfirmTitle')}</strong>
            <span>{t('admin.migration.replaceConfirmBody')}</span>
          </div>
        </div>
        <XTextInput
          label={t('admin.migration.replaceConfirmInput', { value: replaceConfirmationText })}
          value={value}
          isDisabled={isImporting}
          onChange={setValue}
        />
        <div className="dialog-actions">
          <XButton type="button" variant="secondary" label={t('common.cancel')} icon={<X size={17} />} isDisabled={isImporting} onClick={onClose} />
          <XButton
            type="button"
            variant="destructive"
            label={t('admin.migration.replaceConfirmAction')}
            icon={<RotateCcw size={17} />}
            isDisabled={!canConfirm || isImporting}
            isLoading={isImporting}
            onClick={() => onConfirm(value.trim())}
          />
        </div>
      </div>
    </ModalLayer>
  );
}
