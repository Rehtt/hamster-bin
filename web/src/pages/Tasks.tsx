import { ParseTraceView } from '../components/ParseTraceView';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { Pagination, ParseTask } from '../types';
import { taskPending, taskStatusLabel, useTasks } from '../context/tasks';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { PageHeader } from '../components/ui/PageHeader';
import { SupplierReference } from '../components/SupplierReference';

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
  const { track } = useTasks();
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    try {
      const res = await client.get('/tasks', { params: { page, page_size: 20, status, type } });
      setItems(res.data.data); setPagination(res.data.pagination); setError('');
    } catch { setError('任务列表加载失败'); }
  }, [page, status, type]);
  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 2000);
    return () => window.clearInterval(timer);
  }, [load]);
  const detailID = detail?.id;
  useEffect(() => {
    if (!detailID) return;
    let disposed = false;
    const refresh = async () => {
      try { const res = await client.get(`/tasks/${detailID}`); if (!disposed) setDetail(res.data.data); } catch { /* 保留最后一次详情 */ }
    };
    const timer = window.setInterval(() => void refresh(), 2000);
    return () => { disposed = true; window.clearInterval(timer); };
  }, [detailID]);
  const selectedID = params.get('task');
  useEffect(() => {
    if (!selectedID) return;
    let disposed = false;
    void client.get(`/tasks/${selectedID}`).then(res => { if (!disposed) setDetail(res.data.data); }).catch(() => { if (!disposed) toast.error('任务不存在或加载失败'); });
    return () => { disposed = true; };
  }, [selectedID]);
  const action = async (task: ParseTask, kind: 'cancel' | 'retry') => {
    setWorking(task.id);
    try {
      const res = await client.post(`/tasks/${task.id}/${kind}`);
      const updated = res.data.data as ParseTask;
      track(updated);
      if (kind === 'retry') toast.success(`已创建重试任务 #${updated.id}`);
      else toast.success('任务已取消');
      if (detail) setDetail(updated);
      void load();
    } catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '任务操作失败'); }
    finally { setWorking(null); }
  };
  const actions = (task: ParseTask) => <div className="flex flex-wrap gap-2">
    {taskPending(task) && <Button variant="outline" disabled={working === task.id} onClick={() => void action(task, 'cancel')}>取消任务</Button>}
    {(task.status === 'failed' || task.status === 'cancelled') && <Button variant="outline" disabled={working === task.id} onClick={() => void action(task, 'retry')}>重试</Button>}
    {task.status === 'succeeded' && task.result && <Button onClick={() => navigate(`/${task.source || 'components'}?applyTask=${task.id}`)}>应用结果</Button>}
  </div>;
  return <div className="space-y-6">
    <PageHeader title="任务中心" actions={<Button variant="outline" onClick={() => void load()}>刷新</Button>} />
    <p className="text-sm text-muted-foreground">解析结果仅回填表单，确认保存后才写入元件或预入库。</p>
    <div className="flex flex-wrap gap-3">
      <select aria-label="任务状态" className="border rounded-md p-2 bg-background" value={status} onChange={e => { setStatus(e.target.value); setPage(1); }}>
        <option value="">全部状态</option>{Object.entries(taskStatusLabel).map(([key, label]) => <option key={key} value={key}>{label}</option>)}
      </select>
      <select aria-label="任务类型" className="border rounded-md p-2 bg-background" value={type} onChange={e => { setType(e.target.value); setPage(1); }}><option value="">全部类型</option><option value="component_parse">平台编码</option><option value="qrcode_parse">二维码</option><option value="image_parse">照片 AI</option></select>
    </div>
    {error && <p role="alert" className="text-destructive">{error}</p>}
    <div className="border rounded-lg overflow-x-auto"><table className="w-full text-sm text-left"><thead className="bg-muted"><tr>{['任务', '类型 / 来源', '状态', '创建时间', '操作'].map(label => <th key={label} className="p-3">{label}</th>)}</tr></thead><tbody>
      {items.map(task => <tr key={task.id} className="border-t"><td className="p-3"><button className="underline" onClick={() => setDetail(task)}>#{task.id} {task.input.code || task.result?.component?.platform_code || (task.type === 'image_parse' ? '照片' : '二维码')}</button></td><td className="p-3">{typeLabel(task)}<br />{task.source === 'pre-stocks' ? '预入库' : '元件管理'}{task.record_id ? ` #${task.record_id}` : ' · 新增'}</td><td className="p-3">{taskStatusLabel[task.status]}</td><td className="p-3 whitespace-nowrap">{new Date(task.created_at).toLocaleString()}</td><td className="p-3">{actions(task)}</td></tr>)}
      {!items.length && <tr><td colSpan={5} className="p-6 text-center text-muted-foreground">暂无任务</td></tr>}
    </tbody></table></div>
    <div className="flex flex-wrap items-center gap-3"><span>共 {pagination.total} 个任务</span><Button variant="outline" disabled={page <= 1} onClick={() => setPage(page - 1)}>上一页</Button><span>{page} / {Math.max(1, pagination.total_page || 0)}</span><Button variant="outline" disabled={page >= (pagination.total_page || 0)} onClick={() => setPage(page + 1)}>下一页</Button></div>
    <Modal isOpen={!!detail} onClose={() => { setDetail(null); setParams({}, { replace: true }); }} title={`任务 #${detail?.id || ''}`} className="max-w-2xl" footer={detail && actions(detail)}>
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
