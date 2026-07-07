import { type FormEvent, useState } from 'react';
import { Cloud, KeyRound, Settings, ShieldCheck } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { Switch as XSwitch } from '@astryxdesign/core/Switch';
import { TextArea as XTextArea } from '@astryxdesign/core/TextArea';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { useTranslation } from 'react-i18next';
import i18n from '../../i18n';
import type { AstryxThemeName } from '../../shared/astryxThemes';
import { AstryxThemeSelector, LanguageSelector, ThemeToggle } from '../../shared/theme';
import { api } from '../../shared/api';
import { SectionTitle } from '../../shared/components/Feedback';
import type { ThemeMode, Toast, User } from '../../shared/types';
import { runAction } from '../../shared/utils';

export function SetupWizard({
  onComplete,
  setToast,
  themeMode,
  onThemeModeChange,
  astryxThemeName,
  onAstryxThemeChange,
}: {
  onComplete: (user: User) => Promise<void>;
  setToast: (toast: Toast) => void;
  themeMode: ThemeMode;
  onThemeModeChange: (mode: ThemeMode) => void;
  astryxThemeName: AstryxThemeName;
  onAstryxThemeChange: (theme: AstryxThemeName) => void;
}) {
  const { t } = useTranslation();
  const [form, setForm] = useState({
    username: 'admin',
    email: '',
    password: '',
    confirmPassword: '',
    sourcePasswordEnabled: true,
    sourcePassword: '',
    githubDownloadMirrors: '',
    githubRawMirrors: '',
    requireEmailVerify: false,
  });
  const [submitting, setSubmitting] = useState(false);
  const currentLanguage = (i18n.resolvedLanguage || i18n.language).startsWith('en') ? 'en' : 'zh';

  async function submitSetup(event: FormEvent) {
    event.preventDefault();
    if (form.password !== form.confirmPassword) {
      setToast({ tone: 'error', message: t('setup.passwordMismatch') });
      return;
    }
    if (form.sourcePasswordEnabled && !form.sourcePassword.trim()) {
      setToast({ tone: 'error', message: t('setup.sourcePasswordRequired') });
      return;
    }
    setSubmitting(true);
    await runAction(setToast, t('setup.failed'), async () => {
      const data = await api<{ user: User }>('/api/v1/setup', {
        method: 'POST',
        body: JSON.stringify({
          username: form.username,
          email: form.email,
          password: form.password,
          sourcePasswordEnabled: form.sourcePasswordEnabled,
          sourcePassword: form.sourcePassword,
          githubDownloadMirrors: form.githubDownloadMirrors,
          githubRawMirrors: form.githubRawMirrors,
          requireEmailVerify: form.requireEmailVerify,
        }),
      });
      setToast({ tone: 'success', message: t('setup.completed') });
      await onComplete(data.user);
    });
    setSubmitting(false);
  }

  return (
    <main className="setup-shell">
      <div className="setup-panel">
        <section className="setup-copy">
          <span className="eyebrow subtle">{t('setup.eyebrow')}</span>
          <h1>{t('setup.title')}</h1>
          <p>{t('setup.body')}</p>
          <div className="setup-steps" aria-label={t('setup.stepsLabel')}>
            <div><ShieldCheck size={18} /> {t('setup.stepAdmin')}</div>
            <div><Settings size={18} /> {t('setup.stepPolicy')}</div>
            <div><Cloud size={18} /> {t('setup.stepSource')}</div>
          </div>
        </section>
        <form className="panel form-panel setup-form" onSubmit={submitSetup}>
          <div className="form-topline">
            <SectionTitle icon={KeyRound} title={t('setup.formTitle')} />
            <LanguageSelector value={currentLanguage} onChange={(language) => void i18n.changeLanguage(language)} />
            <ThemeToggle mode={themeMode} onChange={onThemeModeChange} />
            <AstryxThemeSelector value={astryxThemeName} onChange={onAstryxThemeChange} />
          </div>
          <XTextInput label={t('common.username')} value={form.username} onChange={(value) => setForm({ ...form, username: value })} />
          <XTextInput type="email" label={t('common.email')} value={form.email} onChange={(value) => setForm({ ...form, email: value })} />
          <XTextInput type="password" label={t('common.password')} value={form.password} onChange={(value) => setForm({ ...form, password: value })} />
          <XTextInput type="password" label={t('setup.confirmPassword')} value={form.confirmPassword} onChange={(value) => setForm({ ...form, confirmPassword: value })} />
          <XSwitch
            label={t('setup.protectSource')}
            value={form.sourcePasswordEnabled}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => setForm({ ...form, sourcePasswordEnabled: checked })}
          />
          {form.sourcePasswordEnabled && (
            <XTextInput type="password" label={t('sources.password')} value={form.sourcePassword} onChange={(value) => setForm({ ...form, sourcePassword: value })} />
          )}
          <XTextArea
            label={t('admin.settings.githubDownloadMirrors')}
            description={t('admin.settingsHelp.githubDownloadMirrors')}
            value={form.githubDownloadMirrors}
            rows={3}
            onChange={(value) => setForm({ ...form, githubDownloadMirrors: value })}
          />
          <XTextArea
            label={t('admin.settings.githubRawMirrors')}
            description={t('admin.settingsHelp.githubRawMirrors')}
            value={form.githubRawMirrors}
            rows={3}
            onChange={(value) => setForm({ ...form, githubRawMirrors: value })}
          />
          <XSwitch
            label={t('setup.requireEmailVerify')}
            value={form.requireEmailVerify}
            labelSpacing="spread"
            width="100%"
            onChange={(checked) => setForm({ ...form, requireEmailVerify: checked })}
          />
          <XButton
            type="submit"
            variant="primary"
            label={submitting ? t('setup.submitting') : t('setup.finish')}
            icon={<ShieldCheck size={18} />}
            isDisabled={submitting}
          />
        </form>
      </div>
    </main>
  );
}
