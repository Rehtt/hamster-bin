package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func batchTestDB(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "batch.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
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
	engine := gin.New()
	handler := handlers.NewBatchHandler(db)
	engine.POST("/components/batch", handler.CreateComponents)
	engine.POST("/pre-stocks/batch", handler.CreatePreStocks)
	return db, engine
}

func batchCall(t *testing.T, engine *gin.Engine, path, body string, status int) []byte {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	if res.Code != status {
		t.Fatalf("%s: status %d, want %d: %s", path, res.Code, status, res.Body)
	}
	return res.Body.Bytes()
}

func assertBatchCounts(t *testing.T, db *gorm.DB, wantComponents, wantPreStocks, wantCategories, wantSuppliers, wantLogs int64) {
	t.Helper()
	for _, check := range []struct {
		model any
		want  int64
	}{{&models.Component{}, wantComponents}, {&models.PreStock{}, wantPreStocks}, {&models.Category{}, wantCategories}, {&models.Supplier{}, wantSuppliers}, {&models.StockLog{}, wantLogs}} {
		var count int64
		if err := db.Model(check.model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.want {
			t.Fatalf("%T: count %d, want %d", check.model, count, check.want)
		}
	}
}

func TestBatchQuantityIsExplicitNonnegativeInteger(t *testing.T) {
	for _, endpoint := range []struct{ path, quantity string }{{"/components/batch", "stock_quantity"}, {"/pre-stocks/batch", "expected_quantity"}} {
		for _, value := range []string{"missing", "null", "-1", "0.5", `"2"`, "0", "3"} {
			t.Run(endpoint.path+"/"+value, func(t *testing.T) {
				db, engine := batchTestDB(t)
				quantity := ""
				if value != "missing" {
					quantity = fmt.Sprintf(",%q:%s", endpoint.quantity, value)
				}
				// A valid first row would create all related records before a later validation failure.
				body := fmt.Sprintf(`{"items":[{"name":"first","category_name":"new-category","supplier_name":"new-supplier",%q:2,"total_price_cents":99},{"name":"second","category_name":"other-category"%s}]}`, endpoint.quantity, quantity)
				status := 400
				if value == "0" || value == "3" {
					status = 201
				}
				batchCall(t, engine, endpoint.path, body, status)
				if status == 400 {
					assertBatchCounts(t, db, 0, 0, 0, 0, 0)
				} else if endpoint.quantity == "stock_quantity" {
					var rows []models.Component
					if err := db.Order("id").Find(&rows).Error; err != nil {
						t.Fatal(err)
					}
					want := 0
					if value == "3" {
						want = 3
					}
					if len(rows) != 2 || rows[1].StockQuantity != want {
						t.Fatalf("quantity not saved: %+v", rows)
					}
				} else {
					var rows []models.PreStock
					if err := db.Order("id").Find(&rows).Error; err != nil {
						t.Fatal(err)
					}
					want := 0
					if value == "3" {
						want = 3
					}
					if len(rows) != 2 || rows[1].ExpectedQuantity != want {
						t.Fatalf("quantity not saved: %+v", rows)
					}
				}
			})
		}
	}
}

func TestBatchSavesFieldsPricesAndAllocatesNumbersAcrossTables(t *testing.T) {
	db, engine := batchTestDB(t)
	fields := `"category_name":"电阻","supplier_name":"测试供应商","name":"核对名称","model":"核对型号","manufacturer":"品牌","value":"10K","package":"0603","supplier_part_number":"C2040","description":"描述","location":"A-01","datasheet_url":"https://example.test/data","image_url":"https://example.test/image","total_price_cents":123`
	body := batchCall(t, engine, "/components/batch", `{"items":[{`+fields+`,"stock_quantity":3},{`+fields+`,"stock_quantity":0,"component_number":" CUSTOM "}]}`, 201)
	var components struct {
		Data []models.Component `json:"data"`
	}
	if err := json.Unmarshal(body, &components); err != nil {
		t.Fatal(err)
	}
	if len(components.Data) != 2 || *components.Data[0].ComponentNumber != "HB-000001" || *components.Data[1].ComponentNumber != "CUSTOM" {
		t.Fatalf("response order/numbers: %s", body)
	}
	body = batchCall(t, engine, "/pre-stocks/batch", `{"items":[{`+fields+`,"expected_quantity":3},{`+fields+`,"expected_quantity":0}]}`, 201)
	var preStocks struct {
		Data []models.PreStock `json:"data"`
	}
	if err := json.Unmarshal(body, &preStocks); err != nil {
		t.Fatal(err)
	}
	if len(preStocks.Data) != 2 || *preStocks.Data[0].ComponentNumber != "HB-000002" || *preStocks.Data[1].ComponentNumber != "HB-000003" {
		t.Fatalf("pre-stock numbers: %s", body)
	}
	var component models.Component
	if err := db.Preload("Category").Preload("Supplier").First(&component, components.Data[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if component.Name != "核对名称" || component.Model != "核对型号" || component.Manufacturer != "品牌" || component.Value != "10K" || component.Package != "0603" || component.SupplierPartNumber != "C2040" || component.Description != "描述" || component.Location != "A-01" || component.DatasheetURL != "https://example.test/data" || component.ImageURL != "https://example.test/image" || component.StockQuantity != 3 || component.UnitPriceMicro != 410000 || component.Category.Name != "电阻" || component.Supplier.Name != "测试供应商" {
		t.Fatalf("component fields: %+v", component)
	}
	var item models.PreStock
	if err := db.First(&item, preStocks.Data[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if item.Name != component.Name || item.Model != component.Model || item.Manufacturer != component.Manufacturer || item.Value != component.Value || item.Package != component.Package || item.SupplierPartNumber != component.SupplierPartNumber || item.Description != component.Description || item.Location != component.Location || item.ImageURL != component.ImageURL || item.DatasheetURL != component.DatasheetURL || item.CategoryID != component.CategoryID || *item.SupplierID != *component.SupplierID || item.ExpectedQuantity != 3 || item.TotalPriceCents != 123 || item.Status != "pending" || item.ComponentID != nil {
		t.Fatalf("pre-stock fields: %+v", item)
	}
	var log models.StockLog
	if err := db.First(&log).Error; err != nil {
		t.Fatal(err)
	}
	if log.ComponentID != component.ID || log.ChangeAmount != 3 || log.TotalPriceCents != 123 || log.UnitPriceMicro != 410000 {
		t.Fatalf("initial log: %+v", log)
	}
	assertBatchCounts(t, db, 2, 2, 1, 1, 1)
}

func TestBatchRollsBackAllTablesOnLateFailure(t *testing.T) {
	for _, path := range []string{"/components/batch", "/pre-stocks/batch"} {
		for _, failure := range []string{`"component_number":"DUPLICATE"`, `"category_id":999`, `"supplier_id":999`, `"name":" "`} {
			t.Run(path+"/"+failure, func(t *testing.T) {
				db, engine := batchTestDB(t)
				quantity := "stock_quantity"
				if strings.HasPrefix(path, "/pre-stocks") {
					quantity = "expected_quantity"
				}
				body := fmt.Sprintf(`{"items":[{"name":"first","category_name":"first-category","supplier_name":"first-supplier","component_number":"DUPLICATE",%q:2,"total_price_cents":100},{"category_name":"second-category","supplier_name":"second-supplier",%q:3,%s%s}]}`, quantity, quantity, func() string {
					if strings.HasPrefix(failure, `"name"`) {
						return ""
					}
					return `"name":"second",`
				}(), failure)
				batchCall(t, engine, path, body, 400)
				assertBatchCounts(t, db, 0, 0, 0, 0, 0)
			})
		}
	}
}

func TestBatchRollsBackWhenStockLogWriteFails(t *testing.T) {
	db, engine := batchTestDB(t)
	if err := db.Exec(`CREATE TRIGGER reject_second_log BEFORE INSERT ON stock_logs WHEN NEW.change_amount = 3 BEGIN SELECT RAISE(ABORT, 'simulated log failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	batchCall(t, engine, "/components/batch", `{"items":[{"name":"first","category_name":"cat1","supplier_name":"sup1","stock_quantity":2,"total_price_cents":100},{"name":"second","category_name":"cat2","supplier_name":"sup2","stock_quantity":3,"total_price_cents":200}]}`, 400)
	assertBatchCounts(t, db, 0, 0, 0, 0, 0)
}
