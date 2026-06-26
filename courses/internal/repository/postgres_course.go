package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"clasynq/api/courses/internal/domain"

	"gorm.io/gorm"
)

type postgresCourseRepository struct {
	db *gorm.DB
}

func NewPostgresCourseRepository(db *gorm.DB) domain.CourseRepository {
	return &postgresCourseRepository{db: db}
}

func (r *postgresCourseRepository) GetCourses(ctx context.Context, role string, userID int64, isFeatured *bool, search string, category string, limit int) ([]domain.Course, error) {
	var courses []domain.Course
	query := r.db.WithContext(ctx).Model(&domain.Course{})

	// 1. Role-based visibility filtering
	switch role {
	case "admin":
		// Admin sees all, no restriction
	case "teacher":
		// Teacher sees public courses OR private courses they are assigned to
		teacherSub := r.db.Table("courses_teachers").Select("course_id").Where("teacher_id = ?", userID)
		query = query.Where("visibility = ? OR id IN (?) OR teacher_id = ?", "public", teacherSub, userID)
	case "student", "user":
		// Student sees public courses OR private courses they are enrolled in
		studentSub := r.db.Table("students").Select("id").Where("user_id = ?", userID)
		enrollSub := r.db.Table("enrollments").Select("course_id").Where("student_id IN (?)", studentSub)
		query = query.Where("visibility = ? OR id IN (?)", "public", enrollSub)
	default:
		// Anonymous / default sees public only
		query = query.Where("visibility = ?", "public")
	}

	// 2. Extra Filters
	if isFeatured != nil {
		query = query.Where("is_featured = ?", *isFeatured)
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}

	if search != "" {
		searchParam := "%" + strings.ToLower(search) + "%"
		
		// Use a subquery to filter course IDs to avoid duplicate rows during M2M preloading
		courseIDsSub := r.db.Table("courses").Select("courses.id").
			Joins("LEFT JOIN teachers ON teachers.id = courses.teacher_id").
			Joins("LEFT JOIN courses_teachers ON courses_teachers.course_id = courses.id").
			Joins("LEFT JOIN teachers t2 ON t2.id = courses_teachers.teacher_id").
			Joins("LEFT JOIN courses_subjects ON courses_subjects.course_id = courses.id").
			Joins("LEFT JOIN subjects ON subjects.id = courses_subjects.subject_id").
			Where("LOWER(courses.course_name) LIKE ? OR LOWER(courses.category) LIKE ? OR LOWER(teachers.name) LIKE ? OR LOWER(t2.name) LIKE ? OR LOWER(subjects.subject_name) LIKE ?", searchParam, searchParam, searchParam, searchParam, searchParam)

		query = query.Where("id IN (?)", courseIDsSub)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	// 3. Preload and Order
	err := query.Preload("Teacher").
		Preload("Teachers").
		Preload("Subjects").
		Order("created_at ASC").
		Find(&courses).Error

	if err != nil {
		return nil, err
	}

	return courses, nil
}

func (r *postgresCourseRepository) GetCourseByIDOrSlug(ctx context.Context, idOrSlug string, role string, userID int64) (*domain.Course, error) {
	var course domain.Course
	query := r.db.WithContext(ctx).Model(&domain.Course{})

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("slug = ?", idOrSlug)
	}

	err := query.Preload("Teacher").
		Preload("Teachers").
		Preload("Subjects").
		First(&course).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// Enforce private visibility check
	if course.Visibility == "private" {
		hasAccess := false
		if role == "admin" {
			hasAccess = true
		} else if role == "teacher" {
			if course.TeacherID != nil && *course.TeacherID == userID {
				hasAccess = true
			} else {
				for _, t := range course.Teachers {
					if t.ID == userID {
						hasAccess = true
						break
					}
				}
			}
		} else if role == "student" || role == "user" {
			// Check if enrolled
			var count int64
			studentSub := r.db.Table("students").Select("id").Where("user_id = ?", userID)
			r.db.Table("enrollments").Where("student_id IN (?) AND course_id = ?", studentSub, course.ID).Count(&count)
			if count > 0 {
				hasAccess = true
			}
		}

		if !hasAccess {
			return nil, errors.New("forbidden: access denied to private course")
		}
	}

	return &course, nil
}

