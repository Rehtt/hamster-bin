package handlers

import (
	"encoding/json"
	"errors"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

type NormalizationHandler struct {
	repo *repository.NormalizationRepository
}

func NewNormalizationHandler(db *gorm.DB) *NormalizationHandler {
	return &NormalizationHandler{repository.NewNormalizationRepository(db)}
}
func (h *NormalizationHandler) Standards(c *gin.Context) {
	s, err := h.repo.Standards(c.Request.Context())
	if err != nil {
		inventoryWriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": s})
}
func (h *NormalizationHandler) Preview(c *gin.Context) {
	var fields normalization.Fields
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(400, gin.H{"error": "请求参数错误"})
		return
	}
	s, err := h.repo.Standards(c.Request.Context())
	if err != nil {
		inventoryWriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": normalization.Preview(fields, s)})
}
func inventoryWriteError(c *gin.Context, err error) {
	var validation *normalization.ValidationError
	switch {
	case errors.As(err, &validation):
		c.JSON(422, gin.H{"error": err.Error(), "issues": validation.Issues, "item": validation.Item})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "记录不存在"})
	case errors.Is(err, repository.ErrComponentNumberDuplicate), errors.Is(err, repository.ErrPreStockAlreadyConfirmed):
		c.JSON(400, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
	}
}
func readDetailPatch(c *gin.Context, preStock bool) (map[string]any, map[string]string, error) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		return nil, nil, err
	}
	updates := map[string]any{}
	confirmations := map[string]string{}
	strings := []string{"component_number", "name", "model", "manufacturer", "value", "package", "supplier_part_number", "description", "location", "datasheet_url", "image_url"}
	for _, key := range strings {
		if v, ok := raw[key]; ok {
			var value string
			if err := json.Unmarshal(v, &value); err != nil {
				return nil, nil, err
			}
			updates[key] = value
		}
	}
	for _, key := range []string{"category_id", "supplier_id"} {
		if v, ok := raw[key]; ok {
			if string(v) == "null" && key == "supplier_id" {
				updates[key] = nil
				continue
			}
			var value uint
			if err := json.Unmarshal(v, &value); err != nil {
				return nil, nil, err
			}
			updates[key] = value
		}
	}
	if preStock {
		for _, key := range []string{"expected_quantity", "total_price_cents"} {
			if v, ok := raw[key]; ok {
				var value int64
				if err := json.Unmarshal(v, &value); err != nil {
					return nil, nil, err
				}
				updates[key] = value
			}
		}
	}
	if v, ok := raw["review_confirmations"]; ok {
		if err := json.Unmarshal(v, &confirmations); err != nil {
			return nil, nil, err
		}
	}
	return updates, confirmations, nil
}
