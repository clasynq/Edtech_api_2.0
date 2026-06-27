package usecase

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"clasynq/api/courses/internal/domain"

	"github.com/redis/go-redis/v9"
)

type courseUsecase struct {
	repo domain.CourseRepository
	rdb  *redis.Client
}

func NewCourseUsecase(repo domain.CourseRepository, rdb *redis.Client) domain.CourseUsecase {
	return &courseUsecase{
		repo: repo,
		rdb:  rdb,
	}
}

func (u *courseUsecase) GetCourses(ctx context.Context, role string, userID int64, isFeatured *bool, search string, category string, limit int) ([]domain.Course, error) {
	featuredStr := "nil"
	if isFeatured != nil {
		featuredStr = strconv.FormatBool(*isFeatured)
	}
	cacheKey := fmt.Sprintf("courses_list:role:%s:user:%d:featured:%s:search:%s:cat:%s:lim:%d", role, userID, featuredStr, search, category, limit)

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cachedCourses []domain.Course
			if err := json.Unmarshal([]byte(val), &cachedCourses); err == nil {
				return cachedCourses, nil
			}
		}
	}

	courses, err := u.repo.GetCourses(ctx, role, userID, isFeatured, search, category, limit)
	if err != nil {
		return nil, err
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(courses); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
		}
	}

	return courses, nil
}

func (u *courseUsecase) GetCourseByIDOrSlug(ctx context.Context, idOrSlug string, role string, userID int64) (*domain.Course, error) {
	cacheKey := fmt.Sprintf("course_detail:%s:role:%s:user:%d", idOrSlug, role, userID)

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cachedCourse domain.Course
			if err := json.Unmarshal([]byte(val), &cachedCourse); err == nil {
				return &cachedCourse, nil
			}
		}
	}

	course, err := u.repo.GetCourseByIDOrSlug(ctx, idOrSlug, role, userID)
	if err != nil {
		return nil, err
	}
	if course == nil {
		return nil, nil
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(course); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
		}
	}

	return course, nil
}

func (u *courseUsecase) CreateCourse(ctx context.Context, course *domain.Course, teacherIDs []int64, subjectIDs []int64) error {
	// 1. Validate Category
	if course.Category == "" {
		return errors.New("Category is required.")
	}
	exists, err := u.repo.CategoryExists(ctx, course.Category)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("Select a valid category. New categories must be created in the Category Management section.")
	}

	// 2. Generate slug if not present
	if course.Slug == "" {
		slug, err := u.generateUniqueSlug(ctx)
		if err != nil {
			return err
		}
		course.Slug = slug
	}

	// 3. Calculate final price
	discount := float64(course.DiscountPercentage)
	course.FinalPrice = math.Round(course.OriginalPrice - (course.OriginalPrice * discount / 100.0))

	// 4. Auto-sync primary teacher
	if course.TeacherID == nil && len(teacherIDs) > 0 {
		course.TeacherID = &teacherIDs[0]
	}

	// 5. Create in DB
	if err := u.repo.CreateCourse(ctx, course, teacherIDs, subjectIDs); err != nil {
		return err
	}

	// 6. Invalidate Cache
	u.invalidateCourseCache(ctx)
	return nil
}

