package repository

import (
	"errors"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

var ErrInsufficientStock = errors.New("库存不足")

type ComponentRepository struct {
	db *gorm.DB
}

func NewComponentRepository(db *gorm.DB) *ComponentRepository {
	return &ComponentRepository{db: db}
}

// Query 查询参数
type ComponentQuery struct {
	CategoryID *uint
	Keyword    string
	Page       int
	PageSize   int
}

// GetAll 获取所有元件（支持分页和搜索）
func (r *ComponentRepository) GetAll(query ComponentQuery) ([]models.Component, int64, error) {
	var components []models.Component
	var total int64

	db := r.db.Model(&models.Component{}).Preload("Category").Preload("Supplier")

	// 分类筛选
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}

	// 关键词搜索
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		db = db.Joins("LEFT JOIN suppliers ON suppliers.id = components.supplier_id").
			Where("components.name LIKE ? OR components.component_number LIKE ? OR components.model LIKE ? OR components.manufacturer LIKE ? OR value LIKE ? OR supplier_part_number LIKE ? OR description LIKE ? OR suppliers.name LIKE ?",
				keyword, keyword, keyword, keyword, keyword, keyword, keyword, keyword)
	}

	// 计算总数
	db.Count(&total)

	// 分页
	if query.Page > 0 && query.PageSize > 0 {
		offset := (query.Page - 1) * query.PageSize
		db = db.Offset(offset).Limit(query.PageSize)
	}

	err := db.Order("updated_at DESC").Find(&components).Error
	return components, total, err
}

// GetByID 根据ID获取元件
func (r *ComponentRepository) GetByID(id uint) (*models.Component, error) {
	var component models.Component
	err := r.db.Preload("Category").Preload("Supplier").First(&component, id).Error
	return &component, err
}

// Create 创建元件
func (r *ComponentRepository) Create(component *models.Component) error {
	return r.db.Create(component).Error
}

// Update 更新元件
func (r *ComponentRepository) Update(component *models.Component) error {
	return r.db.Save(component).Error
}

// Delete 删除元件
func (r *ComponentRepository) Delete(id uint) error {
	return r.db.Delete(&models.Component{}, id).Error
}

// UpdateStock 更新库存数量
func (r *ComponentRepository) UpdateStock(id uint, amount int) error {
	return r.db.Model(&models.Component{}).Where("id = ?", id).
		UpdateColumn("stock_quantity", gorm.Expr("stock_quantity + ?", amount)).Error
}

// StockChangeParams 库存变更参数
type StockChangeParams struct {
	ComponentID     uint
	Amount          int
	Reason          string
	UnitPriceCents  int64
	TotalPriceCents int64
}

// ApplyStockChange 在事务中更新库存并写入流水，可选更新参考单价
func (r *ComponentRepository) ApplyStockChange(params StockChangeParams) (*models.Component, error) {
	var updated models.Component
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var component models.Component
		if err := tx.First(&component, params.ComponentID).Error; err != nil {
			return err
		}

		if component.StockQuantity+params.Amount < 0 {
			return ErrInsufficientStock
		}

		if err := tx.Model(&models.Component{}).Where("id = ?", params.ComponentID).
			UpdateColumn("stock_quantity", gorm.Expr("stock_quantity + ?", params.Amount)).Error; err != nil {
			return err
		}

		if params.Amount > 0 && params.UnitPriceCents > 0 {
			if err := tx.Model(&models.Component{}).Where("id = ?", params.ComponentID).
				Update("unit_price_cents", params.UnitPriceCents).Error; err != nil {
				return err
			}
		}

		log := models.StockLog{
			ComponentID:     params.ComponentID,
			ChangeAmount:    params.Amount,
			UnitPriceCents:  params.UnitPriceCents,
			TotalPriceCents: params.TotalPriceCents,
			Reason:          params.Reason,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}

		return tx.Preload("Category").Preload("Supplier").First(&updated, params.ComponentID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// BatchUpdateLocation 批量更新元件存放位置
func (r *ComponentRepository) BatchUpdateLocation(ids []uint, location string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Model(&models.Component{}).Where("id IN ?", ids).Update("location", location)
	return result.RowsAffected, result.Error
}

// GetDistinctPackages 获取历史封装列表（去重、非空、按名称排序）
func (r *ComponentRepository) GetDistinctPackages() ([]string, error) {
	var packages []string
	err := r.db.Model(&models.Component{}).
		Where("package <> ''").
		Distinct("package").
		Order("package ASC").
		Pluck("package", &packages).Error
	return packages, err
}

// GetDistinctLocations 获取历史位置列表（去重、非空、按名称排序）
func (r *ComponentRepository) GetDistinctLocations() ([]string, error) {
	var locations []string
	err := r.db.Model(&models.Component{}).
		Where("location <> ''").
		Distinct("location").
		Order("location ASC").
		Pluck("location", &locations).Error
	return locations, err
}
