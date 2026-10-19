import { test, expect, mainName, parsedTask } from './fixtures';
import type { Page } from '@playwright/test';
const mainButton = (page: Page) => page.getByRole('button', { name: mainName, exact: true }).filter({ visible: true });
async function openEditor(page: Page) {
  await page.goto('/components'); await mainButton(page).click();
  await page.getByRole('button', { name: '编辑资料' }).click();
  return page.getByRole('dialog', { name: '编辑元件' });
}
async function rowAction(page: Page, name: string) { await page.getByRole('button', { name: '操作', exact: true }).filter({ visible: true }).first().click(); await page.getByRole('menuitem', { name, exact: true }).click(); }

test('统一搜索、分类和高级筛选写入 URL，分页可恢复', async ({ page, api }) => {
  void api; await page.goto('/components');
  await page.getByLabel('每页条数').selectOption('10');
  await page.getByRole('button', { name: '下一页' }).click(); await expect(page).toHaveURL(/page=2/);
  await page.reload(); await expect(page.getByText('第 2 / 3 页')).toBeVisible();
  await page.getByRole('textbox', { name: '搜索元件' }).fill('贴片'); await page.getByRole('button', { name: '搜索', exact: true }).click();
  await expect(page).toHaveURL(/keyword=/); await expect(mainButton(page)).toBeVisible();
  await page.getByRole('combobox', { name: '分类筛选' }).click(); await page.getByRole('option', { name: '电容', exact: true }).click();
  await expect(page.getByText('没有匹配的结果')).toBeVisible();
  await page.getByRole('button', { name: '清空筛选', exact: true }).first().click();
  await page.getByRole('button', { name: '高级筛选' }).click(); await page.getByLabel('厂家型号', { exact: true }).fill('MODEL-1');
  await page.getByRole('button', { name: '应用条件' }).click(); await expect(page).toHaveURL(/model=MODEL-1/);
  await page.getByLabel('排序方向').selectOption('asc'); await expect(page).toHaveURL(/sort_order=asc/);
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('hamster-components-sort') || '{}').sortOrder)).toBe('asc');
});

test('详情资料、库存、流水与编辑保存', async ({ page, api }) => {
  const dialog = await openEditor(page); await expect(dialog.getByRole('textbox', { name: '名称', exact: true })).toHaveValue(mainName);
  await dialog.getByRole('textbox', { name: '名称', exact: true }).fill('手工更新名称'); await dialog.getByRole('button', { name: '保存', exact: true }).click();
  await expect(dialog).toBeHidden(); await expect(page.getByRole('button', { name: '手工更新名称', exact: true }).filter({ visible: true })).toBeVisible();
  const payload = api.writes.find(w => w.method === 'PUT' && w.path === '/components/1')?.data;
  expect(payload?.name).toBe('手工更新名称'); expect(payload).not.toHaveProperty('created_at'); expect(payload).not.toHaveProperty('unit_price_micro'); expect(payload).not.toHaveProperty('id'); expect(payload).not.toHaveProperty('category');
});

test('单次出库数量校验和变更预览', async ({ page, api }) => {
  await page.goto('/components'); await mainButton(page).click(); await page.getByRole('button', { name: '入库 / 出库' }).click();
  const dialog = page.getByRole('dialog', { name: '库存变更' }); await dialog.getByRole('button', { name: '出库', exact: true }).click();
  await dialog.locator('input[type=number]').fill('5'); await expect(dialog.getByText('30', { exact: true })).toBeVisible();
  await expect(dialog.getByText(/预估成本：¥0.75/)).toBeVisible(); await dialog.getByRole('button', { name: '确认', exact: true }).click();
  await expect(dialog).toBeHidden(); expect(api.components[0].stock_quantity).toBe(30);
  expect(api.writes.find(w => w.path === '/components/1/stock')?.data.amount).toBe(-5);
});

