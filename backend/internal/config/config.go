package config

import (
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	StorageRoot string
}

func Load() Config {
	return Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", "dev-insecure-secret"),
		StorageRoot: getEnv("ATTACHMENT_STORAGE_ROOT", "/data/attachments"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
