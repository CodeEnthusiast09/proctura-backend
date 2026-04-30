package middleware

import (
	"slices"
	"strings"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
)

// Authenticate validates the JWT and sets user context values.
func Authenticate(jwtSecret string) gin.HandlerFunc {
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

		c.Set("userID", claims.UserID)
		c.Set("tenantID", claims.TenantID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
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
