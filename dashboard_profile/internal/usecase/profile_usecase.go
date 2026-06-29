package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"clasynq/api/dashboard_profile/internal/domain"
)

type profileUsecase struct {
	repo domain.ProfileRepository
}

func NewProfileUsecase(repo domain.ProfileRepository) domain.ProfileUsecase {
	return &profileUsecase{repo: repo}
}

func (u *profileUsecase) GetMe(ctx context.Context, userID int64, role string) (map[string]interface{}, error) {
	if role == "student" {
		user, err := u.repo.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, errors.New("User not found.")
		}
		return u.compileStudentProfile(ctx, user)
	} else if role == "admin" {
		admin, err := u.repo.GetAdminByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if admin == nil {
			return nil, errors.New("Admin not found.")
		}
		return map[string]interface{}{
			"id":        admin.ID,
			"email":     admin.Email,
			"role":      "admin",
			"fullName":  strings.Title(strings.Replace(strings.Split(admin.Email, "@")[0], ".", " ", -1)),
			"createdAt": admin.CreatedAt,
		}, nil
	} else if role == "teacher" {
		teacher, err := u.repo.GetTeacherByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if teacher == nil {
			return nil, errors.New("Teacher not found.")
		}
		return map[string]interface{}{
			"id":             teacher.ID,
			"email":          teacher.Email,
			"role":           "teacher",
			"fullName":       teacher.Name,
			"specialization": teacher.Specialization,
			"photoUrl":       teacher.PhotoURL,
			"createdAt":      teacher.CreatedAt,
		}, nil
	}

	return nil, errors.New("Unknown user role.")
}

func (u *profileUsecase) UpdateMe(ctx context.Context, userID int64, updates map[string]interface{}) (map[string]interface{}, error) {
	user, err := u.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("User not found.")
	}

	// Update fields selectively
	if val, ok := updates["fullName"].(string); ok {
		user.FullName = val
	}
	if val, ok := updates["avatarUrl"].(string); ok {
		user.AvatarURL = val
	}
	if val, ok := updates["headline"].(string); ok {
		user.Headline = val
	}
	if val, ok := updates["bio"].(string); ok {
		user.Bio = val
	}
	if val, ok := updates["skills"].(string); ok {
		user.Skills = val
	}
	if val, ok := updates["website"].(string); ok {
		user.Website = val
	}
	if val, ok := updates["github"].(string); ok {
		user.Github = val
	}
	if val, ok := updates["linkedin"].(string); ok {
		user.Linkedin = val
	}
	if val, ok := updates["twitter"].(string); ok {
		user.Twitter = val
	}

	dobVal, exists := updates["dateOfBirth"]
	if !exists {
		dobVal, exists = updates["date_of_birth"]
	}
	if exists {
		if valStr, ok := dobVal.(string); ok {
			if valStr == "" {
				user.DateOfBirth = nil
			} else {
				cleanDate := strings.Split(valStr, "T")[0]
				t, err := time.Parse("2006-01-02", cleanDate)
				if err == nil {
					user.DateOfBirth = &t
				}
			}
		}
	}

	if err := u.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return u.compileStudentProfile(ctx, user)
}

