package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"clasynq/api/admin/internal/domain"
	"clasynq/api/admin/internal/utils"

	"github.com/redis/go-redis/v9"
)

type adminUsecase struct {
	repo             domain.AdminRepository
	rdb              *redis.Client
	smtpHost         string
	smtpPort         string
	smtpUser         string
	smtpPass         string
	defaultFromEmail string
}

func NewAdminUsecase(repo domain.AdminRepository, rdb *redis.Client, smtpHost, smtpPort, smtpUser, smtpPass, defaultFromEmail string) domain.AdminUsecase {
	return &adminUsecase{
		repo:             repo,
		rdb:              rdb,
		smtpHost:         smtpHost,
		smtpPort:         smtpPort,
		smtpUser:         smtpUser,
		smtpPass:         smtpPass,
		defaultFromEmail: defaultFromEmail,
	}
}

func (u *adminUsecase) GetOverview(ctx context.Context) (map[string]interface{}, error) {
	stats, err := u.repo.RefreshDashboardStats(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"stats": map[string]interface{}{
			"totalStudents":   stats.TotalStudents,
			"totalTeachers":   stats.TotalTeacher,
			"activeBatches":   stats.ActiveBatches,
			"monthlyRevenue":  135000, // Matching Django mock
		},
		"deltas": map[string]interface{}{
			"totalStudents":   "+0",
			"totalTeachers":   "+0",
			"activeBatches":   "+0",
			"monthlyRevenue":  "+8.4%",
		},
	}, nil
}

func (u *adminUsecase) GetActivities(ctx context.Context) ([]map[string]interface{}, error) {
	list, err := u.repo.GetActivities(ctx, 30)
	if err != nil {
		return nil, err
	}
	res := make([]map[string]interface{}, len(list))
	for i, act := range list {
		res[i] = map[string]interface{}{
			"id":         act.ID,
			"adminEmail": act.AdminEmail,
			"action":     act.Action,
			"entityType": act.EntityType,
			"entityName": act.EntityName,
			"createdAt":  act.CreatedAt.Format(time.RFC3339),
		}
	}
	return res, nil
}

func (u *adminUsecase) ListTeachers(ctx context.Context, query, category string) (map[string]interface{}, error) {
	list, err := u.repo.ListTeachers(ctx, query, category)
	if err != nil {
		return nil, err
	}

	serializedTeachers := make([]map[string]interface{}, len(list))
	serializedAllRegistered := make([]map[string]interface{}, len(list))

	for i, t := range list {
		var coursesList []string
		_ = json.Unmarshal([]byte(t.AssignedCourses), &coursesList)
		if coursesList == nil {
			coursesList = []string{}
		}

		var tasksList []map[string]interface{}
		_ = json.Unmarshal([]byte(t.Tasks), &tasksList)
		if tasksList == nil {
			tasksList = []map[string]interface{}{}
		}

		serializedTeachers[i] = map[string]interface{}{
			"id":              t.ID,
			"email":           t.Email,
			"name":            t.Name,
			"specialization":  t.Specialization,
			"assignedCourses": coursesList,
			"tasks":           tasksList,
			"photoUrl":        t.PhotoURL,
			"category":        t.Category,
			"dateOfBirth":     t.DateOfBirth,
			"createdAt":       t.CreatedAt,
			"updatedAt":       t.UpdatedAt,
		}

		serializedAllRegistered[i] = map[string]interface{}{
			"id":             t.ID,
			"email":          t.Email,
			"name":           t.Name,
			"specialization": t.Specialization,
			"photoUrl":       t.PhotoURL,
			"category":       t.Category,
		}
	}

	return map[string]interface{}{
		"teachers":               serializedTeachers,
		"all_registered_teachers": serializedAllRegistered,
	}, nil
}

