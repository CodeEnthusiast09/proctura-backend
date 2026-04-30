package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/cloudinary"
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/course"
	"github.com/CodeEnthusiast09/proctura-backend/internal/database"
	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/router"
	"github.com/CodeEnthusiast09/proctura-backend/internal/storage"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/tenant"
	"github.com/CodeEnthusiast09/proctura-backend/internal/user"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("[env] no .env file found, using system environment")
	}

	cfg := config.Load()

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	dsn := database.DSN(cfg.DB)
	if err := database.RunMigrations("migrations", dsn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	seedSuperAdmin(db, cfg)

	// Mailer — Resend primary, SMTP fallback, no-op if neither configured
	var m mailer.Mailer
	var providers []mailer.Mailer
	if cfg.Email.ResendAPIKey != "" {
		providers = append(providers, mailer.NewResendMailer(cfg.Email.ResendAPIKey, cfg.Email.From))
	}
	if cfg.SMTP.Host != "" && cfg.SMTP.User != "" {
		providers = append(providers, mailer.NewSMTPMailer(
			cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.User, cfg.SMTP.Password, cfg.Email.From,
		))
	}
	switch len(providers) {
	case 0:
		log.Println("[mailer] no email provider configured — using no-op mailer")
		m = &mailer.NoOpMailer{}
	case 1:
		m = providers[0]
	default:
		m = mailer.NewFallbackMailer(providers...)
		log.Printf("[mailer] %d email providers configured (fallback chain active)", len(providers))
	}

	// Storage — Cloudinary primary, MinIO fallback for large files
	cloudinaryClient := cloudinary.NewClient(cfg.Cloudinary)
	minioProvider, err := storage.NewMinIOProvider(cfg.MinIO)
	if err != nil {
		log.Fatalf("minio init: %v", err)
	}
	if minioProvider != nil {
		log.Println("[storage] MinIO configured — large recordings will route to MinIO")
	} else {
		log.Println("[storage] MinIO not configured — all recordings will use Cloudinary")
	}
	storageRouter := storage.NewRouter(
		storage.NewCloudinaryProvider(cloudinaryClient),
		minioProvider,
	)

	// Services
	authSvc := auth.NewService(db, cfg, m)
	tenantSvc := tenant.NewService(db, m, cfg.App.FrontendURL)
	userSvc := user.NewService(db, m, cfg.App.FrontendURL)
	courseSvc := course.NewService(db)
	examSvc := exam.NewService(db)
	judge0Client := submission.NewJudge0Client(cfg.Judge0)
	submissionSvc := submission.NewService(db, judge0Client)

	// Handlers
	h := router.Handlers{
		Auth:       auth.NewHandler(authSvc),
		Tenant:     tenant.NewHandler(tenantSvc),
		User:       user.NewHandler(userSvc),
		Course:     course.NewHandler(courseSvc),
		Exam:       exam.NewHandler(examSvc),
		Submission: submission.NewHandler(submissionSvc, storageRouter),
	}

	r := gin.Default()
	router.Setup(r, h, db, cfg.JWT.Secret)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("proctura-backend running on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// seedSuperAdmin creates the super_admin account on first boot if it doesn't exist.
func seedSuperAdmin(db *gorm.DB, cfg *config.Config) {
	if cfg.App.SuperAdminEmail == "" || cfg.App.SuperAdminPassword == "" {
		log.Println("[seed] SUPER_ADMIN_EMAIL or SUPER_ADMIN_PASSWORD not set — skipping super_admin seed")
		return
	}

	var existing models.User
	if err := db.Where("email = ? AND role = ?", cfg.App.SuperAdminEmail, models.RoleSuperAdmin).
		First(&existing).Error; err == nil {
		return // already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.App.SuperAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[seed] failed to hash super_admin password: %v", err)
		return
	}

	superAdmin := models.User{
		Email:        cfg.App.SuperAdminEmail,
		PasswordHash: string(hash),
		Role:         models.RoleSuperAdmin,
		FirstName:    "Super",
		LastName:     "Admin",
		IsActive:     true,
		IsVerified:   true,
	}

	if err := db.Create(&superAdmin).Error; err != nil {
		log.Printf("[seed] failed to create super_admin: %v", err)
		return
	}

	log.Printf("[seed] super_admin created: %s", cfg.App.SuperAdminEmail)
}
