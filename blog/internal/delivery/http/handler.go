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
		api.GET("/", optionalAuth, h.GetFeed)
		api.GET("/feed/following", optionalAuth, h.GetFeed)
		api.GET("/feed/following/", optionalAuth, h.GetFeed)
		api.GET("/feed/trending", optionalAuth, h.GetFeed)
		api.GET("/feed/trending/", optionalAuth, h.GetFeed)
		api.GET("/feed/recommended", optionalAuth, h.GetFeed)
		api.GET("/feed/recommended/", optionalAuth, h.GetFeed)
		api.GET("/:slug", optionalAuth, h.GetPostDetail)
		api.GET("/:slug/comments", optionalAuth, h.GetComments)
		api.GET("/:slug/comments/", optionalAuth, h.GetComments)
		api.GET("/posts/:id/comments", optionalAuth, h.GetComments)
		api.GET("/posts/:id/comments/", optionalAuth, h.GetComments)

		// Mutations (requires authentication)
		api.Use(authMiddleware)
		{
			api.GET("/activity", h.GetUserActivities)
			api.GET("/activity/", h.GetUserActivities)
			api.POST("/follow", h.FollowToggle)
			api.POST("/follow/", h.FollowToggle)
			api.POST("/users/:id/follow", h.FollowToggle)
			api.POST("/users/:id/follow/", h.FollowToggle)
			api.POST("", h.CreatePost)
			api.PUT("/:slug", h.UpdatePost)
			api.DELETE("/:slug", h.DeletePost)

			// Interactions (Slug based)
			api.POST("/:slug/like", h.ToggleLike)
			api.POST("/:slug/like/", h.ToggleLike)
			api.DELETE("/:slug/like", h.ToggleLike)
			api.DELETE("/:slug/like/", h.ToggleLike)
			api.POST("/:slug/save", h.ToggleSave)
			api.POST("/:slug/save/", h.ToggleSave)
			api.DELETE("/:slug/save", h.ToggleSave)
			api.DELETE("/:slug/save/", h.ToggleSave)
			api.POST("/:slug/repost", h.ToggleRepost)
			api.POST("/:slug/repost/", h.ToggleRepost)
			api.DELETE("/:slug/repost", h.ToggleRepost)
			api.DELETE("/:slug/repost/", h.ToggleRepost)

			// Interactions (ID based)
			api.POST("/posts/:id/like", h.ToggleLike)
			api.POST("/posts/:id/like/", h.ToggleLike)
			api.DELETE("/posts/:id/like", h.ToggleLike)
			api.DELETE("/posts/:id/like/", h.ToggleLike)
			api.POST("/posts/:id/save", h.ToggleSave)
			api.POST("/posts/:id/save/", h.ToggleSave)
			api.DELETE("/posts/:id/save", h.ToggleSave)
			api.DELETE("/posts/:id/save/", h.ToggleSave)
			api.POST("/posts/:id/repost", h.ToggleRepost)
			api.POST("/posts/:id/repost/", h.ToggleRepost)
			api.DELETE("/posts/:id/repost", h.ToggleRepost)
			api.DELETE("/posts/:id/repost/", h.ToggleRepost)

			// View and Engagement tracking
			api.POST("/posts/:id/view", h.TrackView)
			api.POST("/posts/:id/view/", h.TrackView)
			api.POST("/posts/:id/engagement", h.TrackEngagement)
			api.POST("/posts/:id/engagement/", h.TrackEngagement)

			// Comments
			api.POST("/:slug/comment", h.AddComment)
			api.POST("/:slug/comment/", h.AddComment)
			api.POST("/:slug/comments", h.AddComment)
			api.POST("/:slug/comments/", h.AddComment)
			api.POST("/posts/:id/comment", h.AddComment)
			api.POST("/posts/:id/comment/", h.AddComment)
			api.POST("/posts/:id/comments", h.AddComment)
			api.POST("/posts/:id/comments/", h.AddComment)
			api.DELETE("/comments/:id", h.DeleteComment)
			api.DELETE("/comments/:id/", h.DeleteComment)

			// Admin operations
			admin := api.Group("/admin")
			admin.Use(RequireAdmin())
			{
				admin.GET("/posts", h.GetAdminPosts)
				admin.PATCH("/posts/:id", h.UpdateAdminBlogPost)
				admin.DELETE("/posts/:id", h.DeleteAdminBlogPost)
			}
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
	if tab == "" {
		path := c.Request.URL.Path
		if strings.Contains(path, "/feed/following") {
			tab = "following"
		} else if strings.Contains(path, "/feed/trending") {
			tab = "trending"
		} else if strings.Contains(path, "/feed/recommended") {
			tab = "recommended"
		}
	}
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
	if slugOrID == "" {
		slugOrID = c.Param("id")
	}
	if slugOrID == "" {
		return 0, fmt.Errorf("post identifier (slug or id) is missing")
	}
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

func (h *HttpHandler) GetComments(c *gin.Context) {
	postID, err := h.resolvePostID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	comments, err := h.usecase.GetCommentsForPost(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"comments": comments})
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

func (h *HttpHandler) GetAdminPosts(c *gin.Context) {
	q := c.Query("q")
	userSearch := c.Query("user_search")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	res, err := h.usecase.GetAdminPosts(c.Request.Context(), q, userSearch, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "An error occurred.", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *HttpHandler) UpdateAdminBlogPost(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID."})
		return
	}

	var updates map[string]interface{}
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		updates = make(map[string]interface{})
		if val, ok := c.GetPostForm("is_restricted"); ok {
			updates["is_restricted"] = val == "true"
		}
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

	res, err := h.usecase.UpdatePostAsAdmin(c.Request.Context(), id, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res["post"])
}

func (h *HttpHandler) DeleteAdminBlogPost(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID."})
		return
	}

	err = h.usecase.DeletePostAsAdmin(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *HttpHandler) GetUserActivities(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
		return
	}

	res, err := h.usecase.GetUserActivities(c.Request.Context(), userID, 100)
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
	followerID := getUserIDFromCtx(c)
	if followerID == 0 {
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

	isFollowing, err := h.usecase.ToggleFollowUser(c.Request.Context(), followerID, followedID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isFollowing": isFollowing,
		"message":     "Follow state updated.",
	})
}

