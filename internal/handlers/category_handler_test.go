package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/handlers"
	"github.com/Rehtt/hamster-bin/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func categoryRouter(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	db, engine := batchTestDB(t)
	h := handlers.NewCategoryHandler(db)
	engine.POST("/categories", h.Create)
	engine.PUT("/categories/:id", h.Update)
	engine.DELETE("/categories/:id", h.Delete)
	return db, engine
}

func categoryRequest(t *testing.T, engine *gin.Engine, method, path, body string, status int, message string) []byte {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	if res.Code != status || !strings.Contains(res.Body.String(), message) {
		t.Fatalf("%s %s: %d %s; want %d containing %q", method, path, res.Code, res.Body, status, message)
	}
	return res.Body.Bytes()
}

func TestCategoryDeleteHTTP(t *testing.T) {
	for _, kind := range []string{"empty", "component", "pending", "confirmed", "child", "database_read", "database_check", "database_delete"} {
		t.Run(kind, func(t *testing.T) {
			db, engine := categoryRouter(t)
			c := models.Category{Name: "分类"}
			if err := db.Create(&c).Error; err != nil {
				t.Fatal(err)
			}
			status, message := 409, ""
			switch kind {
			case "empty":
				status, message = 200, `"message":"删除成功"`
			case "component":
				if err := db.Create(&models.Component{Name: "零库存", CategoryID: c.ID}).Error; err != nil {
					t.Fatal(err)
				}
				message = "元件"
			case "pending", "confirmed":
				if err := db.Create(&models.PreStock{Name: "预入库", CategoryID: c.ID, Status: kind}).Error; err != nil {
					t.Fatal(err)
				}
				message = "预入库记录"
			case "child":
				if err := db.Create(&models.Category{Name: "子分类", ParentID: &c.ID}).Error; err != nil {
					t.Fatal(err)
				}
				message = "子分类"
			case "database_read":
				if err := db.Migrator().DropTable(&models.Category{}); err != nil {
					t.Fatal(err)
				}
				status, message = 500, "删除分类失败"
			case "database_check":
				if err := db.Migrator().DropTable(&models.PreStock{}); err != nil {
					t.Fatal(err)
				}
				status, message = 500, "删除分类失败"
			case "database_delete":
				if err := db.Exec("CREATE TRIGGER reject_category BEFORE DELETE ON categories BEGIN SELECT RAISE(ABORT, 'forced failure'); END").Error; err != nil {
					t.Fatal(err)
				}
				status, message = 500, "删除分类失败"
			}
			path := fmt.Sprintf("/categories/%d", c.ID)
			categoryRequest(t, engine, "DELETE", path, "", status, message)
			if kind == "empty" {
				categoryRequest(t, engine, "DELETE", path, "", 404, "分类不存在")
			}
		})
	}
	_, engine := categoryRouter(t)
	for _, id := range []string{"0", "-1", "abc", "4294967296"} {
		categoryRequest(t, engine, "DELETE", "/categories/"+id, "", 400, "无效的ID")
	}
	categoryRequest(t, engine, "DELETE", "/categories/9999", "", 404, "分类不存在")
}

func TestCategoryCreateUpdateHTTP(t *testing.T) {
	db, engine := categoryRouter(t)
	create := func(body string) models.Category {
		t.Helper()
		raw := categoryRequest(t, engine, "POST", "/categories", body, 201, "data")
		var out struct {
			Data models.Category `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out.Data
	}
	root := create(`{"name":"  根分类  "}`)
	if root.Name != "根分类" {
		t.Fatalf("name not trimmed: %+v", root)
	}
	child := create(fmt.Sprintf(`{"name":"子分类","parent_id":%d}`, root.ID))
	path := fmt.Sprintf("/categories/%d", child.ID)
	categoryRequest(t, engine, "PUT", path, `{"name":"  修改后 "}`, 200, "修改后")
	var current models.Category
	if err := db.First(&current, child.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.ParentID == nil || *current.ParentID != root.ID {
		t.Fatal("rename lost parent")
	}
	categoryRequest(t, engine, "PUT", path, `{"parent_id":null,"id":9999}`, 200, "修改后")
	categoryRequest(t, engine, "PUT", path, `{}`, 200, "修改后")
	for _, body := range []string{`{"name":"  "}`, `{"parent_id":9999}`, fmt.Sprintf(`{"parent_id":%d}`, child.ID), `{"parent_id":-1}`, `{"name":123}`} {
		categoryRequest(t, engine, "PUT", path, body, 400, "error")
	}
	categoryRequest(t, engine, "PUT", fmt.Sprintf("/categories/%d", root.ID), fmt.Sprintf(`{"parent_id":%d}`, child.ID), 400, "自身或后代")
	categoryRequest(t, engine, "PUT", path, `{"parent_id":0}`, 200, "修改后")
	current = models.Category{}
	if err := db.First(&current, child.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.ParentID != nil {
		t.Fatal("parent not cleared")
	}
	for _, body := range []string{`{}`, `{"name":" "}`, `{"name":"无效父分类","parent_id":9999}`} {
		categoryRequest(t, engine, "POST", "/categories", body, 400, "error")
	}
	// 历史循环不能被创建或更新接口继续引用。
	if err := db.Model(&root).Update("parent_id", root.ID).Error; err != nil {
		t.Fatal(err)
	}
	categoryRequest(t, engine, "POST", "/categories", fmt.Sprintf(`{"name":"循环","parent_id":%d}`, root.ID), 400, "循环")
	categoryRequest(t, engine, "PUT", path, fmt.Sprintf(`{"parent_id":%d}`, root.ID), 400, "循环")
	categoryRequest(t, engine, "PUT", "/categories/9999", `{"name":"不存在"}`, 404, "分类不存在")
	categoryRequest(t, engine, "PUT", "/categories/0", `{}`, 400, "无效的ID")
	if err := db.Migrator().DropTable(&models.Category{}); err != nil {
		t.Fatal(err)
	}
	categoryRequest(t, engine, "PUT", path, `{"name":"修改"}`, 500, "更新分类失败")
	categoryRequest(t, engine, "POST", "/categories", `{"name":"新分类"}`, 500, "创建分类失败")
}
