package handlers

import (
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
