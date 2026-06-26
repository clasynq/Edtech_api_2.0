package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"clasynq/api/admin/internal/domain"

	"gorm.io/gorm"
)

type postgresAdminRepository struct {
	db *gorm.DB
}

func NewPostgresAdminRepository(db *gorm.DB) domain.AdminRepository {
	return &postgresAdminRepository{db: db}
}

func (r *postgresAdminRepository) GetDashboardStats(ctx context.Context) (*domain.AdminDashboard, error) {
	var stats domain.AdminDashboard
	if err := r.db.WithContext(ctx).FirstOrCreate(&stats, domain.AdminDashboard{ID: 1}).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *postgresAdminRepository) RefreshDashboardStats(ctx context.Context) (*domain.AdminDashboard, error) {
	var stats domain.AdminDashboard
	if err := r.db.WithContext(ctx).FirstOrCreate(&stats, domain.AdminDashboard{ID: 1}).Error; err != nil {
		return nil, err
	}
	var studentCount int64
	var teacherCount int64
	var activeBatches int64
	r.db.WithContext(ctx).Model(&domain.Student{}).Count(&studentCount)
	r.db.WithContext(ctx).Model(&domain.Teacher{}).Count(&teacherCount)
	r.db.WithContext(ctx).Model(&domain.Course{}).Where("course_status <> ?", "completed").Count(&activeBatches)

	stats.TotalStudents = studentCount
	stats.TotalTeacher = teacherCount
	stats.ActiveBatches = activeBatches
	if err := r.db.WithContext(ctx).Save(&stats).Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

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

	// Load emails dynamically in a single batch query (1 query instead of N queries)
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

func (r *postgresAdminRepository) CreateTeacher(ctx context.Context, teacher *domain.Teacher) error {
	return r.db.WithContext(ctx).Create(teacher).Error
}

func (r *postgresAdminRepository) UpdateTeacher(ctx context.Context, teacher *domain.Teacher) error {
	return r.db.WithContext(ctx).Save(teacher).Error
}

func (r *postgresAdminRepository) DeleteTeacher(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.Teacher{}, id).Error
}

func (r *postgresAdminRepository) ListStudents(ctx context.Context, query, category string) ([]domain.Student, error) {
	var students []domain.Student
	dbQuery := r.db.WithContext(ctx).Preload("User")
	
	if category != "" {
		dbQuery = dbQuery.
			Joins("JOIN enrollments ON enrollments.student_id = students.id").
			Joins("JOIN courses ON courses.id = enrollments.course_id").
			Where("LOWER(courses.category) = ?", strings.ToLower(category)).
			Distinct()
	}
	
	if query != "" {
		q := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.
			Joins("JOIN users ON users.id = students.user_id").
			Where("LOWER(users.full_name) LIKE ? OR LOWER(users.contact_number) LIKE ? OR LOWER(users.email) LIKE ?", q, q, q)
	}
	
	err := dbQuery.Find(&students).Error
	return students, err
}

func (r *postgresAdminRepository) GetStudentEnrollmentInfo(ctx context.Context, studentIDs []int64) (map[int64][]string, map[int64][]string, error) {
	if len(studentIDs) == 0 {
		return make(map[int64][]string), make(map[int64][]string), nil
	}
	var results []struct {
		StudentID  int64  `gorm:"column:student_id"`
		CourseName string `gorm:"column:course_name"`
		BatchID    string `gorm:"column:batch_id"`
	}

	err := r.db.WithContext(ctx).Table("enrollments").
		Select("enrollments.student_id, courses.course_name, courses.batch_id").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.student_id IN ?", studentIDs).
		Scan(&results).Error
	if err != nil {
		return nil, nil, err
	}

	coursesMap := make(map[int64][]string)
	batchesMap := make(map[int64][]string)
	for _, res := range results {
		coursesMap[res.StudentID] = append(coursesMap[res.StudentID], res.CourseName)
		batchesMap[res.StudentID] = append(batchesMap[res.StudentID], res.BatchID)
	}
	return coursesMap, batchesMap, nil
}

