import type { ParsedComponentInfo, ReviewedImageItem } from '../types';

// 照片字段与平台解析字段不同，显式转换以保留供应商料号。
export function photoComponentInfo(item: ReviewedImageItem): ParsedComponentInfo {
  return {
    name: item.name, category_name: item.category_name, model: item.model,
    manufacturer: item.manufacturer, value: item.value, package: item.package,
    platform_code: item.supplier_part_number, description: item.description,
  };
}
