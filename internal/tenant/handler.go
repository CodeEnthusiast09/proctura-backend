package tenant

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

// Create godoc
// POST /tenants
type createTenantRequest struct {
	Name           string `json:"name" binding:"required"`
	Subdomain      string `json:"subdomain" binding:"required"`
	AdminEmail     string `json:"admin_email" binding:"required,email"`
	AdminFirstName string `json:"admin_first_name" binding:"required"`
	AdminLastName  string `json:"admin_last_name" binding:"required"`
}

func (h *Handler) Create(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Type conversion is safe here because createTenantRequest and CreateTenantInput
	// are structurally identical. If the request struct ever gains fields that the
	// service does not need (e.g. confirm email, agree to terms), or the service
	// gains fields sourced outside the request (e.g. from auth context), switch
	// back to an explicit field mapping.
	tenant, admin, err := h.svc.Create(CreateTenantInput(req))
	if err != nil {
		if errors.Is(err, ErrSubdomainTaken) || errors.Is(err, ErrAdminEmailTaken) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to create school")
		return
	}

	response.Created(c, "school created successfully", gin.H{
		"tenant": tenant,
		"admin": gin.H{
			"id":           admin.ID,
			"email":        admin.Email,
			"invite_token": admin.InviteToken,
		},
	})
}

// List godoc
// GET /tenants
func (h *Handler) List(c *gin.Context) {
	page, limit := parsePagination(c)

	tenants, total, err := h.svc.List(page, limit)
	if err != nil {
		response.InternalError(c, "failed to list schools")
		return
	}

	response.Paginated(c, "schools retrieved", tenants, response.Meta{
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: response.CalcTotalPages(total, limit),
	})
}

// Update godoc
// PATCH /tenants/:id
type updateTenantRequest struct {
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"`
}

func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")

	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tenant, err := h.svc.Update(id, req.Name, req.IsActive)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to update school")
		return
	}

	response.OK(c, "school updated", tenant)
}

// Delete godoc
// DELETE /tenants/:id
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "failed to delete school")
		return
	}

	response.OK(c, "school deleted", nil)
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	limit := 20
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
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + int(c-'0')
	}
	return v
}
