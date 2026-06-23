package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PreStockHandler struct {
	repo *repository.PreStockRepository
}

func NewPreStockHandler(db *gorm.DB) *PreStockHandler {
	return &PreStockHandler{
		repo: repository.NewPreStockRepository(db),
	}
}

// GetAll 获取预入库记录
// @route GET /api/v1/pre-stocks
func (h *PreStockHandler) GetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	query := repository.PreStockQuery{
		Status:   c.DefaultQuery("status", repository.PreStockStatusPending),
		Page:     page,
		PageSize: pageSize,
	}
	items, total, err := h.repo.GetAll(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取预入库记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": gin.H{
			"page":       page,
			"page_size":  pageSize,
			"total":      total,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// GetByID 获取单个预入库记录
// @route GET /api/v1/pre-stocks/:id
func (h *PreStockHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	item, err := h.repo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "预入库记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Create 创建预入库记录
// @route POST /api/v1/pre-stocks
func (h *PreStockHandler) Create(c *gin.Context) {
	var preStock models.PreStock
	if err := c.ShouldBindJSON(&preStock); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := h.repo.Create(&preStock); err != nil {
		writePreStockError(c, err, "创建预入库记录失败")
		return
	}

	created, err := h.repo.GetByID(preStock.ID)
	if err == nil {
		preStock = *created
	}

	c.JSON(http.StatusCreated, gin.H{"data": preStock})
}

// Update 更新预入库记录
// @route PUT /api/v1/pre-stocks/:id
func (h *PreStockHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var preStock models.PreStock
	if err := c.ShouldBindJSON(&preStock); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	preStock.ID = uint(id)
	preStock.Status = repository.PreStockStatusPending
	preStock.Category = nil
	preStock.Supplier = nil
	preStock.Component = nil

	if err := h.repo.Update(&preStock); err != nil {
		writePreStockError(c, err, "更新预入库记录失败")
		return
	}

	updated, err := h.repo.GetByID(preStock.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新后预入库记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": updated})
}

// Delete 删除待入库记录
// @route DELETE /api/v1/pre-stocks/:id
func (h *PreStockHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		writePreStockError(c, err, "删除预入库记录失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// Confirm 确认预入库并转为正式元件
// @route POST /api/v1/pre-stocks/:id/confirm
func (h *PreStockHandler) Confirm(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	item, err := h.repo.Confirm(uint(id))
	if err != nil {
		writePreStockError(c, err, "确认预入库失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": item, "message": "预入库确认成功"})
}

func writePreStockError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, repository.ErrComponentNumberDuplicate):
		c.JSON(http.StatusBadRequest, gin.H{"error": "元件编号已存在"})
	case errors.Is(err, repository.ErrPreStockAlreadyConfirmed):
		c.JSON(http.StatusBadRequest, gin.H{"error": "预入库记录已确认"})
	case errors.Is(err, repository.ErrInvalidPreStockStatus):
		c.JSON(http.StatusBadRequest, gin.H{"error": "预入库状态无效"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "预入库记录不存在"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}
