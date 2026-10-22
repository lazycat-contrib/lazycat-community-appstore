import { RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button as XButton } from '@astryxdesign/core/Button';

export function CatalogResultSummary({ label, count, onReset }: {
  label: string;
  count: number;
  onReset?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="catalog-result-summary">
      <div className="catalog-result-count" role="status" aria-live="polite" aria-atomic="true">
        <span>{label}</span>
        <strong>{t('search.resultCount', { count })}</strong>
      </div>
      {onReset && (
        <XButton type="button" variant="ghost" label={t('search.clearFilters')} icon={<RotateCcw size={16} aria-hidden="true" />} onClick={onReset} />
      )}
    </div>
  );
}
