package repository

import (
	"fmt"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DeleteSelected       = "selected"
	DeleteFailed         = "failed"
	DeleteAll            = "all"
	DeleteOlderThan7Days = "older_than_7_days"
)

func TaskTerminal(status string) bool {
	return status == TaskSucceeded || status == TaskFailed || status == TaskCancelled
}

// Delete 在同一事务中读取候选、限制终态并硬删除。single 保留单条接口的 404/409 语义。
// 调用方串行化提交、重试与删除，事务失败时不得清理照片。
func (r *TaskRepository) Delete(mode string, ids []uint, cutoff time.Time, single bool) ([]models.Task, int, error) {
	deleted := make([]models.Task, 0)
	skipped := 0
	err := r.db.Transaction(func(tx *gorm.DB) error {
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"})
		switch mode {
		case DeleteSelected:
			q = q.Where("id IN ?", ids)
		case DeleteFailed:
			q = q.Where("status = ?", TaskFailed)
		case DeleteAll:
		case DeleteOlderThan7Days:
			q = q.Where("created_at < ?", cutoff)
		default:
			return fmt.Errorf("无效的删除模式")
		}
		var candidates []models.Task
		if err := q.Order("id ASC").Find(&candidates).Error; err != nil {
			return err
		}
		if single && len(candidates) == 0 {
			return gorm.ErrRecordNotFound
		}
		if mode == DeleteSelected {
			skipped = len(ids) - len(candidates)
		}
		for _, task := range candidates {
			if !TaskTerminal(task.Status) {
				if single {
					return ErrTaskState
				}
				skipped++
				continue
			}
			deleted = append(deleted, task)
		}
		// 为 SQLite 默认参数上限留出余量，所有批次仍在同一事务内。
		for start := 0; start < len(deleted); start += 400 {
			end := min(start+400, len(deleted))
			batch := make([]uint, 0, end-start)
			for _, task := range deleted[start:end] {
				batch = append(batch, task.ID)
			}
			result := tx.Where("id IN ? AND status IN ?", batch, []string{TaskSucceeded, TaskFailed, TaskCancelled}).Delete(&models.Task{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != int64(len(batch)) {
				return ErrTaskState
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return deleted, skipped, nil
}

// ImageInputs 避免依赖特定数据库的 JSON 查询语法。
func (r *TaskRepository) ImageInputs() ([]models.Task, error) {
	var items []models.Task
	err := r.db.Select("id", "input").Where("type = ?", "image_parse").Find(&items).Error
	return items, err
}
