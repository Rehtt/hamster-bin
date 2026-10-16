import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { Camera, Check, ImagePlus, Loader2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { useTasks, taskPending } from '../context/tasks';
import type { ParseTask, ParsedImageItem, ReviewedImageItem, TaskRequest } from '../types';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Modal } from './ui/Modal';
const CameraCapture = lazy(() => import('./CameraCapture'));

interface Props {
  isOpen: boolean;
  onClose: () => void;
  source: TaskRequest['source'];
  recordID?: number;
  initialTask?: ParseTask;
  onConfirm: (items: ReviewedImageItem[]) => void;
}
const normalize = (item: ParsedImageItem): ParsedImageItem => ({ ...item, quantity: item.quantity ?? null });
const fieldLabels = {
  name: '名称', category_name: '分类', model: '厂家型号', manufacturer: '制造商', value: '参数',
  package: '封装', supplier_name: '供应商', supplier_part_number: '供应商料号', quantity: '数量',
} as const;
const isReviewed = (item: ParsedImageItem): item is ReviewedImageItem =>
  !!item.name.trim() && !!item.category_name.trim() && item.quantity !== null && Number.isInteger(item.quantity) && item.quantity >= 0;

// 关闭及切换目标时卸载会话，释放摄像头并清空结果。共享上下文继续跟踪任务。
export function PhotoParseModal(props: Props) {
  return props.isOpen ? <PhotoParseSession key={`${props.source}:${props.recordID ?? 'new'}:${props.initialTask?.id ?? 'capture'}`} {...props} /> : null;
}

function PhotoParseSession({ onClose, source, recordID, initialTask, onConfirm }: Props) {
  const { submitImage, tasks } = useTasks();
  const input = useRef<HTMLInputElement>(null);
  const session = useRef(0);
  const sendingRef = useRef(false);
  const consumed = useRef(initialTask?.id);
  const [camera, setCamera] = useState(false);
  const [task, setTask] = useState<ParseTask | undefined>(initialTask);
  const [items, setItems] = useState<ParsedImageItem[]>(() => (initialTask?.result?.items || []).map(normalize));
  const [selected, setSelected] = useState<number[]>(() => recordID ? (items.length ? [0] : []) : items.map((_, i) => i));
  const [sending, setSending] = useState(false);
  const current = task ? tasks[task.id] ?? task : undefined;
  const busy = sending || taskPending(current);
  useEffect(() => () => { session.current++; }, []);
  useEffect(() => {
    if (!current || taskPending(current) || consumed.current === current.id) return;
    consumed.current = current.id;
    if (current.status === 'failed') toast.error(current.error || '照片解析失败');
    if (current.status === 'succeeded') {
      const next = (current.result?.items || []).map(normalize);
      setItems(next);
      setSelected(recordID ? (next.length ? [0] : []) : next.map((_, i) => i));
    }
  }, [current, recordID]);
  const choose = async (file: File) => {
    if (sendingRef.current) return;
    const activeSession = session.current;
    sendingRef.current = true;
    setSending(true); setTask(undefined); setItems([]); setSelected([]);
    try {
      const next = await submitImage(file, source, recordID);
      if (session.current === activeSession) setTask(next);
    } catch (error) {
      if (session.current === activeSession) toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '提交照片失败');
    } finally {
      if (session.current === activeSession) { sendingRef.current = false; setSending(false); }
    }
  };
  const update = (index: number, key: keyof ParsedImageItem, value: string) => setItems(prev => prev.map((item, i) => i === index ? { ...item, [key]: key === 'quantity' ? (value === '' ? null : Number(value)) : value } : item));
  const reviewed = useMemo(() => selected.map(i => items[i]).filter(isReviewed), [items, selected]);
  const valid = selected.length > 0 && reviewed.length === selected.length;
  const close = () => { session.current++; onClose(); };
  const confirm = () => {
    if (!valid) { toast.error('请为选中结果填写名称、分类和非负整数数量'); return; }
    onConfirm(reviewed); close();
  };
  return <Modal isOpen onClose={close} title="拍照 AI 解析" className="max-w-5xl" footer={<>
    <Button variant="outline" onClick={close}>关闭</Button>
    {items.length > 0 && <Button onClick={confirm} disabled={!valid}><Check className="mr-2 h-4 w-4" />应用选中结果</Button>}
  </>}>
    <div className="space-y-4">
      {!task && !camera && !sending && <div className="flex flex-wrap gap-2">
        <Button onClick={() => setCamera(true)}><Camera className="mr-2 h-4 w-4" />拍照</Button>
        <Button variant="outline" onClick={() => input.current?.click()}><ImagePlus className="mr-2 h-4 w-4" />选择照片</Button>
        <input ref={input} type="file" accept="image/jpeg,image/png" className="hidden" onChange={e => { const file = e.target.files?.[0]; if (file) void choose(file); e.target.value = ''; }} />
      </div>}
      {camera && <Suspense fallback={<p>加载摄像头…</p>}><CameraCapture onCapture={file => { setCamera(false); void choose(file); }} onClose={() => setCamera(false)} /></Suspense>}
      {busy && <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />照片解析中，关闭后任务仍会继续，可到任务中心查看。</div>}
      {current && !taskPending(current) && !items.length && <Button variant="outline" onClick={() => setTask(undefined)}>重新选择照片</Button>}
      {items.length > 0 && <div className="space-y-3 max-h-[55vh] overflow-auto">{items.map((item, index) => <div key={index} className="rounded-md border p-3 space-y-2">
        <label className="flex items-center gap-2 font-medium"><input type={recordID ? 'radio' : 'checkbox'} name="photo-result" checked={selected.includes(index)} onChange={e => setSelected(prev => e.target.checked ? (recordID ? [index] : [...prev, index].sort((a, b) => a - b)) : prev.filter(i => i !== index))} />识别结果 {index + 1}</label>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-2">{(Object.keys(fieldLabels) as (keyof typeof fieldLabels)[]).map(key => <label key={key} className="text-sm space-y-1">{fieldLabels[key]}<Input aria-label={`结果 ${index + 1} ${fieldLabels[key]}`} type={key === 'quantity' ? 'number' : 'text'} min={key === 'quantity' ? 0 : undefined} step={key === 'quantity' ? 1 : undefined} placeholder={key === 'quantity' ? '未知，请填写数量' : fieldLabels[key]} value={item[key] ?? ''} onChange={e => update(index, key, e.target.value)} /></label>)}</div>
        <Input aria-label={`结果 ${index + 1} 描述`} placeholder="描述" value={item.description} onChange={e => update(index, 'description', e.target.value)} />
      </div>)}</div>}
    </div>
  </Modal>;
}
