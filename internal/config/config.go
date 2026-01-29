package config

import "os"

// Config 应用配置
type Config struct {
	Port     string
	DBPath   string
	ImageDir string
	LogLevel string
}

// Load 加载配置（支持环境变量）
func Load() *Config {
	return &Config{
		Port:     getEnv("PORT", "8080"),
		DBPath:   getEnv("DB_PATH", "./data/inventory.db"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		ImageDir: getEnv("IMAGE_DIR", "./data/images"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
