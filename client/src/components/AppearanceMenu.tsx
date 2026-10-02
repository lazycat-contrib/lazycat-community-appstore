import { Settings2, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { IconButton as XIconButton } from '@astryxdesign/core/IconButton';
import { ModalLayer } from '../shared/components/ModalLayer';
import { AstryxThemeSelector, LanguageSelector, ThemeToggle, type LanguageCode } from '../shared/theme';
import type { ThemeMode } from '../shared/types';
import type { AstryxThemeName } from '../shared/astryxThemes';

export function AppearanceMenu({ language, themeMode, themeName, onLanguageChange, onThemeModeChange, onThemeChange }: {
  language: LanguageCode;
  themeMode: ThemeMode;
  themeName: AstryxThemeName;
  onLanguageChange: (language: LanguageCode) => void;
  onThemeModeChange: (mode: ThemeMode) => void;
  onThemeChange: (theme: AstryxThemeName) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <>
      <XIconButton label={t('appearance.title')} variant="ghost" icon={<Settings2 size={19} />} onClick={() => setOpen(true)} />
      {open && (
        <ModalLayer onClose={() => setOpen(false)} label={t('appearance.title')} width="min(380px, calc(100vw - 32px))">
          <section className="modal-panel appearance-dialog">
            <header className="appearance-dialog-head"><h2>{t('appearance.title')}</h2><XIconButton label={t('common.close')} variant="ghost" icon={<X size={18} />} onClick={() => setOpen(false)} /></header>
            <div className="appearance-setting"><span>{t('language.label')}</span><LanguageSelector value={language} onChange={onLanguageChange} /></div>
            <div className="appearance-setting"><span>{t('theme.label')} · {t(`theme.modes.${themeMode}`)}</span><ThemeToggle mode={themeMode} onChange={onThemeModeChange} /></div>
            <div className="appearance-setting"><span>{t('theme.selector')}</span><AstryxThemeSelector value={themeName} onChange={onThemeChange} /></div>
          </section>
        </ModalLayer>
      )}
    </>
  );
}