func (u *profileUsecase) compileStudentProfile(ctx context.Context, user *domain.User) (map[string]interface{}, error) {
	followers, _ := u.repo.GetFollowersList(ctx, user.ID)
	following, _ := u.repo.GetFollowingList(ctx, user.ID)
	referralsCount, _ := u.repo.GetStudentReferralsCount(ctx, user.ID)

	followersList := make([]map[string]interface{}, len(followers))
	for i, f := range followers {
		isFollowed, _ := u.repo.GetFollowRelationship(ctx, user.ID, f.FollowerID)
		followersList[i] = map[string]interface{}{
			"id":         f.Follower.ID,
			"name":       f.Follower.FullName,
			"email":      f.Follower.Email,
			"username":   f.Follower.Username,
			"avatarUrl":  f.Follower.AvatarURL,
			"avatar_url": f.Follower.AvatarURL,
			"bio":        f.Follower.Bio,
			"headline":   f.Follower.Headline,
			"skills":     f.Follower.Skills,
			"website":    f.Follower.Website,
			"github":     f.Follower.Github,
			"linkedin":   f.Follower.Linkedin,
			"twitter":    f.Follower.Twitter,
			"isFollowed": isFollowed != nil,
		}
	}

	followingList := make([]map[string]interface{}, len(following))
	for i, f := range following {
		followingList[i] = map[string]interface{}{
			"id":         f.Followed.ID,
			"name":       f.Followed.FullName,
			"email":      f.Followed.Email,
			"username":   f.Followed.Username,
			"avatarUrl":  f.Followed.AvatarURL,
			"avatar_url": f.Followed.AvatarURL,
			"bio":        f.Followed.Bio,
			"headline":   f.Followed.Headline,
			"skills":     f.Followed.Skills,
			"website":    f.Followed.Website,
			"github":     f.Followed.Github,
			"linkedin":   f.Followed.Linkedin,
			"twitter":    f.Followed.Twitter,
			"isFollowed": true,
		}
	}

	return map[string]interface{}{
		"id":              user.ID,
		"fullName":        user.FullName,
		"username":        user.Username,
		"contactNumber":   user.ContactNumber,
		"email":           user.Email,
		"role":            "student",
		"createdAt":       user.CreatedAt,
		"followersCount":  len(followers),
		"followingCount":  len(following),
		"followersList":   followersList,
		"followingList":   followingList,
		"avatarUrl":       user.AvatarURL,
		"avatar_url":      user.AvatarURL,
		"headline":        user.Headline,
		"bio":             user.Bio,
		"skills":          user.Skills,
		"website":         user.Website,
		"github":          user.Github,
		"linkedin":        user.Linkedin,
		"twitter":         user.Twitter,
		"emailAlerts":     user.EmailAlerts,
		"directMessages":  user.DirectMessages,
		"feedUpdates":     user.FeedUpdates,
		"securityAlerts":  user.SecurityAlerts,
		"coinsBalance":    user.CoinsBalance,
		"referralCode":    user.ReferralCode,
		"dateOfBirth":     user.DateOfBirth,
		"referralsCount":  referralsCount,
	}, nil
}

func (u *profileUsecase) GetMutualConnections(ctx context.Context, userID int64) (map[string]interface{}, error) {
	mutuals, err := u.repo.GetMutualConnections(ctx, userID)
	if err != nil {
		return nil, err
	}

	suggestions := make([]map[string]interface{}, len(mutuals))
	for i, m := range mutuals {
		suggestions[i] = map[string]interface{}{
			"id":          m.User.ID,
			"name":        m.User.FullName,
			"username":    m.User.Username,
			"email":       m.User.Email,
			"avatarUrl":   m.User.AvatarURL,
			"avatar_url":  m.User.AvatarURL,
			"headline":    m.User.Headline,
			"bio":         m.User.Bio,
			"mutualCount": m.MutualCount,
			"isFollowed":  false,
			"is_followed":  false,
		}
	}

	return map[string]interface{}{
		"suggestions": suggestions,
	}, nil
}

func (u *profileUsecase) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID == followedID {
		return false, errors.New("You cannot follow yourself.")
	}

	isFollowing, err := u.repo.ToggleFollowUser(ctx, followerID, followedID)
	if err != nil {
		return false, err
	}

	if isFollowing {
		desc := "Started following user."
		link := fmt.Sprintf("/user/%d", followedID)
		actLog := domain.ActivityLog{
			UserID:       followerID,
			ActivityType: "follow",
			Description:  desc,
			TargetLink:   &link,
		}
		_ = u.repo.CreateActivityLog(ctx, &actLog)

		// Create follow notification
		var followerName string
		followerUser, errU := u.repo.GetUserByID(ctx, followerID)
		if errU == nil && followerUser != nil {
			followerName = followerUser.FullName
		} else {
			teacher, errT := u.repo.GetTeacherByID(ctx, followerID)
			if errT == nil && teacher != nil {
				followerName = teacher.Name
			} else {
				admin, errA := u.repo.GetAdminByID(ctx, followerID)
				if errA == nil && admin != nil {
					followerName = "Admin"
					if admin.Email != "" {
						parts := strings.Split(admin.Email, "@")
						followerName = strings.Title(strings.Replace(parts[0], ".", " ", -1))
					}
				}
			}
		}

		if followerName != "" {
			followedRole := "student"
			msg := fmt.Sprintf("%s started following you.", followerName)
			notif := domain.UserNotification{
				RecipientID:      followedID,
				RecipientRole:    followedRole,
				SenderID:         &followerID,
				NotificationType: "follow",
				Message:          msg,
				IsRead:           false,
			}
			_ = u.repo.CreateNotification(ctx, &notif)
		}
	}

	return isFollowing, nil
}