test('批量出库保留逐行数量和统一备注', async ({ page, api }) => {
  await page.goto('/components'); await page.getByRole('checkbox', { name: `选择 ${mainName}`, exact: true }).filter({ visible: true }).check();
  await page.getByRole('button', { name: '批量出库', exact: true }).last().click();
  const dialog = page.getByRole('dialog', { name: '批量出库' }); await dialog.locator('input[type=number]').fill('3'); await dialog.getByPlaceholder('出库原因，如项目装配').fill('项目装配');
  await dialog.getByRole('button', { name: '确认出库' }).click(); await expect(dialog).toBeHidden();
  expect(api.writes.find(w => w.path === '/components/batch-stock-out')?.data).toEqual({ reason: '项目装配', items: [{ component_id: 1, quantity: 3 }] });
});

test('预入库状态切换与确认入库', async ({ page, api }) => {
  await page.goto('/pre-stocks'); await page.getByRole('button', { name: '操作', exact: true }).click(); await page.getByRole('menuitem', { name: '确认入库' }).click();
  await expect(page.getByRole('alertdialog')).toBeVisible(); await page.getByRole('alertdialog').getByRole('button', { name: '确认', exact: true }).click();
  await expect(page.getByText('没有匹配的结果')).toBeVisible(); await page.getByRole('button', { name: '已入库', exact: true }).click(); await expect(page).toHaveURL(/status=confirmed/); expect(api.preStocks[0].status).toBe('confirmed');
});

