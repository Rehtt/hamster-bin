# Hamster Bin Project Guide for AI Agents

## 项目概览

Hamster Bin 是一个电子元件库存管理系统。后端使用 Go、Gin、GORM，默认 SQLite，也支持 MySQL/PostgreSQL；前端使用 React、TypeScript、Vite 和 Tailwind CSS。系统支持元件、预入库、分类、供应商、库存日志管理，并提供立创商城、淘宝、二维码解析和可选 LLM 辅助解析相关能力。

AI agent 修改项目时，应优先保持现有目录分层、API 路径、数据模型和构建流程稳定。若代码修改影响本文档记录的结构、职责、运行方式、API、配置或约束，必须同步更新本文档。

## 文件树与职责

```text
.
├── README.md                  # 项目介绍、快速开始、配置和部署说明
├── docs/screenshots/           # README 页面截图，使用示例数据展示主要页面及浅色/深色主题
├── LICENSE                    # MIT 许可证
├── Dockerfile                 # 多阶段 Docker 镜像构建（Node.js 24/pnpm + Go 静态编译 + Alpine 运行时）
├── docker-compose.yml         # Docker Compose 部署（默认本地 build + SQLite 卷）
├── .env.example               # Compose 环境变量模板（复制为 .env 后使用）
├── .dockerignore              # Docker build context 排除规则
├── Makefile                   # 后端运行、构建、跨平台构建和前端构建入口
├── go.mod / go.sum            # Go 模块定义和依赖锁定
├── embed.go                   # 将 web/dist 嵌入 Go 二进制
├── cmd/
│   └── server/
│       └── main.go            # 服务入口：处理 --version、加载配置、初始化数据库、注册解析器、管理任务 worker 与 HTTP 服务信号退出
├── internal/
│   ├── config/                # 环境变量配置加载
│   ├── auth/                  # JWT、凭据校验、OIDC 流程、会话认证、令牌加密及刷新
│   ├── database/              # SQLite/MySQL/PostgreSQL 的 GORM 初始化、自动迁移、数据库实例管理
│   ├── handlers/              # Gin HTTP handlers，处理分类、供应商、元件、批量保存、库存日志、解析和鉴权请求
│   ├── middleware/            # Gin 中间件（鉴权）
│   ├── mcp/                   # 远程 MCP：独立 Bearer/Origin 校验、类型化工具参数、工具注册及 SDK 集成测试
│   ├── llm/                   # OpenAI-compatible Chat Completions 客户端
│   ├── models/                # 库存、任务及 OAuth 身份/会话的 GORM 数据模型
│   ├── normalization/        # 版本化纯规则、规范读取接口、来源/变化/问题和复核确认类型
│   ├── price/                 # 单价（微元）与总价（分）换算及加权平均
│   ├── parser/                # 平台解析器、二维码/照片 AI 解析、解析器管理器和解析测试
│   ├── tasks/                 # 持久化队列执行、照片存储、校验、超时、取消、重试与中断恢复
│   ├── repository/            # 数据访问封装，按业务实体拆分
│   ├── router/                # API 路由、CORS、嵌入式前端静态文件服务和 SPA fallback
│   └── version/               # 项目版本变量，默认 v1.0.0，release 构建时通过 ldflags 注入 git tag
├── web/
│   ├── package.json           # 前端脚本和依赖
│   ├── pnpm-lock.yaml         # 唯一前端依赖锁文件（pnpm 11.3.0）
│   ├── pnpm-workspace.yaml    # 安装脚本白名单与运行前依赖校验
│   ├── vite.config.ts         # Vite 配置；生产构建通过 manualChunks 拆分 react、lucide、html5-qrcode 等 vendor
│   ├── eslint.config.js       # 前端 ESLint 配置
│   ├── tsconfig*.json         # TypeScript 配置
│   ├── playwright.config.ts   # Chromium 浏览器回归配置
│   ├── e2e/                   # 模拟 API、关键流程与响应式浏览器测试
│   ├── index.html             # 中文入口、浏览器标题及首屏主题应用
│   ├── scripts/test-auth.mjs   # 复用 Vite SSR 加载器的登录入口/路由保护渲染测试
│   ├── public/
│   │   └── logo.svg           # 静态 logo
│   └── src/
│       ├── main.tsx           # React 挂载入口
│       ├── App.tsx            # 前端路由：Dashboard、Components、Categories、StockLogs、Login
│       ├── api/client.ts      # Axios 客户端，默认 baseURL 为 /api/v1，withCredentials 携带 Cookie
│       ├── context/           # React 鉴权上下文（AuthProvider、AuthContext、useAuth）
│       ├── components/        # 布局、扫码、拍照、路由保护和可复用 UI 组件
│       ├── pages/             # 主要业务页面
│       ├── types/             # 前端共享类型
│       ├── utils/             # 前端工具函数（price.ts 金额换算/格式化，stockLog.ts 流水展示，clipboard.ts 剪贴板复制）
│       └── assets/            # 前端源码内资源
└── web_legacy/
    └── index.html             # 旧版前端页面，保留作历史/兼容参考
```

## 前端工具链与组件约束

- 使用 Node.js 24（根目录 `.node-version`）、pnpm 11.3.0（`web/package.json#packageManager`）。仅保留 `web/pnpm-lock.yaml`，不得重新引入 npm 锁文件。`pnpm-workspace.yaml` 允许 esbuild 构建脚本，运行前依赖校验使用 error，不隐式变更依赖。修改依赖后先安装；CI、Docker 用 `pnpm install --frozen-lockfile`。
- `InventoryFields` 使用独立 `InventoryFieldsValue` 视图模型；两类库存页面各自转换原接口 payload，不混用数量字段。基本资料/库存默认展开，采购/补充资料默认折叠，错误定位字段；保存中禁用并防重入。`useFormGuard` 比较显式快照，关闭脏表单需确认，刷新通过 beforeunload 保护。后台解析不阻塞关闭。
- `SmartEntry` 复用原任务解析流程；`ComponentDetail` 调用现有元件详情/流水接口；`StockLogList` 复用于仪表盘、详情和流水页；`TaskResultView` 优先显示可读字段/照片结果，JSON 和诊断折叠。
- `useListQuery` 更新 URL 时保留任务参数；`useResource` 以 URL/刷新版本绑定结果并取消旧请求。保留全部页面路径、API 前缀/Cookie/401 规则、总价分/单价微元单位，无后端接口或迁移变化。
- `make install-frontend` 安装依赖，`make test-frontend` 执行 lint/typecheck/登录/解析测试；`make build-frontend` 输出 `web/dist`。`web/playwright.config.ts` 与 `e2e/` 使用模拟接口验证关键流程、三个屏宽和两种主题，运行 `pnpm exec playwright install chromium` → `pnpm test:e2e`。可通过 `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` 指定系统 Chromium。CI 执行冻结安装、前端检查/构建/浏览器测试、Go 测试及临时目录二进制构建。

## 后端结构

