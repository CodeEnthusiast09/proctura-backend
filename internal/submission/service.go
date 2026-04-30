package submission

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrSubmissionNotFound  = errors.New("submission not found")
	ErrExamNotAvailable    = errors.New("exam is not currently available")
	ErrAlreadyStarted      = errors.New("you have already started this exam")
	ErrSubmissionNotActive = errors.New("submission is not active")
	ErrTimeExpired         = errors.New("your exam time has expired")
	ErrNotEnrolled         = errors.New("you are not enrolled in this course")
)

type Service struct {
	db     *gorm.DB
	judge0 *Judge0Client
}

func NewService(db *gorm.DB, judge0 *Judge0Client) *Service {
	return &Service{db: db, judge0: judge0}
}

// StartExam creates a submission record when a student begins an exam.
func (s *Service) StartExam(tenantID, examID, studentID string) (*models.Submission, error) {
	// Verify exam exists and is available
	var exam models.Exam
	now := time.Now()
	if err := s.db.Where("id = ? AND tenant_id = ? AND status = ?",
		examID, tenantID, models.ExamStatusActive).
		First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotAvailable
		}
		return nil, fmt.Errorf("get exam: %w", err)
	}
	_ = now

	// Check student is enrolled in the course
	var enrollment models.CourseEnrollment
	if err := s.db.Where("course_id = ? AND student_id = ?", exam.CourseID, studentID).
		First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotEnrolled
		}
		return nil, fmt.Errorf("check enrollment: %w", err)
	}

	// Check if student already started
	var existing models.Submission
	if err := s.db.Where("exam_id = ? AND student_id = ?", examID, studentID).
		First(&existing).Error; err == nil {
		return nil, ErrAlreadyStarted
	}

	submission := models.Submission{
		TenantID:  tenantID,
		ExamID:    examID,
		StudentID: studentID,
		Status:    models.SubmissionStatusInProgress,
		StartedAt: now,
		MaxScore:  calcMaxScore(s.db, examID),
	}

	if err := s.db.Create(&submission).Error; err != nil {
		return nil, fmt.Errorf("create submission: %w", err)
	}

	return &submission, nil
}

// SaveAnswer saves or updates the student's code for a question.
func (s *Service) SaveAnswer(submissionID, questionID, code string) (*models.SubmissionAnswer, error) {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ?", submissionID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}

	if sub.Status != models.SubmissionStatusInProgress {
		return nil, ErrSubmissionNotActive
	}

	if s.isTimeExpired(sub) {
		return nil, ErrTimeExpired
	}

	var answer models.SubmissionAnswer
	err := s.db.Where("submission_id = ? AND question_id = ?", submissionID, questionID).First(&answer).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		answer = models.SubmissionAnswer{
			SubmissionID: submissionID,
			QuestionID:   questionID,
			Code:         code,
		}
		if createErr := s.db.Create(&answer).Error; createErr != nil {
			return nil, fmt.Errorf("create answer: %w", createErr)
		}
	} else if err == nil {
		if updateErr := s.db.Model(&answer).Update("code", code).Error; updateErr != nil {
			return nil, fmt.Errorf("update answer: %w", updateErr)
		}
	} else {
		return nil, fmt.Errorf("get answer: %w", err)
	}

	return &answer, nil
}

// Submit marks the exam as submitted and triggers Judge0 evaluation for all answers.
func (s *Service) Submit(submissionID, studentID string, recordingURL *string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.Preload("Answers").First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}

	if sub.Status != models.SubmissionStatusInProgress {
		return nil, ErrSubmissionNotActive
	}

	now := time.Now()
	updates := map[string]any{
		"status":       models.SubmissionStatusSubmitted,
		"submitted_at": now,
	}
	if recordingURL != nil && *recordingURL != "" {
		updates["recording_url"] = *recordingURL
	}
	if err := s.db.Model(&sub).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update submission: %w", err)
	}

	// Fetch exam language
	var exam models.Exam
	if err := s.db.First(&exam, "id = ?", sub.ExamID).Error; err != nil {
		return nil, fmt.Errorf("get exam: %w", err)
	}

	// Submit each answer to Judge0 asynchronously
	go s.gradeSubmission(&sub, exam.LanguageID)

	return &sub, nil
}

// AllResultsFilter holds query parameters for GetAllResults.
type AllResultsFilter struct {
	CourseID string
	ExamID   string
	Status   string
	Search   string
	Page     int
	Limit    int
}

// AllResultsRow is the response shape for the all-results endpoint.
type AllResultsRow struct {
	models.Submission
	ExamTitle  string `json:"exam_title"`
	CourseCode string `json:"course_code"`
}

