package submission_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/submission"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeEnqueuer stands in for the queue client so the sweep tests never reach
// Judge0, and so they can assert which submissions were handed off for grading.
type fakeEnqueuer struct {
	mu  sync.Mutex
	ids []string
}

func (f *fakeEnqueuer) EnqueueGradeSubmission(submissionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, submissionID)
	return nil
}

func (f *fakeEnqueuer) dispatched() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ids...)
}

// startedAgo backdates a submission so its deadline sits where the test wants
// it. The exam fixture runs 120 minutes, so a submission started 121 minutes
// ago is one minute past expiry.
func startedAgo(t *testing.T, db *gorm.DB, subID string, d time.Duration) {
	t.Helper()
	require.NoError(t, db.Model(&models.Submission{}).
		Where("id = ?", subID).
		Update("started_at", time.Now().Add(-d)).Error)
}

func reload(t *testing.T, db *gorm.DB, subID string) models.Submission {
	t.Helper()
	var sub models.Submission
	require.NoError(t, db.First(&sub, "id = ?", subID).Error)
	return sub
}

func TestSweepExpired_ClosesOutAbandonedSubmission(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	enq := &fakeEnqueuer{}
	svc := submission.NewService(db, nil).WithGradingEnqueuer(enq)

	sub, err := svc.StartExam(tenantID, examID, studentID)
	require.NoError(t, err)
	startedAgo(t, db, sub.ID, 121*time.Minute)

	swept, err := svc.SweepExpired()
	require.NoError(t, err)
	assert.Equal(t, 1, swept)

	got := reload(t, db, sub.ID)
	assert.Equal(t, models.SubmissionStatusSubmitted, got.Status)
	assert.True(t, got.AutoSubmitted, "should be marked as closed out by the sweep")
	assert.NotNil(t, got.SubmittedAt)
	assert.Equal(t, []string{sub.ID}, enq.dispatched(), "should hand the submission off for grading")
}

func TestSweepExpired_LeavesLiveSubmissionAlone(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	enq := &fakeEnqueuer{}
	svc := submission.NewService(db, nil).WithGradingEnqueuer(enq)

	sub, err := svc.StartExam(tenantID, examID, studentID)
	require.NoError(t, err)
	// 30 minutes into a 120 minute exam — still has time left.
	startedAgo(t, db, sub.ID, 30*time.Minute)

	swept, err := svc.SweepExpired()
	require.NoError(t, err)
	assert.Equal(t, 0, swept)

	got := reload(t, db, sub.ID)
	assert.Equal(t, models.SubmissionStatusInProgress, got.Status)
	assert.False(t, got.AutoSubmitted)
	assert.Empty(t, enq.dispatched())
}

func TestSweepExpired_IgnoresAlreadyFinishedSubmissions(t *testing.T) {
	tests := []struct {
		name   string
		status models.SubmissionStatus
	}{
		{"already submitted by the student", models.SubmissionStatusSubmitted},
		{"already graded", models.SubmissionStatusGraded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			t.Cleanup(func() { testutil.CleanupTables(t, db) })

			tenantID, examID, studentID := seedExamFixtures(t, db)

			enq := &fakeEnqueuer{}
			svc := submission.NewService(db, nil).WithGradingEnqueuer(enq)

			sub, err := svc.StartExam(tenantID, examID, studentID)
			require.NoError(t, err)
			startedAgo(t, db, sub.ID, 121*time.Minute)
			require.NoError(t, db.Model(&models.Submission{}).
				Where("id = ?", sub.ID).
				Update("status", tt.status).Error)

			swept, err := svc.SweepExpired()
			require.NoError(t, err)
			assert.Equal(t, 0, swept)

			got := reload(t, db, sub.ID)
			assert.Equal(t, tt.status, got.Status)
			assert.False(t, got.AutoSubmitted, "a finished submission must not be relabelled")
			assert.Empty(t, enq.dispatched())
		})
	}
}

// A normal submit must stay distinguishable from a swept one, otherwise the
// flag tells nobody anything.
func TestSubmit_DoesNotSetAutoSubmitted(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	enq := &fakeEnqueuer{}
	svc := submission.NewService(db, nil).WithGradingEnqueuer(enq)

	sub, err := svc.StartExam(tenantID, examID, studentID)
	require.NoError(t, err)

	_, err = svc.Submit(sub.ID, studentID, nil)
	require.NoError(t, err)

	got := reload(t, db, sub.ID)
	assert.Equal(t, models.SubmissionStatusSubmitted, got.Status)
	assert.False(t, got.AutoSubmitted)
}

func TestSweepExpired_SweepsEveryAbandonedSubmission(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { testutil.CleanupTables(t, db) })

	tenantID, examID, studentID := seedExamFixtures(t, db)

	enq := &fakeEnqueuer{}
	svc := submission.NewService(db, nil).WithGradingEnqueuer(enq)

	// One submission per student, as StartExam allows only one per student per
	// exam. This is the lab-loses-power case.
	var ids []string
	for i, name := range []string{"ada", "bola", "chidi"} {
		student := models.User{
			TenantID:     firstStudentTenant(t, db, studentID),
			Email:        name + "@testuni2.edu",
			PasswordHash: "hash",
			Role:         models.RoleStudent,
			FirstName:    name,
			LastName:     "Test",
			IsActive:     true,
			IsVerified:   true,
		}
		require.NoError(t, db.Create(&student).Error)
		require.NoError(t, db.Create(&models.CourseEnrollment{
			TenantID:  tenantID,
			CourseID:  courseIDForExam(t, db, examID),
			StudentID: student.ID,
		}).Error)

		sub, err := svc.StartExam(tenantID, examID, student.ID)
		require.NoError(t, err)
		startedAgo(t, db, sub.ID, time.Duration(121+i)*time.Minute)
		ids = append(ids, sub.ID)
	}

	swept, err := svc.SweepExpired()
	require.NoError(t, err)
	assert.Equal(t, 3, swept)
	assert.ElementsMatch(t, ids, enq.dispatched())

	for _, id := range ids {
		got := reload(t, db, id)
		assert.Equal(t, models.SubmissionStatusSubmitted, got.Status)
		assert.True(t, got.AutoSubmitted)
	}
}

func firstStudentTenant(t *testing.T, db *gorm.DB, studentID string) *string {
	t.Helper()
	var student models.User
	require.NoError(t, db.First(&student, "id = ?", studentID).Error)
	return student.TenantID
}

func courseIDForExam(t *testing.T, db *gorm.DB, examID string) string {
	t.Helper()
	var exam models.Exam
	require.NoError(t, db.First(&exam, "id = ?", examID).Error)
	return exam.CourseID
}
