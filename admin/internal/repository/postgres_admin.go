package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"clasynq/api/admin/internal/domain"

	"gorm.io/gorm"
)

// postgresAdminRepository implements domain.AdminRepository interface for GORM Postgres.
type postgresAdminRepository struct {
	db *gorm.DB
}

// NewPostgresAdminRepository initializes postgresAdminRepository.
func NewPostgresAdminRepository(db *gorm.DB) domain.AdminRepository {
	return &postgresAdminRepository{db: db}
}

// GetDashboardStats fetches (or builds) the admin overview stats record (ID: 1).
func (r *postgresAdminRepository) GetDashboardStats(ctx context.Context) (*domain.AdminDashboard, error) {
	var stats domain.AdminDashboard
	if err := r.db.WithContext(ctx).FirstOrCreate(&stats, domain.AdminDashboard{ID: 1}).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// RefreshDashboardStats recalculates total student signups, teacher registries,
// and active batch counts, saving them to the admin overview stats record.
func (r *postgresAdminRepository) RefreshDashboardStats(ctx context.Context) (*domain.AdminDashboard, error) {
	var stats domain.AdminDashboard
	if err := r.db.WithContext(ctx).FirstOrCreate(&stats, domain.AdminDashboard{ID: 1}).Error; err != nil {
		return nil, err
	}
	var studentCount int64
	var teacherCount int64
	var activeBatches int64

	// Count students with at least one course enrollment
	r.db.WithContext(ctx).Model(&domain.Student{}).Where(
		"id IN (SELECT student_id FROM enrollments)",
	).Count(&studentCount)
	
	// Count total teacher profiles
	r.db.WithContext(ctx).Model(&domain.Teacher{}).Count(&teacherCount)
	
	// Count non-completed course batches
	r.db.WithContext(ctx).Model(&domain.Course{}).Where("course_status <> ?", "completed").Count(&activeBatches)

	stats.TotalStudents = studentCount
	stats.TotalTeacher = teacherCount
	stats.ActiveBatches = activeBatches
	if err := r.db.WithContext(ctx).Save(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetAdminByID retrieves an administrative profile record using its numeric ID.
func (r *postgresAdminRepository) GetAdminByID(ctx context.Context, id int64) (*domain.Admin, error) {
	var admin domain.Admin
	if err := r.db.WithContext(ctx).First(&admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

// GetAdminByEmail retrieves an admin record by email using a case-insensitive lookup.
func (r *postgresAdminRepository) GetAdminByEmail(ctx context.Context, email string) (*domain.Admin, error) {
	var admin domain.Admin
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

// CreateNotification logs a system notification event for a specific recipient.
func (r *postgresAdminRepository) CreateNotification(ctx context.Context, recipientID int64, recipientRole, notifType, message string) error {
	notif := domain.UserNotification{
		RecipientID:      recipientID,
		RecipientRole:    recipientRole,
		NotificationType: notifType,
		Message:          message,
		IsRead:           false,
		CreatedAt:        time.Now(),
	}
	return r.db.WithContext(ctx).Create(&notif).Error
}

// GetActivities retrieves recent administrative audit logs, preloading admin emails in a batch query to prevent N+1 query loops.
func (r *postgresAdminRepository) GetActivities(ctx context.Context, limit int) ([]domain.AdminActivity, error) {
	var list []domain.AdminActivity
	err := r.db.WithContext(ctx).
		Limit(limit).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return list, nil
	}

	// Fetch admin emails in a single query by mapping administrative IDs.
	adminIDs := make([]int64, 0, len(list))
	seenIDs := make(map[int64]bool)
	for _, act := range list {
		if !seenIDs[act.AdminID] {
			seenIDs[act.AdminID] = true
			adminIDs = append(adminIDs, act.AdminID)
		}
	}

	var admins []domain.Admin
	if err := r.db.WithContext(ctx).Select("id, email").Where("id IN ?", adminIDs).Find(&admins).Error; err == nil {
		emailMap := make(map[int64]string)
		for _, a := range admins {
			emailMap[a.ID] = a.Email
		}
		for i := range list {
			list[i].AdminEmail = emailMap[list[i].AdminID]
		}
	}

	return list, nil
}

// LogActivity writes a new audit entry detailing an admin action.
func (r *postgresAdminRepository) LogActivity(ctx context.Context, adminID int64, action, entityType, entityName string) error {
	activity := domain.AdminActivity{
		AdminID:    adminID,
		Action:     action,
		EntityType: entityType,
		EntityName: entityName,
		CreatedAt:  time.Now(),
	}
	return r.db.WithContext(ctx).Create(&activity).Error
}

// ListTeachers retrieves teacher records filtered by search queries and course category mappings.
func (r *postgresAdminRepository) ListTeachers(ctx context.Context, query, category string) ([]domain.Teacher, error) {
	var list []domain.Teacher
	dbQuery := r.db.WithContext(ctx)
	if category != "" {
		dbQuery = dbQuery.Where("LOWER(category) LIKE ?", "%"+strings.ToLower(category)+"%")
	}
	if query != "" {
		q := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.Where("LOWER(email) LIKE ? OR LOWER(name) LIKE ? OR LOWER(specialization) LIKE ?", q, q, q)
	}
	err := dbQuery.Find(&list).Error
	return list, err
}

// GetTeacherByID retrieves a teacher record using its numeric ID.
func (r *postgresAdminRepository) GetTeacherByID(ctx context.Context, id int64) (*domain.Teacher, error) {
	var teacher domain.Teacher
	if err := r.db.WithContext(ctx).First(&teacher, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

// CreateTeacher creates a new teacher profile in the database.
func (r *postgresAdminRepository) CreateTeacher(ctx context.Context, teacher *domain.Teacher) error {
	return r.db.WithContext(ctx).Create(teacher).Error
}

// UpdateTeacher updates an existing teacher profile in the database.
func (r *postgresAdminRepository) UpdateTeacher(ctx context.Context, teacher *domain.Teacher) error {
	return r.db.WithContext(ctx).Save(teacher).Error
}

// DeleteTeacher removes a teacher profile and cleans up associated assignments, schedules, and activities.
// This is executed as an atomic database transaction to prevent dangling foreign key references.
func (r *postgresAdminRepository) DeleteTeacher(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Set courses.teacher_id = NULL
		if err := tx.Exec("UPDATE courses SET teacher_id = NULL WHERE teacher_id = ?", id).Error; err != nil {
			return err
		}

		// 2. Delete from courses_teachers join table
		if err := tx.Exec("DELETE FROM courses_teachers WHERE teacher_id = ?", id).Error; err != nil {
			return err
		}

		// 3. Delete from class_schedules
		if err := tx.Exec("DELETE FROM class_schedules WHERE teacher_id = ?", id).Error; err != nil {
			return err
		}

		// 4. Delete from teacher_activities
		if err := tx.Exec("DELETE FROM teacher_activities WHERE teacher_id = ?", id).Error; err != nil {
			return err
		}

		// 5. Delete the teacher record
		if err := tx.Delete(&domain.Teacher{}, id).Error; err != nil {
			return err
		}

		return nil
	})
}

// ListStudents retrieves student records preloaded with user profiles, optionally filtering by course categories or search queries.
func (r *postgresAdminRepository) ListStudents(ctx context.Context, query, category string) ([]domain.Student, error) {
	var students []domain.Student
	dbQuery := r.db.WithContext(ctx).Preload("User")
	
	if category != "" {
		dbQuery = dbQuery.
			Joins("JOIN enrollments ON enrollments.student_id = students.id").
			Joins("JOIN courses ON courses.id = enrollments.course_id").
			Where("LOWER(courses.category) = ?", strings.ToLower(category)).
			Distinct()
	} else {
		dbQuery = dbQuery.Where(
			"students.id IN (SELECT student_id FROM enrollments)",
		)
	}
	
	if query != "" {
		q := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.
			Joins("JOIN users ON users.id = students.user_id").
			Where("LOWER(users.full_name) LIKE ? OR LOWER(users.contact_number) LIKE ? OR LOWER(users.email) LIKE ?", q, q, q)
	}
	
	err := dbQuery.Order("(SELECT MAX(created_at) FROM enrollments WHERE enrollments.student_id = students.id) DESC").Find(&students).Error
	return students, err
}

// GetStudentEnrollmentInfo aggregates course names, batch IDs, subjects list, and teacher names
// for a slice of student IDs. It structures responses inside maps to prevent N+1 nested GORM loops.
func (r *postgresAdminRepository) GetStudentEnrollmentInfo(ctx context.Context, studentIDs []int64) (map[int64][]string, map[int64][]string, map[int64][]string, map[int64][]string, error) {
	if len(studentIDs) == 0 {
		return make(map[int64][]string), make(map[int64][]string), make(map[int64][]string), make(map[int64][]string), nil
	}
	var results []struct {
		StudentID   int64  `gorm:"column:student_id"`
		CourseID    int64  `gorm:"column:course_id"`
		CourseName  string `gorm:"column:course_name"`
		BatchID     string `gorm:"column:batch_id"`
		TeacherName string `gorm:"column:teacher_name"`
	}

	err := r.db.WithContext(ctx).Table("enrollments").
		Select("enrollments.student_id, enrollments.course_id, courses.course_name, courses.batch_id, teachers.name as teacher_name").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Joins("LEFT JOIN teachers ON teachers.id = courses.teacher_id").
		Where("enrollments.student_id IN ?", studentIDs).
		Scan(&results).Error
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var courseIDs []int64
	courseIDMap := make(map[int64]bool)
	for _, res := range results {
		if !courseIDMap[res.CourseID] {
			courseIDMap[res.CourseID] = true
			courseIDs = append(courseIDs, res.CourseID)
		}
	}

	var subjectsResults []struct {
		CourseID    int64  `gorm:"column:course_id"`
		SubjectName string `gorm:"column:subject_name"`
	}

	if len(courseIDs) > 0 {
		err = r.db.WithContext(ctx).Table("courses_subjects").
			Select("courses_subjects.course_id, subjects.subject_name").
			Joins("JOIN subjects ON subjects.id = courses_subjects.subject_id").
			Where("courses_subjects.course_id IN ?", courseIDs).
			Scan(&subjectsResults).Error
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	courseSubjects := make(map[int64][]string)
	for _, sub := range subjectsResults {
		courseSubjects[sub.CourseID] = append(courseSubjects[sub.CourseID], sub.SubjectName)
	}

	appendUnique := func(slice []string, val string) []string {
		for _, s := range slice {
			if s == val {
				return slice
			}
		}
		return append(slice, val)
	}

	coursesMap := make(map[int64][]string)
	batchesMap := make(map[int64][]string)
	subjectsMap := make(map[int64][]string)
	teachersMap := make(map[int64][]string)

	for _, res := range results {
		coursesMap[res.StudentID] = appendUnique(coursesMap[res.StudentID], res.CourseName)
		batchesMap[res.StudentID] = appendUnique(batchesMap[res.StudentID], res.BatchID)
		if res.TeacherName != "" {
			teachersMap[res.StudentID] = appendUnique(teachersMap[res.StudentID], res.TeacherName)
		}
		for _, subName := range courseSubjects[res.CourseID] {
			subjectsMap[res.StudentID] = appendUnique(subjectsMap[res.StudentID], subName)
		}
	}

	return coursesMap, batchesMap, subjectsMap, teachersMap, nil
}

// GetCoursesSales retrieves aggregated revenue stats and purchase counts for courses in a date window.
func (r *postgresAdminRepository) GetCoursesSales(ctx context.Context, category string, start, end time.Time) ([]domain.CourseSales, error) {
	var list []domain.CourseSales
	query := r.db.WithContext(ctx).Table("courses").
		Select(`
			courses.id, 
			courses.course_name, 
			courses.batch_id, 
			courses.final_price as price, 
			COUNT(enrollments.id) as sales_count,
			COALESCE((SELECT SUM(amount) FROM payment_orders WHERE payment_orders.course_id = courses.id AND payment_orders.status = 'completed' AND payment_orders.order_type = 'course' AND payment_orders.created_at BETWEEN ? AND ?), 0) as revenue
		`, start, end).
		Joins("LEFT JOIN enrollments ON enrollments.course_id = courses.id AND enrollments.created_at BETWEEN ? AND ?", start, end).
		Group("courses.id, courses.course_name, courses.batch_id, courses.final_price").
		Order("courses.course_name")
	if category != "" {
		query = query.Where("LOWER(courses.category) = ?", strings.ToLower(category))
	}
	if err := query.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetNotesSales retrieves aggregated revenue metrics for public paid smart PDF study notes.
func (r *postgresAdminRepository) GetNotesSales(ctx context.Context, category string, start, end time.Time) ([]domain.NoteSales, error) {
	var list []domain.NoteSales
	query := r.db.WithContext(ctx).Table("notes").
		Select(`
			notes.id, 
			notes.title, 
			notes.price, 
			COUNT(note_accesses.id) as sales_count,
			COALESCE((SELECT SUM(amount) FROM payment_orders WHERE payment_orders.note_id = notes.id AND payment_orders.status = 'completed' AND payment_orders.order_type = 'note' AND payment_orders.created_at BETWEEN ? AND ?), 0) as revenue
		`, start, end).
		Joins("LEFT JOIN note_accesses ON note_accesses.note_id = notes.id AND note_accesses.created_at BETWEEN ? AND ?", start, end).
		Where("notes.note_type = ? AND notes.is_free = ?", "public", false).
		Group("notes.id, notes.title, notes.price").
		Order("notes.title")
	if category != "" {
		query = query.Where("LOWER(notes.category) = ?", strings.ToLower(category))
	}
	if err := query.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetTestSeriesSales retrieves purchase counts and total revenue generated by paid test series.
func (r *postgresAdminRepository) GetTestSeriesSales(ctx context.Context, category string, start, end time.Time) ([]domain.TestSeriesSales, error) {
	var list []domain.TestSeriesSales
	query := r.db.WithContext(ctx).Table("test_series").
		Select(`
			test_series.id, 
			test_series.title, 
			test_series.price, 
			COUNT(test_series_accesses.id) as sales_count,
			COALESCE((SELECT SUM(amount) FROM payment_orders WHERE payment_orders.test_series_id = test_series.id AND payment_orders.status = 'completed' AND payment_orders.order_type = 'test_series' AND payment_orders.created_at BETWEEN ? AND ?), 0) as revenue
		`, start, end).
		Joins("LEFT JOIN test_series_accesses ON test_series_accesses.test_series_id = test_series.id AND test_series_accesses.created_at BETWEEN ? AND ?", start, end).
		Where("test_series.is_free = ? AND test_series.course_id IS NULL", false).
		Group("test_series.id, test_series.title, test_series.price").
		Order("test_series.title")
	if category != "" {
		query = query.Where("LOWER(test_series.category) = ?", strings.ToLower(category))
	}
	if err := query.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListCategories returns a sorted list of registered categories.
func (r *postgresAdminRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	var list []domain.Category
	err := r.db.WithContext(ctx).Order("name").Find(&list).Error
	return list, err
}

// GetCategoryByID retrieves a category record by ID.
func (r *postgresAdminRepository) GetCategoryByID(ctx context.Context, id int64) (*domain.Category, error) {
	var cat domain.Category
	if err := r.db.WithContext(ctx).First(&cat, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cat, nil
}

// GetCategoryByName retrieves a category record by its exact string name.
func (r *postgresAdminRepository) GetCategoryByName(ctx context.Context, name string) (*domain.Category, error) {
	var cat domain.Category
	if err := r.db.WithContext(ctx).Where("LOWER(name) = ?", strings.ToLower(name)).First(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cat, nil
}

// CreateCategory adds a category record.
func (r *postgresAdminRepository) CreateCategory(ctx context.Context, category *domain.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

// UpdateCategory updates category settings.
func (r *postgresAdminRepository) UpdateCategory(ctx context.Context, category *domain.Category) error {
	return r.db.WithContext(ctx).Save(category).Error
}

// DeleteCategory deletes a category record.
func (r *postgresAdminRepository) DeleteCategory(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.Category{}, id).Error
}

// CascadeCategoryUpdate cascades category field updates across related courses, notes, test series, and teachers in a transaction.
func (r *postgresAdminRepository) CascadeCategoryUpdate(ctx context.Context, oldName, newName string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Table("courses").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("notes").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("test_series").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("teachers").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		return nil
	})
}

// CascadeCategoryDelete sets category fields to blank across related tables on category deletion.
func (r *postgresAdminRepository) CascadeCategoryDelete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Table("courses").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("notes").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("test_series").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("teachers").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		return nil
	})
}

// AssignTeacherToCourses establishes joint course assignments in the courses_teachers join table.
func (r *postgresAdminRepository) AssignTeacherToCourses(ctx context.Context, teacherID int64, courseNames []string) error {
	if len(courseNames) == 0 {
		return nil
	}
	var courses []domain.Course
	var lowerNames []string
	for _, name := range courseNames {
		lowerNames = append(lowerNames, strings.ToLower(strings.TrimSpace(name)))
	}
	if err := r.db.WithContext(ctx).Where("LOWER(TRIM(course_name)) IN ?", lowerNames).Find(&courses).Error; err != nil {
		return err
	}
	for _, c := range courses {
		// Append to join table.
		var link struct {
			CourseID  int64 `gorm:"column:course_id"`
			TeacherID int64 `gorm:"column:teacher_id"`
		}
		link.CourseID = c.ID
		link.TeacherID = teacherID
		r.db.WithContext(ctx).Table("courses_teachers").FirstOrCreate(&link, link)

		// Set primary teacher if empty.
		if c.TeacherID == nil {
			tid := teacherID
			r.db.WithContext(ctx).Model(&c).Update("teacher_id", tid)
		}
	}
	return nil
}

// UnassignTeacherFromOldCourses removes teacher course assignments that are no longer part of their courseNames list.
func (r *postgresAdminRepository) UnassignTeacherFromOldCourses(ctx context.Context, teacherID int64, courseNames []string) error {
	var links []struct {
		CourseID int64 `gorm:"column:course_id"`
	}
	if err := r.db.WithContext(ctx).Table("courses_teachers").Where("teacher_id = ?", teacherID).Find(&links).Error; err != nil {
		return err
	}

	for _, link := range links {
		var course domain.Course
		if err := r.db.WithContext(ctx).First(&course, link.CourseID).Error; err == nil {
			inNewList := false
			for _, cn := range courseNames {
				if strings.EqualFold(strings.TrimSpace(course.CourseName), strings.TrimSpace(cn)) {
					inNewList = true
					break
				}
			}
			if !inNewList {
				// Delete from join table using raw SQL for 100% reliability.
				r.db.WithContext(ctx).Exec("DELETE FROM courses_teachers WHERE course_id = ? AND teacher_id = ?", course.ID, teacherID)

				// Update course.teacher_id if it matched.
				if course.TeacherID != nil && *course.TeacherID == teacherID {
					var fallback struct {
						TeacherID int64 `gorm:"column:teacher_id"`
					}
					err := r.db.WithContext(ctx).Table("courses_teachers").
						Where("course_id = ?", course.ID).
						First(&fallback).Error
					var newTid *int64
					if err == nil {
						newTid = &fallback.TeacherID
					}
					r.db.WithContext(ctx).Model(&course).Update("teacher_id", newTid)
				}
			}
		}
	}
	return nil
}

// GetCourseByName retrieves a course record by name using a case-insensitive check.
func (r *postgresAdminRepository) GetCourseByName(ctx context.Context, name string) (*domain.Course, error) {
	var c domain.Course
	if err := r.db.WithContext(ctx).Where("LOWER(course_name) = ?", strings.ToLower(name)).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// GetCourseByBatchID retrieves a course record by its unique batch ID.
func (r *postgresAdminRepository) GetCourseByBatchID(ctx context.Context, batchID string) (*domain.Course, error) {
	var c domain.Course
	if err := r.db.WithContext(ctx).Where("LOWER(batch_id) = ?", strings.ToLower(batchID)).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// DeleteClassSchedulesBySignature removes specific lecture schedules matching teacher, batch, topic, date and start time.
func (r *postgresAdminRepository) DeleteClassSchedulesBySignature(ctx context.Context, teacherID int64, batchID, topic string, date time.Time, startTime string) error {
	// Parse start time to fit DB formats (either HH:MM or HH:MM:SS)
	formattedTime := startTime
	if len(formattedTime) == 5 {
		formattedTime = formattedTime + ":00"
	}
	return r.db.WithContext(ctx).
		Where("teacher_id = ? AND batch_id = ? AND LOWER(topic_name) = ? AND class_date = ? AND start_time::text LIKE ?", 
			teacherID, batchID, strings.ToLower(topic), date, formattedTime+"%").
		Delete(&domain.ClassSchedule{}).Error
}

// UpsertClassSchedule creates or updates a lecture schedule based on teacher, course, date and time parameters.
func (r *postgresAdminRepository) UpsertClassSchedule(ctx context.Context, schedule *domain.ClassSchedule, topic string, subjectObj *domain.Subject) error {
	var existing domain.ClassSchedule
	err := r.db.WithContext(ctx).
		Where("teacher_id = ? AND course_id = ? AND class_date = ? AND start_time = ?", 
			schedule.TeacherID, schedule.CourseID, schedule.ClassDate, schedule.StartTime).
		First(&existing).Error
	if err == nil {
		// Update existing record details.
		existing.TopicName = topic
		existing.BatchID = schedule.BatchID
		if subjectObj != nil {
			existing.SubjectID = &subjectObj.ID
		}
		return r.db.WithContext(ctx).Save(&existing).Error
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new schedule record.
		if subjectObj != nil {
			schedule.SubjectID = &subjectObj.ID
		}
		return r.db.WithContext(ctx).Create(schedule).Error
	}
	return err
}

// GetSiteStatus retrieves (or builds) the site status overview metrics record (ID: 1).
func (r *postgresAdminRepository) GetSiteStatus(ctx context.Context) (*domain.SiteStatus, error) {
	var status domain.SiteStatus
	if err := r.db.WithContext(ctx).FirstOrCreate(&status, domain.SiteStatus{ID: 1}).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

// UpdateSiteStatus updates site landing stats counters in the database.
func (r *postgresAdminRepository) UpdateSiteStatus(ctx context.Context, stats *domain.SiteStatus) error {
	stats.ID = 1
	stats.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(stats).Error
}

// GetTotalUsersCount counts total verified user accounts.
func (r *postgresAdminRepository) GetTotalUsersCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&count).Error
	return count, err
}

// GetWeeklyLiveClassesCount counts non-cancelled class schedules inside a date window.
func (r *postgresAdminRepository) GetWeeklyLiveClassesCount(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.ClassSchedule{}).
		Where("class_date BETWEEN ? AND ? AND class_status <> 'cancelled'", start, end).
		Count(&count).Error
	return count, err
}

// GetActiveBatchesCount counts current non-completed courses.
func (r *postgresAdminRepository) GetActiveBatchesCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Course{}).
		Where("course_status <> 'completed'").
		Count(&count).Error
	return count, err
}

// GetTotalNotesCount counts total notes uploaded to the platform.
func (r *postgresAdminRepository) GetTotalNotesCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Note{}).Count(&count).Error
	return count, err
}

// GetRecordingsCount counts completed class schedules that have playback URLs saved.
func (r *postgresAdminRepository) GetRecordingsCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.ClassSchedule{}).
		Where("recorded_class_url IS NOT NULL AND recorded_class_url <> ''").
		Count(&count).Error
	return count, err
}

