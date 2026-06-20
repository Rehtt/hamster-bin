package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/database"
	"github.com/Rehtt/hamster-bin/internal/llm"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/router"
	"github.com/Rehtt/hamster-bin/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.Version)
		return
	}

	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化解析器管理器
	parserManager := parser.NewParserManager()
	llmClient := llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	parserManager.Register(parser.NewLCSCParser(llmClient)) // 注册立创商城解析器
	parserManager.Register(parser.NewTaobaoParser())        // 注册淘宝解析器（示例）

	// 设置路由
	r := router.Setup(database.GetDB(), parserManager, cfg)

	// 启动服务器
	addr := ":" + cfg.Port
	fmt.Printf("\n🚀 电子元件库存管理系统启动成功！\n")
	fmt.Printf("🏷️  版本: %s\n", version.Version)
	fmt.Printf("📡 API地址: http://localhost%s/api/v1\n", addr)
	fmt.Printf("🌐 Web地址: http://localhost%s\n", addr)
	fmt.Printf("💾 数据库: %s\n", cfg.DBPath)
	fmt.Printf("🔌 支持平台: %v\n\n", parserManager.GetAvailableParsers())

	if cfg.SSLCert != "" && cfg.SSLKey != "" {
		fmt.Printf("🔒 启用HTTPS服务\n")
		if err := r.RunTLS(addr, cfg.SSLCert, cfg.SSLKey); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	} else {
		if err := r.Run(addr); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	}
}
