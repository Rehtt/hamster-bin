package handlers

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Rehtt/hamster-bin/internal/auth"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	cfg     *config.Config
	service *auth.Service
}

func NewAuthHandler(cfg *config.Config, service *auth.Service) *AuthHandler {
	return &AuthHandler{cfg: cfg, service: service}
}

// Config 公开登录方式，不暴露客户端配置或凭据。
func (h *AuthHandler) Config(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"auth_enabled": h.cfg.IsAuthEnabled(), "password_enabled": h.cfg.IsPasswordAuthEnabled(), "oauth_enabled": h.cfg.OAuthEnabled}})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 管理员登录
// @route POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.cfg.IsPasswordAuthEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码登录未启用"})
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

	previous, _ := c.Cookie(auth.CookieName)
	if err := h.service.Logout(c.Request.Context(), previous); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法结束原登录会话，请重试"})
		return
	}
	setAuthCookie(c, h.cfg, token)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"auth_enabled": true,
			"username":     req.Username,
			"auth_method":  "password",
		},
	})
}

// Logout 退出登录
// @route POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(auth.CookieName)
	clearAuthCookie(c, h.cfg)
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退出失败，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// Me 获取当前登录状态
// @route GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
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

	claims, err := h.service.Authenticate(c.Request.Context(), token)
	if err != nil {
		clearAuthCookie(c, h.cfg)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"auth_enabled": true,
			"username":     claims.Username,
			"auth_method":  claims.AuthMethod,
		},
	})
}

func setAuthCookie(c *gin.Context, cfg *config.Config, token string) {
	maxAge := cfg.JWTExpireHours * 3600
	http.SetCookie(c.Writer, auth.Cookie(cfg, token, maxAge))
}

func clearAuthCookie(c *gin.Context, cfg *config.Config) {
	http.SetCookie(c.Writer, auth.Cookie(cfg, "", -1))
}

func (h *AuthHandler) flowCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: auth.FlowCookieName, Value: value, Path: "/api/v1/auth/oauth",
		MaxAge: maxAge, HttpOnly: true, Secure: h.cfg.CookieSecure(), SameSite: http.SameSiteLaxMode})
}

func (h *AuthHandler) OAuthLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	if !h.cfg.OAuthEnabled {
		c.Status(http.StatusNotFound)
		return
	}
	previous, _ := c.Cookie(auth.FlowCookieName)
	location, browser, err := h.service.BeginLogin(c.Request.Context(), previous)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?oauth_error=unavailable")
		return
	}
	h.flowCookie(c, browser, int(auth.FlowLifetime/time.Second))
	c.Redirect(http.StatusFound, location)
}

func (h *AuthHandler) OAuthBackchannelLogout(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if !h.cfg.OAuthEnabled {
		c.Status(http.StatusNotFound)
		return
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	query, queryErr := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || mediaType != "application/x-www-form-urlencoded" || queryErr != nil || query.Has("logout_token") {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.Request.ParseForm() != nil || len(c.Request.PostForm["logout_token"]) != 1 || strings.TrimSpace(c.Request.PostForm.Get("logout_token")) == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.service.BackchannelLogout(c.Request.Context(), c.Request.PostForm.Get("logout_token")); err != nil {
		if errors.Is(err, auth.ErrOAuthUnavailable) {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusOK)
}

func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	h.flowCookie(c, "", -1)
	if !h.cfg.OAuthEnabled {
		c.Status(http.StatusNotFound)
		return
	}
	browser, _ := c.Cookie(auth.FlowCookieName)
	token, expires, err := h.service.CompleteLogin(c.Request.Context(), browser, c.Request.URL.RawQuery)
	if err != nil {
		code := "invalid"
		if errors.Is(err, auth.ErrOAuthDenied) {
			code = "access_denied"
		}
		if errors.Is(err, auth.ErrOAuthUnavailable) {
			code = "unavailable"
		}
		c.Redirect(http.StatusFound, "/login?oauth_error="+code)
		return
	}
	previous, _ := c.Cookie(auth.CookieName)
	if err := h.service.Logout(c.Request.Context(), previous); err != nil {
		_ = h.service.Logout(c.Request.Context(), token)
		c.Redirect(http.StatusFound, "/login?oauth_error=unavailable")
		return
	}
	http.SetCookie(c.Writer, auth.Cookie(h.cfg, token, max(1, int(time.Until(expires)/time.Second))))
	c.Redirect(http.StatusFound, "/")
}
