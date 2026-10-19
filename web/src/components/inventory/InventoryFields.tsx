import { NormalizationReview } from './NormalizationReview';
import type { Category, ParsedComponentInfo, ReviewState } from '../../types';
import { Link } from 'react-router-dom';
import { useState, type FormEvent, type ReactNode } from 'react';
import { Minus, Plus } from 'lucide-react';
import { Input } from '../ui/Input';
import { Label } from '../ui/Label';
import { Button } from '../ui/Button';
import { SearchSelect } from '../ui/SearchSelect';
import { QuantityShortcuts } from '../ui/QuantityShortcuts';
import { SafeImage } from '../ui/SafeImage';
import { calcUnitPriceMicro, formatMicro, yuanToCents } from '../../utils/price';
// 表单视图模型；正式库存和预入库在各自页面转换为接口提交类型。
export type InventoryFieldsValue = {
  review_confirmations?: Record<string,string>; component_number?: string | null; name?: string; model?: string; manufacturer?: string;
  value?: string; package?: string; supplier_part_number?: string; description?: string;
  location?: string; datasheet_url?: string; image_url?: string;
  quantity: number | null | undefined; category: string; supplier: string; totalPrice: string;
};
export function InventoryFields({ value, onChange, onSubmit, categories, suppliers, manufacturers, packages, locations, editing, preStock = false, imageControls, imagePreview, busy, parsed, review }: {
  value: InventoryFieldsValue; onChange: (patch: Partial<InventoryFieldsValue>) => void; onSubmit: () => void;
  review?: ReviewState[]; categories: Category[]; parsed?: ParsedComponentInfo; suppliers: string[]; manufacturers: string[]; packages: string[]; locations: string[];
  editing?: boolean; preStock?: boolean; imageControls?: ReactNode; imagePreview?: string; busy?: boolean;
}) {
  const [errors, setErrors] = useState<Record<string, string>>({});
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const next: Record<string, string> = {};
    if (!value.name?.trim()) next.name = '请输入元件名称';
    if (!value.category.trim()) next.category = '请选择或输入分类';
    if (!Number.isSafeInteger(value.quantity) || Number(value.quantity) < 0) next.quantity = '请输入非负整数数量';
    if (value.totalPrice && (!Number.isFinite(Number(value.totalPrice)) || Number(value.totalPrice) < 0)) next.totalPrice = '请输入有效的非负金额';
    setErrors(next);
    const first = Object.keys(next)[0];
    if (first) {
      const target = event.currentTarget.querySelector<HTMLElement>(`[id="inventory-${first}"]`);
      const section = target?.closest('details'); if (section) section.open = true;
      target?.focus(); target?.scrollIntoView({ block: 'center', behavior: 'smooth' });
      return;
    }
    onSubmit();
  };
  const field = (key: keyof InventoryFieldsValue, label: string, placeholder = '', options?: string[]) => <div className="form-field" key={key}><Label htmlFor={`inventory-${key}`}>{label}{(key === 'name' || key === 'category') && <span aria-hidden="true" className="ml-1 text-destructive">*</span>}</Label>{options ? <SearchSelect source={key === 'supplier' ? 'suppliers' : key === 'manufacturer' ? 'manufacturers' : key === 'package' ? 'packages' : 'locations'} id={`inventory-${key}`} label={label} value={String(value[key] || '')} onChange={v => onChange({ [key]: v })} creatable={key !== 'category'} options={options.map(v => ({ value: v, label: v }))} placeholder={placeholder || `输入或选择${label}`} /> : <Input id={`inventory-${key}`} value={String(value[key] || '')} aria-invalid={!!errors[key]} aria-describedby={errors[key] ? `error-${key}` : undefined} onChange={e => onChange({ [key]: e.target.value })} placeholder={placeholder} />}{errors[key] && <p role="alert" id={`error-${key}`} className="text-xs text-destructive">{errors[key]}</p>}</div>;
  return <form id="inventory-form" noValidate onSubmit={submit} className="space-y-4"><fieldset disabled={busy} className="min-w-0 space-y-4">
    {!!review?.length && <p className="text-xs">资料复核：{review.map(r => `${r.field === 'name' ? '名称' : '描述'}待整理，${r.confirmed ? '已确认保留原文' : '待确认'}`).join('；')}</p>}
    <NormalizationReview value={value} onChange={onChange} parsed={parsed} />
    <details className="form-section" open><summary>基本资料<span className="ml-auto mr-4 text-xs font-normal text-muted-foreground">必填项标注 *</span></summary><div className="form-grid">
      {field('name', '名称', '例如：贴片电阻 10kΩ')}<div className="form-field"><Label htmlFor="inventory-category">分类 *</Label><SearchSelect source="active-categories" keepCategoryID={editing ? value.category : undefined} id="inventory-category" label="分类" value={value.category} onChange={category => onChange({category})} options={categories.map(c => ({value:String(c.id),label:c.name + (c.status === 'active' ? '' : '（未启用，仅保留原关联）')}))} placeholder="选择启用分类" /><Link to="/categories" className="text-xs text-primary">审核或新增分类 →</Link>{errors.category && <p role="alert">{errors.category}</p>}</div>
      {field('component_number', '元件编号', '留空则保存时自动生成')}{field('model', '厂家型号', '例如 RC0603FR-0710KL')}
      {field('manufacturer', '制造商', '', manufacturers)}{field('value', '参数值', '例如 10kΩ ±1%')}{field('package', '封装', '', packages)}
    </div></details>
    <details className="form-section" open><summary>库存与位置</summary><div className="form-grid">
      <div className="form-field"><Label htmlFor="inventory-quantity">{preStock ? '预计数量' : '库存数量'}<span aria-hidden="true" className="ml-1 text-destructive">*</span></Label><div className="flex gap-2"><Button variant="outline" size="icon" aria-label="减少数量" onClick={() => onChange({ quantity: Math.max(0, Number(value.quantity || 0) - 1) })}><Minus className="size-4" /></Button><Input id="inventory-quantity" type="number" inputMode="numeric" min="0" step="1" aria-invalid={!!errors.quantity} value={value.quantity ?? ''} onChange={e => onChange({ quantity: e.target.value === '' ? null : Number(e.target.value) })} className="text-center" /><Button variant="outline" size="icon" aria-label="增加数量" onClick={() => onChange({ quantity: Number(value.quantity || 0) + 1 })}><Plus className="size-4" /></Button></div><QuantityShortcuts value={value.quantity ?? undefined} onSelect={quantity => onChange({ quantity })} />{errors.quantity && <p role="alert" className="text-xs text-destructive">{errors.quantity}</p>}</div>
      {field('location', '位置', '', locations)}
    </div></details>
    <details className="form-section"><summary>采购信息<span className="ml-auto mr-4 text-xs font-normal text-muted-foreground">供应商、采购金额</span></summary><div className="form-grid">
      {field('supplier', '供应商', '', suppliers)}{field('supplier_part_number', '供应商料号', '例如 C2040')}
      {(!editing || preStock) && <div className="form-field sm:col-span-2"><Label htmlFor="inventory-totalPrice">采购总价（元）</Label><Input id="inventory-totalPrice" type="number" inputMode="decimal" step="0.01" min="0" value={value.totalPrice} onChange={e => onChange({ totalPrice: e.target.value })} placeholder="可选，按数量分摊单价" /><p className="text-xs text-muted-foreground">{Number(value.quantity) > 0 && yuanToCents(value.totalPrice) > 0 ? `分摊单价：${formatMicro(calcUnitPriceMicro(yuanToCents(value.totalPrice), Number(value.quantity)))}` : '采购金额用于本地库存成本，与供应商参考报价分别记录。'}</p>{errors.totalPrice && <p role="alert" className="text-xs text-destructive">{errors.totalPrice}</p>}</div>}
    </div></details>
    <details className="form-section"><summary>补充资料<span className="ml-auto mr-4 text-xs font-normal text-muted-foreground">图片、描述、数据手册</span></summary><div className="space-y-4"><div className="form-field"><Label htmlFor="inventory-description">描述</Label><textarea id="inventory-description" rows={3} className="w-full" value={value.description || ''} onChange={e => onChange({ description: e.target.value })} /></div><div className="form-grid">{field('datasheet_url', '数据手册 URL', 'https://...')}{field('image_url', '图片 URL', 'https://...')}</div><SafeImage src={imagePreview || value.image_url} alt="元件图片预览" className="h-40 w-full border" />{imageControls}</div></details>
  </fieldset></form>;
}
