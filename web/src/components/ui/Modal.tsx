import { useEffect, type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { cn } from '../../utils/cn';
import { useConfirm } from '../../context/confirm';
import { Button } from './Button';
interface ModalProps {
  isOpen: boolean; onClose: () => void; title: string; children: ReactNode;
  footer?: ReactNode; className?: string; variant?: 'dialog' | 'drawer'; dirty?: boolean; busy?: boolean;
}
export function Modal({ isOpen, onClose, title, children, footer, className, variant = 'dialog', dirty = false, busy = false }: ModalProps) {
  const confirm = useConfirm();
  useEffect(() => {
    if (!isOpen || !dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [isOpen, dirty]);
  const close = async () => {
    if (busy) return;
    if (dirty && !await confirm({ title: '放弃未保存的修改？', message: '关闭后，本次尚未保存的表单内容将丢失。后台解析任务仍可在任务中心查看。', confirmLabel: '放弃修改' })) return;
    onClose();
  };
  return <Dialog.Root open={isOpen} onOpenChange={open => { if (!open) void close(); }}>
    <Dialog.Portal>
      <Dialog.Overlay className="fixed inset-0 z-50 bg-black/35 backdrop-blur-xs" />
      <Dialog.Content aria-describedby={undefined} onInteractOutside={event => event.preventDefault()} className={cn(
        'dialog-content fixed z-50 flex w-full flex-col overflow-hidden border bg-card shadow-xl outline-none',
        variant === 'drawer' ? 'inset-y-0 right-0 max-w-3xl sm:border-l' : 'left-1/2 top-1/2 max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-xl', className,
      )}>
        <div className="flex shrink-0 items-center justify-between gap-4 border-b px-5 py-4 sm:px-6"><Dialog.Title className="min-w-0 truncate text-lg font-semibold tracking-tight">{title}</Dialog.Title><Button type="button" variant="ghost" size="icon" aria-label="关闭" disabled={busy} onClick={() => void close()}><X className="size-5" /></Button></div>
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-5 sm:p-6">{children}</div>
        {footer && <div className="flex shrink-0 flex-wrap items-center justify-end gap-2 border-t bg-card px-5 py-4 pb-[max(1rem,env(safe-area-inset-bottom))] sm:px-6">{footer}</div>}
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>;
}