// RunCodeResult holds the result of a single public test case run.
type RunCodeResult struct {
	TestCaseID     string `json:"test_case_id"`
	Passed         bool   `json:"passed"`
	Input          string `json:"input,omitempty"`
	ExpectedOutput string `json:"expected_output"`
	ActualOutput   string `json:"actual_output"`
	StatusDesc     string `json:"status_desc"`
}

// RunCode executes the student's code against public test cases only (no grade impact).
func (s *Service) RunCode(submissionID, studentID, questionID, code string) ([]RunCodeResult, error) {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}
	if sub.Status != models.SubmissionStatusInProgress {
		return nil, ErrSubmissionNotActive
	}

	var exam models.Exam
	if err := s.db.First(&exam, "id = ?", sub.ExamID).Error; err != nil {
		return nil, fmt.Errorf("get exam: %w", err)
	}

	var testCases []models.TestCase
	if err := s.db.Where("question_id = ? AND is_hidden = false", questionID).Find(&testCases).Error; err != nil {
		return nil, fmt.Errorf("get test cases: %w", err)
	}

	results := make([]RunCodeResult, 0, len(testCases))
	for _, tc := range testCases {
		token, err := s.judge0.Submit(SubmitRequest{
			SourceCode:     code,
			LanguageID:     exam.LanguageID,
			Stdin:          tc.Input,
			ExpectedOutput: &tc.ExpectedOutput,
		})
		if err != nil {
			results = append(results, RunCodeResult{
				TestCaseID:     tc.ID,
				Passed:         false,
				ExpectedOutput: tc.ExpectedOutput,
				StatusDesc:     "submission error",
			})
			continue
		}

		var result *ResultResponse
		for range 10 {
			result, err = s.judge0.GetResult(token)
			if err != nil || !IsProcessing(result.Status.ID) {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if err != nil || result == nil {
			results = append(results, RunCodeResult{
				TestCaseID:     tc.ID,
				Passed:         false,
				ExpectedOutput: tc.ExpectedOutput,
				StatusDesc:     "failed to get result",
			})
			continue
		}

		actualOutput := ""
		if result.Stdout != nil {
			actualOutput = strings.TrimSpace(*result.Stdout)
		}

		input := ""
		if tc.Input != nil {
			input = *tc.Input
		}

		results = append(results, RunCodeResult{
			TestCaseID:     tc.ID,
			Passed:         result.Status.ID == 3,
			Input:          input,
			ExpectedOutput: tc.ExpectedOutput,
			ActualOutput:   actualOutput,
			StatusDesc:     result.Status.Description,
		})
	}

	return results, nil
}

// GetAllResults returns paginated submissions across all exams, scoped by role.
// Lecturers see only their own exams; admins see all tenant submissions.
func (s *Service) GetAllResults(tenantID, userID, role string, f AllResultsFilter) ([]AllResultsRow, int64, error) {
	q := s.db.Model(&models.Submission{}).
		Joins("JOIN exams ON exams.id = submissions.exam_id").
		Joins("JOIN courses ON courses.id = exams.course_id").
		Joins("JOIN users ON users.id = submissions.student_id").
		Where("submissions.tenant_id = ?", tenantID)

	if role == string(models.RoleLecturer) {
		q = q.Where("courses.lecturer_id = ?", userID)
	}
	if f.CourseID != "" {
		q = q.Where("courses.id = ?", f.CourseID)
	}
	if f.ExamID != "" {
		q = q.Where("submissions.exam_id = ?", f.ExamID)
	}
	if f.Status != "" {
		q = q.Where("submissions.status = ?", f.Status)
	}
	if f.Search != "" {
		like := "%" + strings.ToLower(f.Search) + "%"
		q = q.Where(
			"LOWER(users.first_name || ' ' || users.last_name) LIKE ? OR LOWER(users.matric_number) LIKE ?",
			like, like,
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count results: %w", err)
	}

	offset := (f.Page - 1) * f.Limit

	var subs []models.Submission
	if err := q.
		Preload("Student").
		Preload("Exam.Course").
		Order("submissions.submitted_at DESC NULLS LAST").
		Limit(f.Limit).Offset(offset).
		Find(&subs).Error; err != nil {
		return nil, 0, fmt.Errorf("get results: %w", err)
	}

	rows := make([]AllResultsRow, len(subs))
	for i, sub := range subs {
		rows[i] = AllResultsRow{Submission: sub}
		if sub.Exam != nil {
			rows[i].ExamTitle = sub.Exam.Title
			if sub.Exam.Course != nil {
				rows[i].CourseCode = sub.Exam.Course.Code
			}
		}
	}

	return rows, total, nil
}

// GetMySubmissions returns all submissions for a student across all exams.
func (s *Service) GetMySubmissions(tenantID, studentID string) ([]models.Submission, error) {
	var subs []models.Submission
	if err := s.db.
		Where("tenant_id = ? AND student_id = ?", tenantID, studentID).
		Preload("Exam.Course").
		Order("created_at DESC").
		Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("get submissions: %w", err)
	}
	return subs, nil
}

// GetMySubmission returns a student's submission for a given exam, if one exists.
func (s *Service) GetMySubmission(examID, studentID string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.Where("exam_id = ? AND student_id = ?", examID, studentID).
		First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, fmt.Errorf("get submission: %w", err)
	}
	return &sub, nil
}

