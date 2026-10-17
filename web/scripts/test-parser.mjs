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
  await test('historical JSON retains original timestamp and decimal precision', () => {
    assert.equal(render(JSON.parse(JSON.stringify(component))), render(component));
  });
} finally {
  await server.close();
}
