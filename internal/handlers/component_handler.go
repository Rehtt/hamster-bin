package handlers

import (
	"errors"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	_ "image/jpeg"
	_ "image/png"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
	"github.com/Rehtt/hamster-bin/internal/repository"

	"github.com/gen2brain/avif"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ComponentHandler struct {
	componentRepo *repository.ComponentRepository
	stockLogRepo  *repository.StockLogRepository
}

func NewComponentHandler(db *gorm.DB) *ComponentHandler {
	return &ComponentHandler{
		componentRepo: repository.NewComponentRepository(db),
		stockLogRepo:  repository.NewStockLogRepository(db),
	}
}

// GetAll 获取所有元件（支持分页和搜索）
// @route GET /api/v1/components?page=1&page_size=20&keyword=ESP32&category_id=1
func (h *ComponentHandler) GetAll(c *gin.Context) {
	var query repository.ComponentQuery

	// 解析查询参数
	if page := c.Query("page"); page != "" {
		query.Page, _ = strconv.Atoi(page)
	} else {
		query.Page = 1
	}

	if pageSize := c.Query("page_size"); pageSize != "" {
		query.PageSize, _ = strconv.Atoi(pageSize)
	} else {
		query.PageSize = 20
	}

	query.Keyword = c.Query("keyword")

	if categoryID := c.Query("category_id"); categoryID != "" {
		id, err := strconv.ParseUint(categoryID, 10, 32)
		if err == nil {
			uid := uint(id)
			query.CategoryID = &uid
		}
	}

	components, total, err := h.componentRepo.GetAll(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取元件列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": components,
		"pagination": gin.H{
			"page":       query.Page,
			"page_size":  query.PageSize,
			"total":      total,
			"total_page": (total + int64(query.PageSize) - 1) / int64(query.PageSize),
		},
	})
}

// GetOptions 获取元件录入表单的历史选项（封装、位置）
// @route GET /api/v1/components/options
func (h *ComponentHandler) GetOptions(c *gin.Context) {
	packages, err := h.componentRepo.GetDistinctPackages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取封装选项失败"})
		return
	}

	locations, err := h.componentRepo.GetDistinctLocations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取位置选项失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"packages":  packages,
			"locations": locations,
		},
	})
}

// GetByID 获取单个元件详情
// @route GET /api/v1/components/:id
func (h *ComponentHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	component, err := h.componentRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "元件不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": component})
}

// Create 创建元件
// @route POST /api/v1/components
func (h *ComponentHandler) Create(c *gin.Context) {
	var req struct {
		models.Component
		TotalPriceCents *int64 `json:"total_price_cents"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	component := req.Component
	if req.TotalPriceCents != nil && *req.TotalPriceCents > 0 && component.StockQuantity > 0 {
		component.UnitPriceCents = price.UnitPriceCents(*req.TotalPriceCents, component.StockQuantity)
	}

	if err := h.componentRepo.AssignComponentNumberForCreate(&component); err != nil {
		if errors.Is(err, repository.ErrComponentNumberDuplicate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "元件编号已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分配元件编号失败"})
		return
	}

	if err := h.componentRepo.Create(&component); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建元件失败"})
		return
	}

	if component.StockQuantity > 0 && req.TotalPriceCents != nil && *req.TotalPriceCents > 0 {
		log := models.StockLog{
			ComponentID:     component.ID,
			ChangeAmount:    component.StockQuantity,
			UnitPriceCents:  component.UnitPriceCents,
			TotalPriceCents: *req.TotalPriceCents,
			Reason:          "初始入库",
		}
		if err := h.stockLogRepo.Create(&log); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建初始库存记录失败"})
			return
		}
	}

	created, err := h.componentRepo.GetByID(component.ID)
	if err == nil {
		component = *created
	}

	c.JSON(http.StatusCreated, gin.H{"data": component})
}

// Update 更新元件
// @route PUT /api/v1/components/:id
func (h *ComponentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 1. 先获取现有元件信息
	existing, err := h.componentRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "元件不存在"})
		return
	}
	component := existing

	// 2. 将请求数据绑定到现有对象上（支持部分更新）
	if err := c.ShouldBindJSON(component); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 3. 确保 ID 正确
	component.ID = uint(id)

	// 4. 清除关联对象，防止 GORM 尝试更新关联的分类信息，只更新外键 CategoryID
	component.Category = nil
	component.Supplier = nil

	if err := h.componentRepo.ValidateComponentNumberForUpdate(component, existing); err != nil {
		if errors.Is(err, repository.ErrComponentNumberDuplicate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "元件编号已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验元件编号失败"})
		return
	}

	// 5. 保存更新
	if err := h.componentRepo.Update(component); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新元件失败"})
		return
	}

	component, err = h.componentRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取更新后元件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": component})
}

// BatchUpdateLocation 批量更新元件存放位置
// @route PATCH /api/v1/components/batch-location
// Body: {"ids": [1, 2, 3], "location": "A1-03"}
func (h *ComponentHandler) BatchUpdateLocation(c *gin.Context) {
	var req struct {
		IDs      []uint `json:"ids" binding:"required,min=1"`
		Location string `json:"location"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	updated, err := h.componentRepo.BatchUpdateLocation(req.IDs, req.Location)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量更新位置失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量更新位置成功",
		"updated": updated,
	})
}

