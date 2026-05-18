package dashboard

import (
	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Lecturer(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	userID := c.GetString("userID")

	data, err := h.svc.GetLecturerDashboard(tenantID, userID)
	if err != nil {
		response.InternalError(c, "failed to load dashboard")
		return
	}

	response.OK(c, "dashboard retrieved", data)
}

func (h *Handler) SchoolAdmin(c *gin.Context) {
	tenantID := c.GetString("tenantID")

	data, err := h.svc.GetSchoolAdminDashboard(tenantID)
	if err != nil {
		response.InternalError(c, "failed to load dashboard")
		return
	}

	response.OK(c, "dashboard retrieved", data)
}

func (h *Handler) SuperAdmin(c *gin.Context) {
	data, err := h.svc.GetSuperAdminDashboard()
	if err != nil {
		response.InternalError(c, "failed to load dashboard")
		return
	}

	response.OK(c, "dashboard retrieved", data)
}
