package course

import (
	"errors"
	"fmt"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"gorm.io/gorm"
)

var ErrCourseNotFound = errors.New("course not found")

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(tenantID, lecturerID, title, code string) (*models.Course, error) {
	course := models.Course{
		TenantID:   tenantID,
		LecturerID: lecturerID,
		Title:      title,
		Code:       code,
	}
	if err := s.db.Create(&course).Error; err != nil {
		return nil, fmt.Errorf("create course: %w", err)
	}
	return &course, nil
}

func (s *Service) List(tenantID, lecturerID string, page, limit int) ([]models.Course, int64, error) {
	query := s.db.Where("tenant_id = ?", tenantID)
	if lecturerID != "" {
		query = query.Where("lecturer_id = ?", lecturerID)
	}

	var total int64
	if err := query.Model(&models.Course{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count courses: %w", err)
	}

	var courses []models.Course
	offset := (page - 1) * limit
	if err := query.Preload("Lecturer").Order("created_at DESC").Offset(offset).Limit(limit).Find(&courses).Error; err != nil {
		return nil, 0, fmt.Errorf("list courses: %w", err)
	}

	return courses, total, nil
}

func (s *Service) Update(tenantID, courseID, title, code string) (*models.Course, error) {
	var course models.Course
	if err := s.db.Where("id = ? AND tenant_id = ?", courseID, tenantID).First(&course).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}

	updates := map[string]any{}
	if title != "" {
		updates["title"] = title
	}
	if code != "" {
		updates["code"] = code
	}

	if err := s.db.Model(&course).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update course: %w", err)
	}

	return &course, nil
}

func (s *Service) Delete(tenantID, courseID string) error {
	result := s.db.Where("id = ? AND tenant_id = ?", courseID, tenantID).Delete(&models.Course{})
	if result.Error != nil {
		return fmt.Errorf("delete course: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCourseNotFound
	}
	return nil
}
