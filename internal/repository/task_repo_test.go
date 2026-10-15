package repository

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTaskPersistenceAndAtomicClaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persistent.db")
	open := func() *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		sqlDB.SetMaxOpenConns(1)
		return db
	}
	db := open()
	if err := db.AutoMigrate(&models.Task{}); err != nil {
		t.Fatal(err)
	}
	repo := NewTaskRepository(db)
	for range 20 {
		if err := repo.Create(&models.Task{Type: "component_parse", Status: TaskQueued, Input: models.JSONText(`{"code":"C123"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	// 关闭并重新连接，队列内容不依赖进程内状态。
	db = open()
	sqlDB, _ = db.DB()
	defer sqlDB.Close()
	repo = NewTaskRepository(db)
	items, total, err := repo.List(TaskQueued, "component_parse", 1, 5)
	if err != nil || total != 20 || len(items) != 5 {
		t.Fatalf("restored list total=%d len=%d err=%v", total, len(items), err)
	}
	oldest, err := repo.Claim()
	if err != nil || oldest.ID != 1 {
		t.Fatalf("FIFO claim: %+v %v", oldest, err)
	}
	var wg sync.WaitGroup
	claimed := make(chan uint, 20)
	failures := make(chan error, 4)
	for range 4 {
		wg.Go(func() {
			independent := NewTaskRepository(db)
			for {
				task, err := independent.Claim()
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return
				}
				if err != nil {
					failures <- err
					return
				}
				claimed <- task.ID
			}
		})
	}
	wg.Wait()
	close(claimed)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	seen := map[uint]bool{oldest.ID: true}
	for id := range claimed {
		if seen[id] {
			t.Fatalf("duplicate claim %d", id)
		}
		seen[id] = true
	}
	if len(seen) != 20 {
		t.Fatalf("claimed %d tasks", len(seen))
	}
}
