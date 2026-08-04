package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"clasynq/api/teacher/internal/domain"

	"github.com/redis/go-redis/v9"
)

type teacherUsecase struct {
	repo domain.TeacherRepository
	rdb  *redis.Client
}

func NewTeacherUsecase(repo domain.TeacherRepository, rdb *redis.Client) domain.TeacherUsecase {
	return &teacherUsecase{repo: repo, rdb: rdb}
}

func (u *teacherUsecase) filterSubjectsForTeacher(teacherSubjectsRaw json.RawMessage, teacherID int64, subjects []domain.Subject) ([]string, []map[string]interface{}) {
	var assignedSubjectIDs []int64
	hasFilter := false
	if len(teacherSubjectsRaw) > 0 {
		var teacherSubjects map[string][]int64
		if err := json.Unmarshal(teacherSubjectsRaw, &teacherSubjects); err == nil {
			teacherIDStr := fmt.Sprintf("%d", teacherID)
			if ids, ok := teacherSubjects[teacherIDStr]; ok {
				assignedSubjectIDs = ids
				hasFilter = true
			}
		}
	}

	var subjectNames []string
	var subjectDetails []map[string]interface{}
	for _, s := range subjects {
		if hasFilter {
			isAssigned := false
			for _, id := range assignedSubjectIDs {
				if s.ID == id {
					isAssigned = true
					break
				}
			}
			if !isAssigned {
				continue
			}
		}

		subjectNames = append(subjectNames, s.SubjectName)
		subjectDetails = append(subjectDetails, map[string]interface{}{
			"id":          s.ID,
			"subjectName": s.SubjectName,
			"meetingLink": s.MeetingLink,
		})
	}
	return subjectNames, subjectDetails
}

func (u *teacherUsecase) GetOverview(ctx context.Context, teacherID int64, category string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("teacher_overview_%d", teacherID)
	if category != "" {
		cacheKey += fmt.Sprintf("_cat_%s", category)
	}

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cached map[string]interface{}
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		}
	}

	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	// 1. Fetch courses
	courses, err := u.repo.GetCoursesByTeacher(ctx, teacherID, category)
	if err != nil {
		return nil, err
	}

	// 2. Batches count
	totalBatches := len(courses)

	// 3. Students enrolled in these courses
	var courseIDs []int64
	for _, c := range courses {
		courseIDs = append(courseIDs, c.ID)
	}
	enrollments, err := u.repo.GetEnrollmentsByCourses(ctx, courseIDs)
	if err != nil {
		return nil, err
	}
	seenStudents := make(map[int64]bool)
	var enrolledStudents []map[string]interface{}
	for _, e := range enrollments {
		if e.StudentID != 0 && !seenStudents[e.StudentID] {
			seenStudents[e.StudentID] = true
			
			enrolledCourses := []string{}
			enrolledBatches := []string{}
			for _, enc := range enrollments {
				if enc.StudentID == e.StudentID {
					enrolledCourses = append(enrolledCourses, enc.Course.CourseName)
					enrolledBatches = append(enrolledBatches, enc.Course.BatchID)
				}
			}

			enrolledStudents = append(enrolledStudents, map[string]interface{}{
				"id":              e.Student.ID,
				"name":            e.Student.User.FullName,
				"phone_number":    e.Student.User.ContactNumber,
				"email":           e.Student.User.Email,
				"enrolled_courses": enrolledCourses,
				"enrolled_batches": enrolledBatches,
			})
		}
	}
	totalStudents := len(enrolledStudents)

	// 4. Scheduled classes
	schedules, err := u.repo.GetClassSchedulesByTeacher(ctx, teacherID, category)
	if err != nil {
		return nil, err
	}

	var upcomingClasses []domain.ClassSchedule
	var completedClasses []domain.ClassSchedule
	for _, s := range schedules {
		if s.ClassStatus == "pending" || s.ClassStatus == "rescheduled" {
			upcomingClasses = append(upcomingClasses, s)
		} else if s.ClassStatus == "completed" {
			completedClasses = append(completedClasses, s)
		}
	}

	// Serialize database upcoming schedules
	var dbSchedules []map[string]interface{}
	for _, s := range upcomingClasses {
		dbSchedules = append(dbSchedules, u.serializeSchedule(ctx, s))
	}

	// Build task schedules from teacher.tasks JSON field
	taskSchedules := u.buildTaskSchedules(ctx, teacher, category)

	combinedSchedules := append(dbSchedules, taskSchedules...)
	// Sort by class_date and start_time
	u.sortSchedules(combinedSchedules)

	dedupUpcoming := u.deduplicateSchedules(combinedSchedules)

	// Serialize database completed schedules
	var dbCompleted []map[string]interface{}
	for _, s := range completedClasses {
		dbCompleted = append(dbCompleted, u.serializeSchedule(ctx, s))
	}
	dedupCompleted := u.deduplicateSchedules(dbCompleted)

	// 5. Teacher activities
	activities, err := u.repo.GetTeacherActivities(ctx, teacherID, 30)
	if err != nil {
		return nil, err
	}
	var serializedActivities []map[string]interface{}
	for _, act := range activities {
		serializedActivities = append(serializedActivities, map[string]interface{}{
			"id":            act.ID,
			"teacher_email": teacher.Email,
			"action":        act.Action,
			"entity_type":   act.EntityType,
			"entity_name":   act.EntityName,
			"created_at":    act.CreatedAt,
		})
	}

	// 6. All registered users
	allUsers, err := u.repo.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	var serializedAllStudents []map[string]interface{}
	for _, usr := range allUsers {
		serializedAllStudents = append(serializedAllStudents, map[string]interface{}{
			"id":           usr.ID,
			"name":         usr.FullName,
			"phone_number": usr.ContactNumber,
			"email":        usr.Email,
		})
	}

	// 7. Teacher Courses
	var serializedCourses []map[string]interface{}
	for _, c := range courses {
		subjects, _ := u.repo.GetSubjectsForCourse(ctx, c.ID)
		subjectNames, subjectDetails := u.filterSubjectsForTeacher(c.TeacherSubjects, teacherID, subjects)

		// Count students in course
		var count int64
		var courseStudents []map[string]interface{}
		for _, e := range enrollments {
			if e.CourseID == c.ID && e.StudentID != 0 {
				count++
				courseStudents = append(courseStudents, map[string]interface{}{
					"id":           e.Student.ID,
					"name":         e.Student.User.FullName,
					"email":        e.Student.User.Email,
					"phone_number": e.Student.User.ContactNumber,
					"enrolled_at":  e.CreatedAt.Format("2006-01-02"),
				})
			}
		}

		serializedCourses = append(serializedCourses, map[string]interface{}{
			"id":               c.ID,
			"course_name":      c.CourseName,
			"batch_id":         c.BatchID,
			"subjects":         subjectNames,
			"subjects_details": subjectDetails,
			"student_count":    count,
			"students":         courseStudents,
			"meeting_link":     ensureAbsoluteURL(c.MeetingLink),
		})
	}

	var tasksList []map[string]interface{}
	_ = json.Unmarshal([]byte(teacher.Tasks), &tasksList)
	if tasksList == nil {
		tasksList = []map[string]interface{}{}
	}

	response := map[string]interface{}{
		"profile": map[string]interface{}{
			"name":     teacher.Name,
			"email":    teacher.Email,
			"photoUrl": teacher.PhotoURL,
			"tasks":    tasksList,
		},
		"stats": map[string]interface{}{
			"totalBatches":     totalBatches,
			"totalStudents":     totalStudents,
			"upcomingClasses":  len(dedupUpcoming),
			"completedClasses": len(dedupCompleted),
		},
		"activities":       serializedActivities,
		"upcomingSchedule": dedupUpcoming,
		"completedClasses": dedupCompleted,
		"teacherCourses":   serializedCourses,
		"enrolledStudents": enrolledStudents,
		"allStudents":      serializedAllStudents,
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(response); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 5*time.Minute).Err()
		}
	}

	return response, nil
}

