import type { ParseMode } from '../types';
export function ParseModeSelect({ value, onChange }: { value: ParseMode; onChange: (value: ParseMode) => void }) {
  return <label className="flex items-center gap-2 text-sm text-muted-foreground">解析模式（料号 / 扫码）
    <select aria-label="解析模式" className="border rounded-md p-2 bg-background" value={value} onChange={e => onChange(e.target.value as ParseMode)}>
      <option value="standard">普通解析</option><option value="llm">AI 整理</option><option value="agent">Agent 解析</option>
    </select>
  </label>;
}
