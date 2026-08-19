package middleware

import (
	"errors"
	"slices"
	"strings"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Authenticate validates the JWT, confirms the account is still usable, and
// sets user context values.
//
// A JWT is self-contained, so nothing that happens after it is issued shows up
// in it. Trusting the claims alone meant a user who had been deactivated or
// deleted kept full access until their token expired, which is JWT_EXPIRATION,
// 24h by default. Loading the row on every request makes revocation immediate.
// The tenant-scoped groups already pay for a middleware query in ResolveTenant,
// so this is a second indexed lookup next to one that was there anyway.
func Authenticate(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := c.GetHeader("Authorization")
		if bearer == "" {
			response.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}

		tokenStr, ok := strings.CutPrefix(bearer, "Bearer ")
		if !ok {
			response.Unauthorized(c, "invalid authorization header format")
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, "id = ?", claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// users has no DeletedAt, so Delete is a hard delete and a
				// missing row means the account is genuinely gone.
				response.Unauthorized(c, "account no longer exists")
			} else {
				response.InternalError(c, "failed to verify account")
			}
			c.Abort()
			return
		}

		if !user.IsActive {
			response.Unauthorized(c, "account has been deactivated")
			c.Abort()
			return
		}

		c.Set("userID", user.ID)
		// tenantID deliberately still comes from the claims. On the tenant
		// groups ResolveTenant runs first and sets it from the subdomain, and
		// this overwrite is what stops a user of one school acting inside
		// another one. Keep that behaviour exactly as it was.
		c.Set("tenantID", claims.TenantID)
		c.Set("email", user.Email)
		c.Set("role", string(user.Role))
		c.Next()
	}
}

// RequireRole aborts the request if the authenticated user's role is not in the allowed list.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("role")

		if slices.Contains(roles, userRole) {
			c.Next()
			return
		}

		response.Forbidden(c, "you do not have permission to perform this action")
		c.Abort()
	}
}
