import { useEffect, useRef } from 'react';
import { MoreVertical, type LucideIcon } from 'lucide-react';
import { Button } from './Button';
import { cn } from '../../utils/cn';

export type RowAction = {
  key: string;
  icon: LucideIcon;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  destructive?: boolean;
  iconClassName?: string;
};

type RowActionsMenuProps<T extends string | number> = {
  rowId: T;
  activeRowId: T | null;
  onActiveRowChange: (id: T | null) => void;
  actions: RowAction[];
};

export function RowActionsMenu<T extends string | number>({
  rowId,
  activeRowId,
  onActiveRowChange,
  actions,
}: RowActionsMenuProps<T>) {
  const containerRef = useRef<HTMLDivElement>(null);
  const isActive = activeRowId === rowId;

  useEffect(() => {
    if (!isActive) return;

    const handleMouseDown = (event: MouseEvent) => {
      if (containerRef.current?.contains(event.target as Node)) return;
      onActiveRowChange(null);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onActiveRowChange(null);
    };

    document.addEventListener('mousedown', handleMouseDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleMouseDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [isActive, onActiveRowChange]);

  const handleToggle = () => {
    onActiveRowChange(isActive ? null : rowId);
  };

  const handleActionClick = (action: RowAction) => {
    if (action.disabled) return;
    onActiveRowChange(null);
    action.onClick();
  };

  return (
    <div ref={containerRef} className="relative flex items-center justify-end">
      {isActive && (
        <div
          className="absolute right-full top-1/2 z-20 mr-2 flex max-w-[min(calc(100vw-3rem),480px)] -translate-y-1/2 flex-row flex-nowrap items-center gap-1 overflow-x-auto whitespace-nowrap rounded-md border bg-background p-1 shadow-lg animate-in fade-in zoom-in duration-150"
          onClick={event => event.stopPropagation()}
        >
          {actions.map(action => (
            <Button
              key={action.key}
              type="button"
              variant="ghost"
              size="icon"
              title={action.label}
              aria-label={action.label}
              disabled={action.disabled}
              onClick={() => handleActionClick(action)}
              className={cn('shrink-0', action.destructive && 'text-destructive')}
            >
              <action.icon className={cn('h-4 w-4', action.iconClassName)} />
            </Button>
          ))}
        </div>
      )}

      <Button
        type="button"
        variant="ghost"
        size="icon"
        title="操作"
        aria-label="操作"
        aria-expanded={isActive}
        onClick={handleToggle}
        className="shrink-0 rounded-full border bg-background/95 shadow-md backdrop-blur-sm"
      >
        <MoreVertical className="h-4 w-4" />
      </Button>
    </div>
  );
}
