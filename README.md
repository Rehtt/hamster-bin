<p align="center">
  <img src="web/public/logo.svg" alt="Hamster Bin Logo" width="120" height="120">
</p>

<h1 align="center">Hamster Bin</h1>

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://github.com/Rehtt/hamster-bin/actions/workflows/release.yml/badge.svg)](https://github.com/Rehtt/hamster-bin/actions/workflows/release.yml)

Hamster Bin 是一个面向电子爱好者、实验室和小型团队的电子元件库存管理系统。它用 Go 后端、GORM 数据库层和 React 前端组成，默认使用 SQLite，也支持连接 MySQL 或 PostgreSQL；支持本地运行，也支持将前端静态资源嵌入到单个 Go 二进制文件中部署。

如果你经常需要管理电阻、电容、芯片、模块、传感器等元器件，Hamster Bin 可以帮助你记录库存、位置、供应商料号、采购价格、Datasheet、图片和每一次库存变动。

## 功能特性

- 元件库存管理：新增、编辑、删除、搜索、筛选、排序和分页查看元件。
- 自动编号：为元件生成 `HB-000001` 形式的内部编号，也支持手动填写唯一编号。
- 分类与供应商：支持多级分类、供应商记录、供应商料号和历史输入选项。
- 库存流水：记录入库、出库、批量出库、补录价格、撤销和冲销，保留库存变动原因。
- 价格管理：入库总价按数量分摊为单价，元件参考单价按库存加权平均更新。
- 数据导出：按当前筛选条件导出 CSV，并支持自定义导出列和表头。
- 平台解析：支持立创商城/LCSC 编码解析，二维码解析可提取平台编码和数量。
- 持久化后台解析：元件与预入库的编码、二维码和照片 AI 解析提交后立即返回任务编号，可继续编辑或关闭表单。照片支持 JPEG/PNG 拍摄或选图、识别多条元件，核对后在完整表单中逐条切换编辑并一次批量保存（未知数量需补齐，0 合法）；任务中心支持结果恢复、失败重试和取消，跨页面显示完成提示；结果需确认保存后才写入业务数据。
- 可选 AI 辅助解析：配置 OpenAI-compatible API 后，可辅助解析元件参数。
- 图片与资料：支持元件图片上传、拍照 AI 解析、Datasheet 链接和描述信息。
- 可选登录鉴权：支持管理员密码和统一 OAuth / OIDC 登录，可单独启用或并存，使用 HttpOnly Cookie 保存应用会话。
- 可选远程 MCP：外部 AI 客户端通过独立 Bearer Token 查询和管理元件、预入库、入出库及统计。
- 单文件部署：生产构建可将 React 前端嵌入 Go 二进制，便于在内网或个人服务器运行。

## 项目状态

项目仍在持续迭代中，适合个人库存、实验室轻量管理和自托管使用。当前默认使用 SQLite，也可连接外部 MySQL/PostgreSQL；未内置多用户权限模型。如需公网部署，建议启用 HTTPS、强密码和足够随机的 `JWT_SECRET`。

## 技术栈

后端：

- Go 1.25.6
- Gin
- GORM
- SQLite / MySQL / PostgreSQL
- GoQuery

前端：

- React 19
- TypeScript
- Vite
- Tailwind CSS
- Axios
- Lucide React

## 快速开始

### 环境要求

- Go 1.25.6+
- Node.js 20.19+ 或 22.12+
- npm

### 获取代码

```bash
git clone https://github.com/Rehtt/hamster-bin.git
cd hamster-bin
```

### 开发模式运行

先启动后端：

```bash
go mod download
make run
```

后端默认监听 `http://localhost:8080`。首次运行会自动创建 `data/` 目录，并在 `data/inventory.db` 初始化 SQLite 数据库。

再启动前端：

```bash
cd web
npm install
npm run dev
```

前端开发服务器通常监听 `http://localhost:5173`，API 请求会使用 `/api/v1` 前缀访问后端。

## 生产构建与部署

Hamster Bin 支持单文件部署。生产模式下需要先构建前端，使产物位于 `web/dist`，再构建 Go 二进制。

```bash
cd web
npm ci
npm run build
cd ..
make build
./hamster-bin
```

构建后的 `hamster-bin` 会包含前端静态资源。运行后访问 `http://localhost:8080` 即可打开 Web 界面。

也可以直接使用 Go 命令构建：

```bash
go build -ldflags="-s -w -X github.com/Rehtt/hamster-bin/internal/version.Version=v1.0.0" -trimpath -o hamster-bin cmd/server/main.go
```

查看版本：

