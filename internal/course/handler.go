package course

import (
	"errors"
	"fmt"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createCourseRequest struct {
	Title string `json:"title" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tenantID := c.GetString("tenantID")
	lecturerID := c.GetString("userID")

	course, err := h.svc.Create(tenantID, lecturerID, req.Title, req.Code)
	if err != nil {
		response.InternalError(c, "failed to create course")
		return
	}

	response.Created(c, "course created", course)
}

func (h *Handler) List(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	role := c.GetString("role")

	// Role-based filtering: lecturers see own courses, students see enrolled only, admins see all
	lecturerID, studentID := "", ""
	switch role {
	case string(models.RoleLecturer):
		lecturerID = c.GetString("userID")
	case string(models.RoleStudent):
		studentID = c.GetString("userID")
	}

	page, limit := parsePagination(c)

	courses, total, err := h.svc.List(tenantID, lecturerID, studentID, page, limit)
	if err != nil {
		response.InternalError(c, "failed to list courses")
		return
	}

	response.Paginated(c, "courses retrieved", courses, response.Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: response.CalcTotalPages(total, limit),
	})
}

type updateCourseRequest struct {
	Title string `json:"title"`
	Code  string `json:"code"`
}

func (h *Handler) Update(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Param("id")

	var req updateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	course, err := h.svc.Update(tenantID, courseID, req.Title, req.Code)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update course")
		return
	}

	response.OK(c, "course updated", course)
}

func (h *Handler) Delete(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Param("id")

	if err := h.svc.Delete(tenantID, courseID); err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete course")
		return
	}

	response.OK(c, "course deleted", nil)
}

type enrollRequest struct {
	MatricNumbers []string `json:"matric_numbers" binding:"required,min=1"`
}

func (h *Handler) Enroll(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Param("id")
	requesterID := c.GetString("userID")
	requesterRole := c.GetString("role")

	var req enrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	count, err := h.svc.Enroll(tenantID, courseID, requesterID, requesterRole, req.MatricNumbers)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotCourseOwner) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalError(c, "failed to enroll students")
		return
	}

	response.OK(c, fmt.Sprintf("%d student(s) enrolled", count), nil)
}

func (h *Handler) Unenroll(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Param("id")
	studentID := c.Param("studentId")
	requesterID := c.GetString("userID")
	requesterRole := c.GetString("role")

	if err := h.svc.Unenroll(tenantID, courseID, studentID, requesterID, requesterRole); err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotCourseOwner) {
			response.Forbidden(c, err.Error())
			return
		}
		if errors.Is(err, ErrEnrollmentNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to unenroll student")
		return
	}

	response.OK(c, "student unenrolled", nil)
}

func (h *Handler) ListEnrollments(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	courseID := c.Param("id")
	requesterID := c.GetString("userID")
	requesterRole := c.GetString("role")

	enrollments, err := h.svc.ListEnrollments(tenantID, courseID, requesterID, requesterRole)
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotCourseOwner) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalError(c, "failed to list enrollments")
		return
	}

	response.OK(c, "enrollments retrieved", enrollments)
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
