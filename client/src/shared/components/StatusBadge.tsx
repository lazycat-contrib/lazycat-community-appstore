import { type ReactNode } from 'react';
import { Badge as XBadge, type BadgeVariant } from '@astryxdesign/core/Badge';

export type StatusBadgeTone =
  | 'approved'
  | 'synced'
  | 'success'
  | 'pending'
  | 'syncing'
  | 'info'
  | 'rejected'
  | 'blocked'
  | 'failed'
  | 'error'
  | 'unlisted'
  | 'draft'
  | 'auth'
  | 'stale'
  | 'unsynced'
  | 'warning'
  | 'neutral'
  | string;

export function statusBadgeVariant(tone?: StatusBadgeTone): BadgeVariant {
  switch (tone) {
    case 'approved':
    case 'synced':
    case 'success':
      return 'success';
    case 'pending':
    case 'syncing':
    case 'info':
      return 'info';
    case 'rejected':
    case 'blocked':
    case 'failed':
    case 'error':
      return 'error';
    case 'unlisted':
    case 'draft':
    case 'auth':
    case 'stale':
    case 'unsynced':
    case 'warning':
      return 'warning';
    default:
      return 'neutral';
  }
}

export function StatusBadge({
  tone,
  label,
  icon,
  className,
  'aria-live': ariaLive,
}: {
  tone?: StatusBadgeTone;
  label: ReactNode;
  icon?: ReactNode;
  className?: string;
  'aria-live'?: 'off' | 'polite' | 'assertive';
}) {
  return <XBadge className={className} variant={statusBadgeVariant(tone)} icon={icon} label={label} aria-live={ariaLive} />;
}
