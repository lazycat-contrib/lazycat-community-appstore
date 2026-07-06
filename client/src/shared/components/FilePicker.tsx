import { FileInput as XFileInput } from '@astryxdesign/core/FileInput';
import type { RefObject } from 'react';

export function FilePicker({
  label,
  help,
  value,
  accept,
  required,
  disabled,
  inputRef,
  onChange,
}: {
  label: string;
  help?: string;
  value?: File | null;
  accept?: string;
  required?: boolean;
  disabled?: boolean;
  inputRef?: RefObject<HTMLInputElement | null>;
  onChange: (file: File | null) => void;
}) {
  return (
    <XFileInput
      ref={inputRef}
      label={label}
      description={help}
      value={value || null}
      placeholder={label}
      accept={accept}
      isRequired={required}
      isDisabled={disabled}
      mode="dropzone"
      width="100%"
      onChange={(file) => onChange(Array.isArray(file) ? file[0] || null : file)}
    />
  );
}
