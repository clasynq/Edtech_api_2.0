package http

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	// Webhook endpoints (unauthenticated, Razorpay calls directly)
	r.POST("/api/payments/webhook/", handler.HandleWebhook)
	r.POST("/api/payments/webhook", handler.HandleWebhook)
	r.POST("/payment/webhook/", handler.HandleWebhook)
	r.POST("/payment/webhook", handler.HandleWebhook)

	// Authenticated payment routes (/api/payments)
	authAPI := r.Group("/api/payments", authMiddleware)
	{
		authAPI.POST("/referral/validate/", handler.ValidateReferral)
		authAPI.POST("/referral/validate", handler.ValidateReferral)
		authAPI.POST("/order/", handler.CreateOrder)
		authAPI.POST("/order", handler.CreateOrder)
		authAPI.POST("/verify/", handler.VerifyPayment)
		authAPI.POST("/verify", handler.VerifyPayment)
		authAPI.POST("/order/:id/refund/", AdminRequired(), handler.RefundOrder)
		authAPI.POST("/order/:id/refund", AdminRequired(), handler.RefundOrder)

		// Coupon routes for Admin
		authAPI.GET("/coupons/", AdminRequired(), handler.ListCoupons)
		authAPI.GET("/coupons", AdminRequired(), handler.ListCoupons)
		authAPI.POST("/coupons/", AdminRequired(), handler.CreateCoupon)
		authAPI.POST("/coupons", AdminRequired(), handler.CreateCoupon)
		authAPI.DELETE("/coupons/:id/", AdminRequired(), handler.DeleteCoupon)
		authAPI.DELETE("/coupons/:id", AdminRequired(), handler.DeleteCoupon)

		// Coupon validation for Checkout (student)
		authAPI.POST("/coupons/validate/", handler.ValidateCoupon)
		authAPI.POST("/coupons/validate", handler.ValidateCoupon)

		// Compatibility paths for Notes and Test Series payments
		authAPI.POST("/notes/:id/order/create/", handler.CreateNoteOrder)
		authAPI.POST("/notes/:id/order/create", handler.CreateNoteOrder)
		authAPI.POST("/notes/:id/order/verify/", handler.VerifyPayment)
		authAPI.POST("/notes/:id/order/verify", handler.VerifyPayment)
		authAPI.POST("/test-series/:id/order/create/", handler.CreateTestSeriesOrder)
		authAPI.POST("/test-series/:id/order/create", handler.CreateTestSeriesOrder)
		authAPI.POST("/test-series/:id/order/verify/", handler.VerifyPayment)
		authAPI.POST("/test-series/:id/order/verify", handler.VerifyPayment)
	}

	// Authenticated legacy payment routes (/payment)
	authPayment := r.Group("/payment", authMiddleware)
	{
		authPayment.POST("/order/create/", handler.CreateOrder)
		authPayment.POST("/order/create", handler.CreateOrder)
		authPayment.POST("/order/verify/", handler.VerifyPayment)
		authPayment.POST("/order/verify", handler.VerifyPayment)
		authPayment.POST("/order/validate-referral/", handler.ValidateReferral)
		authPayment.POST("/order/validate-referral", handler.ValidateReferral)
		authPayment.POST("/orders/:id/refund/", AdminRequired(), handler.RefundOrder)
		authPayment.POST("/orders/:id/refund", AdminRequired(), handler.RefundOrder)
	}

	// Study Dashboard Enrollments
	me := r.Group("/api/me", authMiddleware)
	{
		me.GET("/enrollments/", handler.GetMyEnrollments)
		me.GET("/enrollments", handler.GetMyEnrollments)
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

func (h *enrollmentHandler) GetMyEnrollments(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	category := c.Query("category")

	res, err := h.uc.GetMyEnrollments(c.Request.Context(), userID, category)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *enrollmentHandler) ListCoupons(c *gin.Context) {
	coupons, err := h.uc.ListCoupons(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, coupons)
}

func (h *enrollmentHandler) CreateCoupon(c *gin.Context) {
	var req struct {
		Code               string `json:"code" binding:"required"`
		DiscountPercentage int    `json:"discountPercentage" binding:"required"`
		UserEmail          string `json:"userEmail" binding:"required"`
		ExpiresAt          string `json:"expiresAt" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	var expiresAt time.Time
	var parseErr error
	if strings.Contains(req.ExpiresAt, "T") && len(req.ExpiresAt) == 16 {
		istLoc := time.FixedZone("IST", 19800)
		expiresAt, parseErr = time.ParseInLocation("2006-01-02T15:04", req.ExpiresAt, istLoc)
	} else {
		expiresAt, parseErr = time.Parse(time.RFC3339, req.ExpiresAt)
	}

	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid expiresAt format"})
		return
	}

	coupon := &domain.Coupon{
		Code:               req.Code,
		DiscountPercentage: req.DiscountPercentage,
		UserEmail:          req.UserEmail,
		ExpiresAt:          expiresAt,
	}

	if err := h.uc.CreateCoupon(c.Request.Context(), coupon); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, coupon)
}

func (h *enrollmentHandler) DeleteCoupon(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid coupon ID"})
		return
	}

	if err := h.uc.DeleteCoupon(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "message": "Coupon successfully deleted"})
}

func (h *enrollmentHandler) ValidateCoupon(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
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

	coupon, err := h.uc.ValidateCoupon(c.Request.Context(), req.Code, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, coupon)
}
