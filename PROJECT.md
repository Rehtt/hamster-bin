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
│       └── main.go            # 服务入口：加载配置、初始化数据库、注册解析器、启动路由
├── internal/
│   ├── config/                # 环境变量配置加载
│   ├── database/              # SQLite/GORM 初始化、自动迁移、数据库实例管理
│   ├── handlers/              # Gin HTTP handlers，处理分类、供应商、元件、库存日志和解析请求
│   ├── llm/                   # OpenAI-compatible Chat Completions 客户端
│   ├── models/                # GORM 数据模型：Category、Supplier、Component、StockLog
│   ├── parser/                # 平台解析器、二维码解析、解析器管理器和解析测试
│   ├── repository/            # 数据访问封装，按业务实体拆分
│   └── router/                # API 路由、CORS、嵌入式前端静态文件服务和 SPA fallback
├── web/
│   ├── package.json           # 前端脚本和依赖
│   ├── package-lock.json      # 前端 npm 依赖锁定
│   ├── vite.config.ts         # Vite 配置
│   ├── tailwind.config.js     # Tailwind CSS 配置
│   ├── postcss.config.js      # PostCSS 配置
│   ├── eslint.config.js       # 前端 ESLint 配置
│   ├── tsconfig*.json         # TypeScript 配置
│   ├── index.html             # Vite HTML 入口
│   ├── public/
│   │   └── logo.svg           # 静态 logo
│   └── src/
│       ├── main.tsx           # React 挂载入口
│       ├── App.tsx            # 前端路由：Dashboard、Components、Categories、StockLogs
│       ├── api/client.ts      # Axios 客户端，默认 baseURL 为 /api/v1
│       ├── components/        # 布局、扫码、拍照和可复用 UI 组件
│       ├── pages/             # 主要业务页面
│       ├── types/             # 前端共享类型
│       ├── utils/             # 前端工具函数
│       └── assets/            # 前端源码内资源
└── web_legacy/
    └── index.html             # 旧版前端页面，保留作历史/兼容参考
```

## 后端结构

- `cmd/server/main.go` 是唯一服务入口。它调用 `config.Load()`、`database.Init()`、注册 `parser.ParserManager`，然后通过 `router.Setup()` 启动 Gin 服务。
- `internal/config/config.go` 从环境变量读取配置，当前包含 `PORT`、`DB_PATH`、`IMAGE_DIR`、`LOG_LEVEL`、`SSL_CERT`、`SSL_KEY`、`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`。
- `internal/database/database.go` 负责创建 SQLite 数据目录、打开 GORM 连接、设置 SQLite pragma，并自动迁移 `Category`、`Supplier`、`Component`、`StockLog`。
- `internal/models/models.go` 定义数据库表结构和 JSON 字段，是前后端数据契约的重要来源。
- `internal/router/router.go` 暴露 `/api/v1` API，挂载分类、供应商、元件、库存日志、平台解析接口，并从嵌入的 `web/dist` 提供前端静态文件。
- `internal/handlers/` 负责 HTTP 输入输出和状态码。业务实体目前按 `category`、`supplier`、`component`、`stock_log`、`parser` 拆分。
- `internal/repository/` 封装数据库访问。新增复杂查询时优先放在 repository，避免 handler 直接堆叠大量查询逻辑。
- `internal/llm/` 使用标准库实现 OpenAI-compatible `/chat/completions` JSON 响应调用，供解析器按需使用。
- `internal/parser/` 包含 LCSC、淘宝、二维码解析及解析器管理器。LCSC 支持在 `/components/parse` 请求传入 `use_llm: true` 时使用 LLM 辅助解析元件参数。新增平台时应遵循现有 parser 接口/注册方式，并在启动入口注册。

## 前端结构

- `web/src/App.tsx` 定义 SPA 页面路由：`/`、`/components`、`/categories`、`/logs`。
- `web/src/api/client.ts` 是统一 Axios 客户端，API 前缀固定为 `/api/v1`，与后端路由保持一致。
- `web/src/pages/` 存放业务页面：仪表盘、元件管理、分类管理、库存日志。供应商当前不设独立页面，在元件表单内输入/选择并自动创建。
- `web/src/pages/Components.tsx` 是元件管理主页面，负责元件列表、分页搜索、分类筛选、供应商输入/自动创建、供应商料号录入、平台编码导入、解析结果分类填充、可选 AI 解析、二维码录入、图片上传和库存变更入口。
- `web/src/components/Layout.tsx` 提供页面布局；`QRScanner.tsx` 和 `CameraCapture.tsx` 处理扫码和拍照相关交互。
- `web/src/components/ui/` 存放基础 UI 组件。新增通用控件时优先复用这里的组件风格。
- `web/src/types/index.ts` 存放前端共享类型。后端模型字段变化时，应同步检查这里和调用 API 的页面。

## 数据模型要点

- `Component` 是核心库存实体，必须关联 `Category`，可选关联 `Supplier`。
- `Supplier` 表示采购来源/供应商，例如“嘉立创”“淘宝”；`Component.supplier_id` 可为空以兼容历史数据。
- `Component.supplier_part_number` 表示供应商料号，例如 `C2040`，不要与供应商名称混用。
- 平台解析结果中的 `platform_name` 用于前端推断供应商名称；当前立创/LCSC 导入映射为“嘉立创”，`platform_code` 写入 `supplier_part_number`，`name` 使用商品页名称，`category_name` 使用商品目录并写入前端分类输入框，保存时按现有逻辑关联或自动创建分类。
- 元件列表搜索会匹配元件名称、参数值、供应商料号、描述和供应商名称；修改搜索字段时需同步检查 `ComponentRepository.GetAll` 和元件管理页搜索文案。
- 元件表单保存时会清除前端关联对象，只提交 `category_id`、`supplier_id`、`supplier_part_number` 等字段，避免 GORM 更新关联对象。

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
go build -ldflags="-s -w" -trimpath -o hamster-bin cmd/server/main.go
```

