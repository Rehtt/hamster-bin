# Hamster Bin Project Guide for AI Agents

## 项目概览

Hamster Bin 是一个电子元件库存管理系统。后端使用 Go、Gin、GORM 和 SQLite，前端使用 React、TypeScript、Vite 和 Tailwind CSS。系统支持元件、分类、供应商、库存日志管理，并提供立创商城、淘宝、二维码解析和可选 LLM 辅助解析相关能力。

AI agent 修改项目时，应优先保持现有目录分层、API 路径、数据模型和构建流程稳定。若代码修改影响本文档记录的结构、职责、运行方式、API、配置或约束，必须同步更新本文档。

## 文件树与职责

```text
.
├── README.md                  # 项目介绍、快速开始、配置和部署说明
├── LICENSE                    # MIT 许可证
├── Makefile                   # 后端运行、构建、跨平台构建和前端构建入口
├── go.mod / go.sum            # Go 模块定义和依赖锁定
├── package-lock.json          # 根目录 npm lock 文件；当前主要前端依赖在 web/ 内
├── embed.go                   # 将 web/dist 嵌入 Go 二进制
├── cmd/
│   └── server/
│       └── main.go            # 服务入口：处理 --version、加载配置、初始化数据库、注册解析器、启动路由
├── internal/
│   ├── config/                # 环境变量配置加载
│   ├── auth/                  # JWT 签发/解析与凭据校验
│   ├── database/              # SQLite/GORM 初始化、自动迁移、数据库实例管理
│   ├── handlers/              # Gin HTTP handlers，处理分类、供应商、元件、库存日志、解析和鉴权请求
│   ├── middleware/            # Gin 中间件（鉴权）
│   ├── llm/                   # OpenAI-compatible Chat Completions 客户端
│   ├── models/                # GORM 数据模型：Category、Supplier、Component、StockLog
│   ├── parser/                # 平台解析器、二维码解析、解析器管理器和解析测试
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
│       ├── context/           # React Context（AuthProvider）
│       ├── components/        # 布局、扫码、拍照、路由保护和可复用 UI 组件
│       ├── pages/             # 主要业务页面
│       ├── types/             # 前端共享类型
│       ├── utils/             # 前端工具函数
│       └── assets/            # 前端源码内资源
└── web_legacy/
    └── index.html             # 旧版前端页面，保留作历史/兼容参考
```

## 后端结构

- `cmd/server/main.go` 是唯一服务入口。它支持 `--version` 输出版本；正常启动时调用 `config.Load()`、`database.Init()`、注册 `parser.ParserManager`，然后通过 `router.Setup(db, parserManager, cfg)` 启动 Gin 服务。
- `internal/config/config.go` 从环境变量读取配置，当前包含 `PORT`、`DB_PATH`、`IMAGE_DIR`、`LOG_LEVEL`、`SSL_CERT`、`SSL_KEY`、`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`、`ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET`、`JWT_EXPIRE_HOURS`。当 `ADMIN_USERNAME` 与 `ADMIN_PASSWORD` 均非空时启用鉴权，此时 `JWT_SECRET` 必填。
- `internal/auth/` 负责 JWT 签发/解析（Cookie 名 `hamster_token`）和管理员凭据恒定时间比较。
- `internal/middleware/auth.go` 在鉴权启用时校验 Cookie JWT，保护业务 API。
- `internal/database/database.go` 负责创建 SQLite 数据目录、打开 GORM 连接、设置 SQLite pragma，并自动迁移 `Category`、`Supplier`、`Component`、`StockLog`。
- `internal/models/models.go` 定义数据库表结构和 JSON 字段，是前后端数据契约的重要来源。
- `internal/router/router.go` 暴露 `/api/v1` API；`/api/v1/auth/*` 为公开路由，其余业务接口在鉴权启用时需登录。静态资源仍从嵌入的 `web/dist` 提供。
- `internal/handlers/` 负责 HTTP 输入输出和状态码。业务实体目前按 `category`、`supplier`、`component`、`stock_log`、`parser`、`auth` 拆分。
- `internal/repository/` 封装数据库访问。新增复杂查询时优先放在 repository，避免 handler 直接堆叠大量查询逻辑。
- `internal/version/` 保存项目版本变量，默认版本为 `v1.0.0`；发布构建通过 Makefile 的 `VERSION` 变量注入 git tag。
- `internal/llm/` 使用标准库实现 OpenAI-compatible `/chat/completions` JSON 响应调用，供解析器按需使用。
- `internal/parser/` 包含 LCSC、淘宝、二维码解析及解析器管理器。LCSC 支持在 `/components/parse` 请求传入 `use_llm: true` 时使用 LLM 辅助解析元件参数。新增平台时应遵循现有 parser 接口/注册方式，并在启动入口注册。

