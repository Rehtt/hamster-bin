package middleware

import (
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

func testAuthConfig() *config.Config {
	return &config.Config{
		AdminUsername:  "admin",
		AdminPassword:  "secret",
		JWTSecret:      "jwt-secret",
		JWTExpireHours: 24,
	}
}

func TestAuthMiddlewareDisabled(t *testing.T) {
	cfg := testAuthConfig()
	cfg.AdminUsername = ""
	cfg.AdminPassword = ""

	r := gin.New()
	r.GET("/protected", AuthMiddleware(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareMissingCookie(t *testing.T) {
	cfg := testAuthConfig()
	r := gin.New()
	r.GET("/protected", AuthMiddleware(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareValidCookie(t *testing.T) {
	cfg := testAuthConfig()
	token, err := auth.IssueToken("admin", cfg.JWTSecret, cfg.JWTExpireHours)
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	r := gin.New()
	r.GET("/protected", AuthMiddleware(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareInvalidCookie(t *testing.T) {
	cfg := testAuthConfig()
	r := gin.New()
	r.GET("/protected", AuthMiddleware(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "invalid-token"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