func (u *courseUsecase) UpdateCourse(ctx context.Context, idOrSlug string, updates map[string]interface{}) (*domain.Course, error) {
	course, err := u.repo.GetCourseByIDOrSlug(ctx, idOrSlug, "admin", 0)
	if err != nil {
		return nil, err
	}
	if course == nil {
		return nil, errors.New("course not found")
	}

	var teacherIDs *[]int64
	var subjectIDs *[]int64

	// Apply updates
	if val, ok := updates["courseName"]; ok {
		course.CourseName = val.(string)
	}
	if val, ok := updates["batchId"]; ok {
		course.BatchID = val.(string)
	}
	if val, ok := updates["category"]; ok {
		catStr := val.(string)
		if catStr == "" {
			return nil, errors.New("Category is required.")
		}
		exists, err := u.repo.CategoryExists(ctx, catStr)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, errors.New("Select a valid category. New categories must be created in the Category Management section.")
		}
		course.Category = catStr
	}
	if val, ok := updates["language"]; ok {
		course.Language = val.(string)
	}
	if val, ok := updates["description"]; ok {
		course.Description = val.(string)
	}
	if val, ok := updates["teacher"]; ok {
		if val == nil {
			course.TeacherID = nil
		} else {
			var tid int64
			switch v := val.(type) {
			case float64:
				tid = int64(v)
			case int64:
				tid = v
			case int:
				tid = int64(v)
			case string:
				parsed, _ := strconv.ParseInt(v, 10, 64)
				tid = parsed
			}
			if tid > 0 {
				course.TeacherID = &tid
			} else {
				course.TeacherID = nil
			}
		}
	}
	if val, ok := updates["originalPrice"]; ok {
		switch v := val.(type) {
		case float64:
			course.OriginalPrice = v
		case float32:
			course.OriginalPrice = float64(v)
		}
	}
	if val, ok := updates["discountPercentage"]; ok {
		switch v := val.(type) {
		case float64:
			course.DiscountPercentage = int(v)
		case int:
			course.DiscountPercentage = v
		case int64:
			course.DiscountPercentage = int(v)
		}
	}

	// Re-calculate final price
	discount := float64(course.DiscountPercentage)
	course.FinalPrice = math.Round(course.OriginalPrice - (course.OriginalPrice * discount / 100.0))

	if val, ok := updates["courseStatus"]; ok {
		course.CourseStatus = val.(string)
	}
	if val, ok := updates["startDate"]; ok {
		if t, err := parseDate(val); err == nil {
			course.StartDate = domain.DateStr(t)
		}
	}
	if val, ok := updates["endDate"]; ok {
		if t, err := parseDate(val); err == nil {
			course.EndDate = domain.DateStr(t)
		}
	}
	if val, ok := updates["accessDuration"]; ok {
		course.AccessDuration = val.(string)
	}
	if val, ok := updates["bannerUrl"]; ok {
		course.BannerURL = val.(string)
	}
	if val, ok := updates["meetingLink"]; ok {
		course.MeetingLink = val.(string)
	}
	if val, ok := updates["isFeatured"]; ok {
		course.IsFeatured = val.(bool)
	}
	if val, ok := updates["visibility"]; ok {
		course.Visibility = val.(string)
	}
	if val, ok := updates["teacherSubjects"]; ok {
		if raw, err := json.Marshal(val); err == nil {
			course.TeacherSubjects = raw
		}
	}

	// Check updates for lists
	if val, ok := updates["teachers"]; ok {
		ids := parseIDsList(val)
		teacherIDs = &ids

		// Django primary teacher sync logic
		if course.TeacherID == nil && len(ids) > 0 {
			course.TeacherID = &ids[0]
		} else if course.TeacherID != nil && len(ids) > 0 {
			// If current primary teacher not in the new teachers list, sync to first
			found := false
			for _, id := range ids {
				if id == *course.TeacherID {
					found = true
					break
				}
			}
			if !found {
				course.TeacherID = &ids[0]
			}
		} else if len(ids) == 0 {
			course.TeacherID = nil
		}
	}

	if val, ok := updates["subjects"]; ok {
		ids := parseIDsList(val)
		subjectIDs = &ids
	}

	if err := u.repo.UpdateCourse(ctx, course, teacherIDs, subjectIDs); err != nil {
		return nil, err
	}

	u.invalidateCourseCache(ctx)
	return course, nil
}

func (u *courseUsecase) DeleteCourse(ctx context.Context, idOrSlug string) error {
	course, err := u.repo.GetCourseByIDOrSlug(ctx, idOrSlug, "admin", 0)
	if err != nil {
		return err
	}
	if course == nil {
		return errors.New("course not found")
	}

	if err := u.repo.DeleteCourse(ctx, course.ID); err != nil {
		return err
	}

	u.invalidateCourseCache(ctx)
	return nil
}

