package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SupplierHandler struct {
	repo *repository.SupplierRepository
}

func NewSupplierHandler(db *gorm.DB) *SupplierHandler {
	return &SupplierHandler{
		repo: repository.NewSupplierRepository(db),
	}
}

// GetAll 获取所有供应商
// @route GET /api/v1/suppliers
func (h *SupplierHandler) GetAll(c *gin.Context) {
	suppliers, err := h.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取供应商失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": suppliers})
}

// GetByID 获取单个供应商
// @route GET /api/v1/suppliers/:id
func (h *SupplierHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	supplier, err := h.repo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "供应商不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": supplier})
}

// Create 创建供应商；同名供应商已存在时返回已有记录
// @route POST /api/v1/suppliers
func (h *SupplierHandler) Create(c *gin.Context) {
	var supplier models.Supplier
	if err := c.ShouldBindJSON(&supplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	supplier.Name = strings.TrimSpace(supplier.Name)
	if supplier.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "供应商名称不能为空"})
		return
	}

	saved, err := h.repo.FirstOrCreateByName(supplier.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建供应商失败"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": saved})
}