func (h *HttpHandler) TrackView(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	idStr := c.Param("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID."})
		return
	}

	viewerIdentifier := c.ClientIP()
	if userAgent := c.GetHeader("User-Agent"); userAgent != "" {
		viewerIdentifier = viewerIdentifier + ":" + userAgent
	}

	viewsCount, err := h.usecase.TrackPostView(c.Request.Context(), postID, viewerIdentifier, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"viewsCount": viewsCount,
		"message":    "View tracked successfully.",
	})
}

type engagementReq struct {
	ReadTimeSeconds  int `json:"read_time_seconds"`
	ReadTimeSeconds2 int `json:"readTimeSeconds"`
}

func (h *HttpHandler) TrackEngagement(c *gin.Context) {
	userID := getUserIDFromCtx(c)
	idStr := c.Param("id")
	postID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid post ID."})
		return
	}

	var req engagementReq
	_ = c.ShouldBindJSON(&req)

	readTimeSeconds := req.ReadTimeSeconds
	if readTimeSeconds == 0 {
		readTimeSeconds = req.ReadTimeSeconds2
	}

	viewerIdentifier := c.ClientIP()
	if userAgent := c.GetHeader("User-Agent"); userAgent != "" {
		viewerIdentifier = viewerIdentifier + ":" + userAgent
	}

	engagementScore, err := h.usecase.TrackPostEngagement(c.Request.Context(), postID, readTimeSeconds, viewerIdentifier, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"engagementScore": engagementScore,
		"message":         "Engagement tracked successfully.",
	})
}
