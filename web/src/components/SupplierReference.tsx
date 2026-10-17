import type { ParsedComponentInfo } from '../types';

export function SupplierReference({ component }: { component?: ParsedComponentInfo }) {
  const quote = component?.supplier_quote;
  if (!quote) return null;
  return <section className="rounded-md border p-3 space-y-2 text-sm" aria-label="供应商参考信息">
    <p className="font-medium">{quote.source_name || '嘉立创'}参考信息 · {component.platform_code}</p>
    {quote.source_url && <p className="break-all">查询地址：<a href={quote.source_url} target="_blank" rel="noreferrer" className="underline">{quote.source_url}</a></p>}
    <p>供应商库存：{quote.stock_quantity ?? '未知'} · 币种：{quote.currency || '未知'}</p>
    {quote.price_tiers.length ? <table className="w-full text-left">
      <thead><tr><th>起订数量</th><th>参考单价（{quote.currency || '币种未知'}）</th></tr></thead>
      <tbody>{quote.price_tiers.map((tier, index) => <tr key={`${tier.min_quantity}-${index}`}><td>≥ {tier.min_quantity}</td><td>{tier.unit_price}</td></tr>)}</tbody>
    </table> : <p>暂无报价</p>}
    <p className="text-muted-foreground">查询时间：<time dateTime={quote.queried_at}>{new Date(quote.queried_at).toLocaleString()}</time></p>
    <p className="text-muted-foreground">仅供参考；供应商库存不计入本地数量，报价不计入采购金额。</p>
  </section>;
}