func (u *teacherUsecase) AssignStudent(ctx context.Context, teacherID, studentUserID, courseID int64) (map[string]interface{}, error) {
	course, err := u.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course == nil {
		return nil, errors.New("course not found")
	}

	// Verify authorization
	isAssigned := false
	if course.TeacherID != nil && *course.TeacherID == teacherID {
		isAssigned = true
	} else {
		for _, t := range course.Teachers {
			if t.ID == teacherID {
				isAssigned = true
				break
			}
		}
	}
	if !isAssigned {
		return nil, errors.New("you are not authorized to assign students to this course")
	}

	user, err := u.repo.GetUserByID(ctx, studentUserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	studentProfile, err := u.repo.GetOrCreateStudentProfile(ctx, user)
	if err != nil {
		return nil, err
	}

	_, created, err := u.repo.GetOrCreateEnrollment(ctx, studentProfile.ID, course.ID)
	if err != nil {
		return nil, err
	}

	if created {
		_ = u.repo.LogTeacherActivity(ctx, teacherID, "Assigned", "Student", fmt.Sprintf("%s - %s", user.FullName, course.CourseName))
	}

	u.invalidateCache(ctx, teacherID)

	return map[string]interface{}{
		"message":  fmt.Sprintf("Student %s assigned successfully to course %s.", user.FullName, course.CourseName),
		"enrolled": true,
	}, nil
}

func (u *teacherUsecase) GetBatches(ctx context.Context, teacherID int64, category string) ([]map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("teacher_batches_%d", teacherID)
	if category != "" {
		cacheKey += fmt.Sprintf("_cat_%s", category)
	}

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cached []map[string]interface{}
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		}
	}

	courses, err := u.repo.GetCoursesByTeacher(ctx, teacherID, category)
	if err != nil {
		return nil, err
	}

	var serialized []map[string]interface{}
	for _, c := range courses {
		subjects, _ := u.repo.GetSubjectsForCourse(ctx, c.ID)
		subjectNames, subjectDetails := u.filterSubjectsForTeacher(c.TeacherSubjects, teacherID, subjects)

		// Count students in course
		enrollments, _ := u.repo.GetEnrollmentsByCourses(ctx, []int64{c.ID})
		var count int64
		var courseStudents []map[string]interface{}
		for _, e := range enrollments {
			if e.StudentID != 0 {
				count++
				courseStudents = append(courseStudents, map[string]interface{}{
					"id":           e.Student.ID,
					"name":         e.Student.User.FullName,
					"email":        e.Student.User.Email,
					"phone_number": e.Student.User.ContactNumber,
					"enrolled_at":  e.CreatedAt.Format("2006-01-02"),
				})
			}
		}

		serialized = append(serialized, map[string]interface{}{
			"id":               c.ID,
			"course_name":      c.CourseName,
			"batch_id":         c.BatchID,
			"subjects":         subjectNames,
			"subjects_details": subjectDetails,
			"student_count":    count,
			"students":         courseStudents,
			"meeting_link":     ensureAbsoluteURL(c.MeetingLink),
		})
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(serialized); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 5*time.Minute).Err()
		}
	}

	return serialized, nil
}

func (u *teacherUsecase) GetClasses(ctx context.Context, teacherID int64, category string) ([]map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("teacher_classes_%d", teacherID)
	if category != "" {
		cacheKey += fmt.Sprintf("_cat_%s", category)
	}

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cached []map[string]interface{}
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		}
	}

	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	schedules, err := u.repo.GetClassSchedulesByTeacher(ctx, teacherID, category)
	if err != nil {
		return nil, err
	}

	var serializedSchedules []map[string]interface{}
	for _, s := range schedules {
		serializedSchedules = append(serializedSchedules, u.serializeSchedule(ctx, s))
	}

	taskSchedules := u.buildTaskSchedules(ctx, teacher, category)
	combined := append(serializedSchedules, taskSchedules...)
	
	u.sortSchedules(combined)
	dedup := u.deduplicateSchedules(combined)

	if u.rdb != nil {
		if raw, err := json.Marshal(dedup); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 5*time.Minute).Err()
		}
	}

	return dedup, nil
}

func (u *teacherUsecase) ScheduleClass(ctx context.Context, teacherID int64, scheduleData map[string]interface{}) (map[string]interface{}, error) {
	courseIDRaw, ok := scheduleData["course"]
	if !ok {
		return nil, errors.New("course ID is required")
	}
	
	courseID := u.toInt64(courseIDRaw)
	course, err := u.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course == nil {
		return nil, errors.New("course not found")
	}

	// Verify authorization
	isAssigned := false
	if course.TeacherID != nil && *course.TeacherID == teacherID {
		isAssigned = true
	} else {
		for _, t := range course.Teachers {
			if t.ID == teacherID {
				isAssigned = true
				break
			}
		}
	}
	if !isAssigned {
		return nil, errors.New("you are not authorized to schedule classes for this course")
	}

	classDateStr := u.getString(scheduleData["classDate"])
	if classDateStr == "" {
		classDateStr = u.getString(scheduleData["class_date"])
	}
	startTimeStr := u.getString(scheduleData["startTime"])
	if startTimeStr == "" {
		startTimeStr = u.getString(scheduleData["start_time"])
	}
	endTimeStr := u.getString(scheduleData["endTime"])
	if endTimeStr == "" {
		endTimeStr = u.getString(scheduleData["end_time"])
	}
	topicName := u.getString(scheduleData["topicName"])
	if topicName == "" {
		topicName = u.getString(scheduleData["topic_name"])
	}
	rescheduleReason := u.getString(scheduleData["rescheduleReason"])
	if rescheduleReason == "" {
		rescheduleReason = u.getString(scheduleData["reschedule_reason"])
	}

	if classDateStr == "" || startTimeStr == "" || endTimeStr == "" || topicName == "" {
		return nil, errors.New("class date, start time, end time, and topic name are required")
	}

	parsedDate, err := time.Parse("2006-01-02", classDateStr)
	if err != nil {
		parsedDate, err = time.Parse(time.RFC3339, classDateStr)
		if err != nil {
			return nil, errors.New("invalid class date format (expected YYYY-MM-DD)")
		}
	}

	schedule := &domain.ClassSchedule{
		CourseID:    course.ID,
		BatchID:     course.BatchID,
		TeacherID:   teacherID,
		TopicName:   topicName,
		ClassDate:   parsedDate,
		StartTime:   startTimeStr,
		EndTime:     endTimeStr,
		ClassStatus: "pending",
		CreatedAt:   time.Now(),
	}

	if rescheduleReason != "" {
		schedule.RescheduleReason = &rescheduleReason
	}

	if subjectIDRaw, ok := scheduleData["subject"]; ok && subjectIDRaw != nil {
		subID := u.toInt64(subjectIDRaw)
		if subID > 0 {
			schedule.SubjectID = &subID
		}
	}

	if schedule.SubjectID == nil {
		subs, _ := u.repo.GetSubjectsForCourse(ctx, course.ID)
		topicLower := strings.ToLower(topicName)
		for _, sub := range subs {
			subLower := strings.ToLower(sub.SubjectName)
			if subLower != "" && (strings.Contains(topicLower, subLower) || strings.Contains(subLower, topicLower)) {
				schedule.SubjectID = &sub.ID
				break
			}
		}
		
		if schedule.SubjectID == nil {
			if strings.Contains(topicLower, "machine learning") || strings.Contains(topicLower, " ml ") || strings.HasPrefix(topicLower, "ml ") || strings.HasSuffix(topicLower, " ml") || topicLower == "ml" {
				for _, sub := range subs {
					subNameLower := strings.ToLower(sub.SubjectName)
					if subNameLower == "ml" || strings.Contains(subNameLower, "machine learning") {
						schedule.SubjectID = &sub.ID
						break
					}
				}
			} else if strings.Contains(topicLower, "artificial intelligence") || strings.Contains(topicLower, " ai ") || strings.HasPrefix(topicLower, "ai ") || strings.HasSuffix(topicLower, " ai") || topicLower == "ai" {
				for _, sub := range subs {
					subNameLower := strings.ToLower(sub.SubjectName)
					if subNameLower == "ai" || strings.Contains(subNameLower, "artificial intelligence") {
						schedule.SubjectID = &sub.ID
						break
					}
				}
			}
		}
	}

	if err := u.repo.CreateClassSchedule(ctx, schedule); err != nil {
		return nil, err
	}

	_ = u.repo.LogTeacherActivity(ctx, teacherID, "Created", "Class Session", fmt.Sprintf("%s - %s", topicName, course.CourseName))
	u.invalidateCache(ctx, teacherID)

	// Fetch fully loaded struct to return
	fullSched, _ := u.repo.GetClassScheduleByID(ctx, schedule.ID)
	if fullSched != nil {
		return u.serializeSchedule(ctx, *fullSched), nil
	}

	return u.serializeSchedule(ctx, *schedule), nil
}

