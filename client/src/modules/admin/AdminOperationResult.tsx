import { Button as XButton } from '@astryxdesign/core/Button';
import { CheckCircle2, Circle, Info, RefreshCw, TriangleAlert, XCircle } from 'lucide-react';
import { formatDate } from '../../shared/utils';
import type { AdminOperationResult } from './adminState';

function resultIcon(variant: AdminOperationResult['variant']) {
  if (variant === 'success') return <CheckCircle2 size={16} />;
  if (variant === 'error') return <XCircle size={16} />;
  if (variant === 'warning') return <TriangleAlert size={16} />;
  if (variant === 'info') return <Info size={16} />;
  return <Circle size={16} />;
}

export function AdminOperationResultPanel({
  result,
  retryLabel,
  onRetry,
  isRetrying = false,
  isRetryDisabled = false,
}: {
  result: AdminOperationResult | null;
  retryLabel?: string;
  onRetry?: () => void;
  isRetrying?: boolean;
  isRetryDisabled?: boolean;
}) {
  if (!result) return null;
  return (
    <section className="admin-operation-result" data-variant={result.variant}>
      <div className="admin-operation-result-copy" role={result.variant === 'error' ? 'alert' : 'status'} aria-atomic="true">
        <div className="admin-operation-result-head">
          <div>
            <strong>{result.title}</strong>
            <time dateTime={result.occurredAt}>{formatDate(result.occurredAt)}</time>
          </div>
          <span className="admin-operation-result-icon" aria-hidden="true">{resultIcon(result.variant)}</span>
        </div>
        <p className="admin-operation-result-message">{result.message}</p>
        {result.target && <code>{result.target}</code>}
      </div>
      {onRetry && retryLabel && (
        <XButton type="button" variant="secondary" size="sm" label={retryLabel} icon={<RefreshCw size={16} />} isLoading={isRetrying} isDisabled={isRetrying || isRetryDisabled} onClick={onRetry} />
      )}
    </section>
  );
}
