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

// Staff handles GET /dashboard for both school_admin and lecturer.
// Role is read from the JWT claims set by the auth middleware.
func (h *Handler) Staff(c *gin.Context) {
	role := c.GetString("role")
	tenantID := c.GetString("tenantID")
	userID := c.GetString("userID")

	switch role {
	case "lecturer":
		data, err := h.svc.GetLecturerDashboard(tenantID, userID)
		if err != nil {
			response.InternalError(c, "failed to load dashboard")
			return
		}
		response.OK(c, "dashboard retrieved", data)

	case "school_admin":
		data, err := h.svc.GetSchoolAdminDashboard(tenantID)
		if err != nil {
			response.InternalError(c, "failed to load dashboard")
			return
		}
		response.OK(c, "dashboard retrieved", data)

	default:
		response.Forbidden(c, "access denied")
	}
}

func (h *Handler) SuperAdmin(c *gin.Context) {
	data, err := h.svc.GetSuperAdminDashboard()
	if err != nil {
		response.InternalError(c, "failed to load dashboard")
		return
	}

	response.OK(c, "dashboard retrieved", data)
}
