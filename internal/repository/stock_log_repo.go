package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
	"gorm.io/gorm"
)

var (
	ErrAlreadyRevoked       = errors.New("记录已撤销")
	ErrCannotRevokeReversal = errors.New("冲销记录不可撤销")
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

// RevokeStockLog 撤销库存记录：标记原记录并写入反向冲销流水
func (r *StockLogRepository) RevokeStockLog(id uint) (*models.StockLog, *models.StockLog, error) {
	var original models.StockLog
	var reversal models.StockLog

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&original, id).Error; err != nil {
			return err
		}
		if original.RevokedAt != nil {
			return ErrAlreadyRevoked
		}
		if original.ReversalOfID != nil {
			return ErrCannotRevokeReversal
		}

		var component models.Component
		if err := tx.First(&component, original.ComponentID).Error; err != nil {
			return err
		}

		reverseAmount := -original.ChangeAmount
		if component.StockQuantity+reverseAmount < 0 {
			return ErrInsufficientStock
		}

		updates := map[string]any{
			"stock_quantity": gorm.Expr("stock_quantity + ?", reverseAmount),
		}
		if original.ChangeAmount > 0 && original.TotalPriceCents > 0 {
			updates["unit_price_micro"] = price.ReverseAverageUnitPriceMicro(
				component.StockQuantity,
				component.UnitPriceMicro,
				original.ChangeAmount,
				original.TotalPriceCents,
			)
		}
		if err := tx.Model(&models.Component{}).Where("id = ?", original.ComponentID).
			Updates(updates).Error; err != nil {
			return err
		}

		now := time.Now()
		if err := tx.Model(&original).Update("revoked_at", now).Error; err != nil {
			return err
		}
		original.RevokedAt = &now

		reason := fmt.Sprintf("撤销记录 #%d", original.ID)
		if original.Reason != "" {
			reason += "：" + original.Reason
		}

		reversal = models.StockLog{
			ComponentID:     original.ComponentID,
			ChangeAmount:    reverseAmount,
			UnitPriceMicro:  original.UnitPriceMicro,
			TotalPriceCents: original.TotalPriceCents,
			Reason:          reason,
			ReversalOfID:    &original.ID,
		}
		if err := tx.Create(&reversal).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &original, &reversal, nil
}
