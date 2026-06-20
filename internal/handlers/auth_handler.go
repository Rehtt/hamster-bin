package handlers

import (
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/auth"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 管理员登录
// @route POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.cfg.IsAuthEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "鉴权未启用"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	if !auth.CheckCredentials(req.Username, req.Password, h.cfg.AdminUsername, h.cfg.AdminPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := auth.IssueToken(req.Username, h.cfg.JWTSecret, h.cfg.JWTExpireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败"})
		return
	}

	setAuthCookie(c, h.cfg, token)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"auth_enabled": true,
			"username":     req.Username,
		},
	})
}

// Logout 退出登录
// @route POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	clearAuthCookie(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// Me 获取当前登录状态
// @route GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	if !h.cfg.IsAuthEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"auth_enabled": false,
			},
		})
		return
	}

	token, err := c.Cookie(auth.CookieName)
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
		return
	}

	claims, err := auth.ParseToken(token, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"auth_enabled": true,
			"username":     claims.Username,
		},
	})
}

func setAuthCookie(c *gin.Context, cfg *config.Config, token string) {
	maxAge := cfg.JWTExpireHours * 3600
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.CookieName, token, maxAge, "/", "", cfg.IsHTTPS(), true)
}

func clearAuthCookie(c *gin.Context, cfg *config.Config) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.CookieName, "", -1, "/", "", cfg.IsHTTPS(), true)
}
