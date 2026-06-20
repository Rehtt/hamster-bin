package middleware

import (
	"net/http"

	"github.com/Rehtt/hamster-bin/internal/auth"
	"github.com/Rehtt/hamster-bin/internal/config"
	"github.com/gin-gonic/gin"
)

const usernameContextKey = "username"

// AuthMiddleware 校验 JWT Cookie；鉴权关闭时直接放行
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.IsAuthEnabled() {
			c.Next()
			return
		}

		token, err := c.Cookie(auth.CookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
			return
		}

		claims, err := auth.ParseToken(token, cfg.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
			return
		}

		c.Set(usernameContextKey, claims.Username)
		c.Next()
	}
}
