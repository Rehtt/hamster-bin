import { forwardRef, useImperativeHandle, useState, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../utils/cn';
import { Button } from './Button';

export type CollapsibleFilterPanelHandle = {
  collapse: () => void;
};

type CollapsibleFilterPanelProps = {
  title?: string;
  summary?: string;
  activeCount?: number;
  showClear?: boolean;
  onClear?: () => void;
  headerActions?: ReactNode;
  children: ReactNode;
};

export const CollapsibleFilterPanel = forwardRef<CollapsibleFilterPanelHandle, CollapsibleFilterPanelProps>(
  function CollapsibleFilterPanel(
    {
      title = '筛选条件',
      summary,
      activeCount = 0,
      showClear = false,
      onClear,
      headerActions,
      children,
    },
    ref,
  ) {
    const [expanded, setExpanded] = useState(false);

    useImperativeHandle(ref, () => ({
      collapse: () => setExpanded(false),
    }));

    return (
      <div className="rounded-md border">
        <div className="md:hidden flex items-center gap-2 p-3 border-b border-transparent">
          <button
            type="button"
            className="flex flex-1 items-center gap-2 min-w-0 text-left"
            onClick={() => setExpanded(prev => !prev)}
            aria-expanded={expanded}
          >
            <ChevronDown
              className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-180')}
            />
            <span className="text-sm font-medium shrink-0">{title}</span>
            {!expanded && summary ? (
              <span className="text-xs text-muted-foreground truncate">{summary}</span>
            ) : null}
            {activeCount > 0 ? (
              <span className="rounded-full bg-primary/10 text-primary px-2 py-0.5 text-xs shrink-0">
                {activeCount}
              </span>
            ) : null}
          </button>
          {!expanded && headerActions}
          {!expanded && showClear && onClear ? (
            <Button variant="outline" size="sm" onClick={onClear}>
              清空
            </Button>
          ) : null}
        </div>

        <div className={cn('space-y-4 p-4', !expanded && 'hidden md:block', expanded && 'pt-0 md:pt-4')}>
          {children}
        </div>
      </div>
    );
  },
);