- `cmd/server/main.go` 是唯一服务入口。它支持 `--version` 输出版本；正常启动时调用 `config.Load()`、`database.Init()`、注册 `parser.ParserManager`，然后通过 `router.Setup(db, parserManager, cfg, taskService)` 启动 Gin 服务。
- `internal/config/config.go` 从环境变量读取配置，当前包含 `PORT`、`DB_DRIVER`、`DB_DSN`、`DB_PATH`、`IMAGE_DIR`、`LOG_LEVEL`、`SSL_CERT`、`SSL_KEY`、`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`、`ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET`、`JWT_EXPIRE_HOURS`、`MCP_TOKEN`、`MCP_ALLOWED_ORIGINS`、`TASK_WORKERS`、`TASK_TIMEOUT_SECONDS`。另含 `OAUTH_ENABLED`、`OAUTH_ISSUER`、`OAUTH_CLIENT_ID`、`OAUTH_CLIENT_SECRET`、`OAUTH_REDIRECT_URL`、`OAUTH_AUTH_METHOD`。管理员凭据完整或启用 OAuth 时保护业务接口，`JWT_SECRET` 必填；OAuth 配置不完整、URL 非法或认证方式不支持时启动报错。
- `internal/auth/` 负责 JWT 签发/解析（Cookie 名 `hamster_token`）、管理员凭据恒定时间比较及 OIDC。`service.go` 是 handler/middleware 共用认证入口；`oauth.go` 使用 go-oidc v3.21.0 与 oauth2 v0.36.0 完成发现、ID Token/UserInfo 校验、显式刷新/撤销；`oauth_logout.go` 校验后端通道退出令牌并关闭目标会话；`oauth_flow.go` 管理短期浏览器登录流程；`oauth_crypto.go` 用 HKDF 派生密钥并以 AES-GCM 加密令牌。
- `internal/middleware/auth.go` 在鉴权启用时验证本地 JWT；OAuth 登录还需读取持久化会话并在线调用 UserInfo，失败清除会话/Cookie，保护业务 API。
- `internal/mcp/` 使用官方 `github.com/modelcontextprotocol/go-sdk v1.8.0`，提供无状态 Streamable HTTP 和 JSON 响应。`http.go` 处理独立 Token/Origin/CORS，`inputs.go` 定义中文参数 Schema，`server.go` 注册工具并复用 repository/price；SDK 自动生成结构化结果与兼容 JSON 文本，并将业务错误转换为 `isError`。
- `internal/repository/inventory_write.go` 为 MCP 补充带关联校验的创建事务和字段级更新。元件编号、价格分摊、保存及有价初始流水同事务提交；更新限制资料字段，保留未传字段与库存/单价。预入库仅允许编辑 pending，校验失败回滚。REST 与 MCP 统一使用该事务校验，REST PUT 按字段存在性更新。分类与复核字段的新增默认值见下文。
- 入出库与预入库确认在支持的数据库上使用事务行锁；SQLite 沿用事务写锁。库存与金额运算拒绝会造成整数溢出的输入。
- `internal/database/database.go` 按 `DB_DRIVER` 打开 SQLite/MySQL/PostgreSQL 的 GORM 连接；SQLite 会创建数据目录并设置 pragma，所有数据库都会自动迁移 `Category`、`Supplier`、`Component`、`PreStock`、`StockLog`、`Task`、`OAuthIdentity`、`OAuthSession`、`OAuthLogout`。
- `internal/models/models.go` 定义数据库表结构和 JSON 字段，是前后端数据契约的重要来源。
- `internal/router/router.go` 暴露 `/api/v1` API；`/api/v1/auth/*` 为公开路由，其余业务接口在鉴权启用时需登录。静态资源仍从嵌入的 `web/dist` 提供。访问日志只记录路径，不记录查询或请求头；panic 恢复日志不输出请求或 panic 值，避免泄露回调凭据。
- `internal/handlers/` 负责 HTTP 输入输出和状态码。业务实体目前按 `category`、`supplier`、`component`、`stock_log`、`stats`、`parser`、`tasks`、`auth` 拆分。
- `internal/price/price.go` 集中实现单价分摊（`UnitPriceMicro`）、出库成本（`OutboundTotalCents`）、加权平均（`WeightedAverageUnitPriceMicro`）与撤销反算（`ReverseAverageUnitPriceMicro`）；repository 与 handler 应复用此包，避免重复四舍五入逻辑。
- `internal/repository/` 封装数据库访问。新增复杂查询时优先放在 repository，避免 handler 直接堆叠大量查询逻辑。
- `internal/version/` 保存项目版本变量，默认版本为 `v1.0.0`；发布构建通过 Makefile 的 `VERSION` 变量注入 git tag。
- `internal/llm/` 使用标准库实现 OpenAI-compatible `/chat/completions` JSON 响应调用，支持文本消息、图片 data URL 多模态消息，以及 `tools` / `tool_calls` / `tool_call_id` 往返，供解析器按需使用。
- `internal/parser/` 包含 LCSC、淘宝、二维码和照片 AI 解析及解析器管理器。LCSC 料号去首尾空格并转大写，普通与 AI 整理固定按 `lcsc_detail.go` 的 LCSC 详情 API → `easyeda.go` 的 EasyEDA 国际站 → 国内站 → `lcsc_parser.go` 的 jlc-smt HTML 查询。详情校验 HTTP/业务状态、有效结构、精确料号和有效名称。所有商品请求限 30 秒、2 MiB 并接收请求/任务上下文；上下文结束立即终止，客户端自身超时允许回退。全部失败汇总原因，仅包装最终 HTML 错误以保持 HTTP/任务分类。AI 整理取得资料后才调用模型，失败不继续查询。普通解析仅执行确定性词典和映射，来源原文独立保留在 normalization.sources。
- `easyeda.go` 请求 `https://pro.easyeda.com/api/eda/product/search`，失败、无精确匹配或内容无效时请求 `https://pro.lceda.cn/api/eda/product/search`；参数固定 `keyword=<标准化料号>&type=3&page=1&pageSize=10`。兼容国内 `code/model` 与国际 `number/mpn` 返回结构，绝不采用模糊首条结果。只提取商品资料和 `device_info.attributes/description` 等参数，过滤符号/绘图/3D 数据；图片按 `<$>` 拆分或从国际图片数组取有效 URL，PDF 相对地址不补主机。库存兼容字符串/数字；币种仅采用明确字段，不按站点猜测。
- `agent.go` 提供独立 Agent 执行器、工具注册与本次解析缓存，首批 `lcsc_product_detail`、`easyeda_product_search`，模型自主决定顺序并观察结果继续查询。仅支持 C 开头数字料号，参数只允许本次标准化料号；地址/请求头/分页由程序控制。最多 4 轮模型交互、4 次工具调用、60 秒（仍受任务/父请求超时限制）；重复调用复用本次缓存。最终 JSON 必须指定已成功查询的主来源和有效元件资料，程序从该来源绑定原始报价库存。未配置模型、不支持工具调用、无效结果或预算耗尽时降级普通解析，不再 AI 整理；父上下文结束立即终止。没有型号搜索、自然语言查询或库存写入工具。
- `trace.go` 的可选 `parse_trace` 在同步成功/失败响应顶层、编码/二维码任务结果 JSON 中保存：`mode`、`final_source`、`fallback`、`fallback_reason`、`calls`。每条调用包含 `tool`、`arguments`、实际 `endpoint`、`started_at`（RFC3339Nano）、`duration_ms`、`status`、`summary` 和可选 `cached`；不保存模型提示词或密钥。失败任务结果只含诊断，无可应用资料，取消终态仍受条件更新保护。
- `ComponentInfo.supplier_quote` 为可选参考快照：`stock_quantity`（可为 null，0 表示零库存）、`currency`（可为 null）、`price_tiers: [{min_quantity, unit_price}]`（数量升序，单价为原始十进制字符串）、`queried_at`（RFC3339）、可选 `source_name` 与 `source_url`（实际查询地址）。LCSC 报价不写入旧 `price`，供应商库存不写本地数量；AI 不接收或修改报价库存。快照随现有任务结果 JSON 保存，无新增表、迁移或配置项；扫码数量仍来自二维码。未接入型号搜索。

## 前端结构

