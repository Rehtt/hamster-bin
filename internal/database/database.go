package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Config 数据库连接配置。
type Config struct {
	Driver string
	DSN    string
	Path   string
}

// Init 初始化数据库连接
func Init(cfg Config) error {
	dialector, err := openDialector(cfg)
	if err != nil {
		return err
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	if normalizeDriver(cfg.Driver) == "sqlite" {
		setSQLitePragmas(DB)
	}

	// 自动迁移表结构
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	log.Println("数据库初始化成功")
	return nil
}

func openDialector(cfg Config) (gorm.Dialector, error) {
	switch normalizeDriver(cfg.Driver) {
	case "sqlite":
		dbPath := cfg.Path
		if strings.TrimSpace(cfg.DSN) != "" {
			dbPath = cfg.DSN
		}
		if strings.TrimSpace(dbPath) == "" {
			return nil, fmt.Errorf("SQLite 数据库路径为空")
		}
		if err := ensureSQLiteDir(dbPath); err != nil {
			return nil, err
		}
		return sqlite.Open(dbPath), nil
	case "mysql":
		return mysql.Open(cfg.DSN), nil
	case "postgres":
		return postgres.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Driver)
	}
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite3":
		return "sqlite"
	case "postgresql":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func ensureSQLiteDir(dbPath string) error {
	if dbPath == ":memory:" || strings.HasPrefix(dbPath, "file:") {
		return nil
	}

	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	return nil
}

func setSQLitePragmas(db *gorm.DB) {
	db.Exec("pragma journal_mode = wal")
	db.Exec("pragma synchronous = normal")
	db.Exec("pragma temp_store = memory")
	db.Exec("pragma auto_vacuum = INCREMENTAL")
	db.Exec("pragma wal_autocheckpoint = 1000")
	db.Exec("pragma wal_checkpoint(PASSIVE)")
}

// autoMigrate 自动创建/更新表结构
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.Category{},
		&models.Supplier{},
		&models.Component{},
		&models.PreStock{},
		&models.StockLog{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}