单文件部署流程：

```bash
cd web
npm run build
cd ..
make build
```

前端构建产物必须位于 `web/dist`，因为 `embed.go` 和 `internal/router/router.go` 依赖该路径提供嵌入式静态资源。

## API 与配置约束

- 后端 API 基础路径是 `/api/v1`，前端 Axios 客户端也使用该路径。修改 API 前缀时必须同步修改前后端。
- 主要 API 分组：
  - `/api/v1/categories`
  - `/api/v1/suppliers`
  - `/api/v1/components`
  - `/api/v1/components/:id/stock`
  - `/api/v1/components/:id/logs`
  - `/api/v1/components/:id/image`
  - `/api/v1/components/parse`
  - `/api/v1/components/parse-qrcode`
  - `/api/v1/stock-logs`
  - `/api/v1/platforms`
- 默认数据库路径是 `./data/inventory.db`，由 `DB_PATH` 覆盖。
- 默认图片目录是 `./data/images`，由 `IMAGE_DIR` 覆盖。
- 默认端口是 `8080`，由 `PORT` 覆盖。
- 同时设置 `SSL_CERT` 和 `SSL_KEY` 时，服务使用 HTTPS。
- LLM 辅助解析使用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL` 配置。三项均非空时才可用，`LLM_BASE_URL` 应指向 OpenAI-compatible API base，例如 `https://api.openai.com/v1`，实际请求路径为 `{LLM_BASE_URL}/chat/completions`。
- `POST /api/v1/components/parse` 请求体为 `{ "code": "...", "use_llm": false }`，`use_llm` 可省略且默认 false；仅嘉立创/LCSC 解析器会响应该选项。解析响应可包含 `category_name` 作为建议分类名称，不直接返回数据库 `category_id`。

## 修改约束

- 修改数据模型时，必须检查 GORM tag、JSON 字段、前端类型、API 页面调用和数据库迁移影响。
- 修改路由时，必须同步检查 `web/src/api/client.ts` 以及所有页面里的 API 调用。
- 修改前端构建目录、Vite 输出或静态资源路径时，必须同步检查 `embed.go` 和 `internal/router/router.go`。
- 修改 parser 行为时，必须检查 `cmd/server/main.go` 中的注册逻辑和 `/api/v1/platforms` 返回结果。
- 新增后端业务实体时，优先保持现有分层：`models`、`repository`、`handlers`、`router`。
- 新增前端通用 UI 时，优先放在 `web/src/components/ui/` 并复用现有样式工具。
- 不要把运行时数据、构建产物或本地数据库提交为源码。常见运行时路径包括 `data/`、`web/dist/` 和生成的二进制文件。
- 每次修改代码后，根据 `AGENTS.md` 检查并更新本文档。