- `web/src/App.tsx` 定义 SPA 页面路由：`/`、`/components`、`/pre-stocks`、`/categories`、`/logs`、`/tasks`、`/login`。业务页面包裹 `ProtectedRoute` 与 `Layout`；登录页不使用侧边栏。各页面通过 `React.lazy` 按路由懒加载，路由切换时显示 Suspense 加载占位。
- `web/src/context/AuthContext.tsx` 提供 `AuthProvider`，启动时先调用 `GET /auth/config`，鉴权开启再调用 `GET /auth/me`，并维护 `login`、`logout` 和鉴权状态；`context/auth.ts` 定义共享 Context 与类型，`context/useAuth.ts` 提供读取鉴权状态的 hook。为满足 React Fast Refresh 规则，组件文件不导出非组件 hook。认证加载失败通过 `components/AuthError.tsx` 显示错误/重试，不能按免登录处理；登录页按配置展示密码和统一登录入口，OAuth 使用浏览器导航到后端，失败提示后移除错误查询参数。
- `web/src/api/inventory.ts` 定义独立元件草稿/提交类型、显式资料字段映射及保存/库存/价格/CSV 请求；CSV 复用 Axios 的 Cookie 和 401 行为。元件及预入库 PUT 仅更新提供的资料字段；库存、单价、创建时间、关联对象与复核状态由服务端维护，省略保留、显式空字符串清除可选文本。
- `web/src/api/client.ts` 是统一 Axios 客户端，API 前缀固定为 `/api/v1`，`withCredentials: true` 以携带 HttpOnly Cookie；401 时跳转 `/login`（`/auth/config`、`/auth/me` 与 `/auth/login` 除外）。
- `web/src/pages/` 存放业务页面：仪表盘、元件管理、预入库、分类管理、库存日志。供应商当前不设独立页面，在元件/预入库表单内输入/选择并自动创建。库存日志页支持显示总数和切换每页条数。仪表盘（`Dashboard.tsx`）区分全部库存概况与期间统计，并复用库存记录接口展示近期流水及快捷入口；通过 `GET /stats` 展示元件/分类/库存概览、库存总价值，以及按时间范围（本月/本季/全部）筛选的累计入库金额、入库数量、出库数量。
- `components/ui/SearchSelect.tsx` 的必填 `source` 指定分类、启用分类、供应商或历史制造商/封装/位置；所有使用点（含正式库存、预入库、照片草稿和批量位置）打开时重新请求，输入关键词防抖 300ms，禁用 cmdk 本地过滤。旧请求取消且结果与查询绑定，显示加载、错误及重试；传入的 `options` 仅供当前值标签显示。分类录入查询 active，可附带原分类 ID；原关联的保存校验不变。列表搜索仍点击/回车提交，已有后端状态/类型/时间范围筛选保持现有触发方式。
- 搜索回归：`internal/handlers/search_test.go` 使用 SQLite 验证筛选先于计数/分页、旧 API 调用兼容、原分类范围、SQL 参数绑定及历史选项去重；`web/e2e/search.spec.ts` 验证分类分页、下拉远程搜索、失败重试及迟到响应隔离。
- `web/src/pages/Components.tsx` 负责元件查询、CRUD、复制、自动补号、批量位置/出库、价格补录和流水撤销。关键词/分类/高级分字段条件、分页和排序写入 URL，条件支持逐项移除；桌面表格/手机卡片，名称打开详情抽屉，行菜单显示文字。保留点击复制编号/型号/料号、显示列和 CSV 自定义表头/顺序/拖拽。`components/inventory/columns.ts` 管理原偏好结构，`ColumnSettings.tsx` 管理配置 UI；`hamster-components-sort`、`hamster-components-table-columns`、`hamster-components-export-columns` 均保持兼容。采购总价仍按元录入、分提交、微元单价预览。数量快捷值保留 5/10/20/50/100。单次出入库展示当前/变更后数量、单价/金额，提交防重入并拒绝无效数量。
- `web/src/pages/PreStocks.tsx` 负责预入库 CRUD、确认入库和解析录入；pending/confirmed/all 通过状态标签切换，分页与筛选保留在 URL；桌面表格、手机卡片。共用 `InventoryFields` 与 `SmartEntry`，分类按启用 ID 选择，供应商仍可自动创建；保留采购分摊和照片多结果核对。保存生成 HB 编号但不增加正式库存；确认后按原接口生成元件和流水。
- `web/src/components/Layout.tsx` 提供桌面可收起侧栏（保留 `hamster-sidebar-collapsed`）、顶栏/任务状态、手机 Radix 导航抽屉。`ThemeProvider` 默认浅色，`hamster-theme` 保存 light/dark/system，监听系统变化；`index.html` 首屏应用主题。`src/index.css` 用 Tailwind 4 CSS 变量定义浅/深主题；无 Tailwind JS 或 PostCSS 配置。浏览器基线 Safari 16.4+、Chrome 111+、Firefox 128+。扫码与相机继续懒加载，拒绝权限显示重试/上传入口，扫码自动启动只尝试一次，卸载释放资源。
- `web/src/components/ui/` 统一按钮、表单、分页和完整状态。`Modal` 使用 Radix Dialog，可切换居中弹窗/右侧宽抽屉，手机抽屉全屏、底栏固定；`ConfirmProvider` 用 AlertDialog 处理危险操作和放弃修改；菜单用 Radix DropdownMenu，带文字、键盘导航、Esc 与焦点恢复。`SearchSelect` 使用 Popover + cmdk，值/名称分离并支持创建名称。`PageState` 区分加载、空数据、无匹配和失败重试，`SafeImage` 显示失败占位。
- `web/src/types/index.ts` 存放前端共享类型。后端模型字段变化时，应同步检查这里和调用 API 的页面。
- `components/ParseModeSelect.tsx` 提供两表单共用的模式选择；`components/ParseTraceView.tsx` 展示任务调用诊断。`components/SupplierReference.tsx` 供元件/预入库表单和任务详情共用，展示来源名称、实际查询地址、供应商料号、库存、币种、阶梯原始单价及原查询时间，未知值明确标记。`utils/parseResult.ts` 显式挑选业务资料字段，参考快照独立于表单和金额计算；新查询、表单关闭/保存/切换会话清除参考信息，当前料号或查询输入不匹配时隐藏，任务结果继续遵循会话隔离和手动字段修改保护。历史任务应用结果保留原查询时间。
- `web/src/utils/price.ts` 与后端 `internal/price` 对应：总价用分（`yuanToCents`、`formatCents`），单价用微元（`formatMicro`、`calcUnitPriceMicro`、`calcOutboundCostCents`）。修改金额规则时需同步前后端两处。
- `web/src/utils/stockLog.ts` 封装库存流水展示逻辑（撤销/冲销/补录标签、数量样式等），供元件管理页与库存日志页共用。

## 数据模型要点

- `Component` 是核心库存实体，必须关联 `Category`，可选关联 `Supplier`。
- `Component.component_number` 是系统管理的元件编号，全局唯一；数据库字段允许 `NULL` 以兼容历史未编号数据。自动编号格式为 `HB-000001` 递增；创建时留空会自动生成，也可手动输入任意唯一编号。编号生成和唯一性校验同时检查正式元件与预入库记录。
- `PreStock` 是独立预入库实体，必须关联 `Category`，可选关联 `Supplier`。预入库记录先占用 `HB-xxxxxx` 编号但不计入正式库存、库存价值或仪表盘入库统计；确认后创建正式 `Component`、写入库存流水，并将状态从 `pending` 改为 `confirmed`。
- `Component.model` 表示厂家型号，例如 `RC0603FR-0710KL`；与 `name`（商品名称）和 `supplier_part_number`（供应商料号，如 `C2040`）区分。
- `Component.manufacturer` 表示制造商/品牌，例如 `YAGEO`；与 `model`（厂家型号）和 `Supplier`（采购供应商）区分。
- `Supplier` 表示采购来源/供应商，例如“嘉立创”“淘宝”；`Component.supplier_id` 可为空以兼容历史数据。
- `Component.supplier_part_number` 表示供应商料号，例如 `C2040`，不要与供应商名称混用。
- `Component.unit_price_micro` 表示参考单价，单位为微元（1 元 = 1,000,000 微元）；入库或新增元件带价格时按库存加权平均更新（`(原库存×原单价 + 本次总价×10000) / 新库存`，整数除法）；无既有库存或参考单价时直接使用本次入库分摊单价 `round(total_price_cents×10000/quantity)`。
- `StockLog.unit_price_micro` 和 `StockLog.total_price_cents` 分别表示该条库存记录的分摊单价（微元）与录入总价（分，入库）或成本总价（分，出库）；入库时由用户录入总价并按数量分摊单价；出库时若元件有参考单价，则自动按 `round(unit_price_micro×|change_amount|/10000)` 写入成本，无需请求体传价。
- `StockLog.revoked_at` 非空表示该条记录已被撤销；`StockLog.reversal_of_id` 非空表示该条为撤销时自动生成的冲销流水，指向被撤销的原记录 ID。已撤销记录与冲销流水均不可再次撤销。
- 金额约定：总价在接口和数据库中使用整数分（`total_price_cents`）；单价使用整数微元（`unit_price_micro`，1 元 = 1,000,000 微元）；前端总价格式化为元（两位小数），单价格式化为元（最多六位小数）。单条入库分摊规则为 `unit_price_micro = round(total_price_cents×10000/quantity)`；元件参考单价为多次入库的加权平均，撤销入库时会按 `(当前库存×当前单价 - 原记录总价×10000) / 回退后库存` 反算回退。
- 平台解析结果中的 `platform_name` 用于前端推断供应商名称；当前立创/LCSC 导入映射为“嘉立创”，`platform_code` 写入 `supplier_part_number`，`name` 使用商品页名称，`model` 写入厂家型号，`manufacturer` 写入制造商，分类仅按启用白名单精确匹配并返回 `category_id`，未匹配时在表单人工选择；禁止解析或保存时自动建类。
- 元件列表搜索支持分字段 query：`component_number`、`name`、`model`、`manufacturer`、`value`、`supplier`（匹配供应商名称）、`supplier_part_number`；同一字段内按空格拆词，词之间 AND，且均在该字段 LIKE 匹配；多个非空字段之间 AND。`keyword` 仍兼容旧客户端：按空格拆词，每个词需命中编号/名称/厂家型号/制造商/参数/料号/描述/供应商名称任一字段，词之间 AND。修改搜索逻辑时需同步检查 `ComponentRepository.GetAll` 和元件管理页搜索 UI。
- 元件表单保存时会清除前端关联对象，只提交 `category_id`、`supplier_id`、`component_number`、`supplier_part_number`、`manufacturer` 等字段，避免 GORM 更新关联对象。
- 编辑元件时，前端可根据当前 `supplier_part_number` 提交 `component_parse` 后台任务重新解析并回填名称、厂家型号、制造商、参数、封装、描述、数据手册、图片和分类建议；解析结果中空字段不覆盖表单已有值，库存等本地字段保持不变。

## 运行与构建

后端开发运行：

```bash
make run
# 或
go run cmd/server/main.go
```

前端开发运行：

```bash
cd web
pnpm run dev
```

前端构建：

