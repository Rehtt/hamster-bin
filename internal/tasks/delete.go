package tasks

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
)

type DeleteRequest struct {
	Mode string `json:"mode"`
	IDs  []uint `json:"ids,omitempty"`
}

type DeleteResult struct {
	DeletedIDs              []uint `json:"deleted_ids"`
	DeletedCount            int    `json:"deleted_count"`
	SkippedCount            int    `json:"skipped_count"`
	ImageCleanupFailedCount int    `json:"image_cleanup_failed_count"`
}

func (req DeleteRequest) Validate() error {
	switch req.Mode {
	case repository.DeleteSelected:
		if len(req.IDs) < 1 || len(req.IDs) > 100 {
			return errors.New("请选择 1–100 个任务 ID")
		}
		for _, id := range req.IDs {
			if id == 0 || uint64(id) > 1<<32-1 {
				return errors.New("无效的任务 ID")
			}
		}
	case repository.DeleteFailed, repository.DeleteAll, repository.DeleteOlderThan7Days:
		if req.IDs != nil {
			return errors.New("仅 selected 模式接受 ids")
		}
	default:
		return errors.New("无效的删除模式")
	}
	return nil
}

func (s *Service) DeleteOne(id uint) (*DeleteResult, error) {
	return s.delete(DeleteRequest{Mode: repository.DeleteSelected, IDs: []uint{id}}, true)
}
func (s *Service) Delete(req DeleteRequest) (*DeleteResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return s.delete(req, false)
}
func (s *Service) delete(req DeleteRequest, single bool) (*DeleteResult, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	ids := make([]uint, 0, len(req.IDs))
	seen := make(map[uint]bool)
	for _, id := range req.IDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	deleted, skipped, err := s.Repo.Delete(req.Mode, ids, time.Now().Add(-168*time.Hour), single)
	if err != nil {
		return nil, err
	}
	result := &DeleteResult{DeletedIDs: make([]uint, 0, len(deleted)), DeletedCount: len(deleted), SkippedCount: skipped}
	for _, task := range deleted {
		result.DeletedIDs = append(result.DeletedIDs, task.ID)
	}
	result.ImageCleanupFailedCount = s.cleanupImages(deleted)
	return result, nil
}

func imageKey(task models.Task) (string, error) {
	var input Input
	if err := json.Unmarshal(task.Input, &input); err != nil {
		return "", err
	}
	if input.Image == nil || !imageKeyPattern.MatchString(input.Image.Key) {
		return "", errors.New("无效的任务照片引用")
	}
	return input.Image.Key, nil
}

// cleanupImages 只在事务提交后、生命周期锁内运行。引用无法确认时保留原图。
func (s *Service) cleanupImages(deleted []models.Task) int {
	keys := make(map[string]bool)
	failed := 0
	for _, task := range deleted {
		if task.Type != ImageParse {
			continue
		}
		key, err := imageKey(task)
		if err != nil {
			failed++
			log.Printf("删除任务 %d 后无法确认照片引用: %v", task.ID, err)
			continue
		}
		keys[key] = true
	}
	if len(keys) == 0 {
		return failed
	}
	remaining, err := s.Repo.ImageInputs()
	if err == nil {
		for _, task := range remaining {
			var key string
			key, err = imageKey(task)
			if err != nil {
				break
			}
			delete(keys, key)
		}
	}
	if err != nil || s.images == nil {
		log.Printf("删除任务后照片引用检查失败，保留原图: %v", err)
		return failed + len(keys)
	}
	for key := range keys {
		if err := os.Remove(filepath.Join(s.images.dir, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			failed++
			log.Printf("删除任务原图 %s 失败: %v", key, err)
		}
	}
	return failed
}