func (u *adminUsecase) CreateTeacher(ctx context.Context, teacher *domain.Teacher) (map[string]interface{}, error) {
	salt := utils.GenerateSalt(12)
	teacher.Password = utils.EncodeDjangoPassword(teacher.Password, salt, 390000)
	teacher.CreatedAt = time.Now()
	teacher.UpdatedAt = time.Now()

	if teacher.AssignedCourses == "" {
		teacher.AssignedCourses = "[]"
	}
	if teacher.Tasks == "" {
		teacher.Tasks = "[]"
	}

	if err := u.repo.CreateTeacher(ctx, teacher); err != nil {
		return nil, err
	}

	// Refresh stats
	_, _ = u.repo.RefreshDashboardStats(ctx)

	var coursesList []string
	_ = json.Unmarshal([]byte(teacher.AssignedCourses), &coursesList)
	var tasksList []map[string]interface{}
	_ = json.Unmarshal([]byte(teacher.Tasks), &tasksList)

	// Trigger notifications
	_ = u.repo.CreateNotification(ctx, teacher.ID, "teacher", "account_created", "Your teacher account has been successfully created by the Admin.")

	if len(coursesList) > 0 {
		_ = u.repo.CreateNotification(ctx, teacher.ID, "teacher", "course_assigned", fmt.Sprintf("You have been assigned %d courses by the Admin.", len(coursesList)))
	}

	if len(tasksList) > 0 {
		_ = u.repo.CreateNotification(ctx, teacher.ID, "teacher", "task_assigned", fmt.Sprintf("You have been assigned %d tasks by the Admin.", len(tasksList)))
	}

	return map[string]interface{}{
		"id":              teacher.ID,
		"email":           teacher.Email,
		"name":            teacher.Name,
		"specialization":  teacher.Specialization,
		"assignedCourses": coursesList,
		"tasks":           tasksList,
		"photoUrl":        teacher.PhotoURL,
		"category":        teacher.Category,
		"dateOfBirth":     teacher.DateOfBirth,
		"createdAt":       teacher.CreatedAt,
		"updatedAt":       teacher.UpdatedAt,
	}, nil
}

func (u *adminUsecase) UpdateTeacher(ctx context.Context, teacherID int64, updates map[string]interface{}) (map[string]interface{}, error) {
	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	if teacher == nil {
		return nil, errors.New("teacher not found")
	}

	if email, ok := updates["email"].(string); ok {
		teacher.Email = strings.ToLower(strings.TrimSpace(email))
	}
	if password, ok := updates["password"].(string); ok && password != "" {
		salt := utils.GenerateSalt(12)
		teacher.Password = utils.EncodeDjangoPassword(password, salt, 390000)
	}
	if name, ok := updates["name"].(string); ok {
		teacher.Name = strings.TrimSpace(name)
	}
	if specialization, ok := updates["specialization"].(string); ok {
		teacher.Specialization = strings.TrimSpace(specialization)
	}
	if category, ok := updates["category"].(string); ok {
		teacher.Category = strings.TrimSpace(category)
	}
	if dobStr, ok := updates["date_of_birth"].(string); ok {
		if dobStr == "" {
			teacher.DateOfBirth = nil
		} else {
			t, err := time.Parse("2006-01-02", dobStr)
			if err == nil {
				teacher.DateOfBirth = &t
			}
		}
	}
	if photoURL, ok := updates["photo_url"].(string); ok && photoURL != "" {
		teacher.PhotoURL = photoURL
	}

	// Backup old tasks
	var oldTasks []map[string]interface{}
	_ = json.Unmarshal([]byte(teacher.Tasks), &oldTasks)

	tasksUpdated := false
	if newTasks, ok := updates["tasks"]; ok {
		raw, err := json.Marshal(newTasks)
		if err == nil {
			teacher.Tasks = string(raw)
			
			// Sync class schedules
			u.syncTeacherTasksSchedules(ctx, teacher, oldTasks, newTasks)
			tasksUpdated = true
		}
	}

	coursesUpdated := false
	if assignedCourses, ok := updates["assigned_courses"]; ok {
		raw, err := json.Marshal(assignedCourses)
		if err == nil {
			teacher.AssignedCourses = string(raw)
			var coursesList []string
			_ = json.Unmarshal(raw, &coursesList)
			
			// Update database relationships
			_ = u.repo.UnassignTeacherFromOldCourses(ctx, teacher.ID, coursesList)
			_ = u.repo.AssignTeacherToCourses(ctx, teacher.ID, coursesList)
			coursesUpdated = true
		}
	}

	teacher.UpdatedAt = time.Now()
	if err := u.repo.UpdateTeacher(ctx, teacher); err != nil {
		return nil, err
	}

	u.invalidateTeacherCache(ctx, teacher.ID)

	_, _ = u.repo.RefreshDashboardStats(ctx)

	// Trigger notifications for updates
	if tasksUpdated {
		var tasksList []map[string]interface{}
		_ = json.Unmarshal([]byte(teacher.Tasks), &tasksList)
		msg := "Your assigned tasks have been updated by the Admin."
		if len(tasksList) > 0 {
			msg = fmt.Sprintf("You have been assigned tasks by the Admin. Total tasks: %d.", len(tasksList))
		}
		_ = u.repo.CreateNotification(ctx, teacher.ID, "teacher", "task_assigned", msg)
	}

	if coursesUpdated {
		var coursesList []string
		_ = json.Unmarshal([]byte(teacher.AssignedCourses), &coursesList)
		msg := "Your assigned courses have been updated by the Admin."
		if len(coursesList) > 0 {
			msg = fmt.Sprintf("You have been assigned courses by the Admin. Total courses: %d.", len(coursesList))
		}
		_ = u.repo.CreateNotification(ctx, teacher.ID, "teacher", "course_assigned", msg)
	}

	var coursesList []string
	_ = json.Unmarshal([]byte(teacher.AssignedCourses), &coursesList)
	var tasksList []map[string]interface{}
	_ = json.Unmarshal([]byte(teacher.Tasks), &tasksList)

	return map[string]interface{}{
		"id":              teacher.ID,
		"email":           teacher.Email,
		"name":            teacher.Name,
		"specialization":  teacher.Specialization,
		"assignedCourses": coursesList,
		"tasks":           tasksList,
		"photoUrl":        teacher.PhotoURL,
		"category":        teacher.Category,
		"dateOfBirth":     teacher.DateOfBirth,
		"createdAt":       teacher.CreatedAt,
		"updatedAt":       teacher.UpdatedAt,
	}, nil
}

