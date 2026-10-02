import { createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';
import { ModalLayer } from './components/ModalLayer';
import { settleUnsavedChanges, type UnsavedChange } from './unsavedChangesState';

type NavigationGuard = {
  register: (key: string, read: () => UnsavedChange) => () => void;
  hasUnsavedChanges: () => boolean;
  requestNavigation: (action: () => void) => void;
  prompt: ReactNode;
};
const Context = createContext<NavigationGuard | null>(null);

export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const entries = useRef(new Map<string, () => UnsavedChange>());
  const pending = useRef<(() => void) | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(false);
  const busy = useRef(false);
  const register = useCallback((key: string, read: () => UnsavedChange) => {
    entries.current.set(key, read);
    return () => { entries.current.delete(key); };
  }, []);
  const hasUnsavedChanges = useCallback(() => [...entries.current.values()].some((read) => read().isDirty), []);
  const requestNavigation = useCallback((action: () => void) => {
    if (busy.current) return;
    if (!hasUnsavedChanges()) { action(); return; }
    pending.current = action;
    setError(false);
    setIsOpen(true);
  }, [hasUnsavedChanges]);

  useEffect(() => {
    const preventUnload = (event: BeforeUnloadEvent) => {
      if (!hasUnsavedChanges()) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', preventUnload);
    return () => window.removeEventListener('beforeunload', preventUnload);
  }, [hasUnsavedChanges]);

  function stay() {
    if (busy.current) return;
    pending.current = null;
    setIsOpen(false);
  }
  async function leave(action: 'save' | 'discard') {
    if (busy.current) return;
    busy.current = true;
    setSaving(action === 'save');
    setError(false);
    try {
      const changes = [...entries.current.values()].map((read) => read());
      if (!await settleUnsavedChanges(changes, action)) { setError(true); return; }
      const next = pending.current;
      pending.current = null;
      setIsOpen(false);
      next?.();
    } catch {
      setError(true);
    } finally {
      busy.current = false;
      setSaving(false);
    }
  }
  const prompt = isOpen ? (
    <ModalLayer onClose={stay} purpose="form" label={t('unsaved.title')}>
      <section className="modal-panel unsaved-changes-dialog">
        <h2>{t('unsaved.title')}</h2>
        <p>{t('unsaved.body')}</p>
        {error && <p className="inline-error" role="alert">{t('unsaved.saveFailed')}</p>}
        <div className="dialog-actions">
          <XButton variant="secondary" label={t('unsaved.stay')} isDisabled={saving} onClick={stay} />
          <XButton variant="secondary" label={t('unsaved.discard')} isDisabled={saving} onClick={() => void leave('discard')} />
          <XButton variant="primary" label={t('unsaved.save')} isLoading={saving} isDisabled={saving} onClick={() => void leave('save')} />
        </div>
      </section>
    </ModalLayer>
  ) : null;
  const value = useMemo(() => ({ register, hasUnsavedChanges, requestNavigation, prompt }), [register, hasUnsavedChanges, requestNavigation, prompt]);
  return <Context.Provider value={value}>{children}</Context.Provider>;

}

export function useNavigationGuard() {
  const context = useContext(Context);
  if (!context) throw new Error('Navigation guard requires UnsavedChangesProvider');
  return context;
}

export function useUnsavedChanges(change: UnsavedChange | (() => UnsavedChange)) {
  const { register } = useNavigationGuard();
  const key = useId();
  const latest = useRef(change);
  latest.current = change;
  useEffect(() => register(key, () => typeof latest.current === 'function' ? latest.current() : latest.current), [key, register]);
}
