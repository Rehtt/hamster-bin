package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/router"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const token = "integration-test-secret"

func setup(t *testing.T, enabled bool) (*gorm.DB, *httptest.Server) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "mcp.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Category{}, &models.Supplier{}, &models.Component{}, &models.PreStock{}, &models.StockLog{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: "password", JWTSecret: "web-secret", MCPAllowedOrigins: []string{" https://client.example "}}
	if enabled {
		cfg.MCPToken = token
	}
	server := httptest.NewServer(router.Setup(db, parser.NewParserManager(), cfg, nil))
	t.Cleanup(server.Close)
	return db, server
}

type bearerTransport struct{ base http.RoundTripper }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+token)
	return b.base.RoundTrip(r)
}
func connect(t *testing.T, server *httptest.Server, protocol string) *sdk.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{server.Client().Transport}}, DisableStandaloneSSE: true}, &sdk.ClientSessionOptions{ProtocolVersion: protocol})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}
func call(t *testing.T, s *sdk.ClientSession, name string, args map[string]any, wantError bool) json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := s.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if res.IsError != wantError {
		t.Fatalf("%s: isError=%v want %v; content=%+v", name, res.IsError, wantError, res.Content)
	}
	if wantError {
		return nil
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("%s missing text fallback", name)
	}
	var structured, textual any
	if err = json.Unmarshal(raw, &structured); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(res.Content[0].(*sdk.TextContent).Text), &textual); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(structured)
	b, _ := json.Marshal(textual)
	if string(a) != string(b) {
		t.Fatal("text and structured content differ")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
func decode[T any](t *testing.T, b []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Hermes 要求 properties 中的字段 Schema 为对象；additionalProperties: false 等布尔值仍合法。
func checkPropertySchemas(t *testing.T, path string, node any) {
	t.Helper()
	switch node := node.(type) {
	case map[string]any:
		if properties, exists := node["properties"]; exists {
			fields, ok := properties.(map[string]any)
			if !ok {
				t.Fatalf("%s.properties must be an object: %v", path, properties)
			}
			for name, field := range fields {
				if _, ok := field.(map[string]any); !ok {
					t.Fatalf("%s.properties.%s must be an object schema: %v", path, name, field)
				}
			}
		}
		for name, child := range node {
			checkPropertySchemas(t, path+"."+name, child)
		}
	case []any:
		for i, child := range node {
			checkPropertySchemas(t, fmt.Sprintf("%s[%d]", path, i), child)
		}
	}
}

func TestSDKWorkflow(t *testing.T) {
	_, server := setup(t, true)
	s := connect(t, server, "")
	if s.InitializeResult().ServerInfo.Name != "hamster-bin" || s.InitializeResult().ProtocolVersion == "" {
		t.Fatal("missing discovery result")
	}
	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"normalization_standards": true, "normalization_preview": true, "list_components": true, "get_component": true, "create_component": false, "update_component": false, "change_stock": false, "list_pre_stocks": true, "get_pre_stock": true, "create_pre_stock": false, "update_pre_stock": false, "confirm_pre_stock": false, "list_categories": true, "create_category": false, "update_category": false, "delete_category": false, "list_suppliers": true, "create_supplier": false, "list_stock_logs": true, "get_component_stock_logs": true, "get_stats": true}
	if len(tools.Tools) != len(expected) {
		t.Fatalf("tools=%d", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		ro, ok := expected[tool.Name]
		if !ok || tool.Annotations == nil || tool.Annotations.ReadOnlyHint != ro || tool.Description == "" {
			t.Fatalf("bad tool metadata: %+v", tool)
		}
		schema := decode[map[string]any](t, mustJSON(t, tool.InputSchema))
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("bad schema %s: %v", tool.Name, schema)
		}
		checkPropertySchemas(t, tool.Name+".inputSchema", schema)
		output := decode[map[string]any](t, mustJSON(t, tool.OutputSchema))
		if output["type"] != "object" || output["additionalProperties"] != false {
			t.Fatalf("bad output schema %s: %v", tool.Name, output)
		}
		checkPropertySchemas(t, tool.Name+".outputSchema", output)
		properties, ok := output["properties"].(map[string]any)
		if !ok || len(properties) != 2 || properties["data"] == nil || properties["pagination"] == nil {
			t.Fatalf("bad output envelope %s: %v", tool.Name, output)
		}
		dataSchema := properties["data"].(map[string]any)
		if len(dataSchema) != 1 || dataSchema["description"] != "工具返回的数据，结构取决于具体工具" {
			t.Fatalf("data schema must describe unrestricted values %s: %v", tool.Name, dataSchema)
		}
		if tool.Name == "update_category" || tool.Name == "delete_category" {
			if tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Fatalf("category mutation annotations: %+v", tool.Annotations)
			}
			if fmt.Sprint(schema["required"]) != "[id]" {
				t.Fatalf("category mutation required fields: %v", schema["required"])
			}
			properties := schema["properties"].(map[string]any)
			wantFields := 1
			if tool.Name == "update_category" {
				wantFields = 4
			}
			if len(properties) != wantFields || properties["id"].(map[string]any)["type"] != "integer" {
				t.Fatalf("category mutation schema: %v", schema)
			}
			for name, property := range properties {
				if property.(map[string]any)["description"] == "" {
					t.Fatalf("missing parameter description: %s", name)
				}
			}
			if tool.Name == "update_category" {
				if properties["name"] == nil || properties["parent_id"] == nil {
					t.Fatalf("missing category patch fields: %v", schema)
				}
			}
		}
		if tool.Name == "create_component" {
			required := fmt.Sprint(schema["required"])
			for _, name := range []string{"name", "category_id", "stock_quantity"} {
				if !strings.Contains(required, name) {
					t.Fatalf("required: %s", required)
				}
			}
		}
		if tool.Name == "update_component" {
			properties := schema["properties"].(map[string]any)
			if properties["stock_quantity"] != nil || properties["unit_price_micro"] != nil {
				t.Fatal("inventory exposed in patch schema")
			}
		}
	}
	cat := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "电阻"}, false))
	call(t, s, "update_category", map[string]any{"id": cat.ID, "status": "active"}, false)
	supplier := decode[models.Supplier](t, call(t, s, "create_supplier", map[string]any{"name": "嘉立创"}, false))
	same := decode[models.Supplier](t, call(t, s, "create_supplier", map[string]any{"name": "嘉立创"}, false))
	if same.ID != supplier.ID {
		t.Fatal("supplier not reused")
	}
	component := decode[models.Component](t, call(t, s, "create_component", map[string]any{"name": "贴片电阻", "category_id": cat.ID, "supplier_id": supplier.ID, "stock_quantity": 10, "total_price_cents": 100, "model": "R10K", "location": "A1"}, false))
	if component.StockQuantity != 10 || component.UnitPriceMicro != 100000 || component.ComponentNumber == nil {
		t.Fatalf("component: %+v", component)
	}
	updated := decode[models.Component](t, call(t, s, "update_component", map[string]any{"id": component.ID, "location": "B2"}, false))
	if updated.StockQuantity != 10 || updated.UnitPriceMicro != 100000 || updated.Name != component.Name || updated.Model != "R10K" || updated.SupplierID == nil || *updated.SupplierID != supplier.ID {
		t.Fatalf("patch lost fields: %+v", updated)
	}
	call(t, s, "update_component", map[string]any{"id": component.ID, "component_number": *component.ComponentNumber}, false)
	updated = decode[models.Component](t, call(t, s, "update_component", map[string]any{"id": component.ID, "location": "", "supplier_id": 0}, false))
	if updated.Location != "" || updated.SupplierID != nil {
		t.Fatal("clear fields failed")
	}
	updated = decode[models.Component](t, call(t, s, "change_stock", map[string]any{"id": component.ID, "amount": 10, "total_price_cents": 300}, false))
	if updated.StockQuantity != 20 || updated.UnitPriceMicro != 200000 {
		t.Fatalf("weighted price: %+v", updated)
	}
	updated = decode[models.Component](t, call(t, s, "change_stock", map[string]any{"id": component.ID, "amount": -5}, false))
	if updated.StockQuantity != 15 {
		t.Fatal("stock out failed")
	}
	logs := decode[[]models.StockLog](t, call(t, s, "get_component_stock_logs", map[string]any{"id": component.ID}, false))
	if len(logs) != 3 || logs[0].TotalPriceCents != 100 || logs[0].UnitPriceMicro != 200000 || logs[0].ChangeAmount != -5 {
		t.Fatalf("logs: %+v", logs)
	}
	found := decode[[]models.Component](t, call(t, s, "list_components", map[string]any{"name": "贴片 电阻", "model": "R10K", "category_id": cat.ID, "sort_by": "name", "sort_order": "asc"}, false))
	if len(found) != 1 {
		t.Fatal("search failed")
	}
	pre := decode[models.PreStock](t, call(t, s, "create_pre_stock", map[string]any{"name": "预购电阻", "category_id": cat.ID, "supplier_id": supplier.ID, "expected_quantity": 4, "total_price_cents": 100, "location": "P1"}, false))
	pre = decode[models.PreStock](t, call(t, s, "update_pre_stock", map[string]any{"id": pre.ID, "description": "备注"}, false))
	if pre.ExpectedQuantity != 4 || pre.TotalPriceCents != 100 || pre.Location != "P1" || pre.SupplierID == nil {
		t.Fatalf("pre patch lost fields: %+v", pre)
	}
	call(t, s, "create_component", map[string]any{"name": "重复", "category_id": cat.ID, "stock_quantity": 0, "component_number": *pre.ComponentNumber}, true)
	call(t, s, "update_pre_stock", map[string]any{"id": pre.ID, "component_number": *component.ComponentNumber}, true)
	confirmed := decode[models.PreStock](t, call(t, s, "confirm_pre_stock", map[string]any{"id": pre.ID}, false))
	if confirmed.Status != "confirmed" || confirmed.Component == nil || confirmed.Component.StockQuantity != 4 || confirmed.Component.UnitPriceMicro != 250000 {
		t.Fatalf("confirm: %+v", confirmed)
	}
	call(t, s, "confirm_pre_stock", map[string]any{"id": pre.ID}, true)
	call(t, s, "update_pre_stock", map[string]any{"id": pre.ID, "name": "修改"}, true)
	// 已确认预入库保留同一编号，正式元件仍可按原编号编辑。
	call(t, s, "update_component", map[string]any{"id": *confirmed.ComponentID, "component_number": *confirmed.ComponentNumber, "location": "C3"}, false)
	pending := decode[[]models.PreStock](t, call(t, s, "list_pre_stocks", map[string]any{}, false))
	if len(pending) != 0 {
		t.Fatal("pending default failed")
	}
	for _, name := range []string{"get_component", "get_pre_stock"} {
		id := component.ID
		if name == "get_pre_stock" {
			id = pre.ID
		}
		call(t, s, name, map[string]any{"id": id}, false)
	}
	for _, name := range []string{"list_categories", "list_suppliers", "list_stock_logs", "get_stats"} {
		call(t, s, name, map[string]any{}, false)
	}
	stats := decode[map[string]any](t, call(t, s, "get_stats", map[string]any{"range": "all"}, false))
	if stats["total_stock"] != float64(19) || stats["inbound_cost_cents"] != float64(500) {
		t.Fatalf("stats: %v", stats)
	}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestInvalidInputsAndRollback(t *testing.T) {
	db, server := setup(t, true)
	s := connect(t, server, "")
	cat := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "电容"}, false))
	call(t, s, "update_category", map[string]any{"id": cat.ID, "status": "active"}, false)
	c := decode[models.Component](t, call(t, s, "create_component", map[string]any{"name": "电容", "category_id": cat.ID, "stock_quantity": 10, "total_price_cents": 100}, false))
	cases := []struct {
		name string
		args map[string]any
	}{
		{"create_component", map[string]any{"name": "缺数量", "category_id": cat.ID}},
		{"create_component", map[string]any{"name": "负数量", "category_id": cat.ID, "stock_quantity": -1}},
		{"create_component", map[string]any{"name": "  ", "category_id": cat.ID, "stock_quantity": 0}},
		{"create_component", map[string]any{"name": "重复", "category_id": cat.ID, "stock_quantity": 0, "component_number": *c.ComponentNumber}},
		{"create_component", map[string]any{"name": "无分类", "category_id": 9999, "stock_quantity": 0}},
		{"create_component", map[string]any{"name": "金额溢出", "category_id": cat.ID, "stock_quantity": 1, "total_price_cents": int64(math.MaxInt64)}},
		{"create_pre_stock", map[string]any{"name": "无供应商", "category_id": cat.ID, "expected_quantity": 0, "supplier_id": 9999}},
		{"create_pre_stock", map[string]any{"name": "负价格", "category_id": cat.ID, "expected_quantity": 1, "total_price_cents": -1}},
		{"create_category", map[string]any{"name": "子分类", "parent_id": 9999}},
		{"create_supplier", map[string]any{"name": " "}},
		{"change_stock", map[string]any{"id": c.ID, "amount": 0}},
		{"change_stock", map[string]any{"id": c.ID, "amount": math.MaxInt}},
		{"change_stock", map[string]any{"id": c.ID, "amount": math.MinInt}},
		{"change_stock", map[string]any{"id": c.ID, "amount": 1, "total_price_cents": int64(math.MaxInt64)}},
		{"change_stock", map[string]any{"id": c.ID, "amount": -11}},
		{"change_stock", map[string]any{"id": c.ID, "amount": -1, "total_price_cents": 50}},
		{"update_component", map[string]any{"id": c.ID, "stock_quantity": 0}},
		{"update_component", map[string]any{"id": c.ID, "unit_price_micro": 1}},
		{"update_component", map[string]any{"id": c.ID, "name": "修改", "category_id": 9999}},
		{"list_components", map[string]any{"page": -1}},
		{"list_components", map[string]any{"sort_by": "name; DROP TABLE components"}},
		{"list_components", map[string]any{"sort_order": "bad"}},
		{"list_pre_stocks", map[string]any{"status": "bad"}},
		{"get_stats", map[string]any{"range": "bad"}},
		{"get_component", map[string]any{"id": 0}},
		{"get_component_stock_logs", map[string]any{"id": c.ID, "limit": -1}},
		{"list_components", map[string]any{"category_id": 0}},
		{"list_components", map[string]any{"page": math.MaxInt}},
	}
	for _, name := range []string{"get_component", "get_pre_stock", "update_component", "update_pre_stock", "confirm_pre_stock", "get_component_stock_logs", "change_stock"} {
		args := map[string]any{"id": 9999}
		if name == "change_stock" {
			args["amount"] = 1
		}
		cases = append(cases, struct {
			name string
			args map[string]any
		}{name, args})
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) { call(t, s, tc.name, tc.args, true) })
	}
	var current models.Component
	if err := db.First(&current, c.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Name != c.Name || current.StockQuantity != 10 || current.UnitPriceMicro != 100000 {
		t.Fatalf("failed write changed component: %+v", current)
	}
	// 数据库写流水失败必须回滚元件创建、入出库及预入库确认。
	pre := decode[models.PreStock](t, call(t, s, "create_pre_stock", map[string]any{"name": "待确认", "category_id": cat.ID, "expected_quantity": 2, "total_price_cents": 50}, false))
	call(t, s, "update_pre_stock", map[string]any{"id": pre.ID, "name": "不应保存", "expected_quantity": -1}, true)
	if err := db.Exec("CREATE TRIGGER reject_stock_log BEFORE INSERT ON stock_logs BEGIN SELECT RAISE(ABORT, 'forced log failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	call(t, s, "create_component", map[string]any{"name": "回滚元件", "category_id": cat.ID, "stock_quantity": 2, "total_price_cents": 50}, true)
	call(t, s, "change_stock", map[string]any{"id": c.ID, "amount": 2, "total_price_cents": 50}, true)
	call(t, s, "confirm_pre_stock", map[string]any{"id": pre.ID}, true)
	var count int64
	db.Model(&models.Component{}).Count(&count)
	if count != 1 {
		t.Fatalf("partial component left: %d", count)
	}
	if err := db.First(&current, c.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.StockQuantity != 10 || current.UnitPriceMicro != 100000 {
		t.Fatal("stock transaction did not roll back")
	}
	var unchanged models.PreStock
	db.First(&unchanged, pre.ID)
	if unchanged.Status != "pending" || unchanged.Name != "待确认" || unchanged.ComponentID != nil {
		t.Fatalf("partial pre stock: %+v", unchanged)
	}
	db.Model(&models.StockLog{}).Count(&count)
	if count != 1 {
		t.Fatalf("partial logs: %d", count)
	}
}

func TestHTTPAuthenticationAndOrigins(t *testing.T) {
	_, server := setup(t, true)
	tests := []struct {
		method, path, auth, origin string
		status                     int
	}{
		{"POST", "/mcp", "", "", 401}, {"POST", "/mcp", "Bearer wrong", "", 401}, {"POST", "/mcp", "Basic " + token, "", 401},
		{"GET", "/mcp", "Bearer " + token, "", 405},
		{"OPTIONS", "/mcp", "", "https://evil.example", 403},
		{"POST", "/mcp", "Bearer " + token, "http://localhost:5173", 403},
		{"OPTIONS", "/mcp", "", "null", 403},
		{"OPTIONS", "/mcp", "", "https://client.example/path", 403},
		{"OPTIONS", "/mcp", "", "https://client.example", 204},
		{"OPTIONS", "/mcp", "", server.URL, 204},
		{"GET", "/api/v1/components", "Bearer " + token, "", 401},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, server.URL+tc.path, nil)
			req.Header.Set("Authorization", tc.auth)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				body, _ := io.ReadAll(res.Body)
				t.Fatalf("status=%d want %d: %s", res.StatusCode, tc.status, body)
			}
			if tc.status == 204 && res.Header.Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("missing CORS allow origin")
			}
			if tc.status == 403 && res.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("REST CORS bypassed MCP protection")
			}
		})
	}
	login, err := server.Client().Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"admin","password":"password"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK || len(login.Cookies()) == 0 {
		t.Fatal("Web login failed")
	}
	req, _ := http.NewRequest("GET", server.URL+"/mcp", nil)
	for _, cookie := range login.Cookies() {
		req.AddCookie(cookie)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatal("Web Cookie must not authenticate MCP")
	}

	_, off := setup(t, false)
	for _, method := range []string{"GET", "POST", "OPTIONS", "DELETE"} {
		req, _ := http.NewRequest(method, off.URL+"/mcp", nil)
		res, err := off.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatalf("disabled %s: %d", method, res.StatusCode)
		}
	}
}
func TestLegacyHandshakeAndPagination(t *testing.T) {
	db, server := setup(t, true)
	s := connect(t, server, "2025-06-18")
	if s.InitializeResult().ProtocolVersion != "2025-06-18" {
		t.Fatal("legacy handshake failed")
	}
	cat := models.Category{Name: "分类"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatal(err)
	}
	components := make([]models.Component, 105)
	for i := range components {
		components[i] = models.Component{Name: fmt.Sprintf("R%03d", i), CategoryID: cat.ID}
	}
	if err := db.Create(&components).Error; err != nil {
		t.Fatal(err)
	}
	if n := len(decode[[]models.Component](t, call(t, s, "list_components", map[string]any{}, false))); n != 20 {
		t.Fatalf("default page size: %d", n)
	}
	if n := len(decode[[]models.Component](t, call(t, s, "list_components", map[string]any{"page_size": 1000}, false))); n != 100 {
		t.Fatalf("page cap: %d", n)
	}
	if n := len(decode[[]models.Component](t, call(t, s, "list_components", map[string]any{"page_size": 100, "page": 2}, false))); n != 5 {
		t.Fatalf("second page: %d", n)
	}
	logs := make([]models.StockLog, 105)
	for i := range logs {
		logs[i] = models.StockLog{ComponentID: components[0].ID, ChangeAmount: 1}
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if n := len(decode[[]models.StockLog](t, call(t, s, "get_component_stock_logs", map[string]any{"id": components[0].ID}, false))); n != 50 {
		t.Fatalf("log default: %d", n)
	}
	if n := len(decode[[]models.StockLog](t, call(t, s, "get_component_stock_logs", map[string]any{"id": components[0].ID, "limit": 1000}, false))); n != 100 {
		t.Fatalf("log cap: %d", n)
	}
}

func TestSDKCategoryMutations(t *testing.T) {
	db, server := setup(t, true)
	s := connect(t, server, "")
	root := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "根"}, false))
	child := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "子", "parent_id": root.ID}, false))
	other := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "另一根", "parent_id": 0}, false))
	updated := decode[models.Category](t, call(t, s, "update_category", map[string]any{"id": child.ID, "name": " 新名称 "}, false))
	if updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != root.ID {
		t.Fatalf("rename: %+v", updated)
	}
	updated = decode[models.Category](t, call(t, s, "update_category", map[string]any{"id": child.ID, "parent_id": other.ID}, false))
	if updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != other.ID {
		t.Fatalf("move: %+v", updated)
	}
	for _, args := range []map[string]any{{"id": child.ID}, {"id": child.ID, "name": nil, "parent_id": nil}} {
		updated = decode[models.Category](t, call(t, s, "update_category", args, false))
		if updated.Name != "新名称" || updated.ParentID == nil || *updated.ParentID != other.ID {
			t.Fatalf("omission/null: %+v", updated)
		}
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"update_category", map[string]any{}},
		{"delete_category", map[string]any{}},
		{"update_category", map[string]any{"id": 0}},
		{"delete_category", map[string]any{"id": 0}},
		{"update_category", map[string]any{"id": -1}},
		{"delete_category", map[string]any{"id": -1}},
		{"update_category", map[string]any{"id": 9999}},
		{"delete_category", map[string]any{"id": 9999}},
		{"update_category", map[string]any{"id": child.ID, "name": " "}},
		{"update_category", map[string]any{"id": child.ID, "parent_id": 9999}},
		{"update_category", map[string]any{"id": child.ID, "parent_id": -1}},
		{"update_category", map[string]any{"id": child.ID, "parent_id": child.ID}},
		{"update_category", map[string]any{"id": other.ID, "parent_id": child.ID}},
		{"update_category", map[string]any{"id": child.ID, "stock_quantity": 0}},
		{"delete_category", map[string]any{"id": other.ID}},
	} {
		call(t, s, tc.name, tc.args, true)
	}
	updated = decode[models.Category](t, call(t, s, "update_category", map[string]any{"id": child.ID, "parent_id": 0}, false))
	if updated.Name != "新名称" || updated.ParentID != nil {
		t.Fatalf("clear parent: %+v", updated)
	}
	deleted := decode[struct {
		ID      uint   `json:"id"`
		Message string `json:"message"`
	}](t, call(t, s, "delete_category", map[string]any{"id": child.ID}, false))
	if deleted.ID != child.ID || deleted.Message != "删除成功" {
		t.Fatalf("delete result: %+v", deleted)
	}
	call(t, s, "delete_category", map[string]any{"id": child.ID}, true)
	call(t, s, "update_category", map[string]any{"id": child.ID, "name": "不能重建"}, true)
	// 零库存与已确认预入库分别阻止删除；业务错误通过 isError 返回。
	call(t, s, "update_category", map[string]any{"id": root.ID, "status": "active"}, false)
	call(t, s, "create_component", map[string]any{"name": "零库存", "category_id": root.ID, "stock_quantity": 0}, false)
	call(t, s, "delete_category", map[string]any{"id": root.ID}, true)
	pre := models.PreStock{Name: "已确认", CategoryID: other.ID, Status: "confirmed"}
	if err := db.Create(&pre).Error; err != nil {
		t.Fatal(err)
	}
	call(t, s, "delete_category", map[string]any{"id": other.ID}, true)
	// 历史循环必须终止并报告工具错误。
	if err := db.Model(&models.Category{}).Where("id = ?", root.ID).Update("parent_id", root.ID).Error; err != nil {
		t.Fatal(err)
	}
	call(t, s, "update_category", map[string]any{"id": other.ID, "parent_id": root.ID}, true)
	call(t, s, "create_category", map[string]any{"name": "循环", "parent_id": root.ID}, true)
}

