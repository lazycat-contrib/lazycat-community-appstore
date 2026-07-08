import { type FormEvent, useEffect, useState } from 'react';
import { Archive, Check, Home, KeyRound, LogIn, PackagePlus, Plus, Search, ShieldCheck, Users } from 'lucide-react';
import { ClawCaptcha } from 'playcaptcha';
import 'playcaptcha/clawcaptcha.css';
import { Button as XButton } from '@astryxdesign/core/Button';
import { TextInput as XTextInput } from '@astryxdesign/core/TextInput';
import { ToggleButton as XToggleButton, ToggleButtonGroup as XToggleButtonGroup } from '@astryxdesign/core/ToggleButton';
import { useTranslation } from 'react-i18next';
import type { AstryxThemeName } from '../../shared/astryxThemes';
import { AstryxThemeSelector, LanguageSelector, ThemeToggle, type LanguageCode } from '../../shared/theme';
import { api, ApiRequestError } from '../../shared/api';
import { SectionTitle } from '../../shared/components/Feedback';
import type { SiteProfile, ThemeMode, Toast, User } from '../../shared/types';
import { runAction } from '../../shared/utils';

type AuthMode = 'login' | 'register' | 'verify';

function verificationTokenFromURL() {
  const params = new URLSearchParams(window.location.search);
  if (window.location.pathname.includes('verify')) return params.get('token') || '';
  if (window.location.pathname === '/login' && params.get('mode') === 'verify') return params.get('token') || '';
  return '';
}

function authModeFromURL(): AuthMode {
  if (verificationTokenFromURL()) return 'verify';
  const mode = new URLSearchParams(window.location.search).get('mode');
  return mode === 'register' || mode === 'verify' ? mode : 'login';
}

export function LoginPage({
  siteTitle,
  siteProfile,
  currentLanguage,
  themeMode,
  astryxThemeName,
  onLanguageChange,
  onThemeModeChange,
  onAstryxThemeChange,
  onBrowse,
  onAuthenticated,
  setUser,
  refreshAll,
  setToast,
}: {
  siteTitle: string;
  siteProfile: SiteProfile;
  currentLanguage: LanguageCode;
  themeMode: ThemeMode;
  astryxThemeName: AstryxThemeName;
  onLanguageChange: (language: LanguageCode) => void;
  onThemeModeChange: (mode: ThemeMode) => void;
  onAstryxThemeChange: (name: AstryxThemeName) => void;
  onBrowse: () => void;
  onAuthenticated: (user: User) => void;
  setUser: (user: User | null) => void;
  refreshAll: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
}) {
  return (
    <div className="login-shell">
      <header className="login-topbar">
        <XButton type="button" variant="ghost" label={siteTitle} className="brand login-brand" onClick={onBrowse}>
          <div className="brand-mark">
            {siteProfile.iconUrl ? <img src={siteProfile.iconUrl} alt="" /> : <Archive size={22} />}
          </div>
          <strong>{siteTitle}</strong>
        </XButton>
        <div className="top-actions">
          <LanguageSelector value={currentLanguage} onChange={onLanguageChange} />
          <ThemeToggle mode={themeMode} onChange={onThemeModeChange} />
          <AstryxThemeSelector value={astryxThemeName} onChange={onAstryxThemeChange} />
        </div>
      </header>
      <main className="login-main" id="main-content" tabIndex={-1}>
        <AuthGateway
          siteProfile={siteProfile}
          setUser={setUser}
          refreshAll={refreshAll}
          setToast={setToast}
          onBrowse={onBrowse}
          onAuthenticated={onAuthenticated}
        />
      </main>
    </div>
  );
}