func (u *adminUsecase) syncTeacherTasksSchedules(ctx context.Context, teacher *domain.Teacher, oldTasks []map[string]interface{}, newTasks interface{}) {
	// Parse new tasks as slice
	var newTasksSlice []map[string]interface{}
	raw, err := json.Marshal(newTasks)
	if err == nil {
		_ = json.Unmarshal(raw, &newTasksSlice)
	}

	// 1. Build signatures of old schedules
	oldSignatures := make(map[string]bool)
	for _, t := range oldTasks {
		batch := getStringField(t, "batch")
		taskName := getStringField(t, "task")
		schedules, _ := t["schedules"].([]interface{})
		for _, s := range schedules {
			schedMap, ok := s.(map[string]interface{})
			if ok {
				date := getStringField(schedMap, "date")
				timeStr := getStringField(schedMap, "time")
				sig := fmt.Sprintf("%s|%s|%s|%s", batch, taskName, date, timeStr)
				oldSignatures[sig] = true
			}
		}
	}

	// 2. Build signatures of new schedules
	newSignatures := make(map[string]bool)
	for _, t := range newTasksSlice {
		batch := getStringField(t, "batch")
		taskName := getStringField(t, "task")
		schedules, _ := t["schedules"].([]interface{})
		for _, s := range schedules {
			schedMap, ok := s.(map[string]interface{})
			if ok {
				date := getStringField(schedMap, "date")
				timeStr := getStringField(schedMap, "time")
				sig := fmt.Sprintf("%s|%s|%s|%s", batch, taskName, date, timeStr)
				newSignatures[sig] = true
			}
		}
	}

	// 3. Delete old schedules that are missing in the new list
	for sig := range oldSignatures {
		if !newSignatures[sig] {
			parts := strings.Split(sig, "|")
			if len(parts) == 4 {
				batch := parts[0]
				topic := parts[1]
				dateStr := parts[2]
				timeStr := parts[3]

				parsedDate, err := time.Parse("2006-01-02", dateStr)
				if err == nil {
					_ = u.repo.DeleteClassSchedulesBySignature(ctx, teacher.ID, batch, topic, parsedDate, timeStr)
				}
			}
		}
	}

	// 4. Create or update remaining/new schedules
	for _, t := range newTasksSlice {
		batch := getStringField(t, "batch")
		topic := getStringField(t, "task")
		courseName := getStringField(t, "course")

		course, _ := u.repo.GetCourseByBatchID(ctx, batch)
		if course == nil && courseName != "" {
			course, _ = u.repo.GetCourseByName(ctx, courseName)
		}
		if course == nil {
			continue
		}

		schedules, _ := t["schedules"].([]interface{})
		for _, s := range schedules {
			schedMap, ok := s.(map[string]interface{})
			if ok {
				dateStr := getStringField(schedMap, "date")
				timeStr := getStringField(schedMap, "time")

				parsedDate, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					continue
				}

				// Format start_time to HH:MM:SS
				startTime := timeStr
				if len(startTime) == 5 {
					startTime = startTime + ":00"
				}

				// Compute end_time = start_time + 2 hours
				var endTime string
				var dtTime time.Time
				if len(timeStr) == 5 {
					dtTime, err = time.Parse("15:04", timeStr)
				} else {
					dtTime, err = time.Parse("15:04:05", timeStr)
				}
				if err == nil {
					endTime = dtTime.Add(2 * time.Hour).Format("15:04:05")
				} else {
					endTime = startTime
				}

				schedule := &domain.ClassSchedule{
					TeacherID:   teacher.ID,
					CourseID:    course.ID,
					BatchID:     course.BatchID,
					ClassDate:   parsedDate,
					StartTime:   startTime,
					EndTime:     endTime,
					ClassStatus: "pending",
					TopicName:   topic,
				}

				var subjectObj *domain.Subject
				// Just pass subject object search parameters to repository
				_ = u.repo.UpsertClassSchedule(ctx, schedule, topic, subjectObj)
			}
		}
	}
}