func (u *teacherUsecase) UpdateClass(ctx context.Context, teacherID, classID int64, updates map[string]interface{}) (map[string]interface{}, error) {
	schedule, err := u.repo.GetClassScheduleByID(ctx, classID)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, errors.New("class schedule not found")
	}

	if schedule.TeacherID != teacherID {
		return nil, errors.New("you do not have permission to access or modify this class schedule")
	}

	var statusVal string
	if s, ok := updates["classStatus"].(string); ok {
		statusVal = s
	} else if s, ok := updates["class_status"].(string); ok {
		statusVal = s
	}

	if statusVal == "rescheduled" {
		// 1. Parse new date
		var newDateStr string
		if d, ok := updates["classDate"].(string); ok && d != "" {
			newDateStr = d
		} else if d, ok := updates["class_date"].(string); ok && d != "" {
			newDateStr = d
		}
		if newDateStr == "" {
			return nil, errors.New("reschedule date is required")
		}
		newDate, err := time.Parse("2006-01-02", newDateStr)
		if err != nil {
			return nil, errors.New("invalid reschedule date format")
		}

		// 2. Parse new start time
		var newStartTime string
		if t, ok := updates["startTime"].(string); ok && t != "" {
			newStartTime = t
		} else if t, ok := updates["start_time"].(string); ok && t != "" {
			newStartTime = t
		}
		if newStartTime == "" {
			newStartTime = "10:00"
		}
		newStartTime = formatTime(newStartTime)

		// 3. Parse new end time
		var newEndTime string
		if t, ok := updates["endTime"].(string); ok && t != "" {
			newEndTime = t
		} else if t, ok := updates["end_time"].(string); ok && t != "" {
			newEndTime = t
		}
		if newEndTime == "" {
			newEndTime = newStartTime
			tObj, err := time.Parse("15:04", newStartTime[:5])
			if err == nil {
				newEndTime = tObj.Add(2 * time.Hour).Format("15:04:05")
			}
		}
		newEndTime = formatTime(newEndTime)

		// 4. Parse reschedule reason
		var reason string
		if r, ok := updates["rescheduleReason"].(string); ok {
			reason = r
		} else if r, ok := updates["reschedule_reason"].(string); ok {
			reason = r
		}

		// 5. Update original row to cancelled and store the formatted reschedule destination info
		origReason := fmt.Sprintf("Rescheduled to %s %s: %s", newDateStr, newStartTime[:5], reason)
		schedule.ClassStatus = "cancelled"
		schedule.RescheduleReason = &origReason
		if err := u.repo.UpdateClassSchedule(ctx, schedule); err != nil {
			return nil, err
		}

		// 6. Create new pending class schedule row
		newSched := &domain.ClassSchedule{
			CourseID:         schedule.CourseID,
			SubjectID:        schedule.SubjectID,
			BatchID:          schedule.BatchID,
			TeacherID:        schedule.TeacherID,
			TopicName:        schedule.TopicName,
			ClassDate:        newDate,
			StartTime:        newStartTime,
			EndTime:          newEndTime,
			ClassStatus:      "pending",
			RescheduleReason: &reason,
			CreatedAt:        time.Now(),
		}
		if err := u.repo.CreateClassSchedule(ctx, newSched); err != nil {
			return nil, err
		}

		_ = u.repo.LogTeacherActivity(ctx, teacherID, "Rescheduled", "Class Session", fmt.Sprintf("%s to %s", schedule.TopicName, newDateStr))
		u.invalidateCache(ctx, teacherID)
		u.invalidateNotesCache(ctx)

		fullSched, _ := u.repo.GetClassScheduleByID(ctx, newSched.ID)
		if fullSched != nil {
			return u.serializeSchedule(ctx, *fullSched), nil
		}
		return u.serializeSchedule(ctx, *newSched), nil
	}

	if topicVal, ok := updates["topicName"]; ok {
		if topic, ok := topicVal.(string); ok {
			schedule.TopicName = topic
		}
	} else if topicVal, ok := updates["topic_name"]; ok {
		if topic, ok := topicVal.(string); ok {
			schedule.TopicName = topic
		}
	}

	var classDateStr string
	if d, ok := updates["classDate"].(string); ok && d != "" {
		classDateStr = d
	} else if d, ok := updates["class_date"].(string); ok && d != "" {
		classDateStr = d
	}
	if classDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", classDateStr)
		if err == nil {
			schedule.ClassDate = parsedDate
		}
	}

	if startTime, ok := updates["startTime"].(string); ok {
		schedule.StartTime = formatTime(startTime)
	} else if startTime, ok := updates["start_time"].(string); ok {
		schedule.StartTime = formatTime(startTime)
	}

	if endTime, ok := updates["endTime"].(string); ok {
		schedule.EndTime = formatTime(endTime)
	} else if endTime, ok := updates["end_time"].(string); ok {
		schedule.EndTime = formatTime(endTime)
	}

	if classStatus, ok := updates["classStatus"].(string); ok {
		schedule.ClassStatus = classStatus
	} else if classStatus, ok := updates["class_status"].(string); ok {
		schedule.ClassStatus = classStatus
	}

	if reason, ok := updates["rescheduleReason"].(string); ok {
		schedule.RescheduleReason = &reason
	} else if reason, ok := updates["reschedule_reason"].(string); ok {
		schedule.RescheduleReason = &reason
	}

	if notesURL, ok := updates["classNotesUrl"].(string); ok {
		schedule.ClassNotesURL = &notesURL
	} else if notesURL, ok := updates["class_notes_url"].(string); ok {
		schedule.ClassNotesURL = &notesURL
	}

	if recordedURL, ok := updates["recordedClassUrl"].(string); ok {
		schedule.RecordedClassURL = &recordedURL
	} else if recordedURL, ok := updates["recorded_class_url"].(string); ok {
		schedule.RecordedClassURL = &recordedURL
	}
	if descVal, ok := updates["description"]; ok {
		if desc, ok := descVal.(string); ok {
			schedule.Description = &desc
		}
	}
	if subjectIDRaw, ok := updates["subject"]; ok {
		subID := u.toInt64(subjectIDRaw)
		if subID > 0 {
			schedule.SubjectID = &subID
		} else {
			schedule.SubjectID = nil
		}
	}

	if err := u.repo.UpdateClassSchedule(ctx, schedule); err != nil {
		return nil, err
	}

	_ = u.repo.LogTeacherActivity(ctx, teacherID, "Updated", "Class Session", fmt.Sprintf("%s (Status: %s)", schedule.TopicName, schedule.ClassStatus))
	u.invalidateCache(ctx, teacherID)
	u.invalidateNotesCache(ctx)

	fullSched, _ := u.repo.GetClassScheduleByID(ctx, classID)
	if fullSched != nil {
		return u.serializeSchedule(ctx, *fullSched), nil
	}
	return u.serializeSchedule(ctx, *schedule), nil
}

