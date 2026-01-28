package main

import (
	"fmt"
	"log"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/database"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/router"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化解析器管理器
	parserManager := parser.NewParserManager()
	parserManager.Register(parser.NewLCSCParser())   // 注册立创商城解析器
	parserManager.Register(parser.NewTaobaoParser()) // 注册淘宝解析器（示例）

	// 设置路由
	r := router.Setup(database.GetDB(), cfg.WebDir, parserManager)

	// 启动服务器
	addr := ":" + cfg.Port
	fmt.Printf("\n🚀 电子元件库存管理系统启动成功！\n")
	fmt.Printf("📡 API地址: http://localhost%s/api/v1\n", addr)
	fmt.Printf("🌐 Web地址: http://localhost%s\n", addr)
	fmt.Printf("💾 数据库: %s\n", cfg.DBPath)
	fmt.Printf("🔌 支持平台: %v\n\n", parserManager.GetAvailableParsers())

	// if err := r.Run(addr); err != nil {
	// 	log.Fatalf("服务器启动失败: %v", err)
	// }
	if err := r.RunTLS(addr, "certs/cert.pem", "certs/key.pem"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
