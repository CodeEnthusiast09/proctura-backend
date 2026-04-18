package submission

import (
	"errors"
	"strconv"

	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) StartExam(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	studentID := c.GetString("userID")
	examID := c.Param("id")

	sub, err := h.svc.StartExam(tenantID, examID, studentID)
	if err != nil {
		if errors.Is(err, ErrExamNotAvailable) {
			response.BadRequest(c, err.Error())
			return
		}
		if errors.Is(err, ErrAlreadyStarted) {
			response.BadRequest(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotEnrolled) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalError(c, "failed to start exam")
		return
	}

	response.Created(c, "exam started", sub)
}

type saveAnswerRequest struct {
	QuestionID string `json:"question_id" binding:"required,uuid"`
	Code       string `json:"code" binding:"required"`
}

func (h *Handler) SaveAnswer(c *gin.Context) {
	submissionID := c.Param("id")

	var req saveAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	answer, err := h.svc.SaveAnswer(submissionID, req.QuestionID, req.Code)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrSubmissionNotActive) {
			response.BadRequest(c, err.Error())
			return
		}
		if errors.Is(err, ErrTimeExpired) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to save answer")
		return
	}

	response.OK(c, "answer saved", answer)
}

func (h *Handler) Submit(c *gin.Context) {
	submissionID := c.Param("id")
	studentID := c.GetString("userID")

	sub, err := h.svc.Submit(submissionID, studentID)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrSubmissionNotActive) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to submit exam")
		return
	}

	response.OK(c, "exam submitted successfully", sub)
}

func (h *Handler) GetMySubmissions(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	studentID := c.GetString("userID")

	subs, err := h.svc.GetMySubmissions(tenantID, studentID)
	if err != nil {
		response.InternalError(c, "failed to get submissions")
		return
	}

	response.OK(c, "submissions retrieved", subs)
}

func (h *Handler) GetMySubmission(c *gin.Context) {
	examID := c.Param("id")
	studentID := c.GetString("userID")

	sub, err := h.svc.GetMySubmission(examID, studentID)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, "no submission found")
			return
		}
		response.InternalError(c, "failed to get submission")
		return
	}

	response.OK(c, "submission retrieved", sub)
}

func (h *Handler) GetResult(c *gin.Context) {
	submissionID := c.Param("id")
	studentID := c.GetString("userID")

	sub, err := h.svc.GetResult(submissionID, studentID)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to get result")
		return
	}

	response.OK(c, "result retrieved", sub)
}

type runCodeRequest struct {
	QuestionID string `json:"question_id" binding:"required,uuid"`
	Code       string `json:"code" binding:"required"`
}

func (h *Handler) RunCode(c *gin.Context) {
	submissionID := c.Param("id")
	studentID := c.GetString("userID")

	var req runCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	results, err := h.svc.RunCode(submissionID, studentID, req.QuestionID, req.Code)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrSubmissionNotActive) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to run code")
		return
	}

	response.OK(c, "code executed", results)
}

func (h *Handler) GetAllResults(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	userID := c.GetString("userID")
	role := c.GetString("role")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	rows, total, err := h.svc.GetAllResults(tenantID, userID, role, AllResultsFilter{
		CourseID: c.Query("course_id"),
		ExamID:   c.Query("exam_id"),
		Status:   c.Query("status"),
		Search:   c.Query("search"),
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		response.InternalError(c, "failed to get results")
		return
	}

	response.Paginated(c, "results retrieved", rows, response.Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: response.CalcTotalPages(total, limit),
	})
}

func (h *Handler) GetSubmissionDetail(c *gin.Context) {
	submissionID := c.Param("id")
	lecturerID := c.GetString("userID")
	role := c.GetString("role")

	sub, err := h.svc.GetSubmissionDetail(submissionID, lecturerID, role)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, "submission not found")
			return
		}
		response.InternalError(c, "failed to get submission")
		return
	}

	response.OK(c, "submission retrieved", sub)
}

type overrideScoreRequest struct {
	Score int `json:"score" binding:"min=0"`
}

func (h *Handler) OverrideAnswerScore(c *gin.Context) {
	submissionID := c.Param("id")
	answerID := c.Param("answerId")
	lecturerID := c.GetString("userID")
	role := c.GetString("role")

	var req overrideScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sub, err := h.svc.OverrideAnswerScore(submissionID, answerID, lecturerID, role, req.Score)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, "submission not found")
			return
		}
		if errors.Is(err, ErrInvalidScore) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update score")
		return
	}

	response.OK(c, "score updated", sub)
}

type logViolationRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *Handler) LogViolation(c *gin.Context) {
	submissionID := c.Param("id")
	studentID := c.GetString("userID")

	var req logViolationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sub, err := h.svc.LogViolation(submissionID, studentID, req.Reason)
	if err != nil {
		if errors.Is(err, ErrSubmissionNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to log violation")
		return
	}

	response.OK(c, "violation logged", sub)
}
