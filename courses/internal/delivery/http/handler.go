package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clasynq/api/courses/internal/domain"

	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	usecase   domain.CourseUsecase
	mediaRoot string
	baseURL   string
}

func NewHttpHandler(usecase domain.CourseUsecase, mediaRoot string, baseURL string) *HttpHandler {
	return &HttpHandler{
		usecase:   usecase,
		mediaRoot: mediaRoot,
		baseURL:   baseURL,
	}
}

func RegisterRoutes(r *gin.Engine, usecase domain.CourseUsecase, mediaRoot string, baseURL string, authMiddleware gin.HandlerFunc, optionalAuthMiddleware gin.HandlerFunc) {
	handler := NewHttpHandler(usecase, mediaRoot, baseURL)

	// Courses routes
	r.GET("/api/courses/", optionalAuthMiddleware, handler.GetCourses)
	r.POST("/api/courses/", authMiddleware, AdminRequired(), handler.CreateCourse)
	r.GET("/api/courses/:id", optionalAuthMiddleware, handler.GetCourseDetails)
	r.PUT("/api/courses/:id", authMiddleware, AdminRequired(), handler.UpdateCourse)
	r.DELETE("/api/courses/:id", authMiddleware, AdminRequired(), handler.DeleteCourse)

	// Teachers & Subjects routes
	r.GET("/api/courses/teachers", handler.ListTeachers)
	r.GET("/api/courses/subjects", handler.ListSubjects)
	r.POST("/api/courses/subjects", authMiddleware, AdminRequired(), handler.CreateSubject)

	// Classes / Schedules routes
	r.GET("/api/classes/", handler.ListSchedules)
	r.POST("/api/classes/", authMiddleware, AdminRequired(), handler.CreateSchedule)
	r.GET("/api/classes/calendar", handler.ListSchedules) // Calendar uses same filters as schedules
	r.GET("/api/classes/analytics", handler.GetAnalytics)
	r.PUT("/api/classes/:id", authMiddleware, AdminRequired(), handler.UpdateSchedule)
	r.DELETE("/api/classes/:id", authMiddleware, AdminRequired(), handler.DeleteSchedule)
}

// Responses for compatibility
type TeacherResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	TeacherName    string `json:"teacher_name"`
	Specialization string `json:"specialization"`
}

type SubjectResponse struct {
	ID          int64  `json:"id"`
	SubjectName string `json:"subjectName"`
	Name        string `json:"subject_name"`
	MeetingLink string `json:"meetingLink"`
}

func toTeacherResponse(t domain.Teacher) TeacherResponse {
	return TeacherResponse{
		ID:             t.ID,
		Name:           t.Name,
		TeacherName:    t.Name,
		Specialization: t.Specialization,
	}
}

func toSubjectResponse(s domain.Subject) SubjectResponse {
	return SubjectResponse{
		ID:          s.ID,
		SubjectName: s.SubjectName,
		Name:        s.SubjectName,
		MeetingLink: s.MeetingLink,
	}
}

// Handlers
func (h *HttpHandler) GetCourses(c *gin.Context) {
	// Parse parameters
	isFeaturedStr := c.Query("isFeatured")
	if isFeaturedStr == "" {
		isFeaturedStr = c.Query("is_featured")
	}
	var isFeatured *bool
	if isFeaturedStr != "" {
		val := (isFeaturedStr == "true" || isFeaturedStr == "1")
		isFeatured = &val
	}

	search := c.Query("q")
	category := c.Query("category")
	limitStr := c.Query("limit")
	var limit int
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	// Extract auth info
	userID := int64(0)
	role := "anonymous"
	if uid, exists := c.Get("userID"); exists {
		userID = uid.(int64)
	}
	if r, exists := c.Get("role"); exists {
		role = r.(string)
	}

	courses, err := h.usecase.GetCourses(c.Request.Context(), role, userID, isFeatured, search, category, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, courses)
}

