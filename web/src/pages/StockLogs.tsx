import { useEffect, useState } from 'react';
import { History } from 'lucide-react';
import client from '../api/client';
import { type StockLog, type Pagination } from '../types';
import { Card, CardContent } from '../components/ui/Card';
import { Button } from '../components/ui/Button';

export default function StockLogs() {
  const [logs, setLogs] = useState<StockLog[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, page_size: 20, total: 0 });

  const fetchLogs = async (page = 1) => {
    try {
      const res = await client.get('/stock-logs', { params: { page, page_size: pagination.page_size } });
      setLogs(res.data.data || []);
      setPagination(res.data.pagination || { page: 1, page_size: 20, total: 0 });
    } catch (error) {
      console.error(error);
    }
  };

  useEffect(() => {
    fetchLogs();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="space-y-6">
      <h2 className="text-3xl font-bold tracking-tight">全局库存记录</h2>
      
      <div className="space-y-4">
        {logs.map(log => (
            <Card key={log.id}>
                <CardContent className="flex justify-between items-center p-6">
                    <div className="flex items-center gap-4">
                        <div className={`p-2 rounded-full ${log.change_amount > 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                            <History className="w-5 h-5" />
                        </div>
                        <div>
                            <div className="font-semibold">{log.component?.name || '未知元件'}</div>
                            <div className="text-sm text-muted-foreground">
                                {new Date(log.created_at).toLocaleString()}
                            </div>
                        </div>
                    </div>
                    <div className="text-right">
                        <div className={`font-bold text-lg ${log.change_amount > 0 ? 'text-green-600' : 'text-red-600'}`}>
                            {log.change_amount > 0 ? '+' : ''}{log.change_amount}
                        </div>
                        <div className="text-sm text-muted-foreground">{log.reason || '无备注'}</div>
                    </div>
                </CardContent>
            </Card>
        ))}
      </div>

      <div className="flex justify-end gap-2 items-center mt-4">
          <Button 
            variant="outline" 
            disabled={pagination.page <= 1} 
            onClick={() => { setPagination(p => ({...p, page: p.page - 1})); fetchLogs(pagination.page - 1); }}
          >上一页</Button>
          <span>第 {pagination.page} 页</span>
          <Button 
            variant="outline" 
            disabled={logs.length < pagination.page_size} 
            onClick={() => { setPagination(p => ({...p, page: p.page + 1})); fetchLogs(pagination.page + 1); }}
          >下一页</Button>
      </div>
    </div>
  );
}
