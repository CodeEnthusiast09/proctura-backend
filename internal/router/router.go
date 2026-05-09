package router

import (
	"os"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/course"
	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/middleware"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/tenant"
	"github.com/CodeEnthusiast09/proctura-backend/internal/user"
	"github.com/gin-contrib/cors"
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

	allowedOrigins := []string{os.Getenv("APP_BASE_URL")}
	if gin.Mode() != gin.ReleaseMode {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://127.0.0.1:3000")
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Tenant-Subdomain"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

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
		// Recovery: invite a school admin into any tenant
		superAdmin.POST("/tenants/:id/invite-admin", h.User.InviteAdmin)
	}

	// ── Authenticated user (no tenant required) ───────────────────────────────
	// Profile self-service works for super_admin (no tenant) too.
	meRoutes := api.Group("/me")
	meRoutes.Use(middleware.Authenticate(jwtSecret))
	{
		meRoutes.GET("", h.User.Me)
		meRoutes.PATCH("", h.User.UpdateMe)
		meRoutes.POST("/change-password", h.User.ChangePassword)
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
		adminRoutes.POST("/users/invite-admin", h.User.InviteAdmin)
		adminRoutes.POST("/users/invite-lecturer", h.User.InviteLecturer)
		adminRoutes.POST("/users/invite-student", h.User.InviteStudent)
		adminRoutes.POST("/users/import-students", h.User.ImportStudents)
		adminRoutes.PUT("/users/:id", h.User.Update)
		adminRoutes.DELETE("/users/:id", h.User.Delete)
	}

	// Staff read-only views (both school_admin and lecturer can see results,
	// enrollments, submission detail). school_admin is view-only on academic
	// content — they don't author courses or exams.
	staffRoutes := tenantRoutes.Group("")
	staffRoutes.Use(middleware.RequireRole("school_admin", "lecturer"))
	{
		staffRoutes.GET("/courses/:id/enrollments", h.Course.ListEnrollments)
		staffRoutes.GET("/exams/:id/results", h.Exam.GetResults)
		staffRoutes.GET("/results", h.Submission.GetAllResults)
		staffRoutes.GET("/submissions/:id", h.Submission.GetSubmissionDetail)
	}

	// Lecturer routes — write operations on academic content
	lecturerRoutes := tenantRoutes.Group("")
	lecturerRoutes.Use(middleware.RequireRole("lecturer"))
	{
		// Courses
		lecturerRoutes.POST("/courses", h.Course.Create)
		lecturerRoutes.PUT("/courses/:id", h.Course.Update)
		lecturerRoutes.DELETE("/courses/:id", h.Course.Delete)

		// Enrollments
		lecturerRoutes.POST("/courses/:id/enroll", h.Course.Enroll)
		lecturerRoutes.DELETE("/courses/:id/enrollments/:studentId", h.Course.Unenroll)

		// Exams
		lecturerRoutes.POST("/exams", h.Exam.CreateExam)
		lecturerRoutes.PUT("/exams/:id", h.Exam.UpdateExam)
		lecturerRoutes.PATCH("/exams/:id/status", h.Exam.UpdateExamStatus)
		lecturerRoutes.DELETE("/exams/:id", h.Exam.DeleteExam)
		lecturerRoutes.PATCH("/exams/:id/release-results", h.Exam.ReleaseResults)
		lecturerRoutes.PATCH("/submissions/:id/answers/:answerId/score", h.Submission.OverrideAnswerScore)

		// Questions
		lecturerRoutes.POST("/exams/:id/questions", h.Exam.AddQuestion)
		lecturerRoutes.PUT("/questions/:id", h.Exam.UpdateQuestion)
		lecturerRoutes.DELETE("/questions/:id", h.Exam.DeleteQuestion)

		// Test cases
		lecturerRoutes.POST("/questions/:id/test-cases", h.Exam.AddTestCase)
		lecturerRoutes.PUT("/test-cases/:id", h.Exam.UpdateTestCase)
		lecturerRoutes.DELETE("/test-cases/:id", h.Exam.DeleteTestCase)
	}

	// Shared authenticated routes (all roles within a tenant)
	sharedRoutes := tenantRoutes.Group("")
	{
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
		studentRoutes.GET("/my-submissions", h.Submission.GetMySubmissions)
		studentRoutes.POST("/exams/:id/start", h.Submission.StartExam)
		studentRoutes.GET("/exams/:id/my-submission", h.Submission.GetMySubmission)
		studentRoutes.PUT("/submissions/:id/answer", h.Submission.SaveAnswer)
		studentRoutes.POST("/submissions/:id/run", h.Submission.RunCode)
		studentRoutes.GET("/submissions/:id/upload-token", h.Submission.GetUploadToken)
		studentRoutes.PATCH("/submissions/:id/recording", h.Submission.AttachRecording)
		studentRoutes.POST("/submissions/:id/submit", h.Submission.Submit)
		studentRoutes.GET("/submissions/:id/result", h.Submission.GetResult)
		studentRoutes.POST("/submissions/:id/violation", h.Submission.LogViolation)
	}
}
