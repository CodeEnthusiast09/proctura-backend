package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/middleware"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testSecret = "test-secret"

// newAuthRouter builds a router with Authenticate in front of a handler that
// echoes back the context values the middleware set, so tests can assert both
// the status code and what downstream handlers would see.
func newAuthRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.Authenticate(db, testSecret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"userID":   c.GetString("userID"),
			"tenantID": c.GetString("tenantID"),
			"email":    c.GetString("email"),
			"role":     c.GetString("role"),
		})
	})
	return r
}

func newActiveUser(t *testing.T, db *gorm.DB, role models.UserRole) *models.User {
	t.Helper()

	user := models.User{
		Email:      "revocation@test.com",
		Role:       role,
		FirstName:  "Test",
		LastName:   "User",
		IsActive:   true,
		IsVerified: true,
	}
	require.NoError(t, db.Create(&user).Error)
	return &user
}

func tokenFor(t *testing.T, user *models.User) string {
	t.Helper()

	tenantID := ""
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}
	token, err := auth.GenerateToken(
		user.ID, tenantID, user.Email, string(user.Role), testSecret, time.Hour,
	)
	require.NoError(t, err)
	return token
}

func TestAuthenticate(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	tests := []struct {
		name string
		// header builds the Authorization header, and may mutate the user row
		// first to simulate what an admin did after the token was issued.
		header     func(t *testing.T, user *models.User) string
		wantStatus int
		wantBody   string
	}{
		{
			name: "active user is allowed through",
			header: func(t *testing.T, user *models.User) string {
				return "Bearer " + tokenFor(t, user)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "deactivated user is rejected even with a valid token",
			header: func(t *testing.T, user *models.User) string {
				token := tokenFor(t, user)
				require.NoError(t, db.Model(user).Update("is_active", false).Error)
				return "Bearer " + token
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "account has been deactivated",
		},
		{
			name: "deleted user is rejected even with a valid token",
			header: func(t *testing.T, user *models.User) string {
				token := tokenFor(t, user)
				require.NoError(t, db.Delete(user).Error)
				return "Bearer " + token
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "account no longer exists",
		},
		{
			name:       "missing header is rejected",
			header:     func(t *testing.T, user *models.User) string { return "" },
			wantStatus: http.StatusUnauthorized,
			wantBody:   "authentication required",
		},
		{
			name: "header without the Bearer prefix is rejected",
			header: func(t *testing.T, user *models.User) string {
				return tokenFor(t, user)
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid authorization header format",
		},
		{
			name:       "garbage token is rejected",
			header:     func(t *testing.T, user *models.User) string { return "Bearer not-a-jwt" },
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid or expired token",
		},
		{
			name: "token signed with the wrong secret is rejected",
			header: func(t *testing.T, user *models.User) string {
				token, err := auth.GenerateToken(
					user.ID, "", user.Email, string(user.Role), "wrong-secret", time.Hour,
				)
				require.NoError(t, err)
				return "Bearer " + token
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid or expired token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer testutil.CleanupTables(t, db)

			user := newActiveUser(t, db, models.RoleLecturer)
			router := newAuthRouter(t, db)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if header := tt.header(t, user); header != "" {
				req.Header.Set("Authorization", header)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Contains(t, rec.Body.String(), tt.wantBody)
			}
		})
	}
}

// The middleware now sources role from the row rather than the claims, so a
// token minted before a change cannot be used to assert the old value.
func TestAuthenticate_RoleComesFromTheDatabaseNotTheToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	user := newActiveUser(t, db, models.RoleLecturer)

	// Mint a token that claims school_admin for a row that says lecturer.
	token, err := auth.GenerateToken(
		user.ID, "", user.Email, string(models.RoleSchoolAdmin), testSecret, time.Hour,
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	newAuthRouter(t, db).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"role":"lecturer"`)
	assert.NotContains(t, rec.Body.String(), "school_admin")
}
