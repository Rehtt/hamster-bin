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

type CategoryHandler struct {
	repo *repository.CategoryRepository
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{
		repo: repository.NewCategoryRepository(db),
	}
}

// GetAll 获取所有分类
// @route GET /api/v1/categories
func (h *CategoryHandler) GetAll(c *gin.Context) {
	page, size := 0, 0
	if c.Query("page") != "" || c.Query("page_size") != "" {
		page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 || size < 1 || size > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "分页参数无效，每页应为 1–100 条"})
			return
		}
	}
	keepID, _ := strconv.ParseUint(c.Query("keep_id"), 10, 32)
	categories, total, err := h.repo.List(c.Query("keyword"), c.Query("status"), uint(keepID), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取分类失败"})
		return
	}
	response := gin.H{"data": categories}
	if page > 0 {
		response["pagination"] = gin.H{"page": page, "page_size": size, "total": total, "total_page": (total + int64(size) - 1) / int64(size)}
	}
	c.JSON(http.StatusOK, response)
}

// GetByID 获取单个分类
// @route GET /api/v1/categories/:id
func (h *CategoryHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	category, err := h.repo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": category})
}

// Create 创建分类
// @route POST /api/v1/categories
func (h *CategoryHandler) Create(c *gin.Context) {
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if err := h.repo.Create(&category); err != nil {
		categoryWriteError(c, err, "创建分类失败")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": category})
}

// Update 更新分类
// @route PUT /api/v1/categories/:id
func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var patch repository.CategoryPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	category, err := h.repo.Update(uint(id), patch)
	if err != nil {
		categoryWriteError(c, err, "更新分类失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": category})
}

// Delete 删除分类
// @route DELETE /api/v1/categories/:id
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		categoryWriteError(c, err, "删除分类失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// categoryWriteError 区分目标不存在、参数错误、关联冲突与数据库异常。
func categoryWriteError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, repository.ErrInvalidCategory):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
	case errors.Is(err, repository.ErrCategoryInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}
