import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Badge as XBadge } from '@astryxdesign/core/Badge';

export type AdminTaskHeaderProps = {
  icon: LucideIcon;
  title: string;
  body: string;
  statusLabel?: string;
  statusVariant?: 'neutral' | 'success' | 'warning' | 'error' | 'info';
  action?: ReactNode;
};

export function AdminTaskHeader({
  icon: Icon,
  title,
  body,
  statusLabel,
  statusVariant = 'neutral',
  action,
}: AdminTaskHeaderProps) {
  return (
    <header className="admin-task-header">
      <div className="admin-task-heading">
        <span className="admin-task-icon" aria-hidden="true"><Icon size={20} /></span>
        <div>
          <h2>{title}</h2>
          <p>{body}</p>
        </div>
      </div>
      <div className="admin-task-header-actions">
        {statusLabel && <XBadge label={statusLabel} variant={statusVariant} />}
        {action}
      </div>
    </header>
  );
}
