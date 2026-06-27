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
- 可选 AI 辅助解析：配置 OpenAI-compatible API 后，可辅助解析元件参数。
- 图片与资料：支持元件图片上传、Datasheet 链接和描述信息。
- 可选登录鉴权：通过环境变量启用单管理员登录，使用 HttpOnly Cookie 保存 JWT。
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

启用鉴权、HTTPS 或 LLM 辅助解析时，在 `.env` 中设置对应变量（`ADMIN_USERNAME`、`ADMIN_PASSWORD`、`JWT_SECRET`；`SSL_CERT` 与 `SSL_KEY`；`LLM_BASE_URL`、`LLM_API_KEY`、`LLM_MODEL`），详见下方「配置说明」。

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
| `JWT_SECRET` | 空 | JWT 签名密钥；启用鉴权时必填 |
| `JWT_EXPIRE_HOURS` | `168` | JWT 有效期，单位为小时 |
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

启用 HTTPS：

```bash
export SSL_CERT=/path/to/fullchain.pem
export SSL_KEY=/path/to/privkey.pem
./hamster-bin
```

启用 AI 辅助解析：

```bash
export LLM_BASE_URL='https://api.openai.com/v1'
export LLM_API_KEY='sk-...'
export LLM_MODEL='gpt-4o-mini'
./hamster-bin
```

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
npm run preview  # 预览构建结果
```

后端测试：

```bash
go test . ./cmd/... ./internal/...
```

## API

后端 API 前缀为 `/api/v1`。主要资源包括：

- `/api/v1/auth/*`：登录、退出登录和当前用户状态。
- `/api/v1/categories`：分类管理。
- `/api/v1/suppliers`：供应商管理。
- `/api/v1/components`：元件列表、创建、更新、删除、导出和库存操作。
- `/api/v1/stock-logs`：库存流水查询与撤销。
- `/api/v1/stats`：仪表盘统计数据。
- `/api/v1/platforms`：可用解析平台。

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
