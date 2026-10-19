import { ParseTraceView } from '../components/ParseTraceView';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { Pagination, ParseTask } from '../types';
import { taskPending, taskStatusLabel, useTasks } from '../context/tasks';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { PageHeader } from '../components/ui/PageHeader';
import { SupplierReference } from '../components/SupplierReference';
import { TaskResponseGuard, lastTaskPage, pruneTaskSelection, taskDeleteConfirmation } from '../utils/taskDeletion';
import type { TaskDeleteMode, TaskDeleteResult } from '../utils/taskDeletion';

const typeLabel = (task: ParseTask) => `${task.type === 'qrcode_parse' ? '二维码' : task.type === 'image_parse' ? '照片' : '平台编码'}${task.type === 'image_parse' ? ' · AI' : task.input.parse_mode === 'agent' ? ' · Agent 解析' : (task.input.parse_mode ? task.input.parse_mode === 'llm' : task.input.use_llm) ? ' · AI 整理' : ' · 普通解析'}`;
export default function Tasks() {
  const [items, setItems] = useState<ParseTask[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [type, setType] = useState('');
  const [detail, setDetail] = useState<ParseTask | null>(null);
  const [working, setWorking] = useState<number | null>(null);
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const { track, forget } = useTasks();
  const [selected, setSelected] = useState<number[]>([]);
  const [deleting, setDeleting] = useState(false);
  const mutation = useRef(false);
  const guard = useRef(new TaskResponseGuard());
  const listRequest = useRef(0);
  const detailRequest = useRef(0);
  const [error, setError] = useState('');
  const closeDetail = useCallback(() => {
    detailRequest.current++;
    setDetail(null);
    setParams(prev => { const next = new URLSearchParams(prev); next.delete('task'); return next; }, { replace: true });
  }, [setParams]);
  const load = useCallback(async () => {
    if (mutation.current) return;
    const version = guard.current.snapshot();
    const request = ++listRequest.current;
    try {
      const res = await client.get('/tasks', { params: { page, page_size: 20, status, type } });
      if (!guard.current.accepts(version) || request !== listRequest.current) return;
      const pagination = res.data.pagination as Pagination;
      const lastPage = lastTaskPage(pagination.total, pagination.page_size);
      if (page > lastPage) { setPage(lastPage); setSelected([]); return; }
      const items = (res.data.data as ParseTask[]).filter(task => !guard.current.has(task.id));
      setItems(items); setPagination(pagination); setError('');
      setSelected(prev => pruneTaskSelection(prev, items));
    } catch { if (guard.current.accepts(version) && request === listRequest.current) setError('任务列表加载失败'); }
  }, [page, status, type]);
  const invalidateList = useCallback(() => { listRequest.current++; }, []);
  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 2000);
    return () => { invalidateList(); window.clearInterval(timer); };
  }, [load, invalidateList]);
  const detailID = detail?.id;
  const loadRef = useRef(load);
  useEffect(() => { loadRef.current = load; }, [load]);
  const missingDetail = useCallback((id: number) => {
    guard.current.forget([id]); forget([id]);
    toast.error('任务已删除'); closeDetail(); void loadRef.current();
  }, [forget, closeDetail]);
  useEffect(() => {
    if (!detailID) return;
    let disposed = false;
    const refresh = async () => {
      if (mutation.current) return;
      const version = guard.current.snapshot();
      const request = detailRequest.current;
      try {
        const res = await client.get(`/tasks/${detailID}`);
        if (!disposed && guard.current.accepts(version) && request === detailRequest.current) setDetail(res.data.data);
      } catch (error) {
        if (!disposed && guard.current.accepts(version) && request === detailRequest.current && (error as { response?: { status?: number } }).response?.status === 404) missingDetail(detailID);
      }
    };
    const timer = window.setInterval(() => void refresh(), 2000);
    return () => { disposed = true; window.clearInterval(timer); };
  }, [detailID, missingDetail]);
  const selectedID = params.get('task');
  useEffect(() => {
    if (!selectedID) return;
    let disposed = false;
    const version = guard.current.snapshot();
    const request = ++detailRequest.current;
    void client.get(`/tasks/${selectedID}`).then(res => {
      if (!disposed && guard.current.accepts(version) && request === detailRequest.current) setDetail(res.data.data);
    }).catch(error => {
      if (disposed || !guard.current.accepts(version) || request !== detailRequest.current) return;
      if (error.response?.status === 404) missingDetail(Number(selectedID));
      else toast.error('任务加载失败');
    });
    return () => { disposed = true; };
  }, [selectedID, missingDetail]);
  const changeScope = () => { guard.current.invalidate(); setSelected([]); setItems([]); };
  const remove = async (mode: TaskDeleteMode, ids: number[] = [], single = false) => {
    if (mutation.current || !confirm(taskDeleteConfirmation(mode, ids, single))) return;
    mutation.current = true; setDeleting(true); guard.current.invalidate();
    try {
      const res = single ? await client.delete(`/tasks/${ids[0]}`) : await client.post('/tasks/batch-delete', { mode, ...(mode === 'selected' ? { ids } : {}) });
      const result = res.data.data as TaskDeleteResult;
      guard.current.forget(result.deleted_ids); forget(result.deleted_ids);
      setItems(prev => prev.filter(task => !result.deleted_ids.includes(task.id)));
      setSelected(prev => prev.filter(id => !result.deleted_ids.includes(id)));
      if ((detail && result.deleted_ids.includes(detail.id)) || (selectedID && result.deleted_ids.includes(Number(selectedID)))) closeDetail();
      toast.success(`已删除 ${result.deleted_count} 项，跳过 ${result.skipped_count} 项`);
      if (result.image_cleanup_failed_count) toast.error(`任务已删除，但有 ${result.image_cleanup_failed_count} 张原图未能清理`);
    } catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '删除失败，请重试'); }
    finally { mutation.current = false; setDeleting(false); void load(); }
  };
  const action = async (task: ParseTask, kind: 'cancel' | 'retry') => {
    if (mutation.current) return;
    mutation.current = true; setWorking(task.id); guard.current.invalidate();
    const detailVersion = detailRequest.current;
    try {
      const res = await client.post(`/tasks/${task.id}/${kind}`);
      const updated = res.data.data as ParseTask;
      track(updated);
      if (kind === 'retry') toast.success(`已创建重试任务 #${updated.id}`);
      else toast.success('任务已取消');
      if (detail?.id === task.id && detailVersion === detailRequest.current) setDetail(updated);
    } catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '任务操作失败'); }
    finally { mutation.current = false; setWorking(null); void load(); }
  };
  const actions = (task: ParseTask) => <div className="flex flex-wrap gap-2">
    {taskPending(task) && <Button variant="outline" disabled={deleting || working !== null} onClick={() => void action(task, 'cancel')}>取消任务</Button>}
    {(task.status === 'failed' || task.status === 'cancelled') && <Button variant="outline" disabled={deleting || working !== null} onClick={() => void action(task, 'retry')}>重试</Button>}
    {task.status === 'succeeded' && task.result && <Button disabled={deleting || working !== null} onClick={() => navigate(`/${task.source || 'components'}?applyTask=${task.id}`)}>应用结果</Button>}
    {!taskPending(task) && <Button variant="destructive" disabled={deleting || working !== null} onClick={() => void remove('selected', [task.id], true)}>删除</Button>}
  </div>;
  const selectable = items.filter(task => !taskPending(task));
  const busy = deleting || working !== null;
  return <div className="space-y-6">
    <PageHeader title="任务中心" actions={<Button variant="outline" disabled={busy} onClick={() => void load()}>刷新</Button>} />
    <p className="text-sm text-muted-foreground">解析结果仅回填表单，确认保存后才写入元件或预入库。</p>
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="destructive" disabled={busy} onClick={() => void remove('failed')}>删除失败任务</Button>
      <Button variant="destructive" disabled={busy} onClick={() => void remove('all')}>删除全部任务</Button>
      <Button variant="destructive" disabled={busy} onClick={() => void remove('older_than_7_days')}>删除7天以上任务</Button>
      <span className="text-sm text-muted-foreground">仅删除已结束任务；快捷操作覆盖全部分页与筛选</span>
    </div>
    <div className="flex flex-wrap gap-3">
      <select disabled={busy} aria-label="任务状态" className="border rounded-md p-2 bg-background" value={status} onChange={e => { changeScope(); setStatus(e.target.value); setPage(1); }}>
        <option value="">全部状态</option>{Object.entries(taskStatusLabel).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
      </select>
      <select disabled={busy} aria-label="任务类型" className="border rounded-md p-2 bg-background" value={type} onChange={e => { changeScope(); setType(e.target.value); setPage(1); }}><option value="">全部类型</option><option value="component_parse">平台编码</option><option value="qrcode_parse">二维码</option><option value="image_parse">照片 AI</option></select>
    </div>
    {selected.length > 0 && <div className="flex items-center gap-3"><span>已选 {selected.length} 项</span><Button variant="destructive" disabled={busy} onClick={() => void remove('selected', selected)}>删除选中</Button><Button variant="outline" disabled={busy} onClick={() => setSelected([])}>取消选择</Button></div>}
    {error && <p role="alert" className="text-destructive">{error}</p>}
    <div className="border rounded-lg overflow-x-auto"><table className="w-full text-sm text-left"><thead className="bg-muted"><tr><th className="p-3"><input type="checkbox" aria-label="本页全选" disabled={busy || !selectable.length} checked={!!selectable.length && selectable.every(task => selected.includes(task.id))} ref={node => { if (node) node.indeterminate = selected.length > 0 && selected.length < selectable.length; }} onChange={e => setSelected(e.target.checked ? selectable.map(task => task.id) : [])} /></th>{['任务', '类型 / 来源', '状态', '创建时间', '操作'].map(label => <th key={label} className="p-3">{label}</th>)}</tr></thead><tbody>
      {items.map(task => <tr key={task.id} className="border-t"><td className="p-3"><input type="checkbox" aria-label={`选择任务 #${task.id}`} disabled={busy || taskPending(task)} checked={selected.includes(task.id)} onChange={e => setSelected(prev => e.target.checked ? [...prev, task.id] : prev.filter(id => id !== task.id))} /></td><td className="p-3"><button className="underline" disabled={busy} onClick={() => { detailRequest.current++; setDetail(task); }}>#{task.id} {task.input.code || task.result?.component?.platform_code || (task.type === 'image_parse' ? '照片' : '二维码')}</button></td><td className="p-3">{typeLabel(task)}<br />{task.source === 'pre-stocks' ? '预入库' : '元件管理'}{task.record_id ? ` #${task.record_id}` : ' · 新增'}</td><td className="p-3">{taskStatusLabel[task.status]}</td><td className="p-3 whitespace-nowrap">{new Date(task.created_at).toLocaleString()}</td><td className="p-3">{actions(task)}</td></tr>)}
      {!items.length && <tr><td colSpan={6} className="p-6 text-center text-muted-foreground">暂无任务</td></tr>}
    </tbody></table></div>
    <div className="flex flex-wrap items-center gap-3"><span>共 {pagination.total} 个任务</span><Button variant="outline" disabled={busy || page <= 1} onClick={() => { changeScope(); setPage(page - 1); }}>上一页</Button><span>{page} / {Math.max(1, pagination.total_page || 0)}</span><Button variant="outline" disabled={busy || page >= (pagination.total_page || 0)} onClick={() => { changeScope(); setPage(page + 1); }}>下一页</Button></div>
    <Modal isOpen={!!detail} onClose={closeDetail} title={`任务 #${detail?.id || ''}`} className="max-w-2xl" footer={detail && actions(detail)}>
      {detail && <div className="space-y-4 max-h-[65vh] overflow-auto py-4">
        <p>{typeLabel(detail)} · {taskStatusLabel[detail.status]}</p>
        {detail.retry_of_id && <p>重试来源：#{detail.retry_of_id}</p>}
        <p className="text-sm text-muted-foreground">开始：{detail.started_at ? new Date(detail.started_at).toLocaleString() : '—'}<br />结束：{detail.finished_at ? new Date(detail.finished_at).toLocaleString() : '—'}</p>
        <p className="font-medium">输入</p><pre className="whitespace-pre-wrap break-all text-xs bg-muted p-3 rounded">{JSON.stringify(detail.input, null, 2)}</pre>
        {detail.type === 'image_parse' && <img src={`/api/v1/tasks/${detail.id}/image`} alt="照片解析原图" className="max-h-64 max-w-full rounded border object-contain" />}
        {detail.error && <p role="alert" className="text-destructive">{detail.error}（{detail.error_code}）</p>}
        <ParseTraceView trace={detail.result?.parse_trace} />
        {detail.result && <><SupplierReference component={detail.result.component} /><p className="font-medium">解析结果</p><pre className="whitespace-pre-wrap break-all text-xs bg-muted p-3 rounded">{JSON.stringify(detail.result, null, 2)}</pre></>}
      </div>}
    </Modal>
  </div>;
}
