package tenant

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrTenantNotFound    = errors.New("tenant not found")
	ErrSubdomainTaken    = errors.New("subdomain already in use")
	ErrAdminEmailTaken   = errors.New("admin email already in use")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type CreateTenantInput struct {
	Name           string
	Subdomain      string
	AdminEmail     string
	AdminFirstName string
	AdminLastName  string
}

// Create onboards a new school and creates its first school_admin user.
func (s *Service) Create(input CreateTenantInput) (*models.Tenant, *models.User, error) {
	var existing models.Tenant
	if err := s.db.Where("subdomain = ?", input.Subdomain).First(&existing).Error; err == nil {
		return nil, nil, ErrSubdomainTaken
	}

	var existingUser models.User
	if err := s.db.Where("email = ?", input.AdminEmail).First(&existingUser).Error; err == nil {
		return nil, nil, ErrAdminEmailTaken
	}

	tenant := models.Tenant{
		Name:      input.Name,
		Subdomain: input.Subdomain,
		IsActive:  true,
	}

	if err := s.db.Create(&tenant).Error; err != nil {
		return nil, nil, fmt.Errorf("create tenant: %w", err)
	}

	// Generate a temporary password — user must reset via invite link
	tempHash, err := bcrypt.GenerateFromPassword([]byte("change-me"), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash temp password: %w", err)
	}

	inviteToken, err := generateToken()
	if err != nil {
		return nil, nil, fmt.Errorf("generate invite token: %w", err)
	}

	admin := models.User{
		TenantID:     &tenant.ID,
		Email:        input.AdminEmail,
		PasswordHash: string(tempHash),
		Role:         models.RoleSchoolAdmin,
		FirstName:    input.AdminFirstName,
		LastName:     input.AdminLastName,
		IsActive:     true,
		IsVerified:   false,
		InviteToken:  &inviteToken,
	}

	if err := s.db.Create(&admin).Error; err != nil {
		return nil, nil, fmt.Errorf("create admin user: %w", err)
	}

	return &tenant, &admin, nil
}

func (s *Service) List(page, limit int) ([]models.Tenant, int64, error) {
	var tenants []models.Tenant
	var total int64

	if err := s.db.Model(&models.Tenant{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tenants: %w", err)
	}

	offset := (page - 1) * limit
	if err := s.db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tenants).Error; err != nil {
		return nil, 0, fmt.Errorf("list tenants: %w", err)
	}

	return tenants, total, nil
}

func (s *Service) Update(id string, name string, isActive *bool) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := s.db.First(&tenant, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	updates := map[string]any{}
	if name != "" {
		updates["name"] = name
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}

	if err := s.db.Model(&tenant).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update tenant: %w", err)
	}

	return &tenant, nil
}

func (s *Service) Delete(id string) error {
	result := s.db.Delete(&models.Tenant{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete tenant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTenantNotFound
	}
	return nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
