import { Trash2 } from 'lucide-react';
import { useConfirm } from '../context/confirm';
import { useListQuery } from '../hooks/useListQuery';
import { RowActionsMenu } from '../components/ui/RowActionsMenu';
import { PageState } from '../components/ui/PageState';
import { Pagination as PaginationControl } from '../components/ui/Pagination';
import { TaskResultView } from '../components/inventory/TaskResultView';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { Pagination, ParseTask } from '../types';
import { taskPending, taskStatusLabel, useTasks } from '../context/tasks';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { PageHeader } from '../components/ui/PageHeader';
import { TaskResponseGuard, lastTaskPage, pruneTaskSelection, taskDeleteConfirmation } from '../utils/taskDeletion';
import type { TaskDeleteMode, TaskDeleteResult } from '../utils/taskDeletion';

const typeLabel = (task: ParseTask) => `${task.type === 'qrcode_parse' ? '二维码' : task.type === 'image_parse' ? '照片' : '平台编码'}${task.type === 'image_parse' ? ' · AI' : task.input.parse_mode === 'agent' ? ' · Agent 解析' : (task.input.parse_mode ? task.input.parse_mode === 'llm' : task.input.use_llm) ? ' · AI 整理' : ' · 普通解析'}`;
export default function Tasks() {
  const confirm = useConfirm();
  const { params: query, updateQuery, page, pageSize } = useListQuery();
  const status = query.get('status') || '';
  const type = query.get('type') || '';
  const setPage = (value: number) => updateQuery({ page: value });
  const setStatus = (value: string) => updateQuery({ status: value, page: 1 });
  const setType = (value: string) => updateQuery({ type: value, page: 1 });
  const [loading, setLoading] = useState(true);
  const [menu, setMenu] = useState<string | null>(null);

  const [items, setItems] = useState<ParseTask[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
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
      const res = await client.get('/tasks', { params: { page, page_size: pageSize, status, type } });
      if (!guard.current.accepts(version) || request !== listRequest.current) return;
      const pagination = res.data.pagination as Pagination;
      const lastPage = lastTaskPage(pagination.total, pagination.page_size);
      if (page > lastPage) { updateQuery({ page: lastPage }, true); setSelected([]); return; }
      const items = (res.data.data as ParseTask[]).filter(task => !guard.current.has(task.id));
      setItems(items); setPagination(pagination); setError('');
      setSelected(prev => pruneTaskSelection(prev, items));
    } catch { if (guard.current.accepts(version) && request === listRequest.current) setError('任务列表加载失败'); }
    finally { if (request === listRequest.current) setLoading(false); }
  }, [page, pageSize, status, type, updateQuery]);
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
  useEffect(() => { setSelected([]); }, [page, pageSize, status, type]);
  const selectedID = params.get('task');
  useEffect(() => {
    if (!selectedID) { detailRequest.current++; setDetail(null); return; }
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
  const changeScope = () => { guard.current.invalidate(); setSelected([]); setItems([]); setLoading(true); };
  const remove = async (mode: TaskDeleteMode, ids: number[] = [], single = false) => {
    if (mutation.current || !await confirm(taskDeleteConfirmation(mode, ids, single))) return;
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
    <PageHeader title="任务中心" actions={<><Button variant="outline" disabled={busy} onClick={() => { setLoading(true); void load(); }}>刷新</Button><RowActionsMenu rowId="cleanup" activeRowId={menu} onActiveRowChange={setMenu} actions={[{ key: 'failed', icon: Trash2, label: '删除失败任务', onClick: () => void remove('failed'), destructive: true, disabled: busy }, { key: 'old', icon: Trash2, label: '删除7天以上任务', onClick: () => void remove('older_than_7_days'), destructive: true, disabled: busy }, { key: 'all', icon: Trash2, label: '删除全部任务', onClick: () => void remove('all'), destructive: true, disabled: busy }]} /></>} />
    <p className="text-sm text-muted-foreground">解析结果仅回填表单，确认保存后才写入元件或预入库。</p>
    <div className="surface flex flex-wrap gap-3 p-4">
      <select disabled={busy} aria-label="任务状态" className="border rounded-md p-2 bg-background" value={status} onChange={e => { changeScope(); setStatus(e.target.value); }}>
        <option value="">全部状态</option>{Object.entries(taskStatusLabel).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
      </select>
      <select disabled={busy} aria-label="任务类型" className="border rounded-md p-2 bg-background" value={type} onChange={e => { changeScope(); setType(e.target.value); }}><option value="">全部类型</option><option value="component_parse">平台编码</option><option value="qrcode_parse">二维码</option><option value="image_parse">照片 AI</option></select>
    </div>
    {selected.length > 0 && <div className="flex items-center gap-3"><span>已选 {selected.length} 项</span><Button variant="destructive" disabled={busy} onClick={() => void remove('selected', selected)}>删除选中</Button><Button variant="outline" disabled={busy} onClick={() => setSelected([])}>取消选择</Button></div>}
    {(loading || error || !items.length) && <div className="surface"><PageState loading={loading} error={error} empty={!items.length} filtered={!!status || !!type} onRetry={() => void load()} onClear={() => updateQuery({ status: '', type: '', page: 1 })} /></div>}
    <div className={`surface hidden overflow-x-auto ${loading || error || !items.length ? '' : 'md:block'}`}><table className="w-full text-sm text-left"><thead className="bg-muted"><tr><th className="p-3"><input type="checkbox" aria-label="本页全选" disabled={busy || !selectable.length} checked={!!selectable.length && selectable.every(task => selected.includes(task.id))} ref={node => { if (node) node.indeterminate = selected.length > 0 && selected.length < selectable.length; }} onChange={e => setSelected(e.target.checked ? selectable.map(task => task.id) : [])} /></th>{['任务', '类型 / 来源', '状态', '创建时间', '操作'].map(label => <th key={label} className="p-3">{label}</th>)}</tr></thead><tbody>
      {items.map(task => <tr key={task.id} className="border-t"><td className="p-3"><input type="checkbox" aria-label={`选择任务 #${task.id}`} disabled={busy || taskPending(task)} checked={selected.includes(task.id)} onChange={e => setSelected(prev => e.target.checked ? [...prev, task.id] : prev.filter(id => id !== task.id))} /></td><td className="p-3"><button className="underline" disabled={busy} onClick={() => { detailRequest.current++; setDetail(task); updateQuery({ task: task.id }); }}>#{task.id} {task.input.code || task.result?.component?.platform_code || (task.type === 'image_parse' ? '照片' : '二维码')}</button></td><td className="p-3">{typeLabel(task)}<br />{task.source === 'pre-stocks' ? '预入库' : '元件管理'}{task.record_id ? ` #${task.record_id}` : ' · 新增'}</td><td className="p-3"><span className="status-badge">{taskStatusLabel[task.status]}</span></td><td className="p-3 whitespace-nowrap">{new Date(task.created_at).toLocaleString()}</td><td className="p-3">{actions(task)}</td></tr>)}
      {!items.length && <tr><td colSpan={6} className="p-6 text-center text-muted-foreground">暂无任务</td></tr>}
    </tbody></table></div>
    {!loading && !error && <div className="space-y-3 md:hidden">{items.map(task => <article key={task.id} className="surface space-y-4 p-4"><div className="flex items-start gap-3"><input type="checkbox" aria-label={`选择任务 #${task.id}`} checked={selected.includes(task.id)} disabled={busy || taskPending(task)} onChange={e => setSelected(prev => e.target.checked ? [...prev, task.id] : prev.filter(id => id !== task.id))} className="mt-1 size-4" /><div className="min-w-0 flex-1"><button className="break-all text-left font-medium text-primary" onClick={() => { detailRequest.current++; setDetail(task); updateQuery({ task: task.id }); }}>#{task.id} {task.input.code || task.result?.component?.name || (task.type === 'image_parse' ? '照片解析' : '二维码解析')}</button><p className="mt-2 text-xs text-muted-foreground">{typeLabel(task)}</p><p className="mt-1 text-xs text-muted-foreground">{new Date(task.created_at).toLocaleString('zh-CN')}</p></div><span className="status-badge shrink-0">{taskStatusLabel[task.status]}</span></div>{actions(task)}</article>)}</div>}
    <PaginationControl pagination={{ ...pagination, page, page_size: pageSize }} onPageChange={next => { changeScope(); setPage(next); }} onPageSizeChange={page_size => { changeScope(); updateQuery({ page_size, page: 1 }); }} disabled={busy || loading} />
    <Modal isOpen={!!detail} onClose={closeDetail} title={`任务 #${detail?.id || ''}`} variant="drawer" footer={detail && actions(detail)}>
      {detail && <TaskResultView task={detail} />}
    </Modal>
  </div>;
}
