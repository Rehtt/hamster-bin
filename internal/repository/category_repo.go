package repository

import (
	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll 获取所有分类
func (r *CategoryRepository) GetAll() ([]models.Category, error) {
	var categories []models.Category
	err := r.db.Find(&categories).Error
	return categories, err
}

// GetByID 根据ID获取分类
func (r *CategoryRepository) GetByID(id uint) (*models.Category, error) {
	var category models.Category
	err := r.db.First(&category, id).Error
	return &category, err
}

// Create 创建分类
func (r *CategoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

// Update 更新分类
func (r *CategoryRepository) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

// Delete 删除分类
func (r *CategoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.Category{}, id).Error
}
