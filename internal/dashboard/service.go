package dashboard

import (
	"fmt"

	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ── Lecturer ──────────────────────────────────────────────────────────────────

type LecturerSummary struct {
	TotalCourses      int64   `json:"total_courses"`
	TotalExams        int64   `json:"total_exams"`
	ActiveExams       int64   `json:"active_exams"`
	EnrolledStudents  int64   `json:"enrolled_students"`
	GradedSubmissions int64   `json:"graded_submissions"`
	AverageScorePct   float64 `json:"average_score_pct"`
}

type LecturerDashboard struct {
	Summary             LecturerSummary  `json:"summary"`
	ExamStatusBreakdown map[string]int64 `json:"exam_status_breakdown"`
}

func (s *Service) GetLecturerDashboard(tenantID, lecturerID string) (*LecturerDashboard, error) {
	var totalCourses int64
	if err := s.db.Raw(
		`SELECT COUNT(*) FROM courses WHERE tenant_id = ? AND lecturer_id = ?`,
		tenantID, lecturerID,
	).Scan(&totalCourses).Error; err != nil {
		return nil, fmt.Errorf("count courses: %w", err)
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var statusRows []statusRow
	if err := s.db.Raw(`
		SELECT e.status, COUNT(*) as count
		FROM exams e
		JOIN courses c ON c.id = e.course_id
		WHERE e.tenant_id = ? AND c.lecturer_id = ?
		GROUP BY e.status
	`, tenantID, lecturerID).Scan(&statusRows).Error; err != nil {
		return nil, fmt.Errorf("exam status breakdown: %w", err)
	}

	breakdown := map[string]int64{"draft": 0, "scheduled": 0, "active": 0, "closed": 0}
	var totalExams, activeExams int64
	for _, r := range statusRows {
		breakdown[r.Status] = r.Count
		totalExams += r.Count
		if r.Status == "active" {
			activeExams = r.Count
		}
	}

	var enrolledStudents int64
	if err := s.db.Raw(`
		SELECT COUNT(DISTINCT ce.student_id)
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		WHERE c.tenant_id = ? AND c.lecturer_id = ?
	`, tenantID, lecturerID).Scan(&enrolledStudents).Error; err != nil {
		return nil, fmt.Errorf("count enrolled students: %w", err)
	}

	type submissionStats struct {
		GradedCount int64
		AvgScorePct float64
	}
	var stats submissionStats
	if err := s.db.Raw(`
		SELECT
			COUNT(*) as graded_count,
			COALESCE(AVG(
				CASE WHEN s.max_score > 0
				THEN s.total_score::float / s.max_score * 100
				ELSE 0 END
			), 0) as avg_score_pct
		FROM submissions s
		JOIN exams e ON e.id = s.exam_id
		JOIN courses c ON c.id = e.course_id
		WHERE s.tenant_id = ? AND c.lecturer_id = ? AND s.status = 'graded'
	`, tenantID, lecturerID).Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("submission stats: %w", err)
	}

	return &LecturerDashboard{
		Summary: LecturerSummary{
			TotalCourses:      totalCourses,
			TotalExams:        totalExams,
			ActiveExams:       activeExams,
			EnrolledStudents:  enrolledStudents,
			GradedSubmissions: stats.GradedCount,
			AverageScorePct:   stats.AvgScorePct,
		},
		ExamStatusBreakdown: breakdown,
	}, nil
}

// ── School Admin ──────────────────────────────────────────────────────────────

type SchoolAdminSummary struct {
	TotalLecturers int64 `json:"total_lecturers"`
	TotalStudents  int64 `json:"total_students"`
	ActiveStudents int64 `json:"active_students"`
	TotalCourses   int64 `json:"total_courses"`
	ActiveExams    int64 `json:"active_exams"`
}

type SchoolPerformance struct {
	GradedSubmissions int64   `json:"graded_submissions"`
	AverageScorePct   float64 `json:"average_score_pct"`
	PassRate          float64 `json:"pass_rate"`
}

type SchoolAdminDashboard struct {
	Summary             SchoolAdminSummary `json:"summary"`
	ExamStatusBreakdown map[string]int64   `json:"exam_status_breakdown"`
	SchoolPerformance   SchoolPerformance  `json:"school_performance"`
}

func (s *Service) GetSchoolAdminDashboard(tenantID string) (*SchoolAdminDashboard, error) {
	type roleRow struct {
		Role        string
		Total       int64
		ActiveCount int64
	}
	var roleRows []roleRow
	if err := s.db.Raw(`
		SELECT role,
			COUNT(*) as total,
			SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active_count
		FROM users
		WHERE tenant_id = ? AND role IN ('lecturer', 'student')
		GROUP BY role
	`, tenantID).Scan(&roleRows).Error; err != nil {
		return nil, fmt.Errorf("user counts: %w", err)
	}

	var totalLecturers, totalStudents, activeStudents int64
	for _, r := range roleRows {
		switch r.Role {
		case "lecturer":
			totalLecturers = r.Total
		case "student":
			totalStudents = r.Total
			activeStudents = r.ActiveCount
		}
	}

	var totalCourses int64
	if err := s.db.Raw(
		`SELECT COUNT(*) FROM courses WHERE tenant_id = ?`, tenantID,
	).Scan(&totalCourses).Error; err != nil {
		return nil, fmt.Errorf("count courses: %w", err)
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var statusRows []statusRow
	if err := s.db.Raw(`
		SELECT status, COUNT(*) as count
		FROM exams
		WHERE tenant_id = ?
		GROUP BY status
	`, tenantID).Scan(&statusRows).Error; err != nil {
		return nil, fmt.Errorf("exam status breakdown: %w", err)
	}

	breakdown := map[string]int64{"draft": 0, "scheduled": 0, "active": 0, "closed": 0}
	var activeExams int64
	for _, r := range statusRows {
		breakdown[r.Status] = r.Count
		if r.Status == "active" {
			activeExams = r.Count
		}
	}

	type perfRow struct {
		GradedCount int64
		AvgScorePct float64
		PassRate    float64
	}
	var perf perfRow
	if err := s.db.Raw(`
		SELECT
			COUNT(*) as graded_count,
			COALESCE(AVG(
				CASE WHEN max_score > 0
				THEN total_score::float / max_score * 100
				ELSE 0 END
			), 0) as avg_score_pct,
			COALESCE(
				SUM(CASE WHEN max_score > 0 AND total_score::float / max_score >= 0.5 THEN 1 ELSE 0 END)::float
				/ NULLIF(COUNT(*), 0) * 100
			, 0) as pass_rate
		FROM submissions
		WHERE tenant_id = ? AND status = 'graded'
	`, tenantID).Scan(&perf).Error; err != nil {
		return nil, fmt.Errorf("school performance: %w", err)
	}

	return &SchoolAdminDashboard{
		Summary: SchoolAdminSummary{
			TotalLecturers: totalLecturers,
			TotalStudents:  totalStudents,
			ActiveStudents: activeStudents,
			TotalCourses:   totalCourses,
			ActiveExams:    activeExams,
		},
		ExamStatusBreakdown: breakdown,
		SchoolPerformance: SchoolPerformance{
			GradedSubmissions: perf.GradedCount,
			AverageScorePct:   perf.AvgScorePct,
			PassRate:          perf.PassRate,
		},
	}, nil
}

// ── Super Admin ───────────────────────────────────────────────────────────────

type SuperAdminSummary struct {
	TotalSchools    int64 `json:"total_schools"`
	ActiveSchools   int64 `json:"active_schools"`
	InactiveSchools int64 `json:"inactive_schools"`
}

type GrowthPoint struct {
	Month string `json:"month"`
	Count int64  `json:"count"`
}

type SuperAdminDashboard struct {
	Summary SuperAdminSummary `json:"summary"`
	Growth  []GrowthPoint     `json:"growth"`
}

func (s *Service) GetSuperAdminDashboard() (*SuperAdminDashboard, error) {
	type summaryRow struct {
		Total  int64
		Active int64
	}
	var row summaryRow
	if err := s.db.Raw(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active
		FROM tenants
	`).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("tenant summary: %w", err)
	}

	var growth []GrowthPoint
	if err := s.db.Raw(`
		SELECT
			TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') as month,
			COUNT(*) as count
		FROM tenants
		WHERE created_at >= NOW() - INTERVAL '12 months'
		GROUP BY DATE_TRUNC('month', created_at)
		ORDER BY DATE_TRUNC('month', created_at)
	`).Scan(&growth).Error; err != nil {
		return nil, fmt.Errorf("tenant growth: %w", err)
	}

	if growth == nil {
		growth = []GrowthPoint{}
	}

	return &SuperAdminDashboard{
		Summary: SuperAdminSummary{
			TotalSchools:    row.Total,
			ActiveSchools:   row.Active,
			InactiveSchools: row.Total - row.Active,
		},
		Growth: growth,
	}, nil
}
