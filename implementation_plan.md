# Implementation Plan - Fix Teacher Assigned Tasks Subject and Duplicate Schedules

This plan details the changes required to fix the teacher portal "Schedule & Classes" bug where:
1. **Wrong Subject Mapping:** Assigned tasks are mapped to the wrong subject name (e.g. showing `AI` instead of `ML`).
2. **Duplicate Schedules:** Ghost duplicates of class schedules are created in the database every time tasks are saved.
3. **Outdated Dashboard Cache:** The teacher dashboard fails to display updated schedules immediately due to missing cache invalidation.

## User Review Required

> [!IMPORTANT]
> The root cause of the duplicate schedules is a timezone offset mismatch when GORM queries dates using raw Go `time.Time` objects. By formatting the query date to a pure string `"YYYY-MM-DD"`, we prevent timezone shifts from bypassing matching logic.
> We also add automatic subject resolution inside `UpsertClassSchedule` to map topic descriptions to the correct subject ID.

---

## Proposed Changes

### Backend - `admin` Service

#### [MODIFY] [postgres_admin.go](file:///d:/Clasynq_future_update/API_2.0/admin/internal/repository/postgres_admin.go)

- Update `DeleteClassSchedulesBySignature` to query `class_date` using a formatted string `"YYYY-MM-DD"`.
- Update `UpsertClassSchedule` to format `class_date` to string, format `start_time` with seconds, and automatically resolve the `SubjectID` by matching the topic name against the course subjects.

```diff
 func (r *postgresAdminRepository) DeleteClassSchedulesBySignature(ctx context.Context, teacherID int64, batchID, topic string, date time.Time, startTime string) error {
 	// Parse start time to fit DB formats (either HH:MM or HH:MM:SS)
 	formattedTime := startTime
 	if len(formattedTime) == 5 {
 		formattedTime = formattedTime + ":00"
 	}
+	dateStr := date.Format("2006-01-02")
 	return r.db.WithContext(ctx).
 		Where("teacher_id = ? AND batch_id = ? AND LOWER(topic_name) = ? AND class_date = ? AND start_time::text LIKE ?", 
-			teacherID, batchID, strings.ToLower(topic), date, formattedTime+"%").
+			teacherID, batchID, strings.ToLower(topic), dateStr, formattedTime+"%").
 		Delete(&domain.ClassSchedule{}).Error
 }
```

```diff
 func (r *postgresAdminRepository) UpsertClassSchedule(ctx context.Context, schedule *domain.ClassSchedule, topic string, subjectObj *domain.Subject) error {
 	var existing domain.ClassSchedule
+	dateStr := schedule.ClassDate.Format("2006-01-02")
+	formattedTime := schedule.StartTime
+	if len(formattedTime) == 5 {
+		formattedTime = formattedTime + ":00"
+	}
+
+	var matchedSubjectID *int64
+	if subjectObj != nil {
+		idVal := subjectObj.ID
+		matchedSubjectID = &idVal
+	} else {
+		var subjects []struct {
+			ID          int64  `gorm:"column:id"`
+			SubjectName string `gorm:"column:subject_name"`
+		}
+		errSub := r.db.WithContext(ctx).Table("subjects").
+			Joins("JOIN courses_subjects ON courses_subjects.subject_id = subjects.id").
+			Where("courses_subjects.course_id = ?", schedule.CourseID).
+			Find(&subjects).Error
+			
+		if errSub == nil && len(subjects) > 0 {
+			topicLower := strings.ToLower(topic)
+			for _, sub := range subjects {
+				subLower := strings.ToLower(sub.SubjectName)
+				if subLower != "" && (strings.Contains(topicLower, subLower) || strings.Contains(subLower, topicLower)) {
+					idVal := sub.ID
+					matchedSubjectID = &idVal
+					break
+				}
+			}
+			
+			if matchedSubjectID == nil {
+				if strings.Contains(topicLower, "machine learning") || strings.Contains(topicLower, " ml ") || strings.HasPrefix(topicLower, "ml ") || strings.HasSuffix(topicLower, " ml") || topicLower == "ml" {
+					for _, sub := range subjects {
+						subNameLower := strings.ToLower(sub.SubjectName)
+						if subNameLower == "ml" || strings.Contains(subNameLower, "machine learning") {
+							idVal := sub.ID
+							matchedSubjectID = &idVal
+							break
+						}
+					}
+				} else if strings.Contains(topicLower, "artificial intelligence") || strings.Contains(topicLower, " ai ") || strings.HasPrefix(topicLower, "ai ") || strings.HasSuffix(topicLower, " ai") || topicLower == "ai" {
+					for _, sub := range subjects {
+						subNameLower := strings.ToLower(sub.SubjectName)
+						if subNameLower == "ai" || strings.Contains(subNameLower, "artificial intelligence") {
+							idVal := sub.ID
+							matchedSubjectID = &idVal
+							break
+						}
+					}
+				}
+			}
+			
+			if matchedSubjectID == nil {
+				idVal := subjects[0].ID
+				matchedSubjectID = &idVal
+			}
+		}
+	}
+
 	err := r.db.WithContext(ctx).
 		Where("teacher_id = ? AND course_id = ? AND class_date = ? AND start_time = ?", 
-			schedule.TeacherID, schedule.CourseID, schedule.ClassDate, schedule.StartTime).
+			schedule.TeacherID, schedule.CourseID, dateStr, formattedTime).
 		First(&existing).Error
 	if err == nil {
 		// Update existing record details.
 		existing.TopicName = topic
 		existing.BatchID = schedule.BatchID
-		if subjectObj != nil {
-			existing.SubjectID = &subjectObj.ID
+		if matchedSubjectID != nil {
+			existing.SubjectID = matchedSubjectID
 		}
 		return r.db.WithContext(ctx).Save(&existing).Error
 	} else if errors.Is(err, gorm.ErrRecordNotFound) {
 		// Create new schedule record.
-		if subjectObj != nil {
-			schedule.SubjectID = &subjectObj.ID
+		if matchedSubjectID != nil {
+			schedule.SubjectID = matchedSubjectID
 		}
 		return r.db.WithContext(ctx).Create(schedule).Error
 	}
 	return err
 }
```

#### [MODIFY] [admin_usecase.go](file:///d:/Clasynq_future_update/API_2.0/admin/internal/usecase/admin_usecase.go)

- Update `invalidateTeacherCache` to clear the `teacher_classes_{id}*` Redis pattern when the admin updates tasks.

```diff
 func (u *adminUsecase) invalidateTeacherCache(ctx context.Context, teacherID int64) {
 	if u.rdb == nil {
 		return
 	}
 	patterns := []string{
 		fmt.Sprintf("teacher_overview_%d*", teacherID),
 		fmt.Sprintf("teacher_batches_%d*", teacherID),
+		fmt.Sprintf("teacher_classes_%d*", teacherID),
 	}
```

---

## Verification Plan

### Automated Tests
- Build admin Go microservice: `go build ./admin/...`

### Manual Verification
- Log in as admin, edit a teacher's tasks.
- Assign the teacher:
  - Batch: `AIML-HB-UNIQ`
  - Subject: `ML`
  - Task: `Introduction to ML & Python` (Date: 2026-07-22, Time: 7:30 PM)
  - Task: `Data Preprocessing` (Date: 2026-07-26, Time: 7:30 PM)
- Click save. Verify no duplicate rows are created in `class_schedules` table.
- Log in as the teacher, view **Schedule & Classes**.
- Verify that the card for `2026-07-26` shows subject `ML` (not `AI`) and the card for `2026-07-22` shows the correct topic name.
