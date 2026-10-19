package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
	"github.com/Rehtt/hamster-bin/internal/router"
	"github.com/Rehtt/hamster-bin/internal/tasks"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTaskAPIAndLegacyCompatibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "api.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.Task{}, &models.Category{}, &models.Component{}, &models.PreStock{}); err != nil {
		t.Fatal(err)
	}
	manager := parser.NewParserManager()
	manager.Register(parser.NewTaobaoParser())
	manager.Register(parser.NewLCSCParser())
	service := tasks.New(repository.NewTaskRepository(db), manager, 2, time.Second)
	engine := router.Setup(db, manager, &config.Config{}, service)
	call := func(method, path, body string, want int) map[string]json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	body := `{"type":"component_parse","input":{"code":"123456789012","use_llm":false},"source":"components"}`
	response := call("POST", "/api/v1/tasks", body, 202)
	var task models.Task
	if err := json.Unmarshal(response["data"], &task); err != nil {
		t.Fatal(err)
	}
	if task.Status != "queued" || task.ID == 0 || task.StartedAt != nil {
		t.Fatalf("must return durable queued record immediately: %+v", task)
	}
	call("GET", "/api/v1/tasks/1", "", 200)
	call("GET", "/api/v1/tasks?status=queued&type=component_parse&page_size=1", "", 200)
	call("GET", "/api/v1/tasks?status=nope", "", 400)
	call("GET", "/api/v1/tasks?page_size=101", "", 400)
	call("GET", "/api/v1/tasks?page=bad", "", 400)
	call("GET", "/api/v1/tasks/bad", "", 400)
	call("GET", "/api/v1/tasks/999", "", 404)
	call("POST", "/api/v1/tasks", `{}`, 400)
	call("POST", "/api/v1/tasks", `{"type":"component_parse","input":{"code":"C123","use_llm":true}}`, 400)
	call("POST", "/api/v1/tasks/1/retry", "", 409)
	call("POST", "/api/v1/tasks/999/cancel", "", 404)
	call("POST", "/api/v1/tasks/1/cancel", "", 200)
	call("POST", "/api/v1/tasks/1/cancel", "", 409)
	call("POST", "/api/v1/tasks/1/retry", "", 202)

	call("DELETE", "/api/v1/tasks/bad", "", 400)
	call("DELETE", "/api/v1/tasks/999", "", 404)
	call("DELETE", "/api/v1/tasks/2", "", 409)
	deleted := call("DELETE", "/api/v1/tasks/1", "", 200)
	var cleanup tasks.DeleteResult
	if err := json.Unmarshal(deleted["data"], &cleanup); err != nil {
		t.Fatal(err)
	}
	if cleanup.DeletedCount != 1 || len(cleanup.DeletedIDs) != 1 || cleanup.DeletedIDs[0] != 1 {
		t.Fatalf("delete response: %s", deleted["data"])
	}
	call("GET", "/api/v1/tasks/1", "", 404)
	call("GET", "/api/v1/tasks/2", "", 200)
	for _, invalid := range []string{
		`{}`, `{"mode":"unknown"}`, `{"mode":"selected"}`, `{"mode":"selected","ids":[]}`,
		`{"mode":"selected","ids":null}`, `{"mode":"selected","ids":[0]}`, `{"mode":"selected","ids":[-1]}`,
		`{"mode":"selected","ids":[1.5]}`, `{"mode":"selected","ids":["1"]}`, `{"mode":"selected","ids":[null]}`,
		`{"mode":"selected","ids":[4294967296]}`, `{"mode":"all","ids":[1]}`, `{"mode":"failed","ids":[]}`,
		`{"mode":"older_than_7_days","ids":null}`,
	} {
		call("POST", "/api/v1/tasks/batch-delete", invalid, 400)
	}
	tooMany, _ := json.Marshal(tasks.DeleteRequest{Mode: "selected", IDs: make([]uint, 101)})
	call("POST", "/api/v1/tasks/batch-delete", string(tooMany), 400)
	for _, mode := range []string{"selected", "failed", "all", "older_than_7_days"} {
		body := `{"mode":"` + mode + `"}`
		if mode == "selected" {
			body = `{"mode":"selected","ids":[2,2,999,999]}`
		}
		res := call("POST", "/api/v1/tasks/batch-delete", body, 200)
		if err := json.Unmarshal(res["data"], &cleanup); err != nil {
			t.Fatal(err)
		}
		wantSkipped := 0
		if mode == "selected" {
			wantSkipped = 2
		}
		if mode == "all" {
			wantSkipped = 1
		}
		if cleanup.DeletedCount != 0 || cleanup.SkippedCount != wantSkipped || cleanup.DeletedIDs == nil {
			t.Fatalf("%s: %s", mode, res["data"])
		}
	}
	// 旧客户端继续同步返回原有 data 结构。
	old := call("POST", "/api/v1/components/parse", `{"code":"123456789012"}`, 200)
	var info parser.ComponentInfo
	_ = json.Unmarshal(old["data"], &info)
	if info.PlatformCode != "123456789012" {
		t.Fatalf("legacy component: %s", old["data"])
	}
	qr := call("POST", "/api/v1/components/parse-qrcode", `{"qrcode_data":"{pc:123456789012,qty:10}"}`, 200)
	var result tasks.Result
	_ = json.Unmarshal(qr["data"], &result)
	if result.Quantity != 10 || result.Component == nil {
		t.Fatalf("legacy QR: %s", qr["data"])
	}
	// 刷新任务中心/结果恢复链接直接返回 SPA，保留路径与查询参数。
	for _, path := range []string{"/tasks?task=1", "/components?applyTask=1", "/pre-stocks?applyTask=1"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("<html")) {
			t.Fatalf("SPA fallback %s: %d", path, w.Code)
		}
	}
	// 相同路由在管理员模式下受现有 Cookie 鉴权保护。
	protected := router.Setup(db, manager, &config.Config{AdminUsername: "admin", AdminPassword: "password", JWTSecret: "test-secret", JWTExpireHours: 1}, service)
	for _, path := range []string{"/api/v1/tasks", "/api/v1/tasks/1"} {
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/tasks", "/api/v1/tasks/1/cancel", "/api/v1/tasks/1/retry", "/api/v1/tasks/batch-delete"} {
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewBufferString(body)))
		if w.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/v1/tasks/1", nil))
	if w.Code != 401 {
		t.Fatalf("unprotected delete: %d", w.Code)
	}

}
