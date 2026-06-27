package http

import (
	"fmt"
	"net/http"

	"clasynq/api/auth/internal/domain"
	"clasynq/api/auth/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type HttpHandler struct {
	usecase            domain.UserUsecase
	secretKey          string
	turnstileSecretKey string
	rdb                *redis.Client
}

func NewHttpHandler(usecase domain.UserUsecase, secretKey, turnstileSecretKey string, rdb *redis.Client) *HttpHandler {
	return &HttpHandler{
		usecase:            usecase,
		secretKey:          secretKey,
		turnstileSecretKey: turnstileSecretKey,
		rdb:                rdb,
	}
}

func (h *HttpHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", h.Register)
			auth.POST("/verify-otp", h.VerifyOTP)
			auth.POST("/resend-otp", h.ResendOTP)
			auth.POST("/login", h.Login)
			auth.POST("/verify-login-2fa", h.VerifyLogin2FA)
			auth.POST("/logout", authMiddleware, h.Logout)
			auth.POST("/refresh", h.Refresh)
			auth.POST("/forgot-password", h.ForgotPassword)
			auth.POST("/reset-password", h.ResetPassword)
			auth.GET("/me", authMiddleware, h.GetMe)
		}

		me := api.Group("/me")
		me.Use(authMiddleware)
		{
			me.GET("", h.GetMe)
			me.GET("/", h.GetMe)
			me.PUT("", h.UpdateMe)
			me.PUT("/", h.UpdateMe)
			me.PUT("/change-password", h.ChangePassword)
			me.PUT("/change-password/", h.ChangePassword)
			me.POST("/follow/:id", h.FollowUser)
			me.POST("/follow/:id/", h.FollowUser)
			me.DELETE("/unfollow/:id", h.UnfollowUser)
			me.DELETE("/unfollow/:id/", h.UnfollowUser)
			me.GET("/notifications", h.GetNotifications)
			me.GET("/notifications/", h.GetNotifications)
			me.POST("/notifications/read", h.MarkNotificationsAsRead)
			me.POST("/notifications/read/", h.MarkNotificationsAsRead)
		}
	}
}

type registerReq struct {
	FullName       string `json:"fullName" binding:"required"`
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	ContactNumber  string `json:"contactNumber" binding:"required"`
	Password       string `json:"password" binding:"required,min=6"`
	TurnstileToken string `json:"turnstileToken"`
}

func (h *HttpHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input data.", "detail": err.Error()})
		return
	}

	remoteIP := c.ClientIP()
	turnstileToken := req.TurnstileToken
	if turnstileToken == "" {
		turnstileToken = c.GetHeader("X-Turnstile-Token")
	}

	if !utils.ValidateTurnstileToken(turnstileToken, remoteIP, h.turnstileSecretKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "captcha_failed",
			"message": "Security check failed. Please refresh and try again.",
			"data":    nil,
		})
		return
	}

	res, err := h.usecase.Register(c.Request.Context(), req.FullName, req.Username, req.Email, req.ContactNumber, req.Password, remoteIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	if code, ok := res["code"].(string); ok {
		if code == "email_send_failed" {
			c.JSON(http.StatusBadGateway, res)
			return
		}
		c.JSON(http.StatusConflict, res)
		return
	}

	c.JSON(http.StatusAccepted, res)
}

