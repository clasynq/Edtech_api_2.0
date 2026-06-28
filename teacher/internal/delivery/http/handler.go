package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clasynq/api/teacher/internal/domain"

	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	usecase   domain.TeacherUsecase
	secretKey string
	mediaRoot string
	baseURL   string
}

func NewHttpHandler(usecase domain.TeacherUsecase, secretKey, mediaRoot, baseURL string) *HttpHandler {
	return &HttpHandler{
		usecase:   usecase,
		secretKey: secretKey,
		mediaRoot: mediaRoot,
		baseURL:   baseURL,
	}
}

func (h *HttpHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := r.Group("/api")
	{
		teacher := api.Group("/teacher")
		teacher.Use(authMiddleware, RequireTeacher())
		{
			teacher.GET("/overview", h.GetOverview)
			teacher.GET("/categories", h.GetCategories)
			teacher.POST("/assign-student", h.AssignStudent)
			teacher.GET("/batches", h.GetBatches)
			teacher.GET("/chapters", h.GetChapters)
			teacher.GET("/classes", h.GetClasses)
			teacher.POST("/classes", h.ScheduleClass)
			teacher.GET("/classes/:id", h.GetClassDetail)
			teacher.PATCH("/classes/:id", h.UpdateClass)
			teacher.DELETE("/classes/:id", h.DeleteClass)
			teacher.POST("/notes", h.UploadNote)
		}
	}
}

