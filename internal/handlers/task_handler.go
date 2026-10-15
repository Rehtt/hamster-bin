package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/Rehtt/hamster-bin/internal/tasks"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct{ service *tasks.Service }

func NewTaskHandler(service *tasks.Service) *TaskHandler { return &TaskHandler{service: service} }
func (h *TaskHandler) Create(c *gin.Context) {
	var req tasks.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}
	if err := h.service.Validate(req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	t, err := h.service.Submit(req, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "任务保存失败"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": t})
}
func taskID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "无效的任务 ID"})
		return 0, false
	}
	return uint(id), true
}
func taskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "任务不存在"})
	case errors.Is(err, repository.ErrTaskState):
		c.JSON(409, gin.H{"error": err.Error()})
	default:
		c.JSON(500, gin.H{"error": "任务操作失败"})
	}
}
func (h *TaskHandler) Get(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	t, err := h.service.Repo.Get(id)
	if err != nil {
		taskError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": t})
}
func (h *TaskHandler) List(c *gin.Context) {
	page, e1 := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, e2 := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, kind := c.Query("status"), c.Query("type")
	if e1 != nil || e2 != nil || page < 1 || size < 1 || size > 100 || (status != "" && !repository.ValidTaskStatus(status)) || (kind != "" && !tasks.ValidType(kind)) {
		c.JSON(400, gin.H{"error": "无效的任务查询参数"})
		return
	}
	items, total, err := h.service.Repo.List(status, kind, page, size)
	if err != nil {
		taskError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": items, "pagination": gin.H{"page": page, "page_size": size, "total": total, "total_page": (total + int64(size) - 1) / int64(size)}})
}
func (h *TaskHandler) Cancel(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	if err := h.service.Cancel(id); err != nil {
		taskError(c, err)
		return
	}
	h.Get(c)
}
func (h *TaskHandler) Retry(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	original, err := h.service.Repo.Get(id)
	if err != nil {
		taskError(c, err)
		return
	}
	if original.Status != repository.TaskFailed && original.Status != repository.TaskCancelled {
		taskError(c, repository.ErrTaskState)
		return
	}
	t, err := h.service.Retry(id)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(202, gin.H{"data": t})
}