## 前端结构

- `web/src/App.tsx` 定义 SPA 页面路由：`/`、`/components`、`/categories`、`/logs`、`/login`。业务页面包裹 `ProtectedRoute` 与 `Layout`；登录页不使用侧边栏。各页面通过 `React.lazy` 按路由懒加载，路由切换时显示 Suspense 加载占位。
- `web/src/context/AuthContext.tsx` 启动时调用 `GET /auth/me`，提供 `login`、`logout` 和鉴权状态。
- `web/src/api/client.ts` 是统一 Axios 客户端，API 前缀固定为 `/api/v1`，`withCredentials: true` 以携带 HttpOnly Cookie；401 时跳转 `/login`（`/auth/me` 与 `/auth/login` 除外）。
- `web/src/pages/` 存放业务页面：仪表盘、元件管理、分类管理、库存日志。供应商当前不设独立页面，在元件表单内输入/选择并自动创建。库存日志页支持显示总数和切换每页条数。仪表盘（`Dashboard.tsx`）通过 `GET /stats` 展示元件/分类/库存概览、库存总价值，以及按时间范围（本月/本季/全部）筛选的累计入库金额、入库数量、出库数量。
- `web/src/pages/Components.tsx` 是元件管理主页面，负责元件列表、分字段搜索（编号、名称、厂家型号、制造商、参数、供应商、料号）、分类筛选（可输入下拉）、元件编号录入/展示、厂家型号录入/展示、一键为未编号元件自动补号、供应商输入/自动创建、供应商料号录入、封装/位置/供应商历史下拉选项、平台编码导入、解析结果分类填充、可选 AI 解析（平台编码与扫码共用）、二维码录入、图片上传和库存变更入口。搜索区中制造商、供应商、分类为可输入下拉，选项分别来自 `GET /components/options` 的 `manufacturers`、`GET /suppliers` 和 `GET /categories`，输入时动态过滤匹配。新增元件时可输入采购总价（元），前端换算为分提交并按库存数量展示分摊单价；入库弹窗同样支持总价录入。列表支持显示总数、切换每页条数、选择排序字段与方向（`localStorage` 键 `hamster-components-sort` 持久化；清空筛选不重置排序）、多选元件并批量修改存放位置（批量位置弹窗同样支持历史位置下拉）。列表支持「列设置」：勾选显示列、自定义表头名称与列顺序（`localStorage` 键 `hamster-components-table-columns`，与导出列配置、排序配置独立；勾选框、图片、操作列固定）。支持按当前筛选条件导出 CSV，导出前可在弹窗中勾选列、自定义表头名称与列顺序（`localStorage` 键 `hamster-components-export-columns`）。
- `web/src/components/Layout.tsx` 提供页面布局，桌面端侧边栏 fixed 定位于视口（主内容区通过 `margin-left` 避让），支持收起为图标栏（`localStorage` 键 `hamster-sidebar-collapsed` 持久化）；鉴权启用且已登录时显示退出登录按钮。`QRScanner.tsx` 和 `CameraCapture.tsx` 处理扫码和拍照相关交互，由元件管理页按需懒加载（扫码时才加载 `html5-qrcode`）。
- `web/src/components/ui/` 存放基础 UI 组件。新增通用控件时优先复用这里的组件风格。
- `web/src/types/index.ts` 存放前端共享类型。后端模型字段变化时，应同步检查这里和调用 API 的页面。

## 数据模型要点

