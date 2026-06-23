package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultJWTExpireHours = 168
const defaultDBPath = "./data/inventory.db"

// Config 应用配置
type Config struct {
	Port           string
	DBDriver       string
	DBDSN          string
	DBPath         string
	ImageDir       string
	LogLevel       string
	SSLCert        string
	SSLKey         string
	LLMBaseURL     string
	LLMAPIKey      string
	LLMModel       string
	AdminUsername  string
	AdminPassword  string
	JWTSecret      string
	JWTExpireHours int
}

// Load 加载配置（支持环境变量）
func Load() *Config {
	expireHours := defaultJWTExpireHours
	if v := os.Getenv("JWT_EXPIRE_HOURS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			expireHours = parsed
		}
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DBDriver:       normalizeDBDriver(getEnv("DB_DRIVER", "sqlite")),
		DBDSN:          getEnv("DB_DSN", ""),
		DBPath:         getEnv("DB_PATH", defaultDBPath),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		ImageDir:       getEnv("IMAGE_DIR", "./data/images"),
		SSLCert:        getEnv("SSL_CERT", ""),
		SSLKey:         getEnv("SSL_KEY", ""),
		LLMBaseURL:     getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:      getEnv("LLM_API_KEY", ""),
		LLMModel:       getEnv("LLM_MODEL", ""),
		AdminUsername:  getEnv("ADMIN_USERNAME", ""),
		AdminPassword:  getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTExpireHours: expireHours,
	}

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	return cfg
}

// IsAuthEnabled 当管理员用户名和密码均配置时启用鉴权
func (c *Config) IsAuthEnabled() bool {
	return c.AdminUsername != "" && c.AdminPassword != ""
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
