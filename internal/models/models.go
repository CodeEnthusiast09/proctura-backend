package models

import (
	"time"
)

// ── Tenant ────────────────────────────────────────────────────────────────────

type Tenant struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Subdomain string    `gorm:"uniqueIndex;not null" json:"subdomain"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Tenant) TableName() string { return "tenants" }

// ── User ──────────────────────────────────────────────────────────────────────

type UserRole string

const (
	RoleSuperAdmin  UserRole = "super_admin"
	RoleSchoolAdmin UserRole = "school_admin"
	RoleLecturer    UserRole = "lecturer"
	RoleStudent     UserRole = "student"
)

type User struct {
	ID               string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID         *string    `gorm:"type:uuid;index" json:"tenant_id,omitempty"` // null for super_admin
	Email            string     `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash     string     `gorm:"not null" json:"-"`
	Role             UserRole   `gorm:"type:varchar(20);not null" json:"role"`
	FirstName        string     `gorm:"not null" json:"first_name"`
	LastName         string     `gorm:"not null" json:"last_name"`
	MatricNumber     *string    `gorm:"index" json:"matric_number,omitempty"` // students only
	IsActive         bool       `gorm:"default:true" json:"is_active"`
	IsVerified       bool       `gorm:"default:false" json:"is_verified"`
	InviteToken      *string    `gorm:"index" json:"-"`
	ResetToken       *string    `gorm:"index" json:"-"`
	ResetTokenExpiry *time.Time `json:"-"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

func (User) TableName() string { return "users" }

// ── Course ────────────────────────────────────────────────────────────────────

type Course struct {
	ID         string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID   string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	LecturerID string    `gorm:"type:uuid;not null" json:"lecturer_id"`
	Title      string    `gorm:"not null" json:"title"`
	Code       string    `gorm:"not null" json:"code"` // e.g. CSC301
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Tenant   *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Lecturer *User   `gorm:"foreignKey:LecturerID" json:"lecturer,omitempty"`
	Exams    []Exam  `gorm:"foreignKey:CourseID" json:"exams,omitempty"`
}

func (Course) TableName() string { return "courses" }

// ── Exam ──────────────────────────────────────────────────────────────────────

type ExamStatus string

const (
	ExamStatusDraft     ExamStatus = "draft"
	ExamStatusScheduled ExamStatus = "scheduled"
	ExamStatusActive    ExamStatus = "active"
	ExamStatusClosed    ExamStatus = "closed"
)

type Exam struct {
	ID              string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID        string     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	CourseID        string     `gorm:"type:uuid;not null" json:"course_id"`
	Title           string     `gorm:"not null" json:"title"`
	Instructions    *string    `gorm:"type:text" json:"instructions,omitempty"`
	DurationMinutes int        `gorm:"not null" json:"duration_minutes"`
	StartsAt        time.Time  `gorm:"not null" json:"starts_at"`
	EndsAt          time.Time  `gorm:"not null" json:"ends_at"`
	LanguageID      int        `gorm:"not null" json:"language_id"` // Judge0 language ID
	LanguageName    string     `gorm:"not null" json:"language_name"`
	Status          ExamStatus `gorm:"type:varchar(20);default:'draft'" json:"status"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	Tenant    *Tenant    `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Course    *Course    `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Questions []Question `gorm:"foreignKey:ExamID" json:"questions,omitempty"`
}

func (Exam) TableName() string { return "exams" }

// ── Question ──────────────────────────────────────────────────────────────────