func (u *adminUsecase) DeleteTeacher(ctx context.Context, teacherID int64, complete bool, courseName string, adminID int64) error {
	teacher, err := u.repo.GetTeacherByID(ctx, teacherID)
	if err != nil {
		return err
	}
	if teacher == nil {
		return errors.New("teacher not found")
	}

	if complete {
		if err := u.repo.DeleteTeacher(ctx, teacherID); err != nil {
			return err
		}
		_ = u.repo.LogActivity(ctx, adminID, "Deleted", "Teacher", teacher.Name)
	} else {
		// Unassign from courses
		var coursesToKeep []string
		var oldCourses []string
		_ = json.Unmarshal([]byte(teacher.AssignedCourses), &oldCourses)

		for _, c := range oldCourses {
			if courseName != "" && c == courseName {
				continue // skip this course to unassign it
			}
			coursesToKeep = append(coursesToKeep, c)
		}

		_ = u.repo.UnassignTeacherFromOldCourses(ctx, teacher.ID, coursesToKeep)
		
		raw, _ := json.Marshal(coursesToKeep)
		teacher.AssignedCourses = string(raw)
		_ = u.repo.UpdateTeacher(ctx, teacher)

		_ = u.repo.LogActivity(ctx, adminID, "Unassigned", "Teacher from Course", fmt.Sprintf("%s from course(s)", teacher.Name))
	}

	_, _ = u.repo.RefreshDashboardStats(ctx)
	u.invalidateTeacherCache(ctx, teacherID)
	return nil
}

func (u *adminUsecase) ListStudents(ctx context.Context, query, category string) ([]map[string]interface{}, error) {
	list, err := u.repo.ListStudents(ctx, query, category)
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Extract student IDs to fetch enrollments
	studentIDs := make([]int64, len(list))
	for i, s := range list {
		studentIDs[i] = s.ID
	}

	// Fetch courses & batches info in a single batch query
	coursesMap, batchesMap, err := u.repo.GetStudentEnrollmentInfo(ctx, studentIDs)
	if err != nil {
		// Log/fail-safe
		coursesMap = make(map[int64][]string)
		batchesMap = make(map[int64][]string)
	}

	res := make([]map[string]interface{}, len(list))
	for i, s := range list {
		courses := coursesMap[s.ID]
		if courses == nil {
			courses = []string{}
		}
		batches := batchesMap[s.ID]
		if batches == nil {
			batches = []string{}
		}

		res[i] = map[string]interface{}{
			"id":               s.ID,
			"email":            s.User.Email,
			"username":         s.User.Username,
			"fullName":         s.User.FullName,
			"name":             s.User.FullName,      // Frontend expects student.name
			"contactNumber":    s.User.ContactNumber,
			"phoneNumber":      s.User.ContactNumber, // Frontend expects student.phoneNumber
			"avatarUrl":        s.User.AvatarURL,
			"createdAt":        s.CreatedAt.Format(time.RFC3339),
			"purchasedCourses": courses,
			"purchasedBatches": batches,
		}
	}
	return res, nil
}

