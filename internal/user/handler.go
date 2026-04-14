package user

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

// Me godoc
// GET /auth/me
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString("userID")

	user, err := h.svc.Me(userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to get profile")
		return
	}

	response.OK(c, "profile retrieved", user)
}

// InviteLecturer godoc
// POST /users/invite
type inviteLecturerRequest struct {
	Email     string `json:"email" binding:"required,email"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

func (h *Handler) InviteLecturer(c *gin.Context) {
	var req inviteLecturerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tenantID := c.GetString("tenantID")

	user, token, err := h.svc.InviteLecturer(tenantID, req.Email, req.FirstName, req.LastName)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to invite lecturer")
		return
	}

	response.Created(c, "lecturer invited — they will receive an email to set up their account", gin.H{
		"id":           user.ID,
		"email":        user.Email,
		"invite_token": token,
	})
}

// ImportStudents godoc
// POST /users/import
func (h *Handler) ImportStudents(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "CSV file is required")
		return
	}
	defer file.Close()

	tenantID := c.GetString("tenantID")

	created, skipped, err := h.svc.ImportStudents(tenantID, file)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.OK(c, "import complete", gin.H{
		"created": created,
		"skipped": skipped,
	})
}

// List godoc
// GET /users
func (h *Handler) List(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	role := c.Query("role")
	page, limit := parsePagination(c)

	users, total, err := h.svc.List(tenantID, role, page, limit)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}

	response.Paginated(c, "users retrieved", users, response.Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: response.CalcTotalPages(total, limit),
	})
}

// Update godoc
// PATCH /users/:id
type updateUserRequest struct {
	IsActive *bool `json:"is_active"`
}

func (h *Handler) Update(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	userID := c.Param("id")

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.svc.Update(tenantID, userID, req.IsActive)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update user")
		return
	}

	response.OK(c, "user updated", user)
}

// Delete godoc
// DELETE /users/:id
func (h *Handler) Delete(c *gin.Context) {
	tenantID := c.GetString("tenantID")
	userID := c.Param("id")

	if err := h.svc.Delete(tenantID, userID); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete user")
		return
	}

	response.OK(c, "user deleted", nil)
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
