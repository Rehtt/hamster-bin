package config

import (
	"fmt"
	"os"
	"strconv"
)

const defaultJWTExpireHours = 168

// Config 应用配置
type Config struct {
	Port             string
	DBPath           string
	ImageDir         string
	LogLevel         string
	SSLCert          string
	SSLKey           string
	LLMBaseURL       string
	LLMAPIKey        string
	LLMModel         string
	AdminUsername    string
	AdminPassword    string
	JWTSecret        string
	JWTExpireHours   int
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
		DBPath:         getEnv("DB_PATH", "./data/inventory.db"),
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

// Validate 校验配置合法性
func (c *Config) Validate() error {
	if c.IsAuthEnabled() && c.JWTSecret == "" {
		return fmt.Errorf("启用鉴权时必须设置 JWT_SECRET 环境变量")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
