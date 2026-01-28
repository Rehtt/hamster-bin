# Hamster Bin (电子元件库存管理系统)

Hamster Bin 是一个现代化的电子元件库存管理系统，专为电子爱好者和工程师设计。它旨在简化元器件的管理、查找和库存跟踪过程。

## ✨ 主要功能

- **📦 库存管理**：轻松添加、修改和删除元器件信息。
- **🔍 自动解析**：
  - **立创商城 (LCSC)**：只需输入商品编号（如 `C123456`），即可自动抓取元器件的详细参数、图片和数据手册。
  - **淘宝**：支持解析淘宝链接（开发中）。
  - **二维码**：支持扫码快速录入或查找元件。
- **🗂 分类系统**：支持多级分类，方便组织不同类型的元件。
- **📊 库存记录**：详细记录每一次入库和出库的操作及原因，库存变动有迹可循。
- **🖼 图片与资料**：直接关联元件实物图片和 Datasheet 链接。
- **💻 现代化界面**：基于 React 和 Tailwind CSS 构建的响应式 Web 界面，简洁美观。

## 🛠 技术栈

### 后端 (Backend)
- **Language**: Go (Golang)
- **Web Framework**: [Gin](https://github.com/gin-gonic/gin)
- **ORM**: [GORM](https://gorm.io/)
- **Database**: SQLite (默认)
- **Parsing**: [GoQuery](https://github.com/PuerkitoBio/goquery) (用于网页数据抓取)

### 前端 (Frontend)
- **Framework**: React 19
- **Build Tool**: Vite
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
- **HTTP Client**: Axios

## 🚀 快速开始

### 环境要求
- Go 1.25.6+
- Node.js 22.17.1+ & npm/pnpm

### 1. 获取代码
```bash
git clone https://github.com/Rehtt/hamster-bin.git
cd hamster-bin
```

### 2. 启动后端
```bash
# 下载依赖
go mod download

# 运行服务
go run cmd/server/main.go
```
后端默认运行在 `http://localhost:8080`。首次运行时会自动在 `data/` 目录下初始化 SQLite 数据库。

### 3. 启动前端
```bash
cd web

# 安装依赖
npm install

# 开发模式运行
npm run dev
```
前端开发服务器通常运行在 `http://localhost:5173`。

### 4. 构建部署 (单文件部署)
本项目支持将前端静态资源嵌入到 Go 二进制文件中，实现单文件部署。

1. **构建前端**：
   ```bash
   cd web
   npm run build
   cd ..
   ```
   *注意：构建产物必须位于 `web/dist`，因为 `embed.go` 引用了此路径。*

2. **构建后端**：
   ```bash
   go build -o hamster-bin cmd/server/main.go
   ```

3. **运行**：
   生成的 `hamster-bin` 文件是一个独立的可执行文件，包含了所有前端资源。
   ```bash
   ./hamster-bin
   ```

## ⚙️ 配置说明

可以通过环境变量自定义配置：

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8080` | 服务监听端口 |
| `DB_PATH` | `./data/inventory.db` | SQLite 数据库路径 |
| `WEB_DIR` | `./web/dist` | 前端构建产物目录 |
| `IMAGE_DIR` | `./data/images` | 图片存储目录 |
| `LOG_LEVEL` | `info` | 日志级别 |

## 🤝 贡献
欢迎提交 Issue 和 Pull Request！

## 📄 许可证
[MIT License](LICENSE) (待添加)
