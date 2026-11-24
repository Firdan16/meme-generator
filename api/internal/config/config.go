// Package config memuat konfigurasi aplikasi dari environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           int
	AllowedOrigins string
	LogLevel       string
	DB             DBConfig
	Redis          RedisConfig
	Storage        StorageConfig
	JWT            JWTConfig
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr string
}

type StorageConfig struct {
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type JWTConfig struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func Load() (Config, error) {
	port, err := strconv.Atoi(getenv("PORT", "8080"))
	if err != nil {
		return Config{}, fmt.Errorf("config: PORT tidak valid: %w", err)
	}

	accessTTL, err := time.ParseDuration(getenv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("config: JWT_ACCESS_TTL tidak valid: %w", err)
	}
	refreshTTL, err := time.ParseDuration(getenv("JWT_REFRESH_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("config: JWT_REFRESH_TTL tidak valid: %w", err)
	}

	dbPort, err := strconv.Atoi(getenv("POSTGRES_PORT", "5432"))
	if err != nil {
		return Config{}, fmt.Errorf("config: POSTGRES_PORT tidak valid: %w", err)
	}

	return Config{
		Port:           port,
		AllowedOrigins: getenv("ALLOWED_ORIGINS", "http://localhost:3000"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
		DB: DBConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     dbPort,
			User:     getenv("POSTGRES_USER", "meme"),
			Password: getenv("POSTGRES_PASSWORD", "meme"),
			Name:     getenv("POSTGRES_DB", "meme_portal"),
			SSLMode:  getenv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr: getenv("REDIS_ADDR", "localhost:6379"),
		},
		Storage: StorageConfig{
			Region:    getenv("S3_REGION", "garage"),
			Endpoint:  getenv("S3_ENDPOINT", "localhost:3900"),
			AccessKey: getenv("S3_ACCESS_KEY", ""),
			SecretKey: getenv("S3_SECRET_KEY", ""),
			Bucket:    getenv("S3_BUCKET", "meme-portal"),
			UseSSL:    getenv("S3_USE_SSL", "false") == "true",
		},
		JWT: JWTConfig{
			Secret:     getenv("JWT_SECRET", "dev-only-secret-change-me"),
			Issuer:     getenv("JWT_ISSUER", "meme-portal"),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