func (u *adminUsecase) GetSalesAnalysis(ctx context.Context, monthStr, category string) (map[string]interface{}, error) {
	var year, month int
	now := time.Now()
	year = now.Year()
	month = int(now.Month())

	if monthStr != "" {
		parts := strings.Split(monthStr, "-")
		if len(parts) == 2 {
			y, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && m >= 1 && m <= 12 {
				year = y
				month = m
			}
		}
	}

	cacheKey := fmt.Sprintf("admin_sales_analysis_%d_%02d", year, month)
	if category != "" {
		cacheKey += fmt.Sprintf("_cat_%s", category)
	}

	// Cache check
	if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var cached map[string]interface{}
		if err := json.Unmarshal([]byte(val), &cached); err == nil {
			return cached, nil
		}
	}

	// Calendar weeks
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)
	numDays := endOfMonth.Day()

	weeks := [][]int{
		{1, 7},
		{8, 14},
		{15, 21},
		{22, 28},
	}
	if numDays > 28 {
		weeks = append(weeks, []int{29, numDays})
	}

	monthAbbr := startOfMonth.Format("Jan")
	weekLabels := make([]string, len(weeks))
	dateRanges := make([][2]time.Time, len(weeks))

	for i, w := range weeks {
		weekLabels[i] = fmt.Sprintf("Week %d (%s %02d - %s %02d)", i+1, monthAbbr, w[0], monthAbbr, w[1])
		dateRanges[i] = [2]time.Time{
			time.Date(year, time.Month(month), w[0], 0, 0, 0, 0, time.Local),
			time.Date(year, time.Month(month), w[1], 23, 59, 59, 999999, time.Local),
		}
	}

	// 1. Course Sales
	coursesList, err := u.repo.GetCoursesSales(ctx, category, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	coursesData := make([]map[string]interface{}, len(coursesList))
	var coursesTotalEnrollments int64
	var coursesRevenue float64

	for i, c := range coursesList {
		weeklyBreakdown := make([]map[string]interface{}, len(dateRanges))
		var monthlyCourseSales int64
		for wIdx, rng := range dateRanges {
			salesListForWeek, _ := u.repo.GetCoursesSales(ctx, category, rng[0], rng[1])
			var weeklySales int64
			for _, sc := range salesListForWeek {
				if sc.ID == c.ID {
					weeklySales = sc.SalesCount
					break
				}
			}
			weeklyBreakdown[wIdx] = map[string]interface{}{
				"label": weekLabels[wIdx],
				"count": weeklySales,
			}
			monthlyCourseSales += weeklySales
		}

		coursesTotalEnrollments += monthlyCourseSales
		coursesRevenue += float64(monthlyCourseSales) * c.Price

		coursesData[i] = map[string]interface{}{
			"id":                      c.ID,
			"courseName":              c.CourseName,
			"batchId":                 c.BatchID,
			"price":                   c.Price,
			"totalMonthEnrollments":  monthlyCourseSales,
			"weeklyBreakdown":         weeklyBreakdown,
		}
	}

	// 2. Note Sales
	notesList, err := u.repo.GetNotesSales(ctx, category, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	notesData := make([]map[string]interface{}, len(notesList))
	var notesTotalSales int64
	var notesRevenue float64

	for i, n := range notesList {
		weeklyBreakdown := make([]map[string]interface{}, len(dateRanges))
		var monthlyNoteSales int64
		for wIdx, rng := range dateRanges {
			salesListForWeek, _ := u.repo.GetNotesSales(ctx, category, rng[0], rng[1])
			var weeklySales int64
			for _, sn := range salesListForWeek {
				if sn.ID == n.ID {
					weeklySales = sn.SalesCount
					break
				}
			}
			weeklyBreakdown[wIdx] = map[string]interface{}{
				"label": weekLabels[wIdx],
				"count": weeklySales,
			}
			monthlyNoteSales += weeklySales
		}

		notesTotalSales += monthlyNoteSales
		notesRevenue += float64(monthlyNoteSales) * n.Price

		notesData[i] = map[string]interface{}{
			"id":                 n.ID,
			"title":              n.Title,
			"price":              n.Price,
			"totalMonthSales":    monthlyNoteSales,
			"weeklyBreakdown":    weeklyBreakdown,
		}
	}

	// 3. Test Series Sales
	tsList, err := u.repo.GetTestSeriesSales(ctx, category, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	tsData := make([]map[string]interface{}, len(tsList))
	var tsTotalSales int64
	var tsRevenue float64

	for i, ts := range tsList {
		weeklyBreakdown := make([]map[string]interface{}, len(dateRanges))
		var monthlyTsSales int64
		for wIdx, rng := range dateRanges {
			salesListForWeek, _ := u.repo.GetTestSeriesSales(ctx, category, rng[0], rng[1])
			var weeklySales int64
			for _, sts := range salesListForWeek {
				if sts.ID == ts.ID {
					weeklySales = sts.SalesCount
					break
				}
			}
			weeklyBreakdown[wIdx] = map[string]interface{}{
				"label": weekLabels[wIdx],
				"count": weeklySales,
			}
			monthlyTsSales += weeklySales
		}

		tsTotalSales += monthlyTsSales
		tsRevenue += float64(monthlyTsSales) * ts.Price

		tsData[i] = map[string]interface{}{
			"id":                 ts.ID,
			"title":              ts.Title,
			"price":              ts.Price,
			"totalMonthSales":    monthlyTsSales,
			"weeklyBreakdown":    weeklyBreakdown,
		}
	}

	totalRevenue := coursesRevenue + notesRevenue + tsRevenue

	responsePayload := map[string]interface{}{
		"selectedMonth": fmt.Sprintf("%d-%02d", year, month),
		"summary": map[string]interface{}{
			"totalCourseEnrollments": coursesTotalEnrollments,
			"totalNoteSales":          notesTotalSales,
			"totalTestSeriesSales":    tsTotalSales,
			"totalRevenue":            totalRevenue,
		},
		"courses":     coursesData,
		"notes":       notesData,
		"testSeries":  tsData,
	}

	// Cache in Redis for 10 minutes
	if raw, err := json.Marshal(responsePayload); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
	}

	return responsePayload, nil
}

func (u *adminUsecase) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return u.repo.ListCategories(ctx)
}

