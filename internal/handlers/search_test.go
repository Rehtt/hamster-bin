package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/models"
)

func TestDatabaseSearch(t *testing.T) {
	db, engine := batchTestDB(t)
	engine.GET("/categories", handlers.NewCategoryHandler(db).GetAll)
	engine.GET("/suppliers", handlers.NewSupplierHandler(db).GetAll)
	engine.GET("/components/options", handlers.NewComponentHandler(db).GetOptions)
	for _, category := range []models.Category{
		{ID: 2, Name: "Chip resistor A", Status: "active"},
		{ID: 3, Name: "Chip resistor B", Status: "active"},
		{ID: 4, Name: "Chip resistor C", Status: "pending"},
		{ID: 5, Name: "Other", Status: "inactive"},
	} {
		if err := db.Create(&category).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&[]models.Supplier{{Name: "Alpha Parts"}, {Name: "Beta Parts"}, {Name: "O'Reilly"}}).Error; err != nil {
		t.Fatal(err)
	}
	for _, c := range []models.Component{
		{Name: "甲", CategoryID: 1, Package: "SOT-23", Location: "Rack A1", Manufacturer: "Alpha Parts"},
		{Name: "乙", CategoryID: 1, Package: "SOT-23", Location: "Rack A1", Manufacturer: "Alpha Parts"},
		{Name: "丙", CategoryID: 1, Package: "0603", Location: "Rack B", Manufacturer: "Beta Parts"},
		{Name: "丁", CategoryID: 1},
	} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
	}

	t.Run("category filtering precedes count and pagination", func(t *testing.T) {
		response := inventoryCall(t, engine, "GET", "/categories?keyword=Chip+resistor&status=active&page=2&page_size=1", nil, 200)
		var data []models.Category
		var pagination struct {
			Total    int64
			Page     int
			PageSize int `json:"page_size"`
		}
		if err := json.Unmarshal(response["data"], &data); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(response["pagination"], &pagination); err != nil {
			t.Fatal(err)
		}
		if len(data) != 1 || data[0].ID != 3 || pagination.Total != 2 || pagination.Page != 2 || pagination.PageSize != 1 {
			t.Fatalf("data=%+v pagination=%+v", data, pagination)
		}
	})
	t.Run("unpaginated compatibility and original category scope", func(t *testing.T) {
		response := inventoryCall(t, engine, "GET", "/categories", nil, 200)
		var data []models.Category
		_ = json.Unmarshal(response["data"], &data)
		if len(data) != 5 || response["pagination"] != nil {
			t.Fatalf("legacy response: %s", response)
		}
		for _, check := range []struct {
			query string
			ids   []uint
		}{
			{"status=active&keyword=Chip&keep_id=4", []uint{2, 3, 4}},
			{"status=active&keyword=Chip&keep_id=5", []uint{2, 3}},
			{"status=active&keyword=absent&keep_id=4", []uint{}},
		} {
			response = inventoryCall(t, engine, "GET", "/categories?"+check.query, nil, 200)
			_ = json.Unmarshal(response["data"], &data)
			if len(data) != len(check.ids) {
				t.Fatalf("%s: %+v", check.query, data)
			}
			for i, id := range check.ids {
				if data[i].ID != id {
					t.Fatalf("%s: %+v", check.query, data)
				}
			}
		}
		for _, query := range []string{"page=0", "page_size=101", "page=x", "page=1&page_size=-1"} {
			inventoryCall(t, engine, "GET", "/categories?"+query, nil, 400)
		}
	})
	t.Run("supplier tokens and bound parameters", func(t *testing.T) {
		for _, check := range []struct {
			keyword string
			count   int
		}{{"parts ALPHA", 1}, {"O'Reilly", 1}, {"' OR 1=1 --", 0}, {"missing", 0}, {"", 3}} {
			response := inventoryCall(t, engine, "GET", "/suppliers?keyword="+url.QueryEscape(check.keyword), nil, 200)
			var data []models.Supplier
			if err := json.Unmarshal(response["data"], &data); err != nil {
				t.Fatal(err)
			}
			if len(data) != check.count {
				t.Fatalf("%q: %+v", check.keyword, data)
			}
		}
	})
	t.Run("distinct historical fields and empty results", func(t *testing.T) {
		for _, check := range []struct{ field, keyword, want string }{{"packages", "SOT", "SOT-23"}, {"locations", "A1 Rack", "Rack A1"}, {"manufacturers", "parts ALPHA", "Alpha Parts"}, {"packages", "unknown", ""}} {
			response := inventoryCall(t, engine, "GET", fmt.Sprintf("/components/options?field=%s&keyword=%s", check.field, url.QueryEscape(check.keyword)), nil, 200)
			var data map[string][]string
			if err := json.Unmarshal(response["data"], &data); err != nil {
				t.Fatal(err)
			}
			values := data[check.field]
			if len(data) != 1 || values == nil || (check.want == "" && len(values) != 0) || (check.want != "" && (len(values) != 1 || values[0] != check.want)) {
				t.Fatalf("%+v: %+v", check, data)
			}
		}
		response := inventoryCall(t, engine, "GET", "/components/options", nil, 200)
		var data map[string][]string
		_ = json.Unmarshal(response["data"], &data)
		if len(data) != 3 || len(data["packages"]) != 2 {
			t.Fatalf("legacy options: %+v", data)
		}
		inventoryCall(t, engine, "GET", "/components/options?field=name", nil, 400)
	})
}