func (u *teacherUsecase) DeleteClass(ctx context.Context, teacherID, classID int64) error {
	schedule, err := u.repo.GetClassScheduleByID(ctx, classID)
	if err != nil {
		return err
	}
	if schedule == nil {
		return errors.New("class schedule not found")
	}

	if schedule.TeacherID != teacherID {
		return errors.New("you do not have permission to access or modify this class schedule")
	}

	topicName := schedule.TopicName
	if err := u.repo.DeleteClassSchedule(ctx, classID); err != nil {
		return err
	}

	_ = u.repo.LogTeacherActivity(ctx, teacherID, "Deleted", "Class Session", topicName)
	u.invalidateCache(ctx, teacherID)
	u.invalidateNotesCache(ctx)
	return nil
}

func (u *teacherUsecase) UploadNote(ctx context.Context, teacherID int64, batchID, title, fileURL, recordedClassURL, subject, topic, prerequisiteURL, description string) (map[string]interface{}, error) {
	course, err := u.repo.GetCourseByBatchID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	
	// Verify authorization
	if course != nil {
		isAssigned := false
		if course.TeacherID != nil && *course.TeacherID == teacherID {
			isAssigned = true
		} else {
			for _, t := range course.Teachers {
				if t.ID == teacherID {
					isAssigned = true
					break
				}
			}
		}
		if !isAssigned {
			return nil, errors.New("you are not authorized to upload notes for this course batch")
		}
	}

	note := &domain.Note{
		Title:            title,
		Description:      description,
		NoteType:         "class",
		IsFree:           false,
		Price:            0.0,
		BatchID:          batchID,
		FileURL:          fileURL,
		RecordedClassURL: recordedClassURL,
		Subject:          subject,
		Topic:            topic,
		PrerequisiteURL:  prerequisiteURL,
		CreatedAt:        time.Now(),
	}

	if course != nil {
		note.CourseID = &course.ID
		note.Category = course.Category
	} else {
		note.Category = "CSE(Graduation)"
	}

	if err := u.repo.CreateNote(ctx, note); err != nil {
		return nil, err
	}

	_ = u.repo.LogTeacherActivity(ctx, teacherID, "Uploaded", "Note/Study Material", fmt.Sprintf("%s for Batch %s", title, batchID))
	u.invalidateCache(ctx, teacherID)
	u.invalidateNotesCache(ctx)

	return map[string]interface{}{
		"id":               note.ID,
		"title":            note.Title,
		"batchId":          note.BatchID,
		"fileUrl":          note.FileURL,
		"recordedClassUrl": note.RecordedClassURL,
		"prerequisiteUrl":  note.PrerequisiteURL,
		"createdAt":        note.CreatedAt,
	}, nil
}

func (u *teacherUsecase) invalidateCache(ctx context.Context, teacherID int64) {
	if u.rdb == nil {
		return
	}
	patterns := []string{
		fmt.Sprintf("teacher_overview_%d*", teacherID),
		fmt.Sprintf("teacher_batches_%d*", teacherID),
		fmt.Sprintf("teacher_classes_%d*", teacherID),
	}
	for _, pattern := range patterns {
		var cursor uint64
		for {
			keys, nextCursor, err := u.rdb.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				u.rdb.Del(ctx, keys...)
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}
}

func (u *teacherUsecase) invalidateNotesCache(ctx context.Context) {
	if u.rdb == nil {
		return
	}
	patterns := []string{
		"notes_list*",
		"class_notes_list*",
		"note_detail*",
	}
	for _, pattern := range patterns {
		var cursor uint64
		for {
			keys, nextCursor, err := u.rdb.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				u.rdb.Del(ctx, keys...)
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}
}

func ensureAbsoluteURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	trimmed := strings.TrimSpace(urlStr)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return "https://" + trimmed
	}
	return trimmed
}

// Helpers
func (u *teacherUsecase) serializeSchedule(ctx context.Context, s domain.ClassSchedule) map[string]interface{} {
	subjectName := "No Subject Assigned"
	meetingLink := ensureAbsoluteURL(s.Course.MeetingLink)
	
	if s.SubjectID != nil {
		sub, _ := u.repo.GetSubjectByID(ctx, *s.SubjectID)
		if sub != nil {
			subjectName = sub.SubjectName
			if sub.MeetingLink != "" {
				meetingLink = sub.MeetingLink
			}
		}
	}

	if s.Course.ID != 0 {
		subs, _ := u.repo.GetSubjectsForCourse(ctx, s.Course.ID)
		if len(subs) > 0 {
			if subjectName == "No Subject Assigned" {
				subjectName = subs[0].SubjectName
			}
			
			// Smart meeting link resolution: match topic_name against subjects
			if meetingLink == "" || meetingLink == s.Course.MeetingLink {
				topicLower := strings.ToLower(s.TopicName)
				var matchedSubLink string
				var matchedSubName string
				
				for _, sub := range subs {
					subNameLower := strings.ToLower(sub.SubjectName)
					if subNameLower != "" && (strings.Contains(topicLower, subNameLower) || strings.Contains(subNameLower, topicLower)) {
						if sub.MeetingLink != "" {
							matchedSubLink = sub.MeetingLink
							matchedSubName = sub.SubjectName
							break
						}
					}
				}
				
				// Check for common acronyms like AI / ML
				if matchedSubLink == "" {
					if strings.Contains(topicLower, "machine learning") || strings.Contains(topicLower, " ml ") || strings.HasPrefix(topicLower, "ml ") || strings.HasSuffix(topicLower, " ml") || topicLower == "ml" {
						for _, sub := range subs {
							subNameLower := strings.ToLower(sub.SubjectName)
							if subNameLower == "ml" || strings.Contains(subNameLower, "machine learning") {
								if sub.MeetingLink != "" {
									matchedSubLink = sub.MeetingLink
									matchedSubName = sub.SubjectName
									break
								}
							}
						}
					} else if strings.Contains(topicLower, "artificial intelligence") || strings.Contains(topicLower, " ai ") || strings.HasPrefix(topicLower, "ai ") || strings.HasSuffix(topicLower, " ai") || topicLower == "ai" {
						for _, sub := range subs {
							subNameLower := strings.ToLower(sub.SubjectName)
							if subNameLower == "ai" || strings.Contains(subNameLower, "artificial intelligence") {
								if sub.MeetingLink != "" {
									matchedSubLink = sub.MeetingLink
									matchedSubName = sub.SubjectName
									break
								}
							}
						}
					}
				}

				if matchedSubLink != "" {
					meetingLink = matchedSubLink
					if s.SubjectID == nil && matchedSubName != "" {
						subjectName = matchedSubName
					}
				} else if meetingLink == "" {
					// Fallback to first non-empty subject link
					for _, sub := range subs {
						if sub.MeetingLink != "" {
							meetingLink = sub.MeetingLink
							break
						}
					}
				}
			}
		}
	}

	rescheduleReason := ""
	if s.RescheduleReason != nil {
		rescheduleReason = *s.RescheduleReason
	}
	classNotesURL := ""
	if s.ClassNotesURL != nil {
		classNotesURL = *s.ClassNotesURL
	}
	recordedClassURL := ""
	if s.RecordedClassURL != nil {
		recordedClassURL = *s.RecordedClassURL
	}

	return map[string]interface{}{
		"id":                 s.ID,
		"course":             s.CourseID,
		"course_name":        s.Course.CourseName,
		"subject":            s.SubjectID,
		"subject_name":       subjectName,
		"batch_id":           s.BatchID,
		"topic_name":         s.TopicName,
		"class_date":         s.ClassDate.Format("2006-01-02"),
		"start_time":         s.StartTime,
		"end_time":           s.EndTime,
		"class_status":       s.ClassStatus,
		"reschedule_reason":  rescheduleReason,
		"class_notes_url":    classNotesURL,
		"recorded_class_url": recordedClassURL,
		"teacher":            s.TeacherID,
		"teacher_name":       s.Teacher.Name,
		"meeting_link":       ensureAbsoluteURL(meetingLink),
	}
}