func (u *adminUsecase) GetCategory(ctx context.Context, id int64) (*domain.Category, error) {
	return u.repo.GetCategoryByID(ctx, id)
}

func (u *adminUsecase) CreateCategory(ctx context.Context, name string) (*domain.Category, error) {
	category := &domain.Category{
		Name:      name,
		CreatedAt: time.Now(),
	}
	if err := u.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	// Invalidate cache
	u.rdb.Del(ctx, "homepage_platform_stats")
	return category, nil
}

func (u *adminUsecase) UpdateCategory(ctx context.Context, id int64, name string) (*domain.Category, error) {
	category, err := u.repo.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, errors.New("category not found")
	}

	oldName := category.Name
	category.Name = name

	if err := u.repo.UpdateCategory(ctx, category); err != nil {
		return nil, err
	}

	// Cascade changes
	_ = u.repo.CascadeCategoryUpdate(ctx, oldName, name)

	// Invalidate caches
	u.rdb.Del(ctx, "homepage_platform_stats")
	return category, nil
}

func (u *adminUsecase) DeleteCategory(ctx context.Context, id int64) error {
	category, err := u.repo.GetCategoryByID(ctx, id)
	if err != nil {
		return err
	}
	if category == nil {
		return errors.New("category not found")
	}

	if err := u.repo.DeleteCategory(ctx, id); err != nil {
		return err
	}

	// Cascade deletes
	_ = u.repo.CascadeCategoryDelete(ctx, category.Name)

	// Invalidate cache
	u.rdb.Del(ctx, "homepage_platform_stats")
	return nil
}

