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
		auth.POST("/", AdminRequired(), handler.CreateTestSeries)
		auth.POST("/:id/tests", AdminRequired(), handler.CreateTest)
		auth.POST("/tests/:id/questions", AdminRequired(), handler.AddQuestion)
	}

	// Legacy / CRUD routes for tests
	tests := r.Group("/api/tests", authMiddleware)
	{
		tests.GET("/:id/", handler.GetTestDetails)
		tests.PUT("/:id/", AdminRequired(), handler.UpdateTest)
		tests.DELETE("/:id/", AdminRequired(), handler.DeleteTest)
		tests.POST("/:id/upload_questions/", AdminRequired(), handler.UploadQuestions)
	}

	// Legacy / CRUD routes for questions
	questions := r.Group("/api/questions", authMiddleware)
	{
		questions.GET("/", handler.GetQuestions)
		questions.POST("/", AdminRequired(), handler.CreateQuestion)
		questions.DELETE("/:id/", AdminRequired(), handler.DeleteQuestion)
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

	c.JSON(http.StatusOK, gin.H{
		"testSeries": ts,
		"hasAccess":  hasAccess,
	})
}

func (h *httpHandler) CreateTestSeries(c *gin.Context) {
	var req struct {
		Title       string     `json:"title" binding:"required"`
		Description string     `json:"description" binding:"required"`
		BannerURL   string     `json:"bannerUrl" binding:"required"`
		Category    string     `json:"category" binding:"required"`
		BatchID     *string    `json:"batchId"`
		IsPublished bool       `json:"isPublished"`
		StartDate   *time.Time `json:"startDate"`
		EndDate     *time.Time `json:"endDate"`
		CourseID    *int64     `json:"courseId"`
		IsFree      bool       `json:"isFree"`
		Price       float64    `json:"price" binding:"required"`
		Slug        string     `json:"slug"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	ts := &domain.TestSeries{
		Title:       req.Title,
		Description: req.Description,
		BannerURL:   req.BannerURL,
		Category:    req.Category,
		BatchID:     req.BatchID,
		IsPublished: req.IsPublished,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		CourseID:    req.CourseID,
		IsFree:      req.IsFree,
		Price:       req.Price,
		Slug:        req.Slug,
	}

	if err := h.uc.CreateTestSeries(c.Request.Context(), ts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ts)
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
