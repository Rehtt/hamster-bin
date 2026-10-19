import { useEffect } from 'react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import type { ApiResponse, StockLog, Pagination as PaginationData } from '../types';
import { useConfirm } from '../context/confirm';
import { useListQuery } from '../hooks/useListQuery';
import { useResource } from '../hooks/useResource';
import { useSubmitLock } from '../hooks/useFormGuard';
import { PageHeader } from '../components/ui/PageHeader';
import { PageState } from '../components/ui/PageState';
import { Pagination } from '../components/ui/Pagination';
import { Button } from '../components/ui/Button';
import { StockLogList } from '../components/inventory/StockLogList';
export default function StockLogs() {
  const { page, pageSize, updateQuery } = useListQuery();
  const resource = useResource<ApiResponse<StockLog[]>>(`/stock-logs?page=${page}&page_size=${pageSize}`);
  const pagination: PaginationData = resource.data?.pagination || { page, page_size: pageSize, total: 0 };
  const confirm = useConfirm();
  const mutation = useSubmitLock();
  useEffect(() => { const last = Math.max(1, Math.ceil(pagination.total / pageSize)); if (resource.data && page > last) updateQuery({ page: last }, true); }, [pagination.total, resource.data, page, pageSize, updateQuery]);
  const revoke = async (log: StockLog) => {
    if (!await confirm({ title: '撤销这条库存记录？', message: '库存与相关成本将按原记录回滚，撤销后会保留冲销流水。', confirmLabel: '确认撤销' })) return;
    await mutation.run(async () => {
      try { await client.post(`/stock-logs/${log.id}/revoke`); toast.success('撤销成功'); resource.refresh(); }
      catch (error) { toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '撤销失败'); }
    });
  };
  return <div className="space-y-6"><PageHeader title="库存记录" actions={<Button variant="outline" onClick={resource.refresh} disabled={resource.loading}>刷新</Button>} /><section className="surface"><PageState loading={resource.loading} error={resource.error} empty={!resource.data?.data.length} onRetry={resource.refresh} />{!resource.loading && !resource.error && <StockLogList logs={resource.data?.data || []} onRevoke={log => void revoke(log)} busy={mutation.busy} />}</section><Pagination pagination={pagination} onPageChange={page => updateQuery({ page })} onPageSizeChange={page_size => updateQuery({ page_size, page: 1 })} disabled={resource.loading} /></div>;
}