// GetResult returns a submission with all answers and scores.
func (s *Service) GetResult(submissionID, studentID string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.Preload("Answers").
		First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, fmt.Errorf("get submission: %w", err)
	}
	return &sub, nil
}

// GetSubmissionDetail returns a submission with full answers + question bodies for lecturer review.
// Only accessible for submissions belonging to exams in courses the lecturer owns.
func (s *Service) GetSubmissionDetail(submissionID, lecturerID, role string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.
		Preload("Answers.Question.TestCases").
		Preload("Student").
		Preload("Exam.Course").
		First(&sub, "id = ?", submissionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, fmt.Errorf("get submission: %w", err)
	}

	// Lecturers can only view submissions for their own courses
	if role == "lecturer" && sub.Exam != nil && sub.Exam.Course != nil {
		if sub.Exam.Course.LecturerID != lecturerID {
			return nil, ErrSubmissionNotFound
		}
	}

	return &sub, nil
}

var ErrInvalidScore = errors.New("score cannot exceed question max points")

// OverrideAnswerScore lets a lecturer manually set the score for a submission answer.
// Recalculates the submission total score after the update.
func (s *Service) OverrideAnswerScore(submissionID, answerID, lecturerID, role string, score int) (*models.Submission, error) {
	// Load answer with question to check max points
	var answer models.SubmissionAnswer
	if err := s.db.Preload("Question").
		First(&answer, "id = ? AND submission_id = ?", answerID, submissionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, fmt.Errorf("get answer: %w", err)
	}

	if answer.Question != nil && score > answer.Question.Points {
		return nil, ErrInvalidScore
	}

	// Verify lecturer owns this course
	var sub models.Submission
	if err := s.db.Preload("Exam.Course").First(&sub, "id = ?", submissionID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}
	if role == "lecturer" && sub.Exam != nil && sub.Exam.Course != nil {
		if sub.Exam.Course.LecturerID != lecturerID {
			return nil, ErrSubmissionNotFound
		}
	}

	// Update answer score
	if err := s.db.Model(&answer).Update("score", score).Error; err != nil {
		return nil, fmt.Errorf("update score: %w", err)
	}

	// Recalculate total score across all answers for this submission
	var totalScore int
	s.db.Model(&models.SubmissionAnswer{}).
		Where("submission_id = ?", submissionID).
		Select("COALESCE(SUM(score), 0)").
		Scan(&totalScore)

	if err := s.db.Model(&sub).Update("total_score", totalScore).Error; err != nil {
		return nil, fmt.Errorf("update total score: %w", err)
	}
	sub.TotalScore = totalScore

	return &sub, nil
}

// IsInProgress checks whether a submission is still active, used by the handler
// before issuing a Cloudinary upload signature.
func (s *Service) IsInProgress(submissionID, studentID string) error {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		return ErrSubmissionNotFound
	}
	if sub.Status != models.SubmissionStatusInProgress {
		return ErrSubmissionNotActive
	}
	return nil
}

// OwnedByStudent checks only that the submission exists and belongs to the student,
// without requiring any particular status.
func (s *Service) OwnedByStudent(submissionID, studentID string) error {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSubmissionNotFound
		}
		return err
	}
	return nil
}

// LogViolation increments the violation count and auto-submits after 3 violations.
func (s *Service) LogViolation(submissionID, studentID, reason string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}

	if sub.Status != models.SubmissionStatusInProgress {
		return &sub, nil
	}

	newCount := sub.ViolationCount + 1
	updates := map[string]any{"violation_count": newCount}

	// Auto-submit after 3 violations
	if newCount >= 3 {
		now := time.Now()
		updates["status"] = models.SubmissionStatusSubmitted
		updates["submitted_at"] = now

		var exam models.Exam
		if err := s.db.First(&exam, "id = ?", sub.ExamID).Error; err == nil {
			sub.Status = models.SubmissionStatusInProgress
			go s.gradeSubmission(&sub, exam.LanguageID)
		}
	}

	if err := s.db.Model(&sub).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("log violation: %w", err)
	}

	if err := s.db.First(&sub, "id = ?", submissionID).Error; err != nil {
		return nil, fmt.Errorf("reload submission: %w", err)
	}

	return &sub, nil
}

