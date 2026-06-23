package repository

import (
	"errors"
	"time"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
	"gorm.io/gorm"
)

const (
	PreStockStatusPending   = "pending"
	PreStockStatusConfirmed = "confirmed"
)

var (
	ErrPreStockAlreadyConfirmed = errors.New("预入库已确认")
	ErrInvalidPreStockStatus    = errors.New("无效的预入库状态")
)

type PreStockRepository struct {
	db *gorm.DB
}

func NewPreStockRepository(db *gorm.DB) *PreStockRepository {
	return &PreStockRepository{db: db}
}

type PreStockQuery struct {
	Status   string
	Page     int
	PageSize int
}

func normalizePreStockStatus(status string) string {
	if status == "" {
		return PreStockStatusPending
	}
	return status
}

func isValidPreStockStatus(status string) bool {
	return status == PreStockStatusPending || status == PreStockStatusConfirmed
}

func (r *PreStockRepository) GetAll(query PreStockQuery) ([]models.PreStock, int64, error) {
	var items []models.PreStock
	var total int64

	db := r.db.Model(&models.PreStock{}).Preload("Category").Preload("Supplier").Preload("Component")
	if query.Status != "" && query.Status != "all" {
		db = db.Where("status = ?", query.Status)
	}

	db.Count(&total)

	if query.Page > 0 && query.PageSize > 0 {
		offset := (query.Page - 1) * query.PageSize
		db = db.Offset(offset).Limit(query.PageSize)
	}

	err := db.Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (r *PreStockRepository) GetByID(id uint) (*models.PreStock, error) {
	var item models.PreStock
	err := r.db.Preload("Category").Preload("Supplier").Preload("Component").First(&item, id).Error
	return &item, err
}

func (r *PreStockRepository) assignNumberInTx(tx *gorm.DB, preStock *models.PreStock) error {
	preStock.ComponentNumber = NormalizeComponentNumber(preStock.ComponentNumber)
	if preStock.ComponentNumber != nil {
		taken, err := isComponentNumberTakenInTx(tx, *preStock.ComponentNumber, 0, preStock.ID)
		if err != nil {
			return err
		}
		if taken {
			return ErrComponentNumberDuplicate
		}
		return nil
	}

	componentRepo := NewComponentRepository(tx)
	number, err := componentRepo.generateNextInTx(tx)
	if err != nil {
		return err
	}
	preStock.ComponentNumber = &number
	return nil
}

func (r *PreStockRepository) Create(preStock *models.PreStock) error {
	preStock.Status = normalizePreStockStatus(preStock.Status)
	if !isValidPreStockStatus(preStock.Status) || preStock.Status != PreStockStatusPending {
		return ErrInvalidPreStockStatus
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.assignNumberInTx(tx, preStock); err != nil {
			return err
		}
		return tx.Create(preStock).Error
	})
}

func (r *PreStockRepository) Update(preStock *models.PreStock) error {
	if preStock.Status != PreStockStatusPending {
		return ErrPreStockAlreadyConfirmed
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.PreStock
		if err := tx.First(&existing, preStock.ID).Error; err != nil {
			return err
		}
		if existing.Status != PreStockStatusPending {
			return ErrPreStockAlreadyConfirmed
		}
		if err := r.assignNumberInTx(tx, preStock); err != nil {
			return err
		}

		updates := map[string]any{
			"category_id":          preStock.CategoryID,
			"component_number":     preStock.ComponentNumber,
			"name":                 preStock.Name,
			"model":                preStock.Model,
			"manufacturer":         preStock.Manufacturer,
			"value":                preStock.Value,
			"package":              preStock.Package,
			"supplier_id":          preStock.SupplierID,
			"supplier_part_number": preStock.SupplierPartNumber,
			"description":          preStock.Description,
			"expected_quantity":    preStock.ExpectedQuantity,
			"total_price_cents":    preStock.TotalPriceCents,
			"location":             preStock.Location,
			"datasheet_url":        preStock.DatasheetURL,
			"image_url":            preStock.ImageURL,
		}
		return tx.Model(&existing).Updates(updates).Error
	})
}

func (r *PreStockRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.PreStock
		if err := tx.First(&existing, id).Error; err != nil {
			return err
		}
		if existing.Status != PreStockStatusPending {
			return ErrPreStockAlreadyConfirmed
		}
		return tx.Delete(&models.PreStock{}, id).Error
	})
}

func (r *PreStockRepository) Confirm(id uint) (*models.PreStock, error) {
	var confirmed models.PreStock

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var preStock models.PreStock
		if err := tx.First(&preStock, id).Error; err != nil {
			return err
		}
		if preStock.Status != PreStockStatusPending {
			return ErrPreStockAlreadyConfirmed
		}
		preStock.ComponentNumber = NormalizeComponentNumber(preStock.ComponentNumber)
		if preStock.ComponentNumber == nil {
			componentRepo := NewComponentRepository(tx)
			number, err := componentRepo.generateNextInTx(tx)
			if err != nil {
				return err
			}
			preStock.ComponentNumber = &number
		}
		taken, err := isComponentNumberTakenInTx(tx, *preStock.ComponentNumber, 0, preStock.ID)
		if err != nil {
			return err
		}
		if taken {
			return ErrComponentNumberDuplicate
		}

		unitPriceMicro := int64(0)
		if preStock.ExpectedQuantity > 0 && preStock.TotalPriceCents > 0 {
			unitPriceMicro = price.UnitPriceMicro(preStock.TotalPriceCents, preStock.ExpectedQuantity)
		}

		component := models.Component{
			CategoryID:         preStock.CategoryID,
			ComponentNumber:    preStock.ComponentNumber,
			Name:               preStock.Name,
			Model:              preStock.Model,
			Manufacturer:       preStock.Manufacturer,
			Value:              preStock.Value,
			Package:            preStock.Package,
			SupplierID:         preStock.SupplierID,
			SupplierPartNumber: preStock.SupplierPartNumber,
			Description:        preStock.Description,
			StockQuantity:      preStock.ExpectedQuantity,
			UnitPriceMicro:     unitPriceMicro,
			Location:           preStock.Location,
			DatasheetURL:       preStock.DatasheetURL,
			ImageURL:           preStock.ImageURL,
		}
		if err := tx.Create(&component).Error; err != nil {
			return err
		}

		if preStock.ExpectedQuantity > 0 {
			log := models.StockLog{
				ComponentID:     component.ID,
				ChangeAmount:    preStock.ExpectedQuantity,
				UnitPriceMicro:  unitPriceMicro,
				TotalPriceCents: preStock.TotalPriceCents,
				Reason:          "预入库确认",
			}
			if err := tx.Create(&log).Error; err != nil {
				return err
			}
		}

		now := time.Now()
		updates := map[string]any{
			"component_number": preStock.ComponentNumber,
			"status":           PreStockStatusConfirmed,
			"component_id":     component.ID,
			"confirmed_at":     now,
		}
		if err := tx.Model(&preStock).Updates(updates).Error; err != nil {
			return err
		}

		return tx.Preload("Category").Preload("Supplier").Preload("Component").First(&confirmed, id).Error
	})
	if err != nil {
		return nil, err
	}
	return &confirmed, nil
}
