package repository

import (
	"context"
	"github.com/Rehtt/hamster-bin/internal/models"
	"testing"
)

func TestCategoryReviewAndHistoricalSeparation(t *testing.T) {
	db := categoryTestDB(t)
	repo := NewCategoryRepository(db)
	category := models.Category{Name: "电阻"}
	if err := repo.Create(&category); err != nil {
		t.Fatal(err)
	}
	if category.Status != "pending" {
		t.Fatal("category not pending")
	}
	active := "active"
	if _, err := repo.Update(category.ID, CategoryPatch{Status: &active}); err != nil {
		t.Fatal(err)
	}
	duplicate := models.Category{Name: "电阻", Status: "active"}
	if err := repo.Create(&duplicate); err == nil {
		t.Fatal("duplicate active accepted")
	}
	name := "新名称"
	renamed, err := repo.Update(category.ID, CategoryPatch{Name: &name, Status: &active})
	if err != nil || renamed.Status != "pending" {
		t.Fatal(renamed, err)
	}
	legacy := models.Component{Name: "Legacy English", Package: "SOT23", CategoryID: category.ID}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := NewComponentRepository(db).PatchDetails(legacy.ID, map[string]any{"location": "A"})
	if err != nil || updated.Name != legacy.Name || updated.Package != "SOT23" || len(updated.Review) != 0 {
		t.Fatal(updated, err)
	}
	s, err := NewNormalizationRepository(db).Standards(context.Background())
	if err != nil || len(s.Categories) != 0 || len(s.Historical["name"]) != 1 || len(s.CategoryAliases) != 0 {
		t.Fatal(s, err)
	}
}
