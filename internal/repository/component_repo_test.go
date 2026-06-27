package repository

import (
	"errors"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupComponentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Supplier{}, &models.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedComponentFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	category := models.Category{Name: "电阻"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	supplier := models.Supplier{Name: "嘉立创"}
	if err := db.Create(&supplier).Error; err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	components := []models.Component{
		{
			CategoryID:         category.ID,
			SupplierID:         &supplier.ID,
			ComponentNumber:    strPtr("HB-000001"),
			Name:               "贴片电阻",
			Model:              "RC0603FR-0710KL",
			Manufacturer:       "YAGEO",
			Value:              "10k 0603",
			SupplierPartNumber: "C2040",
		},
		{
			CategoryID:   category.ID,
			Name:         "贴片电容",
			Manufacturer: "YAGEO",
			Value:        "100nF",
			Model:        "CC0603KRX7R9BB104",
			Description:  "通用去耦电容",
		},
		{
			CategoryID:   category.ID,
			Name:         "ESP32 模块",
			Manufacturer: "Espressif",
			Value:        "WiFi",
		},
	}
	for i := range components {
		if err := db.Create(&components[i]).Error; err != nil {
			t.Fatalf("create component: %v", err)
		}
	}
}

func strPtr(s string) *string {
	return &s
}

func componentNames(components []models.Component) []string {
	names := make([]string, len(components))
	for i, c := range components {
		names[i] = c.Name
	}
	return names
}

func TestComponentRepositoryGetAllFieldFilter(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	t.Run("manufacturer filter", func(t *testing.T) {
		items, total, err := repo.GetAll(ComponentQuery{Manufacturer: "YAGEO"})
		if err != nil {
			t.Fatalf("GetAll: %v", err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
	})

	t.Run("manufacturer filter miss", func(t *testing.T) {
		_, total, err := repo.GetAll(ComponentQuery{Manufacturer: "Murata"})
		if err != nil {
			t.Fatalf("GetAll: %v", err)
		}
		if total != 0 {
			t.Fatalf("total = %d, want 0", total)
		}
	})
}

func TestComponentRepositoryGetAllValueTokensAND(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	items, total, err := repo.GetAll(ComponentQuery{Value: "10k 0603"})
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Name != "贴片电阻" {
		t.Fatalf("got names %v, want [贴片电阻]", componentNames(items))
	}
}

func TestComponentRepositoryGetAllMultiFieldAND(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	items, total, err := repo.GetAll(ComponentQuery{
		Manufacturer: "YAGEO",
		Value:        "100nF",
	})
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Name != "贴片电容" {
		t.Fatalf("got names %v, want [贴片电容]", componentNames(items))
	}
}

func TestComponentRepositoryGetAllSupplierFilter(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	items, total, err := repo.GetAll(ComponentQuery{SupplierName: "嘉立创"})
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Name != "贴片电阻" {
		t.Fatalf("got names %v, want [贴片电阻]", componentNames(items))
	}
}

func TestComponentRepositoryGetAllKeywordTokens(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	items, total, err := repo.GetAll(ComponentQuery{Keyword: "YAGEO 100nF"})
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Name != "贴片电容" {
		t.Fatalf("got names %v, want [贴片电容]", componentNames(items))
	}
}

func TestComponentRepositoryGetAllSort(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	if err := db.Model(&models.Component{}).Where("name = ?", "贴片电阻").Update("stock_quantity", 10).Error; err != nil {
		t.Fatalf("update stock: %v", err)
	}
	if err := db.Model(&models.Component{}).Where("name = ?", "贴片电容").Update("stock_quantity", 5).Error; err != nil {
		t.Fatalf("update stock: %v", err)
	}
	if err := db.Model(&models.Component{}).Where("name = ?", "ESP32 模块").Update("stock_quantity", 20).Error; err != nil {
		t.Fatalf("update stock: %v", err)
	}

	t.Run("sort by name asc", func(t *testing.T) {
		items, _, err := repo.GetAll(ComponentQuery{SortBy: "name", SortOrder: "asc"})
		if err != nil {
			t.Fatalf("GetAll: %v", err)
		}
		want := []string{"ESP32 模块", "贴片电容", "贴片电阻"}
		got := componentNames(items)
		if len(got) != len(want) {
			t.Fatalf("len(items) = %d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got names %v, want %v", got, want)
			}
		}
	})

	t.Run("sort by stock_quantity desc", func(t *testing.T) {
		items, _, err := repo.GetAll(ComponentQuery{SortBy: "stock_quantity", SortOrder: "desc"})
		if err != nil {
			t.Fatalf("GetAll: %v", err)
		}
		want := []string{"ESP32 模块", "贴片电阻", "贴片电容"}
		for i := range want {
			if items[i].Name != want[i] {
				t.Fatalf("got names %v, want %v", componentNames(items), want)
			}
		}
	})
}

func TestComponentRepositoryGetDistinctManufacturers(t *testing.T) {
	db := setupComponentTestDB(t)
	seedComponentFixtures(t, db)
	repo := NewComponentRepository(db)

	manufacturers, err := repo.GetDistinctManufacturers()
	if err != nil {
		t.Fatalf("GetDistinctManufacturers: %v", err)
	}
	if len(manufacturers) != 2 {
		t.Fatalf("len(manufacturers) = %d, want 2", len(manufacturers))
	}
	if manufacturers[0] != "Espressif" || manufacturers[1] != "YAGEO" {
		t.Fatalf("got manufacturers %v, want [Espressif YAGEO]", manufacturers)
	}
}

func setupComponentStockTestDB(t *testing.T) (*gorm.DB, []models.Component) {
	t.Helper()
	db := setupComponentTestDB(t)
	if err := db.AutoMigrate(&models.StockLog{}); err != nil {
		t.Fatalf("migrate stock log: %v", err)
	}
	seedComponentFixtures(t, db)

	var components []models.Component
	if err := db.Find(&components).Error; err != nil {
		t.Fatalf("load components: %v", err)
	}
	for i := range components {
		stock := 10
		unitPrice := int64(0)
		switch components[i].Name {
		case "贴片电阻":
			stock = 100
			unitPrice = 10000 // 0.01 元
		case "贴片电容":
			stock = 50
			unitPrice = 20000
		case "ESP32 模块":
			stock = 5
			unitPrice = 150000000 // 150 元
		}
		if err := db.Model(&components[i]).Updates(map[string]any{
			"stock_quantity":   stock,
			"unit_price_micro": unitPrice,
		}).Error; err != nil {
			t.Fatalf("update component stock: %v", err)
		}
		components[i].StockQuantity = stock
		components[i].UnitPriceMicro = unitPrice
	}
	return db, components
}

func componentByName(components []models.Component, name string) models.Component {
	for _, c := range components {
		if c.Name == name {
			return c
		}
	}
	return models.Component{}
}

func TestComponentRepositoryBatchApplyStockOutSuccess(t *testing.T) {
	db, fixtures := setupComponentStockTestDB(t)
	repo := NewComponentRepository(db)

	resistor := componentByName(fixtures, "贴片电阻")
	capacitor := componentByName(fixtures, "贴片电容")

	updated, failures, err := repo.BatchApplyStockOut([]BatchStockOutItem{
		{ComponentID: resistor.ID, Quantity: 10},
		{ComponentID: capacitor.ID, Quantity: 5},
	}, "项目装配")
	if err != nil {
		t.Fatalf("BatchApplyStockOut: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %#v, want none", failures)
	}
	if len(updated) != 2 {
		t.Fatalf("len(updated) = %d, want 2", len(updated))
	}

	var reloadedResistor models.Component
	if err := db.First(&reloadedResistor, resistor.ID).Error; err != nil {
		t.Fatalf("reload resistor: %v", err)
	}
	if reloadedResistor.StockQuantity != 90 {
		t.Fatalf("resistor stock = %d, want 90", reloadedResistor.StockQuantity)
	}

	var reloadedCapacitor models.Component
	if err := db.First(&reloadedCapacitor, capacitor.ID).Error; err != nil {
		t.Fatalf("reload capacitor: %v", err)
	}
	if reloadedCapacitor.StockQuantity != 45 {
		t.Fatalf("capacitor stock = %d, want 45", reloadedCapacitor.StockQuantity)
	}

	var logCount int64
	if err := db.Model(&models.StockLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if logCount != 2 {
		t.Fatalf("logCount = %d, want 2", logCount)
	}
}

func TestComponentRepositoryBatchApplyStockOutRollback(t *testing.T) {
	db, fixtures := setupComponentStockTestDB(t)
	repo := NewComponentRepository(db)

	resistor := componentByName(fixtures, "贴片电阻")
	esp32 := componentByName(fixtures, "ESP32 模块")

	_, failures, err := repo.BatchApplyStockOut([]BatchStockOutItem{
		{ComponentID: resistor.ID, Quantity: 10},
		{ComponentID: esp32.ID, Quantity: 10},
	}, "项目装配")
	if !errors.Is(err, ErrBatchStockOutFailed) {
		t.Fatalf("err = %v, want ErrBatchStockOutFailed", err)
	}
	if len(failures) != 1 {
		t.Fatalf("len(failures) = %d, want 1", len(failures))
	}
	if failures[0].ComponentID != esp32.ID {
		t.Fatalf("failure component_id = %d, want %d", failures[0].ComponentID, esp32.ID)
	}
	if failures[0].StockQuantity != 5 || failures[0].Requested != 10 {
		t.Fatalf("failure = %#v, want stock=5 requested=10", failures[0])
	}

	var reloadedResistor models.Component
	if err := db.First(&reloadedResistor, resistor.ID).Error; err != nil {
		t.Fatalf("reload resistor: %v", err)
	}
	if reloadedResistor.StockQuantity != 100 {
		t.Fatalf("resistor stock = %d, want 100 (rollback)", reloadedResistor.StockQuantity)
	}

	var logCount int64
	if err := db.Model(&models.StockLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("logCount = %d, want 0 (rollback)", logCount)
	}
}
