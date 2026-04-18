package exam

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrExamNotFound        = errors.New("exam not found")
	ErrQuestionNotFound    = errors.New("question not found")
	ErrTestCaseNotFound    = errors.New("test case not found")
	ErrExamNotEditable     = errors.New("exam cannot be edited after it has started")
	ErrInvalidTransition   = errors.New("invalid status transition")
)

var validTransitions = map[models.ExamStatus]models.ExamStatus{
	models.ExamStatusDraft:     models.ExamStatusScheduled,
	models.ExamStatusScheduled: models.ExamStatusActive,
	models.ExamStatusActive:    models.ExamStatusClosed,
	models.ExamStatusClosed:    models.ExamStatusDraft,
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ── Exam ──────────────────────────────────────────────────────────────────────

type CreateExamInput struct {
	TenantID        string
	CourseID        string
	Title           string
	Instructions    *string
	DurationMinutes int
	StartsAt        time.Time
	EndsAt          time.Time
	LanguageID      int
	LanguageName    string
}

func (s *Service) CreateExam(input CreateExamInput) (*models.Exam, error) {
	exam := models.Exam{
		TenantID:        input.TenantID,
		CourseID:        input.CourseID,
		Title:           input.Title,
		Instructions:    input.Instructions,
		DurationMinutes: input.DurationMinutes,
		StartsAt:        input.StartsAt,
		EndsAt:          input.EndsAt,
		LanguageID:      input.LanguageID,
		LanguageName:    input.LanguageName,
		Status:          models.ExamStatusDraft,
	}
	if err := s.db.Create(&exam).Error; err != nil {
		return nil, fmt.Errorf("create exam: %w", err)
	}
	return &exam, nil
}

func (s *Service) ListExams(tenantID, courseID string, page, limit int) ([]models.Exam, int64, error) {
	query := s.db.Where("tenant_id = ?", tenantID)
	if courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}

	var total int64
	if err := query.Model(&models.Exam{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count exams: %w", err)
	}

	var exams []models.Exam
	offset := (page - 1) * limit
	if err := query.Preload("Course").Order("starts_at DESC").Offset(offset).Limit(limit).Find(&exams).Error; err != nil {
		return nil, 0, fmt.Errorf("list exams: %w", err)
	}

	return exams, total, nil
}

func (s *Service) GetExam(tenantID, examID string) (*models.Exam, error) {
	var exam models.Exam
	if err := s.db.Preload("Questions.TestCases").
		Where("id = ? AND tenant_id = ?", examID, tenantID).
		First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("get exam: %w", err)
	}
	return &exam, nil
}

func (s *Service) UpdateExam(tenantID, examID string, updates map[string]any) (*models.Exam, error) {
	var exam models.Exam
	if err := s.db.Where("id = ? AND tenant_id = ?", examID, tenantID).First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("get exam: %w", err)
	}

	if exam.Status != models.ExamStatusDraft {
		return nil, ErrExamNotEditable
	}

	if err := s.db.Model(&exam).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update exam: %w", err)
	}

	return &exam, nil
}

func (s *Service) UpdateExamStatus(tenantID, examID string, next models.ExamStatus) (*models.Exam, error) {
	var exam models.Exam
	if err := s.db.Where("id = ? AND tenant_id = ?", examID, tenantID).First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("get exam: %w", err)
	}

	allowed, ok := validTransitions[exam.Status]
	if !ok || allowed != next {
		return nil, ErrInvalidTransition
	}

	if err := s.db.Model(&exam).Update("status", next).Error; err != nil {
		return nil, fmt.Errorf("update exam status: %w", err)
	}

	return &exam, nil
}