test('关闭未修改的编辑表单直接关闭，修改后确认放弃', async ({ page, api }) => {
  void api; const dialog = await openEditor(page); await dialog.getByRole('button', { name: '关闭', exact: true }).click(); await expect(dialog).toBeHidden(); await expect(page.getByRole('alertdialog')).toBeHidden();
  await mainButton(page).click(); await page.getByRole('button', { name: '编辑资料' }).click(); await dialog.getByRole('textbox', { name: '名称', exact: true }).fill('尚未保存'); await page.keyboard.press('Escape');
  const confirm = page.getByRole('alertdialog'); await expect(confirm).toBeVisible(); await confirm.getByRole('button', { name: '取消' }).click(); await expect(dialog.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('尚未保存');
  await dialog.getByRole('button', { name: '取消', exact: true }).click(); await confirm.getByRole('button', { name: '放弃修改' }).click(); await expect(dialog).toBeHidden();
});

test('删除元件必须确认，取消不发请求', async ({ page, api }) => {
  await page.goto('/components'); await rowAction(page, '删除'); await page.getByRole('alertdialog').getByRole('button', { name: '取消' }).click(); expect(api.writes).toHaveLength(0);
  await rowAction(page, '删除'); await page.getByRole('alertdialog').getByRole('button', { name: '确认', exact: true }).click(); await expect(mainButton(page)).toBeHidden(); expect(api.writes.some(w => w.path === '/components/1' && w.method === 'DELETE')).toBe(true);
});

test('任务结果可读、诊断默认折叠并可应用', async ({ page, api }) => {
  void api; await page.goto('/tasks?task=101'); const detail = page.getByRole('dialog', { name: '任务 #101' }); await expect(detail.getByText('解析名称', { exact: true })).toBeVisible();
  await expect(detail.locator('details')).not.toHaveAttribute('open', ''); await detail.getByRole('button', { name: '应用结果' }).click();
  await expect(page).toHaveURL(/\/components/); const form = page.getByRole('dialog', { name: '添加元件' }); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('解析名称'); await expect(form.getByLabel('库存数量', { exact: false })).toHaveValue('0'); await expect(form.getByText('98765', { exact: false })).toBeVisible();
});

test('解析迟到结果保留手动修改和本地数量', async ({ page, api }) => {
  await page.goto('/components?new=1'); const form = page.getByRole('dialog', { name: '添加元件' }); await form.getByRole('textbox', { name: '供应商料号解析' }).fill('C2040'); await form.getByRole('button', { name: '解析', exact: true }).click();
  await expect.poll(() => api.writes.some(w => w.path === '/tasks')).toBe(true);
  await form.getByRole('textbox', { name: '名称', exact: true }).fill('人工名称'); await form.getByLabel('库存数量', { exact: false }).fill('7'); api.tasks = api.tasks.map(t => t.id === 200 ? parsedTask(200) : t);
  await expect(form.getByLabel('厂家型号', { exact: true })).toHaveValue('PARSED-MODEL', { timeout: 8000 }); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('人工名称'); await expect(form.getByLabel('库存数量', { exact: false })).toHaveValue('7');
});

test('关闭表单后不消费迟到解析结果', async ({ page, api }) => {
  await page.goto('/components?new=1'); const form = page.getByRole('dialog', { name: '添加元件' }); await form.getByRole('textbox', { name: '供应商料号解析' }).fill('C2040'); await form.getByRole('button', { name: '解析', exact: true }).click();
  await expect.poll(() => api.writes.some(w => w.path === '/tasks')).toBe(true); await form.getByRole('button', { name: '关闭', exact: true }).click(); await expect(form).toBeHidden(); api.tasks = api.tasks.map(t => t.id === 200 ? parsedTask(200) : t);
  await page.getByRole('button', { name: '添加元件', exact: true }).click(); await page.waitForTimeout(2500); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('');
});

test('照片多结果恢复先核对再进入业务表单', async ({ page, api }) => {
  api.tasks = [{ ...parsedTask(102), type: 'image_parse', result: { items: [{ name: '电阻', category_name: '电阻', model: 'R1', manufacturer: '', value: '', package: '', supplier_name: '', supplier_part_number: 'C1', description: '', quantity: null }, { name: '电容', category_name: '电容', model: 'C1', manufacturer: '', value: '', package: '', supplier_name: '', supplier_part_number: 'C2', description: '', quantity: 0 }] } }];
  await page.goto('/components?applyTask=102'); const review = page.getByRole('dialog', { name: '拍照 AI 解析' }); await expect(review.getByRole('button', { name: '应用选中结果' })).toBeDisabled();
  await review.getByLabel('结果 1 数量', { exact: true }).fill('5'); await review.getByRole('button', { name: '应用选中结果' }).click(); const form = page.getByRole('dialog', { name: '添加元件' }); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('电阻'); await expect(form.getByLabel('库存数量', { exact: false })).toHaveValue('5');
});

test('任务删除后轮询不恢复详情或列表', async ({ page, api }) => {
  await page.goto('/tasks?task=101&status=succeeded'); const detail = page.getByRole('dialog', { name: '任务 #101' }); await detail.getByRole('button', { name: '删除', exact: true }).click(); await page.getByRole('alertdialog').getByRole('button', { name: '确认', exact: true }).click(); await expect(detail).toBeHidden(); await expect(page).not.toHaveURL(/task=101/); await page.waitForTimeout(2200); expect(api.tasks).toHaveLength(0); await expect(page.getByText('#101 C2040', { exact: true })).toBeHidden();
});

test('列设置偏好和 CSV 导出保留筛选', async ({ page, api }) => {
  void api; await page.goto('/components?keyword=贴片'); await page.getByRole('button', { name: '列设置', exact: true }).click(); const columns = page.getByRole('dialog', { name: '列设置' }); await columns.getByRole('checkbox', { name: '厂家型号', exact: true }).uncheck(); await columns.getByRole('button', { name: '确认', exact: true }).click();
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('hamster-components-table-columns') || '[]').find((c: {key: string}) => c.key === 'model')?.selected)).toBe(false);
  await page.getByRole('button', { name: '导出 CSV', exact: true }).click(); const download = page.waitForEvent('download'); const request = page.waitForRequest(r => r.url().includes('/components/export')); await page.getByRole('button', { name: '确认导出' }).click(); expect((await request).url()).toContain('keyword='); expect((await download).suggestedFilename()).toBe('components.csv');
});

test('价格补录和流水撤销保留金额单位', async ({ page, api }) => {
  await page.goto('/components'); await rowAction(page, '补录价格'); const dialog = page.getByRole('dialog', { name: /补录价格/ }); await dialog.getByPlaceholder('本次采购总价').fill('12.34'); await dialog.locator('input[type=number]').last().fill('10'); await dialog.getByRole('button', { name: '确认', exact: true }).click(); await expect(dialog).toBeHidden(); expect(api.writes.find(w => w.path.endsWith('/backfill-price'))?.data).toEqual({ total_price_cents: 1234, quantity: 10 });
  await page.goto('/logs'); await page.getByRole('button', { name: '撤销', exact: true }).click(); await page.getByRole('alertdialog').getByRole('button', { name: '确认撤销' }).click(); await expect(page.getByText('已撤销', { exact: true })).toBeVisible();
});

