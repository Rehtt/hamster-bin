package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/Rehtt/hamster-bin/internal/tasks"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	if err := database.Init(database.Config{
		Driver: cfg.DBDriver,
		DSN:    cfg.DBDSN,
		Path:   cfg.DBPath,
	}); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化解析器管理器
	parserManager := parser.NewParserManager()
	llmClient := llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	parserManager.Register(parser.NewLCSCParser(llmClient)) // 注册立创商城解析器
	parserManager.Register(parser.NewTaobaoParser())        // 注册淘宝解析器（示例）

	// 设置路由
	imageParser := parser.NewImageParser(llmClient)
	taskService := tasks.New(repository.NewTaskRepository(database.GetDB()), parserManager, cfg.TaskWorkers, time.Duration(cfg.TaskTimeoutSeconds)*time.Second,
		tasks.WithImages(imageParser, tasks.NewImageStore(cfg.ImageDir)))
	if err := taskService.Start(); err != nil {
		log.Fatalf("任务服务启动失败: %v", err)
	}
	defer taskService.Stop()
	r := router.Setup(database.GetDB(), parserManager, cfg, taskService)

	// 启动服务器
	addr := ":" + cfg.Port
	fmt.Printf("\n🚀 电子元件库存管理系统启动成功！\n")
	fmt.Printf("🏷️  版本: %s\n", version.Version)
	fmt.Printf("📡 API地址: http://localhost%s/api/v1\n", addr)
	fmt.Printf("🌐 Web地址: http://localhost%s\n", addr)
	fmt.Printf("💾 数据库: %s (%s)\n", cfg.DBDriver, cfg.DatabaseDisplay())
	fmt.Printf("🔌 支持平台: %v\n\n", parserManager.GetAvailableParsers())

	server := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	signals, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	serverError := make(chan error, 1)
	go func() {
		if cfg.IsHTTPS() {
			serverError <- server.ListenAndServeTLS(cfg.SSLCert, cfg.SSLKey)
		} else {
			serverError <- server.ListenAndServe()
		}
	}()
	select {
	case <-signals.Done():
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("服务器退出: %v", err)
		}
	}
	// 先停止领取并中断任务；运行中的任务记录为服务中断，排队任务留待下次启动。
	taskService.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 HTTP 服务失败: %v", err)
	}
}
