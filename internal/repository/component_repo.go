package repository

import (
	"errors"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
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
	CategoryID         *uint
	Keyword            string
	ComponentNumber    string
	Name               string
	Model              string
	Manufacturer       string
	Value              string
	SupplierName       string
	SupplierPartNumber string
	Page               int
	PageSize           int
	SortBy             string
	SortOrder          string
}

// ComponentSortColumns 允许排序的 API 字段名到 SQL 列映射
var ComponentSortColumns = map[string]string{
	"component_number":     "components.component_number",
	"name":                 "components.name",
	"model":                "components.model",
	"manufacturer":         "components.manufacturer",
	"value":                "components.value",
	"package":              "components.package",
	"description":          "components.description",
	"category":             "categories.name",
	"stock_quantity":       "components.stock_quantity",
	"unit_price":           "components.unit_price_cents",
	"location":             "components.location",
	"supplier":             "suppliers.name",
	"supplier_part_number": "components.supplier_part_number",
	"datasheet_url":        "components.datasheet_url",
	"created_at":           "components.created_at",
	"updated_at":           "components.updated_at",
}

func IsValidComponentSortBy(sortBy string) bool {
	_, ok := ComponentSortColumns[sortBy]
	return ok
}

func applyColumnLikeTokens(db *gorm.DB, column, raw string) *gorm.DB {
	for _, token := range strings.Fields(raw) {
		db = db.Where(column+" LIKE ?", "%"+token+"%")
	}
	return db
}

func applyKeywordTokens(db *gorm.DB, keyword string) *gorm.DB {
	for _, token := range strings.Fields(keyword) {
		pattern := "%" + token + "%"
		db = db.Where(
			"components.name LIKE ? OR components.component_number LIKE ? OR components.model LIKE ? OR components.manufacturer LIKE ? OR components.value LIKE ? OR components.supplier_part_number LIKE ? OR components.description LIKE ? OR suppliers.name LIKE ?",
			pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern,
		)
	}
	return db
}

func needsSupplierJoin(query ComponentQuery) bool {
	return query.SupplierName != "" || query.Keyword != "" || query.SortBy == "supplier"
}

func needsCategoryJoin(query ComponentQuery) bool {
	return query.SortBy == "category"
}

func applyComponentSort(db *gorm.DB, query ComponentQuery) *gorm.DB {
	sortBy := strings.TrimSpace(query.SortBy)
	if sortBy == "" {
		sortBy = "updated_at"
	}

	column, ok := ComponentSortColumns[sortBy]
	if !ok {
		column = ComponentSortColumns["updated_at"]
	}

	order := "DESC"
	if strings.EqualFold(strings.TrimSpace(query.SortOrder), "asc") {
		order = "ASC"
	}

	return db.Order(column + " " + order)
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

	if needsSupplierJoin(query) {
		db = db.Joins("LEFT JOIN suppliers ON suppliers.id = components.supplier_id")
	}
	if needsCategoryJoin(query) {
		db = db.Joins("LEFT JOIN categories ON categories.id = components.category_id")
	}

	db = applyColumnLikeTokens(db, "components.component_number", query.ComponentNumber)
	db = applyColumnLikeTokens(db, "components.name", query.Name)
	db = applyColumnLikeTokens(db, "components.model", query.Model)
	db = applyColumnLikeTokens(db, "components.manufacturer", query.Manufacturer)
	db = applyColumnLikeTokens(db, "components.value", query.Value)
	db = applyColumnLikeTokens(db, "suppliers.name", query.SupplierName)
	db = applyColumnLikeTokens(db, "components.supplier_part_number", query.SupplierPartNumber)

	if query.Keyword != "" {
		db = applyKeywordTokens(db, query.Keyword)
	}

	// 计算总数
	db.Count(&total)

	// 分页
	if query.Page > 0 && query.PageSize > 0 {
		offset := (query.Page - 1) * query.PageSize
		db = db.Offset(offset).Limit(query.PageSize)
	}

	err := applyComponentSort(db, query).Find(&components).Error
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

		if params.Amount > 0 && params.TotalPriceCents > 0 {
			newUnitPrice := price.WeightedAverageUnitPriceCents(
				component.StockQuantity,
				component.UnitPriceCents,
				params.Amount,
				params.TotalPriceCents,
			)
			if newUnitPrice > 0 {
				if err := tx.Model(&models.Component{}).Where("id = ?", params.ComponentID).
					Update("unit_price_cents", newUnitPrice).Error; err != nil {
					return err
				}
			}
		}

		logUnitPrice := params.UnitPriceCents
		logTotalPrice := params.TotalPriceCents
		if params.Amount < 0 && logUnitPrice == 0 && component.UnitPriceCents > 0 {
			qty := -params.Amount
			logUnitPrice = component.UnitPriceCents
			logTotalPrice = price.TotalPriceCents(component.UnitPriceCents, qty)
		}

		log := models.StockLog{
			ComponentID:     params.ComponentID,
			ChangeAmount:    params.Amount,
			UnitPriceCents:  logUnitPrice,
			TotalPriceCents: logTotalPrice,
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

// GetDistinctManufacturers 获取历史制造商列表（去重、非空、按名称排序）
func (r *ComponentRepository) GetDistinctManufacturers() ([]string, error) {
	var manufacturers []string
	err := r.db.Model(&models.Component{}).
		Where("manufacturer <> ''").
		Distinct("manufacturer").
		Order("manufacturer ASC").
		Pluck("manufacturer", &manufacturers).Error
	return manufacturers, err
}
