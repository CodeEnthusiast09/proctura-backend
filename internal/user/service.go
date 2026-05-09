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
	ErrUserNotFound        = errors.New("user not found")
	ErrEmailTaken          = errors.New("email already in use")
	ErrAdminLimitReached   = errors.New("this school already has the maximum of 2 active admins — deactivate one first")
	ErrLastAdminProtected  = errors.New("cannot deactivate or remove the last active school admin")
	ErrCurrentPasswordWrong = errors.New("current password is incorrect")
)

const maxAdminsPerTenant = 2

type Service struct {
	db          *gorm.DB
	mailer      mailer.Mailer
	frontendURL string
}

func NewService(db *gorm.DB, m mailer.Mailer, frontendURL string) *Service {
	return &Service{db: db, mailer: m, frontendURL: frontendURL}
}

// InviteAdmin creates an unverified school_admin account, capped at 2 active
// admins per tenant. Used both for school_admin co-admin invites and super_admin
// recovery invites.
func (s *Service) InviteAdmin(tenantID, email, firstName, lastName string) (*models.User, string, error) {
	var activeAdmins int64
	if err := s.db.Model(&models.User{}).
		Where("tenant_id = ? AND role = ? AND is_active = true", tenantID, models.RoleSchoolAdmin).
		Count(&activeAdmins).Error; err != nil {
		return nil, "", fmt.Errorf("count admins: %w", err)
	}
	if activeAdmins >= maxAdminsPerTenant {
		return nil, "", ErrAdminLimitReached
	}

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
		Role:         models.RoleSchoolAdmin,
		FirstName:    firstName,
		LastName:     lastName,
		IsActive:     true,
		IsVerified:   false,
		InviteToken:  &token,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, "", fmt.Errorf("create admin: %w", err)
	}

	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", s.frontendURL, token)
	if err := s.mailer.SendInvite(user.Email, user.FirstName, inviteLink); err != nil {
		fmt.Printf("[mailer] failed to send invite to %s: %v\n", user.Email, err)
	}

	return &user, token, nil
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

// InviteStudent creates a single student account with an invite token.
func (s *Service) InviteStudent(tenantID, email, firstName, lastName, matricNumber string) (*models.User, string, error) {
	var existing models.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil, "", ErrEmailTaken
	}

	tempHash, err := bcrypt.GenerateFromPassword([]byte("student-pending"), bcrypt.DefaultCost)
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
		Role:         models.RoleStudent,
		FirstName:    firstName,
		LastName:     lastName,
		MatricNumber: &matricNumber,
		IsActive:     true,
		IsVerified:   false,
		InviteToken:  &token,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, "", fmt.Errorf("create student: %w", err)
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

		inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", s.frontendURL, token)
		if err := s.mailer.SendInvite(user.Email, user.FirstName, inviteLink); err != nil {
			fmt.Printf("[mailer] failed to send invite to %s: %v\n", user.Email, err)
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

// Update updates a user's active status within a tenant.
// Blocks deactivating the last active school_admin in the tenant.
func (s *Service) Update(tenantID, userID string, isActive *bool) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ? AND tenant_id = ?", userID, tenantID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if isActive != nil {
		if !*isActive && user.Role == models.RoleSchoolAdmin && user.IsActive {
			if err := s.ensureNotLastAdmin(tenantID, user.ID); err != nil {
				return nil, err
			}
		}
		if err := s.db.Model(&user).Update("is_active", *isActive).Error; err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	return &user, nil
}

// Delete removes a user from a tenant.
// Blocks removing the last active school_admin in the tenant.
func (s *Service) Delete(tenantID, userID string) error {
	var user models.User
	if err := s.db.Where("id = ? AND tenant_id = ?", userID, tenantID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user: %w", err)
	}

	if user.Role == models.RoleSchoolAdmin && user.IsActive {
		if err := s.ensureNotLastAdmin(tenantID, user.ID); err != nil {
			return err
		}
	}

	if err := s.db.Delete(&user).Error; err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// BulkUpdateActive sets is_active on every user in the tenant whose ID is
// in the list. All-or-nothing: if any pending change would leave the tenant
// with zero active school_admins, the whole batch is rejected and nothing
// is written. Empty IDs are silently ignored.
func (s *Service) BulkUpdateActive(tenantID string, userIDs []string, isActive bool) error {
	if len(userIDs) == 0 {
		return nil
	}

	var users []models.User
	if err := s.db.Where("tenant_id = ? AND id IN ?", tenantID, userIDs).
		Find(&users).Error; err != nil {
		return fmt.Errorf("load users: %w", err)
	}
	if len(users) == 0 {
		return ErrUserNotFound
	}

	// Last-admin protection only matters when deactivating: count how many
	// currently-active school_admins are NOT in this batch. If the batch
	// would deactivate every active admin and none would remain, reject.
	if !isActive {
		var remaining int64
		if err := s.db.Model(&models.User{}).
			Where("tenant_id = ? AND role = ? AND is_active = true AND id NOT IN ?",
				tenantID, models.RoleSchoolAdmin, userIDs).
			Count(&remaining).Error; err != nil {
			return fmt.Errorf("count remaining admins: %w", err)
		}
		// If there were any active admins in the batch and none survive
		// outside it, this would lock everyone out.
		batchHasActiveAdmin := false
		for _, u := range users {
			if u.Role == models.RoleSchoolAdmin && u.IsActive {
				batchHasActiveAdmin = true
				break
			}
		}
		if batchHasActiveAdmin && remaining == 0 {
			return ErrLastAdminProtected
		}
	}

	if err := s.db.Model(&models.User{}).
		Where("tenant_id = ? AND id IN ?", tenantID, userIDs).
		Update("is_active", isActive).Error; err != nil {
		return fmt.Errorf("bulk update is_active: %w", err)
	}
	return nil
}

// ensureNotLastAdmin returns ErrLastAdminProtected if removing/deactivating
// the given admin would leave the tenant with zero active school_admins.
func (s *Service) ensureNotLastAdmin(tenantID, excludeUserID string) error {
	var count int64
	if err := s.db.Model(&models.User{}).
		Where("tenant_id = ? AND role = ? AND is_active = true AND id <> ?",
			tenantID, models.RoleSchoolAdmin, excludeUserID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count remaining admins: %w", err)
	}
	if count == 0 {
		return ErrLastAdminProtected
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

// UpdateMe updates the authenticated user's first/last name.
func (s *Service) UpdateMe(userID, firstName, lastName string) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	updates := map[string]any{}
	if firstName != "" {
		updates["first_name"] = firstName
	}
	if lastName != "" {
		updates["last_name"] = lastName
	}
	if len(updates) == 0 {
		return &user, nil
	}

	if err := s.db.Model(&user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	user.PasswordHash = ""
	return &user, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *Service) ChangePassword(userID, currentPassword, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordWrong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		return fmt.Errorf("update password: %w", err)
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
