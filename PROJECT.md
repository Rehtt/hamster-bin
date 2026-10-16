# Hamster Bin Project Guide for AI Agents

## 项目概览

Hamster Bin 是一个电子元件库存管理系统。后端使用 Go、Gin、GORM，默认 SQLite，也支持 MySQL/PostgreSQL；前端使用 React、TypeScript、Vite 和 Tailwind CSS。系统支持元件、预入库、分类、供应商、库存日志管理，并提供立创商城、淘宝、二维码解析和可选 LLM 辅助解析相关能力。

AI agent 修改项目时，应优先保持现有目录分层、API 路径、数据模型和构建流程稳定。若代码修改影响本文档记录的结构、职责、运行方式、API、配置或约束，必须同步更新本文档。

## 文件树与职责

```text
.
├── README.md                  # 项目介绍、快速开始、配置和部署说明
├── LICENSE                    # MIT 许可证
├── Dockerfile                 # 多阶段 Docker 镜像构建（前端 + Go 静态编译 + distroless 运行时）
├── docker-compose.yml         # Docker Compose 部署（默认本地 build + SQLite 卷）
├── .env.example               # Compose 环境变量模板（复制为 .env 后使用）
├── .dockerignore              # Docker build context 排除规则
├── Makefile                   # 后端运行、构建、跨平台构建和前端构建入口
├── go.mod / go.sum            # Go 模块定义和依赖锁定
├── package-lock.json          # 根目录 npm lock 文件；当前主要前端依赖在 web/ 内
├── embed.go                   # 将 web/dist 嵌入 Go 二进制
├── cmd/
│   └── server/
│       └── main.go            # 服务入口：处理 --version、加载配置、初始化数据库、注册解析器、管理任务 worker 与 HTTP 服务信号退出
├── internal/
│   ├── config/                # 环境变量配置加载
│   ├── auth/                  # JWT 签发/解析与凭据校验
│   ├── database/              # SQLite/MySQL/PostgreSQL 的 GORM 初始化、自动迁移、数据库实例管理
│   ├── handlers/              # Gin HTTP handlers，处理分类、供应商、元件、批量保存、库存日志、解析和鉴权请求
│   ├── middleware/            # Gin 中间件（鉴权）
│   ├── mcp/                   # 远程 MCP：独立 Bearer/Origin 校验、类型化工具参数、工具注册及 SDK 集成测试
│   ├── llm/                   # OpenAI-compatible Chat Completions 客户端
│   ├── models/                # GORM 数据模型：Category、Supplier、Component、PreStock、StockLog、Task
│   ├── price/                 # 单价（微元）与总价（分）换算及加权平均
│   ├── parser/                # 平台解析器、二维码/照片 AI 解析、解析器管理器和解析测试
│   ├── tasks/                 # 持久化队列执行、照片存储、校验、超时、取消、重试与中断恢复
│   ├── repository/            # 数据访问封装，按业务实体拆分
│   ├── router/                # API 路由、CORS、嵌入式前端静态文件服务和 SPA fallback
│   └── version/               # 项目版本变量，默认 v1.0.0，release 构建时通过 ldflags 注入 git tag
├── web/
│   ├── package.json           # 前端脚本和依赖
│   ├── package-lock.json      # 前端 npm 依赖锁定
│   ├── vite.config.ts         # Vite 配置；生产构建通过 manualChunks 拆分 react、lucide、html5-qrcode 等 vendor
│   ├── tailwind.config.js     # Tailwind CSS 配置
│   ├── postcss.config.js      # PostCSS 配置
│   ├── eslint.config.js       # 前端 ESLint 配置
│   ├── tsconfig*.json         # TypeScript 配置
│   ├── index.html             # Vite HTML 入口
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

## 后端结构

- `cmd/server/main.go` 是唯一服务入口。它支持 `--version` 输出版本；正常启动时调用 `config.Load()`、`database.Init()`、注册 `parser.ParserManager`，然后通过 `router.Setup(db, parserManager, cfg, taskService)` 启动 Gin 服务。
- `internal/config/config.go` 从环境变量读取配置，当前包含 `PORT`、`DB_DRIVER`、`DB_DSN`、`DB_PATH`、`IMAGE_DIR`、`LOG_LEVEL`、`SSL_CERT`、`SSL_KEY`、`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`、`ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET`、`JWT_EXPIRE_HOURS`、`MCP_TOKEN`、`MCP_ALLOWED_ORIGINS`、`TASK_WORKERS`、`TASK_TIMEOUT_SECONDS`。当 `ADMIN_USERNAME` 与 `ADMIN_PASSWORD` 均非空时启用鉴权，此时 `JWT_SECRET` 必填。
- `internal/auth/` 负责 JWT 签发/解析（Cookie 名 `hamster_token`）和管理员凭据恒定时间比较。
- `internal/middleware/auth.go` 在鉴权启用时校验 Cookie JWT，保护业务 API。
- `internal/mcp/` 使用官方 `github.com/modelcontextprotocol/go-sdk v1.8.0`，提供无状态 Streamable HTTP 和 JSON 响应。`http.go` 处理独立 Token/Origin/CORS，`inputs.go` 定义中文参数 Schema，`server.go` 注册工具并复用 repository/price；SDK 自动生成结构化结果与兼容 JSON 文本，并将业务错误转换为 `isError`。
- `internal/repository/inventory_write.go` 为 MCP 补充带关联校验的创建事务和字段级更新。元件编号、价格分摊、保存及有价初始流水同事务提交；更新限制资料字段，保留未传字段与库存/单价。预入库仅允许编辑 pending，校验失败回滚。REST 契约保持原样，无新增迁移或前端页面。
- 入出库与预入库确认在支持的数据库上使用事务行锁；SQLite 沿用事务写锁。库存与金额运算拒绝会造成整数溢出的输入。
- `internal/database/database.go` 按 `DB_DRIVER` 打开 SQLite/MySQL/PostgreSQL 的 GORM 连接；SQLite 会创建数据目录并设置 pragma，所有数据库都会自动迁移 `Category`、`Supplier`、`Component`、`PreStock`、`StockLog`、`Task`。
- `internal/models/models.go` 定义数据库表结构和 JSON 字段，是前后端数据契约的重要来源。
- `internal/router/router.go` 暴露 `/api/v1` API；`/api/v1/auth/*` 为公开路由，其余业务接口在鉴权启用时需登录。静态资源仍从嵌入的 `web/dist` 提供。
- `internal/handlers/` 负责 HTTP 输入输出和状态码。业务实体目前按 `category`、`supplier`、`component`、`stock_log`、`stats`、`parser`、`tasks`、`auth` 拆分。
- `internal/price/price.go` 集中实现单价分摊（`UnitPriceMicro`）、出库成本（`OutboundTotalCents`）、加权平均（`WeightedAverageUnitPriceMicro`）与撤销反算（`ReverseAverageUnitPriceMicro`）；repository 与 handler 应复用此包，避免重复四舍五入逻辑。
- `internal/repository/` 封装数据库访问。新增复杂查询时优先放在 repository，避免 handler 直接堆叠大量查询逻辑。
- `internal/version/` 保存项目版本变量，默认版本为 `v1.0.0`；发布构建通过 Makefile 的 `VERSION` 变量注入 git tag。
- `internal/llm/` 使用标准库实现 OpenAI-compatible `/chat/completions` JSON 响应调用，支持文本消息和图片 data URL 多模态消息，供解析器按需使用。
- `internal/parser/` 包含 LCSC、淘宝、二维码和照片 AI 解析及解析器管理器。LCSC 支持在 `/components/parse` 请求传入 `use_llm: true` 时使用 LLM 辅助解析元件参数；照片解析复用同一 LLM 配置并返回可核对的多条结果。新增平台时应遵循现有 parser 接口/注册方式，并在启动入口注册。

## 前端结构

- `web/src/App.tsx` 定义 SPA 页面路由：`/`、`/components`、`/pre-stocks`、`/categories`、`/logs`、`/tasks`、`/login`。业务页面包裹 `ProtectedRoute` 与 `Layout`；登录页不使用侧边栏。各页面通过 `React.lazy` 按路由懒加载，路由切换时显示 Suspense 加载占位。
- `web/src/context/AuthContext.tsx` 提供 `AuthProvider`，启动时调用 `GET /auth/me` 并维护 `login`、`logout` 和鉴权状态；`context/auth.ts` 定义共享 Context 与类型，`context/useAuth.ts` 提供读取鉴权状态的 hook。为满足 React Fast Refresh 规则，组件文件不导出非组件 hook。
- `web/src/api/client.ts` 是统一 Axios 客户端，API 前缀固定为 `/api/v1`，`withCredentials: true` 以携带 HttpOnly Cookie；401 时跳转 `/login`（`/auth/me` 与 `/auth/login` 除外）。
- `web/src/pages/` 存放业务页面：仪表盘、元件管理、预入库、分类管理、库存日志。供应商当前不设独立页面，在元件/预入库表单内输入/选择并自动创建。库存日志页支持显示总数和切换每页条数。仪表盘（`Dashboard.tsx`）通过 `GET /stats` 展示元件/分类/库存概览、库存总价值，以及按时间范围（本月/本季/全部）筛选的累计入库金额、入库数量、出库数量。
- `web/src/pages/Components.tsx` 是元件管理主页面，负责元件列表、分字段搜索（编号、名称、厂家型号、制造商、参数、供应商、料号）、分类筛选（可输入下拉）、元件编号录入/展示、厂家型号录入/展示、一键为未编号元件自动补号、供应商输入/自动创建、供应商料号录入、封装/位置/供应商历史下拉选项、平台编码导入、解析结果分类填充、可选 AI 解析（平台编码与扫码共用）、二维码录入、图片上传、拍摄和图片 URL 查看/编辑、补录价格（`POST /components/:id/backfill-price`）和库存变更入口。移动端（`< md`）搜索筛选区默认折叠，由 `CollapsibleFilterPanel` 提供折叠头、条件数量 badge 与快捷搜索；搜索成功后自动收起以展示列表。列表中系统编号、厂家型号、供应商料号支持点击复制到剪贴板；列表操作列使用 `RowActionsMenu` 行级悬浮菜单（⋮ 始终可见，操作列 sticky 右固定，横向滚动时不丢失；点击在触发按钮左侧单行横向展开编辑/库存/补录价格/记录/复制/删除，激活行内容 blur，点外部或 Esc 关闭），其中「复制」可将元件资料以新增表单提交副本，副本清空元件编号、库存和参考单价，由后端自动生成新编号。搜索区中制造商、供应商、分类为可输入下拉，选项分别来自 `GET /components/options` 的 `manufacturers`、`GET /suppliers` 和 `GET /categories`，输入时动态过滤匹配。新增元件时可输入采购总价（元），前端换算为分提交并按库存数量展示分摊单价（微元格式化）；库存数量、补录价格采购数量和库存变更数量支持 5、10、20、50、100 快捷选择；入库弹窗同样支持总价录入，出库时展示参考单价与预估成本。列表支持显示总数、切换每页条数、选择排序字段与方向（`localStorage` 键 `hamster-components-sort` 持久化；清空筛选不重置排序）、多选元件并批量修改存放位置（批量位置弹窗同样支持历史位置下拉），以及批量出库（页面顶部按钮或勾选栏入口；`BatchStockOutModal` 支持搜索添加/删除行、逐行填写出库数量与统一备注，调用 `POST /components/batch-stock-out` 一键提交）。列表支持「列设置」：勾选显示列、自定义表头名称与列顺序（`localStorage` 键 `hamster-components-table-columns`，与导出列配置、排序配置独立；勾选框、图片、操作列固定）。支持按当前筛选条件导出 CSV，导出前可在弹窗中勾选列、自定义表头名称与列顺序（`localStorage` 键 `hamster-components-export-columns`）。
- `web/src/pages/PreStocks.tsx` 是预入库页面，负责待入库记录列表、状态筛选、分页、新建/编辑预入库、平台编码解析、二维码解析、分类/供应商输入并自动创建、采购总价分摊预览、图片缩略图/预览、确认入库和删除待入库记录。待入库行操作列同样使用 `RowActionsMenu`（sticky 右列、⋮ 常显、操作单行横向展开：编辑/确认入库/删除）；已入库行显示关联元件 ID 文字。移动端状态筛选区同样使用 `CollapsibleFilterPanel` 折叠，折叠头展示当前状态摘要。预计数量支持加减步进与 5、10、20、50、100 快捷选择。预入库保存时自动生成 `HB-xxxxxx` 编号但不进入正式库存；确认入库后转为正式元件并写库存流水。
- `web/src/components/Layout.tsx` 提供页面布局，桌面端侧边栏 fixed 定位于视口（主内容区通过 `margin-left` 避让），支持收起为图标栏（`localStorage` 键 `hamster-sidebar-collapsed` 持久化）；鉴权启用且已登录时显示退出登录按钮。`BatchStockOutModal.tsx` 提供批量出库弹窗（搜索添加元件、行列表展示供应商与供应商料号、逐行数量与成本预览、失败行高亮）。`QRScanner.tsx`、`CameraCapture.tsx` 和 `PhotoParseModal.tsx` 处理扫码、普通拍照与拍照 AI 解析；照片解析弹窗支持核对结果、数量校验，新增可多选，编辑使用单选。
- `web/src/components/ui/` 存放基础 UI 组件。`PageHeader` 统一页面标题与操作按钮区（移动端 `flex-wrap` 换行）；`CollapsibleFilterPanel` 在 `< md` 时默认折叠筛选内容，桌面端始终展开，可通过 ref 调用 `collapse()` 收起；`RowActionsMenu` 提供表格行级悬浮操作菜单（⋮ 始终可见、sticky 右列；展开后 icon 按钮单行横向排列，外部点击/Esc 关闭）。新增通用控件时优先复用这里的组件风格。
- `web/src/types/index.ts` 存放前端共享类型。后端模型字段变化时，应同步检查这里和调用 API 的页面。
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
- 平台解析结果中的 `platform_name` 用于前端推断供应商名称；当前立创/LCSC 导入映射为“嘉立创”，`platform_code` 写入 `supplier_part_number`，`name` 使用商品页名称，`model` 写入厂家型号，`manufacturer` 写入制造商，`category_name` 使用商品目录并写入前端分类输入框，保存时按现有逻辑关联或自动创建分类。
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
npm run dev
```

前端构建：

```bash
cd web
npm run build
```

后端构建：

```bash
make build
# 或
go build -ldflags="-s -w -X github.com/Rehtt/hamster-bin/internal/version.Version=v1.0.0" -trimpath -o hamster-bin cmd/server/main.go
```

版本查看：

```bash
./hamster-bin --version
```

单文件部署流程：

```bash
cd web
npm run build
cd ..
make build
```

前端构建产物必须位于 `web/dist`，因为 `embed.go` 和 `internal/router/router.go` 依赖该路径提供嵌入式静态资源。

Docker 镜像通过根目录 `Dockerfile` 多阶段构建：`node:20-alpine` 构建 `web/dist`，`golang:1.25-alpine` 以 `CGO_ENABLED=0` 编译并注入 `VERSION`，运行时镜像为 `gcr.io/distroless/static-debian12:nonroot`。容器默认监听 `8080`，数据目录 `/app/data`（`DB_DRIVER=sqlite`、`DB_PATH=/app/data/inventory.db`，`IMAGE_DIR=/app/data/images`），需挂载 volume 持久化 SQLite 与图片；连接外部 MySQL/PostgreSQL 时通过 `DB_DRIVER` 与 `DB_DSN` 覆盖。

Docker Compose 部署（`docker-compose.yml`）：

```bash
cp .env.example .env   # 可选
docker compose up -d --build
docker compose logs -f
docker compose down
```

默认从源码 `build` 镜像；切换 GHCR 预构建镜像时注释 `build` 段并启用 `image: ghcr.io/rehtt/hamster-bin:latest`。全部运行时环境变量见 `.env.example`，通过复制为 `.env` 后由 compose 注入容器。

发布构建由 GitHub Actions 在推送 `v*` git tag 时触发，例如 `v1.0.0`。`build-and-release` job 使用 `npm ci` 安装前端依赖，先构建 `web/dist`，再运行 `go test . ./cmd/... ./internal/...`，最后执行 `make build-all VERSION=${GITHUB_REF_NAME}` 并以同名 tag 创建 GitHub Release。并行的 `docker` job 使用 buildx 构建 `linux/amd64` 与 `linux/arm64` 镜像，推送到 `ghcr.io/rehtt/hamster-bin`，tag 为 git tag 与 `latest`。

## API 与配置约束

- 远程 MCP 入口为 `/mcp`，不属于 `/api/v1`。`MCP_TOKEN` 默认为空，此时所有方法返回 404；配置后使用 `Authorization: Bearer <token>`，对固定长度 Token 摘要做恒定时间比较，与 Web Cookie/JWT 独立。单个部署级 Token 拥有全部 MCP 工具权限，修改环境变量并重启轮换。
- `MCP_ALLOWED_ORIGINS` 为逗号分隔的完整 Origin 列表，空值默认仅同源或无 Origin；不支持通配符或路径。`/mcp` 跳过 REST 全局 CORS，先做独立 Origin 校验再处理预检，非法 Origin 返回 403，缺失/错误 Token 返回 401。代理头不参与同源判断；反向代理终止 HTTPS 后的外部浏览器 Origin 需配置白名单，回环上游 Host 应使用本机地址以兼容 SDK 的 DNS 重绑定保护。
- MCP 工具共 17 个：`list_components`、`get_component`、`create_component`、`update_component`、`change_stock`；`list_pre_stocks`、`get_pre_stock`、`create_pre_stock`、`update_pre_stock`、`confirm_pre_stock`；`list_categories`、`create_category`、`list_suppliers`、`create_supplier`；`list_stock_logs`、`get_component_stock_logs`、`get_stats`。不开放删除、撤销、批量、平台解析和后台任务。
- MCP 分页默认 page=1、page_size=20，上限 100；单元件流水 limit 默认 50，上限 100。预入库状态默认 pending（可选 confirmed/all），统计默认 month（可选 quarter/all）。查询复用 REST 的搜索、分类和排序规则。
- MCP 成功响应保持 `{data, pagination?}` 结构；`result.Data` 保留 `any`，必须带中文 `jsonschema` 描述，使 SDK 输出仅含 `description` 的对象 Schema，避免生成 `"data": true` 导致 Hermes 添加服务时校验失败。不要通过限定 data 类型缩小对象、数组等返回值范围；`additionalProperties: false` 仍是合法约束。
- MCP 创建必填名称、分类 ID 和非负数量（元件 stock_quantity、预入库 expected_quantity）；供应商使用已有 ID，同名创建供应商返回已有记录。总价为整数分，单价为微元；数量为零时不允许正数总价。元件编辑不接受库存/参考单价；局部编辑省略字段保持原值、空字符串清除文本、supplier_id=0 解除关联。`change_stock.amount` 正入负出且不得为零，入库总价非负，出库成本自动计算，复用 `ApplyStockChange`；预入库确认复用 `Confirm`。
- MCP 集成测试位于 `internal/mcp/mcp_test.go`，通过官方 SDK 客户端覆盖发现、新旧版本握手、17 个工具、Schema/文本兼容、认证/Origin、写入与失败回滚。`TestSDKWorkflow` 递归检查全部工具的输入和输出 Schema，要求 `properties` 下各字段为对象，并确认 data Schema 仅含描述；复用业务调用覆盖对象、数组、文本回退和错误结果，分页由 `TestLegacyHandshakeAndPagination` 覆盖。验证使用 `go fix ./...` → `go fmt ./...` → `go test . ./cmd/... ./internal/...` → `make build`。
- 后端 API 基础路径是 `/api/v1`，前端 Axios 客户端也使用该路径。修改 API 前缀时必须同步修改前后端。
- 主要 API 分组：
  - `/api/v1/auth/login`（POST，公开）
  - `/api/v1/auth/logout`（POST，公开）
  - `/api/v1/auth/me`（GET，公开；鉴权关闭返回 `{ auth_enabled: false }`，已登录返回 `{ auth_enabled: true, username }`，未登录返回 401）
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
- 同时设置 `SSL_CERT` 和 `SSL_KEY` 时，服务使用 HTTPS，JWT Cookie 的 `Secure` 标志为 true。
- 鉴权：`ADMIN_USERNAME` 与 `ADMIN_PASSWORD` 均非空时启用单管理员登录；`JWT_SECRET` 为签名密钥（启用鉴权时必填）；`JWT_EXPIRE_HOURS` 默认 `168`（7 天）。未配置管理员凭据时鉴权关闭，本地开发无需登录。
- LLM 辅助解析使用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL` 配置。三项均非空时才可用，`LLM_BASE_URL` 应指向 OpenAI-compatible API base，例如 `https://api.openai.com/v1`，实际请求路径为 `{LLM_BASE_URL}/chat/completions`。
- `POST /api/v1/components/parse` 请求体为 `{ "code": "...", "use_llm": false }`，`use_llm` 可省略且默认 false；仅嘉立创/LCSC 解析器会响应该选项。解析响应可包含 `category_name` 作为建议分类名称，不直接返回数据库 `category_id`。可预期解析失败不会统一返回 500：`400` 表示编码格式无效或启用 AI 解析但 LLM 未配置，`422` 表示上游页面已获取但内容无法解析，`502` 表示上游 LCSC 请求失败，`503` 表示无可用解析器。
- `POST /api/v1/components/parse-qrcode` 请求体为 `{ "qrcode_data": "...", "use_llm": false }`，`use_llm` 可省略且默认 false；二维码解析提取平台编码和数量后，同样通过解析器管理器处理，`use_llm` 行为与 `/components/parse` 一致；元件编码解析阶段的错误语义与 `/components/parse` 相同。
- `PATCH /api/v1/components/batch-location` 请求体为 `{ "ids": [1, 2, 3], "location": "A1-03" }`，用于批量更新选中元件的 `location` 字段；`ids` 必填且至少 1 项，`location` 可为空字符串。
- `POST /api/v1/components/batch-stock-out` 请求体为 `{ "reason": "项目A", "items": [{ "component_id": 1, "quantity": 5 }] }`，用于批量出库；`items` 必填且至少 1 项，每项 `quantity > 0`，`component_id` 不可重复。服务端在单事务中预校验全部元件存在且库存足够，任一失败则整批回滚并返回 `400` 与 `failures` 数组（含 `component_id`、`component_name`、`stock_quantity`、`requested`、`error`）。成功时写入各元件负向库存流水（出库成本规则同 `POST /components/:id/stock`），响应 `data` 含 `updated`、`total_quantity`、`total_cost_cents`。
- `GET /api/v1/components/options` 无请求参数，返回元件录入表单的历史选项；响应示例 `{ "data": { "packages": ["0603", "0805"], "locations": ["A1-03", "B2-01"], "manufacturers": ["Espressif", "YAGEO"] } }`，`packages`、`locations`、`manufacturers` 分别从已有元件的 `package`、`location`、`manufacturer` 字段去重提取（非空、按名称排序）。表单供应商下拉仍使用 `GET /api/v1/suppliers`；搜索区供应商下拉同样使用该接口。
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
- `GET /api/v1/stats` 返回仪表盘聚合统计。可选 query：`range`（`month` | `quarter` | `all`，默认 `month`）。响应 `data` 含：`range`、`range_start` / `range_end`（`all` 时 `range_start` 为 null）、`component_count`、`category_count`、`total_stock`、`inventory_value_cents`（当前库存 `round(stock_quantity×unit_price_micro/10000)` 之和，仅统计有库存且有参考单价的元件）、`inbound_quantity`、`outbound_quantity`、`inbound_cost_cents`（后三项按 `range` 过滤 `stock_logs.created_at`，且排除 `revoked_at` 非空、`reversal_of_id` 非空及 `change_amount=0` 的补录价格记录；入库数量与金额为 `change_amount > 0`，出库数量为 `change_amount < 0` 的绝对值之和）。
- 前端全局库存记录页（`/logs`）与元件管理页的库存记录弹窗均支持撤销操作；已撤销记录显示「已撤销」标签并降低透明度，冲销流水显示「撤销冲销」标签。

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
- 单独的 dispatcher 协程负责领取队列并通过容量等于 `TASK_WORKERS` 的 `work` channel 分发给 worker；入队广播会立即唤醒 dispatcher，数据库每 30 秒兜底扫描一次，领取失败同样等待通知或兜底重试。dispatcher 停止领取后关闭 work；已领取但尚未开始执行的任务恢复为 `queued`，运行中的任务记录为中断。
- `internal/tasks/images.go` 提供 `image_parse` 任务的 JPEG/PNG 校验与 `IMAGE_DIR/task-inputs` 私有照片存储；任务输入只保存照片引用，不把 Base64 写入数据库。照片任务复用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`，模型需支持图片输入，结果为可人工核对的 `items` 数组。
- `TASK_WORKERS` 默认 `2`，`TASK_TIMEOUT_SECONDS` 默认 `120`，从领取开始执行计时；非正整数或无效值使用默认值。抓取仍限 30 秒，模型请求仍限 90 秒，二者同时接收任务上下文以终止实际 HTTP 请求。
- `cmd/server/main.go` 启动 worker 前把遗留 `running` 标记失败（`interrupted`，服务中断），保留 `queued`。SIGINT/SIGTERM 时停止领取并取消执行，记录运行任务中断，再关闭 HTTP 服务。仅支持单服务实例；不引入 Redis，不自动重试，不删除或清理历史。
- 所有 `/api/v1/tasks` 接口使用现有鉴权。POST 请求：`{"type":"component_parse","input":{"code":"C2040","use_llm":false},"source":"components"}`（可选 `record_id`）；二维码任务使用 `type:"qrcode_parse"` 与 `input.qrcode_data`；照片使用 `POST /api/v1/tasks/image` multipart 字段 `image`、`source`、可选 `record_id`，返回 `202 {data:Task}`。照片限制 JPEG/PNG、10 MiB、4000 万像素，结果为 `items`。来源可省略或为 `components`/`pre-stocks`，编辑记录 ID 需有有效来源。GET 列表支持 `status`、`type`、`page`、`page_size`（1–100），返回 `{data,pagination}`；非法状态操作返回 409。详情含原输入和可展示错误。
- `POST /api/v1/components/batch` 与 `POST /api/v1/pre-stocks/batch` 接收 `{ "items": [...] }`，最多 100 条；每条需要名称、分类和显式填写的非负整数数量（元件 `stock_quantity`、预入库 `expected_quantity`）；缺失、`null`、负数或小数返回 400，明确填写的 0 合法。分类/供应商可按名称自动匹配或创建。成功返回 `201 {data:[...]}`，顺序与请求条目一致。整个批量操作使用单事务，任一条失败连同分类、供应商和库存流水全部回滚；元件入库价格和库存流水遵循普通创建规则，无数据库迁移。
- 旧 `/components/parse` 和 `/components/parse-qrcode` 继续同步返回原有结构，复用相同解析器并传递 HTTP 请求上下文。
- `web/src/context/TaskContext.tsx` 在受保护布局内维护共享任务状态，每 2 秒轮询跟踪中的任务。会话存储 `hamster-pending-tasks` 保存未完成 ID；刷新时分页恢复数据库排队/运行任务，并查询原跟踪 ID，历史完成任务不重复通知。退出登录卸载 Provider，终止轮询。
- `/tasks`（`pages/Tasks.tsx`）是任务中心，提供分页、状态/类型筛选、输入/结果/错误详情、取消、重试、应用结果。元件和预入库表单的“拍照 AI”统一采用“核对结果 → 复用业务表单逐条编辑 → 一次批量保存”。核对时新增可多选，编辑已有记录使用单选并可直接切换任意结果；未知数量必须手动补齐，显式 0 会保留。侧边栏有入口。SPA fallback 直接返回嵌入的 `web/dist/index.html`，刷新任务/恢复链接不会丢失路径和查询参数；构建产物不提交。
- `hooks/usePhotoDrafts.ts` 保存独立的完整表单草稿，`components/PhotoDraftNavigation.tsx` 提供条目切换与进度提示；切换和提交前合并当前编辑值，包含分类、供应商、数量、价格、位置、编号和图片。校验失败定位条目，请求失败保留全部草稿并恢复保存状态；元件创建成功后按响应顺序上传各条图片，图片失败提示对应条目但不撤销已保存元件。预入库图片沿用 URL 字段。`utils/photoResult.ts` 显式转换照片字段，正确映射供应商料号；`types/index.ts` 区分原始 `ParsedImageItem`（数量可为 `null`）与已核对 `ReviewedImageItem`（数量为整数），`TaskResult.component` 为可选。
- `components/PhotoParseModal.tsx` 每次打开/切换目标创建独立会话；提交/解析期间可关闭，确认或关闭会卸载弹窗并释放摄像头，重新打开默认显示拍摄/选图入口。共享任务上下文继续跟踪后台任务，会话标识阻止迟到响应或旧任务回填新弹窗。
- `hooks/useParseTask.ts` 隔离表单会话，`utils/parseResult.ts` 按提交快照逐字段合并非空解析值；手动修改保留。关闭、保存、重新打开、复制表单或切换照片草稿使旧编码/二维码任务失去自动回填资格，任务仍继续。`components/ParseTaskStatus.tsx` 显示任务编号和状态，解析不阻塞编辑/关闭/保存，仅禁用同一会话内重复提交。
- `hooks/useTaskApplication.ts` 消费业务页 `?applyTask=<id>` 恢复链接；先取任务，再重新读取编辑记录；照片任务先进入同一核对弹窗，确认后才填入业务表单。目标删除或预入库已确认时询问是否用于新增表单。只恢复解析结果，不恢复完整未保存草稿，业务保存需用户确认。
- 测试：`go test ./...`，`go test -race ./internal/tasks ./internal/repository`；`internal/handlers/batch_handler_test.go` 覆盖数量边界、完整字段/价格、跨表编号和事务回滚；任务测试使用本地解析器/临时数据库，解析器测试使用本地 HTTP 与模型响应，无需访问真实商品页。修改 Go 后先执行 `go fix ./...`、`go fmt ./...` 再测试；前端执行 `npm run lint`、`npm run build`（在 `web/` 下）。
