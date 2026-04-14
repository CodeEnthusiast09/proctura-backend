package submission

import (
	"errors"

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
	examID := c.Param("examID")

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
