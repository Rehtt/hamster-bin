import { useState } from 'react';
import { toast } from 'react-hot-toast';
import client from '../../api/client';
import type { NormalizationResult, ParsedComponentInfo } from '../../types';
import type { InventoryFieldsValue } from './InventoryFields';
import { Button } from '../ui/Button';

const labels: Record<string,string> = {name:'名称', description:'描述', model:'型号', manufacturer:'制造商', package:'封装', category:'分类 ID', value:'参数', supplier_part_number:'供应商料号', datasheet_url:'数据手册', image_url:'图片',quantity:'数量',supplier:'供应商'};
export function NormalizationReview({value,onChange,parsed}: {value:InventoryFieldsValue;onChange:(patch:Partial<InventoryFieldsValue>)=>void;parsed?:ParsedComponentInfo}) {
 const [preview,setPreview]=useState<{input:string;result:NormalizationResult}>();
 const [busy,setBusy]=useState(false);
 const input = JSON.stringify({name:value.name || '',description:value.description || '',model:value.model || '',manufacturer:value.manufacturer || '',package:value.package || '',category_id:Number(value.category)||0});
 const currentPreview = preview?.input === input ? preview.result : undefined;
 const run = async () => {setBusy(true);try {const res=await client.post('/normalization/preview',JSON.parse(input));setPreview({input,result:res.data.data});}catch{toast.error('预览失败，请重试');}finally{setBusy(false);}};
 const candidate: Partial<InventoryFieldsValue> = currentPreview ? {...currentPreview.candidate,category:currentPreview.candidate.category_id ? String(currentPreview.candidate.category_id) : ''} : parsed ? {
  name:parsed.name,description:parsed.description,model:parsed.model,manufacturer:parsed.manufacturer,package:parsed.package,value:parsed.value,
  category:parsed.category_id ? String(parsed.category_id) : '',supplier:parsed.supplier_name,supplier_part_number:parsed.platform_code,datasheet_url:parsed.datasheet_url,image_url:parsed.image_url,quantity:parsed.quantity,
 } : {};
 const meta=currentPreview ? {...currentPreview.normalization,sources:parsed?.normalization?.sources || [],primary_source:parsed?.normalization?.primary_source,issues:[...currentPreview.normalization.issues,...(parsed?.normalization?.issues.filter(i=>i.code==='semantic_candidate' || i.code==='source_conflict') || [])]} : parsed?.normalization;
 const original=meta?.sources.find(source=>source.name===meta.primary_source)?.fields || meta?.sources[0]?.fields;
 const sourceValue = (key: string) => {
  const fields=original as Record<string,unknown> | undefined;
  if (key==='category') return fields?.category_name;
  if (key==='supplier') return fields?.supplier_name;
  if (key==='supplier_part_number') return fields?.supplier_part_number || fields?.platform_code;
  return fields?.[key];
 };
 const keys=(Object.keys(candidate) as (keyof InventoryFieldsValue)[]).filter(k=>labels[k] && candidate[k]!==undefined && candidate[k]!=='' && candidate[k]!==null);
 return <div className="rounded-md border p-3 space-y-3">
  <div className="flex items-center justify-between gap-2"><span className="font-medium">规范与差异确认</span><Button variant="outline" onClick={()=>void run()} disabled={busy}>{busy?'预览中…':'本地规范预览'}</Button></div>
  {currentPreview && parsed && <Button variant="outline" onClick={()=>setPreview(undefined)}>查看解析候选</Button>}
  <p className="text-xs text-muted-foreground">已有值默认保留。逐字段选择替换；未知封装允许保留。需要保留原文时，先预览并确认。</p>
  {meta && <><p className="text-xs">规则版本：{meta.version}</p><div className="overflow-x-auto"><table className="w-full text-xs"><thead><tr><th>字段</th><th>来源原文</th><th>候选值</th><th>当前表单值</th><th>选择</th></tr></thead><tbody>{keys.map(key=><tr key={key}><td>{labels[key]}</td><td>{String(sourceValue(key) ?? meta.changes.find(c=>c.field===key)?.original ?? '—')}</td><td>{String(candidate[key])}</td><td>{String(value[key] ?? '')}</td><td>{candidate[key]!==value[key] && <Button size="sm" variant="outline" onClick={()=>onChange({[key]:candidate[key]})}>替换{labels[key]}</Button>}</td></tr>)}</tbody></table></div>
  {meta.issues.map((issue,i)=><p key={i} className="text-xs">{labels[issue.field] || issue.field}：{issue.message}{issue.sources?.length ? `（${issue.sources.join('、')}）` : ''}</p>)}
  {currentPreview?.normalization.issues.filter(i=>i.code==='needs_review').map(issue=><label key={issue.field} className="flex items-center gap-2 text-sm"><input type="checkbox" checked={value.review_confirmations?.[issue.field] === value[issue.field as 'name'|'description']} onChange={e=>onChange({review_confirmations:{...value.review_confirmations,[issue.field]:e.target.checked ? String(value[issue.field as 'name'|'description'] || '') : ''}})} />确认保留{labels[issue.field]}原文（修改后需重新确认）</label>)}
  {!!meta.sources.length && <details><summary className="text-xs cursor-pointer">来源对照</summary>{meta.sources.map((source,i)=><div key={i} className="text-xs break-all space-y-1"><p>{source.name} · {source.queried_at}{source.truncated ? ' · 参数文本已截断' : ''}</p>{source.url && <a href={source.url} target="_blank" rel="noreferrer">{source.url}</a>}<pre className="whitespace-pre-wrap">{JSON.stringify(source.fields,null,2)}</pre>{source.parameters && <pre className="whitespace-pre-wrap">{source.parameters}</pre>}</div>)}</details>}
  </>}
 </div>;
}
