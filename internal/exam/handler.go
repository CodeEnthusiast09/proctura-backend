package exam

import (
	"errors"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ── Exam handlers ─────────────────────────────────────────────────────────────

type createExamRequest struct {
	CourseID        string  `json:"course_id" binding:"required,uuid"`
	Title           string  `json:"title" binding:"required"`
	Instructions    *string `json:"instructions"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,min=1"`
	StartsAt        string  `json:"starts_at" binding:"required"` // RFC3339
	EndsAt          string  `json:"ends_at" binding:"required"`
	LanguageID      int     `json:"language_id" binding:"required"`
	LanguageName    string  `json:"language_name" binding:"required"`
}

func (h *Handler) CreateExam(c *gin.Context) {
	var req createExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		response.BadRequest(c, "invalid starts_at format — use RFC3339 e.g. 2025-06-01T09:00:00Z")
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		response.BadRequest(c, "invalid ends_at format — use RFC3339")
		return
	}

	tenantID := c.GetString("tenantID")

	exam, err := h.svc.CreateExam(CreateExamInput{
		TenantID:        tenantID,
		CourseID:        req.CourseID,
		Title:           req.Title,
		Instructions:    req.Instructions,
		DurationMinutes: req.DurationMinutes,
		StartsAt:        startsAt,
		EndsAt:          endsAt,
		LanguageID:      req.LanguageID,
		LanguageName:    req.LanguageName,
	})
	if err != nil {
		response.InternalError(c, "failed to create exam")
		return
	}

	response.Created(c, "exam created", exam)
}

func (h *Handler) ListExams(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Query("course_id")
	page, limit := parsePagination(c)

	exams, total, err := h.svc.ListExams(tenantID, courseID, page, limit)
	if err != nil {
		response.InternalError(c, "failed to list exams")
		return
	}

	response.Paginated(c, "exams retrieved", exams, response.Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: response.CalcTotalPages(total, limit),
	})
}

func (h *Handler) GetExam(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	examID := c.Param("id")

	exam, err := h.svc.GetExam(tenantID, examID)
	if err != nil {
		if errors.Is(err, ErrExamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to get exam")
		return
	}

	response.OK(c, "exam retrieved", exam)
}

type updateExamRequest struct {
	Title           string  `json:"title"`
	Instructions    *string `json:"instructions"`
	DurationMinutes *int    `json:"duration_minutes"`
	Status          string  `json:"status"`
}

func (h *Handler) UpdateExam(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	examID := c.Param("id")

	var req updateExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updates := map[string]any{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Instructions != nil {
		updates["instructions"] = *req.Instructions
	}
	if req.DurationMinutes != nil {
		updates["duration_minutes"] = *req.DurationMinutes
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	exam, err := h.svc.UpdateExam(tenantID, examID, updates)
	if err != nil {
		if errors.Is(err, ErrExamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrExamNotEditable) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update exam")
		return
	}

	response.OK(c, "exam updated", exam)
}

func (h *Handler) DeleteExam(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	examID := c.Param("id")

	if err := h.svc.DeleteExam(tenantID, examID); err != nil {
		if errors.Is(err, ErrExamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete exam")
		return
	}

	response.OK(c, "exam deleted", nil)
}

func (h *Handler) GetAvailableExams(c *gin.Context) {
	tenantID := c.GetString("tenantID")

	exams, err := h.svc.GetAvailableExams(tenantID)
	if err != nil {
		response.InternalError(c, "failed to get available exams")
		return
	}

	response.OK(c, "available exams retrieved", exams)
}

func (h *Handler) GetResults(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	examID := c.Param("id")

	submissions, err := h.svc.GetResults(tenantID, examID)
	if err != nil {
		if errors.Is(err, ErrExamNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to get results")
		return
	}

	response.OK(c, "results retrieved", submissions)
}

// ── Question handlers ─────────────────────────────────────────────────────────

type addQuestionRequest struct {
	Body       string `json:"body" binding:"required"`
	OrderIndex int    `json:"order_index"`
	Points     int    `json:"points" binding:"required,min=1"`
}

func (h *Handler) AddQuestion(c *gin.Context) {
	examID := c.Param("id")

	var req addQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	q, err := h.svc.AddQuestion(examID, req.Body, req.OrderIndex, req.Points)
	if err != nil {
		response.InternalError(c, "failed to add question")
		return
	}

	response.Created(c, "question added", q)
}

type updateQuestionRequest struct {
	Body   string `json:"body"`
	Points *int   `json:"points"`
}

func (h *Handler) UpdateQuestion(c *gin.Context) {
	questionID := c.Param("id")

	var req updateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	q, err := h.svc.UpdateQuestion(questionID, req.Body, req.Points)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update question")
		return
	}

	response.OK(c, "question updated", q)
}

func (h *Handler) DeleteQuestion(c *gin.Context) {
	questionID := c.Param("id")

	if err := h.svc.DeleteQuestion(questionID); err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete question")
		return
	}

	response.OK(c, "question deleted", nil)
}

// ── TestCase handlers ─────────────────────────────────────────────────────────

type addTestCaseRequest struct {
	Input          *string `json:"input"`
	ExpectedOutput string  `json:"expected_output" binding:"required"`
	IsHidden       bool    `json:"is_hidden"`
}

func (h *Handler) AddTestCase(c *gin.Context) {
	questionID := c.Param("id")

	var req addTestCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tc, err := h.svc.AddTestCase(questionID, req.Input, req.ExpectedOutput, req.IsHidden)
	if err != nil {
		response.InternalError(c, "failed to add test case")
		return
	}

	response.Created(c, "test case added", tc)
}

type updateTestCaseRequest struct {
	Input          *string `json:"input"`
	ExpectedOutput string  `json:"expected_output"`
	IsHidden       *bool   `json:"is_hidden"`
}

func (h *Handler) UpdateTestCase(c *gin.Context) {
	testCaseID := c.Param("id")

	var req updateTestCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tc, err := h.svc.UpdateTestCase(testCaseID, req.Input, req.ExpectedOutput, req.IsHidden)
	if err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update test case")
		return
	}

	response.OK(c, "test case updated", tc)
}

func (h *Handler) DeleteTestCase(c *gin.Context) {
	testCaseID := c.Param("id")

	if err := h.svc.DeleteTestCase(testCaseID); err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete test case")
		return
	}

	response.OK(c, "test case deleted", nil)
}

func parsePagination(c *gin.Context) (int, int) {
	page, limit := 1, 20
	if p := c.Query("page"); p != "" {
		if v := parseInt(p); v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v := parseInt(l); v > 0 && v <= 100 {
			limit = v
		}
	}
	return page, limit
}

func parseInt(s string) int {
	v := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		v = v*10 + int(ch-'0')
	}
	return v
}
