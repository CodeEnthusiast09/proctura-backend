package course

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"gorm.io/gorm"
)

var (
	ErrCourseNotFound      = errors.New("course not found")
	ErrNotCourseOwner      = errors.New("you can only manage enrollments for your own courses")
	ErrEnrollmentNotFound  = errors.New("enrollment not found")
)

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

func (s *Service) List(tenantID, lecturerID, studentID string, page, limit int) ([]models.Course, int64, error) {
	query := s.db.Where("courses.tenant_id = ?", tenantID)

	if lecturerID != "" {
		query = query.Where("courses.lecturer_id = ?", lecturerID)
	} else if studentID != "" {
		query = query.Joins("JOIN course_enrollments ON course_enrollments.course_id = courses.id AND course_enrollments.student_id = ?", studentID)
	}

	var total int64
	if err := query.Model(&models.Course{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count courses: %w", err)
	}

	var courses []models.Course
	offset := (page - 1) * limit
	if err := query.Preload("Lecturer").Order("courses.created_at DESC").Offset(offset).Limit(limit).Find(&courses).Error; err != nil {
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

// Enroll adds students to a course by matric number. Returns the count of newly enrolled students.
func (s *Service) Enroll(tenantID, courseID, requesterID, requesterRole string, matricNumbers []string) (int, error) {
	var course models.Course
	if err := s.db.Where("id = ? AND tenant_id = ?", courseID, tenantID).First(&course).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrCourseNotFound
		}
		return 0, fmt.Errorf("get course: %w", err)
	}
	if requesterRole == string(models.RoleLecturer) && course.LecturerID != requesterID {
		return 0, ErrNotCourseOwner
	}

	enrolled := 0
	for _, matric := range matricNumbers {
		matric = strings.TrimSpace(matric)
		if matric == "" {
			continue
		}
		var student models.User
		if err := s.db.Where("matric_number = ? AND tenant_id = ? AND role = ?", matric, tenantID, models.RoleStudent).
			First(&student).Error; err != nil {
			continue // skip unknown matric numbers
		}
		e := models.CourseEnrollment{TenantID: tenantID, CourseID: courseID, StudentID: student.ID}
		result := s.db.Where("course_id = ? AND student_id = ?", courseID, student.ID).FirstOrCreate(&e)
		if result.Error != nil {
			continue
		}
		if result.RowsAffected > 0 {
			enrolled++
		}
	}
	return enrolled, nil
}

// Unenroll removes a student from a course.
func (s *Service) Unenroll(tenantID, courseID, studentID, requesterID, requesterRole string) error {
	var course models.Course
	if err := s.db.Where("id = ? AND tenant_id = ?", courseID, tenantID).First(&course).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCourseNotFound
		}
		return fmt.Errorf("get course: %w", err)
	}
	if requesterRole == string(models.RoleLecturer) && course.LecturerID != requesterID {
		return ErrNotCourseOwner
	}

	result := s.db.Where("course_id = ? AND student_id = ? AND tenant_id = ?", courseID, studentID, tenantID).
		Delete(&models.CourseEnrollment{})
	if result.Error != nil {
		return fmt.Errorf("delete enrollment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrEnrollmentNotFound
	}
	return nil
}

// ListEnrollments returns all students enrolled in a course.
func (s *Service) ListEnrollments(tenantID, courseID, requesterID, requesterRole string) ([]models.CourseEnrollment, error) {
	var course models.Course
	if err := s.db.Where("id = ? AND tenant_id = ?", courseID, tenantID).First(&course).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}
	if requesterRole == string(models.RoleLecturer) && course.LecturerID != requesterID {
		return nil, ErrNotCourseOwner
	}

	var enrollments []models.CourseEnrollment
	if err := s.db.Where("course_id = ? AND tenant_id = ?", courseID, tenantID).
		Preload("Student").Find(&enrollments).Error; err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	return enrollments, nil
}
