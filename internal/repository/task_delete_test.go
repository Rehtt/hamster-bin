package repository

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func deleteRepo(t *testing.T) (*TaskRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "delete.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Task{}); err != nil {
		t.Fatal(err)
	}
	return NewTaskRepository(db), db
}

func TestTaskDeleteModesAndTimeBoundary(t *testing.T) {
	cutoff := time.Now().UTC().Truncate(time.Second).Add(-168 * time.Hour)
	for _, mode := range []string{DeleteSelected, DeleteFailed, DeleteAll, DeleteOlderThan7Days} {
		t.Run(mode, func(t *testing.T) {
			repo, _ := deleteRepo(t)
			// 每个状态都有早于、恰好等于和晚于边界的任务。
			states := []string{TaskQueued, TaskRunning, TaskSucceeded, TaskFailed, TaskCancelled}
			var ids []uint
			for _, state := range states {
				for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
					task := &models.Task{Type: "component_parse", Status: state, Input: models.JSONText(`{}`), CreatedAt: cutoff.Add(offset)}
					if err := repo.Create(task); err != nil {
						t.Fatal(err)
					}
					ids = append(ids, task.ID)
				}
			}
			ids = append(ids, 9999)
			deleted, skipped, err := repo.Delete(mode, ids, cutoff, false)
			if err != nil {
				t.Fatal(err)
			}
			wantDeleted, wantSkipped := 9, 6
			switch mode {
			case DeleteSelected:
				wantSkipped = 7
			case DeleteFailed:
				wantDeleted, wantSkipped = 3, 0
			case DeleteOlderThan7Days:
				wantDeleted, wantSkipped = 3, 2
			}
			if len(deleted) != wantDeleted || skipped != wantSkipped {
				t.Fatalf("deleted=%d skipped=%d", len(deleted), skipped)
			}
			for _, task := range deleted {
				if !TaskTerminal(task.Status) || mode == DeleteOlderThan7Days && !task.CreatedAt.Before(cutoff) {
					t.Fatalf("invalid delete: %+v", task)
				}
				if _, err := repo.Get(task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("not hard deleted: %v", err)
				}
			}
		})
	}
}

func TestTaskDeleteChunksRollbackAndLateFinish(t *testing.T) {
	repo, db := deleteRepo(t)
	// 超过列表分页和 SQLite 传统参数上限；跨批次失败必须全部回滚。
	for range 1005 {
		if err := repo.Create(&models.Task{Type: "component_parse", Input: models.JSONText(`{}`), Status: TaskSucceeded}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`CREATE TRIGGER fail_task_delete BEFORE DELETE ON tasks WHEN OLD.id = 850 BEGIN SELECT RAISE(ABORT, 'test rollback'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Delete(DeleteAll, nil, time.Now(), false); err == nil {
		t.Fatal("expected rollback")
	}
	_, total, err := repo.List("", "", 1, 20)
	if err != nil || total != 1005 {
		t.Fatalf("rollback total=%d err=%v", total, err)
	}
	if err := db.Exec("DROP TRIGGER fail_task_delete").Error; err != nil {
		t.Fatal(err)
	}
	deleted, skipped, err := repo.Delete(DeleteAll, nil, time.Now(), false)
	if err != nil || len(deleted) != 1005 || skipped != 0 {
		t.Fatalf("deleted=%d skipped=%d err=%v", len(deleted), skipped, err)
	}
	if err := repo.Finish(1, TaskSucceeded, models.JSONText(`{"late":true}`), "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("late finish recreated task: %v", err)
	}
	deleted, skipped, err = repo.Delete(DeleteAll, nil, time.Now(), false)
	if err != nil || len(deleted) != 0 || skipped != 0 {
		t.Fatal("empty delete must succeed")
	}
}

func TestTaskDeleteSingleState(t *testing.T) {
	repo, _ := deleteRepo(t)
	for _, status := range []string{TaskQueued, TaskRunning, TaskSucceeded, TaskFailed, TaskCancelled} {
		task := &models.Task{Type: "component_parse", Input: models.JSONText(`{}`), Status: status}
		if err := repo.Create(task); err != nil {
			t.Fatal(err)
		}
		_, _, err := repo.Delete(DeleteSelected, []uint{task.ID}, time.Now(), true)
		if TaskTerminal(status) && err != nil || !TaskTerminal(status) && !errors.Is(err, ErrTaskState) {
			t.Fatalf("status=%s err=%v", status, err)
		}
	}
	if _, _, err := repo.Delete(DeleteSelected, []uint{9999}, time.Now(), true); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing: %v", err)
	}
}
