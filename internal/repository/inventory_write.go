package repository

import (
	"fmt"
	"math"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/price"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validateInventory(tx *gorm.DB, name string, categoryID uint, supplierID *uint, quantity int, total int64) error {
	if strings.TrimSpace(name) == "" || categoryID == 0 || quantity < 0 || total < 0 {
		return fmt.Errorf("名称、分类 ID 必填，数量与总价必须非负")
	}
	if quantity == 0 && total > 0 {
		return fmt.Errorf("数量为零时不能填写正数总价")
	}
	if total > (math.MaxInt64-int64(quantity)/2)/price.MicroPerCent {
		return fmt.Errorf("总价过大，无法安全换算为微元")
	}
	var category models.Category
	if err := tx.First(&category, categoryID).Error; err != nil {
		return fmt.Errorf("分类不存在或无法读取: %w", err)
	}
	if supplierID != nil {
		if *supplierID == 0 {
			return fmt.Errorf("供应商 ID 必须大于零")
		}
		var supplier models.Supplier
		if err := tx.First(&supplier, *supplierID).Error; err != nil {
			return fmt.Errorf("供应商不存在或无法读取: %w", err)
		}
	}
	return nil
}

// CreateWithInitialStock 将编号分配、价格分摊、元件及初始流水保存置于同一事务。
func (r *ComponentRepository) CreateWithInitialStock(component *models.Component, total int64) (*models.Component, error) {
	var created *models.Component
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := validateInventory(tx, component.Name, component.CategoryID, component.SupplierID, component.StockQuantity, total); err != nil {
			return err
		}
		repo := NewComponentRepository(tx)
		if err := repo.AssignComponentNumberForCreate(component); err != nil {
			return err
		}
		component.UnitPriceMicro = price.UnitPriceMicro(total, component.StockQuantity)
		if err := repo.Create(component); err != nil {
			return err
		}
		if component.StockQuantity > 0 && total > 0 {
			if err := tx.Create(&models.StockLog{ComponentID: component.ID, ChangeAmount: component.StockQuantity, UnitPriceMicro: component.UnitPriceMicro, TotalPriceCents: total, Reason: "初始入库"}).Error; err != nil {
				return err
			}
		}
		var err error
		created, err = repo.GetByID(component.ID)
		return err
	})
	return created, err
}

var detailColumns = map[string]bool{
	"category_id": true, "component_number": true, "name": true, "model": true,
	"manufacturer": true, "value": true, "package": true, "supplier_id": true,
	"supplier_part_number": true, "description": true, "location": true,
	"datasheet_url": true, "image_url": true,
}

func checkDetailColumns(updates map[string]any, preStock bool) error {
	for key := range updates {
		if !detailColumns[key] && !(preStock && (key == "expected_quantity" || key == "total_price_cents")) {
			return fmt.Errorf("不允许更新字段 %s", key)
		}
	}
	return nil
}

// PatchDetails 只写入明确提供的资料字段，不覆盖库存、单价或其他资料。
func (r *ComponentRepository) PatchDetails(id uint, updates map[string]any) (*models.Component, error) {
	if err := checkDetailColumns(updates, false); err != nil {
		return nil, err
	}
	var updated *models.Component
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.Component
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, id).Error; err != nil {
			return err
		}
		if raw, ok := updates["component_number"]; ok {
			number, ok := raw.(string)
			if !ok {
				return fmt.Errorf("元件编号必须为字符串")
			}
			candidate := existing
			candidate.ComponentNumber = &number
			if NormalizeComponentNumber(&number) != nil && (existing.ComponentNumber == nil || strings.TrimSpace(number) != *existing.ComponentNumber) {
				if err := NewComponentRepository(tx).ValidateComponentNumberForUpdate(&candidate, &existing); err != nil {
					return err
				}
				updates["component_number"] = candidate.ComponentNumber
			} else {
				delete(updates, "component_number")
			}
		}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		var err error
		updated, err = NewComponentRepository(tx).GetByID(id)
		if err != nil {
			return err
		}
		return validateInventory(tx, updated.Name, updated.CategoryID, updated.SupplierID, updated.StockQuantity, 0)
	})
	return updated, err
}

// CreateValidated 校验关联记录后创建待入库记录，失败时回滚编号与保存。
func (r *PreStockRepository) CreateValidated(item *models.PreStock) (*models.PreStock, error) {
	var created *models.PreStock
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := validateInventory(tx, item.Name, item.CategoryID, item.SupplierID, item.ExpectedQuantity, item.TotalPriceCents); err != nil {
			return err
		}
		repo := NewPreStockRepository(tx)
		if err := repo.Create(item); err != nil {
			return err
		}
		var err error
		created, err = repo.GetByID(item.ID)
		return err
	})
	return created, err
}

// PatchPending 仅修改待入库记录中提供的字段，并在同一事务内校验完整结果。
func (r *PreStockRepository) PatchPending(id uint, updates map[string]any) (*models.PreStock, error) {
	if err := checkDetailColumns(updates, true); err != nil {
		return nil, err
	}
	var updated *models.PreStock
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.PreStock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, id).Error; err != nil {
			return err
		}
		if existing.Status != PreStockStatusPending {
			return ErrPreStockAlreadyConfirmed
		}
		if raw, ok := updates["component_number"]; ok {
			number, ok := raw.(string)
			if !ok {
				return fmt.Errorf("元件编号必须为字符串")
			}
			existing.ComponentNumber = &number
			if err := r.assignNumberInTx(tx, &existing); err != nil {
				return err
			}
			updates["component_number"] = existing.ComponentNumber
		}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		var err error
		updated, err = NewPreStockRepository(tx).GetByID(id)
		if err != nil {
			return err
		}
		return validateInventory(tx, updated.Name, updated.CategoryID, updated.SupplierID, updated.ExpectedQuantity, updated.TotalPriceCents)
	})
	return updated, err
}

// ConfirmValidated 校验待入库数据与关联记录后复用 Confirm，整次操作共用事务。
func (r *PreStockRepository) ConfirmValidated(id uint) (*models.PreStock, error) {
	var confirmed *models.PreStock
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var item models.PreStock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error; err != nil {
			return err
		}
		if item.Status != PreStockStatusPending {
			return ErrPreStockAlreadyConfirmed
		}
		if err := validateInventory(tx, item.Name, item.CategoryID, item.SupplierID, item.ExpectedQuantity, item.TotalPriceCents); err != nil {
			return err
		}
		var err error
		confirmed, err = NewPreStockRepository(tx).Confirm(id)
		return err
	})
	return confirmed, err
}
