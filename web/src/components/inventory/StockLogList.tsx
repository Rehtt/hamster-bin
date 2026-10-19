import { ArrowDownLeft, ArrowUpRight, Coins } from 'lucide-react';
import type { StockLog } from '../../types';
import { formatCents, formatMicro } from '../../utils/price';
import { canRevoke, isBackfillLog, isReversal, isRevoked, stockLogAmountClass, formatStockLogChangeAmount } from '../../utils/stockLog';
import { Button } from '../ui/Button';
export function StockLogList({ logs, onRevoke, busy, showComponent = true }: { logs: StockLog[]; onRevoke?: (log: StockLog) => void; busy?: boolean; showComponent?: boolean }) {
  return <div className="divide-y">{logs.map(log => {
    const Icon = isBackfillLog(log) ? Coins : log.change_amount < 0 ? ArrowUpRight : ArrowDownLeft;
    return <article key={log.id} className="flex flex-wrap items-start gap-3 p-4 sm:p-5"><div className={`rounded-lg bg-muted p-2.5 ${stockLogAmountClass(log)}`}><Icon className="size-4" /></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><p className="break-all font-medium">{showComponent ? log.component?.name || '未知元件' : isBackfillLog(log) ? '补录价格' : log.change_amount < 0 ? '出库' : '入库'}</p>{isRevoked(log) && <span className="status-badge bg-muted text-muted-foreground">已撤销</span>}{isReversal(log) && <span className="status-badge bg-muted text-muted-foreground">撤销冲销</span>}</div><p className="mt-1 text-xs text-muted-foreground">{new Date(log.created_at).toLocaleString('zh-CN')}</p><p className="mt-2 break-all text-sm text-muted-foreground">{log.reason || '无备注'}</p></div><div className="max-w-full text-right"><p className={`text-lg font-semibold tabular-nums ${stockLogAmountClass(log)}`}>{formatStockLogChangeAmount(log)}</p>{(log.total_price_cents ?? 0) > 0 && <p className="mt-1 text-xs">{log.change_amount < 0 ? '成本' : '总价'} {formatCents(log.total_price_cents)}</p>}{(log.unit_price_micro ?? 0) > 0 && <p className="mt-1 text-xs text-muted-foreground">单价 {formatMicro(log.unit_price_micro)}</p>}{onRevoke && canRevoke(log) && <Button variant="outline" size="sm" className="mt-3" disabled={busy} onClick={() => onRevoke(log)}>撤销</Button>}</div></article>;
  })}</div>;
}
