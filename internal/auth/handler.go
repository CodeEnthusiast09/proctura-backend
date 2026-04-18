package auth

import (
	"errors"
	"net/http"

	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Login godoc
// POST /auth/login
type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	token, user, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			response.Unauthorized(c, err.Error())
			return
		}
		if errors.Is(err, ErrAccountInactive) || errors.Is(err, ErrAccountUnverified) {
			response.Forbidden(c, err.Error())
			return
		}
		response.InternalError(c, "login failed")
		return
	}

	// Fire login notification email asynchronously
	go h.svc.SendLoginNotification(user.Email, user.FirstName, c.ClientIP())

	var subdomain *string
	if user.Tenant != nil {
		subdomain = &user.Tenant.Subdomain
	}

	response.OK(c, "login successful", gin.H{
		"access_token": token,
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"tenant_id":  user.TenantID,
			"subdomain":  subdomain,
		},
	})
}

// RegisterStudent godoc
// POST /auth/register
type registerRequest struct {
	Subdomain    string `json:"subdomain" binding:"required"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	FirstName    string `json:"first_name" binding:"required"`
	LastName     string `json:"last_name" binding:"required"`
	MatricNumber string `json:"matric_number" binding:"required"`
}

func (h *Handler) RegisterStudent(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.svc.RegisterStudent(
		req.Subdomain, req.Email, req.Password,
		req.FirstName, req.LastName, req.MatricNumber,
	)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		if errors.Is(err, ErrEmailTaken) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "registration failed")
		return
	}

	response.Created(c, "registration successful — check your email to verify your account", gin.H{
		"id":            user.ID,
		"email":         user.Email,
		"matric_number": user.MatricNumber,
	})
}

// ForgotPassword godoc
// POST /auth/forgot-password
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// We don't expose whether the email exists — always return 200
	_ = h.svc.ForgotPassword(req.Email)

	response.OK(c, "if that email exists, a reset link has been sent", nil)
}

// ResetPassword godoc
// POST /auth/reset-password
type resetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.svc.ResetPassword(req.Token, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "password reset failed")
		return
	}

	response.OK(c, "password reset successful", nil)
}

// AcceptInvite godoc
// POST /auth/accept-invite
type acceptInviteRequest struct {
	Token     string `json:"token" binding:"required"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Password  string `json:"password" binding:"required,min=8"`
}

func (h *Handler) AcceptInvite(c *gin.Context) {
	var req acceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.svc.AcceptInvite(req.Token, req.FirstName, req.LastName, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "invite acceptance failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "account activated successfully — you can now log in",
		"data": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}
