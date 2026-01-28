package repository

import (
	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

type StockLogRepository struct {
	db *gorm.DB
}

func NewStockLogRepository(db *gorm.DB) *StockLogRepository {
	return &StockLogRepository{db: db}
}

// Create 创建库存记录
func (r *StockLogRepository) Create(log *models.StockLog) error {
	return r.db.Create(log).Error
}

// GetByComponentID 获取指定元件的库存记录
func (r *StockLogRepository) GetByComponentID(componentID uint, limit int) ([]models.StockLog, error) {
	var logs []models.StockLog
	query := r.db.Where("component_id = ?", componentID).
		Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	err := query.Find(&logs).Error
	return logs, err
}

// GetAll 获取所有库存记录（分页）
func (r *StockLogRepository) GetAll(page, pageSize int) ([]models.StockLog, int64, error) {
	var logs []models.StockLog
	var total int64

	db := r.db.Model(&models.StockLog{}).Preload("Component")
	
	db.Count(&total)

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	err := db.Order("created_at DESC").Find(&logs).Error
	return logs, total, err
}
