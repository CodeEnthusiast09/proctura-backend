package auth_test

import (
	"testing"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/auth"
	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func testCfg() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			Expiration: 24 * time.Hour,
		},
		App: config.AppConfig{
			FrontendURL: "http://localhost:3000",
		},
	}
}


func TestLogin_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := models.User{
		Email:        "lecturer@test.com",
		PasswordHash: string(hash),
		Role:         models.RoleLecturer,
		FirstName:    "Test",
		LastName:     "User",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&user).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	token, returned, err := svc.Login("lecturer@test.com", "password123")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, user.Email, returned.Email)
}

func TestLogin_WrongPassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.DefaultCost)
	user := models.User{
		Email:        "user@test.com",
		PasswordHash: string(hash),
		Role:         models.RoleLecturer,
		FirstName:    "Test",
		LastName:     "User",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&user).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	_, _, err := svc.Login("user@test.com", "wrong")

	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLogin_InactiveAccount(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := models.User{
		Email:        "inactive@test.com",
		PasswordHash: string(hash),
		Role:         models.RoleLecturer,
		FirstName:    "Test",
		LastName:     "User",
		IsActive:     true,
		IsVerified:   true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&user).Update("is_active", false).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	_, _, err := svc.Login("inactive@test.com", "password123")

	assert.ErrorIs(t, err, auth.ErrAccountInactive)
}

func TestRegisterStudent_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	tenant := models.Tenant{Name: "Unilag", Subdomain: "unilag", IsActive: true}
	require.NoError(t, db.Create(&tenant).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	user, err := svc.RegisterStudent("unilag", "student@unilag.edu", "pass123", "John", "Doe", "CSC/2020/001")

	require.NoError(t, err)
	assert.Equal(t, models.RoleStudent, user.Role)
	assert.Equal(t, "John", user.FirstName)
}

func TestRegisterStudent_InvalidTenant(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	_, err := svc.RegisterStudent("nonexistent", "s@test.com", "pass", "A", "B", "MAT/001")

	assert.ErrorIs(t, err, auth.ErrTenantNotFound)
}

func TestForgotPassword_SilentOnMissingEmail(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	err := svc.ForgotPassword("ghost@nobody.com")

	require.NoError(t, err)
}

func TestResetPassword_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("old"), bcrypt.DefaultCost)
	expiry := time.Now().Add(1 * time.Hour)
	resetToken := "valid-reset-token"
	user := models.User{
		Email:            "reset@test.com",
		PasswordHash:     string(hash),
		Role:             models.RoleLecturer,
		FirstName:        "Reset",
		LastName:         "Me",
		IsActive:         true,
		IsVerified:       true,
		ResetToken:       &resetToken,
		ResetTokenExpiry: &expiry,
	}
	require.NoError(t, db.Create(&user).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	err := svc.ResetPassword(resetToken, "newpassword123")
	require.NoError(t, err)

	_, _, loginErr := svc.Login("reset@test.com", "newpassword123")
	assert.NoError(t, loginErr)
}

func TestAcceptInvite_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("temp"), bcrypt.DefaultCost)
	token := "invite-abc123"
	user := models.User{
		Email:        "invited@test.com",
		PasswordHash: string(hash),
		Role:         models.RoleLecturer,
		FirstName:    "",
		LastName:     "",
		IsActive:     true,
		IsVerified:   false,
		InviteToken:  &token,
	}
	require.NoError(t, db.Create(&user).Error)

	svc := auth.NewService(db, testCfg(), &mailer.NoOpMailer{})
	updated, err := svc.AcceptInvite(token, "Jane", "Smith", "securepass")

	require.NoError(t, err)
	assert.True(t, updated.IsVerified)
	assert.Equal(t, "Jane", updated.FirstName)
}
