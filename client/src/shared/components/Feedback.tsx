import type { LucideIcon } from 'lucide-react';

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
        <button type="button" className="secondary-button" onClick={action.onClick}>
          {ActionIcon && <ActionIcon size={18} />}
          <span>{action.label}</span>
        </button>
      )}
    </div>
  );
}