func (u *adminUsecase) GetPlatformStats(ctx context.Context) (map[string]interface{}, error) {
	cacheKey := "homepage_platform_stats"
	if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
		var cached map[string]interface{}
		if err := json.Unmarshal([]byte(val), &cached); err == nil {
			return cached, nil
		}
	}

	statusRow, err := u.repo.GetSiteStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Compute values live
	totalUsers, _ := u.repo.GetTotalUsersCount(ctx)
	
	today := time.Now()
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	startOfWeek := today.AddDate(0, 0, -(weekday - 1))
	endOfWeek := startOfWeek.AddDate(0, 0, 6)
	liveClasses, _ := u.repo.GetWeeklyLiveClassesCount(ctx, startOfWeek, endOfWeek)

	liveBatches, _ := u.repo.GetActiveBatchesCount(ctx)
	smartNotes, _ := u.repo.GetTotalNotesCount(ctx)
	recordings, _ := u.repo.GetRecordingsCount(ctx)

	statusRow.ActiveUser = int(totalUsers)
	statusRow.LiveClassesPerWeek = int(liveClasses)
	statusRow.LiveBatches = int(liveBatches)
	statusRow.SmartNotes = int(smartNotes)
	statusRow.Recordings = int(recordings)

	_ = u.repo.UpdateSiteStatus(ctx, statusRow)

	payload := map[string]interface{}{
		"activeUsers":  statusRow.ActiveUser,
		"liveClasses":  statusRow.LiveClassesPerWeek,
		"liveBatches":  statusRow.LiveBatches,
		"smartNotes":   statusRow.SmartNotes,
		"recordings":   statusRow.Recordings,
	}

	if raw, err := json.Marshal(payload); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
	}

	return payload, nil
}

func (u *adminUsecase) GetPlatformCategories(ctx context.Context) ([]string, error) {
	list, err := u.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	cats := make([]string, len(list))
	for i, c := range list {
		cats[i] = c.Name
	}
	if len(cats) == 0 {
		return []string{"CSE(Graduation)", "11/12(WB Board)"}, nil
	}
	return cats, nil
}

