package http

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clasynq/api/test_series/internal/domain"

	"github.com/gin-gonic/gin"
)

type httpHandler struct {
	uc domain.TestSeriesUsecase
}

func NewHttpHandler(uc domain.TestSeriesUsecase) *httpHandler {
	return &httpHandler{uc: uc}
}

func RegisterRoutes(
	r *gin.Engine,
	uc domain.TestSeriesUsecase,
	authMiddleware gin.HandlerFunc,
	optionalAuthMiddleware gin.HandlerFunc,
) {
	handler := NewHttpHandler(uc)

	r.GET("/api/test-series/", optionalAuthMiddleware, handler.GetTestSeries)
	r.GET("/api/test-series/:id", optionalAuthMiddleware, handler.GetTestSeriesDetails)

	auth := r.Group("/api/test-series", authMiddleware)
	{
		auth.POST("/", AdminOrTeacherRequired(), handler.CreateTestSeries)
		auth.PUT("/:id", AdminOrTeacherRequired(), handler.UpdateTestSeries)
		auth.PUT("/:id/", AdminOrTeacherRequired(), handler.UpdateTestSeries)
		auth.DELETE("/:id", AdminOrTeacherRequired(), handler.DeleteTestSeries)
		auth.DELETE("/:id/", AdminOrTeacherRequired(), handler.DeleteTestSeries)
		auth.POST("/:id/tests", AdminOrTeacherRequired(), handler.CreateTest)
		auth.POST("/tests/:id/questions", AdminOrTeacherRequired(), handler.AddQuestion)
	}

	// Legacy / CRUD routes for tests
	tests := r.Group("/api/tests", authMiddleware)
	{
		tests.GET("/:id/", handler.GetTestDetails)
		tests.POST("/", AdminOrTeacherRequired(), handler.CreateTestLegacy)
		tests.PUT("/:id/", AdminOrTeacherRequired(), handler.UpdateTest)
		tests.DELETE("/:id/", AdminOrTeacherRequired(), handler.DeleteTest)
		tests.POST("/:id/upload_questions/", AdminOrTeacherRequired(), handler.UploadQuestions)
	}

	// Legacy / CRUD routes for questions
	questions := r.Group("/api/questions", authMiddleware)
	{
		questions.GET("/", handler.GetQuestions)
		questions.POST("/", AdminOrTeacherRequired(), handler.CreateQuestion)
		questions.DELETE("/:id/", AdminOrTeacherRequired(), handler.DeleteQuestion)
	}
}

func (h *httpHandler) GetTestSeries(c *gin.Context) {
	filters := map[string]string{
		"category":    c.Query("category"),
		"courseId":    c.Query("courseId"),
		"isPublished": c.Query("isPublished"),
		"search":      c.Query("search"),
	}

	userID := int64(0)
	role := ""
	if uIDVal, exists := c.Get("userID"); exists {
		userID = uIDVal.(int64)
	}
	if rVal, exists := c.Get("role"); exists {
		role = rVal.(string)
	}

	list, err := h.uc.GetTestSeries(c.Request.Context(), userID, role, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}

func (h *httpHandler) GetTestSeriesDetails(c *gin.Context) {
	idOrSlug := c.Param("id")

	userID := int64(0)
	role := ""
	if uIDVal, exists := c.Get("userID"); exists {
		userID = uIDVal.(int64)
	}
	if rVal, exists := c.Get("role"); exists {
		role = rVal.(string)
	}

	ts, hasAccess, err := h.uc.GetTestSeriesByIDOrSlug(c.Request.Context(), userID, role, idOrSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	if ts == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Test series not found"})
		return
	}

	ts.HasAccess = hasAccess
	c.JSON(http.StatusOK, ts)
}

func (h *httpHandler) CreateTestSeries(c *gin.Context) {
	ts, err := h.parseTestSeries(c, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	if err := h.uc.CreateTestSeries(c.Request.Context(), ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ts)
}

func (h *httpHandler) UpdateTestSeries(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test series ID"})
		return
	}

	userID := int64(0)
	role := ""
	if uIDVal, exists := c.Get("userID"); exists {
		userID = uIDVal.(int64)
	}
	if rVal, exists := c.Get("role"); exists {
		role = rVal.(string)
	}

	existing, _, err := h.uc.GetTestSeriesByIDOrSlug(c.Request.Context(), userID, role, idStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Test series not found"})
		return
	}

	ts, err := h.parseTestSeries(c, existing)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	if err := h.uc.UpdateTestSeries(c.Request.Context(), id, ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ts)
}

func (h *httpHandler) DeleteTestSeries(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test series ID"})
		return
	}

	if err := h.uc.DeleteTestSeries(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "message": "Test series successfully deleted"})
}

