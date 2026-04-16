package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port   int
	DB     DBConfig
	JWT    JWTConfig
	Judge0 Judge0Config
	Email  EmailConfig
	App    AppConfig
}

type DBConfig struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

type Judge0Config struct {
	BaseURL string
	APIKey  string
	Host    string
}

type EmailConfig struct {
	From        string
	ResendAPIKey string
}

type AppConfig struct {
	BaseURL            string
	FrontendURL        string
	SuperAdminEmail    string
	SuperAdminPassword string
}

func Load() *Config {
	return &Config{
		Port: getEnvInt("PORT", 8080),
		DB: DBConfig{
			URL:      getEnv("DATABASE_URL", ""),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USERNAME", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_DATABASE", "proctura_db"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-this-secret"),
			Expiration: getEnvDuration("JWT_EXPIRATION", 24*time.Hour),
		},
		Judge0: Judge0Config{
			BaseURL: getEnv("JUDGE0_BASE_URL", "https://judge0-ce.p.rapidapi.com"),
			APIKey:  getEnv("JUDGE0_API_KEY", ""),
			Host:    getEnv("JUDGE0_API_HOST", "judge0-ce.p.rapidapi.com"),
		},
		Email: EmailConfig{
			From:         getEnv("EMAIL_FROM", "Proctura <noreply@proctura.com>"),
			ResendAPIKey: getEnv("RESEND_API_KEY", ""),
		},
		App: AppConfig{
			BaseURL:            getEnv("APP_BASE_URL", "http://localhost:8080"),
			FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
			SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "admin@yopmail.com"),
			SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", "12345678"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
