package repository

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NormalizationRepository struct{ db *gorm.DB }

func NewNormalizationRepository(db *gorm.DB) *NormalizationRepository {
	return &NormalizationRepository{db: db}
}
func (r *NormalizationRepository) Standards(ctx context.Context) (normalization.Standards, error) {
	s := normalization.Rules()
	var categories []models.Category
	if err := r.db.WithContext(ctx).Where("status = ?", "active").Order("id").Find(&categories).Error; err != nil {
		return s, err
	}
	for _, c := range categories {
		s.Categories = append(s.Categories, normalization.Category{ID: c.ID, Name: c.Name})
	}
	for _, field := range []string{"name", "package", "manufacturer"} {
		var values []string
		for _, table := range []string{"components", "pre_stocks"} {
			var entries []string
			if err := r.db.WithContext(ctx).Table(table).Distinct(field).Where(field+" <> ?", "").Order(field).Pluck(field, &entries).Error; err != nil {
				return s, err
			}
			values = append(values, entries...)
		}
		slices.Sort(values)
		s.Historical[field] = slices.Compact(values)
	}
	var candidates []string
	if err := r.db.WithContext(ctx).Model(&models.Category{}).Where("status <> ?", "active").Order("name").Pluck("name", &candidates).Error; err != nil {
		return s, err
	}
	s.Historical["categories"] = candidates
	return s, nil
}
func componentFields(c *models.Component) normalization.Fields {
	return normalization.Fields{Name: c.Name, Description: c.Description, Model: c.Model, Manufacturer: c.Manufacturer, Package: c.Package, CategoryID: c.CategoryID}
}
func preStockFields(c *models.PreStock) normalization.Fields {
	return normalization.Fields{Name: c.Name, Description: c.Description, Model: c.Model, Manufacturer: c.Manufacturer, Package: c.Package, CategoryID: c.CategoryID}
}
func validateMaterials(tx *gorm.DB, f, old normalization.Fields, previous []normalization.Review, confirmations map[string]string, creating, confirming bool) ([]normalization.Review, error) {
	if creating || confirming || f.CategoryID != old.CategoryID {
		var category models.Category
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&category, f.CategoryID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, normalization.Invalid("category_id", "category_unmatched", "分类不存在")
		}
		if err != nil {
			return nil, err
		}
		if category.Status != "active" {
			return nil, normalization.Invalid("category_id", "category_inactive", "请选择经审核启用的分类")
		}
	}
	if strings.TrimSpace(f.Name) == "" {
		return nil, normalization.Invalid("name", "required", "名称不能为空")
	}
	return normalization.ReviewFields(f, old, previous, confirmations, creating)
}

// normalizePatch validates a complete candidate within the transaction, retaining field presence.
func normalizePatch(tx *gorm.DB, existing any, updates map[string]any, confirmations map[string]string) error {
	raw, _ := json.Marshal(existing)
	var candidate models.Component
	var old normalization.Fields
	var previous []normalization.Review
	if p, ok := existing.(*models.PreStock); ok {
		old = preStockFields(p)
		previous = p.Review
	} else {
		c := existing.(*models.Component)
		old = componentFields(c)
		previous = c.Review
	}
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return err
	}
	patch, _ := json.Marshal(updates)
	if err := json.Unmarshal(patch, &candidate); err != nil {
		return err
	}
	f := componentFields(&candidate)
	review, err := validateMaterials(tx, f, old, previous, confirmations, false, false)
	if err != nil {
		return err
	}
	if f.Package != old.Package {
		updates["package"] = normalization.Package(f.Package)
	}
	// GORM map updates do not apply field serializers.
	serialized, err := json.Marshal(review)
	if err != nil {
		return err
	}
	updates["review"] = string(serialized)
	return nil
}

func firstConfirmation(values []map[string]string) map[string]string {
	if len(values) > 0 {
		return values[0]
	}
	return nil
}

// detailMap is used only by legacy full-detail repository methods. HTTP uses presence-aware patches.
func detailMap(value any) map[string]any {
	var c models.Component
	switch v := value.(type) {
	case *models.Component:
		c = *v
	case *models.PreStock:
		c = models.Component{CategoryID: v.CategoryID, ComponentNumber: v.ComponentNumber, Name: v.Name, Model: v.Model, Manufacturer: v.Manufacturer, Value: v.Value, Package: v.Package, SupplierID: v.SupplierID, SupplierPartNumber: v.SupplierPartNumber, Description: v.Description, Location: v.Location, DatasheetURL: v.DatasheetURL, ImageURL: v.ImageURL}
	}
	fields := map[string]any{"category_id": c.CategoryID, "name": c.Name, "model": c.Model, "manufacturer": c.Manufacturer, "value": c.Value, "package": c.Package, "supplier_id": c.SupplierID, "supplier_part_number": c.SupplierPartNumber, "description": c.Description, "location": c.Location, "datasheet_url": c.DatasheetURL, "image_url": c.ImageURL}
	if c.ComponentNumber != nil {
		fields["component_number"] = *c.ComponentNumber
	}
	return fields
}
