import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical } from 'lucide-react';
import { Input } from '../ui/Input';

import { getColumnConfig, type ColumnState, type ExportColumnKey } from './columns';

type SortableColumnRowProps = {
  column: ColumnState;
  onToggle: (key: ExportColumnKey, selected: boolean) => void;
  onUpdateHeader: (key: ExportColumnKey, header: string) => void;
};

function SortableColumnRow({ column, onToggle, onUpdateHeader }: SortableColumnRowProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: column.key });

  const defaultHeader = getColumnConfig(column.key)?.defaultHeader || column.key;
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="grid grid-cols-1 sm:grid-cols-[auto_auto_1fr] gap-2 sm:gap-3 items-center px-3 py-3 border-b last:border-b-0"
    >
      <button
        type="button"
        className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-grab active:cursor-grabbing touch-none"
        title="拖动调整顺序"
        {...attributes}
        {...listeners}
      >
        <GripVertical className="h-4 w-4" />
      </button>
      <label className="flex items-center gap-2 text-sm whitespace-nowrap">
        <input
          type="checkbox"
          checked={column.selected}
          onChange={e => onToggle(column.key, e.target.checked)}
          className="h-4 w-4 rounded border-input"
        />
        {defaultHeader}
      </label>
      <Input
        value={column.header}
        onChange={e => onUpdateHeader(column.key, e.target.value)}
        placeholder={defaultHeader}
        disabled={!column.selected}
      />
    </div>
  );
}

type ColumnSettingsPanelProps = {
  columns: ColumnState[];
  isAllSelected: boolean;
  onToggle: (key: ExportColumnKey, selected: boolean) => void;
  onUpdateHeader: (key: ExportColumnKey, header: string) => void;
  onToggleAll: (selected: boolean) => void;
  onReorder: (columns: ColumnState[]) => void;
};

export function ColumnSettingsPanel({
  columns,
  isAllSelected,
  onToggle,
  onUpdateHeader,
  onToggleAll,
  onReorder,
}: ColumnSettingsPanelProps) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = columns.findIndex(column => column.key === active.id);
    const newIndex = columns.findIndex(column => column.key === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    onReorder(arrayMove(columns, oldIndex, newIndex));
  };

  return (
    <>
      <div className="flex items-center justify-between rounded-md border px-3 py-2">
        <label className="flex items-center gap-2 text-sm font-medium">
          <input
            type="checkbox"
            checked={isAllSelected}
            onChange={e => onToggleAll(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          全选
        </label>
        <span className="text-xs text-muted-foreground">
          已选 {columns.filter(column => column.selected).length} / {columns.length} 列
        </span>
      </div>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={columns.map(column => column.key)} strategy={verticalListSortingStrategy}>
          <div className="max-h-[50vh] overflow-auto rounded-md border">
            {columns.map(column => (
              <SortableColumnRow
                key={column.key}
                column={column}
                onToggle={onToggle}
                onUpdateHeader={onUpdateHeader}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
    </>
  );
}

