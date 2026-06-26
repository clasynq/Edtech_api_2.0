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

	"clasynq/api/dashboard_profile/internal/domain"
	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	usecase domain.ProfileUsecase
}

func NewHttpHandler(usecase domain.ProfileUsecase) *HttpHandler {
	return &HttpHandler{usecase: usecase}
}

func (h *HttpHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	api := r.Group("/api")
	{
		me := api.Group("/me")
		me.Use(authMiddleware)
		{
			me.GET("", h.GetMe)
			me.GET("/", h.GetMe)
			me.PUT("", h.UpdateMe)
			me.PUT("/", h.UpdateMe)
			me.PATCH("", h.UpdateMe)
			me.PATCH("/", h.UpdateMe)
			me.GET("/mutual-connections", h.GetMutualConnections)
			me.GET("/mutual-connections/", h.GetMutualConnections)
			me.POST("/follow", h.FollowToggle)
			me.POST("/follow/", h.FollowToggle)
			me.POST("/users/:id/follow", h.FollowToggle)
			me.POST("/users/:id/follow/", h.FollowToggle)
			me.POST("/avatar", h.UploadAvatar)
			me.POST("/avatar/", h.UploadAvatar)
			me.GET("/study", h.GetStudyDashboard)
			me.GET("/study/", h.GetStudyDashboard)
			me.GET("/history", h.GetHistory)
			me.GET("/history/", h.GetHistory)
		}
	}
}

func (h *HttpHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "User ID not found in context."})
		return
	}
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Role not found in context."})
		return
	}

	res, err := h.usecase.GetMe(c.Request.Context(), userID.(int64), role.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) UpdateMe(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "User ID not found in context."})
		return
	}

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

type followReq struct {
	UserID  int64 `json:"userId"`
	UserID2 int64 `json:"user_id"`
}

func (h *HttpHandler) FollowToggle(c *gin.Context) {
	followerID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}

	var followedID int64
	idStr := c.Param("id")
	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user ID."})
			return
		}
		followedID = id
	} else {
		var req followReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid follow input."})
			return
		}
		if req.UserID != 0 {
			followedID = req.UserID
		} else {
			followedID = req.UserID2
		}
	}

	if followedID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "User ID to follow is required."})
		return
	}

	isFollowing, err := h.usecase.ToggleFollowUser(c.Request.Context(), followerID.(int64), followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isFollowing": isFollowing,
		"message":     "Follow state updated.",
	})
}

func (h *HttpHandler) GetMutualConnections(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}

	res, err := h.usecase.GetMutualConnections(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) UploadAvatar(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}
	roleVal, exists := c.Get("role")
	if exists && roleVal.(string) != "student" {
		c.JSON(http.StatusForbidden, gin.H{"message": "Worker profiles cannot be edited here."})
		return
	}

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "No avatar file provided.", "detail": err.Error()})
		return
	}
	defer file.Close()

	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "../media"
	}

	folder := "avatars"
	targetDir := filepath.Join(mediaRoot, folder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create directory.", "detail": err.Error()})
		return
	}

	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(header.Filename, ext)
	filename := fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
	filePath := filepath.Join(targetDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create destination file.", "detail": err.Error()})
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to copy file contents.", "detail": err.Error()})
		return
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	baseMediaURL := strings.TrimSuffix(baseURL, "/")
	relPath := filepath.ToSlash(filepath.Join(folder, filename))
	avatarURL := fmt.Sprintf("%s/media/%s", baseMediaURL, relPath)

	updates := map[string]interface{}{
		"avatarUrl": avatarURL,
	}
	res, err := h.usecase.UpdateMe(c.Request.Context(), userID.(int64), updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update profile.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": res,
	})
}

func (h *HttpHandler) GetStudyDashboard(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}
	category := c.Query("category")

	res, err := h.usecase.GetStudyDashboard(c.Request.Context(), userID.(int64), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetHistory(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}
	category := c.Query("category")

	res, err := h.usecase.GetHistory(c.Request.Context(), userID.(int64), category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
