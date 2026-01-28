# 电子元件库存管理系统 Makefile

# 变量定义
BINARY_NAME=hamster-bin
GO=go
GOFLAGS=-v
BUILD_DIR=.
CMD_DIR=cmd/server
DATA_DIR=data
WEB_DIR=web

# 默认目标
.DEFAULT_GOAL := help

# 颜色定义（用于输出）
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

##@ 基础命令

.PHONY: help
help: ## 显示帮助信息
	@echo "$(COLOR_BOLD)电子元件库存管理系统 - 可用命令:$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-15s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: build
build: ## 编译项目
	@echo "$(COLOR_GREEN)>>> 正在编译项目...$(COLOR_RESET)"
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "$(COLOR_GREEN)>>> 编译完成: $(BINARY_NAME)$(COLOR_RESET)"

.PHONY: run
run: ## 运行项目
	@echo "$(COLOR_GREEN)>>> 启动服务器...$(COLOR_RESET)"
	@$(GO) run $(CMD_DIR)/main.go

.PHONY: start
start: build ## 编译并运行
	@echo "$(COLOR_GREEN)>>> 启动服务器...$(COLOR_RESET)"
	@./$(BINARY_NAME)

.PHONY: build-frontend
build-frontend: ## 构建前端页面
	@echo "$(COLOR_GREEN)>>> 构建前端页面...$(COLOR_RESET)"
	@cd $(WEB_DIR) && npm run build

##@ 构建命令

.PHONY: build-linux
build-linux: ## 编译 Linux 版本
	@echo "$(COLOR_GREEN)>>> 编译 Linux 版本...$(COLOR_RESET)"
	@GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go
	@echo "$(COLOR_GREEN)>>> 编译完成: $(BINARY_NAME)-linux-amd64$(COLOR_RESET)"

.PHONY: build-windows
build-windows: ## 编译 Windows 版本
	@echo "$(COLOR_GREEN)>>> 编译 Windows 版本...$(COLOR_RESET)"
	@GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)/main.go
	@echo "$(COLOR_GREEN)>>> 编译完成: $(BINARY_NAME)-windows-amd64.exe$(COLOR_RESET)"

.PHONY: build-mac
build-mac: ## 编译 macOS 版本
	@echo "$(COLOR_GREEN)>>> 编译 macOS 版本...$(COLOR_RESET)"
	@GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)/main.go
	@GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)/main.go
	@echo "$(COLOR_GREEN)>>> 编译完成: $(BINARY_NAME)-darwin-amd64, $(BINARY_NAME)-darwin-arm64$(COLOR_RESET)"

.PHONY: build-all
build-all: build-linux build-windows build-mac ## 编译所有平台版本
	@echo "$(COLOR_GREEN)>>> 所有平台编译完成$(COLOR_RESET)"

##@ 信息命令

.PHONY: info
info: ## 显示项目信息
	@echo "$(COLOR_BOLD)项目信息:$(COLOR_RESET)"
	@echo "  项目名称: 电子元件库存管理系统"
	@echo "  二进制名: $(BINARY_NAME)"
	@echo "  Go 版本:  $$($(GO) version)"
	@echo "  构建目录: $(BUILD_DIR)"
	@echo "  数据目录: $(DATA_DIR)"
	@echo "  Web 目录: $(WEB_DIR)"

