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

// HttpHandler maps REST routes to Admin Usecase executions.
type HttpHandler struct {
	usecase   domain.AdminUsecase
	secretKey string // Secret key to verify JWT signatures
	mediaRoot string // Disk path to save file uploads statically
	baseURL   string // Host URL (to build fully-qualified media URLs)
}

// NewHttpHandler initializes a new HttpHandler controller instance.
func NewHttpHandler(usecase domain.AdminUsecase, secretKey, mediaRoot, baseURL string) *HttpHandler {
	return &HttpHandler{
		usecase:   usecase,
		secretKey: secretKey,
		mediaRoot: mediaRoot,
		baseURL:   baseURL,
	}
}

// RegisterRoutes sets up the HTTP paths for the admin panel microservice.
func (h *HttpHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := r.Group("/api")
	{
		// Platform routes (General/Public counters)
		platform := api.Group("/platform")
		{
			platform.GET("/stats", h.GetPlatformStats)
			platform.GET("/categories", h.GetPlatformCategories)
		}

		// Careers Public routes (Job Listings & Application submissions)
		careersPublic := api.Group("/careers")
		{
			careersPublic.GET("/positions", h.GetCareersPositions)
			careersPublic.POST("/apply", h.SubmitJobApplication)
		}

		// Careers Admin routes (Authenticated + Require Admin role check)
		careersAdmin := api.Group("/careers/admin")
		careersAdmin.Use(authMiddleware, RequireAdmin())
		{
			careersAdmin.GET("/applications", h.GetAdminApplications)
			careersAdmin.POST("/applications/:id/notify", h.SendCandidateNotification)
			careersAdmin.GET("/positions", h.GetAdminPositions)
			careersAdmin.POST("/positions", h.CreateJobPosition)
			careersAdmin.PATCH("/positions/:id", h.UpdateJobPosition)
			careersAdmin.DELETE("/positions/:id", h.DeleteJobPosition)
		}

		// Admin specific management actions (Requires Admin auth token validation)
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

// GetPlatformStats returns high-level metric counts for the public homepage.
func (h *HttpHandler) GetPlatformStats(c *gin.Context) {
	if c.Query("clear_cache") == "true" {
		// Cache clearance behavior details.
	}
	res, err := h.usecase.GetPlatformStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetPlatformCategories lists category names for course selections.
func (h *HttpHandler) GetPlatformCategories(c *gin.Context) {
	res, err := h.usecase.GetPlatformCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetOverview returns counts (total students, total teachers, active courses) for admin panel dashboard cards.
func (h *HttpHandler) GetOverview(c *gin.Context) {
	res, err := h.usecase.GetOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetActivities returns admin audit trail activity logs.
func (h *HttpHandler) GetActivities(c *gin.Context) {
	res, err := h.usecase.GetActivities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListTeachers returns teacher profiles matching search or category filters.
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

// CreateTeacher registers a teacher profile from form data parameter bindings.
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

	// Handle profile image upload if present.
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

// UpdateTeacher modifies teacher profiles. Checks request Content-Type to support both raw JSON bodies and multipart form submissions.
func (h *HttpHandler) UpdateTeacher(c *gin.Context) {
	idStr := c.Param("id")
	teacherID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid teacher ID"})
		return
	}

	updates := make(map[string]interface{})

	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var jsonUpdates map[string]interface{}
		if err := c.ShouldBindJSON(&jsonUpdates); err == nil {
			for k, v := range jsonUpdates {
				updates[k] = v
			}
		}
	} else {
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
	}

	// Handle updated profile photo
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

// DeleteTeacher removes a teacher profile. If complete query parameter is false, it unassigns them from a single course.
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

// ListStudents returns student profile details matching name filters.
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

// GetSalesAnalysis returns monthly revenue breakdown metrics across courses, notes, and test series.
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

// ListCategories returns course categories sorted by name.
func (h *HttpHandler) ListCategories(c *gin.Context) {
	res, err := h.usecase.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetCategory returns a single category by ID.
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

// CreateCategory adds a course category.
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

// UpdateCategory updates category settings.
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

// DeleteCategory deletes a category record.
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

// saveFileLocally parses file headers, creates necessary folders under h.mediaRoot,
// and saves files using a unique timestamp key to avoid filename collisions.
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

// GetCareersPositions lists active job openings for applicant selection.
func (h *HttpHandler) GetCareersPositions(c *gin.Context) {
	list, err := h.usecase.ListActiveJobPositions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// SubmitJobApplication handles candidate application submissions. It executes spam-prevention checks:
// (1) Honeypot check (hidden field middle_name must be blank).
// (2) Link checks (fields must not contain promotional hyperlinks).
// (3) Phone number format verification (only numbers/symbols).
// (4) File size and extension constraints (resume <= 200KB PDF, photo <= 100KB JPEG/PNG).
func (h *HttpHandler) SubmitJobApplication(c *gin.Context) {
	// 1. Honeypot check
	if c.PostForm("middle_name") != "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid submission."})
		return
	}

	// 2. Link/HTML hyperlink spam check
	spamMarkers := []string{"http://", "https://", "www.", "<a href=", "[url="}
	textFields := []string{"full_name", "qualification", "branch", "apply_for_role", "specialization"}
	for _, field := range textFields {
		val := strings.ToLower(c.PostForm(field))
		for _, marker := range spamMarkers {
			if strings.Contains(val, marker) {
				c.JSON(http.StatusBadRequest, gin.H{"message": "Links and promotional content are not allowed."})
				return
			}
		}
	}

	// 3. Phone validation
	phone := c.PostForm("phone")
	if phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "phone is required"})
		return
	}
	for _, char := range phone {
		if (char < '0' || char > '9') && char != '+' && char != '-' && char != '(' && char != ')' && char != ' ' {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid mobile number format."})
			return
		}
	}

	// 4. Resume size and format constraints (must be <= 200KB PDF)
	resumeHeader, err := c.FormFile("resume_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "resume_file is required"})
		return
	}
	if resumeHeader.Size > 200*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "resume file must be less than or equal to 200KB"})
		return
	}
	if strings.ToLower(filepath.Ext(resumeHeader.Filename)) != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "resume must be a PDF file"})
		return
	}

	// 5. Photo size and format constraints (must be <= 100KB JPEG/PNG)
	photoHeader, err := c.FormFile("photo_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "photo_file is required"})
		return
	}
	if photoHeader.Size > 100*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "photo file must be less than or equal to 100KB"})
		return
	}
	photoExt := strings.ToLower(filepath.Ext(photoHeader.Filename))
	if photoExt != ".jpg" && photoExt != ".jpeg" && photoExt != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "photo must be a JPG, JPEG, or PNG file"})
		return
	}

	// Save files on disk
	resumeURL, err := h.saveFileLocally(c, "resume_file", "careers/resumes")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to save resume: " + err.Error()})
		return
	}

	photoURL, err := h.saveFileLocally(c, "photo_file", "careers/photos")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to save photo: " + err.Error()})
		return
	}

	var positionID *int64
	posStr := c.PostForm("position")
	if posStr != "" {
		if pID, err := strconv.ParseInt(posStr, 10, 64); err == nil {
			positionID = &pID
		}
	}

	passingYear, _ := strconv.Atoi(c.PostForm("passing_year"))
	cgpa, _ := strconv.ParseFloat(c.PostForm("cgpa"), 64)
	expYears, _ := strconv.ParseFloat(c.PostForm("experience_years"), 64)

	app := domain.JobApplication{
		PositionID:      positionID,
		FullName:        c.PostForm("full_name"),
		Email:           c.PostForm("email"),
		Phone:           c.PostForm("phone"),
		Qualification:   c.PostForm("qualification"),
		Branch:          c.PostForm("branch"),
		PursuitStatus:   c.PostForm("pursuit_status"),
		PassingYear:     passingYear,
		CGPA:            cgpa,
		ApplyForRole:    c.PostForm("apply_for_role"),
		Specialization:  c.PostForm("specialization"),
		ExperienceYears: expYears,
		ResumeURL:       resumeURL,
		PhotoURL:        photoURL,
	}

	if err := h.usecase.CreateJobApplication(c.Request.Context(), &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Application submitted successfully", "application": app})
}