func (u *courseUsecase) ListTeachers(ctx context.Context) ([]domain.Teacher, error) {
	return u.repo.ListTeachers(ctx)
}

func (u *courseUsecase) ListSubjects(ctx context.Context) ([]domain.Subject, error) {
	return u.repo.ListSubjects(ctx)
}

func (u *courseUsecase) CreateSubject(ctx context.Context, subject *domain.Subject) error {
	if err := u.repo.CreateSubject(ctx, subject); err != nil {
		return err
	}
	u.invalidateSubjectCache(ctx)
	return nil
}

func (u *courseUsecase) UpdateSubjectMeetingLink(ctx context.Context, id int64, link string) error {
	if err := u.repo.UpdateSubjectMeetingLink(ctx, id, link); err != nil {
		return err
	}
	u.invalidateSubjectCache(ctx)
	return nil
}

func (u *courseUsecase) ListSchedules(ctx context.Context, filters map[string]string) ([]domain.ClassSchedule, error) {
	return u.repo.ListSchedules(ctx, filters)
}

func (u *courseUsecase) CreateSchedule(ctx context.Context, schedule *domain.ClassSchedule) error {
	// 1. Fetch parent course to auto-sync
	course, err := u.repo.GetCourseByIDOrSlug(ctx, strconv.FormatInt(schedule.CourseID, 10), "admin", 0)
	if err != nil {
		return err
	}
	if course == nil {
		return errors.New("parent course not found")
	}

	// 2. Sync BatchID if empty
	if schedule.BatchID == "" {
		schedule.BatchID = course.BatchID
	}

	// 3. Resolve teacher if omitted
	if schedule.TeacherID == 0 {
		var assignedTeacherID int64
		if schedule.SubjectID != nil && len(course.TeacherSubjects) > 0 {
			var teacherSubjects map[string][]interface{}
			if err := json.Unmarshal(course.TeacherSubjects, &teacherSubjects); err == nil {
				for tIDStr, subIDs := range teacherSubjects {
					for _, subIDVal := range subIDs {
						var subID int64
						switch val := subIDVal.(type) {
						case float64:
							subID = int64(val)
						case string:
							subID, _ = strconv.ParseInt(val, 10, 64)
						}
						if subID == *schedule.SubjectID {
							assignedTeacherID, _ = strconv.ParseInt(tIDStr, 10, 64)
							break
						}
					}
					if assignedTeacherID != 0 {
						break
					}
				}
			}
		}

		if assignedTeacherID != 0 {
			schedule.TeacherID = assignedTeacherID
		} else if course.TeacherID != nil {
			schedule.TeacherID = *course.TeacherID
		} else if len(course.Teachers) > 0 {
			schedule.TeacherID = course.Teachers[0].ID
		} else {
			return errors.New("cannot create schedule: no teacher assigned to the course")
		}
	}

	// 4. Create in DB
	if err := u.repo.CreateSchedule(ctx, schedule); err != nil {
		return err
	}

	// 5. Invalidate Cache
	u.invalidateScheduleCache(ctx)
	return nil
}

