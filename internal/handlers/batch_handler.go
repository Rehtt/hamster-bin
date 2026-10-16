package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
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

func resolveCategory(tx *gorm.DB, id uint, name string) (uint, error) {
	if id > 0 {
		var c models.Category
		if err := tx.First(&c, id).Error; err != nil {
			return 0, errors.New("分类不存在")
		}
		return id, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("分类不能为空")
	}
	var c models.Category
	if err := tx.Where("name = ?", name).First(&c).Error; err == nil {
		return c.ID, nil
	}
	c = models.Category{Name: name}
	if err := tx.Create(&c).Error; err != nil {
		return 0, err
	}
	return c.ID, nil
}
func resolveSupplier(tx *gorm.DB, id *uint, name string) (*uint, error) {
	if id != nil && *id > 0 {
		var s models.Supplier
		if err := tx.First(&s, *id).Error; err != nil {
			return nil, errors.New("供应商不存在")
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
		logs := repository.NewStockLogRepository(tx)
		for i := range req.Items {
			item := &req.Items[i]
			item.Name = strings.TrimSpace(item.Name)
			if item.Name == "" {
				return errors.New("第 " + itoa(i+1) + " 条名称不能为空")
			}
			if item.Quantity == nil || *item.Quantity < 0 {
				return errors.New("第 " + itoa(i+1) + " 条库存数量必须显式填写非负整数")
			}
			item.StockQuantity = *item.Quantity
			category, err := resolveCategory(tx, item.CategoryID, item.CategoryName)
			if err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			supplier, err := resolveSupplier(tx, item.SupplierID, item.SupplierName)
			if err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			item.CategoryID, item.SupplierID, item.Category, item.Supplier = category, supplier, nil, nil
			if item.StockQuantity > 0 && item.TotalPriceCents > 0 {
				item.UnitPriceMicro = price.UnitPriceMicro(item.TotalPriceCents, item.StockQuantity)
			}
			item.ID = 0
			if err := components.AssignComponentNumberForCreate(&item.Component); err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			if err := tx.Create(&item.Component).Error; err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			if item.StockQuantity > 0 && item.TotalPriceCents > 0 {
				if err := logs.Create(&models.StockLog{ComponentID: item.ID, ChangeAmount: item.StockQuantity, UnitPriceMicro: item.UnitPriceMicro, TotalPriceCents: item.TotalPriceCents, Reason: "初始入库"}); err != nil {
					return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
				}
			}
			created = append(created, item.Component)
		}
		return nil
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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
			item.Name = strings.TrimSpace(item.Name)
			if item.Name == "" {
				return errors.New("第 " + itoa(i+1) + " 条名称不能为空")
			}
			if item.Quantity == nil || *item.Quantity < 0 {
				return errors.New("第 " + itoa(i+1) + " 条数量必须显式填写非负整数")
			}
			item.ExpectedQuantity = *item.Quantity
			category, err := resolveCategory(tx, item.CategoryID, item.CategoryName)
			if err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			supplier, err := resolveSupplier(tx, item.SupplierID, item.SupplierName)
			if err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			item.CategoryID, item.SupplierID, item.Category, item.Supplier, item.Component = category, supplier, nil, nil, nil
			item.ID, item.Status = 0, repository.PreStockStatusPending
			if err := repo.Create(&item.PreStock); err != nil {
				return errors.New("第 " + itoa(i+1) + " 条：" + err.Error())
			}
			created = append(created, item.PreStock)
		}
		return nil
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	b := make([]byte, 0, 10)
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
