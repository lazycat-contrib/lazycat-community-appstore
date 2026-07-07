import type { LucideIcon } from 'lucide-react';
import { Button as XButton } from '@astryxdesign/core/Button';
import { EmptyState as XEmptyState } from '@astryxdesign/core/EmptyState';

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
    <XEmptyState
      icon={<Icon size={28} />}
      title={title}
      description={body}
      actions={action && (
        <XButton type="button" variant="secondary" label={action.label} icon={ActionIcon ? <ActionIcon size={18} /> : undefined} onClick={action.onClick} />
      )}
    />
  );
}