```bash
./hamster-bin --version
```

## Docker 部署

Release 版本会发布到 GitHub Container Registry，镜像地址为 `ghcr.io/rehtt/hamster-bin`，支持 `linux/amd64` 与 `linux/arm64`。

拉取并运行：

```bash
docker pull ghcr.io/rehtt/hamster-bin:latest
docker run -d \
  --name hamster-bin \
  -p 8080:8080 \
  -v hamster-data:/app/data \
  ghcr.io/rehtt/hamster-bin:latest
```

容器内默认数据目录为 `/app/data`（SQLite 数据库 `/app/data/inventory.db`，图片 `/app/data/images`）。可通过 `-e` 传入环境变量，例如启用登录鉴权：

```bash
docker run -d \
  --name hamster-bin \
  -p 8080:8080 \
  -v hamster-data:/app/data \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD='your-strong-password' \
  -e JWT_SECRET='random-32+-char-secret' \
  ghcr.io/rehtt/hamster-bin:v1.0.0
```

本地从源码构建镜像：

```bash
docker build --build-arg VERSION=v1.0.0 -t hamster-bin:local .
```

### Docker Compose

推荐使用 `docker-compose.yml` 一键启动（默认从源码构建镜像，SQLite 数据持久化到 `./data` 目录）：

```bash
# 复制环境变量模板并按需修改（全部可配置项见 .env.example）
cp .env.example .env

# 启动（后台）
docker compose up -d --build

# 查看日志
docker compose logs -f

# 停止
docker compose down
```

启用鉴权、HTTPS 或 LLM 辅助解析时，在 `.env` 中设置对应变量（`ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET`，或下文 OAuth 配置；`SSL_CERT` 与 `SSL_KEY`；`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`），详见下方「配置说明」。

使用 GHCR 预构建镜像时，在 `docker-compose.yml` 中注释掉 `build` 段、取消 `image: ghcr.io/rehtt/hamster-bin:latest` 注释，然后执行 `docker compose up -d`（无需 `--build`）。

## 配置说明

所有配置均通过环境变量提供。

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8080` | 服务监听端口 |
| `DB_DRIVER` | `sqlite` | 数据库类型，支持 `sqlite`、`mysql`、`postgres`（`postgresql` 会按 `postgres` 处理） |
| `DB_DSN` | 空 | 数据库连接串；MySQL/PostgreSQL 必填，SQLite 可选 |
| `DB_PATH` | `./data/inventory.db` | SQLite 数据库路径；仅在 `DB_DRIVER=sqlite` 且 `DB_DSN` 为空时使用 |
| `IMAGE_DIR` | `./data/images` | 上传图片存储目录 |
| `LOG_LEVEL` | `info` | 日志级别 |
| `SSL_CERT` | 空 | HTTPS 证书路径；需与 `SSL_KEY` 同时设置 |
| `SSL_KEY` | 空 | HTTPS 私钥路径；需与 `SSL_CERT` 同时设置 |
| `ADMIN_USERNAME` | 空 | 管理员用户名；与 `ADMIN_PASSWORD` 同时设置时启用登录鉴权 |
| `ADMIN_PASSWORD` | 空 | 管理员密码 |
| `JWT_SECRET` | 空 | JWT 签名及 OAuth 令牌加密的根密钥；启用鉴权时必填 |
| `JWT_EXPIRE_HOURS` | `168` | 本地登录有效期（小时）；OAuth 会话额外受授权 30 天上限限制 |
| `OAUTH_ENABLED` | `false` | 启用统一登录；可与管理员密码并存 |
| `OAUTH_ISSUER` | `https://oauth.home.rehtt.com` | OIDC Issuer，必须与发现文档精确一致，末尾不带 `/` |
| `OAUTH_CLIENT_ID` | 空 | OAuth 应用 ID，启用时必填 |
| `OAUTH_CLIENT_SECRET` | 空 | OAuth 客户端密钥，启用时必填，仅服务端使用 |
| `OAUTH_REDIRECT_URL` | 空 | 完整回调地址，路径固定为 `/api/v1/auth/oauth/callback`，启用时必填 |
| `OAUTH_AUTH_METHOD` | `client_secret_basic` | 客户端认证方式，可选 `client_secret_post`，须与应用注册一致 |
| `MCP_TOKEN` | 空 | 远程 MCP 的部署级 Token；空值关闭 `/mcp`，配置后持有者拥有全部 MCP 工具权限 |
| `MCP_ALLOWED_ORIGINS` | 空 | 逗号分隔的完整 Origin，例如 `https://client.example,http://localhost:3000`；默认仅同源或无 Origin 请求 |
| `TASK_WORKERS` | `2` | 后台任务并发数，正整数 |
| `TASK_TIMEOUT_SECONDS` | `120` | 单个任务从开始执行计时的超时秒数，正整数 |
| `LLM_BASE_URL` | 空 | OpenAI-compatible API base，例如 `https://api.openai.com/v1` |
| `LLM_API_KEY` | 空 | LLM API Key |
| `LLM_MODEL` | 空 | LLM 模型名称 |

