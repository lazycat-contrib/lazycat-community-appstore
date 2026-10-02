import { useId, useLayoutEffect, useRef, type ReactNode } from 'react';
import { Dialog as XDialog, type DialogPurpose } from '@astryxdesign/core/Dialog';

export function ModalLayer({
  children,
  onClose,
  purpose = 'info',
  width = 'min(560px, calc(100vw - 36px))',
  maxHeight = 'calc(100vh - 36px)',
  className,
  label,
}: {
  children: ReactNode;
  onClose: () => void;
  purpose?: DialogPurpose;
  width?: number | string;
  maxHeight?: number | string;
  className?: string;
  label?: string;
}) {
  const id = useId();
  const triggerRef = useRef(document.activeElement instanceof HTMLElement ? document.activeElement : null);
  useLayoutEffect(() => {
    const trigger = triggerRef.current;
    const dialog = document.getElementById(id);
    const heading = dialog?.querySelector('h1, h2, h3');
    if (!label && heading) {
      if (!heading.id) heading.id = `${id}-title`;
      dialog?.setAttribute('aria-labelledby', heading.id);
    }
    return () => {
      queueMicrotask(() => {
        const openDialogs = [...document.querySelectorAll('dialog[open]')];
        const topDialog = openDialogs.at(-1);
        if (trigger?.isConnected && trigger !== document.body && (!topDialog || topDialog.contains(trigger))) {
          trigger.focus({ preventScroll: true });
        }
      });
    };
  }, [id, label]);
  return (
    <XDialog
      id={id}
      aria-label={label}
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      purpose={purpose}
      width={width}
      maxHeight={maxHeight}
      padding={0}
      className={className ? `modal-dialog-shell ${className}` : 'modal-dialog-shell'}
    >
      {children}
    </XDialog>
  );
}
