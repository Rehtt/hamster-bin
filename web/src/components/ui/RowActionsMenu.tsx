import * as DropdownMenu from '@radix-ui/react-dropdown-menu';
import { MoreHorizontal, type LucideIcon } from 'lucide-react';
import { Button } from './Button';
import { cn } from '../../utils/cn';
export type RowAction = { key: string; icon: LucideIcon; label: string; onClick: () => void; disabled?: boolean; destructive?: boolean; iconClassName?: string };
export function RowActionsMenu<T extends string | number>({ rowId, activeRowId, onActiveRowChange, actions }: { rowId: T; activeRowId: T | null; onActiveRowChange: (id: T | null) => void; actions: RowAction[] }) {
  return <DropdownMenu.Root open={activeRowId === rowId} onOpenChange={open => onActiveRowChange(open ? rowId : null)}>
    <DropdownMenu.Trigger asChild><Button type="button" variant="ghost" size="icon" aria-label="操作" title="操作"><MoreHorizontal className="size-5" /></Button></DropdownMenu.Trigger>
    <DropdownMenu.Portal><DropdownMenu.Content align="end" sideOffset={6} className="z-[65] min-w-40 rounded-lg border bg-popover p-1.5 text-popover-foreground shadow-lg">
      {actions.map(action => <DropdownMenu.Item key={action.key} disabled={action.disabled} onSelect={action.onClick} className={cn('flex cursor-pointer items-center gap-3 rounded-md px-3 py-2.5 text-sm outline-none data-[highlighted]:bg-accent data-[disabled]:pointer-events-none data-[disabled]:opacity-40', action.destructive && 'text-destructive')}><action.icon className={cn('size-4', action.iconClassName)} />{action.label}</DropdownMenu.Item>)}
    </DropdownMenu.Content></DropdownMenu.Portal>
  </DropdownMenu.Root>;
}