默认 SQLite 无需额外配置。连接 MySQL 示例：

```bash
export DB_DRIVER=mysql
export DB_DSN='user:password@tcp(127.0.0.1:3306)/hamster_bin?charset=utf8mb4&parseTime=True&loc=Local'
./hamster-bin
```

连接 PostgreSQL 示例：

```bash
export DB_DRIVER=postgres
export DB_DSN='host=127.0.0.1 user=hamster password=secret dbname=hamster_bin port=5432 sslmode=disable TimeZone=Asia/Shanghai'
./hamster-bin
```

启用登录鉴权：

```bash
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD='your-strong-password'
export JWT_SECRET='random-32+-char-secret'
./hamster-bin
```

### 统一 OAuth / OIDC 登录

在统一登录管理端创建机密客户端应用，精确登记 `https://inventory.example.com/api/v1/auth/oauth/callback`（替换为本站地址），允许 `openid profile email offline_access`，并为用户分配本应用。本站配置示例：

```bash
export OAUTH_ENABLED=true
export OAUTH_ISSUER='https://oauth.home.rehtt.com'
export OAUTH_CLIENT_ID='replace-with-client-id'
# 安全设置并导出 OAUTH_CLIENT_SECRET，使用管理员交付的真实密钥。
export OAUTH_REDIRECT_URL='https://inventory.example.com/api/v1/auth/oauth/callback'
export OAUTH_AUTH_METHOD=client_secret_basic
export JWT_SECRET='replace-with-a-long-random-secret'
./hamster-bin
```

登录页会显示“使用统一账号登录”。同时设置管理员用户名和密码会保留密码登录；只启用 OAuth 时无需配置管理员凭据。已分配的统一登录用户共享同一份库存和全部操作权限，没有角色或个人库存隔离。首次授权需要确认离线访问；未允许 `offline_access` 或没有返回刷新令牌时，本站不会建立登录会话。

OAuth 身份按 `(iss, sub)` 关联，用户名或邮箱变化不会创建重复身份，也不会按未验证邮箱合并账号。令牌仅在服务端加密持久化，浏览器只保存应用自己的 HttpOnly 会话 Cookie。应用重启后会话可继续使用，但正在进行的授权或刷新需要重新登录。默认七天有效，最长不超过本次授权的 30 天，刷新不会延长本地会话期限；修改 `JWT_SECRET` 会使已有登录失效，也会使已保存的令牌无法解密。

如需同步退出，在 OAuth 管理端的应用设置中登记后端通道退出地址 `https://<库存域名>/api/v1/auth/oauth/backchannel-logout`，例如 `https://inventory.example.com/api/v1/auth/oauth/backchannel-logout`。此地址必须能从 OAuth 服务直接访问，不能依赖浏览器网络或登录 Cookie。用户在统一登录退出页面勾选“同步退出应用”后，本站验证签名通知并结束对应浏览器下的全部 OAuth 本地会话；其他设备、密码登录和 MCP 保持原有行为。页面下次请求收到 401 后跳回登录页，不额外轮询。通知只删除本站会话及保存的令牌，不额外撤销离线授权，也不增加本站主动退出统一登录的流程。

同步退出要求 ID Token 包含已验证的 `sid`；缺失时无法登录，刷新返回 ID Token 时也必须保持原 `sid`。升级会自动迁移会话字段并新增退出记录表，清理没有 `sid` 的旧 OAuth 会话，用户需重新登录。退出记录持久化保留 31 天，防止迟到的登录回调恢复已结束的会话。

通知接口只接收 `application/x-www-form-urlencoded` 请求体中的单个非空 `logout_token`（上限 64 KiB），无需 Cookie，不设置 Cookie，所有响应均为 `Cache-Control: no-store`。有效通知、重复通知和未知会话返回 200，非法请求或令牌返回 400；发现文档、公钥或数据库暂时故障返回 503，OAuth 服务应重试。OAuth 未启用时返回 404。

