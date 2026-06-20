package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StockLogHandler struct {
	repo *repository.StockLogRepository
}

func NewStockLogHandler(db *gorm.DB) *StockLogHandler {
	return &StockLogHandler{
		repo: repository.NewStockLogRepository(db),
	}
}

// GetAll 获取所有库存记录（分页）
// @route GET /api/v1/stock-logs?page=1&page_size=20
func (h *StockLogHandler) GetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.repo.GetAll(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": logs,
		"pagination": gin.H{
			"page":       page,
			"page_size":  pageSize,
			"total":      total,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// Revoke 撤销库存记录
// @route POST /api/v1/stock-logs/:id/revoke
func (h *StockLogHandler) Revoke(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录 ID"})
		return
	}

	original, reversal, err := h.repo.RevokeStockLog(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		if errors.Is(err, repository.ErrAlreadyRevoked) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "记录已撤销"})
			return
		}
		if errors.Is(err, repository.ErrCannotRevokeReversal) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "冲销记录不可撤销"})
			return
		}
		if errors.Is(err, repository.ErrInsufficientStock) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "库存不足，无法撤销该入库记录"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "撤销失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"original": original,
			"reversal": reversal,
		},
	})
}
