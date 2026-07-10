import { Cloud, Plus } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { useTranslation } from 'react-i18next';

export function SourceOnboarding({ onAdd }: { onAdd: () => void }) {
  const { t } = useTranslation();
  return (
    <section className="client-source-onboarding" aria-labelledby="client-source-onboarding-title">
      <span className="client-source-onboarding-icon" aria-hidden="true"><Cloud size={28} /></span>
      <div>
        <span className="eyebrow subtle">{t('mode.standaloneClient')}</span>
        <h1 id="client-source-onboarding-title">{t('sources.onboardingTitle')}</h1>
        <p>{t('sources.onboardingBody')}</p>
      </div>
      <XButton type="button" variant="primary" label={t('sources.add')} icon={<Plus size={18} />} onClick={onAdd} />
    </section>
  );
}
