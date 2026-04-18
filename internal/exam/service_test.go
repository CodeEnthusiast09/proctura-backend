package exam_test

import (
	"testing"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/exam"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (svc *exam.Service, tenantID, courseID, studentID string) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenant := models.Tenant{Name: "Test Uni", Subdomain: "testuni", IsActive: true}
	require.NoError(t, db.Create(&tenant).Error)

	lecturer := models.User{
		TenantID:     &tenant.ID,
		Email:        "lecturer@testuni.edu",
		PasswordHash: "hash",
		Role:         models.RoleLecturer,
		FirstName:    "Dr",
		LastName:     "Smith",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&lecturer).Error)

	course := models.Course{
		TenantID:   tenant.ID,
		LecturerID: lecturer.ID,
		Title:      "Data Structures",
		Code:       "CSC301",
	}
	require.NoError(t, db.Create(&course).Error)

	matric := "CSC/2021/001"
	student := models.User{
		TenantID:     &tenant.ID,
		Email:        "student@testuni.edu",
		PasswordHash: "hash",
		Role:         models.RoleStudent,
		FirstName:    "John",
		LastName:     "Doe",
		MatricNumber: &matric,
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&student).Error)

	enrollment := models.CourseEnrollment{
		TenantID:  tenant.ID,
		CourseID:  course.ID,
		StudentID: student.ID,
	}
	require.NoError(t, db.Create(&enrollment).Error)

	return exam.NewService(db), tenant.ID, course.ID, student.ID
}

func TestCreateExam_Success(t *testing.T) {
	svc, tenantID, courseID, _ := setup(t)

	now := time.Now()
	e, err := svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Midterm Exam",
		DurationMinutes: 90,
		StartsAt:        now.Add(1 * time.Hour),
		EndsAt:          now.Add(3 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})

	require.NoError(t, err)
	assert.Equal(t, "Midterm Exam", e.Title)
	assert.Equal(t, models.ExamStatusDraft, e.Status)
}

func TestGetExam_NotFound(t *testing.T) {
	svc, tenantID, _, _ := setup(t)

	_, err := svc.GetExam(tenantID, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, exam.ErrExamNotFound)
}

func TestUpdateExam_OnlyDraftAllowed(t *testing.T) {
	svc, tenantID, courseID, _ := setup(t)

	now := time.Now()
	e, err := svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Draft Exam",
		DurationMinutes: 60,
		StartsAt:        now.Add(1 * time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})
	require.NoError(t, err)

	// Update title while in draft — should succeed
	updated, err := svc.UpdateExam(tenantID, e.ID, map[string]any{"title": "Updated Title"})
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)

	// Change status to scheduled, then try to update — should fail
	_, err = svc.UpdateExam(tenantID, e.ID, map[string]any{"status": "scheduled"})
	require.NoError(t, err)

	_, err = svc.UpdateExam(tenantID, e.ID, map[string]any{"title": "Blocked"})
	assert.ErrorIs(t, err, exam.ErrExamNotEditable)
}

func TestAddQuestion_Success(t *testing.T) {
	svc, tenantID, courseID, _ := setup(t)

	now := time.Now()
	e, err := svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Exam",
		DurationMinutes: 60,
		StartsAt:        now.Add(1 * time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})
	require.NoError(t, err)

	q, err := svc.AddQuestion(e.ID, "Write a function to reverse a string.", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 20, q.Points)
	assert.Equal(t, e.ID, q.ExamID)
}

func TestAddTestCase_Success(t *testing.T) {
	svc, tenantID, courseID, _ := setup(t)

	now := time.Now()
	e, _ := svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Exam",
		DurationMinutes: 60,
		StartsAt:        now.Add(1 * time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})
	q, _ := svc.AddQuestion(e.ID, "Question body", 1, 10)

	input := "hello"
	tcs, err := svc.AddTestCases(q.ID, []exam.TestCaseInput{
		{Input: &input, ExpectedOutput: "olleh", IsHidden: false},
	})
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	assert.Equal(t, "olleh", tcs[0].ExpectedOutput)
	assert.False(t, tcs[0].IsHidden)
}

func TestGetAvailableExams_ReturnsOnlyActiveWindow(t *testing.T) {
	svc, tenantID, courseID, studentID := setup(t)

	now := time.Now()

	// Exam currently active and scheduled
	active, _ := svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Active Exam",
		DurationMinutes: 120,
		StartsAt:        now.Add(-1 * time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})
	_, err := svc.UpdateExam(tenantID, active.ID, map[string]any{"status": "scheduled"})
	require.NoError(t, err)

	// Exam in the future — not available yet
	_, _ = svc.CreateExam(exam.CreateExamInput{
		TenantID:        tenantID,
		CourseID:        courseID,
		Title:           "Future Exam",
		DurationMinutes: 60,
		StartsAt:        now.Add(5 * time.Hour),
		EndsAt:          now.Add(7 * time.Hour),
		LanguageID:      71,
		LanguageName:    "Python 3",
	})

	exams, err := svc.GetAvailableExams(tenantID, studentID)
	require.NoError(t, err)
	assert.Len(t, exams, 1)
	assert.Equal(t, "Active Exam", exams[0].Title)
}
