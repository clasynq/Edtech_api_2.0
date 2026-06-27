package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"clasynq/api/teacher/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresTeacherRepository struct {
	db *gorm.DB
}

func NewPostgresTeacherRepository(db *gorm.DB) domain.TeacherRepository {
	return &postgresTeacherRepository{db: db}
}

func (r *postgresTeacherRepository) GetTeacherByID(ctx context.Context, id int64) (*domain.Teacher, error) {
	var teacher domain.Teacher
	if err := r.db.WithContext(ctx).First(&teacher, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (r *postgresTeacherRepository) GetCoursesByTeacher(ctx context.Context, teacherID int64, category string) ([]domain.Course, error) {
	var courses []domain.Course
	dbQuery := r.db.WithContext(ctx).Table("courses").
		Joins("LEFT JOIN courses_teachers ON courses_teachers.course_id = courses.id").
		Where("courses.teacher_id = ? OR courses_teachers.teacher_id = ?", teacherID, teacherID).
		Distinct("courses.*")

	if category != "" {
		categoryLower := strings.ToLower(strings.TrimSpace(category))
		dbQuery = dbQuery.Where("LOWER(courses.category) = ?", categoryLower)
	}
	
	err := dbQuery.Find(&courses).Error
	return courses, err
}

func (r *postgresTeacherRepository) GetEnrollmentsByCourses(ctx context.Context, courseIDs []int64) ([]domain.Enrollment, error) {
	var list []domain.Enrollment
	if len(courseIDs) == 0 {
		return list, nil
	}
	err := r.db.WithContext(ctx).
		Preload("Student").
		Preload("Student.User").
		Where("course_id IN ?", courseIDs).
		Find(&list).Error
	return list, err
}

func (r *postgresTeacherRepository) GetClassSchedulesByTeacher(ctx context.Context, teacherID int64, category string) ([]domain.ClassSchedule, error) {
	var list []domain.ClassSchedule
	dbQuery := r.db.WithContext(ctx).Preload("Course").Preload("Subject").Preload("Teacher").
		Joins("JOIN courses ON courses.id = class_schedules.course_id").
		Where("class_schedules.teacher_id = ?", teacherID)
	
	if category != "" {
		categoryLower := strings.ToLower(strings.TrimSpace(category))
		dbQuery = dbQuery.Where("LOWER(courses.category) = ?", categoryLower)
	}
	
	err := dbQuery.Order("class_schedules.class_date DESC, class_schedules.start_time DESC").Find(&list).Error
	return list, err
}

func (r *postgresTeacherRepository) GetTeacherActivities(ctx context.Context, teacherID int64, limit int) ([]domain.TeacherActivity, error) {
	var list []domain.TeacherActivity
	err := r.db.WithContext(ctx).
		Where("teacher_id = ?", teacherID).
		Limit(limit).
		Order("created_at DESC").
		Find(&list).Error
	return list, err
}

func (r *postgresTeacherRepository) LogTeacherActivity(ctx context.Context, teacherID int64, action, entityType, entityName string) error {
	act := domain.TeacherActivity{
		TeacherID:  teacherID,
		Action:     action,
		EntityType: entityType,
		EntityName: entityName,
		CreatedAt:  time.Now(),
	}
	return r.db.WithContext(ctx).Create(&act).Error
}

func (r *postgresTeacherRepository) GetCourseByID(ctx context.Context, courseID int64) (*domain.Course, error) {
	var course domain.Course
	if err := r.db.WithContext(ctx).Preload("Teachers").First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

func (r *postgresTeacherRepository) GetUserByID(ctx context.Context, userID int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresTeacherRepository) GetOrCreateStudentProfile(ctx context.Context, user *domain.User) (*domain.Student, error) {
	var student domain.Student
	err := r.db.WithContext(ctx).Where("user_id = ?", user.ID).First(&student).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		student = domain.Student{
			UserID:    user.ID,
			CreatedAt: time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(&student).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return &student, nil
}

func (r *postgresTeacherRepository) GetOrCreateEnrollment(ctx context.Context, studentID, courseID int64) (*domain.Enrollment, bool, error) {
	var enrollment domain.Enrollment
	err := r.db.WithContext(ctx).Where("student_id = ? AND course_id = ?", studentID, courseID).First(&enrollment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		enrollment = domain.Enrollment{
			StudentID: studentID,
			CourseID:  courseID,
			CreatedAt: time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(&enrollment).Error; err != nil {
			return nil, false, err
		}
		return &enrollment, true, nil
	} else if err != nil {
		return nil, false, err
	}
	return &enrollment, false, nil
}

func (r *postgresTeacherRepository) CreateClassSchedule(ctx context.Context, schedule *domain.ClassSchedule) error {
	return r.db.WithContext(ctx).Omit(clause.Associations).Create(schedule).Error
}

func (r *postgresTeacherRepository) GetClassScheduleByID(ctx context.Context, id int64) (*domain.ClassSchedule, error) {
	var sched domain.ClassSchedule
	if err := r.db.WithContext(ctx).Preload("Course").First(&sched, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sched, nil
}

func (r *postgresTeacherRepository) UpdateClassSchedule(ctx context.Context, schedule *domain.ClassSchedule) error {
	return r.db.WithContext(ctx).Omit(clause.Associations, "CreatedAt").Save(schedule).Error
}

func (r *postgresTeacherRepository) DeleteClassSchedule(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.ClassSchedule{}, id).Error
}

func (r *postgresTeacherRepository) GetSubjectsForCourse(ctx context.Context, courseID int64) ([]domain.Subject, error) {
	var list []domain.Subject
	err := r.db.WithContext(ctx).Table("subjects").
		Joins("JOIN courses_subjects ON courses_subjects.subject_id = subjects.id").
		Where("courses_subjects.course_id = ?", courseID).
		Find(&list).Error
	return list, err
}

func (r *postgresTeacherRepository) GetSubjectByID(ctx context.Context, subjectID int64) (*domain.Subject, error) {
	var sub domain.Subject
	if err := r.db.WithContext(ctx).First(&sub, subjectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *postgresTeacherRepository) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	var list []domain.User
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

func (r *postgresTeacherRepository) CreateNote(ctx context.Context, note *domain.Note) error {
	return r.db.WithContext(ctx).Create(note).Error
}

func (r *postgresTeacherRepository) GetCourseByBatchID(ctx context.Context, batchID string) (*domain.Course, error) {
	var course domain.Course
	err := r.db.WithContext(ctx).Where("LOWER(batch_id) = ?", strings.ToLower(batchID)).First(&course).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &course, err
}

func parseCategories(catStr string) []string {
	var list []string
	parts := strings.Split(catStr, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			list = append(list, strings.ToLower(trimmed))
		}
	}
	return list
}

