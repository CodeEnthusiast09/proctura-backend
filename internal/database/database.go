package database

import (
	"fmt"
	"log"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg config.DBConfig) (*gorm.DB, error) {
	dsn := cfg.URL
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		log.Printf("[db] pgcrypto extension: %v (non-fatal)", err)
	}

	log.Println("[db] connected")
	return db, nil
}

// DSN builds and returns the full connection string.
// Used by Atlas migrations which need the raw DSN.
func DSN(cfg config.DBConfig) string {
	if cfg.URL != "" {
		return cfg.URL
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name,
	)
}
