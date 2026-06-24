package http

import (
	"log"
	"net/http"
	"strconv"

	"clasynq/api/enrollments/internal/domain"

	"github.com/gin-gonic/gin"
)

type enrollmentHandler struct {
	uc domain.EnrollmentUsecase
}

func RegisterRoutes(
	r *gin.Engine,
	uc domain.EnrollmentUsecase,
	authMiddleware gin.HandlerFunc,
) {
	handler := &enrollmentHandler{uc: uc}

	// Webhook endpoint (unauthenticated, Razorpay calls directly)
	r.POST("/api/payments/webhook/", handler.HandleWebhook)

	// Authenticated payment routes
	auth := r.Group("/api/payments", authMiddleware)
	{
		auth.POST("/referral/validate/", handler.ValidateReferral)
		auth.POST("/order/", handler.CreateOrder)
		auth.POST("/verify/", handler.VerifyPayment)
		auth.POST("/order/:id/refund/", AdminRequired(), handler.RefundOrder)

		// Compatibility paths for Notes and Test Series payments
		auth.POST("/notes/:id/order/create/", handler.CreateNoteOrder)
		auth.POST("/notes/:id/order/verify/", handler.VerifyPayment)
		auth.POST("/test-series/:id/order/create/", handler.CreateTestSeriesOrder)
		auth.POST("/test-series/:id/order/verify/", handler.VerifyPayment)
	}
}

func (h *enrollmentHandler) ValidateReferral(c *gin.Context) {
	var req struct {
		ReferralCode string `json:"referralCode" binding:"required"`
		CourseID     int64  `json:"courseId" binding:"required"`
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
	buyerID := userIDVal.(int64)

	buyerIP := c.ClientIP()

	res, err := h.uc.ValidateReferral(c.Request.Context(), buyerID, buyerIP, req.ReferralCode, req.CourseID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *enrollmentHandler) CreateOrder(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	buyerID := userIDVal.(int64)

	buyerIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	res, err := h.uc.CreateOrder(c.Request.Context(), buyerID, buyerIP, userAgent, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *enrollmentHandler) VerifyPayment(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	buyerID := userIDVal.(int64)

	res, err := h.uc.VerifyPayment(c.Request.Context(), buyerID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *enrollmentHandler) HandleWebhook(c *gin.Context) {
	signature := c.GetHeader("X-Razorpay-Signature")
	if signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "missing X-Razorpay-Signature header"})
		return
	}

	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "failed to read request body"})
		return
	}

	err = h.uc.HandleWebhook(c.Request.Context(), rawBody, signature)
	if err != nil {
		log.Printf("Webhook processing error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "processed"})
}

func (h *enrollmentHandler) RefundOrder(c *gin.Context) {
	idStr := c.Param("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid order ID"})
		return
	}

	err = h.uc.RefundOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "refunded", "message": "Order successfully refunded"})
}

func (h *enrollmentHandler) CreateNoteOrder(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		req = make(map[string]interface{})
	}
	noteIDStr := c.Param("id")
	noteID, err := strconv.ParseInt(noteIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid note id"})
		return
	}
	req["orderType"] = "note"
	req["noteId"] = float64(noteID)

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	buyerID := userIDVal.(int64)

	buyerIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	res, err := h.uc.CreateOrder(c.Request.Context(), buyerID, buyerIP, userAgent, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *enrollmentHandler) CreateTestSeriesOrder(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		req = make(map[string]interface{})
	}
	tsIDStr := c.Param("id")
	tsID, err := strconv.ParseInt(tsIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid test series id"})
		return
	}
	req["orderType"] = "test_series"
	req["testSeriesId"] = float64(tsID)

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	buyerID := userIDVal.(int64)

	buyerIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	res, err := h.uc.CreateOrder(c.Request.Context(), buyerID, buyerIP, userAgent, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
