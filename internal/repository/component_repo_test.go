package repository

import (
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
			CategoryID:      category.ID,
			Name:            "贴片电容",
			Manufacturer:    "YAGEO",
			Value:           "100nF",
			Model:           "CC0603KRX7R9BB104",
			Description:     "通用去耦电容",
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