```bash
cd web
pnpm run build
pnpm run test:auth   # 登录方式、认证错误和路由保护的 SSR 渲染测试
```

后端构建：

```bash
make build
# 或
go build -ldflags="-s -w -X github.com/Rehtt/hamster-bin/internal/version.Version=v1.0.0" -trimpath -o hamster-bin cmd/server/main.go
```

后端验证按顺序执行 `go fix ./...`、`go fmt ./...`、`go test . ./cmd/... ./internal/...`。统计查询边界测试使用 `go test ./internal/repository -run 'TestStatsRepository' -v`，覆盖空库存、不足一分、汇总后换算和库存/价格过滤。

`internal/repository/stats_repo_mysql_test.go` 提供可选 MySQL 集成测试，覆盖空库存及有库存时的本月、本季、全部统计。测试专用环境变量 `TEST_MYSQL_DSN` 必须指向已创建、名称以 `_test` 结尾的隔离数据库；连接账号须有创建临时表的权限。测试固定单连接，仅创建会话临时表，关闭连接后自动清理，不运行应用迁移；未设置 DSN 时明确跳过。这不是运行时配置项。

```bash
TEST_MYSQL_DSN='user:password@tcp(127.0.0.1:3306)/hamster_bin_test?charset=utf8mb4&parseTime=True&loc=Local' go test ./internal/repository -run '^TestStatsRepositoryMySQL$' -count=1 -v
# 保留工作区已有二进制时，将构建输出写入临时目录
build_dir=$(mktemp -d)
make build BUILD_DIR="$build_dir"
```

版本查看：

```bash
./hamster-bin --version
```

单文件部署流程：

```bash
cd web
pnpm run build
cd ..
make build
```

前端构建产物必须位于 `web/dist`，因为 `embed.go` 和 `internal/router/router.go` 依赖该路径提供嵌入式静态资源。

Docker 镜像通过根目录 `Dockerfile` 多阶段构建：前端基于 `node:24-bookworm-slim`，通过 `corepack enable && corepack prepare pnpm@11.3.0 --activate` 启用 pnpm，使用 `pnpm install --frozen-lockfile` 安装依赖，再执行 `pnpm build` 构建 `web/dist`；`golang:1.25-alpine3.23` 以 `CGO_ENABLED=0` 编译并注入 `VERSION`，运行时镜像为 `alpine:3.23`。容器默认监听 `8080`，数据目录 `/app/data`（`DB_DRIVER=sqlite`、`DB_PATH=/app/data/inventory.db`，`IMAGE_DIR=/app/data/images`），需挂载 volume 持久化 SQLite 与图片；连接外部 MySQL/PostgreSQL 时通过 `DB_DRIVER` 与 `DB_DSN` 覆盖。

Docker Compose 部署（`docker-compose.yml`）：

```bash
cp .env.example .env   # 可选
docker compose up -d --build
docker compose logs -f
docker compose down
```

默认从源码 `build` 镜像；切换 GHCR 预构建镜像时注释 `build` 段并启用 `image: ghcr.io/rehtt/hamster-bin:latest`。全部运行时环境变量见 `.env.example`，通过复制为 `.env` 后由 compose 注入容器。

发布构建由 GitHub Actions 在推送 `v*` git tag 时触发，例如 `v1.0.0`。`build-and-release` job 使用 `pnpm install --frozen-lockfile` 安装前端依赖，先构建 `web/dist`，再运行 `go test . ./cmd/... ./internal/...`，最后执行 `make build-all VERSION=${GITHUB_REF_NAME}` 并以同名 tag 创建 GitHub Release。并行的 `docker` job 使用 buildx 构建 `linux/amd64` 与 `linux/arm64` 镜像，推送到 `ghcr.io/rehtt/hamster-bin`，tag 为 git tag 与 `latest`。

## API 与配置约束

- 远程 MCP 入口为 `/mcp`，不属于 `/api/v1`。`MCP_TOKEN` 默认为空，此时所有方法返回 404；配置后使用 `Authorization: Bearer <token>`，对固定长度 Token 摘要做恒定时间比较，与 Web Cookie/JWT 独立。单个部署级 Token 拥有全部 MCP 工具权限，修改环境变量并重启轮换。
- `MCP_ALLOWED_ORIGINS` 为逗号分隔的完整 Origin 列表，空值默认仅同源或无 Origin；不支持通配符或路径。`/mcp` 跳过 REST 全局 CORS，先做独立 Origin 校验再处理预检，非法 Origin 返回 403，缺失/错误 Token 返回 401。代理头不参与同源判断；反向代理终止 HTTPS 后的外部浏览器 Origin 需配置白名单，回环上游 Host 应使用本机地址以兼容 SDK 的 DNS 重绑定保护。
- MCP 工具共 21 个：`normalization_standards`、`normalization_preview`；`list_components`、`get_component`、`create_component`、`update_component`、`change_stock`；`list_pre_stocks`、`get_pre_stock`、`create_pre_stock`、`update_pre_stock`、`confirm_pre_stock`；`list_categories`、`create_category`、`update_category`、`delete_category`、`list_suppliers`、`create_supplier`；`list_stock_logs`、`get_component_stock_logs`、`get_stats`。仅开放空分类删除，不开放其他删除、撤销、批量、平台解析和后台任务。
- MCP 分页默认 page=1、page_size=20，上限 100；单元件流水 limit 默认 50，上限 100。预入库状态默认 pending（可选 confirmed/all），统计默认 month（可选 quarter/all）。查询复用 REST 的搜索、分类和排序规则。
- MCP 成功响应保持 `{data, pagination?}` 结构；`result.Data` 保留 `any`，必须带中文 `jsonschema` 描述，使 SDK 输出仅含 `description` 的对象 Schema，避免生成 `"data": true` 导致 Hermes 添加服务时校验失败。不要通过限定 data 类型缩小对象、数组等返回值范围；`additionalProperties: false` 仍是合法约束。
- MCP 创建必填名称、分类 ID 和非负数量（元件 stock_quantity、预入库 expected_quantity）；供应商使用已有 ID，同名创建供应商返回已有记录。总价为整数分，单价为微元；数量为零时不允许正数总价。元件编辑不接受库存/参考单价；局部编辑省略字段保持原值、空字符串清除文本、supplier_id=0 解除关联。`change_stock.amount` 正入负出且不得为零，入库总价非负，出库成本自动计算，复用 `ApplyStockChange`；预入库确认复用 `Confirm`。
- MCP `update_category` 必填正数 `id`，可选 `name`、`parent_id`、`status`；仅传 `id` 返回原记录。省略或 null 保留原值，`parent_id=0` 移为顶级分类，成功 `data` 为更新后的分类。`delete_category` 必填正数 `id`，成功 `data` 为 `{id, message: "删除成功"}`。两个工具均标记写入及破坏性操作，错误使用 `isError`。
- `internal/repository/category_repo.go` 统一分类创建、字段级修改与删除约束。名称去首尾空白后不能为空，父分类必须存在且不能是自身或后代；父链遍历检测历史循环。更新使用明确字段，不使用 Save，目标不存在不会新建。`parent_id=0` 写为 NULL；新增 status 默认为 pending，改名重新待审核。删除在同一事务中行锁读取目标、检查关联、硬删除；关联元件（含零库存）、任意状态预入库或子分类时拒绝，提示具体关联类型，不迁移、不级联。
- REST 分类 POST/PUT 复用上述名称及层级校验；PUT 保留部分更新能力，省略或 null 不修改父分类。`DELETE /api/v1/categories/:id` 路径和成功 `{message: "删除成功"}` 不变，无效/零 ID 返回 400，不存在返回 404，关联冲突返回 409，数据库异常返回 500。分类网页删除确认列出三类约束，并展示服务器具体错误；无父分类编辑入口。
- MCP 集成测试位于 `internal/mcp/mcp_test.go`，通过官方 SDK 客户端覆盖发现、新旧版本握手、21 个工具、Schema/文本兼容、认证/Origin、写入与失败回滚。`TestSDKWorkflow` 递归检查全部工具的输入和输出 Schema，要求 `properties` 下各字段为对象，并确认 data Schema 仅含描述；复用业务调用覆盖对象、数组、文本回退和错误结果，分页由 `TestLegacyHandshakeAndPagination` 覆盖。分类测试另见 `internal/repository/category_repo_test.go` 与 `internal/handlers/category_handler_test.go`，覆盖关联阻止、失败数据不变、层级循环、部分更新及 HTTP 状态码。验证使用 `go fix ./...` → `go fmt ./...` → `go test . ./cmd/... ./internal/...` → `go test -race ./internal/tasks ./internal/repository ./internal/mcp`；前端 lint/typecheck/build 后用临时 BUILD_DIR 执行 `make build`。
- 后端 API 基础路径是 `/api/v1`，前端 Axios 客户端也使用该路径。修改 API 前缀时必须同步修改前后端。
- 主要 API 分组：
  - `/api/v1/auth/config`（GET，公开；`data` 含 `auth_enabled`、`password_enabled`、`oauth_enabled`，不暴露密钥）
  - `/api/v1/auth/oauth/login`、`/api/v1/auth/oauth/callback`（GET，公开；OAuth 未启用返回 404）
  - `/api/v1/auth/oauth/backchannel-logout`（POST，公开；签名令牌认证，无需 Cookie；OAuth 未启用返回 404）
  - `/api/v1/auth/login`（POST，公开；仅密码登录已启用时可用）
  - `/api/v1/auth/logout`（POST，公开）
  - `/api/v1/auth/me`（GET，公开；鉴权关闭返回 `{ auth_enabled: false }`，已登录返回 `{ auth_enabled: true, username, auth_method }`，未登录返回 401）
  - `/api/v1/categories`
  - `/api/v1/suppliers`
  - `/api/v1/components`
  - `/api/v1/pre-stocks`
  - `/api/v1/components/options`
  - `/api/v1/components/export`
  - `/api/v1/components/batch-location`
  - `/api/v1/components/batch-stock-out`
  - `/api/v1/components/batch`
  - `/api/v1/components/generate-numbers`
  - `/api/v1/components/:id/stock`
  - `/api/v1/components/:id/backfill-price`
  - `/api/v1/components/:id/logs`
  - `/api/v1/components/:id/image`
  - `/api/v1/components/parse`
  - `/api/v1/components/parse-qrcode`
  - `/api/v1/pre-stocks/batch`
  - `/api/v1/stock-logs`
  - `/api/v1/stock-logs/:id/revoke`
  - `/api/v1/stats`
  - `/api/v1/tasks`（POST 入队、GET 分页筛选），`/image`（照片 multipart 入队），`/:id`（GET），`/:id/image`（GET 原图），`/:id/cancel`、`/:id/retry`（POST）
  - `/api/v1/platforms`