type verifyOtpReq struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func (h *HttpHandler) VerifyOTP(c *gin.Context) {
	var req verifyOtpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.VerifyOTP(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	if code, ok := res["code"].(string); ok {
		switch code {
		case "otp_no_pending_registration":
			c.JSON(http.StatusNotFound, res)
		case "otp_expired":
			c.JSON(http.StatusGone, res)
		case "otp_too_many_attempts":
			c.JSON(http.StatusTooManyRequests, res)
		case "otp_invalid":
			c.JSON(http.StatusBadRequest, res)
		default:
			c.JSON(http.StatusConflict, res)
		}
		return
	}

	c.JSON(http.StatusCreated, res)
}

type verifyLogin2FAReq struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (h *HttpHandler) VerifyLogin2FA(c *gin.Context) {
	var req verifyLogin2FAReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.VerifyLogin2FA(c.Request.Context(), req.Email, req.Code, req.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type resendOtpReq struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *HttpHandler) ResendOTP(c *gin.Context) {
	var req resendOtpReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.ResendOTP(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	if code, ok := res["code"].(string); ok {
		switch code {
		case "otp_no_pending_registration":
			c.JSON(http.StatusNotFound, res)
		case "otp_resend_cooldown":
			c.JSON(http.StatusTooManyRequests, res)
		case "otp_resend_limit":
			c.JSON(http.StatusTooManyRequests, res)
		default:
			c.JSON(http.StatusBadGateway, res)
		}
		return
	}

	c.JSON(http.StatusAccepted, res)
}

type loginReq struct {
	Email          string `json:"email" binding:"required"`
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstileToken"`
}

func (h *HttpHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid credentials format.", "detail": err.Error()})
		return
	}

	remoteIP := c.ClientIP()
	turnstileToken := req.TurnstileToken
	if turnstileToken == "" {
		turnstileToken = c.GetHeader("X-Turnstile-Token")
	}

	if !utils.ValidateTurnstileToken(turnstileToken, remoteIP, h.turnstileSecretKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "captcha_failed",
			"message": "Security check failed. Please refresh and try again.",
			"data":    nil,
		})
		return
	}

	res, err := h.usecase.Login(c.Request.Context(), req.Email, req.Password, remoteIP)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type logoutReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *HttpHandler) Logout(c *gin.Context) {
	// Best-effort blacklist/session termination is handled by removing JTI from Redis
	// Gin AuthMiddleware checks validity of JWT.
	c.Status(http.StatusNoContent)
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *HttpHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "refresh_token_required",
			"message": "Refresh token is required.",
			"data":    nil,
		})
		return
	}

	newTokens, err := h.usecase.TokenRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "token_not_valid",
			"message": err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, newTokens)
}

type forgotPasswordReq struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *HttpHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.ForgotPassword(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	if _, ok := res["code"].(string); ok {
		c.JSON(http.StatusNotFound, res)
		return
	}

	c.JSON(http.StatusAccepted, res)
}

type resetPasswordReq struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

func (h *HttpHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.ResetPassword(c.Request.Context(), req.Email, req.Code, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	if code, ok := res["code"].(string); ok {
		switch code {
		case "reset_no_pending_request":
			c.JSON(http.StatusNotFound, res)
		case "reset_expired":
			c.JSON(http.StatusGone, res)
		case "reset_too_many_attempts":
			c.JSON(http.StatusTooManyRequests, res)
		default:
			c.JSON(http.StatusBadRequest, res)
		}
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetMe(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	res, err := h.usecase.GetMe(c.Request.Context(), userID.(int64), role.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}

	// Disable caching
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, post-check=0, pre-check=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) UpdateMe(c *gin.Context) {
	userID, _ := c.Get("userID")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid inputs.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.UpdateMe(c.Request.Context(), userID.(int64), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type changePasswordReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

func (h *HttpHandler) ChangePassword(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid input.", "detail": err.Error()})
		return
	}

	err := h.usecase.ChangePassword(c.Request.Context(), userID.(int64), req.OldPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully."})
}

func (h *HttpHandler) FollowUser(c *gin.Context) {
	followerID, _ := c.Get("userID")
	followedIDStr := c.Param("id")

	var followedID int64
	_, err := fmt.Sscan(followedIDStr, &followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID format."})
		return
	}

	err = h.usecase.FollowUser(c.Request.Context(), followerID.(int64), followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully followed user."})
}

func (h *HttpHandler) UnfollowUser(c *gin.Context) {
	followerID, _ := c.Get("userID")
	followedIDStr := c.Param("id")

	var followedID int64
	_, err := fmt.Sscan(followedIDStr, &followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID format."})
		return
	}

	err = h.usecase.UnfollowUser(c.Request.Context(), followerID.(int64), followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unfollowed user."})
}

func (h *HttpHandler) GetNotifications(c *gin.Context) {
	userID, _ := c.Get("userID")
	roleVal, exists := c.Get("role")
	role := "student"
	if exists {
		role = roleVal.(string)
	}

	res, err := h.usecase.GetNotifications(c.Request.Context(), userID.(int64), role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	unreadCount := 0
	for _, n := range res {
		if !n.IsRead {
			unreadCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": res,
		"unreadCount":   unreadCount,
	})
}

func (h *HttpHandler) MarkNotificationsAsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	roleVal, exists := c.Get("role")
	role := "student"
	if exists {
		role = roleVal.(string)
	}

	err := h.usecase.MarkNotificationsAsRead(c.Request.Context(), userID.(int64), role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read."})
}
