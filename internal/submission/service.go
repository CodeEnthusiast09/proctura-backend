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
	if err := s.db.Where("id = ? AND tenant_id = ? AND starts_at <= ? AND ends_at >= ? AND status = ?",
		examID, tenantID, now, now, models.ExamStatusScheduled).
		First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotAvailable
		}
		return nil, fmt.Errorf("get exam: %w", err)
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

	if isTimeExpired(sub) {
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
		if err := s.db.Create(&answer).Error; err != nil {
			return nil, fmt.Errorf("create answer: %w", err)
		}
	} else if err == nil {
		if err := s.db.Model(&answer).Update("code", code).Error; err != nil {
			return nil, fmt.Errorf("update answer: %w", err)
		}
	} else {
		return nil, fmt.Errorf("get answer: %w", err)
	}

	return &answer, nil
}

// Submit marks the exam as submitted and triggers Judge0 evaluation for all answers.
func (s *Service) Submit(submissionID, studentID string) (*models.Submission, error) {
	var sub models.Submission
	if err := s.db.Preload("Answers").First(&sub, "id = ? AND student_id = ?", submissionID, studentID).Error; err != nil {
		return nil, ErrSubmissionNotFound
	}

	if sub.Status != models.SubmissionStatusInProgress {
		return nil, ErrSubmissionNotActive
	}

	now := time.Now()
	if err := s.db.Model(&sub).Updates(map[string]any{
		"status":       models.SubmissionStatusSubmitted,
		"submitted_at": now,
	}).Error; err != nil {
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
			"test_results": []string{resultsStr},
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

func isTimeExpired(sub models.Submission) bool {
	var exam models.Exam
	// We don't have exam here — check via duration later if needed
	_ = exam
	return false
}