- 默认数据库类型是 `sqlite`，由 `DB_DRIVER` 覆盖；支持 `sqlite`、`mysql`、`postgres`（`postgresql` 会按 `postgres` 处理）。
- `DB_DSN` 是数据库连接串：MySQL/PostgreSQL 必填；SQLite 可选，设置后优先于 `DB_PATH`。
- 默认 SQLite 数据库路径是 `./data/inventory.db`，由 `DB_PATH` 覆盖。
- 默认图片目录是 `./data/images`，由 `IMAGE_DIR` 覆盖。
- 默认端口是 `8080`，由 `PORT` 覆盖。
- 同时设置 `SSL_CERT` 和 `SSL_KEY` 时服务使用 HTTPS。Cookie 在直接 TLS 或 OAuth 外部回调为 HTTPS 时使用 Secure，均为 HttpOnly、SameSite=Lax，不信任任意代理头推断协议。
- 鉴权：完整管理员凭据启用密码登录；`OAUTH_ENABLED=true` 启用统一登录，支持并存或仅 OAuth；两者均关闭时免登录。`JWT_SECRET` 为签名和令牌加密的根密钥（启用鉴权必填），轮换后原会话失效；`JWT_EXPIRE_HOURS` 默认 `168`（7 天）。旧版密码 JWT 只在管理员登录仍启用且用户名匹配时接受。
- LLM 辅助解析使用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL` 配置。三项均非空时才可用，`LLM_BASE_URL` 应指向 OpenAI-compatible API base，例如 `https://api.openai.com/v1`，实际请求路径为 `{LLM_BASE_URL}/chat/completions`。
- `POST /api/v1/components/parse` 请求体为 `{ "code": "C2040", "parse_mode": "standard" }`；可选 `parse_mode` 支持 `standard` / `llm` / `agent`，明确提供时优先于旧 `use_llm`；省略时 `use_llm:true` 推导为 `llm`，否则 `standard`。未知模式返回 400。模式仅影响嘉立创/LCSC 解析。解析响应追加可选 `category_id` 和 `normalization`；未匹配启用分类时省略 ID、清空可应用分类名称，来源名称保留在来源快照。可预期解析失败不会统一返回 500：`400` 表示编码格式无效或启用 AI 解析但 LLM 未配置，`422` 表示上游页面已获取但内容无法解析，`502` 表示上游 LCSC 请求失败，`503` 表示无可用解析器。
- `POST /api/v1/components/parse-qrcode` 请求体为 `{ "qrcode_data": "...", "parse_mode": "agent" }`；提取料号/数量后复用管理器，`parse_mode` 与旧 `use_llm` 的兼容行为同 `/components/parse`；元件编码解析阶段的错误语义与 `/components/parse` 相同。
- `PATCH /api/v1/components/batch-location` 请求体为 `{ "ids": [1, 2, 3], "location": "A1-03" }`，用于批量更新选中元件的 `location` 字段；`ids` 必填且至少 1 项，`location` 可为空字符串。
- `POST /api/v1/components/batch-stock-out` 请求体为 `{ "reason": "项目A", "items": [{ "component_id": 1, "quantity": 5 }] }`，用于批量出库；`items` 必填且至少 1 项，每项 `quantity > 0`，`component_id` 不可重复。服务端在单事务中预校验全部元件存在且库存足够，任一失败则整批回滚并返回 `400` 与 `failures` 数组（含 `component_id`、`component_name`、`stock_quantity`、`requested`、`error`）。成功时写入各元件负向库存流水（出库成本规则同 `POST /components/:id/stock`），响应 `data` 含 `updated`、`total_quantity`、`total_cost_cents`。
- 所有业务搜索及筛选均查询数据库。`GET /api/v1/categories` 支持 `keyword`（名称按空格拆词 AND 的 SQL LIKE）、`status`、`keep_id`（状态范围内额外允许编辑记录原分类，但仍须匹配关键词）。提供 `page` 或 `page_size` 时在数据库内计数、分页，默认 1/20，每页 1–100，非法分页返回 400，响应增加与元件列表同结构的 `pagination`；不提供分页时保持 `{data:[...]}` 全量返回。分类管理页面使用后端筛选及分页，URL 保留条件。
- `GET /api/v1/components/options` 支持可选 `keyword`（空格拆词 AND 的 SQL LIKE）及 `field=packages|locations|manufacturers`（其他值返回 400），只查询并返回指定字段；不传参数仍返回全部三组历史选项；响应示例 `{ "data": { "packages": ["0603", "0805"], "locations": ["A1-03", "B2-01"], "manufacturers": ["Espressif", "YAGEO"] } }`，`packages`、`locations`、`manufacturers` 分别从已有元件的 `package`、`location`、`manufacturer` 字段去重提取（非空、按名称排序）。表单及搜索区供应商下拉使用 `GET /api/v1/suppliers?keyword=...`，名称在数据库内按空格拆词 AND 的 SQL LIKE 查询；不传关键词保持返回全部供应商。
- `GET /api/v1/components` 支持分页与筛选。常用 query：`page`、`page_size`、`category_id`，以及分字段搜索 `component_number`、`name`、`model`、`manufacturer`、`value`、`supplier`、`supplier_part_number`（语义见上文「元件列表搜索」）。可选排序 query：`sort_by`（白名单字段名，默认 `updated_at`）、`sort_order`（`asc` 或 `desc`，默认 `desc`）；可排序字段与 CSV 导出字段一致。`keyword` 仍兼容 `web_legacy`，React 前端不再使用。
- `GET /api/v1/components/export` 按当前筛选条件导出全部匹配元件为 CSV 文件。必填 query：`columns`（逗号分隔字段名，如 `component_number,name,model`）；可选 query：`headers`（逗号分隔自定义表头，数量需与 `columns` 一致）。筛选与排序 query 与 `GET /api/v1/components` 相同（不含分页），含 `sort_by`、`sort_order`。支持字段：`component_number`、`name`、`model`、`manufacturer`、`value`、`package`、`description`、`category`、`stock_quantity`、`unit_price`（元，最多六位小数）、`location`、`supplier`、`supplier_part_number`、`datasheet_url`、`created_at`、`updated_at`。响应 `Content-Type` 为 `text/csv; charset=utf-8`，带 UTF-8 BOM，文件名形如 `components_YYYYMMDD.csv`。
- `PATCH /api/v1/components/generate-numbers` 无请求体，用于为数据库中所有 `component_number` 为空的元件按 `id` 顺序自动生成 `HB-xxxxxx` 编号；响应示例 `{ "message": "自动编号完成", "updated": 12 }`。
- `GET /api/v1/pre-stocks` 获取预入库记录，支持 `page`、`page_size`、`status`（`pending` | `confirmed` | `all`，默认 `pending`），响应包含 `data` 与 `pagination`。
- `POST /api/v1/pre-stocks` 创建预入库记录；请求体字段与元件信息类似，使用 `expected_quantity` 表示预计入库数量、`total_price_cents` 表示采购总价（分）。`component_number` 留空时自动生成 `HB-xxxxxx` 编号。
- `PUT /api/v1/pre-stocks/:id` 更新待入库记录；已确认记录不可更新。
- `POST /api/v1/pre-stocks/:id/confirm` 确认预入库，服务端在事务中创建正式元件、按 `expected_quantity` 设置库存、按 `total_price_cents` 计算参考单价并写入 reason 为「预入库确认」的 `StockLog`，然后标记预入库 `status=confirmed`、记录 `component_id` 与 `confirmed_at`。已确认记录不可重复确认。
- `DELETE /api/v1/pre-stocks/:id` 删除待入库记录；已确认记录不可删除。
- `POST /api/v1/components` 创建元件时可额外传 `total_price_cents`（分）。当 `stock_quantity > 0` 且 `total_price_cents > 0` 时，服务端计算分摊单价写入 `unit_price_micro`，并自动创建一条 reason 为「初始入库」的 `StockLog`。
- `PUT /api/v1/components/:id` 更新元件字段；请求体与创建相同，可传元件各字段。`unit_price_micro` 不可通过此接口修改（服务端保留原值）。
- `POST /api/v1/components/:id/backfill-price` 补录价格；请求体为 `{ "total_price_cents": 1234, "quantity": 100 }`，`total_price_cents` 与 `quantity` 均须大于 0。按采购数量分摊本批单价；无参考单价时直接设为 `round(total_price_cents×10000/quantity)`，已有参考单价时按当前库存与本次采购数量加权平均更新 `unit_price_micro`（不改库存）。写入一条 `change_amount=0`、reason 形如「补录价格（采购 N 件）」的 `StockLog`。前端入口为元件列表行操作菜单「补录价格」，不在编辑表单中补录。
- `POST /api/v1/components/:id/stock` 请求体为 `{ "amount": 10, "reason": "采购", "total_price_cents": 1234 }`；`amount` 正数为入库、负数为出库。入库且 `total_price_cents > 0` 时写入分摊单价与总价到流水，并按加权平均更新元件 `unit_price_micro`；出库无需传价，若元件有参考单价则自动写入出库成本到流水。库存更新与流水写入在同一事务中完成。
- `POST /api/v1/stock-logs/:id/revoke` 无请求体，用于撤销指定库存记录。服务端在事务中标记原记录 `revoked_at`、回滚库存并写入一条反向冲销流水（`reversal_of_id` 指向原记录）；撤销入库且原记录有总价时会反算回退元件 `unit_price_micro`。撤销入库时若当前库存不足则返回 `400`；已撤销记录或冲销流水再次撤销亦返回 `400`。成功响应示例 `{ "data": { "original": { ... }, "reversal": { ... } } }`。
- `GET /api/v1/stats` 返回仪表盘聚合统计。可选 query：`range`（`month` | `quarter` | `all`，默认 `month`）。响应 `data` 含：`range`、`range_start` / `range_end`（`all` 时 `range_start` 为 null）、`component_count`、`category_count`、`total_stock`、`inventory_value_cents`（仅统计 `stock_quantity > 0` 且 `unit_price_micro > 0` 的元件，SQL 汇总 `SUM(stock_quantity×unit_price_micro)` 微元总额，空库存取 0；Go 使用 `price.MicroPerCent` 整数除法换算为分，先汇总再舍去不足一分的部分，避免 MySQL SQL 除法产生小数而无法扫描到整数）、`inbound_quantity`、`outbound_quantity`、`inbound_cost_cents`（后三项按 `range` 过滤 `stock_logs.created_at`，且排除 `revoked_at` 非空、`reversal_of_id` 非空及 `change_amount=0` 的补录价格记录；入库数量与金额为 `change_amount > 0`，出库数量为 `change_amount < 0` 的绝对值之和）。
- 前端全局库存记录页（`/logs`）与元件管理页的库存记录弹窗均支持撤销操作；已撤销记录显示「已撤销」标签并降低透明度，冲销流水显示「撤销冲销」标签。

