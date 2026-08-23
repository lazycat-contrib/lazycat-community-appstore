import { Plus, X } from 'lucide-react';
import { type ClipboardEvent, type KeyboardEvent, useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { addGroupCodeBatch, MAX_SOURCE_GROUP_CODES } from './sourceConfig';

export function GroupCodeInput({
  value,
  onChange,
  isDisabled = false,
  maxEntries = MAX_SOURCE_GROUP_CODES,
}: {
  value: string[];
  onChange: (codes: string[]) => void;
  isDisabled?: boolean;
  maxEntries?: number;
}) {
  const { t } = useTranslation();
  const inputID = useId();
  const helpID = useId();
  const statusID = useId();
  const [draft, setDraft] = useState('');
  const [status, setStatus] = useState<{ tone: 'error' | 'neutral'; message: string } | null>(null);

  function commit(raw = draft) {
    if (!raw.trim()) return;
    const result = addGroupCodeBatch(raw, value, maxEntries);
    onChange(result.codes);
    setDraft(result.pending.join(' '));

    if (result.invalidCount > 0) {
      setStatus({ tone: 'error', message: t('sources.groupCodeInvalid', { count: result.invalidCount }) });
    } else if (result.overflowCount > 0) {
      setStatus({ tone: 'error', message: t('sources.groupCodeLimit', { count: maxEntries }) });
    } else if (result.duplicateCount > 0) {
      setStatus({ tone: 'neutral', message: t('sources.groupCodeDuplicatesSkipped', { count: result.duplicateCount }) });
    } else if (result.addedCount > 0) {
      setStatus({ tone: 'neutral', message: t('sources.groupCodeAdded', { count: result.addedCount }) });
    }
  }

  function remove(code: string) {
    onChange(value.filter((item) => item !== code));
    setStatus({ tone: 'neutral', message: t('sources.groupCodeRemoved', { code }) });
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter' || event.key === ',' || event.key === ';' || event.key === '，' || event.key === '；') {
      event.preventDefault();
      commit();
      return;
    }
    if (event.key === ' ' && draft.trim().length >= 6) {
      event.preventDefault();
      commit();
      return;
    }
    if (event.key === 'Backspace' && !draft && value.length > 0) {
      event.preventDefault();
      remove(value[value.length - 1]);
    }
  }

  function handlePaste(event: ClipboardEvent<HTMLInputElement>) {
    const pasted = event.clipboardData.getData('text');
    if (!pasted.trim()) return;
    event.preventDefault();
    commit(`${draft} ${pasted}`);
  }

  return (
    <div className="group-code-field">
      <div className="group-code-field-heading">
        <label htmlFor={inputID}>{t('sources.groupCodes')}</label>
        <span>{t('sources.groupCodeCount', { count: value.length, max: maxEntries })}</span>
      </div>
      <p id={helpID} className="group-code-help">{t('sources.groupCodesHelp')}</p>
      {value.length > 0 && (
        <div className="group-code-chip-list" aria-label={t('sources.groupCodesAdded')}>
          {value.map((code) => (
            <span className="group-code-chip" key={code}>
              <code>{code}</code>
              <button
                type="button"
                className="group-code-chip-remove"
                aria-label={t('sources.groupCodeRemove', { code })}
                disabled={isDisabled}
                onClick={() => remove(code)}
              >
                <X size={15} aria-hidden="true" />
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="group-code-entry-row">
        <input
          id={inputID}
          type="text"
          value={draft}
          aria-describedby={`${helpID} ${status ? statusID : ''}`.trim()}
          aria-invalid={status?.tone === 'error' || undefined}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          disabled={isDisabled}
          placeholder={t('sources.groupCodesPlaceholder')}
          onChange={(event) => {
            setDraft(event.target.value.toUpperCase());
            if (status) setStatus(null);
          }}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
        />
        <XButton
          type="button"
          variant="secondary"
          label={t('sources.groupCodeAdd')}
          icon={<Plus size={16} />}
          isDisabled={isDisabled || !draft.trim()}
          onClick={() => commit()}
        />
      </div>
      {status && (
        <p
          id={statusID}
          className={`group-code-status ${status.tone === 'error' ? 'error' : ''}`}
          role={status.tone === 'error' ? 'alert' : 'status'}
        >
          {status.message}
        </p>
      )}
    </div>
  );
}
