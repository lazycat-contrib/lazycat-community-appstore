import { Upload, X } from 'lucide-react';
import { useEffect, useId, useRef, useState, type RefObject } from 'react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { useTranslation } from 'react-i18next';
import { fileMatchesAccept } from '../filePickerState';

export function FilePicker({ label, help, value, accept, required, disabled, multiple, maxFiles, inputRef, onChange }: {
  label: string;
  help?: string;
  value?: File | File[] | null;
  accept?: string;
  required?: boolean;
  disabled?: boolean;
  multiple?: boolean;
  maxFiles?: number;
  inputRef?: RefObject<HTMLInputElement | null>;
  onChange: (file: File | File[] | null) => void;
}) {
  const { t } = useTranslation();
  const id = useId();
  const [error, setError] = useState('');
  const input = useRef<HTMLInputElement | null>(null);
  const files = Array.isArray(value) ? value : value ? [value] : [];
  useEffect(() => {
    if (files.length === 0 && input.current) input.current.value = '';
  }, [files.length]);
  return (
    <div className="file-picker-field">
      <label htmlFor={id}>{label}{required && <span aria-hidden="true"> *</span>}</label>
      {help && <p id={`${id}-help`} className="field-help">{help}</p>}
      <div className={`file-picker${disabled ? ' is-disabled' : ''}`}>
        <Upload size={22} aria-hidden="true" />
        <span id={`${id}-selection`}>{files.length ? files.map((file) => file.name).join(', ') : label}</span>
        <input
          id={id}
          className="file-picker-input"
          type="file"
          ref={(node) => { input.current = node; if (inputRef) inputRef.current = node; }}
          accept={accept}
          required={required && files.length === 0}
          disabled={disabled}
          multiple={multiple}
          aria-describedby={[help ? `${id}-help` : null, `${id}-selection`].filter(Boolean).join(' ')}
          aria-invalid={Boolean(error)}
          onChange={(event) => {
            const selected = Array.from(event.currentTarget.files || []);
            const invalid = selected.find((file) => !fileMatchesAccept(file, accept));
            if (invalid || (maxFiles && selected.length > maxFiles)) {
              setError(invalid ? t('filePicker.unsupported', { name: invalid.name }) : t('filePicker.limit', { count: maxFiles }));
              event.currentTarget.value = '';
              return;
            }
            setError('');
            onChange(multiple ? selected : selected[0] || null);
          }}
        />
      </div>
      {error && <p className="inline-error" role="alert">{error}</p>}
      {files.length > 0 && <XButton size="sm" variant="ghost" label={t('common.clearSelection')} icon={<X size={15} />} isDisabled={disabled} onClick={() => { if (input.current) input.current.value = ''; setError(''); onChange(multiple ? [] : null); }} />}
    </div>
  );
}
