package repository

import (
	"errors"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPreStockTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Supplier{}, &models.Component{}, &models.PreStock{}, &models.StockLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedPreStockCategory(t *testing.T, db *gorm.DB) models.Category {
	t.Helper()
	category := models.Category{Name: "电阻"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	return category
}

func TestPreStockCreateUsesSharedHBSequence(t *testing.T) {
	db := setupPreStockTestDB(t)
	category := seedPreStockCategory(t, db)
	if err := db.Create(&models.Component{
		CategoryID:      category.ID,
		ComponentNumber: strPtr("HB-000003"),
		Name:            "已有元件",
	}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := db.Create(&models.PreStock{
		CategoryID:      category.ID,
		ComponentNumber: strPtr("HB-000005"),
		Name:            "已有预入库",
		Status:          PreStockStatusPending,
	}).Error; err != nil {
		t.Fatalf("create pre stock: %v", err)
	}

	repo := NewPreStockRepository(db)
	item := models.PreStock{
		CategoryID:       category.ID,
		Name:             "新预入库",
		ExpectedQuantity: 10,
	}
	if err := repo.Create(&item); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if item.ComponentNumber == nil || *item.ComponentNumber != "HB-000006" {
		t.Fatalf("component_number = %v, want HB-000006", item.ComponentNumber)
	}
}

func TestPreStockCreateRejectsDuplicateNumberAcrossTables(t *testing.T) {
	db := setupPreStockTestDB(t)
	category := seedPreStockCategory(t, db)
	if err := db.Create(&models.Component{
		CategoryID:      category.ID,
		ComponentNumber: strPtr("HB-000001"),
		Name:            "已有元件",
	}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	repo := NewPreStockRepository(db)
	item := models.PreStock{
		CategoryID:      category.ID,
		ComponentNumber: strPtr("HB-000001"),
		Name:            "重复编号预入库",
	}
	err := repo.Create(&item)
	if !errors.Is(err, ErrComponentNumberDuplicate) {
		t.Fatalf("Create error = %v, want ErrComponentNumberDuplicate", err)
	}
}

func TestPreStockConfirmCreatesComponentAndStockLog(t *testing.T) {
	db := setupPreStockTestDB(t)
	category := seedPreStockCategory(t, db)
	supplier := models.Supplier{Name: "嘉立创"}
	if err := db.Create(&supplier).Error; err != nil {
		t.Fatalf("create supplier: %v", err)
	}

	repo := NewPreStockRepository(db)
	item := models.PreStock{
		CategoryID:         category.ID,
		SupplierID:         &supplier.ID,
		ComponentNumber:    strPtr("HB-000010"),
		Name:               "贴片电阻",
		Model:              "RC0603FR-0710KL",
		Manufacturer:       "YAGEO",
		Value:              "10k",
		Package:            "0603",
		SupplierPartNumber: "C2040",
		ExpectedQuantity:   50,
		TotalPriceCents:    1234,
		Location:           "A1-03",
	}
	if err := repo.Create(&item); err != nil {
		t.Fatalf("Create: %v", err)
	}

	confirmed, err := repo.Confirm(item.ID)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if confirmed.Status != PreStockStatusConfirmed {
		t.Fatalf("status = %s, want confirmed", confirmed.Status)
	}
	if confirmed.ComponentID == nil {
		t.Fatal("expected component_id")
	}
	if confirmed.ConfirmedAt == nil {
		t.Fatal("expected confirmed_at")
	}

	var component models.Component
	if err := db.First(&component, *confirmed.ComponentID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	if component.ComponentNumber == nil || *component.ComponentNumber != "HB-000010" {
		t.Fatalf("component_number = %v, want HB-000010", component.ComponentNumber)
	}
	if component.StockQuantity != 50 {
		t.Fatalf("stock_quantity = %d, want 50", component.StockQuantity)
	}
	if component.UnitPriceMicro != 246800 {
		t.Fatalf("unit_price_micro = %d, want 246800", component.UnitPriceMicro)
	}

	var log models.StockLog
	if err := db.Where("component_id = ?", component.ID).First(&log).Error; err != nil {
		t.Fatalf("load stock log: %v", err)
	}
	if log.ChangeAmount != 50 || log.TotalPriceCents != 1234 || log.Reason != "预入库确认" {
		t.Fatalf("unexpected stock log: %+v", log)
	}
}

func TestPreStockConfirmedRecordCannotBeMutated(t *testing.T) {
	db := setupPreStockTestDB(t)
	category := seedPreStockCategory(t, db)
	repo := NewPreStockRepository(db)
	item := models.PreStock{
		CategoryID:       category.ID,
		Name:             "贴片电容",
		ExpectedQuantity: 10,
	}
	if err := repo.Create(&item); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.Confirm(item.ID); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	if _, err := repo.Confirm(item.ID); !errors.Is(err, ErrPreStockAlreadyConfirmed) {
		t.Fatalf("Confirm again error = %v, want ErrPreStockAlreadyConfirmed", err)
	}

	item.Name = "改名"
	item.Status = PreStockStatusConfirmed
	if err := repo.Update(&item); !errors.Is(err, ErrPreStockAlreadyConfirmed) {
		t.Fatalf("Update error = %v, want ErrPreStockAlreadyConfirmed", err)
	}

	if err := repo.Delete(item.ID); !errors.Is(err, ErrPreStockAlreadyConfirmed) {
		t.Fatalf("Delete error = %v, want ErrPreStockAlreadyConfirmed", err)
	}
}