func TestNormalizationToolsAndWriteConfirmation(t *testing.T) {
	_, server := setup(t, true)
	s := connect(t, server, "")
	cat := decode[models.Category](t, call(t, s, "create_category", map[string]any{"name": "电阻"}, false))
	if cat.Status != "pending" {
		t.Fatal(cat)
	}
	call(t, s, "create_component", map[string]any{"name": "电阻", "category_id": cat.ID, "stock_quantity": 0}, true)
	call(t, s, "update_category", map[string]any{"id": cat.ID, "status": "active"}, false)
	call(t, s, "normalization_standards", map[string]any{}, false)
	call(t, s, "normalization_preview", map[string]any{"name": "Resistor", "category_id": cat.ID}, false)
	call(t, s, "create_component", map[string]any{"name": "Unknown", "category_id": cat.ID, "stock_quantity": 0}, true)
	c := decode[models.Component](t, call(t, s, "create_component", map[string]any{"name": "Unknown", "category_id": cat.ID, "stock_quantity": 0, "package": "SOT23", "review_confirmations": map[string]string{"name": "Unknown"}}, false))
	if len(c.Review) != 1 || c.Package != "SOT-23" {
		t.Fatal(c)
	}
	call(t, s, "update_component", map[string]any{"id": c.ID, "name": "Changed", "review_confirmations": map[string]string{"name": "Unknown"}}, true)
	call(t, s, "update_component", map[string]any{"id": c.ID, "description": ""}, false)
	call(t, s, "update_category", map[string]any{"id": cat.ID, "status": "inactive"}, false)
	call(t, s, "update_component", map[string]any{"id": c.ID, "location": "A1"}, false)
	call(t, s, "create_pre_stock", map[string]any{"name": "电阻", "category_id": cat.ID, "expected_quantity": 1}, true)
}