func (h *HttpHandler) GetOverview(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	category := c.Query("category")
	
	res, err := h.usecase.GetOverview(c.Request.Context(), teacherID, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetCategories(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	res, err := h.usecase.GetCategories(c.Request.Context(), teacherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) AssignStudent(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	
	var body struct {
		StudentID int64 `json:"student_id" binding:"required"`
		CourseID  int64 `json:"course_id" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "student_id and course_id are required."})
		return
	}

	res, err := h.usecase.AssignStudent(c.Request.Context(), teacherID, body.StudentID, body.CourseID)
	if err != nil {
		if strings.Contains(err.Error(), "authorized") {
			c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetBatches(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	category := c.Query("category")

	res, err := h.usecase.GetBatches(c.Request.Context(), teacherID, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetChapters(c *gin.Context) {
	// Monolithic was returning empty list
	c.JSON(http.StatusOK, gin.H{"chapters": []interface{}{}})
}

func (h *HttpHandler) GetClasses(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	category := c.Query("category")

	res, err := h.usecase.GetClasses(c.Request.Context(), teacherID, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) ScheduleClass(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.usecase.ScheduleClass(c.Request.Context(), teacherID, body)
	if err != nil {
		if strings.Contains(err.Error(), "authorized") {
			c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) GetClassDetail(c *gin.Context) {
	// Simple lookup
	c.JSON(http.StatusNotImplemented, gin.H{"message": "GET detail not fully implemented individually"})
}

func (h *HttpHandler) UpdateClass(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	idStr := c.Param("id")
	classID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid class schedule ID"})
		return
	}

	// Parse fields
	updates := make(map[string]interface{})

	// Supports multipart form for class notes upload
	_ = c.Request.ParseMultipartForm(32 << 20)

	if topic := c.PostForm("topicName"); topic != "" {
		updates["topicName"] = topic
	} else if topic := c.PostForm("topic_name"); topic != "" {
		updates["topic_name"] = topic
	}

	if classDate := c.PostForm("classDate"); classDate != "" {
		updates["classDate"] = classDate
	} else if classDate := c.PostForm("class_date"); classDate != "" {
		updates["class_date"] = classDate
	}

	if startTime := c.PostForm("startTime"); startTime != "" {
		updates["startTime"] = startTime
	} else if startTime := c.PostForm("start_time"); startTime != "" {
		updates["start_time"] = startTime
	}

	if endTime := c.PostForm("endTime"); endTime != "" {
		updates["endTime"] = endTime
	} else if endTime := c.PostForm("end_time"); endTime != "" {
		updates["end_time"] = endTime
	}

	if status := c.PostForm("classStatus"); status != "" {
		updates["classStatus"] = status
	} else if status := c.PostForm("class_status"); status != "" {
		updates["class_status"] = status
	}

	if reason := c.PostForm("rescheduleReason"); reason != "" {
		updates["rescheduleReason"] = reason
	} else if reason := c.PostForm("reschedule_reason"); reason != "" {
		updates["reschedule_reason"] = reason
	}

	if recorded := c.PostForm("recordedClassUrl"); recorded != "" {
		updates["recordedClassUrl"] = recorded
	} else if recorded := c.PostForm("recorded_class_url"); recorded != "" {
		updates["recorded_class_url"] = recorded
	}

	if notesURL := c.PostForm("classNotesUrl"); notesURL != "" {
		updates["classNotesUrl"] = notesURL
	} else if notesURL := c.PostForm("class_notes_url"); notesURL != "" {
		updates["class_notes_url"] = notesURL
	}

	if subject := c.PostForm("subject"); subject != "" {
		if subID, err := strconv.ParseInt(subject, 10, 64); err == nil {
			updates["subject"] = subID
		}
	}

	// Support JSON body fallback if form is empty
	if len(updates) == 0 {
		var jsonBody map[string]interface{}
		if err := c.ShouldBindJSON(&jsonBody); err == nil {
			for k, v := range jsonBody {
				updates[k] = v
			}
		}
	}

	// Handle notes file upload
	_, _, err = c.Request.FormFile("notes_file")
	if err == nil {
		notesURL, err := h.saveFileLocally(c, "notes_file", "notes")
		if err == nil {
			updates["classNotesUrl"] = notesURL
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "notes upload failed: " + err.Error()})
			return
		}
	}

	res, err := h.usecase.UpdateClass(c.Request.Context(), teacherID, classID, updates)
	if err != nil {
		if strings.Contains(err.Error(), "permission") {
			c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) DeleteClass(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)
	idStr := c.Param("id")
	classID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid class ID"})
		return
	}

	err = h.usecase.DeleteClass(c.Request.Context(), teacherID, classID)
	if err != nil {
		if strings.Contains(err.Error(), "permission") {
			c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *HttpHandler) UploadNote(c *gin.Context) {
	teacherID := c.MustGet("userID").(int64)

	_ = c.Request.ParseMultipartForm(32 << 20)

	batchID := c.PostForm("batchId")
	title := c.PostForm("title")
	recordedClassURL := c.PostForm("recordedClassUrl")
	if recordedClassURL == "" {
		recordedClassURL = c.PostForm("recorded_class_url")
	}

	subject := c.PostForm("subject")
	topic := c.PostForm("topic")
	prerequisiteURL := c.PostForm("prerequisiteUrl")
	if prerequisiteURL == "" {
		prerequisiteURL = c.PostForm("prerequisite_url")
	}

	if batchID == "" || title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "batchId and title are required"})
		return
	}

	_, _, err := c.Request.FormFile("file")
	if err != nil && recordedClassURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Either file or class recording link is required"})
		return
	}

	fileURL := ""
	if err == nil {
		fileURL, err = h.saveFileLocally(c, "file", "notes")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "file upload failed: " + err.Error()})
			return
		}
	}

	description := c.PostForm("description")

	res, err := h.usecase.UploadNote(c.Request.Context(), teacherID, batchID, title, fileURL, recordedClassURL, subject, topic, prerequisiteURL, description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) saveFileLocally(c *gin.Context, formFieldName, folder string) (string, error) {
	file, header, err := c.Request.FormFile(formFieldName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	targetDir := filepath.Join(h.mediaRoot, folder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(header.Filename, ext)
	filename := fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
	filePath := filepath.Join(targetDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		return "", err
	}

	relPath := filepath.ToSlash(filepath.Join(folder, filename))
	baseMediaURL := strings.TrimSuffix(h.baseURL, "/")
	return fmt.Sprintf("%s/media/%s", baseMediaURL, relPath), nil
}
