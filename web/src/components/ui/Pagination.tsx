import type { Pagination as PaginationData } from '../../types';
import { Button } from './Button';
export function Pagination({ pagination, onPageChange, onPageSizeChange, disabled = false }: { pagination: PaginationData; onPageChange: (page: number) => void; onPageSizeChange?: (size: number) => void; disabled?: boolean }) {
  const pages = Math.max(1, pagination.total_page ?? Math.ceil(pagination.total / pagination.page_size));
  return <nav aria-label="分页" className="flex flex-wrap items-center justify-between gap-3 py-2 text-sm">
    <div className="flex items-center gap-3 text-muted-foreground"><span>共 {pagination.total} 条</span>{onPageSizeChange && <label className="flex items-center gap-2">每页<select aria-label="每页条数" className="h-9 py-0" value={pagination.page_size} disabled={disabled} onChange={e => onPageSizeChange(Number(e.target.value))}>{[10, 20, 50, 100].map(size => <option key={size} value={size}>{size}</option>)}</select>条</label>}</div>
    <div className="flex items-center gap-2"><Button size="sm" variant="outline" disabled={disabled || pagination.page <= 1} onClick={() => onPageChange(pagination.page - 1)}>上一页</Button><span className="px-1 text-xs text-muted-foreground">第 {pagination.page} / {pages} 页</span><Button size="sm" variant="outline" disabled={disabled || pagination.page >= pages} onClick={() => onPageChange(pagination.page + 1)}>下一页</Button></div>
  </nav>;
}
