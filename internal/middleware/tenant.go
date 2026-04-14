package middleware

import (
	"strings"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ResolveTenant extracts the subdomain from the Host header and loads the tenant.
// Expects requests from <subdomain>.proctura.com or X-Tenant-Subdomain header (for local dev).
func ResolveTenant(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		subdomain := c.GetHeader("X-Tenant-Subdomain")

		if subdomain == "" {
			host := c.Request.Host
			// strip port if present
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				host = host[:idx]
			}
			// extract subdomain: unilag.proctura.com → unilag
			parts := strings.SplitN(host, ".", 2)
			if len(parts) >= 2 {
				subdomain = parts[0]
			}
		}

		if subdomain == "" || subdomain == "www" || subdomain == "app" {
			response.BadRequest(c, "tenant subdomain is required")
			c.Abort()
			return
		}

		var tenant models.Tenant
		if err := db.Where("subdomain = ? AND is_active = true", subdomain).First(&tenant).Error; err != nil {
			response.NotFound(c, "school not found or inactive")
			c.Abort()
			return
		}

		c.Set("tenant", &tenant)
		c.Set("tenantID", tenant.ID)
		c.Next()
	}
}

// TenantFromContext is a helper to retrieve the resolved tenant in handlers.
func TenantFromContext(c *gin.Context) *models.Tenant {
	if t, exists := c.Get("tenant"); exists {
		if tenant, ok := t.(*models.Tenant); ok {
			return tenant
		}
	}
	return nil
}