func (u *courseUsecase) UpdateSchedule(ctx context.Context, id int64, updates map[string]interface{}) (*domain.ClassSchedule, error) {
	schedule, err := u.repo.GetScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, errors.New("schedule not found")
	}

	// Apply updates
	if val, ok := updates["subject"]; ok {
		if val == nil {
			schedule.SubjectID = nil
		} else {
			var sid int64
			switch v := val.(type) {
			case float64:
				sid = int64(v)
			case int64:
				sid = v
			case int:
				sid = int64(v)
			}
			schedule.SubjectID = &sid
		}
	}
	if val, ok := updates["teacher"]; ok {
		var tid int64
		switch v := val.(type) {
		case float64:
			tid = int64(v)
		case int64:
			tid = v
		case int:
			tid = int64(v)
		}
		schedule.TeacherID = tid
	}
	if val, ok := updates["topicName"]; ok {
		schedule.TopicName = val.(string)
	}
	if val, ok := updates["classDate"]; ok {
		if t, err := parseDate(val); err == nil {
			schedule.ClassDate = domain.DateStr(t)
		}
	}
	if val, ok := updates["startTime"]; ok {
		schedule.StartTime = domain.TimeStr(val.(string))
	}
	if val, ok := updates["endTime"]; ok {
		schedule.EndTime = domain.TimeStr(val.(string))
	}
	if val, ok := updates["classStatus"]; ok {
		schedule.ClassStatus = val.(string)
	}
	if val, ok := updates["rescheduleReason"]; ok {
		reason := val.(string)
		schedule.RescheduleReason = &reason
	}
	if val, ok := updates["classNotesUrl"]; ok {
		notes := val.(string)
		schedule.ClassNotesURL = &notes
	}
	if val, ok := updates["recordedClassUrl"]; ok {
		recorded := val.(string)
		schedule.RecordedClassURL = &recorded
	}

	if err := u.repo.UpdateSchedule(ctx, schedule); err != nil {
		return nil, err
	}

	// Invalidate Cache
	u.invalidateScheduleCache(ctx)
	
	// Fetch updated details for population
	return u.repo.GetScheduleByID(ctx, id)
}

func (u *courseUsecase) DeleteSchedule(ctx context.Context, id int64) error {
	if err := u.repo.DeleteSchedule(ctx, id); err != nil {
		return err
	}
	u.invalidateScheduleCache(ctx)
	return nil
}

func (u *courseUsecase) GetAnalytics(ctx context.Context, category string) (*domain.ClassesAnalytics, error) {
	return u.repo.GetAnalytics(ctx, category)
}

// Helpers
func (u *courseUsecase) generateUniqueSlug(ctx context.Context) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 22)
	for {
		for i := range b {
			num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			b[i] = charset[num.Int64()]
		}
		slug := string(b)
		existing, err := u.repo.GetCourseByIDOrSlug(ctx, slug, "admin", 0)
		if err == nil && existing == nil {
			return slug, nil
		}
	}
}

func parseDate(val interface{}) (time.Time, error) {
	switch v := val.(type) {
	case string:
		return time.Parse("2006-01-02", v)
	}
	return time.Time{}, errors.New("invalid date format")
}

func parseIDsList(val interface{}) []int64 {
	var ids []int64
	switch list := val.(type) {
	case []interface{}:
		for _, item := range list {
			switch num := item.(type) {
			case float64:
				ids = append(ids, int64(num))
			case int64:
				ids = append(ids, num)
			case int:
				ids = append(ids, int64(num))
			case string:
				if parsed, err := strconv.ParseInt(num, 10, 64); err == nil {
					ids = append(ids, parsed)
				}
			}
		}
	case []int64:
		ids = list
	}
	return ids
}

func (u *courseUsecase) invalidateCache(ctx context.Context, patterns ...string) {
	if u.rdb == nil {
		return
	}
	for _, pattern := range patterns {
		iter := u.rdb.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			u.rdb.Del(ctx, iter.Val())
		}
	}
}

func (u *courseUsecase) invalidateCourseCache(ctx context.Context) {
	u.invalidateCache(ctx,
		"courses_list*",
		"course_detail:*",
		"homepage_platform_stats",
		"classes_analytics_summary*",
		"teacher_overview_*",
		"teacher_batches_*",
		"teacher_classes_*",
		"me_study_dashboard_*",
		"me_history_*",
	)
}

func (u *courseUsecase) invalidateScheduleCache(ctx context.Context) {
	u.invalidateCache(ctx,
		"homepage_platform_stats",
		"classes_analytics_summary*",
		"teacher_overview_*",
		"teacher_classes_*",
		"me_study_dashboard_*",
		"me_history_*",
	)
}

func (u *courseUsecase) invalidateSubjectCache(ctx context.Context) {
	u.invalidateCache(ctx,
		"courses_list*",
		"me_study_dashboard_*",
	)
}
