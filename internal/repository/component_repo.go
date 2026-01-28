package repository

import (
	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

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

	db := r.db.Model(&models.Component{}).Preload("Category")

	// 分类筛选
	if query.CategoryID != nil {
		db = db.Where("category_id = ?", *query.CategoryID)
	}

	// 关键词搜索
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		db = db.Where("name LIKE ? OR value LIKE ? OR description LIKE ?",
			keyword, keyword, keyword)
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
	err := r.db.Preload("Category").First(&component, id).Error
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
