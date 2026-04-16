package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/CodeEnthusiast09/proctura-backend/internal/config"
	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrAccountUnverified  = errors.New("account is not verified — check your email")
	ErrEmailTaken         = errors.New("email already in use")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTenantNotFound     = errors.New("school not found")
)

type Service struct {
	db     *gorm.DB
	cfg    *config.Config
	mailer mailer.Mailer
}

func NewService(db *gorm.DB, cfg *config.Config, m mailer.Mailer) *Service {
	return &Service{db: db, cfg: cfg, mailer: m}
}

// Login validates credentials and returns a signed JWT.
func (s *Service) Login(email, password string) (string, *models.User, error) {
	var user models.User

	if err := s.db.Preload("Tenant").Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, fmt.Errorf("get user: %w", err)
	}

	if !user.IsActive {
		return "", nil, ErrAccountInactive
	}

	if !user.IsVerified && user.Role != models.RoleSuperAdmin {
		return "", nil, ErrAccountUnverified
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	tenantID := ""
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}

	token, err := GenerateToken(user.ID, tenantID, user.Email, string(user.Role), s.cfg.JWT.Secret, s.cfg.JWT.Expiration)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}

	return token, &user, nil
}

// RegisterStudent allows a student to self-register using a school subdomain + matric number.
func (s *Service) RegisterStudent(subdomain, email, password, firstName, lastName, matricNumber string) (*models.User, error) {
	var tenant models.Tenant
	if err := s.db.Where("subdomain = ? AND is_active = true", subdomain).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	var existing models.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		TenantID:     &tenant.ID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.RoleStudent,
		FirstName:    firstName,
		LastName:     lastName,
		MatricNumber: &matricNumber,
		IsActive:     true,
		IsVerified:   false,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

// ForgotPassword generates a reset token and emails the reset link.
func (s *Service) ForgotPassword(email string) error {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Silently succeed — don't reveal whether email exists
			return nil
		}
		return fmt.Errorf("get user: %w", err)
	}

	token, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	expiry := time.Now().Add(1 * time.Hour)
	if err := s.db.Model(&user).Updates(map[string]any{
		"reset_token":        token,
		"reset_token_expiry": expiry,
	}).Error; err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.App.FrontendURL, token)
	if err := s.mailer.SendPasswordReset(user.Email, user.FirstName, resetLink); err != nil {
		fmt.Printf("[mailer] failed to send reset email to %s: %v\n", user.Email, err)
	}

	return nil
}

// ResetPassword validates the token and updates the password.
func (s *Service) ResetPassword(token, newPassword string) error {
	var user models.User
	if err := s.db.Where("reset_token = ?", token).First(&user).Error; err != nil {
		return ErrInvalidToken
	}

	if user.ResetTokenExpiry == nil || time.Now().After(*user.ResetTokenExpiry) {
		return ErrInvalidToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&user).Updates(map[string]any{
		"password_hash":      string(hash),
		"reset_token":        nil,
		"reset_token_expiry": nil,
	}).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}

// AcceptInvite sets name + password for an invited lecturer/school_admin and marks them verified.
func (s *Service) AcceptInvite(token, firstName, lastName, password string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("invite_token = ?", token).First(&user).Error; err != nil {
		return nil, ErrInvalidToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&user).Updates(map[string]any{
		"first_name":    firstName,
		"last_name":     lastName,
		"password_hash": string(hash),
		"is_verified":   true,
		"invite_token":  nil,
	}).Error; err != nil {
		return nil, fmt.Errorf("accept invite: %w", err)
	}

	return &user, nil
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