func (u *teacherUsecase) buildTaskSchedules(ctx context.Context, teacher *domain.Teacher, category string) []map[string]interface{} {
	var taskSchedules []map[string]interface{}
	if teacher.Tasks == "" || teacher.Tasks == "[]" {
		return taskSchedules
	}

	type TaskItem struct {
		Batch     string `json:"batch"`
		Course    string `json:"course"`
		Task      string `json:"task"`
		Schedules []struct {
			Date string `json:"date"`
			Time string `json:"time"`
		} `json:"schedules"`
	}

	var tasks []TaskItem
	if err := json.Unmarshal([]byte(teacher.Tasks), &tasks); err != nil {
		return taskSchedules
	}

	courses, _ := u.repo.GetCoursesByTeacher(ctx, teacher.ID, "")

	for tIdx, taskItem := range tasks {
		batchID := taskItem.Batch
		courseName := taskItem.Course
		topicName := taskItem.Task

		// Resolve Course Object
		var courseObj *domain.Course
		for i := range courses {
			if courses[i].BatchID == batchID {
				courseObj = &courses[i]
				break
			}
		}
		if courseObj == nil && courseName != "" {
			for i := range courses {
				if strings.ToLower(courses[i].CourseName) == strings.ToLower(courseName) {
					courseObj = &courses[i]
					break
				}
			}
		}

		if courseObj == nil {
			continue
		}

		// Filter category if active
		if category != "" && strings.ToLower(courseObj.Category) != strings.ToLower(category) {
			continue
		}

		for sIdx, sched := range taskItem.Schedules {
			dateStr := sched.Date
			timeStr := sched.Time
			if timeStr == "" {
				timeStr = "10:00"
			}

			startTime := timeStr
			if len(startTime) == 5 {
				startTime = startTime + ":00"
			}

			// End time calculation
			endTime := startTime
			tObj, err := time.Parse("15:04", timeStr)
			if err == nil {
				endTime = tObj.Add(2 * time.Hour).Format("15:04:05")
			}

			subjectName := courseName
			meetingLink := courseObj.MeetingLink
			subs, _ := u.repo.GetSubjectsForCourse(ctx, courseObj.ID)
			
			topicLower := strings.ToLower(topicName)
			var matchedSub *domain.Subject
			
			for _, sub := range subs {
				subLower := strings.ToLower(sub.SubjectName)
				if subLower != "" && (strings.Contains(topicLower, subLower) || strings.Contains(subLower, topicLower)) {
					matchedSub = &sub
					break
				}
			}
			
			if matchedSub == nil {
				if strings.Contains(topicLower, "machine learning") || strings.Contains(topicLower, " ml ") || strings.HasPrefix(topicLower, "ml ") || strings.HasSuffix(topicLower, " ml") || topicLower == "ml" {
					for _, sub := range subs {
						subNameLower := strings.ToLower(sub.SubjectName)
						if subNameLower == "ml" || strings.Contains(subNameLower, "machine learning") {
							matchedSub = &sub
							break
						}
					}
				} else if strings.Contains(topicLower, "artificial intelligence") || strings.Contains(topicLower, " ai ") || strings.HasPrefix(topicLower, "ai ") || strings.HasSuffix(topicLower, " ai") || topicLower == "ai" {
					for _, sub := range subs {
						subNameLower := strings.ToLower(sub.SubjectName)
						if subNameLower == "ai" || strings.Contains(subNameLower, "artificial intelligence") {
							matchedSub = &sub
							break
						}
					}
				}
			}

			if matchedSub != nil {
				subjectName = matchedSub.SubjectName
				if matchedSub.MeetingLink != "" {
					meetingLink = matchedSub.MeetingLink
				}
			} else {
				if subjectName == "" {
					if len(subs) > 0 {
						subjectName = subs[0].SubjectName
					} else {
						subjectName = "No Subject Assigned"
					}
				}
				if meetingLink == "" && len(subs) > 0 {
					for _, sub := range subs {
						if sub.MeetingLink != "" {
							meetingLink = sub.MeetingLink
							break
						}
					}
				}
			}

			taskSchedules = append(taskSchedules, map[string]interface{}{
				"id":                 fmt.Sprintf("task-%d-%d", tIdx, sIdx),
				"course":             courseObj.ID,
				"course_name":        courseObj.CourseName,
				"subject":            nil,
				"subject_name":       subjectName,
				"batch_id":           batchID,
				"topic_name":         topicName,
				"class_date":         dateStr,
				"start_time":         startTime,
				"end_time":           endTime,
				"class_status":       "pending",
				"reschedule_reason":  "",
				"class_notes_url":    "",
				"recorded_class_url": "",
				"teacher":            teacher.ID,
				"teacher_name":       teacher.Name,
				"meeting_link":       ensureAbsoluteURL(meetingLink),
			})
		}
	}
	return taskSchedules
}


func (u *teacherUsecase) sortSchedules(schedules []map[string]interface{}) {
	normalize := func(tStr string) string {
		if len(tStr) > 5 {
			return tStr[:5]
		}
		return tStr
	}

	// Simple bubble sort or stable sort
	for i := 0; i < len(schedules); i++ {
		for j := i + 1; j < len(schedules); j++ {
			dateI := u.getString(schedules[i]["class_date"])
			timeI := u.getString(schedules[i]["start_time"])
			dateJ := u.getString(schedules[j]["class_date"])
			timeJ := u.getString(schedules[j]["start_time"])

			normI := normalize(timeI)
			normJ := normalize(timeJ)

			if dateI > dateJ || (dateI == dateJ && normI > normJ) {
				schedules[i], schedules[j] = schedules[j], schedules[i]
			} else if dateI == dateJ && normI == normJ {
				// If date and normalized time are identical, prefer the one with a non-zero ID (actual database schedule over virtual task)
				idI := u.toInt64(schedules[i]["id"])
				idJ := u.toInt64(schedules[j]["id"])
				if idI == 0 && idJ > 0 {
					schedules[i], schedules[j] = schedules[j], schedules[i]
				}
			}
		}
	}
}