function AuthGateway({
  siteProfile,
  setUser,
  refreshAll,
  setToast,
  onBrowse,
  onAuthenticated,
}: {
  siteProfile: SiteProfile;
  setUser: (user: User | null) => void;
  refreshAll: (options?: { silent?: boolean }) => Promise<void>;
  setToast: (toast: Toast) => void;
  onBrowse: () => void;
  onAuthenticated: (user: User) => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<AuthMode>(authModeFromURL);
  const [authForm, setAuthForm] = useState({ username: '', password: '', email: '', inviteCode: '' });
  const [verifyToken, setVerifyToken] = useState(verificationTokenFromURL);
  const [captchaRequired, setCaptchaRequired] = useState(false);
  const [captchaVerified, setCaptchaVerified] = useState(false);
  const [failedAttempts, setFailedAttempts] = useState(0);
  const registrationMode = siteProfile.registration?.mode || 'open';
  const registrationOpen = registrationMode !== 'closed';
  const inviteRegistration = registrationMode === 'invite';
  const authModeLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verify');
  const authSubmitLabel = mode === 'login' ? t('auth.login') : mode === 'register' ? t('auth.register') : t('auth.verifyEmail');
  const authHint = mode === 'login'
    ? t('auth.loginHint')
    : mode === 'register'
      ? t(inviteRegistration ? 'auth.registerInviteHint' : 'auth.registerHint')
      : t('auth.verifyHint');
  const AuthSubmitIcon = mode === 'verify' ? Check : mode === 'register' ? Plus : LogIn;

  useEffect(() => {
    if (!registrationOpen && mode === 'register') {
      setMode('login');
    }
  }, [mode, registrationOpen]);

  useEffect(() => {
    if (verifyToken) setMode('verify');
  }, [verifyToken]);

  useEffect(() => {
    if (mode !== 'login') {
      setCaptchaRequired(false);
      setCaptchaVerified(false);
      setFailedAttempts(0);
    }
  }, [mode]);

  function formString(formData: FormData, key: string, fallback = '') {
    const value = formData.get(key);
    return typeof value === 'string' ? value : fallback;
  }

  async function submitVerificationToken(token: string) {
    await runAction(setToast, t('auth.verifyFailed'), async () => {
      const data = await api<{ user: User }>('/api/v1/auth/verify-email', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      setUser(data.user);
      setToast({ tone: 'success', message: t('auth.emailVerified') });
      await refreshAll({ silent: true });
      onAuthenticated(data.user);
    });
  }

  async function submitAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    if (mode === 'verify') {
      await submitVerificationToken(formString(formData, 'token', verifyToken));
      return;
    }
    const submittedForm = {
      username: formString(formData, 'username', authForm.username),
      password: formString(formData, 'password', authForm.password),
      email: formString(formData, 'email', authForm.email),
      inviteCode: formString(formData, 'inviteCode', authForm.inviteCode),
    };
    if (mode === 'login') {
      if (captchaRequired && !captchaVerified) {
        setToast({ tone: 'neutral', message: t('auth.captchaRequired') });
        return;
      }
      try {
        const data = await api<{ user: User }>('/api/v1/auth/login', {
          method: 'POST',
          body: JSON.stringify({ username: submittedForm.username, password: submittedForm.password }),
        });
        setCaptchaRequired(false);
        setCaptchaVerified(false);
        setFailedAttempts(0);
        setUser(data.user);
        if (data.user.emailVerified === false) {
          setMode('verify');
          setToast({ tone: 'neutral', message: t('auth.completeEmailVerification') });
        } else {
          setToast({ tone: 'success', message: t('auth.loggedIn') });
          onAuthenticated(data.user);
        }
        await refreshAll({ silent: true });
      } catch (error) {
        const details = error instanceof ApiRequestError && error.details && typeof error.details === 'object'
          ? error.details as { failedAttempts?: number; captchaRequired?: boolean }
          : null;
        if (details?.failedAttempts) setFailedAttempts(details.failedAttempts);
        if (details?.captchaRequired) {
          setCaptchaRequired(true);
          setCaptchaVerified(false);
        }
        setToast({ tone: 'error', message: error instanceof Error && error.message ? error.message : t('auth.loginFailed') });
      }
      return;
    }
    await runAction(setToast, t('auth.registerFailed'), async () => {
      const data = await api<{ user: User }>(`/api/v1/auth/${mode}`, {
        method: 'POST',
        body: JSON.stringify(submittedForm),
      });
      setUser(data.user);
      if (data.user.emailVerified === false) {
        setMode('verify');
        setToast({ tone: 'neutral', message: t('auth.completeEmailVerification') });
      } else {
        setToast({ tone: 'success', message: t('auth.registered') });
        onAuthenticated(data.user);
      }
      await refreshAll({ silent: true });
    });
  }

  return (
    <section className="page-grid auth-gateway login-auth-gateway">
      <div className="page-heading">
        <span className="eyebrow subtle">{t('auth.entryEyebrow')}</span>
        <h1>{t('auth.entryTitle')}</h1>
        <p>{t('auth.entryBody')}</p>
      </div>
      <div className="split auth-split">
        <form className="panel form-panel profile-panel auth-panel" onSubmit={submitAuth}>
          <SectionTitle icon={KeyRound} title={mode === 'verify' ? t('auth.verifyEmail') : authModeLabel} />
          <p className="inline-note">{authHint}</p>
          <XToggleButtonGroup value={mode} onChange={(value) => value && setMode(value as AuthMode)} label={t('auth.modeSwitch')} size="sm">
            <XToggleButton value="login" label={t('auth.login')} />
            {registrationOpen && <XToggleButton value="register" label={t('auth.register')} />}
            <XToggleButton value="verify" label={t('auth.verify')} />
          </XToggleButtonGroup>
          {mode === 'verify' ? (
            <XTextInput htmlName="token" label={t('auth.verifyToken')} value={verifyToken} isRequired onChange={setVerifyToken} />
          ) : (
            <>
              <XTextInput htmlName="username" label={t('common.username')} value={authForm.username} isRequired onChange={(value) => setAuthForm({ ...authForm, username: value })} />
              {mode === 'register' && (
                <XTextInput htmlName="email" type="email" label={t('common.email')} value={authForm.email} onChange={(value) => setAuthForm({ ...authForm, email: value })} />
              )}
              {mode === 'register' && inviteRegistration && (
                <XTextInput htmlName="inviteCode" label={t('auth.inviteCode')} value={authForm.inviteCode} isRequired onChange={(value) => setAuthForm({ ...authForm, inviteCode: value })} />
              )}
              <XTextInput
                htmlName="password"
                type="password"
                label={t('common.password')}
                description={mode === 'register' ? t('auth.passwordHelp') : undefined}
                value={authForm.password}
                isRequired
                onChange={(value) => setAuthForm({ ...authForm, password: value })}
              />
              {mode === 'login' && captchaRequired && (
                <div className="captcha-panel" role="group" aria-label={t('auth.captchaTitle')}>
                  <ClawCaptcha title={t('auth.captchaTitle')} onVerify={() => setCaptchaVerified(true)} />
                  <p className={captchaVerified ? 'inline-success' : 'inline-warning'}>
                    <ShieldCheck size={15} />
                    <span>{captchaVerified ? t('auth.captchaVerified') : t('auth.captchaBody', { count: failedAttempts })}</span>
                  </p>
                </div>
              )}
            </>
          )}
          <XButton type="submit" variant="primary" label={authSubmitLabel} icon={<AuthSubmitIcon size={18} />} isDisabled={mode === 'login' && captchaRequired && !captchaVerified} />
        </form>

        <section className="panel auth-path-panel">
          <SectionTitle icon={Users} title={t('auth.entryPaths')} />
          <div className="auth-path-list">
            <div className="auth-path-row">
              <Search size={19} />
              <div>
                <strong>{t('auth.pathBrowseTitle')}</strong>
                <span>{t('auth.pathBrowseBody')}</span>
              </div>
              <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathBrowseAction')} icon={<Home size={17} />} onClick={onBrowse} />
            </div>
            {registrationOpen && (
              <div className="auth-path-row">
                <PackagePlus size={19} />
                <div>
                  <strong>{t('auth.pathSubmitTitle')}</strong>
                  <span>{t(inviteRegistration ? 'auth.pathSubmitInviteBody' : 'auth.pathSubmitBody')}</span>
                </div>
                <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathSubmitAction')} icon={<Plus size={17} />} onClick={() => setMode('register')} />
              </div>
            )}
            <div className="auth-path-row">
              <ShieldCheck size={19} />
              <div>
                <strong>{t('auth.pathAdminTitle')}</strong>
                <span>{t('auth.pathAdminBody')}</span>
              </div>
              <XButton className="auth-path-action" type="button" variant="secondary" size="sm" label={t('auth.pathAdminAction')} icon={<LogIn size={17} />} onClick={() => setMode('login')} />
            </div>
          </div>
        </section>
      </div>
    </section>
  );
}
