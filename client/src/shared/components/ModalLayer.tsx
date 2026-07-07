import type { ReactNode } from 'react';
import { Dialog as XDialog, type DialogPurpose } from '@astryxdesign/core/Dialog';

export function ModalLayer({
  children,
  onClose,
  purpose = 'info',
  width = 'min(560px, calc(100vw - 36px))',
  maxHeight = 'calc(100vh - 36px)',
  className,
}: {
  children: ReactNode;
  onClose: () => void;
  purpose?: DialogPurpose;
  width?: number | string;
  maxHeight?: number | string;
  className?: string;
}) {
  return (
    <XDialog
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
