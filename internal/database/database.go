package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Rehtt/hamster-bin/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库连接
func Init(dbPath string) error {
	// 确保数据目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 打开数据库连接
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	DB.Exec("pragma journal_mode = wal")
	DB.Exec("pragma synchronous = normal")
	DB.Exec("pragma temp_store = memory")
	DB.Exec("pragma auto_vacuum = INCREMENTAL")

	// 自动迁移表结构
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	log.Println("数据库初始化成功")
	return nil
}

// autoMigrate 自动创建/更新表结构
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.Category{},
		&models.Component{},
		&models.StockLog{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}