- `Component` 是核心库存实体，必须关联 `Category`，可选关联 `Supplier`。
- `Component.component_number` 是系统管理的元件编号，全局唯一；数据库字段允许 `NULL` 以兼容历史未编号数据。自动编号格式为 `HB-000001` 递增；创建时留空会自动生成，也可手动输入任意唯一编号。
- `Component.model` 表示厂家型号，例如 `RC0603FR-0710KL`；与 `name`（商品名称）和 `supplier_part_number`（供应商料号，如 `C2040`）区分。
- `Component.manufacturer` 表示制造商/品牌，例如 `YAGEO`；与 `model`（厂家型号）和 `Supplier`（采购供应商）区分。
- `Supplier` 表示采购来源/供应商，例如“嘉立创”“淘宝”；`Component.supplier_id` 可为空以兼容历史数据。
- `Component.supplier_part_number` 表示供应商料号，例如 `C2040`，不要与供应商名称混用。
- `Component.unit_price_cents` 表示参考单价，单位为分；入库或新增元件带价格时按库存加权平均更新（`(原库存×原单价 + 本次总价) / 新库存`，整数除法向下取整）；无历史库存或历史单价时直接使用本次入库分摊单价。
- `StockLog.unit_price_cents` 和 `StockLog.total_price_cents` 分别表示该条库存记录的分摊单价与录入总价（入库）或成本总价（出库），单位为分；入库时由用户录入总价并按数量分摊单价；出库时若元件有参考单价，则自动按 `unit_price_cents × |change_amount|` 写入成本，无需请求体传价。
- `StockLog.revoked_at` 非空表示该条记录已被撤销；`StockLog.reversal_of_id` 非空表示该条为撤销时自动生成的冲销流水，指向被撤销的原记录 ID。已撤销记录与冲销流水均不可再次撤销。
- 金额在接口和数据库中统一使用整数分，前端展示时格式化为元；单条入库分摊规则为 `unit_price_cents = total_price_cents / quantity`（整数除法，余数不做逐件分摊）；元件参考单价为多次入库的加权平均，撤销入库时会按 `(当前库存×当前单价 - 原记录总价) / 回退后库存` 反算回退，可能存在最多约 1 分的取整漂移。
- 平台解析结果中的 `platform_name` 用于前端推断供应商名称；当前立创/LCSC 导入映射为“嘉立创”，`platform_code` 写入 `supplier_part_number`，`name` 使用商品页名称，`model` 写入厂家型号，`manufacturer` 写入制造商，`category_name` 使用商品目录并写入前端分类输入框，保存时按现有逻辑关联或自动创建分类。
- 元件列表搜索支持分字段 query：`component_number`、`name`、`model`、`manufacturer`、`value`、`supplier`（匹配供应商名称）、`supplier_part_number`；同一字段内按空格拆词，词之间 AND，且均在该字段 LIKE 匹配；多个非空字段之间 AND。`keyword` 仍兼容旧客户端：按空格拆词，每个词需命中编号/名称/厂家型号/制造商/参数/料号/描述/供应商名称任一字段，词之间 AND。修改搜索逻辑时需同步检查 `ComponentRepository.GetAll` 和元件管理页搜索 UI。
- 元件表单保存时会清除前端关联对象，只提交 `category_id`、`supplier_id`、`component_number`、`supplier_part_number`、`manufacturer` 等字段，避免 GORM 更新关联对象。
- 编辑元件时，前端可根据当前 `supplier_part_number` 调用 `POST /api/v1/components/parse` 重新解析并回填名称、厂家型号、制造商、参数、封装、描述、数据手册、图片和分类建议；解析结果中空字段不覆盖表单已有值，库存等本地字段保持不变。

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

发布构建由 GitHub Actions 在推送 `v*` git tag 时触发，例如 `v1.0.0`。Workflow 使用 `npm ci` 安装前端依赖，先构建 `web/dist`，再运行 `go test . ./cmd/... ./internal/...`，最后执行 `make build-all VERSION=${GITHUB_REF_NAME}` 并以同名 tag 创建 GitHub Release。

## API 与配置约束

