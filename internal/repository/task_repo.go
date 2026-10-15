package repository

import (
	"errors"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

const (
	TaskQueued    = "queued"
	TaskRunning   = "running"
	TaskSucceeded = "succeeded"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

var ErrTaskState = errors.New("任务当前状态不允许此操作")

func ValidTaskStatus(s string) bool {
	return s == TaskQueued || s == TaskRunning || s == TaskSucceeded || s == TaskFailed || s == TaskCancelled
}

type TaskRepository struct{ db *gorm.DB }

func NewTaskRepository(db *gorm.DB) *TaskRepository      { return &TaskRepository{db: db} }
func (r *TaskRepository) Create(task *models.Task) error { return r.db.Create(task).Error }
func (r *TaskRepository) Get(id uint) (*models.Task, error) {
	var t models.Task
	err := r.db.First(&t, id).Error
	return &t, err
}
func (r *TaskRepository) List(status, kind string, page, size int) ([]models.Task, int64, error) {
	q := r.db.Model(&models.Task{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if kind != "" {
		q = q.Where("type = ?", kind)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.Task, 0)
	err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

// Claim 使用条件更新领取，多个 worker 只能成功领取一次。
func (r *TaskRepository) Claim() (*models.Task, error) {
	for {
		var t models.Task
		candidate := r.db.Where("status = ?", TaskQueued).Order("created_at ASC, id ASC").Limit(1).Find(&t)
		if candidate.Error != nil {
			return nil, candidate.Error
		}
		if candidate.RowsAffected == 0 {
			return nil, gorm.ErrRecordNotFound
		}
		now := time.Now()
		result := r.db.Model(&models.Task{}).Where("id = ? AND status = ?", t.ID, TaskQueued).Updates(map[string]any{"status": TaskRunning, "started_at": now})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			t.Status = TaskRunning
			t.StartedAt = &now
			return &t, nil
		}
	}
}
func (r *TaskRepository) Finish(id uint, status string, result models.JSONText, message, code string) error {
	return r.db.Model(&models.Task{}).Where("id = ? AND status = ?", id, TaskRunning).Updates(map[string]any{"status": status, "result": result, "error": message, "error_code": code, "finished_at": time.Now()}).Error
}
func (r *TaskRepository) Cancel(id uint) error {
	result := r.db.Model(&models.Task{}).Where("id = ? AND status IN ?", id, []string{TaskQueued, TaskRunning}).Updates(map[string]any{"status": TaskCancelled, "finished_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := r.Get(id); err != nil {
			return err
		}
		return ErrTaskState
	}
	return nil
}
func (r *TaskRepository) Recover() error {
	return r.db.Model(&models.Task{}).Where("status = ?", TaskRunning).Updates(map[string]any{"status": TaskFailed, "error": "服务中断，请重试", "error_code": "interrupted", "finished_at": time.Now()}).Error
}