func (s *Service) DeleteExam(tenantID, examID string) error {
	result := s.db.Where("id = ? AND tenant_id = ?", examID, tenantID).Delete(&models.Exam{})
	if result.Error != nil {
		return fmt.Errorf("delete exam: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrExamNotFound
	}
	return nil
}

// GetAvailableExams returns active exams for courses the student is enrolled in.
func (s *Service) GetAvailableExams(tenantID, studentID string) ([]models.Exam, error) {
	var exams []models.Exam
	if err := s.db.
		Joins("JOIN course_enrollments ON course_enrollments.course_id = exams.course_id AND course_enrollments.student_id = ?", studentID).
		Where("exams.tenant_id = ? AND exams.status = ?", tenantID, models.ExamStatusActive).
		Preload("Course").
		Find(&exams).Error; err != nil {
		return nil, fmt.Errorf("get available exams: %w", err)
	}
	return exams, nil
}

// GetResults returns all submissions for an exam (lecturer view).
func (s *Service) GetResults(tenantID, examID string) ([]models.Submission, error) {
	var exam models.Exam
	if err := s.db.Where("id = ? AND tenant_id = ?", examID, tenantID).First(&exam).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrExamNotFound
		}
		return nil, fmt.Errorf("get exam: %w", err)
	}

	var submissions []models.Submission
	if err := s.db.
		Where("exam_id = ?", examID).
		Preload("Student").
		Preload("Answers").
		Find(&submissions).Error; err != nil {
		return nil, fmt.Errorf("get results: %w", err)
	}

	return submissions, nil
}

// ── Question ──────────────────────────────────────────────────────────────────

func (s *Service) AddQuestion(examID, body string, orderIndex, points int) (*models.Question, error) {
	q := models.Question{
		ExamID:     examID,
		Body:       body,
		OrderIndex: orderIndex,
		Points:     points,
	}
	if err := s.db.Create(&q).Error; err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return &q, nil
}

func (s *Service) UpdateQuestion(questionID, body string, points *int) (*models.Question, error) {
	var q models.Question
	if err := s.db.First(&q, "id = ?", questionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("get question: %w", err)
	}

	updates := map[string]any{}
	if body != "" {
		updates["body"] = body
	}
	if points != nil {
		updates["points"] = *points
	}

	if err := s.db.Model(&q).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update question: %w", err)
	}

	return &q, nil
}

func (s *Service) DeleteQuestion(questionID string) error {
	result := s.db.Delete(&models.Question{}, "id = ?", questionID)
	if result.Error != nil {
		return fmt.Errorf("delete question: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrQuestionNotFound
	}
	return nil
}

// ── TestCase ──────────────────────────────────────────────────────────────────

type TestCaseInput struct {
	Input          *string
	ExpectedOutput string
	IsHidden       bool
}

func (s *Service) AddTestCases(questionID string, inputs []TestCaseInput) ([]models.TestCase, error) {
	tcs := make([]models.TestCase, len(inputs))
	for i, inp := range inputs {
		tcs[i] = models.TestCase{
			QuestionID:     questionID,
			Input:          inp.Input,
			ExpectedOutput: inp.ExpectedOutput,
			IsHidden:       inp.IsHidden,
		}
	}
	if err := s.db.Create(&tcs).Error; err != nil {
		return nil, fmt.Errorf("create test cases: %w", err)
	}
	return tcs, nil
}

func (s *Service) UpdateTestCase(testCaseID string, input *string, expectedOutput string, isHidden *bool) (*models.TestCase, error) {
	var tc models.TestCase
	if err := s.db.First(&tc, "id = ?", testCaseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTestCaseNotFound
		}
		return nil, fmt.Errorf("get test case: %w", err)
	}

	updates := map[string]any{}
	if input != nil {
		updates["input"] = *input
	}
	if expectedOutput != "" {
		updates["expected_output"] = expectedOutput
	}
	if isHidden != nil {
		updates["is_hidden"] = *isHidden
	}

	if err := s.db.Model(&tc).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update test case: %w", err)
	}

	return &tc, nil
}

func (s *Service) DeleteTestCase(testCaseID string) error {
	result := s.db.Delete(&models.TestCase{}, "id = ?", testCaseID)
	if result.Error != nil {
		return fmt.Errorf("delete test case: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTestCaseNotFound
	}
	return nil
}
