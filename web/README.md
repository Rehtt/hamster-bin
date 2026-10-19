# Hamster Bin 前端开发

前端使用 React 19、TypeScript、Vite 7、Tailwind CSS 4、Radix UI 和 cmdk。固定使用 Node.js 24 与 pnpm 11.3.0（`packageManager`），依赖只由 `pnpm-lock.yaml` 锁定。

## 运行与验证

```bash
# 在 web/ 执行；先在仓库根目录 make run 启动后端
npm install --global pnpm@11.3.0  # 仅首次安装包管理器
pnpm install --frozen-lockfile
pnpm dev
pnpm lint
pnpm typecheck
pnpm test:auth
pnpm test:parser
pnpm build

# 浏览器测试：使用模拟 API，不访问真实供应商、不修改业务数据库
pnpm exec playwright install chromium
pnpm test:e2e
pnpm test:e2e:ui
```

开发服务器将 `/api` 代理到 `http://localhost:8080`。生产构建输出 `web/dist`，必须先构建前端再编译 Go；不要提交构建产物。根目录 `make install-frontend`、`make test-frontend`、`make build-frontend` 提供同样入口。

Playwright 默认使用其管理的 Chromium，也可通过 `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium` 使用本机浏览器。`e2e/fixtures.ts` 集中维护请求契约和测试数据；测试覆盖查询与 URL 恢复、详情/编辑、单次和批量出库、预入库确认、解析与照片结果恢复、手动修改保护、任务删除、价格补录/撤销、CSV/列偏好、表单关闭保护、错误重试、相机权限和主题保存。在 390/768/1440 像素、浅色/深色下检查全部页面、整体横向溢出和表单保存按钮。报告、截图和失败 trace 位于忽略的 `playwright-report` / `test-results`。

## 结构与交互约束

- `src/index.css`：Tailwind 4 官方 Vite 插件与 CSS 主题变量；默认浅色，`hamster-theme` 保存 `light` / `dark` / `system`。系统主题变化由 `ThemeProvider` 监听，入口脚本避免刷新闪色。兼容 Safari 16.4+、Chrome 111+、Firefox 128+（[官方要求](https://tailwindcss.com/docs/compatibility)）。
- `components/Layout.tsx`：可收起桌面侧栏、顶栏、手机导航抽屉和任务状态。原有 `hamster-sidebar-collapsed` 偏好保留。
- `components/ui/`：Radix Dialog/AlertDialog/DropdownMenu 提供焦点管理、键盘操作与 Esc；`SearchSelect` 使用 Radix Popover + cmdk；`PageState`、`Pagination`、`SafeImage` 统一状态、分页、图片失败占位。新增组件遵循同一主题令牌。
- `components/inventory/InventoryFields.tsx`：正式元件和预入库共用视图模型，基本资料/库存默认展开、采购/补充资料按需展开，校验后定位首个错误。两类业务的接口转换仍由各自页面处理。抽屉底部保存区固定，提交防重入，脏表单关闭和页面刷新受保护。
- `components/inventory/SmartEntry.tsx`：料号、扫码、拍照统一入口；复用已有解析任务、照片草稿和结果合并逻辑，供应商报价不写入本地库存/采购金额。
- `components/inventory/ComponentDetail.tsx`：通过现有详情和流水接口展示元件资料/库存/价格/近期流水；`StockLogList` 复用于详情、仪表盘和流水页。
- `components/inventory/columns.ts` / `ColumnSettings.tsx`：原有列偏好格式、CSV 字段、表头和拖拽排序。保留 `hamster-components-sort`、`hamster-components-table-columns`、`hamster-components-export-columns`。
- `api/inventory.ts`：独立草稿/提交类型，显式选择可编辑资料字段，集中保存、库存变更、补价和 CSV 请求；不回传实体 ID/关联对象/参考单价作为资料更新。旧元件 PUT 使用全量保存，原 `created_at` 作为独立元数据携带，避免编辑清空创建时间。
- `hooks/useListQuery.ts`：筛选/分页/排序更新保留其他 URL 参数（尤其 `applyTask`、`task`）。`useResource` 绑定请求参数并取消旧请求，避免旧结果覆盖新查询。
- `hooks/useFormGuard.ts`：显式快照比较与提交锁。`useParseTask` / `useTaskApplication` / `usePhotoDrafts` 保留会话隔离、多结果核对、后台继续执行和结果恢复。原页面路径全部保留；金额仍为总价分、单价微元。
- `pnpm-workspace.yaml`：仅允许 esbuild 安装脚本，运行脚本时校验依赖并报错，不隐式安装。修改依赖后先显式安装，再提交锁文件；CI 使用冻结安装。

API 客户端继续使用 `/api/v1`、Cookie 鉴权与原有 401 跳转。没有新增后端接口、迁移或业务配置。扫码与页面继续懒加载。
