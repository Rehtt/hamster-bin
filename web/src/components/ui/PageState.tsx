import { Inbox, RotateCw } from 'lucide-react';
import { Button } from './Button';
export function PageState({ loading, error, empty, filtered, onRetry, onClear }: { loading?: boolean; error?: string; empty?: boolean; filtered?: boolean; onRetry?: () => void; onClear?: () => void }) {
  if (loading) return <div role="status" aria-label="加载中" className="space-y-3 p-6">{[1, 2, 3].map(i => <div key={i} className="h-14 animate-pulse rounded-md bg-muted" />)}</div>;
  if (error) return <div role="alert" className="flex flex-col items-center gap-3 px-4 py-10 text-center"><p className="font-medium">{error}</p><p className="text-sm text-muted-foreground">请检查网络连接后重试。</p><Button variant="outline" onClick={onRetry}><RotateCw className="mr-2 size-4" />重试</Button></div>;
  if (empty) return <div className="flex flex-col items-center gap-3 px-4 py-12 text-center"><div className="rounded-xl bg-muted p-3"><Inbox className="size-6 text-muted-foreground" /></div><p className="font-medium">{filtered ? '没有匹配的结果' : '暂无数据'}</p><p className="text-sm text-muted-foreground">{filtered ? '试试减少条件或更换关键词。' : '添加第一条记录，开始管理你的元件库存。'}</p>{filtered && onClear && <Button variant="outline" onClick={onClear}>清空筛选</Button>}</div>;
  return null;
}
