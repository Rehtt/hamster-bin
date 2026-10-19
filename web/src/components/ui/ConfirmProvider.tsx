import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import * as AlertDialog from '@radix-ui/react-alert-dialog';
import { ConfirmContext, type ConfirmOptions } from '../../context/confirm';
import { Button } from './Button';
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [options, setOptions] = useState<ConfirmOptions | null>(null);
  const resolve = useRef<((value: boolean) => void) | null>(null);
  const confirm = useCallback((value: string | ConfirmOptions) => new Promise<boolean>(done => {
    resolve.current?.(false);
    resolve.current = done;
    setOptions(typeof value === 'string' ? { message: value } : value);
  }), []);
  useEffect(() => () => resolve.current?.(false), []);
  const finish = (value: boolean) => { resolve.current?.(value); resolve.current = null; setOptions(null); };
  return <ConfirmContext.Provider value={confirm}>{children}
    <AlertDialog.Root open={!!options} onOpenChange={open => { if (!open) finish(false); }}>
      <AlertDialog.Portal><AlertDialog.Overlay className="fixed inset-0 z-[80] bg-black/40 backdrop-blur-xs" />
        <AlertDialog.Content className="dialog-content fixed left-1/2 top-1/2 z-[81] w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl border bg-card p-6 shadow-xl">
          <AlertDialog.Title className="text-lg font-semibold">{options?.title || '确认操作'}</AlertDialog.Title>
          <AlertDialog.Description className="mt-3 whitespace-pre-line text-sm leading-6 text-muted-foreground">{options?.message}</AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2"><AlertDialog.Cancel asChild><Button variant="outline" onClick={() => finish(false)}>取消</Button></AlertDialog.Cancel><AlertDialog.Action asChild><Button variant={options?.destructive === false ? 'default' : 'destructive'} onClick={() => finish(true)}>{options?.confirmLabel || '确认'}</Button></AlertDialog.Action></div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  </ConfirmContext.Provider>;
}
