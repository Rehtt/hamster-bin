interface Props { index: number; count: number; disabled: boolean; onSelect: (index: number) => void; }

export function PhotoDraftNavigation({ index, count, disabled, onSelect }: Props) {
  if (!count) return null;
  return <div className="rounded-md border p-3 space-y-2">
    <p className="text-sm">当前第 {index + 1} 条／共 {count} 条，逐条核对后一次保存全部</p>
    <select aria-label="切换照片条目" className="w-full rounded-md border bg-background p-2" value={index} disabled={disabled} onChange={e => onSelect(Number(e.target.value))}>
      {Array.from({ length: count }, (_, i) => <option key={i} value={i}>第 {i + 1} 条</option>)}
    </select>
  </div>;
}