每次受保护请求都会通过 UserInfo 检查授权，访问令牌即将到期时自动串行刷新。停用用户、移除分配或撤销授权后，下次请求会退出登录；统一登录服务不可用或检查失败时同样拒绝访问并清除本地会话。密码登录和 MCP 不依赖 OAuth 在线检查。退出本站会清除本地会话并尝试撤销该次授权，不会退出统一登录站点或其他应用。

生产使用 HTTPS；反向代理终止 HTTPS 时，`OAUTH_REDIRECT_URL` 仍填写外部 HTTPS 地址，以便正确设置 Secure Cookie。应用与前端需通过同一站点访问。开发时可在管理端登记 `http://127.0.0.1:5173/api/v1/auth/oauth/callback`，浏览器访问 `http://127.0.0.1:5173`，由现有 Vite `/api` 代理转发到后端；使用嵌入式前端时改用后端实际地址。回调协议、域名、端口、路径必须与登记值完全一致，不支持附加查询参数。反向代理日志也应过滤回调查询、Cookie 和认证头。

### HTTPS

```bash
export SSL_CERT=/path/to/fullchain.pem
export SSL_KEY=/path/to/privkey.pem
./hamster-bin
```

### AI 辅助解析

```bash
export LLM_BASE_URL='https://api.openai.com/v1'
export LLM_API_KEY='sk-...'
export LLM_MODEL='gpt-4o-mini'
./hamster-bin
```

## 远程 MCP

设置 `MCP_TOKEN` 并启动或重启服务：

```bash
export MCP_TOKEN='replace-with-a-long-random-secret'
./hamster-bin
```

在支持自定义认证头的 MCP 客户端中填写：

```json
{
  "mcpServers": {
    "hamster-bin": {
      "type": "http",
      "url": "https://inventory.example.com/mcp",
      "headers": {
        "Authorization": "Bearer replace-with-a-long-random-secret"
      }
    }
  }
}
```

本地地址为 `http://localhost:8080/mcp`；客户端配置格式可能不同，连接类型选择 Streamable HTTP，填写相同 URL 和认证头即可。服务使用[官方 Go SDK v1.8.0](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0)，以无状态 HTTP 返回 JSON，由 SDK 处理发现、握手和协议版本协商。

| 能力 | 工具 |
| :--- | :--- |
| 元件 | `list_components`、`get_component`、`create_component`、`update_component` |
| 入出库 | `change_stock` |
| 预入库 | `list_pre_stocks`、`get_pre_stock`、`create_pre_stock`、`update_pre_stock`、`confirm_pre_stock` |
| 分类与供应商 | `list_categories`、`create_category`、`list_suppliers`、`create_supplier` |
| 流水与统计 | `list_stock_logs`、`get_component_stock_logs`、`get_stats` |

工具参数包含中文说明和读写标记，返回 `structuredContent` 及相同 JSON 的文本内容；结果包含 `data`，分页结果还包含 `pagination`。业务失败通过 MCP `isError` 返回。删除、撤销、批量操作、平台解析和后台任务不对 MCP 开放。

调用顺序示例（以下为 `tools/call` 的参数）：

```json
{"name":"create_category","arguments":{"name":"电阻"}}
{"name":"create_supplier","arguments":{"name":"嘉立创"}}
{"name":"create_component","arguments":{"name":"10k 贴片电阻","category_id":1,"supplier_id":1,"stock_quantity":100,"total_price_cents":250,"location":"A1"}}
{"name":"list_components","arguments":{"name":"电阻","value":"10k","page":1,"page_size":20}}
{"name":"update_component","arguments":{"id":1,"location":"B2"}}
{"name":"change_stock","arguments":{"id":1,"amount":-5,"reason":"项目领用"}}
```

实际使用前两次调用返回的分类、供应商 ID，以及创建元件返回的 ID。创建元件必须传 `name`、`category_id`、非负 `stock_quantity`；创建预入库改传非负 `expected_quantity`。采购总价 `total_price_cents` 使用整数分，返回单价 `unit_price_micro` 使用微元（1 元 = 1,000,000 微元）；数量为零时不得填写正数总价。

编辑仅更新传入字段，省略字段保持原值；空字符串清除文本，`supplier_id: 0` 解除供应商关联。元件编辑不接受库存数量或参考单价。库存通过 `change_stock` 的 `amount` 修改，正数入库、负数出库，禁止为零；入库可填写非负总价，出库成本自动计算。预入库确认会创建正式元件及现有规则要求的流水，已确认记录不可再次确认或编辑。

分页默认第 1 页、每页 20 条，每页上限 100；单元件流水默认最近 50 条，上限 100。元件搜索与 REST 使用相同的分字段、空格分词和排序规则；预入库默认 `pending`（可选 `confirmed`、`all`），统计默认 `month`（可选 `quarter`、`all`）。

