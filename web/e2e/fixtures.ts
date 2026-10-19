import { test as base, expect, type Page } from '@playwright/test';
import type { Category, Component, ParseTask, PreStock, StockLog } from '../src/types';
export const mainName = '贴片电阻 10kΩ';
export type MockApi = {
  components: Component[]; preStocks: PreStock[]; tasks: ParseTask[]; categories: Category[]; logs: StockLog[];
  writes: { path: string; method: string; data: Record<string, unknown> }[];
  failList: boolean; failSave: boolean; auth: boolean;
};
const now = '2026-10-19T02:00:00Z';
export const parsedTask = (id = 101, status: ParseTask['status'] = 'succeeded'): ParseTask => ({
  id, type: 'component_parse', source: 'components', input: { code: 'C2040', parse_mode: 'standard' }, status, created_at: now,
  result: status === 'succeeded' ? { component: { name: '解析名称', model: 'PARSED-MODEL', manufacturer: 'YAGEO', category_name: '电阻', platform_code: 'C2040', platform_name: 'LCSC', value: '10kΩ', supplier_quote: { stock_quantity: 98765, currency: 'CNY', price_tiers: [{ min_quantity: 1, unit_price: '0.123456' }], queried_at: now } } } : null,
});
export async function mockApi(page: Page): Promise<MockApi> {
  const category: Category = { id: 1, name: '电阻', status:'active' };
  const components: Component[] = Array.from({ length: 25 }, (_, i) => ({ id: i + 1, component_number: `HB-${String(i + 1).padStart(6, '0')}`, name: i === 0 ? mainName : `测试元件${String(i + 1).padStart(2, '0')}`, model: `MODEL-${i + 1}`, manufacturer: 'YAGEO', value: '10kΩ', package: '0603', supplier_part_number: `C${2040 + i}`, description: '用于电路装配', stock_quantity: 35, unit_price_micro: 150000, location: 'A1-03', datasheet_url: '', image_url: '', category_id: 1, category, supplier_id: 1, supplier: { id: 1, name: '嘉立创' }, created_at: now, updated_at: now }));
  const state: MockApi = { components, categories: [category, { id: 2, name: '电容', status:'active' }], preStocks: [{ ...components[0], id: 1, expected_quantity: 10, total_price_cents: 2000, status: 'pending' }], tasks: [parsedTask()], logs: [{ id: 1, component_id: 1, component: components[0], change_amount: 35, unit_price_micro: 150000, total_price_cents: 525, reason: '初始入库', created_at: now }], writes: [], failList: false, failSave: false, auth: false };
  await page.route('**/api/v1/**', async route => {
    const request = route.request(); const url = new URL(request.url()); const path = url.pathname.replace('/api/v1', ''); const method = request.method();
    const ok = async (data: unknown) => route.fulfill({ json: { data } });
    const fail = async (status: number, error: string) => route.fulfill({ status, json: { error } });
    const list = async <T>(items: T[]) => { const page = Number(url.searchParams.get('page') || 1); const size = Number(url.searchParams.get('page_size') || 20); return route.fulfill({ json: { data: items.slice((page - 1) * size, page * size), pagination: { page, page_size: size, total: items.length, total_page: Math.ceil(items.length / size) } } }); };
    if (path === '/auth/config') return ok({ auth_enabled: state.auth, password_enabled: state.auth, oauth_enabled: false });
    if (path === '/auth/me') return fail(401, '请登录');
    if (path.endsWith('/image')) return fail(404, '无图');
    if (method !== 'GET') {
      const data = request.postDataJSON() as Record<string, unknown> || {};
      state.writes.push({ path, method, data });
      if (state.failSave) return fail(500, '保存失败，请重试');
      if (path === '/components/batch-stock-out') { for (const item of data.items as { component_id: number; quantity: number }[]) { const component = state.components.find(c => c.id === item.component_id)!; component.stock_quantity -= item.quantity; } return ok({}); }
      if (path === '/components/batch-location') { state.components.filter(c => (data.ids as number[]).includes(c.id)).forEach(c => { c.location = data.location as string; }); return ok({}); }
      if (path.match(/^\/components\/\d+\/stock$/)) { const c = state.components.find(c => c.id === Number(path.split('/')[2]))!; c.stock_quantity += Number(data.amount); return ok(c); }
      if (path.match(/^\/components\/\d+\/backfill-price$/)) return ok({});
      if (path.match(/^\/stock-logs\/\d+\/revoke$/)) { state.logs[0].revoked_at = now; return ok(state.logs[0]); }
      if (path === '/normalization/preview') return ok({candidate:{...data,category_id:data.category_id || state.categories.find(c=>c.status==='active' && c.name===data.category_name)?.id},normalization:{version:'test',sources:[],changes:[],issues:[]}});
      if (path.match(/^\/categories\/\d+$/) && method === 'PUT') { const category=state.categories.find(c=>c.id===Number(path.split('/')[2]))!; Object.assign(category,data); if(data.name)category.status='pending'; return ok(category); }
      if (path === '/tasks') { const t = { ...parsedTask(200, 'running'), ...data } as ParseTask; state.tasks.push(t); return ok(t); }
      if (path.match(/^\/tasks\/\d+$/) && method === 'DELETE') { state.tasks = state.tasks.filter(t => t.id !== Number(path.split('/')[2])); return ok({ deleted_ids: [Number(path.split('/')[2])], deleted_count: 1, skipped_count: 0 }); }
      if (path.match(/^\/pre-stocks\/\d+\/confirm$/)) { const item = state.preStocks.find(item => item.id === Number(path.split('/')[2]))!; item.status = 'confirmed'; item.component_id = 99; return ok(item); }
      if (path === '/categories' && method === 'POST') { const category: Category = { id: state.categories.length + 1, name: String(data.name), status:'pending' }; state.categories.push(category); return ok(category); }
      const match = path.match(/^\/components\/(\d+)$/);
      if (match) { const index = state.components.findIndex(c => c.id === Number(match[1])); if (method === 'DELETE') { state.components.splice(index, 1); return ok({}); } state.components[index] = { ...state.components[index], ...data } as Component; return ok(state.components[index]); }
      if (path === '/components' && method === 'POST') { const component = { ...state.components[0], ...data, id: 99 } as Component; state.components.push(component); return ok(component); }
      return ok({});
    }
    if (path === '/components/options') return ok({ manufacturers: ['YAGEO'], packages: ['0603', '0805'], locations: ['A1-03', 'B2-01'] });
    if (path === '/components/export') return route.fulfill({ contentType: 'text/csv', headers: { 'Content-Disposition': 'attachment; filename="components.csv"' }, body: '名称,库存数量\n贴片电阻,35\n' });
    if (path === '/components') {
      if (state.failList) return fail(503, '网络不可用');
      let items = state.components;
      for (const key of ['keyword', 'name', 'model', 'manufacturer', 'component_number', 'supplier_part_number', 'value']) { const value = url.searchParams.get(key); if (value) items = items.filter(c => (key === 'keyword' ? Object.values(c).join(' ') : String(c[key as keyof Component])).toLowerCase().includes(value.toLowerCase())); }
      if (url.searchParams.get('category_id')) items = items.filter(c => String(c.category_id) === url.searchParams.get('category_id'));
      return list(items);
    }
    if (path.match(/^\/components\/\d+$/)) { const c = state.components.find(c => c.id === Number(path.split('/')[2])); return c ? ok(c) : fail(404, '元件已删除'); }
    if (path === '/categories') return ok(state.categories);
    if (path === '/suppliers') return ok([{ id: 1, name: '嘉立创' }]);
    if (path === '/pre-stocks') return list(state.preStocks.filter(item => !url.searchParams.get('status') || url.searchParams.get('status') === 'all' || item.status === url.searchParams.get('status')));
    if (path.match(/^\/pre-stocks\/\d+$/)) return ok(state.preStocks.find(item => item.id === Number(path.split('/')[2])));
    if (path === '/stock-logs') return list(state.logs);
    if (path.match(/^\/components\/\d+\/logs$/)) return ok(state.logs);
    if (path === '/tasks') return list(state.tasks.filter(t => (!url.searchParams.get('status') || t.status === url.searchParams.get('status')) && (!url.searchParams.get('type') || t.type === url.searchParams.get('type'))));
    if (path.match(/^\/tasks\/\d+$/)) { const t = state.tasks.find(t => t.id === Number(path.split('/')[2])); return t ? ok(t) : fail(404, '任务已删除'); }
    if (path === '/stats') return ok({ range: url.searchParams.get('range') || 'month', component_count: 25, category_count: 2, total_stock: 875, inventory_value_cents: 13125, inbound_quantity: 100, outbound_quantity: 25, inbound_cost_cents: 1500, range_start: null, range_end: null });
    return fail(404, `未模拟接口 ${path}`);
  });
  return state;
}
export const test = base.extend<{ api: MockApi }>({ api: async ({ page }, provide) => { await provide(await mockApi(page)); } });
export { expect };
