package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func inventoryCall(t *testing.T, e *gin.Engine, method, path string, payload any, status int) map[string]json.RawMessage {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	e.ServeHTTP(res, req)
	if res.Code != status {
		t.Fatalf("%s %s: %d want %d: %s", method, path, res.Code, status, res.Body)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestInventoryNormalizationAcrossRESTAndBatch(t *testing.T) {
	db, e := batchTestDB(t)
	components := handlers.NewComponentHandler(db)
	pre := handlers.NewPreStockHandler(db)
	n := handlers.NewNormalizationHandler(db)
	e.POST("/components", components.Create)
	e.PUT("/components/:id", components.Update)
	e.POST("/pre-stocks", pre.Create)
	e.PUT("/pre-stocks/:id", pre.Update)
	e.POST("/pre-stocks/:id/confirm", pre.Confirm)
	e.GET("/normalization/standards", n.Standards)
	e.POST("/normalization/preview", n.Preview)
	pending := models.Category{Name: "候选"}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	inventoryCall(t, e, "POST", "/components", map[string]any{"name": "电阻", "category_id": pending.ID}, 422)
	inventoryCall(t, e, "POST", "/components", map[string]any{"name": "Unknown", "category_id": 1, "review": []any{map[string]any{"field": "name", "confirmed": true}}}, 422)
	payload := map[string]any{"name": "Unknown", "description": "Original text", "package": " SOT23 ", "category_id": 1, "stock_quantity": 2, "total_price_cents": 100, "review_confirmations": map[string]string{"name": "Unknown", "description": "Original text"}, "created_at": "2000-01-01T00:00:00Z"}
	response := inventoryCall(t, e, "POST", "/components", payload, 201)
	var c models.Component
	if err := json.Unmarshal(response["data"], &c); err != nil {
		t.Fatal(err)
	}
	if c.Name != "Unknown" || c.Package != "SOT-23" || len(c.Review) != 2 || c.CreatedAt.Year() == 2000 {
		t.Fatalf("create: %+v", c)
	}
	path := fmt.Sprintf("/components/%d", c.ID)
	response = inventoryCall(t, e, "PUT", path, map[string]any{"location": "A1", "created_at": "2000-01-01T00:00:00Z", "stock_quantity": 999, "review": nil}, 200)
	var updated models.Component
	_ = json.Unmarshal(response["data"], &updated)
	if updated.Name != c.Name || updated.StockQuantity != 2 || !updated.CreatedAt.Equal(c.CreatedAt) || len(updated.Review) != 2 {
		t.Fatalf("patch clobbered metadata: %+v", updated)
	}
	inventoryCall(t, e, "PUT", path, map[string]any{"name": "Changed", "review_confirmations": map[string]string{"name": "Unknown"}}, 422)
	response = inventoryCall(t, e, "PUT", path, map[string]any{"description": "", "name": "电阻"}, 200)
	updated = models.Component{}
	_ = json.Unmarshal(response["data"], &updated)
	if updated.Description != "" || len(updated.Review) != 0 {
		t.Fatalf("clear/correct: %+v", updated)
	}
	if err := db.Model(&models.Category{}).Where("id = ?", 1).Update("status", "inactive").Error; err != nil {
		t.Fatal(err)
	}
	inventoryCall(t, e, "PUT", path, map[string]any{"name": "电容", "category_id": 1}, 200)
	inventoryCall(t, e, "POST", "/pre-stocks", map[string]any{"name": "电阻", "category_id": 1}, 422)
	if err := db.Model(&models.Category{}).Where("id = ?", 1).Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}
	response = inventoryCall(t, e, "POST", "/pre-stocks", map[string]any{"name": "Unknown", "category_id": 1, "expected_quantity": 3, "review_confirmations": map[string]string{"name": "Unknown"}}, 201)
	var p models.PreStock
	_ = json.Unmarshal(response["data"], &p)
	if err := db.Model(&models.Category{}).Where("id = ?", 1).Update("status", "inactive").Error; err != nil {
		t.Fatal(err)
	}
	confirmPath := fmt.Sprintf("/pre-stocks/%d/confirm", p.ID)
	inventoryCall(t, e, "POST", confirmPath, map[string]any{}, 422)
	if err := db.Model(&models.Category{}).Where("id = ?", 1).Update("status", "active").Error; err != nil {
		t.Fatal(err)
	}
	inventoryCall(t, e, "POST", confirmPath, map[string]any{}, 200)
	// A late unconfirmed original-text row rolls back every item in the transaction.
	inventoryCall(t, e, "POST", "/components/batch", map[string]any{"items": []any{map[string]any{"name": "电阻", "category_id": 1, "stock_quantity": 1}, map[string]any{"name": "Unknown", "category_id": 1, "stock_quantity": 1}}}, 422)
	var count int64
	db.Model(&models.Component{}).Count(&count)
	if count != 2 {
		t.Fatalf("partial batch persisted: %d", count)
	}
	inventoryCall(t, e, "POST", "/components/batch", map[string]any{"items": []any{map[string]any{"name": "电阻", "category_name": "new", "stock_quantity": 1}}}, 422)
	db.Model(&models.Category{}).Count(&count)
	if count != 2 {
		t.Fatal("batch created a category")
	}
	response = inventoryCall(t, e, "POST", "/normalization/preview", map[string]any{"name": "Resistor", "package": "SOT23", "category_name": "电阻"}, 200)
	if !bytes.Contains(response["data"], []byte(`"name":"电阻"`)) {
		t.Fatalf("preview: %s", response["data"])
	}
	inventoryCall(t, e, "GET", "/normalization/standards", nil, 200)
}