func (h *HttpHandler) CreateCourse(c *gin.Context) {
	var course domain.Course
	var teacherIDs []int64
	var subjectIDs []int64

	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		course.CourseName = c.PostForm("courseName")
		if course.CourseName == "" {
			course.CourseName = c.PostForm("course_name")
		}
		course.BatchID = c.PostForm("batchId")
		if course.BatchID == "" {
			course.BatchID = c.PostForm("batch_id")
		}
		course.Category = c.PostForm("category")
		course.Language = c.PostForm("language")
		course.Description = c.PostForm("description")
		course.CourseStatus = c.PostForm("courseStatus")
		if course.CourseStatus == "" {
			course.CourseStatus = c.PostForm("course_status")
		}
		course.AccessDuration = c.PostForm("accessDuration")
		if course.AccessDuration == "" {
			course.AccessDuration = c.PostForm("access_duration")
		}
		course.MeetingLink = c.PostForm("meetingLink")
		if course.MeetingLink == "" {
			course.MeetingLink = c.PostForm("meeting_link")
		}
		course.Visibility = c.PostForm("visibility")
		if course.Visibility == "" {
			course.Visibility = "public"
		}

		if val := c.PostForm("isFeatured"); val != "" {
			course.IsFeatured = (val == "true" || val == "1")
		} else if val := c.PostForm("is_featured"); val != "" {
			course.IsFeatured = (val == "true" || val == "1")
		}

		if val := c.PostForm("originalPrice"); val != "" {
			course.OriginalPrice, _ = strconv.ParseFloat(val, 64)
		} else if val := c.PostForm("original_price"); val != "" {
			course.OriginalPrice, _ = strconv.ParseFloat(val, 64)
		}

		if val := c.PostForm("discountPercentage"); val != "" {
			discount, _ := strconv.Atoi(val)
			course.DiscountPercentage = discount
		} else if val := c.PostForm("discount_percentage"); val != "" {
			discount, _ := strconv.Atoi(val)
			course.DiscountPercentage = discount
		}

		if val := c.PostForm("teacher"); val != "" && val != "null" {
			if tid, err := strconv.ParseInt(val, 10, 64); err == nil {
				course.TeacherID = &tid
			}
		} else if val := c.PostForm("teacher_id"); val != "" && val != "null" {
			if tid, err := strconv.ParseInt(val, 10, 64); err == nil {
				course.TeacherID = &tid
			}
		}

		if val := c.PostForm("startDate"); val != "" {
			if t, err := time.Parse("2006-01-02", val); err == nil {
				course.StartDate = domain.DateStr(t)
			}
		} else if val := c.PostForm("start_date"); val != "" {
			if t, err := time.Parse("2006-01-02", val); err == nil {
				course.StartDate = domain.DateStr(t)
			}
		}

		if val := c.PostForm("endDate"); val != "" {
			if t, err := time.Parse("2006-01-02", val); err == nil {
				course.EndDate = domain.DateStr(t)
			}
		} else if val := c.PostForm("end_date"); val != "" {
			if t, err := time.Parse("2006-01-02", val); err == nil {
				course.EndDate = domain.DateStr(t)
			}
		}

		if val := c.PostForm("teacherSubjects"); val != "" {
			course.TeacherSubjects = json.RawMessage(val)
		} else if val := c.PostForm("teacher_subjects"); val != "" {
			course.TeacherSubjects = json.RawMessage(val)
		} else {
			course.TeacherSubjects = json.RawMessage("{}")
		}

		teacherIDs = getListFromForm(c, "teachers")
		subjectIDs = getListFromForm(c, "subjects")

		// Local VPS banner upload
		_, _, err := c.Request.FormFile("banner")
		if err == nil {
			fileURL, err := h.saveFileLocally(c, "banners")
			if err == nil {
				course.BannerURL = fileURL
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "upload_error",
					"message": "Banner upload failed. Failed to write to VPS storage.",
					"data":    gin.H{},
				})
				return
			}
		}

		// Handle subject_meeting_links
		sml := c.PostForm("subject_meeting_links")
		if sml != "" {
			var links map[string]string
			if err := json.Unmarshal([]byte(sml), &links); err == nil {
				for subIDStr, link := range links {
					if subID, err := strconv.ParseInt(subIDStr, 10, 64); err == nil {
						_ = h.usecase.UpdateSubjectMeetingLink(c.Request.Context(), subID, link)
					}
				}
			}
		}
	} else {
		// JSON
		type CreateCourseInput struct {
			CourseName         string            `json:"courseName"`
			BatchID            string            `json:"batchId"`
			Category           string            `json:"category"`
			Language           string            `json:"language"`
			Description        string            `json:"description"`
			Teacher            *int64            `json:"teacher"`
			Teachers           []int64           `json:"teachers"`
			Subjects           []int64           `json:"subjects"`
			OriginalPrice      float64           `json:"originalPrice"`
			DiscountPercentage int               `json:"discountPercentage"`
			CourseStatus       string            `json:"courseStatus"`
			StartDate          string            `json:"startDate"`
			EndDate            string            `json:"endDate"`
			AccessDuration     string            `json:"accessDuration"`
			BannerURL          string            `json:"bannerUrl"`
			MeetingLink        string            `json:"meetingLink"`
			IsFeatured         bool              `json:"isFeatured"`
			Visibility         string            `json:"visibility"`
			TeacherSubjects    interface{}       `json:"teacherSubjects"`
			SubMeetingLinks    map[string]string `json:"subject_meeting_links"`
		}

		var input CreateCourseInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
			return
		}

		course.CourseName = input.CourseName
		course.BatchID = input.BatchID
		course.Category = input.Category
		course.Language = input.Language
		course.Description = input.Description
		course.TeacherID = input.Teacher
		course.OriginalPrice = input.OriginalPrice
		course.DiscountPercentage = input.DiscountPercentage
		course.CourseStatus = input.CourseStatus
		course.AccessDuration = input.AccessDuration
		course.BannerURL = input.BannerURL
		course.MeetingLink = input.MeetingLink
		course.IsFeatured = input.IsFeatured
		course.Visibility = input.Visibility
		if course.Visibility == "" {
			course.Visibility = "public"
		}

		if t, err := time.Parse("2006-01-02", input.StartDate); err == nil {
			course.StartDate = domain.DateStr(t)
		}
		if t, err := time.Parse("2006-01-02", input.EndDate); err == nil {
			course.EndDate = domain.DateStr(t)
		}

		if input.TeacherSubjects != nil {
			if raw, err := json.Marshal(input.TeacherSubjects); err == nil {
				course.TeacherSubjects = raw
			}
		} else {
			course.TeacherSubjects = json.RawMessage("{}")
		}

		teacherIDs = input.Teachers
		subjectIDs = input.Subjects

		for subIDStr, link := range input.SubMeetingLinks {
			if subID, err := strconv.ParseInt(subIDStr, 10, 64); err == nil {
				_ = h.usecase.UpdateSubjectMeetingLink(c.Request.Context(), subID, link)
			}
		}
	}

	if err := h.usecase.CreateCourse(c.Request.Context(), &course, teacherIDs, subjectIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	res, err := h.usecase.GetCourseByIDOrSlug(c.Request.Context(), strconv.FormatInt(course.ID, 10), "admin", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) GetCourseDetails(c *gin.Context) {
	idOrSlug := c.Param("id")

	userID := int64(0)
	role := "anonymous"
	if uid, exists := c.Get("userID"); exists {
		userID = uid.(int64)
	}
	if r, exists := c.Get("role"); exists {
		role = r.(string)
	}

	course, err := h.usecase.GetCourseByIDOrSlug(c.Request.Context(), idOrSlug, role, userID)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"detail": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"detail": "Course not found."})
		return
	}

	if course == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Course not found."})
		return
	}

	c.JSON(http.StatusOK, course)
}

