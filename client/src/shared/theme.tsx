import { Monitor, Moon, Sun } from 'lucide-react';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { Selector as XSelector } from '@astryxdesign/core/Selector';
import { useTranslation } from 'react-i18next';
import { ASTRYX_THEME_STORAGE_KEY, THEME_STORAGE_KEY } from './constants';
import { ASTRYX_THEMES, getAstryxTheme, type AstryxThemeName } from './astryxThemes';
import type { ResolvedTheme, ThemeMode } from './types';

export type LanguageCode = 'zh' | 'en';

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

export function readAstryxThemeName(): AstryxThemeName {
  try {
    return getAstryxTheme(localStorage.getItem(ASTRYX_THEME_STORAGE_KEY)).name;
  } catch {
    return 'neutral';
  }
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
    <XIconButton type="button" variant="ghost" label={label} tooltip={label} icon={<Icon size={18} />} onClick={() => onChange(nextThemeMode(mode))} />
  );
}

export function AstryxThemeSelector({ value, onChange }: { value: AstryxThemeName; onChange: (theme: AstryxThemeName) => void }) {
  const { t } = useTranslation();
  return (
    <XSelector
      label={t('theme.selector')}
      isLabelHidden
      size="sm"
      width={164}
      options={ASTRYX_THEMES.map((theme) => ({ value: theme.name, label: t(theme.labelKey) }))}
      value={value}
      onChange={(nextTheme) => onChange(getAstryxTheme(nextTheme).name)}
    />
  );
}

export function LanguageSelector({ value, onChange }: { value: LanguageCode; onChange: (language: LanguageCode) => void }) {
  const { t } = useTranslation();
  return (
    <XSelector
      label={t('language.label')}
      isLabelHidden
      size="sm"
      width={152}
      options={[
        { value: 'zh', label: t('language.zh') },
        { value: 'en', label: t('language.en') },
      ]}
      value={value}
      onChange={(nextLanguage) => onChange(nextLanguage === 'en' ? 'en' : 'zh')}
    />
  );
}
