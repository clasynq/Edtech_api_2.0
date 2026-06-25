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

	"clasynq/api/admin/internal/domain"

	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	usecase   domain.AdminUsecase
	secretKey string
	mediaRoot string
	baseURL   string
}

func NewHttpHandler(usecase domain.AdminUsecase, secretKey, mediaRoot, baseURL string) *HttpHandler {
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
		// Platform (General/Public)
		platform := api.Group("/platform")
		{
			platform.GET("/stats", h.GetPlatformStats)
			platform.GET("/categories", h.GetPlatformCategories)
		}

		// Admin Specific (Requires Admin JWT role)
		admin := api.Group("/admin")
		admin.Use(authMiddleware, RequireAdmin())
		{
			admin.GET("/overview", h.GetOverview)
			admin.GET("/activities", h.GetActivities)
			admin.GET("/teachers", h.ListTeachers)
			admin.POST("/teachers", h.CreateTeacher)
			admin.PATCH("/teachers/:id", h.UpdateTeacher)
			admin.DELETE("/teachers/:id", h.DeleteTeacher)
			admin.GET("/students", h.ListStudents)
			admin.GET("/sales-analysis", h.GetSalesAnalysis)
			admin.GET("/categories", h.ListCategories)
			admin.POST("/categories", h.CreateCategory)
			admin.GET("/categories/:id", h.GetCategory)
			admin.PUT("/categories/:id", h.UpdateCategory)
			admin.DELETE("/categories/:id", h.DeleteCategory)
		}
	}
}

func (h *HttpHandler) GetPlatformStats(c *gin.Context) {
	if c.Query("clear_cache") == "true" {
		// handle clear cache
	}
	res, err := h.usecase.GetPlatformStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetPlatformCategories(c *gin.Context) {
	res, err := h.usecase.GetPlatformCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetOverview(c *gin.Context) {
	res, err := h.usecase.GetOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetActivities(c *gin.Context) {
	res, err := h.usecase.GetActivities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) ListTeachers(c *gin.Context) {
	q := c.Query("q")
	category := c.Query("category")
	res, err := h.usecase.ListTeachers(c.Request.Context(), q, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) CreateTeacher(c *gin.Context) {
	var teacher domain.Teacher
	teacher.Email = strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	teacher.Password = c.PostForm("password")
	teacher.Name = strings.TrimSpace(c.PostForm("name"))
	teacher.Specialization = strings.TrimSpace(c.PostForm("specialization"))
	teacher.Category = strings.TrimSpace(c.PostForm("category"))

	dobStr := c.PostForm("date_of_birth")
	if dobStr != "" {
		t, err := time.Parse("2006-01-02", dobStr)
		if err == nil {
			teacher.DateOfBirth = &t
		}
	}

	assignedStr := c.PostForm("assigned_courses")
	if assignedStr != "" {
		teacher.AssignedCourses = assignedStr
	} else {
		teacher.AssignedCourses = "[]"
	}

	tasksStr := c.PostForm("tasks")
	if tasksStr != "" {
		teacher.Tasks = tasksStr
	} else {
		teacher.Tasks = "[]"
	}

	// Handle photo upload
	_, _, err := c.Request.FormFile("photo")
	if err == nil {
		photoURL, err := h.saveFileLocally(c, "photo", "teachers")
		if err == nil {
			teacher.PhotoURL = photoURL
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "photo upload failed: " + err.Error()})
			return
		}
	}

	if teacher.Email == "" || teacher.Password == "" || teacher.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "email, password, and name are required"})
		return
	}

	res, err := h.usecase.CreateTeacher(c.Request.Context(), &teacher)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) UpdateTeacher(c *gin.Context) {
	idStr := c.Param("id")
	teacherID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid teacher ID"})
		return
	}

	// Read fields from Form Data or JSON body
	updates := make(map[string]interface{})

	// Handle multipart form
	_ = c.Request.ParseMultipartForm(32 << 20)
	
	if email := c.PostForm("email"); email != "" {
		updates["email"] = email
	}
	if password := c.PostForm("password"); password != "" {
		updates["password"] = password
	}
	if name := c.PostForm("name"); name != "" {
		updates["name"] = name
	}
	if spec := c.PostForm("specialization"); spec != "" {
		updates["specialization"] = spec
	}
	if cat := c.PostForm("category"); cat != "" {
		updates["category"] = cat
	}
	if dob := c.PostForm("date_of_birth"); dob != "" {
		updates["date_of_birth"] = dob
	}
	if tasks := c.PostForm("tasks"); tasks != "" {
		var tasksArr []interface{}
		if err := json.Unmarshal([]byte(tasks), &tasksArr); err == nil {
			updates["tasks"] = tasksArr
		}
	}
	if assigned := c.PostForm("assigned_courses"); assigned != "" {
		var coursesArr []interface{}
		if err := json.Unmarshal([]byte(assigned), &coursesArr); err == nil {
			updates["assigned_courses"] = coursesArr
		}
	}

	// Handle photo update
	_, _, err = c.Request.FormFile("photo")
	if err == nil {
		photoURL, err := h.saveFileLocally(c, "photo", "teachers")
		if err == nil {
			updates["photo_url"] = photoURL
		}
	}

	res, err := h.usecase.UpdateTeacher(c.Request.Context(), teacherID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) DeleteTeacher(c *gin.Context) {
	idStr := c.Param("id")
	teacherID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid teacher ID"})
		return
	}

	complete := c.DefaultQuery("complete", "true") == "true"
	course := c.Query("course")

	adminID := c.MustGet("userID").(int64)

	err = h.usecase.DeleteTeacher(c.Request.Context(), teacherID, complete, course, adminID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *HttpHandler) ListStudents(c *gin.Context) {
	q := c.Query("q")
	category := c.Query("category")
	res, err := h.usecase.ListStudents(c.Request.Context(), q, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"students": res})
}

func (h *HttpHandler) GetSalesAnalysis(c *gin.Context) {
	month := c.Query("month")
	category := c.Query("category")
	res, err := h.usecase.GetSalesAnalysis(c.Request.Context(), month, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) ListCategories(c *gin.Context) {
	res, err := h.usecase.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid category ID"})
		return
	}
	res, err := h.usecase.GetCategory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if res == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "category not found"})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) CreateCategory(c *gin.Context) {
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"name": []string{"This field is required."}})
		return
	}

	name := strings.TrimSpace(body.Name)
	res, err := h.usecase.CreateCategory(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"name": []string{err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) UpdateCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid category ID"})
		return
	}

	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"name": []string{"This field is required."}})
		return
	}

	name := strings.TrimSpace(body.Name)
	res, err := h.usecase.UpdateCategory(c.Request.Context(), id, name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"name": []string{err.Error()}})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) DeleteCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid category ID"})
		return
	}

	err = h.usecase.DeleteCategory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
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
