import { test, expect, mainName } from './fixtures';
const pages = [{ path: '/', title: '仪表盘' }, { path: '/components', title: '元件管理' }, { path: '/pre-stocks', title: '预入库' }, { path: '/categories', title: '分类管理' }, { path: '/tasks', title: '任务中心' }, { path: '/logs', title: '库存记录' }, { path: '/login', title: '欢迎回到 Hamster Bin' }];
for (const width of [390, 768, 1440]) {
  for (const theme of ['light', 'dark']) {
    for (const item of pages) {
      test(`${width}px ${theme} ${item.path} 页面布局`, async ({ page, api }, info) => {
        api.auth = item.path === '/login';
        const errors: string[] = []; page.on('pageerror', error => errors.push(error.message));
        await page.setViewportSize({ width, height: 900 }); await page.addInitScript(value => localStorage.setItem('hamster-theme', value), theme);
        await page.goto(item.path); await expect(page.getByRole('heading', { name: item.title, exact: true })).toBeVisible();
        await expect(page.locator('html')).toHaveClass(theme === 'dark' ? /dark/ : '');
        await expect.poll(() => page.getByRole('status', { name: '加载中', exact: true }).count()).toBe(0);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
        if (width < 768 && item.path !== '/login') { await page.getByRole('button', { name: '打开导航' }).click(); await expect(page.getByRole('dialog', { name: '导航' })).toBeVisible(); await page.keyboard.press('Escape'); await expect(page.getByRole('dialog', { name: '导航' })).toBeHidden(); }
        if (item.path === '/components') await expect(page.getByRole('button', { name: mainName, exact: true }).filter({ visible: true })).toBeVisible();
        await info.attach(`${width}-${theme}-${item.title}`, { body: await page.screenshot({ fullPage: true }), contentType: 'image/png' });
        expect(errors).toEqual([]);
      });
    }
    test(`${width}px ${theme} 表单底部操作可见`, async ({ page, api }) => {
      void api; await page.setViewportSize({ width, height: 844 }); await page.addInitScript(value => localStorage.setItem('hamster-theme', value), theme);
      await page.goto('/components?new=1'); const dialog = page.getByRole('dialog', { name: '添加元件' }); await expect(dialog).toBeVisible(); const box = await dialog.getByRole('button', { name: '保存', exact: true }).boundingBox(); expect(box!.y + box!.height).toBeLessThanOrEqual(844); expect(await dialog.evaluate(node => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
      await dialog.getByText('补充资料', { exact: false }).click(); await dialog.getByLabel('描述', { exact: true }).fill('较长资料'.repeat(100)); const after = await dialog.getByRole('button', { name: '保存', exact: true }).boundingBox(); expect(after!.y + after!.height).toBeLessThanOrEqual(844);
    });
  }
}
test('主题保存与跟随系统动态切换', async ({ page, api }) => {
  void api; await page.goto('/'); await expect(page.locator('html')).not.toHaveClass(/dark/); await page.getByRole('button', { name: '切换主题' }).click(); await page.getByRole('menuitemradio', { name: '深色模式' }).click(); await page.reload(); await expect(page.locator('html')).toHaveClass(/dark/);
  await page.getByRole('button', { name: '切换主题' }).click(); await page.getByRole('menuitemradio', { name: '跟随系统' }).click(); await page.emulateMedia({ colorScheme: 'light' }); await expect(page.locator('html')).not.toHaveClass(/dark/); await page.emulateMedia({ colorScheme: 'dark' }); await expect(page.locator('html')).toHaveClass(/dark/); expect(await page.evaluate(() => localStorage.getItem('hamster-theme'))).toBe('system');
});
test('扫码拒绝摄像头权限只尝试一次，并保留图片上传', async ({ page, api }) => {
  void api; await page.setViewportSize({ width: 390, height: 844 }); await page.addInitScript(() => { Object.defineProperty(navigator.mediaDevices, 'getUserMedia', { value: async () => { throw new DOMException('Permission denied', 'NotAllowedError'); } }); });
  await page.goto('/components'); await page.getByRole('button', { name: '扫码录入', exact: true }).click(); const dialog = page.getByRole('dialog', { name: '扫描二维码' }); await expect(dialog.getByRole('alert')).toContainText('无法启动摄像头'); await expect(dialog.getByRole('button', { name: '开始扫描' })).toBeEnabled(); await expect(dialog.getByLabel('上传二维码图片')).toBeAttached();
});
test('拍摄拒绝权限显示重试并可关闭', async ({ page, api }) => {
  void api; await page.addInitScript(() => { Object.defineProperty(navigator.mediaDevices, 'getUserMedia', { value: async () => { throw new DOMException('Permission denied', 'NotAllowedError'); } }); }); await page.goto('/components?new=1'); const form = page.getByRole('dialog', { name: '添加元件' }); await form.getByText('补充资料', { exact: false }).click(); await form.getByRole('button', { name: '拍摄图片', exact: true }).click(); const camera = page.getByRole('dialog', { name: '拍摄图片' }); await expect(camera.getByRole('alert')).toContainText('无法访问摄像头'); await expect(camera.getByRole('button', { name: '重试摄像头' })).toBeEnabled(); await camera.getByRole('button', { name: '关闭', exact: true }).first().click(); await expect(camera).toBeHidden(); await expect(form).toBeVisible();
});
