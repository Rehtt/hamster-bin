import type { ComponentType } from 'react';
import { Link } from 'react-router-dom';
import { Package, Layers, Database, CircleDollarSign, ArrowDownToLine, ArrowUpFromLine, ArrowRight, Plus, ClipboardList, Sparkles } from 'lucide-react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageHeader } from '../components/ui/PageHeader';
import { PageState } from '../components/ui/PageState';
import { StockLogList } from '../components/inventory/StockLogList';
import { useResource } from '../hooks/useResource';
import { useListQuery } from '../hooks/useListQuery';
import type { ApiResponse, DashboardStats, StatsRange, StockLog } from '../types';
import { formatCents } from '../utils/price';
const ranges: { value: StatsRange; label: string }[] = [{ value: 'month', label: '本月' }, { value: 'quarter', label: '本季' }, { value: 'all', label: '全部' }];
export default function Dashboard() {
  const { params, updateQuery } = useListQuery();
  const range = params.get('range') === 'quarter' || params.get('range') === 'all' ? params.get('range')! : 'month';
  const resource = useResource<ApiResponse<DashboardStats>>(`/stats?range=${range}`);
  const logs = useResource<ApiResponse<StockLog[]>>('/stock-logs?page=1&page_size=5');
  const stats = resource.data?.data;
  return <div className="space-y-7"><PageHeader title="仪表盘" actions={<Link to="/components?new=1"><Button><Plus className="mr-2 size-4" />录入元件</Button></Link>} />
    <section className="relative overflow-hidden rounded-xl border border-primary/15 bg-card p-6 sm:p-8"><div className="absolute -right-12 -top-16 size-64 rounded-full bg-primary/5" /><div className="relative flex items-center justify-between gap-4"><div><span className="text-[10px] font-semibold tracking-[0.2em] text-primary">YOUR INVENTORY, ORGANIZED</span><h2 className="mt-3 text-xl font-semibold sm:text-2xl">从一颗元件，到下一个灵感。</h2><p className="mt-2 text-sm leading-6 text-muted-foreground">查找、采购、入库，所有元件资料和库存变化尽在掌握。</p><Link to="/components" className="mt-5 inline-flex items-center gap-2 text-sm font-medium text-primary">查看元件库存<ArrowRight className="size-4" /></Link></div><div aria-hidden className="hidden size-24 shrink-0 rotate-6 items-center justify-center rounded-3xl border border-primary/10 bg-primary/5 sm:flex"><Package className="size-12 text-primary/70" strokeWidth={1.2} /></div></div></section>
    {resource.error && <div className="surface"><PageState error={resource.error} onRetry={resource.refresh} /></div>}
    <section aria-label="库存概况"><div className="mb-4 flex items-center justify-between"><h2 className="section-title">库存概况</h2><span className="text-xs text-muted-foreground">当前全部库存</span></div><div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <StatCard title="元件种类" value={stats?.component_count} icon={Package} loading={resource.loading} unit="种" />
      <StatCard title="库存总量" value={stats?.total_stock} icon={Database} loading={resource.loading} unit="件" />
      <StatCard title="分类数量" value={stats?.category_count} icon={Layers} loading={resource.loading} unit="类" />
      <StatCard title="库存总价值" value={stats && (stats.inventory_value_cents === 0 ? '¥0.00' : formatCents(stats.inventory_value_cents))} icon={CircleDollarSign} loading={resource.loading} />
    </div></section>
    <section aria-label="期间统计"><div className="mb-4 flex flex-wrap items-center justify-between gap-3"><h2 className="section-title">期间统计</h2><div aria-label="统计范围" className="flex rounded-lg border bg-card p-1">{ranges.map(option => <Button key={option.value} size="sm" variant={range === option.value ? 'secondary' : 'ghost'} aria-pressed={range === option.value} onClick={() => updateQuery({ range: option.value })}>{option.label}</Button>)}</div></div><div className="grid gap-3 sm:grid-cols-3"><StatCard title="累计入库金额" value={stats && (stats.inbound_cost_cents === 0 ? '¥0.00' : formatCents(stats.inbound_cost_cents))} icon={CircleDollarSign} loading={resource.loading} /><StatCard title="入库数量" value={stats?.inbound_quantity} icon={ArrowDownToLine} loading={resource.loading} unit="件" /><StatCard title="出库数量" value={stats?.outbound_quantity} icon={ArrowUpFromLine} loading={resource.loading} unit="件" /></div></section>
    <div className="grid items-start gap-6 xl:grid-cols-[1fr_300px]"><section className="surface"><div className="flex items-center justify-between border-b p-5"><h2 className="section-title">最近库存流水</h2><Link to="/logs" className="flex items-center gap-1 text-xs text-primary">查看全部<ArrowRight className="size-3" /></Link></div><PageState loading={logs.loading} error={logs.error} empty={!logs.data?.data.length} onRetry={logs.refresh} />{!logs.loading && !logs.error && <StockLogList logs={logs.data?.data || []} />}</section><section className="surface p-5"><h2 className="section-title mb-4">快捷入口</h2><div className="space-y-2">{[{ to: '/components?new=1', title: '新增元件', text: '料号、扫码或照片智能录入', icon: Plus }, { to: '/pre-stocks', title: '采购待入库', text: '核对采购，确认收货', icon: ClipboardList }, { to: '/tasks', title: '解析任务', text: '查看进度与应用结果', icon: Sparkles }].map(item => <Link key={item.to} to={item.to} className="flex items-center gap-3 rounded-lg p-3 hover:bg-muted"><div className="rounded-lg bg-primary/5 p-2 text-primary"><item.icon className="size-4" /></div><div><p className="text-sm font-medium">{item.title}</p><p className="mt-1 text-xs text-muted-foreground">{item.text}</p></div><ArrowRight className="ml-auto size-4 text-muted-foreground" /></Link>)}</div></section></div>
  </div>;
}
function StatCard({ title, value, icon: Icon, loading, unit }: { title: string; value?: number | string; icon: ComponentType<{ className?: string }>; loading: boolean; unit?: string }) {
  return <Card className="p-4 sm:p-5"><div className="flex items-center justify-between gap-2"><p className="text-xs text-muted-foreground">{title}</p><Icon className="size-4 text-primary/70" /></div>{loading ? <div aria-label="加载中" className="mt-5 h-8 w-20 animate-pulse rounded bg-muted" /> : <p className="mt-5 break-all text-2xl font-semibold tracking-tight tabular-nums">{value ?? '—'}{unit && value != null && <span className="ml-2 text-xs font-normal text-muted-foreground">{unit}</span>}</p>}</Card>;
}
