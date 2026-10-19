package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BatchHandler struct{ db *gorm.DB }

func NewBatchHandler(db *gorm.DB) *BatchHandler { return &BatchHandler{db: db} }

type BatchComponentItem struct {
	models.Component
	Quantity        *int   `json:"stock_quantity"`
	CategoryName    string `json:"category_name"`
	SupplierName    string `json:"supplier_name"`
	TotalPriceCents int64  `json:"total_price_cents"`
}
type BatchPreStockItem struct {
	models.PreStock
	Quantity     *int   `json:"expected_quantity"`
	CategoryName string `json:"category_name"`
	SupplierName string `json:"supplier_name"`
}

func resolveCategory(id uint) (uint, error) {
	if id == 0 {
		return 0, normalization.Invalid("category_id", "required", "请按 ID 选择启用分类，需要新分类请进入分类管理")
	}
	return id, nil
}
func resolveSupplier(tx *gorm.DB, id *uint, name string) (*uint, error) {
	if id != nil && *id > 0 {
		var s models.Supplier
		if err := tx.First(&s, *id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, normalization.Invalid("supplier_id", "invalid", "供应商不存在")
			}
			return nil, err
		}
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var s models.Supplier
	if err := tx.Where("name = ?", name).First(&s).Error; err == nil {
		return &s.ID, nil
	}
	s = models.Supplier{Name: name}
	if err := tx.Create(&s).Error; err != nil {
		return nil, err
	}
	return &s.ID, nil
}
func (h *BatchHandler) CreateComponents(c *gin.Context) {
	var req struct {
		Items []BatchComponentItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 || len(req.Items) > 100 {
		c.JSON(400, gin.H{"error": "items 必须包含 1-100 条记录"})
		return
	}
	created := make([]models.Component, 0, len(req.Items))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		components := repository.NewComponentRepository(tx)
		for i := range req.Items {
			item := &req.Items[i]
			if strings.TrimSpace(item.Name) == "" {
				return batchItemError(i, normalization.Invalid("name", "required", "名称不能为空"))
			}
			if item.Quantity == nil || *item.Quantity < 0 {
				return batchItemError(i, normalization.Invalid("stock_quantity", "required", "库存数量必须显式填写非负整数"))
			}
			item.StockQuantity = *item.Quantity
			category, err := resolveCategory(item.CategoryID)
			if err != nil {
				return batchItemError(i, err)
			}
			supplier, err := resolveSupplier(tx, item.SupplierID, item.SupplierName)
			if err != nil {
				return batchItemError(i, err)
			}
			item.CategoryID, item.SupplierID, item.Category, item.Supplier = category, supplier, nil, nil
			item.ID = 0
			if _, err := components.CreateWithInitialStock(&item.Component, item.TotalPriceCents); err != nil {
				return batchItemError(i, err)
			}
			created = append(created, item.Component)
		}
		return nil
	})
	if err != nil {
		inventoryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}
func (h *BatchHandler) CreatePreStocks(c *gin.Context) {
	var req struct {
		Items []BatchPreStockItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 || len(req.Items) > 100 {
		c.JSON(400, gin.H{"error": "items 必须包含 1-100 条记录"})
		return
	}
	created := make([]models.PreStock, 0, len(req.Items))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewPreStockRepository(tx)
		for i := range req.Items {
			item := &req.Items[i]
			if strings.TrimSpace(item.Name) == "" {
				return batchItemError(i, normalization.Invalid("name", "required", "名称不能为空"))
			}
			if item.Quantity == nil || *item.Quantity < 0 {
				return batchItemError(i, normalization.Invalid("expected_quantity", "required", "数量必须显式填写非负整数"))
			}
			item.ExpectedQuantity = *item.Quantity
			category, err := resolveCategory(item.CategoryID)
			if err != nil {
				return batchItemError(i, err)
			}
			supplier, err := resolveSupplier(tx, item.SupplierID, item.SupplierName)
			if err != nil {
				return batchItemError(i, err)
			}
			item.CategoryID, item.SupplierID, item.Category, item.Supplier, item.Component = category, supplier, nil, nil, nil
			item.ID, item.Status = 0, repository.PreStockStatusPending
			if err := repo.Create(&item.PreStock); err != nil {
				return batchItemError(i, err)
			}
			created = append(created, item.PreStock)
		}
		return nil
	})
	if err != nil {
		inventoryWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}
func batchItemError(index int, err error) error {
	var validation *normalization.ValidationError
	if errors.As(err, &validation) {
		validation.Item = index + 1
		return validation
	}
	return fmt.Errorf("第 %d 条: %w", index+1, err)
}
