package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidCategory = errors.New("分类参数无效")
	ErrCategoryInUse   = errors.New("分类存在关联")
)

// CategoryPatch 仅更新明确提供的字段；ParentID 为 0 时解除父分类关联。
type CategoryPatch struct {
	Name     *string `json:"name"`
	ParentID *uint   `json:"parent_id"`
}

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// GetAll 获取所有分类
func (r *CategoryRepository) GetAll() ([]models.Category, error) {
	var categories []models.Category
	err := r.db.Find(&categories).Error
	return categories, err
}

// GetByID 根据ID获取分类
func (r *CategoryRepository) GetByID(id uint) (*models.Category, error) {
	var category models.Category
	err := r.db.First(&category, id).Error
	return &category, err
}

// Create 创建分类
func (r *CategoryRepository) Create(category *models.Category) error {
	name, err := categoryName(category.Name)
	if err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := validateCategoryParent(tx, category.ID, category.ParentID); err != nil {
			return err
		}
		category.Name = name
		category.ParentID = categoryParent(category.ParentID)
		return tx.Create(category).Error
	})
}

// Update 在事务内读取并局部更新，不通过 Save 创建不存在的分类。
func (r *CategoryRepository) Update(id uint, patch CategoryPatch) (*models.Category, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: ID 必须大于零", ErrInvalidCategory)
	}
	var category models.Category
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&category, id).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if patch.Name != nil {
			name, err := categoryName(*patch.Name)
			if err != nil {
				return err
			}
			updates["name"] = name
			category.Name = name
		}
		if patch.ParentID != nil {
			if err := validateCategoryParent(tx, id, patch.ParentID); err != nil {
				return err
			}
			category.ParentID = categoryParent(patch.ParentID)
			updates["parent_id"] = category.ParentID
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&models.Category{}).Where("id = ?", id).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func categoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: 分类名称不能为空", ErrInvalidCategory)
	}
	return name, nil
}

func categoryParent(parentID *uint) *uint {
	if parentID == nil || *parentID == 0 {
		return nil
	}
	id := *parentID
	return &id
}

// 遍历父链，同时拒绝自引用、后代引用以及历史数据中已有的循环。
func validateCategoryParent(tx *gorm.DB, id uint, parentID *uint) error {
	visited := map[uint]bool{}
	for parentID != nil && *parentID != 0 {
		if id != 0 && *parentID == id {
			return fmt.Errorf("%w: 父分类不能是自身或后代分类", ErrInvalidCategory)
		}
		if visited[*parentID] {
			return fmt.Errorf("%w: 父分类层级存在循环", ErrInvalidCategory)
		}
		visited[*parentID] = true
		var parent models.Category
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent, *parentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: 父分类不存在", ErrInvalidCategory)
			}
			return err
		}
		parentID = parent.ParentID
	}
	return nil
}

// Delete 将目标读取、所有状态的关联检查和删除放在同一事务，不迁移或级联关联数据。
func (r *CategoryRepository) Delete(id uint) error {
	if id == 0 {
		return fmt.Errorf("%w: ID 必须大于零", ErrInvalidCategory)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var category models.Category
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&category, id).Error; err != nil {
			return err
		}
		var linked []string
		for _, check := range []struct {
			model any
			field string
			label string
		}{
			{&models.Component{}, "category_id", "元件（包括零库存元件）"},
			{&models.PreStock{}, "category_id", "预入库记录（包括待入库和已确认记录）"},
			{&models.Category{}, "parent_id", "子分类"},
		} {
			var count int64
			if err := tx.Model(check.model).Where(check.field+" = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				linked = append(linked, check.label)
			}
		}
		if len(linked) > 0 {
			return fmt.Errorf("%w: 分类关联了%s，不能删除", ErrCategoryInUse, strings.Join(linked, "、"))
		}
		return tx.Delete(&category).Error
	})
}
