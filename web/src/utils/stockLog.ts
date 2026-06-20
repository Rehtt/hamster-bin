import { type StockLog } from '../types';

export function isRevoked(log: StockLog): boolean {
  return !!log.revoked_at;
}

export function isReversal(log: StockLog): boolean {
  return !!log.reversal_of_id;
}

export function canRevoke(log: StockLog): boolean {
  return !isRevoked(log) && !isReversal(log);
}

export function isBackfillLog(log: StockLog): boolean {
  return log.change_amount === 0;
}

export function stockLogAmountClass(log: StockLog): string {
  if (isRevoked(log)) return 'text-muted-foreground line-through';
  if (isReversal(log)) return 'text-muted-foreground';
  if (isBackfillLog(log)) return 'text-amber-700';
  return log.change_amount > 0 ? 'text-green-600' : 'text-red-600';
}

export function stockLogIconClass(log: StockLog): string {
  if (isRevoked(log) || isReversal(log)) return 'bg-muted text-muted-foreground';
  if (isBackfillLog(log)) return 'bg-amber-100 text-amber-700';
  return log.change_amount > 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700';
}

export function stockLogCardClass(log: StockLog): string {
  if (isRevoked(log)) return 'opacity-60';
  if (isReversal(log)) return 'bg-muted/30';
  return '';
}

export function formatStockLogChangeAmount(log: StockLog): string {
  if (isBackfillLog(log)) return '补录';
  return `${log.change_amount > 0 ? '+' : ''}${log.change_amount}`;
}

export function stockLogIconLabel(log: StockLog): string {
  if (isBackfillLog(log)) return '¥';
  return log.change_amount > 0 ? '+' : '-';
}
