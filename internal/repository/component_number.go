package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/models"
	"gorm.io/gorm"
)

var ErrComponentNumberDuplicate = errors.New("元件编号已存在")

const componentNumberPrefix = "HB-"

// NormalizeComponentNumber 去除首尾空格，空字符串转为 nil。
func NormalizeComponentNumber(num *string) *string {
	if num == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*num)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func formatHBComponentNumber(seq int) string {
	return fmt.Sprintf("%s%06d", componentNumberPrefix, seq)
}

func (r *ComponentRepository) getMaxHBSequence(tx *gorm.DB) (int, error) {
	var numbers []string
	err := tx.Model(&models.Component{}).
		Where("component_number LIKE ?", componentNumberPrefix+"%").
		Pluck("component_number", &numbers).Error
	if err != nil {
		return 0, err
	}

	max := 0
	for _, number := range numbers {
		if !strings.HasPrefix(number, componentNumberPrefix) {
			continue
		}
		seq, err := strconv.Atoi(strings.TrimPrefix(number, componentNumberPrefix))
		if err == nil && seq > max {
			max = seq
		}
	}
	return max, nil
}

func (r *ComponentRepository) generateNextInTx(tx *gorm.DB) (string, error) {
	max, err := r.getMaxHBSequence(tx)
	if err != nil {
		return "", err
	}
	return formatHBComponentNumber(max + 1), nil
}

// IsComponentNumberTaken 检查编号是否已被其他元件使用。
func (r *ComponentRepository) IsComponentNumberTaken(number string, excludeID uint) (bool, error) {
	var count int64
	db := r.db.Model(&models.Component{}).Where("component_number = ?", number)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	err := db.Count(&count).Error
	return count > 0, err
}

// AssignComponentNumberForCreate 创建前分配编号：留空则自动生成，手动填写则校验唯一性。
func (r *ComponentRepository) AssignComponentNumberForCreate(component *models.Component) error {
	component.ComponentNumber = NormalizeComponentNumber(component.ComponentNumber)
	if component.ComponentNumber != nil {
		taken, err := r.IsComponentNumberTaken(*component.ComponentNumber, 0)
		if err != nil {
			return err
		}
		if taken {
			return ErrComponentNumberDuplicate
		}
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		number, err := r.generateNextInTx(tx)
		if err != nil {
			return err
		}
		component.ComponentNumber = &number
		return nil
	})
}

// ValidateComponentNumberForUpdate 更新前校验编号：留空则保留原编号，原编号也为空则自动生成。
func (r *ComponentRepository) ValidateComponentNumberForUpdate(component *models.Component, existing *models.Component) error {
	normalized := NormalizeComponentNumber(component.ComponentNumber)
	if normalized == nil {
		if existing.ComponentNumber != nil && strings.TrimSpace(*existing.ComponentNumber) != "" {
			component.ComponentNumber = existing.ComponentNumber
			return nil
		}
		return r.db.Transaction(func(tx *gorm.DB) error {
			number, err := r.generateNextInTx(tx)
			if err != nil {
				return err
			}
			component.ComponentNumber = &number
			return nil
		})
	}

	taken, err := r.IsComponentNumberTaken(*normalized, component.ID)
	if err != nil {
		return err
	}
	if taken {
		return ErrComponentNumberDuplicate
	}
	component.ComponentNumber = normalized
	return nil
}

// GenerateMissingComponentNumbers 为所有未编号元件按 id 顺序批量生成 HB-xxxxxx 编号。
func (r *ComponentRepository) GenerateMissingComponentNumbers() (int64, error) {
	var updated int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var components []models.Component
		if err := tx.Where("component_number IS NULL OR component_number = ''").
			Order("id ASC").
			Find(&components).Error; err != nil {
			return err
		}
		if len(components) == 0 {
			return nil
		}

		max, err := r.getMaxHBSequence(tx)
		if err != nil {
			return err
		}

		for _, component := range components {
			max++
			number := formatHBComponentNumber(max)
			if err := tx.Model(&component).Update("component_number", number).Error; err != nil {
				return err
			}
			updated++
		}
		return nil
	})
	return updated, err
}
