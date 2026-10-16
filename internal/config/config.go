package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultJWTExpireHours = 168
const defaultDBPath = "./data/inventory.db"

// Config 应用配置
type Config struct {
	TaskWorkers        int
	TaskTimeoutSeconds int
	Port               string
	DBDriver           string
	DBDSN              string
	DBPath             string
	ImageDir           string
	LogLevel           string
	SSLCert            string
	SSLKey             string
	LLMBaseURL         string
	LLMAPIKey          string
	LLMModel           string
	AdminUsername      string
	AdminPassword      string
	JWTSecret          string
	JWTExpireHours     int
	MCPToken           string
	MCPAllowedOrigins  []string
	OAuthEnabled       bool
	OAuthIssuer        string
	OAuthClientID      string
	OAuthClientSecret  string
	OAuthRedirectURL   string
	OAuthAuthMethod    string
}

// Load 加载配置（支持环境变量）
func Load() *Config {
	oauthEnabled := false
	if raw := os.Getenv("OAUTH_ENABLED"); raw != "" {
		var err error
		oauthEnabled, err = strconv.ParseBool(raw)
		if err != nil {
			panic("OAUTH_ENABLED 必须是布尔值")
		}
	}
	expireHours := defaultJWTExpireHours
	if v := os.Getenv("JWT_EXPIRE_HOURS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			expireHours = parsed
		}
	}

	cfg := &Config{
		TaskWorkers:        positiveEnv("TASK_WORKERS", 2),
		TaskTimeoutSeconds: positiveEnv("TASK_TIMEOUT_SECONDS", 120),
		Port:               getEnv("PORT", "8080"),
		DBDriver:           normalizeDBDriver(getEnv("DB_DRIVER", "sqlite")),
		DBDSN:              getEnv("DB_DSN", ""),
		DBPath:             getEnv("DB_PATH", defaultDBPath),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		ImageDir:           getEnv("IMAGE_DIR", "./data/images"),
		SSLCert:            getEnv("SSL_CERT", ""),
		SSLKey:             getEnv("SSL_KEY", ""),
		LLMBaseURL:         getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:          getEnv("LLM_API_KEY", ""),
		LLMModel:           getEnv("LLM_MODEL", ""),
		AdminUsername:      getEnv("ADMIN_USERNAME", ""),
		AdminPassword:      getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpireHours:     expireHours,
		MCPToken:           getEnv("MCP_TOKEN", ""),
		MCPAllowedOrigins:  commaSeparated(os.Getenv("MCP_ALLOWED_ORIGINS")),
		OAuthEnabled:       oauthEnabled,
		OAuthIssuer:        getEnv("OAUTH_ISSUER", "https://oauth.home.rehtt.com"),
		OAuthClientID:      os.Getenv("OAUTH_CLIENT_ID"),
		OAuthClientSecret:  os.Getenv("OAUTH_CLIENT_SECRET"),
		OAuthRedirectURL:   os.Getenv("OAUTH_REDIRECT_URL"),
		OAuthAuthMethod:    getEnv("OAUTH_AUTH_METHOD", "client_secret_basic"),
	}

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	return cfg
}

// IsAuthEnabled 密码或 OAuth 任一启用时保护业务接口。
func (c *Config) IsAuthEnabled() bool {
	return c.IsPasswordAuthEnabled() || c.OAuthEnabled
}

func (c *Config) IsPasswordAuthEnabled() bool {
	return c.AdminUsername != "" && c.AdminPassword != ""
}

// CookieSecure 使用明确配置的外部协议，不信任任意代理请求头。
func (c *Config) CookieSecure() bool {
	if c.IsHTTPS() {
		return true
	}
	if !c.OAuthEnabled {
		return false
	}
	callback, err := url.Parse(c.OAuthRedirectURL)
	return err == nil && callback.Scheme == "https"
}

// IsHTTPS 是否启用 HTTPS
func (c *Config) IsHTTPS() bool {
	return c.SSLCert != "" && c.SSLKey != ""
}

// DatabaseDisplay 返回可安全打印的数据库目标。
func (c *Config) DatabaseDisplay() string {
	if c.DBDriver == "sqlite" {
		if strings.TrimSpace(c.DBDSN) != "" {
			return c.DBDSN
		}
		return c.DBPath
	}
	return "external"
}

// Validate 校验配置合法性
func (c *Config) Validate() error {
	if c.IsAuthEnabled() && c.JWTSecret == "" {
		return fmt.Errorf("启用鉴权时必须设置 JWT_SECRET 环境变量")
	}
	if c.OAuthEnabled {
		if strings.TrimSpace(c.OAuthClientID) == "" || strings.TrimSpace(c.OAuthClientSecret) == "" {
			return fmt.Errorf("启用 OAuth 时必须设置 OAUTH_CLIENT_ID 和 OAUTH_CLIENT_SECRET")
		}
		if c.OAuthAuthMethod != "client_secret_basic" && c.OAuthAuthMethod != "client_secret_post" {
			return fmt.Errorf("OAUTH_AUTH_METHOD 必须为 client_secret_basic 或 client_secret_post")
		}
		issuer, err := validateOAuthURL(c.OAuthIssuer)
		if err != nil || issuer.RawQuery != "" || strings.HasSuffix(c.OAuthIssuer, "/") {
			return fmt.Errorf("OAUTH_ISSUER 必须为有效的 HTTPS Issuer 且末尾不带斜杠；本地可使用 HTTP 回环地址")
		}
		callback, err := validateOAuthURL(c.OAuthRedirectURL)
		if err != nil || callback.Path != "/api/v1/auth/oauth/callback" || callback.RawQuery != "" {
			return fmt.Errorf("OAUTH_REDIRECT_URL 必须为完整回调地址，路径为 /api/v1/auth/oauth/callback，不带查询参数")
		}
	}
	switch c.DBDriver {
	case "sqlite":
	case "mysql", "postgres":
		if strings.TrimSpace(c.DBDSN) == "" {
			return fmt.Errorf("使用 %s 数据库时必须设置 DB_DSN 环境变量", c.DBDriver)
		}
	default:
		return fmt.Errorf("不支持的 DB_DRIVER: %s", c.DBDriver)
	}
	return nil
}

func validateOAuthURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.ForceQuery {
		return nil, fmt.Errorf("无效 URL")
	}
	if u.Scheme == "https" {
		return u, nil
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
		return u, nil
	}
	return nil, fmt.Errorf("必须使用 HTTPS 或 HTTP 回环地址")
}

func normalizeDBDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite3":
		return "sqlite"
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func positiveEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err == nil && value > 0 {
		return value
	}
	return fallback
}

func commaSeparated(raw string) []string {
	var values []string
	for value := range strings.SplitSeq(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}