## OAuth 身份与会话约束

- `OAUTH_ENABLED` 默认 `false`；Issuer 默认 `https://oauth.home.rehtt.com`，必须与发现文档完全一致且末尾无 `/`；客户端 ID、密钥、回调地址启用时必填；认证方式默认 `client_secret_basic`，可选 `client_secret_post`，禁止自动探测/重试认证方式。生产 HTTPS，仅本地回环允许 HTTP。
- 回调固定 `/api/v1/auth/oauth/callback`，配置为浏览器访问站点的完整地址，不带查询/片段，统一登录侧须精确登记。开发可使用 `http://127.0.0.1:5173` 经 Vite `/api` 代理；不额外增加前端回调页。发现文档按需读取并缓存，HTTP 客户端超时 10 秒、禁止跟随重定向，不在启动时依赖提供方在线。
- 固定申请 `openid profile email offline_access`；管理员须允许这些 scope 并分配用户。已分配用户共享现有全部库存权限，无用户管理页面、角色或数据隔离。邮箱未验证，不可据其合并账号。
- `oauth_identities` 记录 `(iss, sub)`、展示资料；唯一键为 JSON 编码二元组的 SHA-256 摘要，避免数据库排序规则合并不同标识。`oauth_sessions` 保存身份引用、客户端 ID、会话 ID 摘要、已验证 ID Token 的 `provider_sid`、`target_key`、加密令牌、访问/本地过期时间和刷新中标记；`target_key` 是 JSON 编码 `(iss, client_id, sub, sid)` 四元组的 SHA-256 摘要并建索引。令牌字段不序列化；加密密钥由 JWT_SECRET 经 HKDF 独立派生，AES-GCM 绑定会话摘要；提供方令牌不出现在浏览器、日志或业务响应中。
- 本地 OAuth JWT 保存 `auth_method:oauth` 和随机 `session_id` 引用。OAuth 会话按 `JWT_EXPIRE_HOURS` 固定到期，最多 30 天，不随刷新续期；密码会话继续使用原 JWT 机制。正常重启恢复会话；启动清理过期、刷新中及缺少提供方 sid/目标索引的旧记录，升级后旧 OAuth 会话须重新登录。登录时也清理过期/旧记录。
- 授权流程用绑定浏览器的 HttpOnly Cookie `hamster_oauth_flow`（Path `/api/v1/auth/oauth`）关联服务端 state/nonce/S256 verifier；10 分钟有效，最多 1000 条，过期惰性清理。同浏览器重新发起会替换旧流程；回调先消费记录，再拒绝缺失/重复/错误/重放参数。重启后未完成流程重新登录。
- ID Token 固定 RS256，通过发现的 JWKS 验签并检查 iss/aud/azp/exp/iat/sub/nonce/sid，再核对 UserInfo sub；未知 kid 由库刷新公钥缓存。登录和刷新均拒绝 `typ=logout+jwt` 或含退出事件的令牌；初次登录缺少 sid 不建会话，刷新返回 ID Token 时必须与原 sid 一致。最终落库前再次检查 ID Token 未过期。回调设置 no-store/no-referrer，成功跳转 `/`，失败仅携带固定 oauth_error 标识到 `/login`。
- OAuth 每个受保护请求和 `/auth/me` 都调用 UserInfo，无有效结果缓存；检查失败（含网络异常）清除本地会话并返回 401。访问令牌剩余不足 30 秒时刷新，同一会话的检查/刷新/应用内退出串行化。网络交换前提交 refreshing 标记，成功条件更新令牌；已删除会话不能由刷新重建，在线检查结束后重新确认本地会话仍存在。超时、保存失败、重启中断均失效，不能重试旧刷新令牌或授权码。
- 管理员在 OAuth 应用登记 `https://<库存域名>/api/v1/auth/oauth/backchannel-logout`，该地址必须能从 OAuth 服务直接访问。通知仅接收表单请求体内单个非空 `logout_token`，上限 64 KiB，拒绝 JSON、查询串令牌和重复令牌参数；所有响应设 `Cache-Control: no-store`，不读取或设置浏览器 Cookie。有效、重复或未知目标返回 200，非法请求/令牌返回 400，发现文档/JWKS/数据库暂时故障返回 503 供重试。
- 退出令牌复用 go-oidc `VerifyLogout`，每次通知从可信发现文档的 `jwks_uri` 取公钥，匹配非空 kid，限定 RS256 和 `typ=logout+jwt`；不使用令牌内的公钥地址。检查 iss/aud、非空 sub/sid/jti、数值 iat/exp；iat 最多未来 1 分钟/过去 3 分钟，exp 尚有效且晚于 iat；退出事件必须为 JSON 对象，禁止任何值的 nonce。
- `oauth_logouts` 按 `target_key` 持久化结束状态，保留 31 天，启动、登录和退出写入时清理过期记录；结束记录与目标下全部本地会话的删除在同一事务提交。登录最终落库与通知事务使用同一目标锁串行执行，拒绝已结束目标的迟到回调；幂等以目标为单位，不依赖每次重试可能变化的 jti。不会退化为按账号关闭其他设备。
- 后端通知只删除本地会话和令牌，不额外撤销离线授权；页面下次业务请求 401 后由现有客户端跳转登录，不增加轮询。应用内退出仍先清除本地记录和 Cookie，再尽力撤销最新刷新令牌；切换登录方式也清理原 OAuth 会话。密码与 MCP 不参与同步退出或 OAuth 在线检查。本次不实现应用主动退出统一登录；当前仅支持单实例，多实例需重新设计分布式锁及短期流程存储。
- 测试：`internal/auth/oauth_test.go` 与 `oauth_logout_test.go` 使用本地 OIDC/JWKS 和临时 SQLite，覆盖校验失败、撤权、轮换、通知幂等/目标隔离、并发、事务回滚、重启和加密；`internal/database/oauth_test.go` 覆盖旧表升级失效；`internal/router/auth_test.go` 覆盖完整 HTTP 登录/退出通知、Cookie、状态码、兼容和日志过滤。新增认证代码先执行 `go fix ./...`、`go fmt ./...`，再执行全量测试及 `go test -race ./internal/auth ./internal/database ./internal/handlers ./internal/middleware ./internal/repository ./internal/router`；前端在 `web/` 执行 `pnpm run lint`、`pnpm run test:auth` 和 `pnpm run build`，最后 `make build`。