func (h *HttpHandler) UpdateCourse(c *gin.Context) {
	idOrSlug := c.Param("id")
	updates := make(map[string]interface{})

	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		if val, ok := c.GetPostForm("courseName"); ok { updates["courseName"] = val }
		if val, ok := c.GetPostForm("course_name"); ok { updates["courseName"] = val }
		if val, ok := c.GetPostForm("batchId"); ok { updates["batchId"] = val }
		if val, ok := c.GetPostForm("batch_id"); ok { updates["batchId"] = val }
		if val, ok := c.GetPostForm("category"); ok { updates["category"] = val }
		if val, ok := c.GetPostForm("language"); ok { updates["language"] = val }
		if val, ok := c.GetPostForm("description"); ok { updates["description"] = val }
		if val, ok := c.GetPostForm("courseStatus"); ok { updates["courseStatus"] = val }
		if val, ok := c.GetPostForm("course_status"); ok { updates["courseStatus"] = val }
		if val, ok := c.GetPostForm("accessDuration"); ok { updates["accessDuration"] = val }
		if val, ok := c.GetPostForm("access_duration"); ok { updates["accessDuration"] = val }
		if val, ok := c.GetPostForm("meetingLink"); ok { updates["meetingLink"] = val }
		if val, ok := c.GetPostForm("meeting_link"); ok { updates["meetingLink"] = val }
		if val, ok := c.GetPostForm("visibility"); ok { updates["visibility"] = val }
		if val, ok := c.GetPostForm("isFeatured"); ok { updates["isFeatured"] = (val == "true" || val == "1") }
		if val, ok := c.GetPostForm("is_featured"); ok { updates["isFeatured"] = (val == "true" || val == "1") }

		if val, ok := c.GetPostForm("originalPrice"); ok {
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				updates["originalPrice"] = parsed
			}
		} else if val, ok := c.GetPostForm("original_price"); ok {
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				updates["originalPrice"] = parsed
			}
		}

		if val, ok := c.GetPostForm("discountPercentage"); ok {
			if parsed, err := strconv.Atoi(val); err == nil {
				updates["discountPercentage"] = parsed
			}
		} else if val, ok := c.GetPostForm("discount_percentage"); ok {
			if parsed, err := strconv.Atoi(val); err == nil {
				updates["discountPercentage"] = parsed
			}
		}

		if val, ok := c.GetPostForm("teacher"); ok {
			if val == "null" || val == "" {
				updates["teacher"] = nil
			} else if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
				updates["teacher"] = parsed
			}
		} else if val, ok := c.GetPostForm("teacher_id"); ok {
			if val == "null" || val == "" {
				updates["teacher"] = nil
			} else if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
				updates["teacher"] = parsed
			}
		}

		if val, ok := c.GetPostForm("startDate"); ok { updates["startDate"] = val }
		if val, ok := c.GetPostForm("start_date"); ok { updates["startDate"] = val }
		if val, ok := c.GetPostForm("endDate"); ok { updates["endDate"] = val }
		if val, ok := c.GetPostForm("end_date"); ok { updates["endDate"] = val }

		if val, ok := c.GetPostForm("teacherSubjects"); ok {
			var ts map[string]interface{}
			if err := json.Unmarshal([]byte(val), &ts); err == nil {
				updates["teacherSubjects"] = ts
			}
		} else if val, ok := c.GetPostForm("teacher_subjects"); ok {
			var ts map[string]interface{}
			if err := json.Unmarshal([]byte(val), &ts); err == nil {
				updates["teacherSubjects"] = ts
			}
		}

		if c.Request.MultipartForm != nil {
			keys := c.Request.MultipartForm.Value
			if _, ok := keys["teachers"]; ok || hasKeyPrefix(keys, "teachers") {
				updates["teachers"] = getListFromForm(c, "teachers")
			}
			if _, ok := keys["subjects"]; ok || hasKeyPrefix(keys, "subjects") {
				updates["subjects"] = getListFromForm(c, "subjects")
			}
		}

		// Local VPS banner upload
		_, _, err := c.Request.FormFile("banner")
		if err == nil {
			fileURL, err := h.saveFileLocally(c, "banners")
			if err == nil {
				updates["bannerUrl"] = fileURL
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "upload_error",
					"message": "Banner upload failed. Failed to write to VPS storage.",
					"data":    gin.H{},
				})
				return
			}
		}

		// Handle subject_meeting_links
		sml := c.PostForm("subject_meeting_links")
		if sml != "" {
			var links map[string]string
			if err := json.Unmarshal([]byte(sml), &links); err == nil {
				for subIDStr, link := range links {
					if subID, err := strconv.ParseInt(subIDStr, 10, 64); err == nil {
						_ = h.usecase.UpdateSubjectMeetingLink(c.Request.Context(), subID, link)
					}
				}
			}
		}
	} else {
		// JSON
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
			return
		}

		// Handle inner links if in JSON
		if sml, ok := updates["subject_meeting_links"]; ok {
			if raw, err := json.Marshal(sml); err == nil {
				var links map[string]string
				if err := json.Unmarshal(raw, &links); err == nil {
					for subIDStr, link := range links {
						if subID, err := strconv.ParseInt(subIDStr, 10, 64); err == nil {
							_ = h.usecase.UpdateSubjectMeetingLink(c.Request.Context(), subID, link)
						}
					}
				}
			}
		}
	}

	course, err := h.usecase.UpdateCourse(c.Request.Context(), idOrSlug, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, course)
}