// GenerateMissingNumbers 为所有未编号元件自动生成编号
// @route PATCH /api/v1/components/generate-numbers
func (h *ComponentHandler) GenerateMissingNumbers(c *gin.Context) {
	updated, err := h.componentRepo.GenerateMissingComponentNumbers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "自动编号失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "自动编号完成",
		"updated": updated,
	})
}

// Delete 删除元件
// @route DELETE /api/v1/components/:id
func (h *ComponentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.componentRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除元件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// UpdateStock 库存变更（入库/出库）
// @route POST /api/v1/components/:id/stock
// Body: {"amount": 10, "reason": "采购", "total_price_cents": 1234}
func (h *ComponentHandler) UpdateStock(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req struct {
		Amount          int    `json:"amount" binding:"required"`
		Reason          string `json:"reason"`
		TotalPriceCents *int64 `json:"total_price_cents"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	params := repository.StockChangeParams{
		ComponentID: uint(id),
		Amount:      req.Amount,
		Reason:      req.Reason,
	}

	if req.Amount > 0 && req.TotalPriceCents != nil && *req.TotalPriceCents > 0 {
		params.TotalPriceCents = *req.TotalPriceCents
		params.UnitPriceCents = price.UnitPriceCents(*req.TotalPriceCents, req.Amount)
	}

	component, err := h.componentRepo.ApplyStockChange(params)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "库存不足"})
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "元件不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新库存失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": component, "message": "库存更新成功"})
}

// GetStockLogs 获取元件的库存变更记录
// @route GET /api/v1/components/:id/logs
func (h *ComponentHandler) GetStockLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	limit := 50 // 默认最近50条
	if l := c.Query("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	logs, err := h.stockLogRepo.GetByComponentID(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// UploadImage 上传并压缩图片
// @route POST /api/v1/components/:id/image
func (h *ComponentHandler) UploadImage(c *gin.Context) {
	idStr := c.Param("id")
	_, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取图片失败"})
		return
	}

	// 打开文件
	srcFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开图片失败"})
		return
	}
	defer srcFile.Close()

	// 解码图片
	img, _, err := image.Decode(srcFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片格式不支持"})
		return
	}

	// 确保存储目录存在
	storageDir := config.Load().ImageDir
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 创建目标文件
	dstPath := filepath.Join(storageDir, idStr+".avif")
	dstFile, err := os.Create(dstPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目标文件失败"})
		return
	}
	defer dstFile.Close()

	// 编码为 AVIF
	// Quality: 50 (可根据需要调整，范围 1-100)
	if err := avif.Encode(dstFile, img, avif.Options{Quality: 50}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "图片压缩失败"})
		return
	}

	// 可选：更新数据库中的 ImageURL 字段指向本地 API（或者留空，由前端统一处理）
	// 这里我们选择更新为一个本地标识，或者保持原样，因为前端会优先请求本地接口

	c.JSON(http.StatusOK, gin.H{"message": "图片上传成功", "url": "/api/v1/components/" + idStr + "/image"})
}

// GetImage 获取元件图片
// @route GET /api/v1/components/:id/image
func (h *ComponentHandler) GetImage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	// 1. 检查本地 AVIF 文件
	localPath := filepath.Join(config.Load().ImageDir, idStr+".avif")
	if _, err := os.Stat(localPath); err == nil {
		c.Header("Cache-Control", "public, max-age=86400") // 缓存一天
		c.File(localPath)
		return
	}

	// 2. 如果本地没有，检查数据库中是否有 External URL
	component, err := h.componentRepo.GetByID(uint(id))
	if err == nil && component.ImageURL != "" {
		// 如果是外部链接，重定向
		c.Redirect(http.StatusFound, component.ImageURL)
		return
	}

	// 3. 都没有，返回默认图片或 404
	// 这里返回 404，由前端决定显示什么默认图，或者返回一个特定的 placeholder
	c.Status(http.StatusNotFound)
}