func (r *postgresAdminRepository) GetCoursesSales(ctx context.Context, category string, start, end time.Time) ([]domain.CourseSales, error) {
	var list []domain.CourseSales
	query := r.db.WithContext(ctx).Table("courses").
		Select("courses.id, courses.course_name, courses.batch_id, courses.final_price as price, COUNT(enrollments.id) as sales_count").
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

func (r *postgresAdminRepository) GetNotesSales(ctx context.Context, category string, start, end time.Time) ([]domain.NoteSales, error) {
	var list []domain.NoteSales
	query := r.db.WithContext(ctx).Table("notes").
		Select("notes.id, notes.title, notes.price, COUNT(note_accesses.id) as sales_count").
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

func (r *postgresAdminRepository) GetTestSeriesSales(ctx context.Context, category string, start, end time.Time) ([]domain.TestSeriesSales, error) {
	var list []domain.TestSeriesSales
	query := r.db.WithContext(ctx).Table("test_series").
		Select("test_series.id, test_series.title, test_series.price, COUNT(test_series_accesses.id) as sales_count").
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

func (r *postgresAdminRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	var list []domain.Category
	err := r.db.WithContext(ctx).Order("name").Find(&list).Error
	return list, err
}

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

func (r *postgresAdminRepository) CreateCategory(ctx context.Context, category *domain.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *postgresAdminRepository) UpdateCategory(ctx context.Context, category *domain.Category) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *postgresAdminRepository) DeleteCategory(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.Category{}, id).Error
}

func (r *postgresAdminRepository) CascadeCategoryUpdate(ctx context.Context, oldName, newName string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Table("courses").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("notes").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("test_series").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		tx.Table("teachers").Where("LOWER(category) = ?", strings.ToLower(oldName)).Update("category", newName)
		return nil
	})
}

func (r *postgresAdminRepository) CascadeCategoryDelete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Table("courses").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("notes").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("test_series").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		tx.Table("teachers").Where("LOWER(category) = ?", strings.ToLower(name)).Update("category", "")
		return nil
	})
}

func (r *postgresAdminRepository) AssignTeacherToCourses(ctx context.Context, teacherID int64, courseNames []string) error {
	if len(courseNames) == 0 {
		return nil
	}
	var courses []domain.Course
	if err := r.db.WithContext(ctx).Where("course_name IN ?", courseNames).Find(&courses).Error; err != nil {
		return err
	}
	for _, c := range courses {
		// Append to join table
		var link struct {
			CourseID  int64 `gorm:"column:course_id"`
			TeacherID int64 `gorm:"column:teacher_id"`
		}
		link.CourseID = c.ID
		link.TeacherID = teacherID
		r.db.WithContext(ctx).Table("courses_teachers").FirstOrCreate(&link, link)

		// Set primary teacher if empty
		if c.TeacherID == nil {
			tid := teacherID
			r.db.WithContext(ctx).Model(&c).Update("teacher_id", tid)
		}
	}
	return nil
}

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
				if course.CourseName == cn {
					inNewList = true
					break
				}
			}
			if !inNewList {
				// Delete from join table
				r.db.WithContext(ctx).Table("courses_teachers").
					Where("course_id = ? AND teacher_id = ?", course.ID, teacherID).
					Delete(nil)

				// Update course.teacher_id if it matched
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

func (r *postgresAdminRepository) UpsertClassSchedule(ctx context.Context, schedule *domain.ClassSchedule, topic string, subjectObj *domain.Subject) error {
	var existing domain.ClassSchedule
	err := r.db.WithContext(ctx).
		Where("teacher_id = ? AND course_id = ? AND class_date = ? AND start_time = ?", 
			schedule.TeacherID, schedule.CourseID, schedule.ClassDate, schedule.StartTime).
		First(&existing).Error
	if err == nil {
		// Update
		existing.TopicName = topic
		existing.BatchID = schedule.BatchID
		if subjectObj != nil {
			existing.SubjectID = &subjectObj.ID
		}
		return r.db.WithContext(ctx).Save(&existing).Error
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create
		if subjectObj != nil {
			schedule.SubjectID = &subjectObj.ID
		}
		return r.db.WithContext(ctx).Create(schedule).Error
	}
	return err
}

func (r *postgresAdminRepository) GetSiteStatus(ctx context.Context) (*domain.SiteStatus, error) {
	var status domain.SiteStatus
	if err := r.db.WithContext(ctx).FirstOrCreate(&status, domain.SiteStatus{ID: 1}).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *postgresAdminRepository) UpdateSiteStatus(ctx context.Context, stats *domain.SiteStatus) error {
	stats.ID = 1
	stats.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(stats).Error
}

func (r *postgresAdminRepository) GetTotalUsersCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Count(&count).Error
	return count, err
}

func (r *postgresAdminRepository) GetWeeklyLiveClassesCount(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.ClassSchedule{}).
		Where("class_date BETWEEN ? AND ? AND class_status <> 'cancelled'", start, end).
		Count(&count).Error
	return count, err
}

func (r *postgresAdminRepository) GetActiveBatchesCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Course{}).
		Where("course_status <> 'completed'").
		Count(&count).Error
	return count, err
}

func (r *postgresAdminRepository) GetTotalNotesCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Note{}).Count(&count).Error
	return count, err
}

func (r *postgresAdminRepository) GetRecordingsCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.ClassSchedule{}).
		Where("recorded_class_url IS NOT NULL AND recorded_class_url <> ''").
		Count(&count).Error
	return count, err
}
