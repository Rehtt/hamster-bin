package repository

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func categoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "categories.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Category{}, &models.Supplier{}, &models.Component{}, &models.PreStock{}, &models.StockLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func categorySeed(t *testing.T, db *gorm.DB, name string, parent *uint) models.Category {
	t.Helper()
	c := models.Category{Name: name, ParentID: parent}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c
}

func categorySnapshot(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var categories []models.Category
	var components []models.Component
	var preStocks []models.PreStock
	var logs []models.StockLog
	for _, dest := range []any{&categories, &components, &preStocks, &logs} {
		if err := db.Order("id").Find(dest).Error; err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.Marshal([]any{categories, components, preStocks, logs})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCategoryDelete(t *testing.T) {
	for _, kind := range []string{"empty", "component", "zero_stock", "pending", "confirmed", "other_status", "child", "all"} {
		t.Run(kind, func(t *testing.T) {
			db := categoryTestDB(t)
			repo := NewCategoryRepository(db)
			category := categorySeed(t, db, "目标", nil)
			categorySeed(t, db, "无关分类", nil)
			if kind == "component" || kind == "zero_stock" || kind == "all" {
				quantity := 10
				if kind == "zero_stock" {
					quantity = 0
				}
				component := models.Component{Name: "元件", CategoryID: category.ID, StockQuantity: quantity}
				if err := db.Create(&component).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&models.StockLog{ComponentID: component.ID, ChangeAmount: quantity}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if kind == "pending" || kind == "confirmed" || kind == "other_status" || kind == "all" {
				status := kind
				if kind == "all" {
					status = "pending"
				}
				if err := db.Create(&models.PreStock{Name: "预入库", CategoryID: category.ID, Status: status}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if kind == "child" || kind == "all" {
				categorySeed(t, db, "子分类", &category.ID)
			}
			before := categorySnapshot(t, db)
			err := repo.Delete(category.ID)
			if kind == "empty" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := repo.GetByID(category.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("deleted category: %v", err)
				}
				if err := repo.Delete(category.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
					t.Fatalf("repeat delete: %v", err)
				}
				remaining, err := repo.GetAll()
				if err != nil || len(remaining) != 1 || remaining[0].Name != "无关分类" {
					t.Fatalf("unrelated category changed: %+v, %v", remaining, err)
				}
			} else {
				if !errors.Is(err, ErrCategoryInUse) {
					t.Fatalf("delete: %v", err)
				}
				if after := categorySnapshot(t, db); after != before {
					t.Fatalf("failed deletion changed data: %s -> %s", before, after)
				}
			}
		})
	}
}

func TestCategoryUpdate(t *testing.T) {
	db := categoryTestDB(t)
	repo := NewCategoryRepository(db)
	root := categorySeed(t, db, "根", nil)
	other := categorySeed(t, db, "另一根", nil)
	child := categorySeed(t, db, "子", &root.ID)
	leaf := categorySeed(t, db, "孙", &child.ID)
	name := "  新名称\t"
	updated, err := repo.Update(child.ID, CategoryPatch{Name: &name})
	if err != nil || updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != root.ID {
		t.Fatalf("rename: %+v, %v", updated, err)
	}
	updated, err = repo.Update(child.ID, CategoryPatch{ParentID: &other.ID})
	if err != nil || updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != other.ID {
		t.Fatalf("move: %+v, %v", updated, err)
	}
	before := categorySnapshot(t, db)
	updated, err = repo.Update(child.ID, CategoryPatch{})
	if err != nil || updated.ID != child.ID || updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != other.ID || categorySnapshot(t, db) != before {
		t.Fatalf("no-op: %+v, %v", updated, err)
	}
	zero := uint(0)
	updated, err = repo.Update(child.ID, CategoryPatch{ParentID: &zero})
	if err != nil || updated.ParentID != nil || updated.Name != "新名称" {
		t.Fatalf("top level: %+v, %v", updated, err)
	}
	blank, rename := " \t\n", "不应保存"
	missing := uint(9999)
	for _, tc := range []struct {
		name  string
		id    uint
		patch CategoryPatch
		want  error
	}{
		{"empty name", child.ID, CategoryPatch{Name: &blank}, ErrInvalidCategory},
		{"missing parent", child.ID, CategoryPatch{Name: &rename, ParentID: &missing}, ErrInvalidCategory},
		{"self", child.ID, CategoryPatch{ParentID: &child.ID}, ErrInvalidCategory},
		{"descendant", child.ID, CategoryPatch{ParentID: &leaf.ID}, ErrInvalidCategory},
		{"missing target", missing, CategoryPatch{Name: &rename}, gorm.ErrRecordNotFound},
		{"zero target", zero, CategoryPatch{Name: &rename}, ErrInvalidCategory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := categorySnapshot(t, db)
			if _, err := repo.Update(tc.id, tc.patch); !errors.Is(err, tc.want) {
				t.Fatalf("update: %v, want %v", err, tc.want)
			}
			if categorySnapshot(t, db) != before {
				t.Fatal("failed update changed data")
			}
		})
	}
}

func TestCategoryCreateAndExistingCycle(t *testing.T) {
	db := categoryTestDB(t)
	repo := NewCategoryRepository(db)
	root := categorySeed(t, db, "根", nil)
	zero := uint(0)
	c := models.Category{Name: "  新分类\n", ParentID: &zero}
	if err := repo.Create(&c); err != nil || c.ID == 0 || c.Name != "新分类" || c.ParentID != nil {
		t.Fatalf("create: %+v, %v", c, err)
	}
	child := models.Category{Name: "子分类", ParentID: &root.ID}
	if err := repo.Create(&child); err != nil || child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("create child: %+v, %v", child, err)
	}
	// 直接写入历史循环，验证遍历能终止且不会把新记录接入循环。
	if err := db.Model(&root).Update("parent_id", child.ID).Error; err != nil {
		t.Fatal(err)
	}
	missing := uint(9999)
	for _, candidate := range []models.Category{
		{Name: " "}, {Name: "无效父分类", ParentID: &missing}, {Name: "循环父链", ParentID: &root.ID}, {ID: missing, Name: "自身", ParentID: &missing},
	} {
		before := categorySnapshot(t, db)
		if err := repo.Create(&candidate); !errors.Is(err, ErrInvalidCategory) {
			t.Fatalf("create: %v", err)
		}
		if categorySnapshot(t, db) != before {
			t.Fatal("failed creation changed data")
		}
	}
	before := categorySnapshot(t, db)
	if _, err := repo.Update(c.ID, CategoryPatch{ParentID: &root.ID}); !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("existing cycle: %v", err)
	}
	if categorySnapshot(t, db) != before {
		t.Fatal("cycle update changed data")
	}
	// 可通过移为顶级分类修复历史循环。
	if _, err := repo.Update(root.ID, CategoryPatch{ParentID: &zero}); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryDatabaseFailures(t *testing.T) {
	for _, kind := range []string{"read", "association", "delete", "update", "parent"} {
		t.Run(kind, func(t *testing.T) {
			db := categoryTestDB(t)
			repo := NewCategoryRepository(db)
			c := categorySeed(t, db, "分类", nil)
			if kind == "read" {
				if err := db.Migrator().DropTable(&models.Category{}); err != nil {
					t.Fatal(err)
				}
			} else if kind == "association" {
				if err := db.Migrator().DropTable(&models.PreStock{}); err != nil {
					t.Fatal(err)
				}
			} else if kind == "parent" {
				queries := 0
				if err := db.Callback().Query().Before("gorm:query").Register("fail_parent", func(tx *gorm.DB) {
					queries++
					if queries == 2 {
						tx.AddError(errors.New("forced parent read failure"))
					}
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				event := "DELETE"
				if kind == "update" {
					event = "UPDATE"
				}
				if err := db.Exec("CREATE TRIGGER reject_category BEFORE " + event + " ON categories BEGIN SELECT RAISE(ABORT, 'forced failure'); END").Error; err != nil {
					t.Fatal(err)
				}
			}
			var err error
			if kind == "parent" {
				missing := uint(9999)
				_, err = repo.Update(c.ID, CategoryPatch{ParentID: &missing})
			} else if kind == "update" {
				name := "修改"
				_, err = repo.Update(c.ID, CategoryPatch{Name: &name})
			} else {
				err = repo.Delete(c.ID)
			}
			if err == nil || errors.Is(err, ErrCategoryInUse) || errors.Is(err, ErrInvalidCategory) || errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("database failure misclassified: %v", err)
			}
			if kind != "read" {
				current, err := repo.GetByID(c.ID)
				if err != nil || current.Name != c.Name {
					t.Fatalf("failed write changed category: %+v, %v", current, err)
				}
			}
		})
	}
}
