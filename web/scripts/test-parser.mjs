import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { createServer } from 'vite';

const server = await createServer({
  server: { middlewareMode: true, ws: false, hmr: false, watch: null },
  optimizeDeps: { noDiscovery: true, include: [] },
  logLevel: 'error',
});
try {
  const { ParseModeSelect } = await server.ssrLoadModule('/src/components/ParseModeSelect.tsx');
  const { ParseTraceView } = await server.ssrLoadModule('/src/components/ParseTraceView.tsx');
  const { SupplierReference } = await server.ssrLoadModule('/src/components/SupplierReference.tsx');
  const { parsedComponentFields, matchingSupplierReference, mergeParsed } = await server.ssrLoadModule('/src/utils/parseResult.ts');
  const { TaskResponseGuard, pruneTaskSelection, lastTaskPage, taskDeleteConfirmation } = await server.ssrLoadModule('/src/utils/taskDeletion.ts');
  await test('task selection retains only terminal tasks on the current page during polling', () => {
    const page = [{ id: 1, status: 'succeeded' }, { id: 2, status: 'failed' }, { id: 3, status: 'cancelled' }, { id: 4, status: 'queued' }, { id: 5, status: 'running' }];
    assert.deepEqual(pruneTaskSelection([1, 2, 3, 4, 5, 6], page), [1, 2, 3]);
    assert.deepEqual(pruneTaskSelection([1, 2], page), [1, 2]);
    assert.deepEqual(pruneTaskSelection([1, 2], [{ id: 7, status: 'succeeded' }]), []);
  });
  await test('task deletion rejects late responses and keeps tombstones across subsequent polling', async () => {
    const guard = new TaskResponseGuard();
    const beforeDelete = guard.snapshot();
    let resolve;
    const lateResponse = new Promise(done => { resolve = done; });
    const apply = lateResponse.then(task => guard.accepts(beforeDelete) && !guard.has(task.id));
    guard.forget([1, 3]); resolve({ id: 1 });
    assert.equal(await apply, false);
    assert.equal(guard.has(1), true);
    assert.equal(guard.has(2), false);
    assert.equal(guard.accepts(guard.snapshot()), true);
    const beforeFilter = guard.snapshot(); guard.invalidate();
    assert.equal(guard.accepts(beforeFilter), false);
    assert.equal(guard.has(1), true);
  });
  await test('task cleanup confirmation explains scope, permanence, and image references', () => {
    for (const mode of ['failed', 'all', 'older_than_7_days']) {
      const message = taskDeleteConfirmation(mode, []);
      for (const text of ['覆盖全部分页与筛选', '不可恢复', '没有其他任务引用', '排队和运行中的任务保留']) assert.ok(message.includes(text), text);
    }
    assert.ok(taskDeleteConfirmation('older_than_7_days', []).includes('168 小时'));
    assert.ok(taskDeleteConfirmation('selected', [5], true).includes('任务 #5'));
    assert.ok(taskDeleteConfirmation('selected', [5, 6]).includes('2 项'));
    assert.ok(!taskDeleteConfirmation('selected', [5, 6]).includes('覆盖全部分页'));
    assert.equal(lastTaskPage(0, 20), 1);
    assert.equal(lastTaskPage(20, 20), 1);
    assert.equal(lastTaskPage(21, 20), 2);
  });
  const component = {
    platform_code: 'C25804', name: 'RES 10kΩ', model: '0603WAF1002T5E',
    supplier_quote: { stock_quantity: 0, currency: 'USD',
      price_tiers: [{ min_quantity: 100, unit_price: '0.0024' }, { min_quantity: 1000, unit_price: '0.0020' }],
      source_name: 'EasyEDA', source_url: 'https://pro.easyeda.com/api/eda/product/search?keyword=C25804',
      queried_at: '2026-01-02T03:04:05Z' },
  };
  const render = value => renderToStaticMarkup(React.createElement(SupplierReference, { component: value }));
  await test('three modes retain the selected value', () => {
    for (const mode of ['standard', 'llm', 'agent']) {
      const html = renderToStaticMarkup(React.createElement(ParseModeSelect, { value: mode, onChange() {} }));
      assert.match(html, new RegExp(`value="${mode}" selected=""`));
      for (const text of ['普通解析', 'AI 整理', 'Agent 解析']) assert.ok(html.includes(text));
    }
  });
  await test('trace shows source, fallback and query diagnostics', () => {
    const html = renderToStaticMarkup(React.createElement(ParseTraceView, { trace: {
      mode: 'agent', final_source: 'easyeda_product_search', fallback: true, fallback_reason: '预算耗尽',
      calls: [{ tool: 'easyeda_product_search', arguments: { code: 'C25804' }, endpoint: 'https://pro.lceda.cn/api/eda/product/search', started_at: '2026-01-02T03:04:05Z', duration_ms: 25, status: 'success', summary: 'RES', cached: true }],
    } }));
    for (const text of ['调用记录', 'agent', 'easyeda_product_search', '预算耗尽', 'C25804', '25', 'success', '缓存', 'https://pro.lceda.cn/api/eda/product/search']) assert.ok(html.includes(text), text);
    assert.equal(renderToStaticMarkup(React.createElement(ParseTraceView, {})), '');
  });
  await test('renders USD decimal strings, zero stock and original query time', () => {
    const html = render(component);
    for (const text of ['EasyEDA参考信息', 'https://pro.easyeda.com/api/eda/product/search?keyword=C25804', 'C25804', 'USD', '0.0024', '0.0020', '供应商库存：0']) assert.ok(html.includes(text), text);
    assert.match(html, /datetime="2026-01-02T03:04:05Z"/i);
    assert.ok(!html.includes('¥'));
  });
  await test('missing values explicitly show unknown and no quote', () => {
    const html = render({ ...component, supplier_quote: { ...component.supplier_quote, stock_quantity: null, currency: null, price_tiers: [] } });
    assert.ok(html.includes('供应商库存：未知'));
    assert.ok(html.includes('币种：未知'));
    assert.ok(html.includes('暂无报价'));
    assert.equal(render({ platform_code: 'C25804' }), '');
  });
  await test('reference follows current part number and query input', () => {
    assert.equal(matchingSupplierReference(component, ' c25804 ', 'C25804'), component);
    for (const [part, query] of [['C123', 'C25804'], ['C25804', 'C123'], ['C25804', '']]) {
      assert.equal(matchingSupplierReference(component, part, query), undefined);
    }
    assert.equal(matchingSupplierReference(undefined, 'C25804', 'C25804'), undefined);
  });
  await test('form mapping excludes supplier stock, quote and purchase price', () => {
    const fields = parsedComponentFields(component);
    for (const field of ['supplier_quote', 'stock_quantity', 'expected_quantity', 'price', 'unit_price_micro', 'total_price_cents', 'total_price_yuan']) assert.ok(!(field in fields), field);
    assert.equal(fields.supplier_part_number, 'C25804');
    const current = { stock_quantity: 7, expected_quantity: 8, total_price_yuan: '12.00', name: 'manual name' };
    const merged = mergeParsed(current, fields, { ...current, name: 'old name' });
    assert.equal(merged.stock_quantity, 7);
    assert.equal(merged.expected_quantity, 8);
    assert.equal(merged.total_price_yuan, '12.00');
    assert.equal(merged.name, 'manual name');
  });
  await test('manual values survive parsing before submission, late results and task restoration', () => {
    const current={name:'人工名称',description:'人工描述',model:'',package:'SOT32',quantity:7};
    for(const snapshot of [undefined,{...current},{...current,model:'before edit'}]) {
      const merged=mergeParsed(current,{name:'候选名称',description:'',model:'M1',package:'SOT-32',quantity:99},snapshot);
      assert.equal(merged.name,'人工名称'); assert.equal(merged.description,'人工描述'); assert.equal(merged.package,'SOT32'); assert.equal(merged.quantity,7);
      assert.equal(merged.model,snapshot?.model==='before edit' ? '' : 'M1');
    }
    const fields=parsedComponentFields({name:'AI名称',description:'AI描述',normalization:{changes:[{field:'name',semantic:true}],issues:[{field:'description',code:'source_conflict'}]}});
    assert.ok(!('name' in fields)); assert.ok(!('description' in fields));
  });
  await test('historical JSON retains original timestamp and decimal precision', () => {
    assert.equal(render(JSON.parse(JSON.stringify(component))), render(component));
  });
} finally {
  await server.close();
}
