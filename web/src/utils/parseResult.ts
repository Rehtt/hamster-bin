import type { ParsedComponentInfo } from '../types';

// 只回填提交后未修改的字段；空解析值保留当前值。
export function mergeParsed<T extends object>(current: T, parsed: Partial<T>, snapshot?: T): T {
  const next = { ...current };
  for (const key of Object.keys(parsed) as (keyof T)[]) {
    const value = parsed[key];
    if (value !== '' && value !== undefined && value !== null && (!snapshot || current[key] === snapshot[key])) next[key] = value as T[keyof T];
  }
  return next;
}

// 显式挑选业务表单资料；供应商报价和库存始终留在参考快照中。
export function parsedComponentFields(info: ParsedComponentInfo) {
  return {
    name: info.name || info.model || '',
    model: info.model || '',
    manufacturer: info.manufacturer || '',
    value: info.value || '',
    package: info.package || '',
    supplier_part_number: info.platform_code || '',
    description: info.description || '',
    datasheet_url: info.datasheet_url || '',
    image_url: info.image_url || '',
  };
}

export function matchingSupplierReference(info: ParsedComponentInfo | undefined, partNumber: string, queryCode: string) {
  const normalize = (value: string) => value.trim().toUpperCase();
  return info?.supplier_quote && info.platform_code &&
    normalize(info.platform_code) === normalize(partNumber) && normalize(info.platform_code) === normalize(queryCode)
    ? info : undefined;
}