func (u *profileUsecase) GetStudyDashboard(ctx context.Context, userID int64, category string) (map[string]interface{}, error) {
	// 1. Get student profile
	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return map[string]interface{}{
			"enrolledCourses": []interface{}{},
			"upcomingClasses": []interface{}{},
			"liveClass":       nil,
		}, nil
	}

	// 2. Get enrollments
	enrollments, err := u.repo.GetEnrollmentsByStudentID(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	if len(enrollments) == 0 {
		return map[string]interface{}{
			"enrolledCourses": []interface{}{},
			"upcomingClasses": []interface{}{},
			"liveClass":       nil,
		}, nil
	}

	courseIDs := make([]int64, len(enrollments))
	for i, e := range enrollments {
		courseIDs[i] = e.CourseID
	}

	// 3. Get enrolled courses
	courses, err := u.repo.GetCoursesByIDs(ctx, courseIDs, category)
	if err != nil {
		return nil, err
	}
	if len(courses) == 0 {
		return map[string]interface{}{
			"enrolledCourses": []interface{}{},
			"upcomingClasses": []interface{}{},
			"liveClass":       nil,
		}, nil
	}

	// Re-filter courseIDs for category filtering if active
	filteredCourseIDs := make([]int64, len(courses))
	coursesData := make([]map[string]interface{}, len(courses))
	for i, c := range courses {
		filteredCourseIDs[i] = c.ID
		teacherName := "Instructor"
		if len(c.Teachers) > 0 {
			names := make([]string, len(c.Teachers))
			for j, t := range c.Teachers {
				names[j] = t.Name
			}
			teacherName = strings.Join(names, ", ")
		} else if c.Teacher != nil {
			teacherName = c.Teacher.Name
		}

		coursesData[i] = map[string]interface{}{
			"id":          c.ID,
			"courseName":  c.CourseName,
			"batchId":     c.BatchID,
			"bannerUrl":   c.BannerURL,
			"meetingLink": c.MeetingLink,
			"teacherName": teacherName,
			"category":    c.Category,
		}
	}

	// 4. Get class schedules for the next 7 days
	now := time.Now()
	// Clean up local time to date
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWeek := today.AddDate(0, 0, 7)

	schedules, err := u.repo.GetClassSchedulesByCourseIDsAndDateRange(ctx, filteredCourseIDs, today, endOfWeek)
	if err != nil {
		return nil, err
	}

	// Filter schedules using the same rescheduling/cancellation skip-logic from Django monolith
	// Build map of completed/cancelled topics per course per day
	completedOrCancelled := make(map[string]bool)
	for _, s := range schedules {
		if s.ClassStatus == "completed" || s.ClassStatus == "cancelled" {
			tDate := time.Time(s.ClassDate)
			key := fmt.Sprintf("%d:%s:%s", s.CourseID, tDate.Format("2006-01-02"), strings.TrimSpace(strings.ToLower(s.TopicName)))
			completedOrCancelled[key] = true
		}
	}

	upcomingClasses := make([]map[string]interface{}, 0)
	var liveClass map[string]interface{}

	for _, s := range schedules {
		tDate := time.Time(s.ClassDate)
		key := fmt.Sprintf("%d:%s:%s", s.CourseID, tDate.Format("2006-01-02"), strings.TrimSpace(strings.ToLower(s.TopicName)))
		if completedOrCancelled[key] && (s.ClassStatus == "pending" || s.ClassStatus == "rescheduled") {
			continue
		}

		// Fallback for subjectName
		subjectName := ""
		if s.Subject != nil {
			subjectName = s.Subject.SubjectName
		} else if s.Course != nil && s.Teacher != nil && len(s.Course.TeacherSubjects) > 0 {
			var teacherSubjects map[string][]int64
			if err := json.Unmarshal(s.Course.TeacherSubjects, &teacherSubjects); err == nil {
				teacherIDStr := fmt.Sprintf("%d", s.TeacherID)
				if assignedIDs, ok := teacherSubjects[teacherIDStr]; ok && len(assignedIDs) > 0 {
					for _, sub := range s.Course.Subjects {
						if sub.ID == assignedIDs[0] {
							subjectName = sub.SubjectName
							break
						}
					}
				}
			}
		}
		if subjectName == "" && s.Course != nil && len(s.Course.Subjects) > 0 {
			subjectName = s.Course.Subjects[0].SubjectName
		}

		// Fallback for meetingLink
		meetingLink := ""
		if s.Subject != nil && s.Subject.MeetingLink != "" {
			meetingLink = s.Subject.MeetingLink
		} else if s.Course != nil && s.Teacher != nil && len(s.Course.TeacherSubjects) > 0 {
			var teacherSubjects map[string][]int64
			if err := json.Unmarshal(s.Course.TeacherSubjects, &teacherSubjects); err == nil {
				teacherIDStr := fmt.Sprintf("%d", s.TeacherID)
				if assignedIDs, ok := teacherSubjects[teacherIDStr]; ok && len(assignedIDs) > 0 {
					for _, sub := range s.Course.Subjects {
						for _, aid := range assignedIDs {
							if sub.ID == aid && sub.MeetingLink != "" {
								meetingLink = sub.MeetingLink
								break
							}
						}
						if meetingLink != "" {
							break
						}
					}
				}
			}
		}
		if meetingLink == "" && s.Course != nil {
			for _, sub := range s.Course.Subjects {
				if sub.MeetingLink != "" {
					meetingLink = sub.MeetingLink
					break
				}
			}
			if meetingLink == "" {
				meetingLink = s.Course.MeetingLink
			}
		}

		teacherName := "Instructor"
		if s.Teacher != nil {
			teacherName = s.Teacher.Name
		}

		courseName := ""
		if s.Course != nil {
			courseName = s.Course.CourseName
		}

		classInfo := map[string]interface{}{
			"id":               s.ID,
			"topicName":        s.TopicName,
			"classDate":        tDate.Format("2006-01-02"),
			"startTime":        formatTimeStr(s.StartTime),
			"endTime":          formatTimeStr(s.EndTime),
			"status":           s.ClassStatus,
			"courseName":       courseName,
			"subjectName":      subjectName,
			"meetingLink":      meetingLink,
			"teacherName":      teacherName,
			"batchId":          s.BatchID,
			"classNotesUrl":    s.ClassNotesURL,
			"recordedClassUrl": s.RecordedClassURL,
		}

		// Set liveClass if class is today and status is pending or rescheduled
		// (Following Django matching exactly)
		isToday := tDate.Year() == today.Year() && tDate.Month() == today.Month() && tDate.Day() == today.Day()
		if isToday && (s.ClassStatus == "pending" || s.ClassStatus == "rescheduled") {
			if liveClass == nil {
				liveClass = classInfo
			}
		}

		upcomingClasses = append(upcomingClasses, classInfo)
	}

	return map[string]interface{}{
		"enrolledCourses": coursesData,
		"upcomingClasses": upcomingClasses,
		"liveClass":       liveClass,
	}, nil
}