func (u *teacherUsecase) deduplicateSchedules(schedules []map[string]interface{}) []map[string]interface{} {
	seen := make(map[string]int) // maps signature -> index in deduplicated
	var deduplicated []map[string]interface{}
	for _, s := range schedules {
		classDate := u.getString(s["class_date"])
		startTime := u.getString(s["start_time"])
		if len(startTime) > 5 {
			startTime = startTime[:5]
		}
		batchID := strings.TrimSpace(strings.ToLower(u.getString(s["batch_id"])))
		sig := batchID + "|" + classDate + "|" + startTime
		
		if idx, found := seen[sig]; found {
			existing := deduplicated[idx]
			existingBatch := u.getString(existing["batch_id"])
			newBatch := u.getString(s["batch_id"])
			if existingBatch != "" && newBatch != "" && !strings.Contains(existingBatch, newBatch) {
				existing["batch_id"] = existingBatch + " / " + newBatch
				
				existingCourse := u.getString(existing["courseName"])
				newCourse := u.getString(s["courseName"])
				if existingCourse != "" && newCourse != "" && !strings.Contains(existingCourse, newCourse) {
					existing["courseName"] = existingCourse + " / " + newCourse
				}
				deduplicated[idx] = existing
			}
		} else {
			seen[sig] = len(deduplicated)
			deduplicated = append(deduplicated, s)
		}
	}
	return deduplicated
}

