package testutil

import (
	"fmt"
	"os"
	"testing"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB connects to the test database and auto-migrates all models.
// Reads TEST_DATABASE_URL from env; falls back to a default local DSN.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USERNAME", "postgres")
		pass := getEnv("DB_PASSWORD", "postgres")
		name := getEnv("DB_DATABASE", "proctura_test_db")
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
			host, port, user, pass, name)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		// non-fatal — extension may already exist
	}

	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.Course{},
		&models.Exam{},
		&models.Question{},
		&models.TestCase{},
		&models.Submission{},
		&models.SubmissionAnswer{},
	); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	return db
}

// CleanupTables truncates all tables in dependency-safe order.
func CleanupTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	tables := []string{
		"submission_answers",
		"submissions",
		"test_cases",
		"questions",
		"exams",
		"courses",
		"users",
		"tenants",
	}
	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			t.Logf("truncate %s: %v (non-fatal)", table, err)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