func (h *HttpHandler) DeleteCourse(c *gin.Context) {
	idOrSlug := c.Param("id")
	if err := h.usecase.DeleteCourse(c.Request.Context(), idOrSlug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *HttpHandler) ListTeachers(c *gin.Context) {
	teachers, err := h.usecase.ListTeachers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	resList := make([]TeacherResponse, len(teachers))
	for i, t := range teachers {
		resList[i] = toTeacherResponse(t)
	}

	c.JSON(http.StatusOK, gin.H{"teachers": resList})
}

func (h *HttpHandler) ListSubjects(c *gin.Context) {
	subjects, err := h.usecase.ListSubjects(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	resList := make([]SubjectResponse, len(subjects))
	for i, s := range subjects {
		resList[i] = toSubjectResponse(s)
	}

	c.JSON(http.StatusOK, gin.H{"subjects": resList})
}

func (h *HttpHandler) CreateSubject(c *gin.Context) {
	var input struct {
		SubjectName string `json:"subjectName"`
		Name        string `json:"subject_name"`
		MeetingLink string `json:"meetingLink"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	name := input.SubjectName
	if name == "" {
		name = input.Name
	}

	subject := domain.Subject{
		SubjectName: name,
		MeetingLink: input.MeetingLink,
	}

	if err := h.usecase.CreateSubject(c.Request.Context(), &subject); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toSubjectResponse(subject))
}

func (h *HttpHandler) ListSchedules(c *gin.Context) {
	filters := make(map[string]string)
	for _, p := range []string{"teacher", "course", "batch", "status", "start_date", "end_date", "category"} {
		filters[p] = c.Query(p)
	}

	schedules, err := h.usecase.ListSchedules(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, schedules)
}

func parseToInt64(val interface{}) int64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func parseToPtrInt64(val interface{}) *int64 {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case float64:
		i := int64(v)
		return &i
	case int64:
		return &v
	case int:
		i := int64(v)
		return &i
	case string:
		if v == "" || v == "null" {
			return nil
		}
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &i
		}
	}
	return nil
}

func (h *HttpHandler) CreateSchedule(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	var schedule domain.ClassSchedule

	// Parse course
	schedule.CourseID = parseToInt64(body["course"])
	if schedule.CourseID == 0 {
		schedule.CourseID = parseToInt64(body["course_id"])
	}

	// Parse subject
	if sub, ok := body["subject"]; ok {
		schedule.SubjectID = parseToPtrInt64(sub)
	} else if subID, ok := body["subject_id"]; ok {
		schedule.SubjectID = parseToPtrInt64(subID)
	}

	// Parse teacher
	schedule.TeacherID = parseToInt64(body["teacher"])
	if schedule.TeacherID == 0 {
		schedule.TeacherID = parseToInt64(body["teacher_id"])
	}

	// Parse topicName
	if topic, ok := body["topicName"].(string); ok {
		schedule.TopicName = topic
	} else if topic, ok := body["topic_name"].(string); ok {
		schedule.TopicName = topic
	}

	// Parse classDate
	var classDateStr string
	if d, ok := body["classDate"].(string); ok {
		classDateStr = d
	} else if d, ok := body["class_date"].(string); ok {
		classDateStr = d
	}
	if classDateStr != "" {
		if t, err := time.Parse("2006-01-02", classDateStr); err == nil {
			schedule.ClassDate = domain.DateStr(t)
		} else if t, err := time.Parse("02-01-2006", classDateStr); err == nil {
			schedule.ClassDate = domain.DateStr(t)
		} else if t, err := time.Parse(time.RFC3339, classDateStr); err == nil {
			schedule.ClassDate = domain.DateStr(t)
		}
	}

	// Parse startTime / endTime
	var startTimeStr string
	if t, ok := body["startTime"].(string); ok {
		startTimeStr = t
	} else if t, ok := body["start_time"].(string); ok {
		startTimeStr = t
	}
	schedule.StartTime = domain.TimeStr(startTimeStr)

	var endTimeStr string
	if t, ok := body["endTime"].(string); ok {
		endTimeStr = t
	} else if t, ok := body["end_time"].(string); ok {
		endTimeStr = t
	}
	schedule.EndTime = domain.TimeStr(endTimeStr)

	// Parse batchId
	if b, ok := body["batchId"].(string); ok {
		schedule.BatchID = b
	} else if b, ok := body["batch_id"].(string); ok {
		schedule.BatchID = b
	}

	// Parse classStatus
	if s, ok := body["classStatus"].(string); ok {
		schedule.ClassStatus = s
	} else if s, ok := body["class_status"].(string); ok {
		schedule.ClassStatus = s
	}

	// Parse rescheduleReason
	if r, ok := body["rescheduleReason"].(string); ok {
		schedule.RescheduleReason = &r
	} else if r, ok := body["reschedule_reason"].(string); ok {
		schedule.RescheduleReason = &r
	}

	// Parse classNotesUrl
	if n, ok := body["classNotesUrl"].(string); ok {
		schedule.ClassNotesURL = &n
	} else if n, ok := body["class_notes_url"].(string); ok {
		schedule.ClassNotesURL = &n
	}

	// Parse recordedClassUrl
	if rec, ok := body["recordedClassUrl"].(string); ok {
		schedule.RecordedClassURL = &rec
	} else if rec, ok := body["recorded_class_url"].(string); ok {
		schedule.RecordedClassURL = &rec
	}

	if err := h.usecase.CreateSchedule(c.Request.Context(), &schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// Return fully preloaded details
	res, err := h.usecase.ListSchedules(c.Request.Context(), map[string]string{"course": strconv.FormatInt(schedule.CourseID, 10)})
	if err == nil {
		for _, s := range res {
			if s.ID == schedule.ID {
				c.JSON(http.StatusCreated, s)
				return
			}
		}
	}

	c.JSON(http.StatusCreated, schedule)
}

func (h *HttpHandler) UpdateSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid ID format"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	res, err := h.usecase.UpdateSchedule(c.Request.Context(), id, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) DeleteSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid ID format"})
		return
	}

	if err := h.usecase.DeleteSchedule(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *HttpHandler) GetAnalytics(c *gin.Context) {
	category := c.Query("category")
	analytics, err := h.usecase.GetAnalytics(c.Request.Context(), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, analytics)
}

// Helper functions
func getListFromForm(c *gin.Context, fieldName string) []int64 {
	values := c.PostFormArray(fieldName)
	if len(values) == 0 {
		values = c.PostFormArray(fieldName + "[]")
	}
	if len(values) == 0 {
		val := c.PostForm(fieldName)
		if val == "" {
			val = c.PostForm(fieldName + "[]")
		}
		if val != "" {
			values = strings.Split(val, ",")
		}
	}

	var ids []int64
	for _, v := range values {
		parts := strings.Split(v, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if id, err := strconv.ParseInt(part, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func hasKeyPrefix(m map[string][]string, prefix string) bool {
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func (h *HttpHandler) saveFileLocally(c *gin.Context, folder string) (string, error) {
	file, header, err := c.Request.FormFile("banner")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Ensure mediaRoot directory exists
	targetDir := filepath.Join(h.mediaRoot, folder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	// Generate filename (original base + nano timestamp + extension)
	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(header.Filename, ext)
	filename := fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
	filePath := filepath.Join(targetDir, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copy contents
	if _, err = io.Copy(dst, file); err != nil {
		return "", err
	}

	// Return full backend media URL
	relPath := filepath.ToSlash(filepath.Join(folder, filename))
	baseMediaURL := strings.TrimSuffix(h.baseURL, "/")
	return fmt.Sprintf("%s/media/%s", baseMediaURL, relPath), nil
}
