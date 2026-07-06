import { Monitor, Moon, Sun } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { THEME_STORAGE_KEY } from './constants';
import type { ResolvedTheme, ThemeMode } from './types';

export function readThemeMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    return saved === 'light' || saved === 'dark' || saved === 'system' ? saved : 'system';
  } catch {
    return 'system';
  }
}

export function readSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined' || !window.matchMedia) return 'light';
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export function nextThemeMode(mode: ThemeMode): ThemeMode {
  if (mode === 'system') return readSystemTheme() === 'dark' ? 'light' : 'dark';
  if (mode === 'light') return 'dark';
  return 'system';
}

export function ThemeToggle({ mode, onChange }: { mode: ThemeMode; onChange: (mode: ThemeMode) => void }) {
  const { t } = useTranslation();
  const Icon = mode === 'system' ? Monitor : mode === 'dark' ? Moon : Sun;
  const label = t('theme.toggle', { mode: t(`theme.modes.${mode}`) });
  return (
    <button type="button" className="icon-button" aria-label={label} title={label} onClick={() => onChange(nextThemeMode(mode))}>
      <Icon size={18} />
    </button>
  );
}
