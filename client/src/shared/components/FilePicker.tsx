import { Upload } from 'lucide-react';
import type { RefObject } from 'react';
import { useTranslation } from 'react-i18next';

export function FilePicker({
  label,
  help,
  fileName,
  accept,
  required,
  disabled,
  inputRef,
  onChange,
}: {
  label: string;
  help?: string;
  fileName?: string;
  accept?: string;
  required?: boolean;
  disabled?: boolean;
  inputRef?: RefObject<HTMLInputElement | null>;
  onChange: (file: File | null) => void;
}) {
  const { t } = useTranslation();
  return (
    <label className="file-drop-field" data-disabled={disabled || undefined}>
      <span className="file-drop-label">{label}</span>
      <span className="file-drop-box">
        <Upload size={19} />
        <strong>{fileName || t('common.chooseFile')}</strong>
        {help && <small>{help}</small>}
      </span>
      <input
        ref={inputRef}
        className="file-drop-input"
        type="file"
        accept={accept}
        required={required}
        disabled={disabled}
        onChange={(event) => onChange(event.target.files?.[0] || null)}
      />
    </label>
  );
}
