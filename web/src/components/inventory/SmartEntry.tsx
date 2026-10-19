import { Camera, Loader2, QrCode, Sparkles } from 'lucide-react';
import type { ParsedComponentInfo, ParseMode, ParseTask } from '../../types';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { ParseModeSelect } from '../ParseModeSelect';
import { ParseTaskStatus } from '../ParseTaskStatus';
import { SupplierReference } from '../SupplierReference';
export function SmartEntry({ code, onCodeChange, onParse, onScan, onPhoto, busy, mode, onModeChange, task, reference }: {
  code: string; onCodeChange: (value: string) => void; onParse: () => void; onScan: () => void; onPhoto: () => void;
  busy: boolean; mode: ParseMode; onModeChange: (mode: ParseMode) => void; task?: ParseTask; reference?: ParsedComponentInfo;
}) {
  return <section aria-label="智能录入" className="space-y-3 rounded-lg border border-primary/15 bg-primary/5 p-4"><div className="flex items-center gap-2 font-medium"><Sparkles className="size-4 text-primary" />智能录入<span className="ml-auto text-xs font-normal text-muted-foreground">识别后仍可手动修改</span></div><div className="flex flex-wrap gap-2"><Input aria-label="供应商料号解析" placeholder="输入供应商料号，例如 C2040" className="min-w-40 flex-1" value={code} onChange={e => onCodeChange(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); onParse(); } }} /><Button onClick={onParse} disabled={busy || !code.trim()}>{busy ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}解析</Button><Button variant="outline" onClick={onScan} disabled={busy}><QrCode className="mr-2 size-4" />扫码</Button><Button variant="outline" onClick={onPhoto} disabled={busy}><Camera className="mr-2 size-4" />拍照 AI</Button></div><ParseModeSelect value={mode} onChange={onModeChange} /><ParseTaskStatus task={task} /><SupplierReference component={reference} /></section>;
}