func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func (u *adminUsecase) invalidateTeacherCache(ctx context.Context, teacherID int64) {
	if u.rdb == nil {
		return
	}
	patterns := []string{
		fmt.Sprintf("teacher_overview_%d*", teacherID),
		fmt.Sprintf("teacher_batches_%d*", teacherID),
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

func (u *adminUsecase) ListActiveJobPositions(ctx context.Context) ([]domain.JobPosition, error) {
	return u.repo.ListActiveJobPositions(ctx)
}

func (u *adminUsecase) CreateJobApplication(ctx context.Context, app *domain.JobApplication) error {
	app.Status = "applied"
	if err := u.repo.CreateJobApplication(ctx, app); err != nil {
		return err
	}

	// Trigger async confirmation email
	go func(email, name, role string) {
		subject := "Application Received - ClaSynq"
		body := fmt.Sprintf(
			"Hi %s,\n\n"+
				"Thank you for applying for the %s position at ClaSynq!\n\n"+
				"We have successfully received your application. Our recruitment team is currently reviewing "+
				"your profile and qualifications. If your experience aligns with our needs, we will reach out "+
				"to you for the next steps in the interview process.\n\n"+
				"Thank you again for your interest in ClaSynq and for taking the time to apply.\n\n"+
				"Best regards,\n"+
				"The ClaSynq Team\n"+
				"https://clasynq.in",
			name, role,
		)
		_ = utils.SendEmail(email, subject, body, u.defaultFromEmail, u.smtpHost, u.smtpPort, u.smtpUser, u.smtpPass)
	}(app.Email, app.FullName, app.ApplyForRole)

	return nil
}

func (u *adminUsecase) ListJobApplications(ctx context.Context) ([]domain.JobApplication, error) {
	return u.repo.ListJobApplications(ctx)
}

func (u *adminUsecase) GetAdminPositions(ctx context.Context) ([]domain.JobPosition, error) {
	return u.repo.ListJobPositions(ctx)
}

func (u *adminUsecase) CreateJobPosition(ctx context.Context, jp *domain.JobPosition) error {
	return u.repo.CreateJobPosition(ctx, jp)
}

func (u *adminUsecase) UpdateJobPosition(ctx context.Context, id int64, updates map[string]interface{}) (*domain.JobPosition, error) {
	return u.repo.UpdateJobPosition(ctx, id, updates)
}

func (u *adminUsecase) DeleteJobPosition(ctx context.Context, id int64) error {
	return u.repo.DeleteJobPosition(ctx, id)
}

func (u *adminUsecase) SendCandidateNotification(ctx context.Context, id int64, emailType, meetingLink, interviewDatetime string, joiningLetterName string, joiningLetterData []byte) error {
	app, err := u.repo.GetJobApplicationByID(ctx, id)
	if err != nil {
		return err
	}
	if app == nil {
		return errors.New("candidate application not found")
	}

	emailType = strings.ToLower(emailType)
	switch emailType {
	case "selection":
		if len(joiningLetterData) == 0 {
			return errors.New("joining letter PDF is required")
		}

		subject := "Congratulations! Welcome to the ClaSynq family"
		body := fmt.Sprintf(
			"Hi %s,\n\n"+
				"Congratulations! You are selected for the %s position. Welcome to the ClaSynq family!\n\n"+
				"We are thrilled to have you join our team. Please find attached your official Joining Letter.\n\n"+
				"Best regards,\n"+
				"The ClaSynq Team\n"+
				"https://clasynq.in",
			app.FullName, app.ApplyForRole,
		)

		err = utils.SendEmailWithAttachment(
			app.Email, subject, body, u.defaultFromEmail, u.smtpHost, u.smtpPort, u.smtpUser, u.smtpPass,
			joiningLetterName, joiningLetterData,
		)
		if err != nil {
			return fmt.Errorf("failed to send selection email: %w", err)
		}

		app.Status = "hired"
		return u.repo.UpdateJobApplication(ctx, app)

	case "rejection":
		subject := "Update on your job application - ClaSynq"
		body := fmt.Sprintf(
			"Hi %s,\n\n"+
				"Thank you for your interest in joining ClaSynq and applying for the %s position.\n\n"+
				"We appreciate you taking the time to apply and speak with us. "+
				"Unfortunately, we are not moving forward with your application at this time. "+
				"We will keep your details on file and reach out if a matching opportunity opens in the future.\n\n"+
				"We wish you all the best in your job search and future career endeavors.\n\n"+
				"Best regards,\n"+
				"The ClaSynq Team\n"+
				"https://clasynq.in",
			app.FullName, app.ApplyForRole,
		)

		err = utils.SendEmail(
			app.Email, subject, body, u.defaultFromEmail, u.smtpHost, u.smtpPort, u.smtpUser, u.smtpPass,
		)
		if err != nil {
			return fmt.Errorf("failed to send rejection email: %w", err)
		}

		app.Status = "rejected"
		return u.repo.UpdateJobApplication(ctx, app)

	case "interview":
		if meetingLink == "" || interviewDatetime == "" {
			return errors.New("both meeting_link and interview_datetime are required")
		}

		subject := "Invitation to Interview - ClaSynq"
		body := fmt.Sprintf(
			"Hi %s,\n\n"+
				"Congratulations! You have been selected for the interview round for the %s position at ClaSynq.\n\n"+
				"Here are the details for your scheduled interview:\n"+
				"Date & Time: %s\n"+
				"Meeting Link: %s\n\n"+
				"Please join using the link above at the scheduled time. If you have any questions or need to reschedule, please reply to this email.\n\n"+
				"Best regards,\n"+
				"The ClaSynq Team\n"+
				"https://clasynq.in",
			app.FullName, app.ApplyForRole, interviewDatetime, meetingLink,
		)

		err = utils.SendEmail(
			app.Email, subject, body, u.defaultFromEmail, u.smtpHost, u.smtpPort, u.smtpUser, u.smtpPass,
		)
		if err != nil {
			return fmt.Errorf("failed to send interview invite email: %w", err)
		}

		app.Status = "shortlisted"
		app.MeetingLink = meetingLink
		
		t, parseErr := time.Parse(time.RFC3339, interviewDatetime)
		if parseErr == nil {
			app.InterviewDateTime = &t
		} else {
			// Fallback: parse simple datetime string "YYYY-MM-DD HH:MM"
			t2, parseErr2 := time.Parse("2006-01-02 15:04", interviewDatetime)
			if parseErr2 == nil {
				app.InterviewDateTime = &t2
			} else {
				t3, parseErr3 := time.Parse("2006-01-02T15:04", interviewDatetime)
				if parseErr3 == nil {
					app.InterviewDateTime = &t3
				}
			}
		}

		return u.repo.UpdateJobApplication(ctx, app)

	default:
		return errors.New("invalid email_type")
	}
}