func (u *teacherUsecase) getString(val interface{}) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func (u *teacherUsecase) toInt64(val interface{}) int64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func (u *teacherUsecase) GetCategories(ctx context.Context, teacherID int64) ([]string, error) {
	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	var list []string
	parts := strings.Split(teacher.Category, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list, nil
}

func (u *teacherUsecase) SendNotice(ctx context.Context, teacherID int64, batchID, message string) (map[string]interface{}, error) {
	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	if strings.TrimSpace(message) == "" {
		return nil, errors.New("notice message cannot be empty")
	}

	var courses []domain.Course
	if batchID != "" && strings.ToLower(batchID) != "all" {
		course, err := u.repo.GetCourseByBatchID(ctx, batchID)
		if err != nil {
			return nil, err
		}
		if course == nil {
			return nil, errors.New("batch not found")
		}
		// Verify if this teacher is assigned to this course
		isAssigned := false
		if course.TeacherID != nil && *course.TeacherID == teacher.ID {
			isAssigned = true
		} else {
			assignedCourses, err := u.repo.GetCoursesByTeacher(ctx, teacherID, "")
			if err == nil {
				for _, ac := range assignedCourses {
					if ac.ID == course.ID {
						isAssigned = true
						break
					}
				}
			}
		}
		if !isAssigned {
			return nil, errors.New("you are not authorized to send a notice to this batch")
		}
		courses = append(courses, *course)
	} else {
		// Get all courses assigned to this teacher
		courses, err = u.repo.GetCoursesByTeacher(ctx, teacherID, "")
		if err != nil {
			return nil, err
		}
	}

	if len(courses) == 0 {
		return nil, errors.New("no assigned courses/batches found to send notice to")
	}

	// Get student enrollments for these courses
	var courseIDs []int64
	for _, c := range courses {
		courseIDs = append(courseIDs, c.ID)
	}

	enrollments, err := u.repo.GetEnrollmentsByCourses(ctx, courseIDs)
	if err != nil {
		return nil, err
	}

	// Broadcast to unique users
	recipientUserIDs := make(map[int64]bool)
	for _, e := range enrollments {
		if e.Student.UserID != 0 {
			recipientUserIDs[e.Student.UserID] = true
		}
	}

	if len(recipientUserIDs) == 0 {
		return map[string]interface{}{"message": "Notice created, but no students are enrolled in these batches yet.", "sent_count": 0}, nil
	}

	// Create notifications
	sentCount := 0
	for uID := range recipientUserIDs {
		notif := &domain.UserNotification{
			RecipientID:      uID,
			SenderID:         &teacher.ID,
			IsRead:           false,
			NotificationType: "notice",
			Message:          fmt.Sprintf("Notice from %s: %s", teacher.Name, message),
			RecipientRole:    "student",
			CreatedAt:        time.Now(),
		}
		err = u.repo.CreateNotification(ctx, notif)
		if err == nil {
			sentCount++
		}
	}

	// Log teacher activity
	batchLabel := batchID
	if batchID == "" || strings.ToLower(batchID) == "all" {
		batchLabel = "all assigned batches"
	}
	_ = u.repo.LogTeacherActivity(ctx, teacherID, "published a notice", "Notice", batchLabel)

	return map[string]interface{}{
		"message":    fmt.Sprintf("Notice successfully sent to %d student(s).", sentCount),
		"sent_count": sentCount,
	}, nil
}

func (u *teacherUsecase) UpdateTaskClass(ctx context.Context, teacherID int64, taskID string, updates map[string]interface{}) (map[string]interface{}, error) {
	// 1. Fetch teacher profile
	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	// 2. Parse task-tIdx-sIdx
	parts := strings.Split(taskID, "-")
	if len(parts) < 3 || parts[0] != "task" {
		return nil, errors.New("invalid task schedule format")
	}
	tIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, errors.New("invalid task index")
	}
	sIdx, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, errors.New("invalid schedule index")
	}

	// 3. Unmarshal teacher.Tasks JSON
	if teacher.Tasks == "" || teacher.Tasks == "[]" {
		return nil, errors.New("teacher tasks list is empty")
	}

	type TaskItem struct {
		Batch     string `json:"batch"`
		Course    string `json:"course"`
		Task      string `json:"task"`
		Schedules []struct {
			Date string `json:"date"`
			Time string `json:"time"`
		} `json:"schedules"`
	}

	var tasks []TaskItem
	if err := json.Unmarshal([]byte(teacher.Tasks), &tasks); err != nil {
		return nil, errors.New("failed to parse teacher tasks")
	}

	if tIdx < 0 || tIdx >= len(tasks) {
		return nil, errors.New("task index out of bounds")
	}
	taskItem := tasks[tIdx]
	if sIdx < 0 || sIdx >= len(taskItem.Schedules) {
		return nil, errors.New("schedule index out of bounds")
	}
	taskSched := taskItem.Schedules[sIdx]

	// 4. Resolve course matching batchID
	courses, err := u.repo.GetCoursesByTeacher(ctx, teacherID, "")
	if err != nil {
		return nil, err
	}
	var courseObj *domain.Course
	for i := range courses {
		if courses[i].BatchID == taskItem.Batch {
			courseObj = &courses[i]
			break
		}
	}
	if courseObj == nil && taskItem.Course != "" {
		for i := range courses {
			if strings.ToLower(courses[i].CourseName) == strings.ToLower(taskItem.Course) {
				courseObj = &courses[i]
				break
			}
		}
	}
	if courseObj == nil {
		return nil, errors.New("could not resolve course/batch for this task schedule")
	}

	// Calculate end time helper (2 hours after start_time)
	originalTimeStr := taskSched.Time
	if originalTimeStr == "" {
		originalTimeStr = "10:00"
	}
	originalStartTime := originalTimeStr
	if len(originalStartTime) == 5 {
		originalStartTime = originalStartTime + ":00"
	}
	originalEndTime := originalStartTime
	tObj, err := time.Parse("15:04", originalTimeStr)
	if err == nil {
		originalEndTime = tObj.Add(2 * time.Hour).Format("15:04:05")
	}

	// 5. Check requested classStatus
	var classStatus string
	if status, ok := updates["classStatus"].(string); ok && status != "" {
		classStatus = status
	} else if status, ok := updates["class_status"].(string); ok && status != "" {
		classStatus = status
	}

	if classStatus == "" {
		return nil, errors.New("class status is required for updates")
	}

	originalDate, err := time.Parse("2006-01-02", taskSched.Date)
	if err != nil {
		return nil, errors.New("invalid task original date format")
	}

	var notesURL *string
	if n, ok := updates["classNotesUrl"].(string); ok {
		notesURL = &n
	} else if n, ok := updates["class_notes_url"].(string); ok {
		notesURL = &n
	}

	var recordedURL *string
	if r, ok := updates["recordedClassUrl"].(string); ok {
		recordedURL = &r
	} else if r, ok := updates["recorded_class_url"].(string); ok {
		recordedURL = &r
	}

	var description *string
	if d, ok := updates["description"].(string); ok {
		description = &d
	}

	var subjectID *int64
	if s, ok := updates["subject"]; ok && s != nil {
		subID := u.toInt64(s)
		if subID > 0 {
			subjectID = &subID
		}
	}

	if subjectID == nil && courseObj != nil {
		subs, _ := u.repo.GetSubjectsForCourse(ctx, courseObj.ID)
		topicLower := strings.ToLower(taskItem.Task)
		for _, sub := range subs {
			subLower := strings.ToLower(sub.SubjectName)
			if subLower != "" && (strings.Contains(topicLower, subLower) || strings.Contains(subLower, topicLower)) {
				subjectID = &sub.ID
				break
			}
		}
		
		if subjectID == nil {
			if strings.Contains(topicLower, "machine learning") || strings.Contains(topicLower, " ml ") || strings.HasPrefix(topicLower, "ml ") || strings.HasSuffix(topicLower, " ml") || topicLower == "ml" {
				for _, sub := range subs {
					subNameLower := strings.ToLower(sub.SubjectName)
					if subNameLower == "ml" || strings.Contains(subNameLower, "machine learning") {
						subjectID = &sub.ID
						break
					}
				}
			} else if strings.Contains(topicLower, "artificial intelligence") || strings.Contains(topicLower, " ai ") || strings.HasPrefix(topicLower, "ai ") || strings.HasSuffix(topicLower, " ai") || topicLower == "ai" {
				for _, sub := range subs {
					subNameLower := strings.ToLower(sub.SubjectName)
					if subNameLower == "ai" || strings.Contains(subNameLower, "artificial intelligence") {
						subjectID = &sub.ID
						break
					}
				}
			}
		}
	}

	// Check if there is an active (non-cancelled) schedule for this teacher/course/date/time slot
	existing, err := u.repo.GetActiveScheduleBySlot(ctx, teacherID, courseObj.ID, originalDate, originalStartTime)
	if err == nil && existing != nil {
		// An active schedule exists! Let's update it instead of creating a duplicate row.
		if classStatus == "rescheduled" {
			// Update existing schedule to cancelled
			var reschedDateStr, reschedTimeStr, reschedReasonStr string
			if d, ok := updates["classDate"].(string); ok {
				reschedDateStr = d
			} else if d, ok := updates["class_date"].(string); ok {
				reschedDateStr = d
			}
			if t, ok := updates["startTime"].(string); ok {
				reschedTimeStr = t
			} else if t, ok := updates["start_time"].(string); ok {
				reschedTimeStr = t
			}
			if r, ok := updates["rescheduleReason"].(string); ok {
				reschedReasonStr = r
			} else if r, ok := updates["reschedule_reason"].(string); ok {
				reschedReasonStr = r
			}
			origReason := fmt.Sprintf("Rescheduled to %s %s: %s", reschedDateStr, reschedTimeStr, reschedReasonStr)

			existing.ClassStatus = "cancelled"
			existing.RescheduleReason = &origReason
			if err := u.repo.UpdateClassSchedule(ctx, existing); err != nil {
				return nil, err
			}

			// We need to parse rescheduled date/time/reason to create the new slot
			var newDateStr string
			if d, ok := updates["classDate"].(string); ok && d != "" {
				newDateStr = d
			} else if d, ok := updates["class_date"].(string); ok && d != "" {
				newDateStr = d
			}
			if newDateStr == "" {
				return nil, errors.New("reschedule date is required")
			}
			newDate, err := time.Parse("2006-01-02", newDateStr)
			if err != nil {
				return nil, errors.New("invalid reschedule date format")
			}

			var newStartTime string
			if t, ok := updates["startTime"].(string); ok && t != "" {
				newStartTime = t
			} else if t, ok := updates["start_time"].(string); ok && t != "" {
				newStartTime = t
			}
			if newStartTime == "" {
				newStartTime = "10:00"
			}
			if len(newStartTime) == 5 {
				newStartTime = newStartTime + ":00"
			}

			var newEndTime string
			if t, ok := updates["endTime"].(string); ok && t != "" {
				newEndTime = t
			} else if t, ok := updates["end_time"].(string); ok && t != "" {
				newEndTime = t
			}
			if newEndTime == "" {
				newEndTime = newStartTime
				tObj, err := time.Parse("15:04", newStartTime[:5])
				if err == nil {
					newEndTime = tObj.Add(2 * time.Hour).Format("15:04:05")
				}
			}
			if len(newEndTime) == 5 {
				newEndTime = newEndTime + ":00"
			}

			var reason string
			if r, ok := updates["rescheduleReason"].(string); ok {
				reason = r
			} else if r, ok := updates["reschedule_reason"].(string); ok {
				reason = r
			}

			// Create rescheduled active slot as PENDING in DB
			newSched := &domain.ClassSchedule{
				CourseID:         courseObj.ID,
				SubjectID:        subjectID,
				BatchID:          courseObj.BatchID,
				TeacherID:        teacherID,
				TopicName:        taskItem.Task,
				ClassDate:        newDate,
				StartTime:        newStartTime,
				EndTime:          newEndTime,
				ClassStatus:      "pending",
				RescheduleReason: &reason,
				CreatedAt:        time.Now(),
			}
			if err := u.repo.CreateClassSchedule(ctx, newSched); err != nil {
				return nil, err
			}

			_ = u.repo.LogTeacherActivity(ctx, teacherID, "Rescheduled", "Class Session", fmt.Sprintf("%s to %s", taskItem.Task, newDateStr))
			u.invalidateCache(ctx, teacherID)
			
			fullSched, _ := u.repo.GetClassScheduleByID(ctx, newSched.ID)
			if fullSched != nil {
				return u.serializeSchedule(ctx, *fullSched), nil
			}
			return u.serializeSchedule(ctx, *newSched), nil

		} else if classStatus == "completed" {
			existing.ClassStatus = "completed"
			existing.ClassNotesURL = notesURL
			existing.RecordedClassURL = recordedURL
			existing.Description = description
			if err := u.repo.UpdateClassSchedule(ctx, existing); err != nil {
				return nil, err
			}

			_ = u.repo.LogTeacherActivity(ctx, teacherID, "Completed", "Class Session", taskItem.Task)
			u.invalidateCache(ctx, teacherID)
			u.invalidateNotesCache(ctx)

			fullSched, _ := u.repo.GetClassScheduleByID(ctx, existing.ID)
			if fullSched != nil {
				return u.serializeSchedule(ctx, *fullSched), nil
			}
			return u.serializeSchedule(ctx, *existing), nil

		} else if classStatus == "cancelled" {
			var reason string
			if r, ok := updates["rescheduleReason"].(string); ok {
				reason = r
			} else if r, ok := updates["reschedule_reason"].(string); ok {
				reason = r
			}

			existing.ClassStatus = "cancelled"
			existing.RescheduleReason = &reason
			if err := u.repo.UpdateClassSchedule(ctx, existing); err != nil {
				return nil, err
			}

			_ = u.repo.LogTeacherActivity(ctx, teacherID, "Cancelled", "Class Session", taskItem.Task)
			u.invalidateCache(ctx, teacherID)

			fullSched, _ := u.repo.GetClassScheduleByID(ctx, existing.ID)
			if fullSched != nil {
				return u.serializeSchedule(ctx, *fullSched), nil
			}
			return u.serializeSchedule(ctx, *existing), nil
		}
	}

	// 6. Handle action based on status
	if classStatus == "rescheduled" {
		// We need to parse rescheduled date/time/reason
		var newDateStr string
		if d, ok := updates["classDate"].(string); ok && d != "" {
			newDateStr = d
		} else if d, ok := updates["class_date"].(string); ok && d != "" {
			newDateStr = d
		}
		if newDateStr == "" {
			return nil, errors.New("reschedule date is required")
		}
		newDate, err := time.Parse("2006-01-02", newDateStr)
		if err != nil {
			return nil, errors.New("invalid reschedule date format")
		}

		var newStartTime string
		if t, ok := updates["startTime"].(string); ok && t != "" {
			newStartTime = t
		} else if t, ok := updates["start_time"].(string); ok && t != "" {
			newStartTime = t
		}
		if newStartTime == "" {
			newStartTime = "10:00"
		}
		if len(newStartTime) == 5 {
			newStartTime = newStartTime + ":00"
		}

		var newEndTime string
		if t, ok := updates["endTime"].(string); ok && t != "" {
			newEndTime = t
		} else if t, ok := updates["end_time"].(string); ok && t != "" {
			newEndTime = t
		}
		if newEndTime == "" {
			newEndTime = newStartTime
			tObj, err := time.Parse("15:04", newStartTime[:5])
			if err == nil {
				newEndTime = tObj.Add(2 * time.Hour).Format("15:04:05")
			}
		}
		if len(newEndTime) == 5 {
			newEndTime = newEndTime + ":00"
		}

		var reason string
		if r, ok := updates["rescheduleReason"].(string); ok {
			reason = r
		} else if r, ok := updates["reschedule_reason"].(string); ok {
			reason = r
		}

		// Create original slot as RESCHEDULED in DB
		origReason := fmt.Sprintf("Rescheduled to %s %s: %s", newDateStr, newStartTime[:5], reason)
		origSched := &domain.ClassSchedule{
			CourseID:         courseObj.ID,
			SubjectID:        subjectID,
			BatchID:          courseObj.BatchID,
			TeacherID:        teacherID,
			TopicName:        taskItem.Task,
			ClassDate:        originalDate,
			StartTime:        originalStartTime,
			EndTime:          originalEndTime,
			ClassStatus:      "cancelled",
			RescheduleReason: &origReason,
			CreatedAt:        time.Now(),
		}
		if err := u.repo.CreateClassSchedule(ctx, origSched); err != nil {
			return nil, err
		}

		// Create rescheduled active slot as PENDING in DB
		newSched := &domain.ClassSchedule{
			CourseID:         courseObj.ID,
			SubjectID:        subjectID,
			BatchID:          courseObj.BatchID,
			TeacherID:        teacherID,
			TopicName:        taskItem.Task,
			ClassDate:        newDate,
			StartTime:        newStartTime,
			EndTime:          newEndTime,
			ClassStatus:      "pending",
			RescheduleReason: &reason,
			CreatedAt:        time.Now(),
		}
		if err := u.repo.CreateClassSchedule(ctx, newSched); err != nil {
			return nil, err
		}

		_ = u.repo.LogTeacherActivity(ctx, teacherID, "Rescheduled", "Class Session", fmt.Sprintf("%s to %s", taskItem.Task, newDateStr))
		u.invalidateCache(ctx, teacherID)
		
		fullSched, _ := u.repo.GetClassScheduleByID(ctx, newSched.ID)
		if fullSched != nil {
			return u.serializeSchedule(ctx, *fullSched), nil
		}
		return u.serializeSchedule(ctx, *newSched), nil

	} else if classStatus == "completed" {
		// Create completed record in DB
		sched := &domain.ClassSchedule{
			CourseID:         courseObj.ID,
			SubjectID:        subjectID,
			BatchID:          courseObj.BatchID,
			TeacherID:        teacherID,
			TopicName:        taskItem.Task,
			ClassDate:        originalDate,
			StartTime:        originalStartTime,
			EndTime:          originalEndTime,
			ClassStatus:      "completed",
			ClassNotesURL:    notesURL,
			RecordedClassURL: recordedURL,
			Description:      description,
			CreatedAt:        time.Now(),
		}
		if err := u.repo.CreateClassSchedule(ctx, sched); err != nil {
			return nil, err
		}

		_ = u.repo.LogTeacherActivity(ctx, teacherID, "Completed", "Class Session", taskItem.Task)
		u.invalidateCache(ctx, teacherID)
		u.invalidateNotesCache(ctx)

		fullSched, _ := u.repo.GetClassScheduleByID(ctx, sched.ID)
		if fullSched != nil {
			return u.serializeSchedule(ctx, *fullSched), nil
		}
		return u.serializeSchedule(ctx, *sched), nil

	} else if classStatus == "cancelled" {
		var reason string
		if r, ok := updates["rescheduleReason"].(string); ok {
			reason = r
		} else if r, ok := updates["reschedule_reason"].(string); ok {
			reason = r
		}

		// Create cancelled record in DB
		sched := &domain.ClassSchedule{
			CourseID:         courseObj.ID,
			SubjectID:        subjectID,
			BatchID:          courseObj.BatchID,
			TeacherID:        teacherID,
			TopicName:        taskItem.Task,
			ClassDate:        originalDate,
			StartTime:        originalStartTime,
			EndTime:          originalEndTime,
			ClassStatus:      "cancelled",
			RescheduleReason: &reason,
			CreatedAt:        time.Now(),
		}
		if err := u.repo.CreateClassSchedule(ctx, sched); err != nil {
			return nil, err
		}

		_ = u.repo.LogTeacherActivity(ctx, teacherID, "Cancelled", "Class Session", taskItem.Task)
		u.invalidateCache(ctx, teacherID)

		fullSched, _ := u.repo.GetClassScheduleByID(ctx, sched.ID)
		if fullSched != nil {
			return u.serializeSchedule(ctx, *fullSched), nil
		}
		return u.serializeSchedule(ctx, *sched), nil
	}

	return nil, errors.New("unsupported class status update")
}

func formatTime(tStr string) string {
	if len(tStr) == 5 {
		return tStr + ":00"
	}
	return tStr
}