## 修改约束

- 修改数据模型时，必须检查 GORM tag、JSON 字段、前端类型、API 页面调用和数据库迁移影响。
- 修改路由时，必须同步检查 `web/src/api/client.ts` 以及所有页面里的 API 调用。
- 修改前端构建目录、Vite 输出或静态资源路径时，必须同步检查 `embed.go` 和 `internal/router/router.go`。
- 修改 parser 行为时，必须检查 `cmd/server/main.go` 中的注册逻辑和 `/api/v1/platforms` 返回结果。
- 新增后端业务实体时，优先保持现有分层：`models`、`repository`、`handlers`、`router`。
- 新增前端通用 UI 时，优先放在 `web/src/components/ui/` 并复用现有样式工具。
- 不要把运行时数据、构建产物或本地数据库提交为源码。常见运行时路径包括 `data/`、`web/dist/` 和生成的二进制文件。
- 每次修改代码后，根据 `AGENTS.md` 检查并更新本文档。

## 持久化后台任务

- `internal/models/task.go` 定义 `Task` 和跨数据库 `JSONText`（数据库 `text`，API JSON 对象）。`tasks` 表保存类型、原输入、来源页面、可选编辑记录 ID、状态、结果、错误及分类、重试来源 ID 和创建/开始/结束时间；不保存模型配置或密钥。任务不会直接修改元件或预入库。
- `internal/repository/task_repo.go` 负责分页筛选、按创建时间/ID 领取、条件终态更新、中断恢复和未执行任务回退。状态为 `queued → running → succeeded/failed/cancelled`；排队也可直接取消。领取、回退和终态更新均有状态条件，取消后成功结果不能覆盖终态。
- `internal/tasks/service.go` 提供通用队列和首批 `component_parse`、`qrcode_parse` 执行器，复用 `ParserManager`。入队前只验证类型、来源、编码/二维码和必要模型配置，网络抓取和 AI 请求由 worker 执行。取消先写数据库，再取消执行上下文；panic 转失败，worker 继续工作。重试仅允许失败/取消任务，复制输入创建新记录并保存 `retry_of_id`。
- `internal/repository/task_delete.go` 在事务中读取候选并硬删除终态任务，删除 SQL 再次限制状态，每批最多 400 个 ID；数据库错误全部回滚。仅 `succeeded`、`failed`、`cancelled` 可删除，`queued`、`running` 保留。`retry_of_id` 仅作历史编号，删除原任务不级联重试记录，不影响已有元件/预入库；迟到执行结果只条件更新，不重建任务。无数据库迁移和新增配置。
- `internal/tasks/delete.go` 提供删除及提交后的原图清理；独立生命周期锁串行化提交、照片保存、重试、删除与引用检查，内部提交方法避免重复加锁，网络解析不持锁。从剩余照片任务的输入解析引用，不使用数据库专用 JSON 查询；仅最后引用消失时删除私有原图，先校验 key，文件已不存在视为成功。引用无法确认或文件删除失败时保留原图、记录日志并计入清理失败数；数据库回滚时不清理照片。
- 删除 API 受现有鉴权保护：`DELETE /api/v1/tasks/:id` 不存在返回 404、未结束返回 409；`POST /api/v1/tasks/batch-delete` 接收 `{mode, ids?}`，mode 为 `selected`、`failed`、`all`、`older_than_7_days`。仅 selected 必须提供 1–100 个有效正整数 ID（去重后处理），其他模式禁止 ids，非法参数返回 400。failed 仅匹配 `failed`；older_than_7_days 严格按创建时间早于服务器当前时间减去 168 小时匹配。批量跳过未结束任务及不存在的勾选 ID，零匹配也成功；返回 `200 {data:{deleted_ids,deleted_count,skipped_count,image_cleanup_failed_count}}`，照片清理失败不撤销已提交删除。
- 单独的 dispatcher 协程负责领取队列并通过容量等于 `TASK_WORKERS` 的 `work` channel 分发给 worker；入队广播会立即唤醒 dispatcher，数据库每 30 秒兜底扫描一次，领取失败同样等待通知或兜底重试。dispatcher 停止领取后关闭 work；已领取但尚未开始执行的任务恢复为 `queued`，运行中的任务记录为中断。
- `internal/tasks/images.go` 提供 `image_parse` 任务的 JPEG/PNG 校验与 `IMAGE_DIR/task-inputs` 私有照片存储；任务输入只保存照片引用，不把 Base64 写入数据库。照片任务复用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`，模型需支持图片输入，结果为可人工核对的 `items` 数组。
- `TASK_WORKERS` 默认 `2`，`TASK_TIMEOUT_SECONDS` 默认 `120`，从领取开始执行计时；非正整数或无效值使用默认值。抓取仍限 30 秒，普通 AI 整理/照片模型请求仍限 90 秒，Agent 总预算 60 秒，二者同时接收任务上下文以终止实际 HTTP 请求。
- `cmd/server/main.go` 启动 worker 前把遗留 `running` 标记失败（`interrupted`，服务中断），保留 `queued`。SIGINT/SIGTERM 时停止领取并取消执行，记录运行任务中断，再关闭 HTTP 服务。仅支持单服务实例；不引入 Redis，不自动重试，仅提供手动删除历史任务，不增加定时清理。
- 所有 `/api/v1/tasks` 接口使用现有鉴权。POST 请求：`{"type":"component_parse","input":{"code":"C2040","parse_mode":"standard"},"source":"components"}`（可选 `record_id`）；二维码任务使用 `type:"qrcode_parse"` 与 `input.qrcode_data`；照片使用 `POST /api/v1/tasks/image` multipart 字段 `image`、`source`、可选 `record_id`，返回 `202 {data:Task}`。照片限制 JPEG/PNG、10 MiB、4000 万像素，结果为 `items`。来源可省略或为 `components`/`pre-stocks`，编辑记录 ID 需有有效来源。GET 列表支持 `status`、`type`、`page`、`page_size`（1–100），返回 `{data,pagination}`；非法状态操作返回 409。详情含原输入、可展示错误及编码/二维码调用记录；历史无 `parse_mode` / `parse_trace` / 来源字段的任务仍可恢复与重试。
- `POST /api/v1/components/batch` 与 `POST /api/v1/pre-stocks/batch` 接收 `{ "items": [...] }`，最多 100 条；每条需要名称、分类和显式填写的非负整数数量（元件 `stock_quantity`、预入库 `expected_quantity`）；小数等类型错误返回 400；缺失、`null`、负数或规范问题返回 422，明确填写的 0 合法。分类必须提供当前启用 ID，供应商仍可按名称匹配或创建。成功返回 `201 {data:[...]}`，顺序与请求条目一致。整个批量操作使用单事务并复用底层 repository 校验，任一条失败连同供应商和库存流水全部回滚；元件入库价格和库存流水遵循普通创建规则。错误包含条目序号及字段问题。
- 旧 `/components/parse` 和 `/components/parse-qrcode` 继续同步返回原有结构，复用相同解析器并传递 HTTP 请求上下文。
- `web/src/context/TaskContext.tsx` 在受保护布局内维护共享任务状态，每 2 秒轮询跟踪中的任务。会话存储 `hamster-pending-tasks` 保存未完成 ID；刷新时分页恢复数据库排队/运行任务，并查询原跟踪 ID，历史完成任务不重复通知。退出登录卸载 Provider，终止轮询。
- `/tasks`（`pages/Tasks.tsx`）是任务中心，提供 URL 分页、状态/类型筛选、可读结果/错误详情及折叠 JSON/调用诊断，清理操作收进菜单；保留输入详情、取消、重试、应用结果和删除。列表/详情可单条删除；列表支持本页全选、勾选删除以及删除失败任务、全部任务、7 天以上任务三个全库快捷操作（不受分页/筛选影响）。仅终态任务可勾选，翻页/筛选清空选择，轮询保留当前页有效选择；所有删除使用确认框，操作期间禁止重复提交及冲突操作，成功后按实际删除 ID 清理共享缓存/会话跟踪、刷新总数并回退越界页，删除当前详情后关闭弹窗并移除 task 查询参数，详情 404 同样关闭提示。请求失败保留有效勾选。`utils/taskDeletion.ts` 集中选择裁剪、页码计算、确认文本与响应版本/删除标记，阻止旧轮询恢复已删除任务。元件和预入库表单的“拍照 AI”统一采用“核对结果 → 复用业务表单逐条编辑 → 一次批量保存”。核对时新增可多选，编辑已有记录使用单选并可直接切换任意结果；未知数量必须手动补齐，显式 0 会保留。侧边栏有入口。SPA fallback 直接返回嵌入的 `web/dist/index.html`，刷新任务/恢复链接不会丢失路径和查询参数；构建产物不提交。
- `hooks/usePhotoDrafts.ts` 保存独立的完整表单草稿，`components/PhotoDraftNavigation.tsx` 提供条目切换与进度提示；切换和提交前合并当前编辑值，包含分类、供应商、数量、价格、位置、编号和图片。校验失败定位条目，请求失败保留全部草稿并恢复保存状态；元件创建成功后按响应顺序上传各条图片，图片失败提示对应条目但不撤销已保存元件。预入库图片沿用 URL 字段。`utils/photoResult.ts` 显式转换照片字段，正确映射供应商料号；`types/index.ts` 区分原始 `ParsedImageItem`（数量可为 `null`）与已核对 `ReviewedImageItem`（数量为整数），`TaskResult.component` 为可选。
- `components/PhotoParseModal.tsx` 每次打开/切换目标创建独立会话；提交/解析期间可关闭，确认或关闭会卸载弹窗并释放摄像头，重新打开默认显示拍摄/选图入口。共享任务上下文继续跟踪后台任务，会话标识阻止迟到响应或旧任务回填新弹窗。
- `hooks/useParseTask.ts` 隔离表单会话，`utils/parseResult.ts` 按提交快照逐字段合并非空解析值；手动修改保留。关闭、保存、重新打开、复制表单或切换照片草稿使旧编码/二维码任务失去自动回填资格，任务仍继续。`components/ParseTaskStatus.tsx` 显示任务编号和状态，解析不阻塞编辑/关闭/保存，仅禁用同一会话内重复提交。
- `hooks/useTaskApplication.ts` 消费业务页 `?applyTask=<id>` 恢复链接；先取任务，再重新读取编辑记录；照片任务先进入同一核对弹窗，确认后才填入业务表单。目标删除或预入库已确认时询问是否用于新增表单。只恢复解析结果，不恢复完整未保存草稿，业务保存需用户确认。
- 测试：修改 Go 后按顺序执行 `go fix ./...` → `go fmt ./...` → `go test . ./cmd/... ./internal/...`；删除功能需执行 `go test -race ./internal/tasks ./internal/repository`；`task_delete_test.go` / `tasks/delete_test.go` 覆盖删除模式、七天边界、跨批次事务回滚、共享照片/最后引用、清理失败、重试并发及迟到结果，`handlers/task_handler_test.go` 覆盖输入/状态码和鉴权。`internal/handlers/batch_handler_test.go` 覆盖数量边界、完整字段/价格、跨表编号和事务回滚；任务测试使用本地解析器/临时数据库，LCSC 测试使用本地 HTTP 与模型响应覆盖 API 映射、报价精度、回退、AI 及取消/超时，handler/task 测试覆盖报价传递和持久化，无需访问真实商品页。前端在 `web/` 执行 `pnpm run lint`、`pnpm run test:parser`（`scripts/test-parser.mjs`，Vite SSR + Node，验证参考展示、字段隔离、手动修改保护及删除的选择/响应失效/空页回退逻辑）、`pnpm run build`，并执行 `pnpm run test:auth`，然后根目录 `make build BUILD_DIR=<临时输出目录>` 验证静态资源嵌入后的二进制，保护已有本地二进制。


## 共享规范化与保存审核

- `internal/normalization/normalization.go` 是纯函数规则及契约入口，规则版本 `2026-10-19.1`。`Rules` 中封装仅 trim 并精确映射 SOT23/SOT-23、SOT23-5/SOT-23-5；未知、大小写、其他引脚数及 SOT32/SOT-32 保持独立。词典仅整字段精确匹配 Resistor/Resistors、Capacitor/Capacitors、Inductor/Inductors、Diode/Diodes、Transistor/Transistors、Connector/Connectors。分类/厂商审核别名初始为空，修改规则需代码评审。
- `internal/repository/normalization.go` 实现规范读取接口，将启用分类 ID/名称、版本化规则和历史候选分开；历史候选不参与自动选类或创建别名。`GET /api/v1/normalization/standards` 返回规范，`POST /api/v1/normalization/preview` 接受 name/description/model/manufacturer/package/category_name/category_id，仅本地预览，返回 `{candidate, normalization}`，不保存、不联网。两个路径沿用业务鉴权；MCP 同名职责的两个工具标记只读。
- `Category.status` 新增 pending/active/inactive，内部 nullable `active_name` 唯一索引保护并发启用重名（不作为 API 输入输出）；迁移后历史及新增分类默认 pending，必须逐项审核；改名强制 pending，启用拒绝空名称及重复启用名称。REST 分类读取支持 status 筛选，REST/MCP 均可修改状态。表单分类按 ID 选择，新增、更换分类、预入库确认均在当前事务重新验证 active；旧关联未变时允许保留 pending/inactive。无历史名称、封装或分类关联批量迁移。
- `internal/parser/normalization.go` 是普通、AI、Agent（含降级）、二维码及照片结果的共享出口。来源字段在模型处理前复制；Agent 仅保留本次成功查询的来源，不增加请求。`normalization` 包含 version/sources/changes/issues；来源含实际 URL、查询时间、字段及参数文本、截断标记；AI 语义改动标为待确认候选。已查询来源冲突保留主来源事实并报告对照，不自动混合冲突字段。报价库存仍使用独立 supplier_quote。
- 来源及规范化过程只存在同步响应或现有任务结果 JSON 中，无额外来源表；删除任务即删除对应来源记录。库存/预入库只新增 `review` 简短复核状态（字段、原因、确认状态及值摘要），不保存来源原文。`review_confirmations` 是独立输入，键 name/description，值必须等于本次提交的字段；不持久化该输入，也不接受客户端覆盖 review。
- repository 的 Create/PatchDetails/PatchPending/Confirm 统一校验，REST 单条、照片批量、MCP 均复用。保存不翻译名称/描述，不调用模型或联网，仅对新增/变更的封装执行确定性规则；未知封装允许保存。无法可靠中文化的新增/修改名称与描述需明确确认，专业标识、型号、品牌、单位排除；字段值变化需重新确认，中文修正后清除标记，未变化的历史资料不重写。独立库存、价格、位置及图片操作不触发资料重写。
- 规范校验错误返回 `422 {error, issues, item?}`，issues 含 field/code/message；批量 item 从 1 开始，保持全事务回滚。类型错误仍为 400，数据库异常为 500。
- 前端 `NormalizationReview` 由两类库存 `InventoryFields` 及照片核对共用，显示来源原文、候选、当前值和问题；仅空字段自动填充，已有非空值默认保留，AI 语义及冲突候选逐字段选择。预览后可确认保留原文，确认绑定当前值；旧任务无元数据时按当前规范重新本地预览，异步结果不覆盖等待期间修改及恢复时重新读取的记录。`Categories` 页面支持审核启用、停用及状态筛选。
- 验证顺序：`go fix ./...` → `go fmt ./...` → `go test . ./cmd/... ./internal/...` → `go test -race ./internal/repository ./internal/tasks ./internal/mcp`；空间不足时使用 `GOFLAGS=-p=1` 串行运行。执行 `make test-frontend`、`make build-frontend`、`cd web && PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/chromium pnpm test:e2e`，最后 `go build -o /tmp/hamster-bin-check ./cmd/server`。静态资源仍由 embed.go 嵌入 web/dist，未改变嵌入路径。
