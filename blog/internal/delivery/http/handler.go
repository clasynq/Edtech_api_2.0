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

	"clasynq/api/blog/internal/domain"
	"github.com/gin-gonic/gin"
)

type HttpHandler struct {
	usecase   domain.BlogUsecase
	mediaRoot string
	baseURL   string
}

func NewHttpHandler(usecase domain.BlogUsecase, mediaRoot string, baseURL string) *HttpHandler {
	return &HttpHandler{
		usecase:   usecase,
		mediaRoot: mediaRoot,
		baseURL:   baseURL,
	}
}

func (h *HttpHandler) RegisterRoutes(r *gin.Engine, authMiddleware gin.HandlerFunc, optionalAuth gin.HandlerFunc) {
	api := r.Group("/api/blog")
	{
		// Feed and Read details (supports optional authentication)
		api.GET("", optionalAuth, h.GetFeed)
		api.GET("/:slug", optionalAuth, h.GetPostDetail)

		// Mutations (requires authentication)
		api.Use(authMiddleware)
		{
			api.POST("", h.CreatePost)
			api.PUT("/:slug", h.UpdatePost)
			api.DELETE("/:slug", h.DeletePost)

			// Interactions
			api.POST("/:slug/like", h.ToggleLike)
			api.DELETE("/:slug/like", h.ToggleLike)
			api.POST("/:slug/save", h.ToggleSave)
			api.DELETE("/:slug/save", h.ToggleSave)
			api.POST("/:slug/repost", h.ToggleRepost)
			api.DELETE("/:slug/repost", h.ToggleRepost)

			// Comments
			api.POST("/:slug/comment", h.AddComment)
			api.DELETE("/comments/:id", h.DeleteComment)
		}
	}
}

