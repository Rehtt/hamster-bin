package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOAuthMigrationInvalidatesLegacySessions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "upgrade.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	// 升级前的会话表没有提供方 sid 或目标索引。
	if err := db.Exec(`CREATE TABLE oauth_sessions (
		id text PRIMARY KEY, identity_id integer NOT NULL, client_id text NOT NULL,
		tokens blob NOT NULL, access_expires_at datetime NOT NULL, expires_at datetime NOT NULL,
		refreshing numeric NOT NULL, created_at datetime, updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO oauth_sessions (id,identity_id,client_id,tokens,access_expires_at,expires_at,refreshing) VALUES (?,?,?,?,?,?,?)", "legacy", 1, "client", []byte("old-encrypted-tokens"), time.Now().Add(time.Hour), time.Now().Add(time.Hour), false).Error; err != nil {
		t.Fatal(err)
	}
	previous := DB
	DB = db
	defer func() { DB = previous }()
	if err := autoMigrate(); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&models.OAuthSession{}, "provider_sid") || !db.Migrator().HasIndex(&models.OAuthSession{}, "idx_oauth_sessions_target_key") || !db.Migrator().HasTable(&models.OAuthLogout{}) {
		t.Fatal("migration missing logout schema")
	}
	if err := repository.NewOAuthRepository(db).Cleanup(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.OAuthSession{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("legacy OAuth session survived upgrade")
	}
	// 重复迁移和启动清理应保持幂等。
	if err := autoMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := repository.NewOAuthRepository(db).Cleanup(context.Background(), true); err != nil {
		t.Fatal(err)
	}
}