// AttachRecording updates the recording URL on an already-submitted submission.
// Called by the student's client after the background upload completes.
func (s *Service) AttachRecording(submissionID, studentID, recordingURL string) error {
	var sub models.Submission
	if err := s.db.First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSubmissionNotFound
		}
		return err
	}
	return s.db.Model(&sub).Update("recording_url", recordingURL).Error
}

// gradeSubmission runs in a goroutine after final submit.
// For each answer, it submits to Judge0 and checks against all test cases.
func (s *Service) gradeSubmission(sub *models.Submission, languageID int) {
	var answers []models.SubmissionAnswer
	if err := s.db.Where("submission_id = ?", sub.ID).Find(&answers).Error; err != nil {
		return
	}

	totalScore := 0

	for i := range answers {
		answer := &answers[i]

		var testCases []models.TestCase
		if err := s.db.Where("question_id = ?", answer.QuestionID).Find(&testCases).Error; err != nil {
			continue
		}

		var question models.Question
		if err := s.db.First(&question, "id = ?", answer.QuestionID).Error; err != nil {
			continue
		}

		score, testResults := s.runAgainstTestCases(answer.Code, languageID, testCases)

		// Partial scoring: score = (passed / total) * question.Points
		pointsEarned := 0
		if len(testCases) > 0 {
			pointsEarned = (score * question.Points) / len(testCases)
		}

		resultsJSON, _ := json.Marshal(testResults)
		resultsStr := string(resultsJSON)

		s.db.Model(answer).Updates(map[string]any{
			"score":        pointsEarned,
			"test_results": resultsStr,
		})

		totalScore += pointsEarned
	}

	s.db.Model(sub).Updates(map[string]any{
		"status":      models.SubmissionStatusGraded,
		"total_score": totalScore,
	})
}

type testCaseResult struct {
	TestCaseID     string `json:"test_case_id"`
	Passed         bool   `json:"passed"`
	IsHidden       bool   `json:"is_hidden"`
	ActualOutput   string `json:"actual_output,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	StatusDesc     string `json:"status_desc"`
}

func (s *Service) runAgainstTestCases(code string, languageID int, testCases []models.TestCase) (int, []testCaseResult) {
	passed := 0
	results := make([]testCaseResult, 0, len(testCases))

	for _, tc := range testCases {
		token, err := s.judge0.Submit(SubmitRequest{
			SourceCode:     code,
			LanguageID:     languageID,
			Stdin:          tc.Input,
			ExpectedOutput: &tc.ExpectedOutput,
		})
		if err != nil {
			results = append(results, testCaseResult{
				TestCaseID: tc.ID,
				Passed:     false,
				IsHidden:   tc.IsHidden,
				StatusDesc: "submission error",
			})
			continue
		}

		// Poll for result (max 10 seconds)
		var result *ResultResponse
		for range 10 {
			result, err = s.judge0.GetResult(token)
			if err != nil || !IsProcessing(result.Status.ID) {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if err != nil || result == nil {
			results = append(results, testCaseResult{
				TestCaseID: tc.ID,
				Passed:     false,
				IsHidden:   tc.IsHidden,
				StatusDesc: "failed to get result",
			})
			continue
		}

		// Status ID 3 = Accepted
		testPassed := result.Status.ID == 3

		actualOutput := ""
		if result.Stdout != nil {
			actualOutput = strings.TrimSpace(*result.Stdout)
		}

		r := testCaseResult{
			TestCaseID: tc.ID,
			Passed:     testPassed,
			IsHidden:   tc.IsHidden,
			StatusDesc: result.Status.Description,
		}
		if !tc.IsHidden {
			r.ActualOutput = actualOutput
			r.ExpectedOutput = tc.ExpectedOutput
		}

		results = append(results, r)
		if testPassed {
			passed++
		}
	}

	return passed, results
}

func calcMaxScore(db *gorm.DB, examID string) int {
	var total int64
	db.Model(&models.Question{}).
		Where("exam_id = ?", examID).
		Select("COALESCE(SUM(points), 0)").
		Scan(&total)
	return int(total)
}

func (s *Service) isTimeExpired(sub models.Submission) bool {
	var exam models.Exam
	if err := s.db.Select("duration_minutes").First(&exam, "id = ?", sub.ExamID).Error; err != nil {
		return false // can't verify — don't block
	}
	deadline := sub.StartedAt.Add(time.Duration(exam.DurationMinutes) * time.Minute)
	return time.Now().After(deadline)
}
