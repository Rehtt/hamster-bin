import type { ParsedComponentInfo } from '../types';

// 只回填提交后未修改的字段；空解析值保留当前值。
export function mergeParsed<T extends object>(current: T, parsed: Partial<T>, snapshot?: T): T {
  const next = { ...current };
  for (const key of Object.keys(parsed) as (keyof T)[]) {
    const value = parsed[key];
    if (value !== '' && value !== undefined && value !== null && (current[key] === undefined || current[key] === null || current[key] === '') && (!snapshot || current[key] === snapshot[key])) next[key] = value as T[keyof T];
  }
  return next;
}

// 显式挑选业务表单资料；供应商报价和库存始终留在参考快照中。
export function parsedComponentFields(info: ParsedComponentInfo) {
  const fields = {
    name: info.name || '',
    model: info.model || '',
    manufacturer: info.manufacturer || '',
    value: info.value || '',
    package: info.package || '',
    supplier_part_number: info.platform_code || '',
    description: info.description || '',
    datasheet_url: info.datasheet_url || '',
    image_url: info.image_url || '',
  };
  for (const change of info.normalization?.changes || []) if (change.semantic && change.field in fields) delete (fields as Partial<typeof fields>)[change.field as keyof typeof fields];
  for (const issue of info.normalization?.issues || []) if (issue.code === 'source_conflict' && issue.field in fields) delete (fields as Partial<typeof fields>)[issue.field as keyof typeof fields];
  return fields;
}

export function matchingSupplierReference(info: ParsedComponentInfo | undefined, partNumber: string, queryCode: string) {
  const normalize = (value: string) => value.trim().toUpperCase();
  return info?.supplier_quote && info.platform_code &&
    normalize(info.platform_code) === normalize(partNumber) && normalize(info.platform_code) === normalize(queryCode)
    ? info : undefined;
}
