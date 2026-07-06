import type { LucideIcon } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';

export function SectionTitle({ icon: Icon, title }: { icon: LucideIcon; title: string }) {
  return (
    <div className="section-title">
      <Icon size={19} />
      <h2>{title}</h2>
    </div>
  );
}

export function EmptyState({
  icon: Icon,
  title,
  body,
  action,
}: {
  icon: LucideIcon;
  title: string;
  body?: string;
  action?: { label: string; icon?: LucideIcon; onClick: () => void };
}) {
  const ActionIcon = action?.icon;
  return (
    <div className="empty-state">
      <Icon size={28} />
      <strong>{title}</strong>
      {body && <p>{body}</p>}
      {action && (
        <XButton type="button" variant="secondary" label={action.label} icon={ActionIcon ? <ActionIcon size={18} /> : undefined} onClick={action.onClick} />
      )}
    </div>
  );
}