func (h *httpHandler) parseTestSeries(c *gin.Context, existing *domain.TestSeries) (*domain.TestSeries, error) {
	contentType := c.ContentType()
	ts := &domain.TestSeries{}
	if existing != nil {
		ts = existing
	}

	if strings.Contains(contentType, "multipart/form-data") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
		_ = c.Request.ParseMultipartForm(32 << 20)

		if val, exists := c.GetPostForm("title"); exists {
			ts.Title = val
		}
		if val, exists := c.GetPostForm("description"); exists {
			ts.Description = val
		}
		if val, exists := c.GetPostForm("category"); exists {
			ts.Category = val
		}

		if val, exists := c.GetPostForm("batch_id"); exists {
			ts.BatchID = &val
		} else if val, exists := c.GetPostForm("batchId"); exists {
			ts.BatchID = &val
		}

		isPublishedStr, hasPublished := c.GetPostForm("is_published")
		if !hasPublished {
			isPublishedStr, hasPublished = c.GetPostForm("isPublished")
		}
		if hasPublished {
			ts.IsPublished, _ = strconv.ParseBool(isPublishedStr)
		}

		startDateStr, hasStart := c.GetPostForm("start_date")
		if !hasStart {
			startDateStr, hasStart = c.GetPostForm("startDate")
		}
		if hasStart {
			if startDateStr == "" {
				ts.StartDate = nil
			} else if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
				ts.StartDate = &t
			} else if t, err := time.Parse(time.RFC3339, startDateStr); err == nil {
				ts.StartDate = &t
			}
		}

		endDateStr, hasEnd := c.GetPostForm("end_date")
		if !hasEnd {
			endDateStr, hasEnd = c.GetPostForm("endDate")
		}
		if hasEnd {
			if endDateStr == "" {
				ts.EndDate = nil
			} else if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
				ts.EndDate = &t
			} else if t, err := time.Parse(time.RFC3339, endDateStr); err == nil {
				ts.EndDate = &t
			}
		}

		courseIDStr, hasCourse := c.GetPostForm("course")
		if !hasCourse {
			courseIDStr, hasCourse = c.GetPostForm("courseId")
		}
		if hasCourse {
			if courseIDStr == "" {
				ts.CourseID = nil
			} else if cid, err := strconv.ParseInt(courseIDStr, 10, 64); err == nil {
				ts.CourseID = &cid
			}
		}

		isFreeStr, hasFree := c.GetPostForm("is_free")
		if !hasFree {
			isFreeStr, hasFree = c.GetPostForm("isFree")
		}
		if hasFree {
			ts.IsFree, _ = strconv.ParseBool(isFreeStr)
		}

		if priceStr, exists := c.GetPostForm("price"); exists {
			ts.Price, _ = strconv.ParseFloat(priceStr, 64)
		}

		if slugStr, exists := c.GetPostForm("slug"); exists {
			ts.Slug = slugStr
		}

		// Handle banner image upload
		bannerURL, err := saveUploadFile(c, "banner", "banners")
		if err == nil && bannerURL != "" {
			ts.BannerURL = bannerURL
		} else {
			// Only update BannerURL from text field if it is provided
			if val, exists := c.GetPostForm("bannerUrl"); exists {
				ts.BannerURL = val
			}
		}
	} else {
		// Fallback to JSON
		var req struct {
			Title       *string    `json:"title"`
			Description *string    `json:"description"`
			BannerURL   *string    `json:"bannerUrl"`
			Category    *string    `json:"category"`
			BatchID     **string   `json:"batchId"`
			IsPublished *bool      `json:"isPublished"`
			StartDate   *time.Time `json:"startDate"`
			EndDate     *time.Time `json:"endDate"`
			CourseID    *int64     `json:"courseId"`
			IsFree      *bool      `json:"isFree"`
			Price       *float64   `json:"price"`
			Slug        *string    `json:"slug"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil, err
		}

		if req.Title != nil {
			ts.Title = *req.Title
		}
		if req.Description != nil {
			ts.Description = *req.Description
		}
		if req.BannerURL != nil {
			ts.BannerURL = *req.BannerURL
		}
		if req.Category != nil {
			ts.Category = *req.Category
		}
		if req.BatchID != nil {
			ts.BatchID = *req.BatchID
		}
		if req.IsPublished != nil {
			ts.IsPublished = *req.IsPublished
		}
		if req.StartDate != nil {
			ts.StartDate = req.StartDate
		}
		if req.EndDate != nil {
			ts.EndDate = req.EndDate
		}
		if req.CourseID != nil {
			ts.CourseID = req.CourseID
		}
		if req.IsFree != nil {
			ts.IsFree = *req.IsFree
		}
		if req.Price != nil {
			ts.Price = *req.Price
		}
		if req.Slug != nil {
			ts.Slug = *req.Slug
		}
	}

	return ts, nil
}


func (h *httpHandler) CreateTest(c *gin.Context) {
	seriesIDStr := c.Param("id")
	seriesID, err := strconv.ParseInt(seriesIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test series ID"})
		return
	}

	var req struct {
		Title           string  `json:"title" binding:"required"`
		Description     string  `json:"description" binding:"required"`
		DurationMinutes int     `json:"durationMinutes" binding:"required"`
		TotalMarks      int     `json:"totalMarks" binding:"required"`
		NegativeMarking bool    `json:"negativeMarking"`
		Instructions    *string `json:"instructions"`
		IsPublished     bool    `json:"isPublished"`
		Slug            string  `json:"slug"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	test := &domain.Test{
		Title:           req.Title,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		TotalMarks:      req.TotalMarks,
		NegativeMarking: req.NegativeMarking,
		Instructions:    req.Instructions,
		IsPublished:     req.IsPublished,
		TestSeriesID:    seriesID,
		Slug:            req.Slug,
	}

	if err := h.uc.CreateTest(c.Request.Context(), test); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, test)
}

