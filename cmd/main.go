package main

import (
	"fmt"
	"log"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/course"
	"github.com/CodeEnthusiast09/proctura-backend/internal/database"
	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/router"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/tenant"
	"github.com/CodeEnthusiast09/proctura-backend/internal/user"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
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

	// Services
	authSvc := auth.NewService(db, cfg)
	tenantSvc := tenant.NewService(db)
	userSvc := user.NewService(db)
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
		Submission: submission.NewHandler(submissionSvc),
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