type Question struct {
	ID         string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ExamID     string    `gorm:"type:uuid;not null;index" json:"exam_id"`
	Body       string    `gorm:"type:text;not null" json:"body"`
	OrderIndex int       `gorm:"not null;default:0" json:"order_index"`
	Points     int       `gorm:"not null;default:10" json:"points"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Exam      *Exam      `gorm:"foreignKey:ExamID" json:"exam,omitempty"`
	TestCases []TestCase `gorm:"foreignKey:QuestionID" json:"test_cases,omitempty"`
}

func (Question) TableName() string { return "questions" }

// ── TestCase ──────────────────────────────────────────────────────────────────

type TestCase struct {
	ID             string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	QuestionID     string    `gorm:"type:uuid;not null;index" json:"question_id"`
	Input          *string   `gorm:"type:text" json:"input,omitempty"`
	ExpectedOutput string    `gorm:"type:text;not null" json:"expected_output"`
	IsHidden       bool      `gorm:"default:false" json:"is_hidden"` // hidden = not shown to student
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Question *Question `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
}

func (TestCase) TableName() string { return "test_cases" }

// ── Submission ────────────────────────────────────────────────────────────────

type SubmissionStatus string

const (
	SubmissionStatusInProgress SubmissionStatus = "in_progress"
	SubmissionStatusSubmitted  SubmissionStatus = "submitted"
	SubmissionStatusGraded     SubmissionStatus = "graded"
)

type Submission struct {
	ID             string           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string           `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ExamID         string           `gorm:"type:uuid;not null;index" json:"exam_id"`
	StudentID      string           `gorm:"type:uuid;not null;index" json:"student_id"`
	Status         SubmissionStatus `gorm:"type:varchar(20);default:'in_progress'" json:"status"`
	StartedAt      time.Time        `gorm:"not null" json:"started_at"`
	SubmittedAt    *time.Time       `json:"submitted_at,omitempty"`
	TotalScore     int              `gorm:"default:0" json:"total_score"`
	MaxScore       int              `gorm:"default:0" json:"max_score"`
	ViolationCount int              `gorm:"default:0" json:"violation_count"`
	RecordingURL   *string          `gorm:"type:text" json:"recording_url,omitempty"`
	CreatedAt      time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time        `gorm:"autoUpdateTime" json:"updated_at"`

	Exam    *Exam              `gorm:"foreignKey:ExamID" json:"exam,omitempty"`
	Student *User              `gorm:"foreignKey:StudentID" json:"student,omitempty"`
	Answers []SubmissionAnswer `gorm:"foreignKey:SubmissionID" json:"answers,omitempty"`
}

func (Submission) TableName() string { return "submissions" }

// ── SubmissionAnswer ──────────────────────────────────────────────────────────

type SubmissionAnswer struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SubmissionID  string    `gorm:"type:uuid;not null;index" json:"submission_id"`
	QuestionID    string    `gorm:"type:uuid;not null" json:"question_id"`
	Code          string    `gorm:"type:text" json:"code"`
	Score         int       `gorm:"default:0" json:"score"`
	Judge0Token   *string   `json:"-"` // Judge0 submission token
	Stdout        *string   `gorm:"type:text" json:"stdout,omitempty"`
	Stderr        *string   `gorm:"type:text" json:"stderr,omitempty"`
	CompileOutput *string   `gorm:"type:text" json:"compile_output,omitempty"`
	StatusDesc    *string   `json:"status_desc,omitempty"`
	TestResults   string    `gorm:"type:text;default:''" json:"test_results,omitempty"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Submission *Submission `gorm:"foreignKey:SubmissionID" json:"submission,omitempty"`
	Question   *Question   `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
}

func (SubmissionAnswer) TableName() string { return "submission_answers" }

// ── CourseEnrollment ──────────────────────────────────────────────────────────

type CourseEnrollment struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	CourseID  string    `gorm:"type:uuid;not null;uniqueIndex:idx_enrollment_course_student" json:"course_id"`
	StudentID string    `gorm:"type:uuid;not null;uniqueIndex:idx_enrollment_course_student" json:"student_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"enrolled_at"`

	Course  *Course `gorm:"foreignKey:CourseID" json:"course,omitempty"`
	Student *User   `gorm:"foreignKey:StudentID" json:"student,omitempty"`
}

func (CourseEnrollment) TableName() string { return "course_enrollments" }