func (h *httpHandler) CreateTestLegacy(c *gin.Context) {
	var req struct {
		TestSeries      int64   `json:"testSeries" binding:"required"`
		Title           string  `json:"title" binding:"required"`
		Description     string  `json:"description" binding:"required"`
		DurationMinutes int     `json:"durationMinutes" binding:"required"`
		TotalMarks      int     `json:"totalMarks" binding:"required"`
		NegativeMarking bool    `json:"negativeMarking"`
		Instructions    *string `json:"instructions"`
		IsPublished     bool    `json:"isPublished"`
		Slug            string  `json:"slug"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	test := &domain.Test{
		Title:           req.Title,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		TotalMarks:      req.TotalMarks,
		NegativeMarking: req.NegativeMarking,
		Instructions:    req.Instructions,
		IsPublished:     req.IsPublished,
		TestSeriesID:    req.TestSeries,
		Slug:            req.Slug,
	}

	if err := h.uc.CreateTest(c.Request.Context(), test); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, test)
}


func (h *httpHandler) AddQuestion(c *gin.Context) {
	testIDStr := c.Param("id")
	testID, err := strconv.ParseInt(testIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test ID"})
		return
	}

	var req struct {
		QuestionType        string                  `json:"questionType" binding:"required"`
		QuestionText        *string                 `json:"questionText"`
		QuestionImageURL    *string                 `json:"questionImageUrl"`
		CorrectAnswer       *string                 `json:"correctAnswer"`
		Marks               int                     `json:"marks" binding:"required"`
		NegativeMarks       float64                 `json:"negativeMarks"`
		QuestionTimer       *int                    `json:"questionTimer"`
		ExplanationText     *string                 `json:"explanationText"`
		ExplanationImageURL *string                 `json:"explanationImageUrl"`
		Options             []domain.QuestionOption `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	q := &domain.Question{
		QuestionType:        req.QuestionType,
		QuestionText:        req.QuestionText,
		QuestionImageURL:    req.QuestionImageURL,
		CorrectAnswer:       req.CorrectAnswer,
		Marks:               req.Marks,
		NegativeMarks:       req.NegativeMarks,
		QuestionTimer:       req.QuestionTimer,
		ExplanationText:     req.ExplanationText,
		ExplanationImageURL: req.ExplanationImageURL,
		TestID:              testID,
	}

	if err := h.uc.AddQuestion(c.Request.Context(), q, req.Options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, q)
}

func saveUploadFile(c *gin.Context, fieldName, subDir string) (string, error) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		return "", err
	}
	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "./media"
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	targetDir := filepath.Join(mediaRoot, "test_series", subDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	dest := filepath.Join(targetDir, filename)
	if err := c.SaveUploadedFile(file, dest); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/media/test_series/%s/%s", baseURL, subDir, filename), nil
}

