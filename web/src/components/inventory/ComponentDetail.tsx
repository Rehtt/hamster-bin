import { ArrowDownToLine, Edit, History, Coins } from 'lucide-react';
import type { ApiResponse, Component, StockLog } from '../../types';
import { useResource } from '../../hooks/useResource';
import { Modal } from '../ui/Modal';
import { PageState } from '../ui/PageState';
import { Button } from '../ui/Button';
import { SafeImage } from '../ui/SafeImage';
import { formatMicro, calcOutboundCostCents, formatCents } from '../../utils/price';
import { StockLogList } from './StockLogList';
type Props = { id: number | null; onClose: () => void; onEdit: (c: Component) => void; onStock: (c: Component) => void; onBackfill: (c: Component) => void; onLogs: (c: Component) => void };
export function ComponentDetail(props: Props) { return props.id ? <DetailContent key={props.id} {...props} id={props.id} /> : null; }
function DetailContent({ id, onClose, onEdit, onStock, onBackfill, onLogs }: Props & { id: number }) {
  const resource = useResource<ApiResponse<Component>>(`/components/${id}`);
  const logs = useResource<ApiResponse<StockLog[]>>(`/components/${id}/logs`);
  const component = resource.data?.data;
  return <Modal isOpen onClose={onClose} title="元件详情" variant="drawer" footer={component && <><Button variant="outline" onClick={() => onEdit(component)}><Edit className="mr-2 size-4" />编辑资料</Button><Button onClick={() => onStock(component)}><ArrowDownToLine className="mr-2 size-4" />入库 / 出库</Button></>}>
    <PageState loading={resource.loading} error={resource.error} onRetry={resource.refresh} />
    {!resource.loading && !resource.error && component && <div className="space-y-6"><div className="flex items-start gap-4"><SafeImage src={`/api/v1/components/${id}/image`} alt={component.name} className="size-24 shrink-0 border" /><div className="min-w-0"><span className="status-badge">{component.category?.name || '未分类'}</span><h2 className="mt-3 break-all text-xl font-semibold">{component.name}</h2><p className="mt-2 break-all font-mono text-xs text-muted-foreground">{component.component_number || '暂无编号'}</p></div></div>
      <div className="grid grid-cols-2 gap-4 rounded-lg border bg-muted/40 p-5"><div><p className="text-xs text-muted-foreground">当前库存</p><p className="mt-2 text-3xl font-semibold tabular-nums">{component.stock_quantity}<span className="ml-2 text-xs font-normal text-muted-foreground">件</span></p></div><div><p className="text-xs text-muted-foreground">参考单价</p><p className="mt-2 text-lg font-semibold">{component.unit_price_micro == null ? '未记录' : formatMicro(component.unit_price_micro)}</p><p className="mt-1 text-xs text-muted-foreground">库存价值 {component.unit_price_micro == null ? '—' : formatCents(calcOutboundCostCents(component.unit_price_micro, component.stock_quantity))}</p></div></div>
      <section><h3 className="section-title mb-4">元件资料</h3><dl className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">{[['厂家型号', component.model], ['制造商', component.manufacturer], ['参数值', component.value], ['封装', component.package], ['位置', component.location], ['供应商', component.supplier?.name], ['供应商料号', component.supplier_part_number]].map(([label, value]) => <div key={label}><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1.5 break-all">{value || '—'}</dd></div>)}</dl>{!!component.review?.length && <p className="text-xs">资料复核：{component.review.map(r=>`${r.field === 'name' ? '名称' : '描述'}待整理，${r.confirmed ? '已确认保留原文' : '待确认'}`).join('；')}</p>}{component.description && <p className="mt-5 whitespace-pre-wrap break-all rounded-md bg-muted p-4 text-sm leading-6">{component.description}</p>}{component.datasheet_url && <a href={component.datasheet_url} target="_blank" rel="noreferrer" className="mt-4 inline-block text-primary hover:underline">查看数据手册 ↗</a>}</section>
      <div className="flex flex-wrap gap-2"><Button variant="outline" size="sm" onClick={() => onBackfill(component)}><Coins className="mr-2 size-4" />补录价格</Button><Button variant="outline" size="sm" onClick={() => onLogs(component)}><History className="mr-2 size-4" />管理流水与撤销</Button></div>
      <section><h3 className="section-title mb-3">最近库存流水</h3><div className="rounded-lg border"><PageState loading={logs.loading} error={logs.error} empty={!logs.data?.data.length} onRetry={logs.refresh} />{!logs.loading && !logs.error && <StockLogList logs={logs.data?.data.slice(0, 10) || []} showComponent={false} />}</div></section>
    </div>}
  </Modal>;
}
