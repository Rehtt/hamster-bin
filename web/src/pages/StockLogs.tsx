import { useEffect, useState } from 'react';
import { History } from 'lucide-react';
import { toast } from 'react-hot-toast';
import client from '../api/client';
import { type StockLog, type Pagination } from '../types';
import { Card, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { formatCents, formatMicro } from '../utils/price';
import {
  canRevoke,
  isReversal,
  isRevoked,
  stockLogAmountClass,
  stockLogCardClass,
  stockLogIconClass,
  formatStockLogChangeAmount,
} from '../utils/stockLog';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

export default function StockLogs() {
  const [logs, setLogs] = useState<StockLog[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0, total_page: 0 });
  const [revokingId, setRevokingId] = useState<number | null>(null);

  const fetchLogs = async (page = 1, pageSize = pagination.page_size) => {
    try {
      const res = await client.get('/stock-logs', { params: { page, page_size: pageSize } });
      setLogs(res.data.data || []);
      setPagination(res.data.pagination || { page: 1, page_size: 20, total: 0, total_page: 0 });
    } catch (error) {
      console.error(error);
    }
  };

  useEffect(() => {
    void fetchLogs(1, pagination.page_size);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handlePageSizeChange = (pageSize: number) => {
    setPagination(prev => ({ ...prev, page: 1, page_size: pageSize }));
    void fetchLogs(1, pageSize);
  };

  const handlePageChange = (page: number) => {
    setPagination(prev => ({ ...prev, page }));
    void fetchLogs(page, pagination.page_size);
  };

  const handleRevoke = async (log: StockLog) => {
    if (!confirm('确定撤销此记录？库存将回滚。')) return;
    setRevokingId(log.id);
    try {
      await client.post(`/stock-logs/${log.id}/revoke`);
      toast.success('撤销成功');
      await fetchLogs(pagination.page, pagination.page_size);
    } catch (err: unknown) {
      const message = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || '撤销失败';
      toast.error(message);
    } finally {
      setRevokingId(null);
    }
  };

  const totalPage = pagination.total_page ?? (pagination.total > 0 ? Math.ceil(pagination.total / pagination.page_size) : 0);

  return (
    <div className="space-y-6">
      <h2 className="text-3xl font-bold tracking-tight">全局库存记录</h2>
      
      <div className="space-y-4">
        {logs.map(log => (
            <Card key={log.id} className={stockLogCardClass(log)}>
                <CardContent className="flex justify-between items-center p-6 gap-4">
                    <div className="flex items-center gap-4 min-w-0">
                        <div className={`p-2 rounded-full shrink-0 ${stockLogIconClass(log)}`}>
                            <History className="w-5 h-5" />
                        </div>
                        <div className="min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="font-semibold">{log.component?.name || '未知元件'}</span>
                              {isRevoked(log) && (
                                <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">已撤销</span>
                              )}
                              {isReversal(log) && (
                                <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">撤销冲销</span>
                              )}
                            </div>
                            <div className="text-sm text-muted-foreground">
                                {new Date(log.created_at).toLocaleString()}
                            </div>
                        </div>
                    </div>
                    <div className="text-right shrink-0">
                        <div className={`font-bold text-lg ${stockLogAmountClass(log)}`}>
                            {formatStockLogChangeAmount(log)}
                        </div>
                        {(log.total_price_cents ?? 0) > 0 && (
                          <div className="text-sm text-foreground">
                            {log.change_amount < 0 ? '成本' : '总价'} {formatCents(log.total_price_cents)}
                            {(log.unit_price_micro ?? 0) > 0 && (
                              <span className="text-muted-foreground"> · 单价 {formatMicro(log.unit_price_micro)}</span>
                            )}
                          </div>
                        )}
                        <div className="text-sm text-muted-foreground">{log.reason || '无备注'}</div>
                        {canRevoke(log) && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="mt-2"
                            disabled={revokingId === log.id}
                            onClick={() => void handleRevoke(log)}
                          >
                            撤销
                          </Button>
                        )}
                    </div>
                </CardContent>
            </Card>
        ))}
      </div>

      <div className="flex flex-col sm:flex-row justify-between gap-3 items-start sm:items-center mt-4">
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <span>共 {pagination.total} 条</span>
          <div className="flex items-center gap-2">
            <span>每页</span>
            <select
              className="h-9 rounded-md border border-input bg-background px-2 text-sm"
              value={pagination.page_size}
              onChange={e => handlePageSizeChange(Number(e.target.value))}
            >
              {PAGE_SIZE_OPTIONS.map(size => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
            <span>条</span>
          </div>
        </div>
        <div className="flex gap-2 items-center">
          <Button 
            variant="outline" 
            disabled={pagination.page <= 1} 
            onClick={() => handlePageChange(pagination.page - 1)}
          >上一页</Button>
          <span>第 {pagination.page} / {Math.max(totalPage, 1)} 页</span>
          <Button 
            variant="outline" 
            disabled={totalPage === 0 || pagination.page >= totalPage} 
            onClick={() => handlePageChange(pagination.page + 1)}
          >下一页</Button>
        </div>
      </div>
    </div>
  );
}