func (h *httpHandler) GetTestDetails(c *gin.Context) {
	idOrSlug := c.Param("id")
	test, err := h.uc.GetTestByIDOrSlug(c.Request.Context(), idOrSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	if test == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Test not found"})
		return
	}
	c.JSON(http.StatusOK, test)
}

func (h *httpHandler) UpdateTest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test ID"})
		return
	}

	var req struct {
		Title           string  `json:"title" binding:"required"`
		Description     string  `json:"description" binding:"required"`
		DurationMinutes int     `json:"durationMinutes" binding:"required"`
		TotalMarks      int     `json:"totalMarks" binding:"required"`
		NegativeMarking bool    `json:"negativeMarking"`
		Instructions    *string `json:"instructions"`
		IsPublished     bool    `json:"isPublished"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	test := &domain.Test{
		Title:           req.Title,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		TotalMarks:      req.TotalMarks,
		NegativeMarking: req.NegativeMarking,
		Instructions:    req.Instructions,
		IsPublished:     req.IsPublished,
	}

	if err := h.uc.UpdateTest(c.Request.Context(), id, test); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, test)
}

func (h *httpHandler) DeleteTest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test ID"})
		return
	}

	if err := h.uc.DeleteTest(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "message": "Test successfully deleted"})
}

func (h *httpHandler) GetQuestions(c *gin.Context) {
	testIDStr := c.Query("test_id")
	if testIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "test_id is required"})
		return
	}
	testID, err := strconv.ParseInt(testIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test_id"})
		return
	}

	list, err := h.uc.GetQuestionsByTestID(c.Request.Context(), testID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, list)
}

func (h *httpHandler) CreateQuestion(c *gin.Context) {
	testIDStr := c.PostForm("test_id")
	if testIDStr == "" {
		testIDStr = c.PostForm("testId")
	}
	testID, err := strconv.ParseInt(testIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test_id"})
		return
	}

	questionType := c.PostForm("questionType")
	questionTextStr := c.PostForm("questionText")
	var questionText *string
	if questionTextStr != "" {
		questionText = &questionTextStr
	}

	correctAnswerStr := c.PostForm("correct_answer")
	var correctAnswer *string
	if correctAnswerStr != "" {
		correctAnswer = &correctAnswerStr
	}

	marksStr := c.PostForm("marks")
	marks, _ := strconv.Atoi(marksStr)
	if marks == 0 {
		marks = 2
	}

	negativeMarksStr := c.PostForm("negativeMarks")
	negativeMarks, _ := strconv.ParseFloat(negativeMarksStr, 64)

	questionTimerStr := c.PostForm("questionTimer")
	var questionTimer *int
	if questionTimerStr != "" {
		if parsed, err := strconv.Atoi(questionTimerStr); err == nil {
			questionTimer = &parsed
		}
	}

	explanationTextStr := c.PostForm("explanationText")
	var explanationText *string
	if explanationTextStr != "" {
		explanationText = &explanationTextStr
	}

	qImgURL, _ := saveUploadFile(c, "question_image", "questions")
	var qImgURLPtr *string
	if qImgURL != "" {
		qImgURLPtr = &qImgURL
	}

	expImgURL, _ := saveUploadFile(c, "explanation_image", "explanations")
	var expImgURLPtr *string
	if expImgURL != "" {
		expImgURLPtr = &expImgURL
	}

	q := &domain.Question{
		QuestionType:        questionType,
		QuestionText:        questionText,
		QuestionImageURL:    qImgURLPtr,
		CorrectAnswer:       correctAnswer,
		Marks:               marks,
		NegativeMarks:       negativeMarks,
		QuestionTimer:       questionTimer,
		ExplanationText:     explanationText,
		ExplanationImageURL: expImgURLPtr,
		TestID:              testID,
	}

	var options []domain.QuestionOption
	if questionType != "NAT" {
		optionKeys := []string{"option_a", "option_b", "option_c", "option_d"}
		correctList := []string{}
		if correctAnswerStr != "" {
			for _, item := range strings.Split(correctAnswerStr, ",") {
				correctList = append(correctList, strings.TrimSpace(strings.ToUpper(item)))
			}
		}

		for _, key := range optionKeys {
			letter := strings.ToUpper(strings.Split(key, "_")[1])
			optTextStr := c.PostForm(key)
			optImgURL, _ := saveUploadFile(c, key+"_image", "options")

			if optTextStr != "" || optImgURL != "" {
				isCorrect := false
				for _, item := range correctList {
					if item == letter {
						isCorrect = true
						break
					}
				}
				var optText *string
				if optTextStr != "" {
					optText = &optTextStr
				}
				var optImg *string
				if optImgURL != "" {
					optImg = &optImgURL
				}
				options = append(options, domain.QuestionOption{
					OptionText:     optText,
					OptionImageURL: optImg,
					IsCorrect:      isCorrect,
				})
			} else if key == "option_a" || key == "option_b" {
				c.JSON(http.StatusBadRequest, gin.H{"detail": "option_a and option_b are required for MCQ/MSQ"})
				return
			}
		}
	}

	if err := h.uc.CreateQuestion(c.Request.Context(), q, options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, q)
}

func (h *httpHandler) DeleteQuestion(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid question ID"})
		return
	}

	if err := h.uc.DeleteQuestion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "message": "Question successfully deleted"})
}

func (h *httpHandler) UploadQuestions(c *gin.Context) {
	testIDStr := c.Param("id")
	testID, err := strconv.ParseInt(testIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test ID"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "file is required"})
		return
	}

	filename := strings.ToLower(file.Filename)
	var dataList []map[string]interface{}

	fileReader, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "failed to open file"})
		return
	}
	defer fileReader.Close()

	if strings.HasSuffix(filename, ".json") {
		var list []map[string]interface{}
		if err := json.NewDecoder(fileReader).Decode(&list); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("invalid json file: %v", err)})
			return
		}
		dataList = list
	} else if strings.HasSuffix(filename, ".csv") {
		reader := csv.NewReader(fileReader)
		records, err := reader.ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("invalid csv file: %v", err)})
			return
		}
		if len(records) > 0 {
			headers := records[0]
			for _, row := range records[1:] {
				item := make(map[string]interface{})
				for idx, hName := range headers {
					if idx < len(row) {
						item[hName] = row[idx]
					}
				}
				dataList = append(dataList, item)
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "unsupported file format. Please upload .csv or .json file"})
		return
	}

	count, err := h.uc.UploadQuestions(c.Request.Context(), testID, dataList)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("Successfully uploaded %d questions.", count),
		"count":   count,
	})
}
