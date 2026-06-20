package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/auth"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testAuthConfig(enabled bool) *config.Config {
	cfg := &config.Config{
		AdminUsername:  "admin",
		AdminPassword:  "secret",
		JWTSecret:      "jwt-secret",
		JWTExpireHours: 24,
	}
	if !enabled {
		cfg.AdminUsername = ""
		cfg.AdminPassword = ""
	}
	return cfg
}

func TestAuthHandlerMeAuthDisabled(t *testing.T) {
	handler := NewAuthHandler(testAuthConfig(false))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)

	handler.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["data"]["auth_enabled"] != false {
		t.Fatalf("auth_enabled = %v, want false", resp["data"]["auth_enabled"])
	}
}

func TestAuthHandlerLoginSuccess(t *testing.T) {
	handler := NewAuthHandler(testAuthConfig(true))
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "secret",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Login(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	var tokenCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == auth.CookieName {
			tokenCookie = cookie
			break
		}
	}
	if tokenCookie == nil || tokenCookie.Value == "" {
		t.Fatal("expected auth cookie to be set")
	}
	if !tokenCookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
}

func TestAuthHandlerLoginInvalidCredentials(t *testing.T) {
	handler := NewAuthHandler(testAuthConfig(true))
	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrong",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Login(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandlerMeAuthenticated(t *testing.T) {
	cfg := testAuthConfig(true)
	handler := NewAuthHandler(cfg)

	token, err := auth.IssueToken("admin", cfg.JWTSecret, cfg.JWTExpireHours)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	c.Request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})

	handler.Me(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["data"]["auth_enabled"] != true {
		t.Fatalf("auth_enabled = %v, want true", resp["data"]["auth_enabled"])
	}
	if resp["data"]["username"] != "admin" {
		t.Fatalf("username = %v, want admin", resp["data"]["username"])
	}
}

func TestAuthHandlerLogoutClearsCookie(t *testing.T) {
	handler := NewAuthHandler(testAuthConfig(true))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

	handler.Logout(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	cookies := w.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == auth.CookieName && cookie.MaxAge != -1 {
			t.Fatalf("expected cookie MaxAge = -1, got %d", cookie.MaxAge)
		}
	}
}
