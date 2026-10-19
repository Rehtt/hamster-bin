import client from './client';
import type { ApiResponse, Component, PreStock } from '../types';
// 表单可编辑字段与服务端实体分开，禁止把 ID、关联对象和参考单价回传为资料修改。
type MaterialFields = Pick<Component, 'name' | 'model' | 'manufacturer' | 'value' | 'package' | 'supplier_part_number' | 'description' | 'location' | 'datasheet_url' | 'image_url'>;
export type ComponentDraft = Partial<MaterialFields> & {
  review_confirmations?: Record<string, string>; component_number?: string | null; category_id?: number; supplier_id?: number | null; stock_quantity?: number;
};
export type ComponentSaveInput = MaterialFields & {
  review_confirmations?: Record<string, string>; component_number?: string; category_id: number; supplier_id: number | null; stock_quantity: number; total_price_cents?: number;
};
export type PreStockSaveInput = Omit<ComponentSaveInput, 'stock_quantity'> & { expected_quantity: number };
export function componentSaveInput(draft: ComponentDraft, categoryID: number, supplierID: number | null, totalPriceCents: number, creating: boolean): ComponentSaveInput {
  return {
    review_confirmations: draft.review_confirmations,
    component_number: draft.component_number?.trim() || undefined,
    category_id: categoryID, supplier_id: supplierID,
    name: draft.name || '', model: draft.model || '', manufacturer: draft.manufacturer || '',
    value: draft.value || '', package: draft.package || '', supplier_part_number: draft.supplier_part_number || '',
    description: draft.description || '', location: draft.location || '', datasheet_url: draft.datasheet_url || '', image_url: draft.image_url || '',
    stock_quantity: Number(draft.stock_quantity),
    ...(creating && Number(draft.stock_quantity) > 0 && totalPriceCents > 0 ? { total_price_cents: totalPriceCents } : {}),
  };
}
export const inventoryApi = {
  saveComponent: (id: number | undefined, input: ComponentSaveInput) => id ? client.put<ApiResponse<Component>>(`/components/${id}`, input) : client.post<ApiResponse<Component>>('/components', input),
  savePreStock: (id: number | undefined, input: PreStockSaveInput) => id ? client.put<ApiResponse<PreStock>>(`/pre-stocks/${id}`, input) : client.post<ApiResponse<PreStock>>('/pre-stocks', input),
  changeStock: (id: number, input: { amount: number; reason: string; total_price_cents?: number }) => client.post(`/components/${id}/stock`, input),
  backfillPrice: (id: number, input: { total_price_cents: number; quantity: number }) => client.post(`/components/${id}/backfill-price`, input),
  exportCSV: (params: URLSearchParams) => client.get<Blob>('/components/export', { params, responseType: 'blob' }),
};
