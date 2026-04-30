package submission_test

import (
	"testing"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedExamFixtures creates tenant, lecturer, course, exam, and student for submission tests.
func seedExamFixtures(t *testing.T, db *gorm.DB) (tenantID, examID, studentID string) {
	t.Helper()

	tenant := models.Tenant{Name: "Test Uni", Subdomain: "testuni2", IsActive: true}
	require.NoError(t, db.Create(&tenant).Error)

	lecturer := models.User{
		TenantID:     &tenant.ID,
		Email:        "lect@testuni2.edu",
		PasswordHash: "hash",
		Role:         models.RoleLecturer,
		FirstName:    "Dr",
		LastName:     "Obi",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&lecturer).Error)

	course := models.Course{
		TenantID:   tenant.ID,
		LecturerID: lecturer.ID,
		Title:      "Algorithms",
		Code:       "CSC402",
	}
	require.NoError(t, db.Create(&course).Error)

	now := time.Now()
	exam := models.Exam{
		TenantID:        tenant.ID,
		CourseID:        course.ID,
		Title:           "Final Exam",
		DurationMinutes: 120,
		StartsAt:        now.Add(-30 * time.Minute),
		EndsAt:          now.Add(90 * time.Minute),
		LanguageID:      71,
		LanguageName:    "Python 3",
		Status:          models.ExamStatusScheduled,
	}
	require.NoError(t, db.Create(&exam).Error)

	student := models.User{
		TenantID:     &tenant.ID,
		Email:        "student@testuni2.edu",
		PasswordHash: "hash",
		Role:         models.RoleStudent,
		FirstName:    "Emeka",
		LastName:     "Nwosu",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&student).Error)

	return tenant.ID, exam.ID, student.ID
}

func TestStartExam_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	svc := submission.NewService(db, nil)
	sub, err := svc.StartExam(tenantID, examID, studentID)

	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusInProgress, sub.Status)
	assert.Equal(t, examID, sub.ExamID)
	assert.Equal(t, studentID, sub.StudentID)
}

func TestStartExam_AlreadyStarted(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	svc := submission.NewService(db, nil)
	_, err := svc.StartExam(tenantID, examID, studentID)
	require.NoError(t, err)

	_, err = svc.StartExam(tenantID, examID, studentID)
	assert.ErrorIs(t, err, submission.ErrAlreadyStarted)
}

func TestStartExam_ExamNotAvailable(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, _, studentID := seedExamFixtures(t, db)

	// Create an exam that hasn't started yet
	future := models.Exam{
		TenantID:        tenantID,
		CourseID:        "00000000-0000-0000-0000-000000000000", // won't matter
		Title:           "Future Exam",
		DurationMinutes: 60,
		StartsAt:        time.Now().Add(5 * time.Hour),
		EndsAt:          time.Now().Add(7 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
		Status:          models.ExamStatusScheduled,
	}

	// Get a real course id first
	var course models.Course
	require.NoError(t, db.Where("tenant_id = ?", tenantID).First(&course).Error)
	future.CourseID = course.ID
	require.NoError(t, db.Create(&future).Error)

	svc := submission.NewService(db, nil)
	_, err := svc.StartExam(tenantID, future.ID, studentID)
	assert.ErrorIs(t, err, submission.ErrExamNotAvailable)
}

func TestSaveAnswer_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	// Add a question to the exam
	question := models.Question{ExamID: examID, Body: "Write hello world", OrderIndex: 1, Points: 10}
	require.NoError(t, db.Create(&question).Error)

	svc := submission.NewService(db, nil)
	sub, err := svc.StartExam(tenantID, examID, studentID)
	require.NoError(t, err)

	answer, err := svc.SaveAnswer(sub.ID, question.ID, "print('hello world')")
	require.NoError(t, err)
	assert.Equal(t, "print('hello world')", answer.Code)
}

func TestSaveAnswer_UpdatesExistingAnswer(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	question := models.Question{ExamID: examID, Body: "Write hello world", OrderIndex: 1, Points: 10}
	require.NoError(t, db.Create(&question).Error)

	svc := submission.NewService(db, nil)
	sub, _ := svc.StartExam(tenantID, examID, studentID)

	_, _ = svc.SaveAnswer(sub.ID, question.ID, "print('v1')")
	answer, err := svc.SaveAnswer(sub.ID, question.ID, "print('v2')")

	require.NoError(t, err)
	assert.Equal(t, "print('v2')", answer.Code)
}

func TestSubmit_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	svc := submission.NewService(db, nil)
	sub, _ := svc.StartExam(tenantID, examID, studentID)

	submitted, err := svc.Submit(sub.ID, studentID, nil)
	require.NoError(t, err)
	assert.Equal(t, models.SubmissionStatusSubmitted, submitted.Status)
	assert.NotNil(t, submitted.SubmittedAt)
}

func TestLogViolation_AutoSubmitAt3(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	svc := submission.NewService(db, nil)
	sub, _ := svc.StartExam(tenantID, examID, studentID)

	_, _ = svc.LogViolation(sub.ID, studentID, "tab switch")
	_, _ = svc.LogViolation(sub.ID, studentID, "tab switch")
	result, err := svc.LogViolation(sub.ID, studentID, "tab switch")

	require.NoError(t, err)
	assert.Equal(t, 3, result.ViolationCount)
	assert.Equal(t, models.SubmissionStatusSubmitted, result.Status)
}

func TestGetResult_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	svc := submission.NewService(db, nil)
	_, err := svc.GetResult("00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000001")
	assert.ErrorIs(t, err, submission.ErrSubmissionNotFound)
}
