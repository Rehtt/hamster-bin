import { Button } from './Button';

const COMMON_QUANTITIES = [5, 10, 20, 50, 100];

type QuantityShortcutsProps = {
  value?: number;
  onSelect: (quantity: number) => void;
};

export function QuantityShortcuts({ value, onSelect }: QuantityShortcutsProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {COMMON_QUANTITIES.map(quantity => (
        <Button
          key={quantity}
          type="button"
          variant={value === quantity ? 'secondary' : 'outline'}
          size="sm"
          className="h-8 min-w-12 px-3"
          onClick={() => onSelect(quantity)}
        >
          {quantity}
        </Button>
      ))}
    </div>
  );
}