func (u *profileUsecase) GetHistory(ctx context.Context, userID int64, category string) (map[string]interface{}, error) {
	// 1. Get student profile
	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return map[string]interface{}{"buckets": []interface{}{}}, nil
	}

	// 2. Get enrollments
	enrollments, err := u.repo.GetEnrollmentsByStudentID(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	if len(enrollments) == 0 {
		return map[string]interface{}{"buckets": []interface{}{}}, nil
	}

	courseIDs := make([]int64, len(enrollments))
	for i, e := range enrollments {
		courseIDs[i] = e.CourseID
	}

	// 3. Filter courseIDs if category is provided
	if category != "" {
		courses, err := u.repo.GetCoursesByIDs(ctx, courseIDs, category)
		if err != nil {
			return nil, err
		}
		if len(courses) == 0 {
			return map[string]interface{}{"buckets": []interface{}{}}, nil
		}
		courseIDs = make([]int64, len(courses))
		for i, c := range courses {
			courseIDs[i] = c.ID
		}
	}

	// 4. Get completed schedules
	schedules, err := u.repo.GetCompletedClassSchedulesByCourseIDs(ctx, courseIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)

	todayEntries := make([]map[string]interface{}, 0)
	yesterdayEntries := make([]map[string]interface{}, 0)
	weekEntries := make([]map[string]interface{}, 0)
	olderEntries := make([]map[string]interface{}, 0)

	for _, s := range schedules {
		tDate := time.Time(s.ClassDate)
		subjectPart := ""
		if s.Subject != nil {
			subjectPart = " · " + s.Subject.SubjectName
		}

		courseName := ""
		if s.Course != nil {
			courseName = s.Course.CourseName
		}

		classNotesURL := ""
		if s.ClassNotesURL != nil {
			classNotesURL = *s.ClassNotesURL
		}
		toLink := classNotesURL
		if toLink == "" {
			toLink = "/dashboard/learning_corner/study"
		}

		entry := map[string]interface{}{
			"id":      fmt.Sprintf("class-%d", s.ID),
			"type":    "course-lesson",
			"title":   s.TopicName,
			"context": fmt.Sprintf("%s%s · %s", courseName, subjectPart, s.BatchID),
			"time":    fmt.Sprintf("%s at %s", tDate.Format("Jan 02, 2006"), formatTimeStr(s.StartTime)),
			"to":      toLink,
			"meta": map[string]interface{}{
				"duration": fmt.Sprintf("%s - %s", formatTimeStr(s.StartTime), formatTimeStr(s.EndTime)),
			},
		}

		isToday := tDate.Year() == today.Year() && tDate.Month() == today.Month() && tDate.Day() == today.Day()
		isYesterday := tDate.Year() == yesterday.Year() && tDate.Month() == yesterday.Month() && tDate.Day() == yesterday.Day()
		isWeek := tDate.After(weekAgo) || tDate.Equal(weekAgo)

		if isToday {
			todayEntries = append(todayEntries, entry)
		} else if isYesterday {
			yesterdayEntries = append(yesterdayEntries, entry)
		} else if isWeek {
			weekEntries = append(weekEntries, entry)
		} else {
			olderEntries = append(olderEntries, entry)
		}
	}

	buckets := make([]map[string]interface{}, 0)
	if len(todayEntries) > 0 {
		buckets = append(buckets, map[string]interface{}{"bucket": "Today", "entries": todayEntries})
	}
	if len(yesterdayEntries) > 0 {
		buckets = append(buckets, map[string]interface{}{"bucket": "Yesterday", "entries": yesterdayEntries})
	}
	if len(weekEntries) > 0 {
		buckets = append(buckets, map[string]interface{}{"bucket": "Earlier this week", "entries": weekEntries})
	}
	if len(olderEntries) > 0 {
		buckets = append(buckets, map[string]interface{}{"bucket": "Earlier", "entries": olderEntries})
	}

	return map[string]interface{}{
		"buckets": buckets,
	}, nil
}

// helper to format TimeStr
func formatTimeStr(tStr domain.TimeStr) string {
	t, err := time.Parse("15:04:05", string(tStr))
	if err != nil {
		t, err = time.Parse("15:04", string(tStr))
		if err != nil {
			return string(tStr)
		}
	}
	return t.Format("03:04 PM")
}
