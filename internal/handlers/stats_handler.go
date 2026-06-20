package handlers

import (
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatsHandler struct {
	repo *repository.StatsRepository
}

func NewStatsHandler(db *gorm.DB) *StatsHandler {
	return &StatsHandler{
		repo: repository.NewStatsRepository(db),
	}
}

// GetDashboard 获取仪表盘统计
// @route GET /api/v1/stats
func (h *StatsHandler) GetDashboard(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", repository.StatsRangeMonth)
	if rangeKey != repository.StatsRangeMonth &&
		rangeKey != repository.StatsRangeQuarter &&
		rangeKey != repository.StatsRangeAll {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 range 参数，可选值：month、quarter、all"})
		return
	}

	stats, err := h.repo.GetDashboardStats(rangeKey)
	if err != nil {
		if err == repository.ErrInvalidStatsRange {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计数据失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
