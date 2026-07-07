package http

import (
	"net/http"
	"strconv"
	"time"

	"clasynq/api/cbt_exam/internal/domain"

	"github.com/gin-gonic/gin"
)

// httpHandler handles HTTP requests for CBT (Computer Based Test) Exams.
type httpHandler struct {
	uc domain.CbtExamUsecase
}

// NewHttpHandler initializes the CBT httpHandler.
func NewHttpHandler(uc domain.CbtExamUsecase) *httpHandler {
	return &httpHandler{uc: uc}
}

// RegisterRoutes registers routes to the Gin engine.
// Maps endpoints for beginning attempts, saving questions, submitting results,
// querying leaderboards, and legacy monolith compatibilities.
func RegisterRoutes(
	r *gin.Engine,
	uc domain.CbtExamUsecase,
	authMiddleware gin.HandlerFunc,
	optionalAuthMiddleware gin.HandlerFunc,
) {
	handler := NewHttpHandler(uc)

	r.GET("/api/cbt/tests/:id/leaderboard/", optionalAuthMiddleware, handler.GetLeaderboard)

	auth := r.Group("/api/cbt", authMiddleware)
	{
		auth.POST("/tests/:id/start/", handler.StartAttempt)
		auth.POST("/attempts/:slug/answers/", handler.SubmitAnswer)
		auth.POST("/attempts/:slug/submit/", handler.SubmitTest)
		auth.GET("/attempts/:slug/result", handler.GetAttemptResult)
	}

	// Legacy frontend compatibility routes supporting older request payloads
	r.POST("/api/test-attempts/start/", authMiddleware, handler.StartAttemptLegacy)
	r.POST("/api/test-attempts/submit/", authMiddleware, handler.SubmitTestLegacy)
	r.GET("/api/results/:id/", authMiddleware, handler.GetAttemptResultLegacy)
	r.GET("/api/tests/:id/attempts_monitoring/", authMiddleware, handler.GetAttemptsMonitoring)
}

// StartAttempt starts or resumes a test attempt for a student.
func (h *httpHandler) StartAttempt(c *gin.Context) {
	testIDOrSlug := c.Param("id")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	attempt, questions, err := h.uc.StartAttempt(c.Request.Context(), userID, testIDOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"attempt":   attempt,
		"questions": questions,
	})
}

// SubmitAnswer saves a single question answer choice.
func (h *httpHandler) SubmitAnswer(c *gin.Context) {
	attemptSlug := c.Param("slug")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	var req struct {
		QuestionID     int64  `json:"questionId" binding:"required"`
		SelectedAnswer string `json:"selectedAnswer" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	ans, err := h.uc.SubmitAnswer(c.Request.Context(), userID, attemptSlug, req.QuestionID, req.SelectedAnswer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ans)
}

// SubmitTest completes and scores the overall assessment attempt.
func (h *httpHandler) SubmitTest(c *gin.Context) {
	attemptSlug := c.Param("slug")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	res, err := h.uc.SubmitTest(c.Request.Context(), userID, attemptSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetAttemptResult returns calculation details for a completed attempt.
func (h *httpHandler) GetAttemptResult(c *gin.Context) {
	attemptSlug := c.Param("slug")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	res, err := h.uc.GetAttemptResult(c.Request.Context(), userID, attemptSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetLeaderboard displays top-scoring users for a test.
func (h *httpHandler) GetLeaderboard(c *gin.Context) {
	testIDOrSlug := c.Param("id")

	leaderboard, err := h.uc.GetLeaderboard(c.Request.Context(), testIDOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, leaderboard)
}

// StartAttemptLegacy processes legacy JSON formats (parsing test_id as float64 or string).
func (h *httpHandler) StartAttemptLegacy(c *gin.Context) {
	var req struct {
		TestID interface{} `json:"test_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	var testIDStr string
	switch v := req.TestID.(type) {
	case string:
		testIDStr = v
	case float64:
		testIDStr = strconv.FormatInt(int64(v), 10)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test_id format"})
		return
	}

	attempt, questions, err := h.uc.StartAttempt(c.Request.Context(), userID, testIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// Calculate remaining exam seconds
	test, err := h.uc.GetTestByIDOrSlug(c.Request.Context(), testIDStr)
	remainingSeconds := 0
	if err == nil && test != nil {
		duration := time.Duration(test.DurationMinutes) * time.Minute
		elapsed := time.Since(attempt.StartedAt)
		rem := duration - elapsed
		if rem > 0 {
			remainingSeconds = int(rem.Seconds())
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"attempt":          attempt,
		"questions":        questions,
		"remainingSeconds": remainingSeconds,
	})
}

type legacyAnswer struct {
	QuestionID     int64  `json:"question_id"`
	SelectedAnswer string `json:"selected_answer"`
}

type legacySubmitReq struct {
	AttemptID interface{}    `json:"attempt_id"`
	Answers   []legacyAnswer `json:"answers"`
}

// SubmitTestLegacy handles bulk answer submissions and finalizes attempts.
func (h *httpHandler) SubmitTestLegacy(c *gin.Context) {
	var req legacySubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	var attemptIDStr string
	switch v := req.AttemptID.(type) {
	case string:
		attemptIDStr = v
	case float64:
		attemptIDStr = strconv.FormatInt(int64(v), 10)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid attempt_id format"})
		return
	}

	// Submit all answers sequentially
	for _, ans := range req.Answers {
		if ans.SelectedAnswer != "" {
			_, _ = h.uc.SubmitAnswer(c.Request.Context(), userID, attemptIDStr, ans.QuestionID, ans.SelectedAnswer)
		}
	}

	// Submit and grade the attempt
	res, err := h.uc.SubmitTest(c.Request.Context(), userID, attemptIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetAttemptResultLegacy retrieves attempt details using legacy URL params.
func (h *httpHandler) GetAttemptResultLegacy(c *gin.Context) {
	attemptIDOrSlug := c.Param("id")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	res, err := h.uc.GetAttemptResult(c.Request.Context(), userID, attemptIDOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetAttemptsMonitoring fetches progress logs for monitoring.
func (h *httpHandler) GetAttemptsMonitoring(c *gin.Context) {
	testIDStr := c.Param("id")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	res, err := h.uc.GetAttemptsMonitoring(c.Request.Context(), userID, testIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

