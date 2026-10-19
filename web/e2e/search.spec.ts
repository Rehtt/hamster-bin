import { test, expect } from './fixtures';

test('分类名称和状态由后端筛选，使用服务端分页及总数', async ({ page, api }) => {
  api.categories.push(...Array.from({ length: 23 }, (_, i) => ({ id: i + 10, name: `远程分类${String(i).padStart(2, '0')}`, status: 'active' as const })));
  api.categories.push({ id: 99, name: '远程分类待审核', status: 'pending' });
  await page.goto('/categories');
  await page.getByLabel('每页条数').selectOption('10');
  await page.getByRole('textbox', { name: '搜索分类' }).fill('远程分类');
  await page.getByRole('button', { name: '搜索', exact: true }).click();
  await page.getByLabel('分类状态').selectOption('active');
  await expect(page.getByText('第 1 / 3 页')).toBeVisible();
  await expect(page.getByRole('article').filter({ hasText: '远程分类待审核' })).toBeHidden();
  const request = page.waitForRequest(r => {
    const url = new URL(r.url());
    return url.pathname.endsWith('/categories') && url.searchParams.get('page') === '3' && url.searchParams.get('keyword') === '远程分类' && url.searchParams.get('status') === 'active';
  });
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await expect(page.getByText('第 2 / 3 页')).toBeVisible();
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await request;
  await expect(page.getByRole('article').filter({ hasText: '远程分类22' })).toBeVisible();
  await page.reload();
  await expect(page.getByText('第 3 / 3 页')).toBeVisible();
  // 删除最后一页的服务端数据后，刷新应回退到有效页。
  api.categories = api.categories.filter(c => c.id < 30 || c.id === 99);
  await page.reload();
  await expect(page).toHaveURL(/page=2/);
});

for (const field of [
  { label: '分类', source: 'categories', active: true },
  { label: '制造商', source: 'manufacturers' },
  { label: '封装', source: 'packages' },
  { label: '位置', source: 'locations' },
  { label: '供应商', source: 'suppliers' },
]) {
  test(`${field.label}下拉输入请求后端，直接展示服务端候选`, async ({ page, api }) => {
    void api;
    await page.goto('/components?new=1');
    const form = page.getByRole('dialog', { name: '添加元件' });
    await form.locator('summary').filter({ hasText: '采购信息' }).click();
    await page.route('**/api/v1/**', async route => {
      const url = new URL(route.request().url());
      if (url.searchParams.get('keyword') !== 'remote') return route.fallback();
      if (field.source === 'categories') {
        expect(url.searchParams.get('status')).toBe('active');
        return route.fulfill({ json: { data: [{ id: 998, name: '数据库候选分类', status: 'active' }] } });
      }
      if (field.source === 'suppliers') return route.fulfill({ json: { data: [{ id: 998, name: '数据库候选供应商' }] } });
      expect(url.searchParams.get('field')).toBe(field.source);
      return route.fulfill({ json: { data: { [field.source]: ['数据库候选选项'] } } });
    });
    await form.getByRole('combobox', { name: field.label, exact: true }).click();
    await page.getByPlaceholder(`搜索${field.label}`, { exact: true }).fill('remote');
    // 返回文字不包含输入词，证明组件没有再次进行前端匹配。
    await expect(page.getByRole('option', { name: /^数据库候选/ })).toBeVisible();
    await page.getByRole('option', { name: /^数据库候选/ }).click();
    await expect(form.getByRole('combobox', { name: field.label, exact: true })).toContainText('数据库候选');
  });
}

test('远程搜索可重试，迟到请求不能覆盖新关键词的结果', async ({ page, api }) => {
  void api;
  let failed = false;
  let oldFinished = false;
  let oldStarted = false;
  await page.route('**/api/v1/components/options?*', async route => {
    const keyword = new URL(route.request().url()).searchParams.get('keyword');
    if (keyword === 'retry' && !failed) {
      failed = true;
      return route.fulfill({ status: 503, json: { error: '暂时不可用' } });
    }
    if (keyword === 'old') {
      oldStarted = true;
      await new Promise(resolve => setTimeout(resolve, 900));
      await route.fulfill({ json: { data: { manufacturers: ['旧候选'] } } });
      oldFinished = true;
      return;
    }
    if (keyword === 'retry' || keyword === 'new') return route.fulfill({ json: { data: { manufacturers: ['最新候选'] } } });
    return route.fallback();
  });
  await page.goto('/components?new=1');
  await page.getByRole('dialog', { name: '添加元件' }).getByRole('combobox', { name: '制造商', exact: true }).click();
  const input = page.getByPlaceholder('搜索制造商', { exact: true });
  await input.fill('retry');
  await expect(page.getByRole('alert').filter({ hasText: '加载失败' })).toBeVisible();
  await page.getByRole('button', { name: '重试', exact: true }).click();
  await expect(page.getByRole('option', { name: '最新候选', exact: true })).toBeVisible();
  await input.fill('old');
  await expect.poll(() => oldStarted).toBe(true);
  await input.fill('new');
  await expect(page.getByRole('option', { name: '最新候选', exact: true })).toBeVisible();
  await expect.poll(() => oldFinished).toBe(true);
  await expect(page.getByRole('option', { name: '旧候选', exact: true })).toBeHidden();
  await expect(page.getByRole('option', { name: '最新候选', exact: true })).toBeVisible();
});