test('网络失败可重试，保存失败保留输入', async ({ page, api }) => {
  api.failList = true; await page.goto('/components'); await expect(page.getByRole('alert')).toContainText('数据加载失败'); api.failList = false; await page.getByRole('button', { name: '重试', exact: true }).click(); await expect(mainButton(page)).toBeVisible();
  api.failSave = true; await mainButton(page).click(); await page.getByRole('button', { name: '编辑资料' }).click(); const form = page.getByRole('dialog', { name: '编辑元件' }); await form.getByRole('textbox', { name: '名称', exact: true }).fill('失败时保留'); await form.getByRole('button', { name: '保存', exact: true }).click(); await expect(page.getByText('保存失败，请重试', { exact: true })).toBeVisible(); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toHaveValue('失败时保留');
});

test('表单校验定位到首个错误字段，Esc 关闭菜单后恢复焦点', async ({ page, api }) => {
  void api; await page.goto('/components'); await page.getByRole('button', { name: '添加元件', exact: true }).click(); const form = page.getByRole('dialog', { name: '添加元件' }); await form.getByRole('button', { name: '保存', exact: true }).click(); await expect(form.getByRole('textbox', { name: '名称', exact: true })).toBeFocused(); await expect(form.getByText('请输入元件名称')).toBeVisible(); await form.getByRole('button', { name: '关闭', exact: true }).click();
  const button = page.getByRole('button', { name: '操作', exact: true }).filter({ visible: true }).first(); await button.click(); await page.keyboard.press('Escape'); await expect(button).toBeFocused();
});

test('任务恢复保护已有资料并逐字段选择替换', async ({page,api})=>{
 api.tasks=[{...parsedTask(301),record_id:1}];
 await page.goto('/components?applyTask=301'); const form=page.getByRole('dialog',{name:'编辑元件'});
 await expect(form.getByRole('textbox',{name:'名称',exact:true})).toHaveValue(mainName);
 await form.getByRole('button',{name:'替换名称',exact:true}).click();
 await expect(form.getByRole('textbox',{name:'名称',exact:true})).toHaveValue('解析名称');
 await expect(form.getByLabel('厂家型号',{exact:true})).toHaveValue('MODEL-1');
});

test('分类逐项审核启用，改名重新待审核',async({page,api})=>{
 api.categories[0].status='pending';await page.goto('/categories');
 await page.getByRole('combobox',{name:'分类状态'}).selectOption('pending'); await expect(page).toHaveURL(/status=pending/); await expect(page.getByRole('button',{name:'操作',exact:true})).toHaveCount(1);
 await page.getByRole('button',{name:'操作',exact:true}).click();
 await page.getByRole('menuitem',{name:'审核启用'}).click();
 await expect.poll(()=>api.categories[0].status).toBe('active');
 await page.getByRole('combobox',{name:'分类状态'}).selectOption('active');
 await page.getByRole('button',{name:'操作',exact:true}).first().click();await page.getByRole('menuitem',{name:'编辑',exact:true}).click();
 await page.getByLabel('分类名称',{exact:true}).fill('贴片电阻');await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect.poll(()=>api.categories[0].status).toBe('pending');
});

test('原文确认绑定字段值，修改后需要重新预览确认',async({page,api})=>{
 void api;
 await page.route('**/api/v1/normalization/preview',async route=>{const input=route.request().postDataJSON();await route.fulfill({json:{data:{candidate:input,normalization:{version:'test',sources:[],changes:[],issues:[{field:'name',code:'needs_review',message:'请明确确认保留原文'}]}}}});});
 await page.goto('/components?new=1');const form=page.getByRole('dialog',{name:'添加元件'});
 await form.getByRole('textbox',{name:'名称',exact:true}).fill('Unknown');await form.getByRole('button',{name:'本地规范预览'}).click();
 await form.getByRole('checkbox',{name:'确认保留名称原文（修改后需重新确认）'}).check();
 await form.getByRole('textbox',{name:'名称',exact:true}).fill('Changed');
 await expect(form.getByRole('checkbox',{name:'确认保留名称原文（修改后需重新确认）'})).toBeHidden();
 await form.getByRole('button',{name:'本地规范预览'}).click();await expect(form.getByRole('checkbox',{name:'确认保留名称原文（修改后需重新确认）'})).not.toBeChecked();
});
