package repository

import (
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

type SupplierRepository struct {
	db *gorm.DB
}

func NewSupplierRepository(db *gorm.DB) *SupplierRepository {
	return &SupplierRepository{db: db}
}

// GetAll 获取所有供应商
func (r *SupplierRepository) GetAll() ([]models.Supplier, error) {
	var suppliers []models.Supplier
	err := r.db.Order("name ASC").Find(&suppliers).Error
	return suppliers, err
}

// GetByID 根据ID获取供应商
func (r *SupplierRepository) GetByID(id uint) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.First(&supplier, id).Error
	return &supplier, err
}

// FindByName 根据名称获取供应商
func (r *SupplierRepository) FindByName(name string) (*models.Supplier, error) {
	var supplier models.Supplier
	err := r.db.Where("name = ?", strings.TrimSpace(name)).First(&supplier).Error
	return &supplier, err
}

// FirstOrCreateByName 按名称获取或创建供应商
func (r *SupplierRepository) FirstOrCreateByName(name string) (*models.Supplier, error) {
	supplier := models.Supplier{Name: strings.TrimSpace(name)}
	err := r.db.Where("name = ?", supplier.Name).FirstOrCreate(&supplier).Error
	return &supplier, err
}

// Update 更新供应商
func (r *SupplierRepository) Update(supplier *models.Supplier) error {
	supplier.Name = strings.TrimSpace(supplier.Name)
	return r.db.Save(supplier).Error
}

// Delete 删除供应商
func (r *SupplierRepository) Delete(id uint) error {
	return r.db.Delete(&models.Supplier{}, id).Error
}
