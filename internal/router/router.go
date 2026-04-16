package router

import (
	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/course"
	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/middleware"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/tenant"
	"github.com/CodeEnthusiast09/proctura-backend/internal/user"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handlers struct {
	Auth       *auth.Handler
	Tenant     *tenant.Handler
	User       *user.Handler
	Course     *course.Handler
	Exam       *exam.Handler
	Submission *submission.Handler
}

func Setup(r *gin.Engine, h Handlers, db *gorm.DB, jwtSecret string) {
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")

	// ── Public auth routes ─────────────────────────────────────────────────────
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/login", h.Auth.Login)
		authRoutes.POST("/forgot-password", h.Auth.ForgotPassword)
		authRoutes.POST("/reset-password", h.Auth.ResetPassword)
		authRoutes.POST("/accept-invite", h.Auth.AcceptInvite)
		authRoutes.POST("/register", h.Auth.RegisterStudent)
	}

	// ── Super admin routes (no tenant required) ────────────────────────────────
	superAdmin := api.Group("/admin")
	superAdmin.Use(middleware.Authenticate(jwtSecret))
	superAdmin.Use(middleware.RequireRole("super_admin"))
	{
		superAdmin.POST("/tenants", h.Tenant.Create)
		superAdmin.GET("/tenants", h.Tenant.List)
		superAdmin.PUT("/tenants/:id", h.Tenant.Update)
		superAdmin.DELETE("/tenants/:id", h.Tenant.Delete)
	}

	// ── Tenant-scoped routes ───────────────────────────────────────────────────
	tenantRoutes := api.Group("")
	tenantRoutes.Use(middleware.ResolveTenant(db))
	tenantRoutes.Use(middleware.Authenticate(jwtSecret))

	// School admin routes
	adminRoutes := tenantRoutes.Group("")
	adminRoutes.Use(middleware.RequireRole("school_admin"))
	{
		// User management
		adminRoutes.GET("/users", h.User.List)
		adminRoutes.POST("/users/invite-lecturer", h.User.InviteLecturer)
		adminRoutes.POST("/users/import-students", h.User.ImportStudents)
		adminRoutes.PUT("/users/:id", h.User.Update)
		adminRoutes.DELETE("/users/:id", h.User.Delete)
	}

	// Lecturer routes
	lecturerRoutes := tenantRoutes.Group("")
	lecturerRoutes.Use(middleware.RequireRole("school_admin", "lecturer"))
	{
		// Courses
		lecturerRoutes.POST("/courses", h.Course.Create)
		lecturerRoutes.PUT("/courses/:id", h.Course.Update)
		lecturerRoutes.DELETE("/courses/:id", h.Course.Delete)

		// Exams
		lecturerRoutes.POST("/exams", h.Exam.CreateExam)
		lecturerRoutes.PUT("/exams/:id", h.Exam.UpdateExam)
		lecturerRoutes.DELETE("/exams/:id", h.Exam.DeleteExam)
		lecturerRoutes.GET("/exams/:id/results", h.Exam.GetResults)

		// Questions
		lecturerRoutes.POST("/exams/:id/questions", h.Exam.AddQuestion)
		lecturerRoutes.PUT("/questions/:id", h.Exam.UpdateQuestion)
		lecturerRoutes.DELETE("/questions/:id", h.Exam.DeleteQuestion)

		// Test cases
		lecturerRoutes.POST("/questions/:id/test-cases", h.Exam.AddTestCase)
		lecturerRoutes.PUT("/test-cases/:id", h.Exam.UpdateTestCase)
		lecturerRoutes.DELETE("/test-cases/:id", h.Exam.DeleteTestCase)
	}

	// Shared authenticated routes (all roles)
	sharedRoutes := tenantRoutes.Group("")
	{
		sharedRoutes.GET("/me", h.User.Me)
		sharedRoutes.GET("/courses", h.Course.List)
		sharedRoutes.GET("/exams", h.Exam.ListExams)
		sharedRoutes.GET("/exams/:id", h.Exam.GetExam)
	}

	// Student routes
	studentRoutes := tenantRoutes.Group("")
	studentRoutes.Use(middleware.RequireRole("student"))
	{
		studentRoutes.GET("/exams/available", h.Exam.GetAvailableExams)

		// Submissions
		studentRoutes.POST("/exams/:id/start", h.Submission.StartExam)
		studentRoutes.PUT("/submissions/:id/answer", h.Submission.SaveAnswer)
		studentRoutes.POST("/submissions/:id/submit", h.Submission.Submit)
		studentRoutes.GET("/submissions/:id/result", h.Submission.GetResult)
		studentRoutes.POST("/submissions/:id/violation", h.Submission.LogViolation)
	}
}