- 后端 API 基础路径是 `/api/v1`，前端 Axios 客户端也使用该路径。修改 API 前缀时必须同步修改前后端。
- 主要 API 分组：
  - `/api/v1/auth/login`（POST，公开）
  - `/api/v1/auth/logout`（POST，公开）
  - `/api/v1/auth/me`（GET，公开；鉴权关闭返回 `{ auth_enabled: false }`，已登录返回 `{ auth_enabled: true, username }`，未登录返回 401）
  - `/api/v1/categories`
  - `/api/v1/suppliers`
  - `/api/v1/components`
  - `/api/v1/components/options`
  - `/api/v1/components/export`
  - `/api/v1/components/batch-location`
  - `/api/v1/components/generate-numbers`
  - `/api/v1/components/:id/stock`
  - `/api/v1/components/:id/logs`
  - `/api/v1/components/:id/image`
  - `/api/v1/components/parse`
  - `/api/v1/components/parse-qrcode`
  - `/api/v1/stock-logs`
  - `/api/v1/stock-logs/:id/revoke`
  - `/api/v1/stats`
  - `/api/v1/platforms`
- 默认数据库路径是 `./data/inventory.db`，由 `DB_PATH` 覆盖。
- 默认图片目录是 `./data/images`，由 `IMAGE_DIR` 覆盖。
- 默认端口是 `8080`，由 `PORT` 覆盖。
- 同时设置 `SSL_CERT` 和 `SSL_KEY` 时，服务使用 HTTPS，JWT Cookie 的 `Secure` 标志为 true。
- 鉴权：`ADMIN_USERNAME` 与 `ADMIN_PASSWORD` 均非空时启用单管理员登录；`JWT_SECRET` 为签名密钥（启用鉴权时必填）；`JWT_EXPIRE_HOURS` 默认 `168`（7 天）。未配置管理员凭据时鉴权关闭，本地开发无需登录。
- LLM 辅助解析使用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL` 配置。三项均非空时才可用，`LLM_BASE_URL` 应指向 OpenAI-compatible API base，例如 `https://api.openai.com/v1`，实际请求路径为 `{LLM_BASE_URL}/chat/completions`。
- `POST /api/v1/components/parse` 请求体为 `{ "code": "...", "use_llm": false }`，`use_llm` 可省略且默认 false；仅嘉立创/LCSC 解析器会响应该选项。解析响应可包含 `category_name` 作为建议分类名称，不直接返回数据库 `category_id`。可预期解析失败不会统一返回 500：`400` 表示编码格式无效或启用 AI 解析但 LLM 未配置，`422` 表示上游页面已获取但内容无法解析，`502` 表示上游 LCSC 请求失败，`503` 表示无可用解析器。
- `POST /api/v1/components/parse-qrcode` 请求体为 `{ "qrcode_data": "...", "use_llm": false }`，`use_llm` 可省略且默认 false；二维码解析提取平台编码和数量后，同样通过解析器管理器处理，`use_llm` 行为与 `/components/parse` 一致；元件编码解析阶段的错误语义与 `/components/parse` 相同。
- `PATCH /api/v1/components/batch-location` 请求体为 `{ "ids": [1, 2, 3], "location": "A1-03" }`，用于批量更新选中元件的 `location` 字段；`ids` 必填且至少 1 项，`location` 可为空字符串。
- `GET /api/v1/components/options` 无请求参数，返回元件录入表单的历史选项；响应示例 `{ "data": { "packages": ["0603", "0805"], "locations": ["A1-03", "B2-01"], "manufacturers": ["Espressif", "YAGEO"] } }`，`packages`、`locations`、`manufacturers` 分别从已有元件的 `package`、`location`、`manufacturer` 字段去重提取（非空、按名称排序）。表单供应商下拉仍使用 `GET /api/v1/suppliers`；搜索区供应商下拉同样使用该接口。
- `GET /api/v1/components` 支持分页与筛选。常用 query：`page`、`page_size`、`category_id`，以及分字段搜索 `component_number`、`name`、`model`、`manufacturer`、`value`、`supplier`、`supplier_part_number`（语义见上文「元件列表搜索」）。可选排序 query：`sort_by`（白名单字段名，默认 `updated_at`）、`sort_order`（`asc` 或 `desc`，默认 `desc`）；可排序字段与 CSV 导出字段一致。`keyword` 仍兼容 `web_legacy`，React 前端不再使用。
- `GET /api/v1/components/export` 按当前筛选条件导出全部匹配元件为 CSV 文件。必填 query：`columns`（逗号分隔字段名，如 `component_number,name,model`）；可选 query：`headers`（逗号分隔自定义表头，数量需与 `columns` 一致）。筛选与排序 query 与 `GET /api/v1/components` 相同（不含分页），含 `sort_by`、`sort_order`。支持字段：`component_number`、`name`、`model`、`manufacturer`、`value`、`package`、`description`、`category`、`stock_quantity`、`unit_price`（元，保留两位小数）、`location`、`supplier`、`supplier_part_number`、`datasheet_url`、`created_at`、`updated_at`。响应 `Content-Type` 为 `text/csv; charset=utf-8`，带 UTF-8 BOM，文件名形如 `components_YYYYMMDD.csv`。
- `PATCH /api/v1/components/generate-numbers` 无请求体，用于为数据库中所有 `component_number` 为空的元件按 `id` 顺序自动生成 `HB-xxxxxx` 编号；响应示例 `{ "message": "自动编号完成", "updated": 12 }`。
- `POST /api/v1/components` 创建元件时可额外传 `total_price_cents`（分）。当 `stock_quantity > 0` 且 `total_price_cents > 0` 时，服务端计算分摊单价写入 `unit_price_cents`，并自动创建一条 reason 为「初始入库」的 `StockLog`。
- `PUT /api/v1/components/:id` 更新元件字段；请求体与创建相同，可传元件各字段。`unit_price_cents` 不可通过此接口修改（服务端保留原值）。
- `POST /api/v1/components/:id/backfill-price` 补录价格；请求体为 `{ "total_price_cents": 1234, "quantity": 100 }`，`total_price_cents` 与 `quantity` 均须大于 0。按采购数量分摊本批单价；无参考单价时直接设为 `total_price_cents / quantity`，已有参考单价时按当前库存与本次采购数量加权平均更新 `unit_price_cents`（不改库存）。写入一条 `change_amount=0`、reason 形如「补录价格（采购 N 件）」的 `StockLog`。前端入口为元件列表操作栏「补录价格」按钮，不在编辑表单中补录。
- `POST /api/v1/components/:id/stock` 请求体为 `{ "amount": 10, "reason": "采购", "total_price_cents": 1234 }`；`amount` 正数为入库、负数为出库。入库且 `total_price_cents > 0` 时写入分摊单价与总价到流水，并按加权平均更新元件 `unit_price_cents`；出库无需传价，若元件有参考单价则自动写入出库成本（参考单价 × 数量）到流水。库存更新与流水写入在同一事务中完成。
- `POST /api/v1/stock-logs/:id/revoke` 无请求体，用于撤销指定库存记录。服务端在事务中标记原记录 `revoked_at`、回滚库存并写入一条反向冲销流水（`reversal_of_id` 指向原记录）；撤销入库且原记录有总价时会反算回退元件 `unit_price_cents`。撤销入库时若当前库存不足则返回 `400`；已撤销记录或冲销流水再次撤销亦返回 `400`。成功响应示例 `{ "data": { "original": { ... }, "reversal": { ... } } }`。
- `GET /api/v1/stats` 返回仪表盘聚合统计。可选 query：`range`（`month` | `quarter` | `all`，默认 `month`）。响应 `data` 含：`range`、`range_start` / `range_end`（`all` 时 `range_start` 为 null）、`component_count`、`category_count`、`total_stock`、`inventory_value_cents`（当前库存 `stock_quantity × unit_price_cents` 之和，仅统计有库存且有参考单价的元件）、`inbound_quantity`、`outbound_quantity`、`inbound_cost_cents`（后三项按 `range` 过滤 `stock_logs.created_at`，且排除 `revoked_at` 非空、`reversal_of_id` 非空及 `change_amount=0` 的补录价格记录；入库数量与金额为 `change_amount > 0`，出库数量为 `change_amount < 0` 的绝对值之和）。
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