// GetAdminPositions lists all careers openings for administration.
func (h *HttpHandler) GetAdminPositions(c *gin.Context) {
	list, err := h.usecase.GetAdminPositions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// CreateJobPosition publishes a job opening position.
func (h *HttpHandler) CreateJobPosition(c *gin.Context) {
	var payload struct {
		Title          string `json:"title" binding:"required"`
		Department     string `json:"department" binding:"required"`
		Location       string `json:"location" binding:"required"`
		EmploymentType string `json:"employment_type" binding:"required"`
		Description    string `json:"description" binding:"required"`
		Requirements   string `json:"requirements" binding:"required"`
		IsActive       *bool  `json:"is_active" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	jp := domain.JobPosition{
		Title:          payload.Title,
		Department:     payload.Department,
		Location:       payload.Location,
		EmploymentType: payload.EmploymentType,
		Description:    payload.Description,
		Requirements:   payload.Requirements,
		IsActive:       *payload.IsActive,
	}

	if err := h.usecase.CreateJobPosition(c.Request.Context(), &jp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, jp)
}

// UpdateJobPosition updates specific details on a job opening position.
func (h *HttpHandler) UpdateJobPosition(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid position ID"})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if val, ok := body["title"]; ok {
		updates["title"] = val
	}
	if val, ok := body["department"]; ok {
		updates["department"] = val
	}
	if val, ok := body["location"]; ok {
		updates["location"] = val
	}
	if val, ok := body["employment_type"]; ok {
		updates["employment_type"] = val
	}
	if val, ok := body["description"]; ok {
		updates["description"] = val
	}
	if val, ok := body["requirements"]; ok {
		updates["requirements"] = val
	}
	if val, ok := body["is_active"]; ok {
		updates["is_active"] = val
	}

	jp, err := h.usecase.UpdateJobPosition(c.Request.Context(), id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, jp)
}

// DeleteJobPosition deletes a job opening position record.
func (h *HttpHandler) DeleteJobPosition(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid position ID"})
		return
	}

	if err := h.usecase.DeleteJobPosition(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetAdminApplications lists submitted applications for administration.
func (h *HttpHandler) GetAdminApplications(c *gin.Context) {
	list, err := h.usecase.ListJobApplications(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// SendCandidateNotification triggers recruitment updates (shortlisting, selection, rejection) to applicants.
// If type is "selection", it validates and reads an attached Joining Letter PDF file (size <= 100KB).
func (h *HttpHandler) SendCandidateNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid application ID"})
		return
	}

	emailType := c.PostForm("email_type")
	if emailType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "email_type is required"})
		return
	}

	meetingLink := c.PostForm("meeting_link")
	interviewDatetime := c.PostForm("interview_datetime")

	var joiningLetterName string
	var joiningLetterData []byte

	if strings.ToLower(emailType) == "selection" {
		file, header, err := c.Request.FormFile("joining_letter")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "joining_letter file is required for selection notification"})
			return
		}
		defer file.Close()

		if header.Size > 100*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "joining letter PDF must be less than or equal to 100KB"})
			return
		}

		if strings.ToLower(filepath.Ext(header.Filename)) != ".pdf" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "joining letter must be a PDF file"})
			return
		}

		joiningLetterName = header.Filename
		joiningLetterData, err = io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to read joining letter file: " + err.Error()})
			return
		}
	}

	err = h.usecase.SendCandidateNotification(c.Request.Context(), id, emailType, meetingLink, interviewDatetime, joiningLetterName, joiningLetterData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email sent successfully!"})
}


