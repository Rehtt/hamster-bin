import { toast } from 'react-hot-toast';
import client from '../../api/client';
import type { ApiResponse, StockLog } from '../../types';
import { useConfirm } from '../../context/confirm';
import { useResource } from '../../hooks/useResource';
import { useSubmitLock } from '../../hooks/useFormGuard';
import { Modal } from '../ui/Modal';
import { PageState } from '../ui/PageState';
import { StockLogList } from './StockLogList';
export function ComponentLogsDialog({ id, onClose, onUpdated }: { id: number; onClose: () => void; onUpdated: () => void }) {
  const resource = useResource<ApiResponse<StockLog[]>>(`/components/${id}/logs`);
  const confirm = useConfirm();
  const mutation = useSubmitLock();
  const revoke = async (log: StockLog) => {
    if (!await confirm('确定撤销此记录？库存将回滚。')) return;
    await mutation.run(async () => {
      try {
        await client.post(`/stock-logs/${log.id}/revoke`);
        toast.success('撤销成功'); resource.refresh(); onUpdated();
      } catch (error) {
        toast.error((error as { response?: { data?: { error?: string } } }).response?.data?.error || '撤销失败');
      }
    });
  };
  return <Modal isOpen onClose={onClose} title="库存记录" busy={mutation.busy}>
    <PageState loading={resource.loading} error={resource.error} empty={!resource.data?.data.length} onRetry={resource.refresh} />
    {!resource.loading && !resource.error && <StockLogList logs={resource.data?.data || []} onRevoke={log => void revoke(log)} busy={mutation.busy} showComponent={false} />}
  </Modal>;
}