func (r *postgresCourseRepository) CreateCourse(ctx context.Context, course *domain.Course, teacherIDs []int64, subjectIDs []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create course (omitting slices to handle join tables manually and avoid duplication)
		if err := tx.Omit("Teacher", "Teachers", "Subjects").Create(course).Error; err != nil {
			return err
		}

		// Save teachers association
		if len(teacherIDs) > 0 {
			var teachers []domain.Teacher
			if err := tx.Where("id IN ?", teacherIDs).Find(&teachers).Error; err != nil {
				return err
			}
			for _, t := range teachers {
				if err := tx.Exec("INSERT INTO courses_teachers (course_id, teacher_id) VALUES (?, ?) ON CONFLICT DO NOTHING", course.ID, t.ID).Error; err != nil {
					return err
				}
			}
		}

		// Save subjects association
		if len(subjectIDs) > 0 {
			var subjects []domain.Subject
			if err := tx.Where("id IN ?", subjectIDs).Find(&subjects).Error; err != nil {
				return err
			}
			for _, s := range subjects {
				if err := tx.Exec("INSERT INTO courses_subjects (course_id, subject_id) VALUES (?, ?) ON CONFLICT DO NOTHING", course.ID, s.ID).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (r *postgresCourseRepository) UpdateCourse(ctx context.Context, course *domain.Course, teacherIDs *[]int64, subjectIDs *[]int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update course standard fields
		if err := tx.Omit("Teacher", "Teachers", "Subjects").Save(course).Error; err != nil {
			return err
		}

		// Update teachers association if provided
		if teacherIDs != nil {
			if err := tx.Exec("DELETE FROM courses_teachers WHERE course_id = ?", course.ID).Error; err != nil {
				return err
			}
			if len(*teacherIDs) > 0 {
				var teachers []domain.Teacher
				if err := tx.Where("id IN ?", *teacherIDs).Find(&teachers).Error; err != nil {
					return err
				}
				for _, t := range teachers {
					if err := tx.Exec("INSERT INTO courses_teachers (course_id, teacher_id) VALUES (?, ?) ON CONFLICT DO NOTHING", course.ID, t.ID).Error; err != nil {
						return err
					}
				}
			}
		}

		// Update subjects association if provided
		if subjectIDs != nil {
			if err := tx.Exec("DELETE FROM courses_subjects WHERE course_id = ?", course.ID).Error; err != nil {
				return err
			}
			if len(*subjectIDs) > 0 {
				var subjects []domain.Subject
				if err := tx.Where("id IN ?", *subjectIDs).Find(&subjects).Error; err != nil {
					return err
				}
				for _, s := range subjects {
					if err := tx.Exec("INSERT INTO courses_subjects (course_id, subject_id) VALUES (?, ?) ON CONFLICT DO NOTHING", course.ID, s.ID).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func (r *postgresCourseRepository) DeleteCourse(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear join table associations first
		var course domain.Course
		course.ID = id
		if err := tx.Model(&course).Association("Teachers").Clear(); err != nil {
			return err
		}
		if err := tx.Model(&course).Association("Subjects").Clear(); err != nil {
			return err
		}

		// 1. Delete associated class_schedules (since course_id is NOT NULL)
		if err := tx.Exec("DELETE FROM class_schedules WHERE course_id = ?", id).Error; err != nil {
			return err
		}

		// 2. Delete associated enrollments (since course_id is NOT NULL)
		if err := tx.Exec("DELETE FROM enrollments WHERE course_id = ?", id).Error; err != nil {
			return err
		}

		// 3. Nullify nullable course_id references to prevent foreign key constraint violations
		if err := tx.Exec("UPDATE notes SET course_id = NULL WHERE course_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE test_series SET course_id = NULL WHERE course_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE payment_orders SET course_id = NULL WHERE course_id = ?", id).Error; err != nil {
			return err
		}

		// Delete the course itself
		if err := tx.Delete(&course).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *postgresCourseRepository) ListTeachers(ctx context.Context) ([]domain.Teacher, error) {
	var teachers []domain.Teacher
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&teachers).Error; err != nil {
		return nil, err
	}
	return teachers, nil
}

func (r *postgresCourseRepository) ListSubjects(ctx context.Context) ([]domain.Subject, error) {
	var subjects []domain.Subject
	if err := r.db.WithContext(ctx).Order("subject_name ASC").Find(&subjects).Error; err != nil {
		return nil, err
	}
	return subjects, nil
}

func (r *postgresCourseRepository) CreateSubject(ctx context.Context, subject *domain.Subject) error {
	return r.db.WithContext(ctx).Create(subject).Error
}

func (r *postgresCourseRepository) UpdateSubjectMeetingLink(ctx context.Context, id int64, link string) error {
	return r.db.WithContext(ctx).Model(&domain.Subject{}).Where("id = ?", id).Update("meeting_link", link).Error
}

func (r *postgresCourseRepository) ListSchedules(ctx context.Context, filters map[string]string) ([]domain.ClassSchedule, error) {
	var schedules []domain.ClassSchedule
	query := r.db.WithContext(ctx).Model(&domain.ClassSchedule{})

	if teacherID, ok := filters["teacher"]; ok && teacherID != "" {
		query = query.Where("teacher_id = ?", teacherID)
	}
	if courseID, ok := filters["course"]; ok && courseID != "" {
		query = query.Where("course_id = ?", courseID)
	}
	if batchID, ok := filters["batch"]; ok && batchID != "" {
		query = query.Where("batch_id = ?", batchID)
	}
	if status, ok := filters["status"]; ok && status != "" {
		query = query.Where("class_status = ?", status)
	}
	if startDate, ok := filters["start_date"]; ok && startDate != "" {
		query = query.Where("class_date >= ?", startDate)
	}
	if endDate, ok := filters["end_date"]; ok && endDate != "" {
		query = query.Where("class_date <= ?", endDate)
	}
	if category, ok := filters["category"]; ok && category != "" {
		query = query.Joins("JOIN courses ON courses.id = class_schedules.course_id").
			Where("courses.category = ?", category)
	}

	query = query.Preload("Course").Preload("Teacher").Preload("Subject")

	if err := query.Order("class_date ASC, start_time ASC").Find(&schedules).Error; err != nil {
		return nil, err
	}

	for i := range schedules {
		r.populateVirtualFields(&schedules[i])
	}

	return schedules, nil
}

func (r *postgresCourseRepository) GetScheduleByID(ctx context.Context, id int64) (*domain.ClassSchedule, error) {
	var schedule domain.ClassSchedule
	err := r.db.WithContext(ctx).
		Preload("Course").
		Preload("Teacher").
		Preload("Subject").
		First(&schedule, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	r.populateVirtualFields(&schedule)
	return &schedule, nil
}

func (r *postgresCourseRepository) CreateSchedule(ctx context.Context, schedule *domain.ClassSchedule) error {
	return r.db.WithContext(ctx).Create(schedule).Error
}

func (r *postgresCourseRepository) UpdateSchedule(ctx context.Context, schedule *domain.ClassSchedule) error {
	return r.db.WithContext(ctx).Save(schedule).Error
}

func (r *postgresCourseRepository) DeleteSchedule(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&domain.ClassSchedule{}, id).Error
}

func (r *postgresCourseRepository) CategoryExists(ctx context.Context, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Category{}).Where("LOWER(name) = ?", strings.ToLower(name)).Count(&count).Error
	return count > 0, err
}

func (r *postgresCourseRepository) populateVirtualFields(sch *domain.ClassSchedule) {
	if sch.Course != nil {
		sch.CourseName = sch.Course.CourseName
		sch.CourseBannerURL = sch.Course.BannerURL
	}
	if sch.Teacher != nil {
		sch.TeacherName = sch.Teacher.Name
		sch.TeacherSpecialization = sch.Teacher.Specialization
	}
	if sch.Subject != nil {
		sch.SubjectName = sch.Subject.SubjectName
		sch.MeetingLink = sch.Subject.MeetingLink
	} else {
		sch.SubjectName = "No Subject Assigned"
		if sch.Course != nil {
			sch.MeetingLink = sch.Course.MeetingLink
		}
	}

	if sch.Course != nil && sch.Teacher != nil && len(sch.Course.TeacherSubjects) > 0 {
		var teacherSubjects map[string][]int64
		if err := json.Unmarshal(sch.Course.TeacherSubjects, &teacherSubjects); err == nil {
			teacherIDStr := fmt.Sprintf("%d", sch.TeacherID)
			subjectIDs, exists := teacherSubjects[teacherIDStr]
			if exists && len(subjectIDs) > 0 {
				var courseSubjects []domain.Subject
				r.db.Table("subjects").
					Joins("JOIN courses_subjects ON courses_subjects.subject_id = subjects.id").
					Where("courses_subjects.course_id = ?", sch.CourseID).
					Find(&courseSubjects)

				var matchedSubject *domain.Subject
				for _, subID := range subjectIDs {
					for _, cs := range courseSubjects {
						if cs.ID == subID {
							matchedSubject = &cs
							break
						}
					}
					if matchedSubject != nil {
						break
					}
				}

				if matchedSubject != nil {
					if sch.Subject == nil {
						sch.SubjectName = matchedSubject.SubjectName
					}
					if sch.Subject == nil || sch.Subject.MeetingLink == "" {
						if matchedSubject.MeetingLink != "" {
							sch.MeetingLink = matchedSubject.MeetingLink
						}
					}
				}
			}
		}
	}

	if (sch.SubjectName == "No Subject Assigned" || sch.MeetingLink == "") && sch.Course != nil {
		var courseSubjects []domain.Subject
		r.db.Table("subjects").
			Joins("JOIN courses_subjects ON courses_subjects.subject_id = subjects.id").
			Where("courses_subjects.course_id = ?", sch.CourseID).
			Find(&courseSubjects)
		
		if len(courseSubjects) > 0 {
			if sch.SubjectName == "No Subject Assigned" {
				sch.SubjectName = courseSubjects[0].SubjectName
			}
			if sch.MeetingLink == "" {
				for _, cs := range courseSubjects {
					if cs.MeetingLink != "" {
						sch.MeetingLink = cs.MeetingLink
						break
					}
				}
				if sch.MeetingLink == "" {
					sch.MeetingLink = sch.Course.MeetingLink
				}
			}
		}
	}
}

func (r *postgresCourseRepository) GetAnalytics(ctx context.Context, category string) (*domain.ClassesAnalytics, error) {
	var analytics domain.ClassesAnalytics

	// 1. Overall stats
	var totalClasses, completed, pending, cancelled, rescheduled int64
	schedulesQuery := r.db.WithContext(ctx).Table("class_schedules")
	if category != "" {
		schedulesQuery = schedulesQuery.Joins("JOIN courses ON courses.id = class_schedules.course_id").
			Where("courses.category = ?", category)
	}

	schedulesQuery.Count(&totalClasses)
	schedulesQuery.Where("class_status = ?", "completed").Count(&completed)
	schedulesQuery.Where("class_status = ?", "pending").Count(&pending)
	schedulesQuery.Where("class_status = ?", "cancelled").Count(&cancelled)
	schedulesQuery.Where("class_status = ?", "rescheduled").Count(&rescheduled)

	progressPercentage := 0.0
	if totalClasses > 0 {
		progressPercentage = math.Round((float64(completed)/float64(totalClasses))*100*100) / 100
	}

	analytics.OverallStats = domain.OverallStats{
		TotalClasses:       totalClasses,
		Completed:          completed,
		Pending:            pending,
		Cancelled:          cancelled,
		Rescheduled:        rescheduled,
		ProgressPercentage: progressPercentage,
	}

	// 2. Batches & Courses analytics
	var courses []domain.Course
	coursesQuery := r.db.WithContext(ctx).Preload("Teacher")
	if category != "" {
		coursesQuery = coursesQuery.Where("category = ?", category)
	}
	if err := coursesQuery.Find(&courses).Error; err != nil {
		return nil, err
	}

	analytics.Batches = make([]domain.BatchAnalytics, 0)
	analytics.Courses = make([]domain.CourseAnalytics, 0)

	for _, c := range courses {
		var total, cCompleted, cPending, cCancelled, cRescheduled, studentCount int64
		r.db.WithContext(ctx).Table("class_schedules").Where("course_id = ?", c.ID).Count(&total)
		r.db.WithContext(ctx).Table("class_schedules").Where("course_id = ? AND class_status = ?", c.ID, "completed").Count(&cCompleted)
		r.db.WithContext(ctx).Table("class_schedules").Where("course_id = ? AND class_status = ?", c.ID, "pending").Count(&cPending)
		r.db.WithContext(ctx).Table("class_schedules").Where("course_id = ? AND class_status = ?", c.ID, "cancelled").Count(&cCancelled)
		r.db.WithContext(ctx).Table("class_schedules").Where("course_id = ? AND class_status = ?", c.ID, "rescheduled").Count(&cRescheduled)
		r.db.WithContext(ctx).Table("enrollments").Where("course_id = ?", c.ID).Count(&studentCount)

		cProgress := 0.0
		if total > 0 {
			cProgress = math.Round((float64(cCompleted)/float64(total))*100*100) / 100
		}

		teacherName := "Unknown"
		if c.Teacher != nil {
			teacherName = c.Teacher.Name
		}

		startDateVal := c.StartDate
		endDateVal := c.EndDate

		analytics.Batches = append(analytics.Batches, domain.BatchAnalytics{
			ID:                 c.ID,
			CourseName:         c.CourseName,
			BatchID:            c.BatchID,
			TeacherName:        teacherName,
			CourseBannerURL:    c.BannerURL,
			TotalStudents:      studentCount,
			TotalClasses:       total,
			Completed:          cCompleted,
			Pending:            cPending,
			Cancelled:          cCancelled,
			Rescheduled:        cRescheduled,
			StartDate:          &startDateVal,
			EndDate:            &endDateVal,
			ProgressPercentage: cProgress,
		})

		analytics.Courses = append(analytics.Courses, domain.CourseAnalytics{
			ID:                   c.ID,
			CourseName:           c.CourseName,
			BatchID:              c.BatchID,
			TotalClasses:         total,
			Completed:            cCompleted,
			Cancelled:            cCancelled,
			Rescheduled:          cRescheduled,
			CompletionPercentage: cProgress,
			TotalStudents:        studentCount,
		})
	}

	// 3. Teacher analytics
	var teachers []domain.Teacher
	teachersQuery := r.db.WithContext(ctx)
	if category != "" {
		teachersQuery = teachersQuery.Where("category = ?", category)
	}
	if err := teachersQuery.Find(&teachers).Error; err != nil {
		return nil, err
	}

	analytics.Teachers = make([]domain.TeacherAnalytics, 0)
	for _, t := range teachers {
		var tTotal, tCompleted, tCancelled, tRescheduled, activeBatchesCount, totalStudentsAssigned int64
		r.db.WithContext(ctx).Table("class_schedules").Where("teacher_id = ?", t.ID).Count(&tTotal)
		r.db.WithContext(ctx).Table("class_schedules").Where("teacher_id = ? AND class_status = ?", t.ID, "completed").Count(&tCompleted)
		r.db.WithContext(ctx).Table("class_schedules").Where("teacher_id = ? AND class_status = ?", t.ID, "cancelled").Count(&tCancelled)
		r.db.WithContext(ctx).Table("class_schedules").Where("teacher_id = ? AND class_status = ?", t.ID, "rescheduled").Count(&tRescheduled)
		
		r.db.WithContext(ctx).Table("courses").Where("teacher_id = ?", t.ID).Count(&activeBatchesCount)

		r.db.WithContext(ctx).Table("enrollments").
			Joins("JOIN courses ON courses.id = enrollments.course_id").
			Where("courses.teacher_id = ?", t.ID).
			Distinct("enrollments.student_id").
			Count(&totalStudentsAssigned)

		tCompletion := 0.0
		if tTotal > 0 {
			tCompletion = math.Round((float64(tCompleted)/float64(tTotal))*100*100) / 100
		}

		analytics.Teachers = append(analytics.Teachers, domain.TeacherAnalytics{
			ID:                   t.ID,
			TeacherName:          t.Name,
			Specialization:       t.Specialization,
			TotalClasses:         tTotal,
			Completed:            tCompleted,
			Cancelled:            tCancelled,
			Rescheduled:          tRescheduled,
			CompletionPercentage: tCompletion,
			ActiveBatches:        activeBatchesCount,
			TotalStudents:        totalStudentsAssigned,
		})
	}

	return &analytics, nil
}
