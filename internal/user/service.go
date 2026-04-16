package user

import (
	"crypto/rand"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email already in use")
)

type Service struct {
	db          *gorm.DB
	mailer      mailer.Mailer
	frontendURL string
}

func NewService(db *gorm.DB, m mailer.Mailer, frontendURL string) *Service {
	return &Service{db: db, mailer: m, frontendURL: frontendURL}
}

// InviteLecturer creates an unverified lecturer account with an invite token.
func (s *Service) InviteLecturer(tenantID, email, firstName, lastName string) (*models.User, string, error) {
	var existing models.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, "", ErrEmailTaken
	}

	tempHash, err := bcrypt.GenerateFromPassword([]byte("invite-pending"), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash temp password: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate invite token: %w", err)
	}

	user := models.User{
		TenantID:     &tenantID,
		Email:        email,
		PasswordHash: string(tempHash),
		Role:         models.RoleLecturer,
		FirstName:    firstName,
		LastName:     lastName,
		IsActive:     true,
		IsVerified:   false,
		InviteToken:  &token,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, "", fmt.Errorf("create lecturer: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", s.frontendURL, token)
	if err := s.mailer.SendInvite(user.Email, user.FirstName, inviteLink); err != nil {
		fmt.Printf("[mailer] failed to send invite to %s: %v\n", user.Email, err)
	}

	return &user, token, nil
}

// ImportStudents parses a CSV and bulk-creates student accounts.
// Expected CSV columns: email, first_name, last_name, matric_number
func (s *Service) ImportStudents(tenantID string, r io.Reader) (int, []string, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, nil, fmt.Errorf("parse csv: %w", err)
	}

	if len(records) < 2 {
		return 0, nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	tempHash, err := bcrypt.GenerateFromPassword([]byte("student-pending"), bcrypt.DefaultCost)
	if err != nil {
		return 0, nil, fmt.Errorf("hash temp password: %w", err)
	}

	created := 0
	var skipped []string

	// Skip header row
	for i, row := range records[1:] {
		if len(row) < 4 {
			skipped = append(skipped, fmt.Sprintf("row %d: not enough columns", i+2))
			continue
		}

		email := strings.TrimSpace(row[0])
		firstName := strings.TrimSpace(row[1])
		lastName := strings.TrimSpace(row[2])
		matricNumber := strings.TrimSpace(row[3])

		if email == "" || matricNumber == "" {
			skipped = append(skipped, fmt.Sprintf("row %d: email and matric_number are required", i+2))
			continue
		}

		var existing models.User
		if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
			skipped = append(skipped, fmt.Sprintf("row %d: email %s already exists", i+2, email))
			continue
		}

		token, _ := generateToken()
		user := models.User{
			TenantID:     &tenantID,
			Email:        email,
			PasswordHash: string(tempHash),
			Role:         models.RoleStudent,
			FirstName:    firstName,
			LastName:     lastName,
			MatricNumber: &matricNumber,
			IsActive:     true,
			IsVerified:   false,
			InviteToken:  &token,
		}

		if err := s.db.Create(&user).Error; err != nil {
			skipped = append(skipped, fmt.Sprintf("row %d: failed to create user", i+2))
			continue
		}

		created++
	}

	return created, skipped, nil
}

// List returns paginated users for a tenant, optionally filtered by role.
func (s *Service) List(tenantID, role string, page, limit int) ([]models.User, int64, error) {
	query := s.db.Where("tenant_id = ?", tenantID)
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	if err := query.Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var users []models.User
	offset := (page - 1) * limit
	if err := query.Omit("password_hash").Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// Update updates a user's active status or role within a tenant.
func (s *Service) Update(tenantID, userID string, isActive *bool) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ? AND tenant_id = ?", userID, tenantID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if isActive != nil {
		if err := s.db.Model(&user).Update("is_active", *isActive).Error; err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	return &user, nil
}

// Delete removes a user from a tenant.
func (s *Service) Delete(tenantID, userID string) error {
	result := s.db.Where("id = ? AND tenant_id = ?", userID, tenantID).Delete(&models.User{})
	if result.Error != nil {
		return fmt.Errorf("delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Me returns the currently authenticated user's profile.
func (s *Service) Me(userID string) (*models.User, error) {
	var user models.User
	if err := s.db.Omit("password_hash").First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
