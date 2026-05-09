package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/cloudinary"
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/course"
	"github.com/CodeEnthusiast09/proctura-backend/internal/database"
	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/queue"
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
	if migrationErr := database.RunMigrations("migrations", dsn); migrationErr != nil {
		log.Fatalf("run migrations: %v", migrationErr)
	}

	seedSuperAdmin(db, cfg)

	// Async queue for email + grading. The API server only enqueues; the
	// cmd/worker binary consumes and does the actual delivery / grading.
	queueClient := queue.NewClient(cfg.Redis)
	defer queueClient.Close()
	log.Printf("[queue] enqueueing to redis %s", cfg.Redis.Addr)

	// Services use a queue-backed mailer — sends never block the request path.
	m := mailer.NewQueueMailer(queueClient)

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
	submissionSvc := submission.NewService(db, judge0Client).
		WithGradingEnqueuer(queueClient)

	// Handlers
	h := router.Handlers{
		Auth:       auth.NewHandler(authSvc),
		Tenant:     tenant.NewHandler(tenantSvc),
		User:       user.NewHandler(userSvc),
		Course:     course.NewHandler(courseSvc),
		Exam:       exam.NewHandler(examSvc),
		Submission: submission.NewHandler(submissionSvc, storageRouter),
	}

	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.DebugMode)
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