MCP Token 与 Web 登录 Cookie 独立；Web 登录不能代替 MCP Token，MCP Token 也不能用于 REST 登录。Token 持有者拥有上述全部工具权限，通过修改环境变量并重启服务轮换。公网连接使用 HTTPS。浏览器跨源连接时，设置 `MCP_ALLOWED_ORIGINS` 为客户端的完整 Origin（协议、主机和可选端口，不带路径、不支持通配符）；MCP 独立校验 Origin，默认拒绝其他跨源请求。无 Origin 的非浏览器客户端无需配置白名单。反向代理终止 HTTPS 时，可将外部浏览器 Origin 加入白名单；代理到本机回环地址时，应将上游 Host 设置为本机地址，以兼容 SDK 的 DNS 重绑定保护。

## 常用命令

```bash
make help            # 查看可用命令
make run             # 运行后端开发服务
make build           # 构建当前平台二进制
make build-frontend  # 构建前端
make build-all       # 构建 Linux、Windows、macOS 发布文件
```

前端命令：

```bash
cd web
npm run dev      # 开发服务
npm run build    # 类型检查并构建
npm run lint     # ESLint 检查
npm run test:auth # 登录入口、路由保护与错误展示渲染测试
npm run preview  # 预览构建结果
```

后端测试：

```bash
go test . ./cmd/... ./internal/...
```

## API

后端 API 前缀为 `/api/v1`。主要资源包括：

- `/api/v1/auth/*`：登录方式配置（`GET /config`）、密码登录、OAuth 发起/回调（`GET /oauth/login`、`GET /oauth/callback`）、后端通道同步退出（`POST /oauth/backchannel-logout`）、退出登录和当前用户状态（含 `auth_method`）。
- `/api/v1/categories`：分类管理。
- `/api/v1/suppliers`：供应商管理。
- `/api/v1/components`：元件列表、创建、更新、删除、导出和库存操作。
- `/api/v1/stock-logs`：库存流水查询与撤销。
- `/api/v1/stats`：仪表盘统计数据。
- `/api/v1/tasks`：后台任务提交（POST，返回 202）、照片 multipart 提交（`/image`）、分页查询（GET，支持 `status`、`type`）；`/:id` 查询详情，`/:id/image` 获取任务照片，`/:id/cancel` 取消，`/:id/retry` 新建重试任务（POST，返回 202）。
- `/api/v1/platforms`：可用解析平台。

任务提交示例：`{"type":"component_parse","input":{"code":"C2040","use_llm":false},"source":"components"}`。扫码使用 `qrcode_parse` 和 `input.qrcode_data`；照片使用 `/api/v1/tasks/image` 上传 JPEG/PNG（不超过 10 MiB），复用 `LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`，模型需支持图片输入；`source` 可为 `components` 或 `pre-stocks`，编辑记录可带 `record_id`。旧 `/components/parse`、`/components/parse-qrcode` 仍保持同步接口。

任务与业务数据使用同一数据库，服务重启后继续处理排队任务，运行中任务标记为“服务中断”并允许手动重试。当前按单服务实例运行，不支持多实例调度，不自动重试、删除或清理历史。照片核对弹窗在提交或解析期间均可关闭，任务继续后台执行，重新打开从拍摄/选图开始。任务中心“应用结果”会重新读取最新编辑记录，照片结果先进入核对弹窗，确认后才填入表单；记录已删除或预入库已确认时，可选择作为新增表单使用。刷新后可取回解析结果，但不恢复未保存的完整表单草稿。

## 数据与文件

- 默认数据库：`data/inventory.db`
- 默认图片目录：`data/images`
- 前端构建产物：`web/dist`
- 运行时数据和构建产物不应提交到仓库。

## 发布

项目在推送 `v*` 格式的 Git tag 时触发 GitHub Actions 发布流程。发布流程会安装前端依赖、构建 `web/dist`、运行后端测试，并生成 Linux、Windows 和 macOS 的二进制文件；同时构建多架构 Docker 镜像并推送到 `ghcr.io/rehtt/hamster-bin`（tag 为版本号与 `latest`）。

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 贡献

欢迎提交 Issue 和 Pull Request。建议在提交前运行：

```bash
go test . ./cmd/... ./internal/...
cd web
npm run lint
npm run build
```

参与贡献时请尽量保持改动聚焦，说明问题背景、复现方式和验证结果。如果变更涉及 API、数据模型、配置、构建流程或前端主要行为，请同步更新相关文档。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。
