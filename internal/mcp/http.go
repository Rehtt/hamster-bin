package mcp

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/Rehtt/hamster-bin/internal/config"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
)

// NewHandler 提供独立于 Web Cookie 的无状态 MCP 入口。
func NewHandler(db *gorm.DB, cfg *config.Config) http.Handler {
	if cfg.MCPToken == "" {
		return http.NotFoundHandler()
	}
	server := newServer(db)
	transport := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	tokenHash := sha256.Sum256([]byte(cfg.MCPToken))
	allowed := map[string]bool{}
	for _, origin := range cfg.MCPAllowedOrigins {
		allowed[strings.TrimSpace(origin)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Add("Vary", "Origin")
		origins := r.Header.Values("Origin")
		origin := r.Header.Get("Origin")
		if len(origins) > 1 || (origin != "" && !validOrigin(origin, r, allowed)) {
			http.Error(w, "不允许的 Origin", http.StatusForbidden)
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, MCP-Protocol-Version, MCP-Session-Id, Last-Event-ID")
			w.Header().Set("Access-Control-Expose-Headers", "MCP-Protocol-Version, MCP-Session-Id")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		auth := strings.Fields(r.Header.Get("Authorization"))
		supplied := ""
		if len(auth) == 2 && strings.EqualFold(auth[0], "Bearer") {
			supplied = auth[1]
		}
		suppliedHash := sha256.Sum256([]byte(supplied))
		if len(r.Header.Values("Authorization")) != 1 || supplied == "" || subtle.ConstantTimeCompare(suppliedHash[:], tokenHash[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="hamster-bin-mcp"`)
			http.Error(w, "需要有效的 Bearer Token", http.StatusUnauthorized)
			return
		}
		transport.ServeHTTP(w, r)
	})
}

func validOrigin(origin string, r *http.Request, allowed map[string]bool) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || origin != u.Scheme+"://"+u.Host {
		return false
	}
	if allowed[origin] {
		return true
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// 不信任客户端传入的 X-Forwarded-*；代理后的外部 Origin 可通过白名单配置。
	requestURL := &url.URL{Scheme: scheme, Host: r.Host}
	return u.Scheme == scheme && strings.EqualFold(u.Hostname(), requestURL.Hostname()) && originPort(u) == originPort(requestURL)
}

func originPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
