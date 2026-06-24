package http

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"clasynq/api/notes/internal/domain"

	"github.com/gin-gonic/gin"
)

type httpHandler struct {
	uc        domain.NoteUsecase
	mediaRoot string
	baseURL   string
}

func NewHttpHandler(uc domain.NoteUsecase, mediaRoot string, baseURL string) *httpHandler {
	return &httpHandler{
		uc:        uc,
		mediaRoot: mediaRoot,
		baseURL:   baseURL,
	}
}

func RegisterRoutes(
	r *gin.Engine,
	uc domain.NoteUsecase,
	mediaRoot string,
	baseURL string,
	authMiddleware gin.HandlerFunc,
	optionalAuthMiddleware gin.HandlerFunc,
) {
	handler := NewHttpHandler(uc, mediaRoot, baseURL)

	r.GET("/api/notes/", optionalAuthMiddleware, handler.GetNotes)
	r.GET("/api/notes/:id", optionalAuthMiddleware, handler.GetNoteDetails)

	auth := r.Group("/api/notes", authMiddleware)
	{
		auth.POST("/", AdminOrTeacherRequired(), handler.CreateNote)
		auth.PUT("/:id", AdminOrTeacherRequired(), handler.UpdateNote)
		auth.DELETE("/:id", AdminOrTeacherRequired(), handler.DeleteNote)
		auth.GET("/:id/access", handler.CheckAccess)
	}
}

func (h *httpHandler) GetNotes(c *gin.Context) {
	filters := map[string]string{
		"category": c.Query("category"),
		"courseId": c.Query("courseId"),
		"batchId":  c.Query("batchId"),
		"isFree":   c.Query("isFree"),
		"search":   c.Query("search"),
	}

	userID := int64(0)
	role := ""
	if uIDVal, exists := c.Get("userID"); exists {
		userID = uIDVal.(int64)
	}
	if rVal, exists := c.Get("role"); exists {
		role = rVal.(string)
	}

	notes, err := h.uc.GetNotes(c.Request.Context(), userID, role, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

func (h *httpHandler) GetNoteDetails(c *gin.Context) {
	idOrSlug := c.Param("id")

	userID := int64(0)
	role := ""
	if uIDVal, exists := c.Get("userID"); exists {
		userID = uIDVal.(int64)
	}
	if rVal, exists := c.Get("role"); exists {
		role = rVal.(string)
	}

	note, hasAccess, err := h.uc.GetNoteByIDOrSlug(c.Request.Context(), userID, role, idOrSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	if note == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Note not found"})
		return
	}

	// For frontend integration support, return note along with hasAccess flag
	c.JSON(http.StatusOK, gin.H{
		"note":      note,
		"hasAccess": hasAccess,
	})
}

func (h *httpHandler) CreateNote(c *gin.Context) {
	title := c.PostForm("title")
	description := c.PostForm("description")
	noteType := c.PostForm("noteType")
	isFree, _ := strconv.ParseBool(c.PostForm("isFree"))
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	batchID := c.PostForm("batchId")
	category := c.PostForm("category")
	slug := c.PostForm("slug")
	hasSvgs, _ := strconv.ParseBool(c.PostForm("hasSvgs"))
	pageCount, _ := strconv.Atoi(c.PostForm("pageCount"))

	var courseID *int64
	if cIDStr := c.PostForm("courseId"); cIDStr != "" {
		if cID, err := strconv.ParseInt(cIDStr, 10, 64); err == nil {
			courseID = &cID
		}
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "PDF note file is required"})
		return
	}

	// 1. Create target directory
	notesDir := filepath.Join(h.mediaRoot, "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": fmt.Sprintf("failed to create notes directory: %v", err)})
		return
	}

	// 2. Save file
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	dest := filepath.Join(notesDir, filename)
	if err := c.SaveUploadedFile(file, dest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": fmt.Sprintf("failed to save file: %v", err)})
		return
	}

	fileURL := fmt.Sprintf("%s/media/notes/%s", h.baseURL, filename)

	note := &domain.Note{
		Title:       title,
		Description: description,
		NoteType:    noteType,
		IsFree:      isFree,
		Price:       price,
		BatchID:     batchID,
		FileURL:     fileURL,
		CourseID:    courseID,
		HasSvgs:     hasSvgs,
		PageCount:   pageCount,
		Category:    category,
		Slug:        slug,
	}

	if err := h.uc.CreateNote(c.Request.Context(), note); err != nil {
		// Clean up saved file if DB insert fails
		_ = os.Remove(dest)
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, note)
}

func (h *httpHandler) UpdateNote(c *gin.Context) {
	idOrSlug := c.Param("id")

	// Read fields from multipart form
	updates := make(map[string]interface{})

	if val := c.PostForm("title"); val != "" {
		updates["title"] = val
	}
	if val := c.PostForm("description"); val != "" {
		updates["description"] = val
	}
	if val := c.PostForm("noteType"); val != "" {
		updates["noteType"] = val
	}
	if val := c.PostForm("isFree"); val != "" {
		isFree, err := strconv.ParseBool(val)
		if err == nil {
			updates["isFree"] = isFree
		}
	}
	if val := c.PostForm("price"); val != "" {
		price, err := strconv.ParseFloat(val, 64)
		if err == nil {
			updates["price"] = price
		}
	}
	if val := c.PostForm("batchId"); val != "" {
		updates["batchId"] = val
	}
	if val := c.PostForm("category"); val != "" {
		updates["category"] = val
	}
	if val := c.PostForm("slug"); val != "" {
		updates["slug"] = val
	}
	if val := c.PostForm("hasSvgs"); val != "" {
		hasSvgs, err := strconv.ParseBool(val)
		if err == nil {
			updates["hasSvgs"] = hasSvgs
		}
	}
	if val := c.PostForm("pageCount"); val != "" {
		pageCount, err := strconv.Atoi(val)
		if err == nil {
			updates["pageCount"] = float64(pageCount)
		}
	}
	if val := c.PostForm("courseId"); val != "" {
		if val == "null" || val == "none" {
			updates["courseId"] = nil
		} else {
			courseID, err := strconv.ParseInt(val, 10, 64)
			if err == nil {
				updates["courseId"] = float64(courseID)
			}
		}
	}

	// File check
	file, err := c.FormFile("file")
	if err == nil && file != nil {
		notesDir := filepath.Join(h.mediaRoot, "notes")
		_ = os.MkdirAll(notesDir, 0755)

		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
		dest := filepath.Join(notesDir, filename)
		if err := c.SaveUploadedFile(file, dest); err == nil {
			updates["fileUrl"] = fmt.Sprintf("%s/media/notes/%s", h.baseURL, filename)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": fmt.Sprintf("failed to save uploaded file: %v", err)})
			return
		}
	}

	note, err := h.uc.UpdateNote(c.Request.Context(), idOrSlug, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, note)
}

func (h *httpHandler) DeleteNote(c *gin.Context) {
	idOrSlug := c.Param("id")

	if err := h.uc.DeleteNote(c.Request.Context(), idOrSlug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note successfully deleted"})
}

func (h *httpHandler) CheckAccess(c *gin.Context) {
	idOrSlug := c.Param("id")

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
		return
	}
	userID := userIDVal.(int64)

	roleVal, exists := c.Get("role")
	role := ""
	if exists {
		role = roleVal.(string)
	}

	hasAccess, err := h.uc.HasAccess(c.Request.Context(), userID, role, idOrSlug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hasAccess": hasAccess,
	})
}