// ListActiveJobPositions retrieves job openings flagged as active.
func (r *postgresAdminRepository) ListActiveJobPositions(ctx context.Context) ([]domain.JobPosition, error) {
	var list []domain.JobPosition
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("created_at desc").Find(&list).Error
	return list, err
}

// ListJobPositions returns all job openings sorted by creation date.
func (r *postgresAdminRepository) ListJobPositions(ctx context.Context) ([]domain.JobPosition, error) {
	var list []domain.JobPosition
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&list).Error
	return list, err
}

// GetJobPositionByID retrieves a job opening by its ID.
func (r *postgresAdminRepository) GetJobPositionByID(ctx context.Context, id int64) (*domain.JobPosition, error) {
	var jp domain.JobPosition
	err := r.db.WithContext(ctx).First(&jp, id).Error
	if err != nil {
		return nil, err
	}
	return &jp, nil
}

// CreateJobPosition adds a job opening position in the database.
func (r *postgresAdminRepository) CreateJobPosition(ctx context.Context, jp *domain.JobPosition) error {
	return r.db.WithContext(ctx).Create(jp).Error
}

// UpdateJobPosition updates specific attributes on a job opening position in the database.
func (r *postgresAdminRepository) UpdateJobPosition(ctx context.Context, id int64, updates map[string]interface{}) (*domain.JobPosition, error) {
	var jp domain.JobPosition
	if err := r.db.WithContext(ctx).First(&jp, id).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&jp).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &jp, nil
}

// DeleteJobPosition deletes a job opening position record.
func (r *postgresAdminRepository) DeleteJobPosition(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.JobPosition{}, id).Error
}

// ListJobApplications returns all submitted candidate job applications.
func (r *postgresAdminRepository) ListJobApplications(ctx context.Context) ([]domain.JobApplication, error) {
	var list []domain.JobApplication
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&list).Error
	return list, err
}

// GetJobApplicationByID retrieves a candidate job application by ID.
func (r *postgresAdminRepository) GetJobApplicationByID(ctx context.Context, id int64) (*domain.JobApplication, error) {
	var app domain.JobApplication
	err := r.db.WithContext(ctx).First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// CreateJobApplication inserts a new candidate job application record.
func (r *postgresAdminRepository) CreateJobApplication(ctx context.Context, app *domain.JobApplication) error {
	return r.db.WithContext(ctx).Create(app).Error
}

// UpdateJobApplication updates a candidate job application record.
func (r *postgresAdminRepository) UpdateJobApplication(ctx context.Context, app *domain.JobApplication) error {
	return r.db.WithContext(ctx).Save(app).Error
}

