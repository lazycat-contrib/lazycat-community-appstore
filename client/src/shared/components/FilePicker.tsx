import { FileInput as XFileInput } from '@astryxdesign/core/FileInput';
import type { RefObject } from 'react';

export function FilePicker({
  label,
  help,
  value,
  accept,
  required,
  disabled,
  multiple,
  maxFiles,
  inputRef,
  onChange,
}: {
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
      isMultiple={multiple}
      maxFiles={maxFiles}
      mode="dropzone"
      width="100%"
      onChange={(file) => onChange(multiple ? (Array.isArray(file) ? file : file ? [file] : []) : Array.isArray(file) ? file[0] || null : file)}
    />
  );
}