func getUserIDFromCtx(c *gin.Context) int64 {
	val, exists := c.Get("userID")
	if !exists {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

func (h *HttpHandler) GetFeed(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	category := c.Query("category")
	query := c.Query("q")
	cursor := c.Query("cursor")
	tab := c.Query("tab")
	limitStr := c.DefaultQuery("limit", "20")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	res, err := h.usecase.GetFeed(c.Request.Context(), userID, category, query, cursor, tab, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) GetPostDetail(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	slug := c.Param("slug")
	viewerIP := c.ClientIP()

	res, err := h.usecase.GetPostDetail(c.Request.Context(), userID, slug, viewerIP)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type createPostReq struct {
	Title       string `json:"title" binding:"required"`
	Excerpt     string `json:"excerpt"`
	Content     string `json:"content" binding:"required"`
	Category    string `json:"category" binding:"required"`
	BannerURL   string `json:"bannerUrl"`
	ExploreLink string `json:"exploreLink"`
	ImageURL    string `json:"imageUrl"`
	VideoURL    string `json:"videoUrl"`
}

func (h *HttpHandler) CreatePost(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	var req createPostReq

	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		req.Title = c.PostForm("title")
		req.Excerpt = c.PostForm("excerpt")
		req.Content = c.PostForm("content")
		req.Category = c.PostForm("category")
		req.BannerURL = c.PostForm("bannerUrl")
		if req.BannerURL == "" {
			req.BannerURL = c.PostForm("banner_url")
		}
		req.ExploreLink = c.PostForm("exploreLink")
		if req.ExploreLink == "" {
			req.ExploreLink = c.PostForm("explore_link")
		}
		req.ImageURL = c.PostForm("imageUrl")
		if req.ImageURL == "" {
			req.ImageURL = c.PostForm("image_url")
		}
		req.VideoURL = c.PostForm("videoUrl")
		if req.VideoURL == "" {
			req.VideoURL = c.PostForm("video_url")
		}

		if req.Title == "" || req.Content == "" || req.Category == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid inputs.", "detail": "title, content, and category are required"})
			return
		}

		// Save banner locally if uploaded
		_, _, err := c.Request.FormFile("banner")
		if err == nil {
			fileURL, err := h.saveFileLocally(c, "banners")
			if err == nil {
				req.BannerURL = fileURL
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Banner upload failed.",
					"detail":  err.Error(),
				})
				return
			}
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid inputs.", "detail": err.Error()})
			return
		}
	}

	res, err := h.usecase.CreatePost(
		c.Request.Context(), userID, req.Title, req.Excerpt, req.Content, req.Category,
		req.BannerURL, req.ExploreLink, req.ImageURL, req.VideoURL,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) UpdatePost(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	slug := c.Param("slug")

	var updates map[string]interface{}
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		updates = make(map[string]interface{})
		if val, ok := c.GetPostForm("title"); ok { updates["title"] = val }
		if val, ok := c.GetPostForm("excerpt"); ok { updates["excerpt"] = val }
		if val, ok := c.GetPostForm("content"); ok { updates["content"] = val }
		if val, ok := c.GetPostForm("category"); ok { updates["category"] = val }
		if val, ok := c.GetPostForm("exploreLink"); ok { updates["exploreLink"] = val }
		if val, ok := c.GetPostForm("explore_link"); ok { updates["exploreLink"] = val }
		if val, ok := c.GetPostForm("imageUrl"); ok { updates["imageUrl"] = val }
		if val, ok := c.GetPostForm("image_url"); ok { updates["imageUrl"] = val }
		if val, ok := c.GetPostForm("videoUrl"); ok { updates["videoUrl"] = val }
		if val, ok := c.GetPostForm("video_url"); ok { updates["videoUrl"] = val }
		if val, ok := c.GetPostForm("tags"); ok {
			var tags []string
			if err := json.Unmarshal([]byte(val), &tags); err == nil {
				updates["tags"] = tags
			} else {
				parts := strings.Split(val, ",")
				var cleaned []string
				for _, p := range parts {
					if strings.TrimSpace(p) != "" {
						cleaned = append(cleaned, strings.TrimSpace(p))
					}
				}
				updates["tags"] = cleaned
			}
		}

		// Save banner locally if uploaded
		_, _, err := c.Request.FormFile("banner")
		if err == nil {
			fileURL, err := h.saveFileLocally(c, "banners")
			if err == nil {
				updates["bannerUrl"] = fileURL
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Banner upload failed.",
					"detail":  err.Error(),
				})
				return
			}
		} else if bannerFormVal, ok := c.GetPostForm("banner"); ok && bannerFormVal == "" {
			updates["bannerUrl"] = nil
		}
	} else {
		if err := c.ShouldBindJSON(&updates); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid updates input."})
			return
		}
	}

	res, err := h.usecase.UpdatePost(c.Request.Context(), userID, slug, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) DeletePost(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	slug := c.Param("slug")

	err := h.usecase.DeletePost(c.Request.Context(), userID, slug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *HttpHandler) resolvePostID(c *gin.Context) (int64, error) {
	slugOrID := c.Param("slug")
	postID, err := strconv.ParseInt(slugOrID, 10, 64)
	if err == nil {
		return postID, nil
	}
	return h.usecase.GetPostIDBySlug(c.Request.Context(), slugOrID)
}

func (h *HttpHandler) ToggleLike(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	postID, err := h.resolvePostID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.usecase.ToggleLike(c.Request.Context(), userID, postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) ToggleSave(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	postID, err := h.resolvePostID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.usecase.ToggleSave(c.Request.Context(), userID, postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) ToggleRepost(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	postID, err := h.resolvePostID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.usecase.ToggleRepost(c.Request.Context(), userID, postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type commentReq struct {
	Content  string `json:"content" binding:"required"`
	ParentID *int64 `json:"parentId"`
}

func (h *HttpHandler) AddComment(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	postID, err := h.resolvePostID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Content is required.", "detail": err.Error()})
		return
	}

	res, err := h.usecase.AddComment(c.Request.Context(), userID, postID, req.Content, req.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (h *HttpHandler) DeleteComment(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	idStr := c.Param("id")

	commentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid comment ID."})
		return
	}

	err = h.usecase.DeleteComment(c.Request.Context(), userID, commentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *HttpHandler) saveFileLocally(c *gin.Context, folder string) (string, error) {
	file, header, err := c.Request.FormFile("banner")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Ensure mediaRoot directory exists
	targetDir := filepath.Join(h.mediaRoot, folder)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	// Generate filename (original base + nano timestamp + extension)
	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(header.Filename, ext)
	filename := fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
	filePath := filepath.Join(targetDir, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copy contents
	if _, err = io.Copy(dst, file); err != nil {
		return "", err
	}

	// Return full backend media URL
	relPath := filepath.ToSlash(filepath.Join(folder, filename))
	baseMediaURL := strings.TrimSuffix(h.baseURL, "/")
	return fmt.Sprintf("%s/media/%s", baseMediaURL, relPath), nil
}
