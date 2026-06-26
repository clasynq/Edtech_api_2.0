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
	r.GET("/api/notes/public/", optionalAuthMiddleware, handler.GetPublicNotes)
	r.GET("/api/notes/public", optionalAuthMiddleware, handler.GetPublicNotes)
	r.GET("/api/notes/class-related/", optionalAuthMiddleware, handler.GetClassNotes)
	r.GET("/api/notes/class-related", optionalAuthMiddleware, handler.GetClassNotes)
	r.GET("/api/notes/:id", optionalAuthMiddleware, handler.GetNoteDetails)

	auth := r.Group("/api/notes", authMiddleware)
	{
		auth.GET("/admin/", AdminOrTeacherRequired(), handler.GetAdminNotes)
		auth.GET("/admin", AdminOrTeacherRequired(), handler.GetAdminNotes)
		auth.POST("/admin/", AdminOrTeacherRequired(), handler.CreateNote)
		auth.POST("/admin", AdminOrTeacherRequired(), handler.CreateNote)
		auth.PUT("/admin/:id/", AdminOrTeacherRequired(), handler.UpdateNote)
		auth.PUT("/admin/:id", AdminOrTeacherRequired(), handler.UpdateNote)
		auth.DELETE("/admin/:id/", AdminOrTeacherRequired(), handler.DeleteNote)
		auth.DELETE("/admin/:id", AdminOrTeacherRequired(), handler.DeleteNote)

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
	if noteType == "" {
		noteType = c.PostForm("note_type")
	}
	
	// Enforce: admins are not allowed to upload class notes
	roleVal, exists := c.Get("role")
	if exists {
		role := roleVal.(string)
		if role == "admin" && noteType == "class" {
			c.JSON(http.StatusForbidden, gin.H{"detail": "Admins are not allowed to upload class-related notes. Class notes can only be uploaded by teachers."})
			return
		}
	}
	
	isFreeStr := c.PostForm("isFree")
	if isFreeStr == "" {
		isFreeStr = c.PostForm("is_free")
	}
	isFree, _ := strconv.ParseBool(isFreeStr)
	
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	
	batchID := c.PostForm("batchId")
	if batchID == "" {
		batchID = c.PostForm("batch_id")
	}
	
	category := c.PostForm("category")
	slug := c.PostForm("slug")
	
	hasSvgsStr := c.PostForm("hasSvgs")
	if hasSvgsStr == "" {
		hasSvgsStr = c.PostForm("has_svgs")
	}
	hasSvgs, _ := strconv.ParseBool(hasSvgsStr)
	
	pageCountStr := c.PostForm("pageCount")
	if pageCountStr == "" {
		pageCountStr = c.PostForm("page_count")
	}
	pageCount, _ := strconv.Atoi(pageCountStr)

	var courseID *int64
	courseIDStr := c.PostForm("courseId")
	if courseIDStr == "" {
		courseIDStr = c.PostForm("course")
	}
	if courseIDStr != "" {
		if cID, err := strconv.ParseInt(courseIDStr, 10, 64); err == nil {
			courseID = &cID
		}
	}

	recordedClassURL := c.PostForm("recordedClassUrl")
	if recordedClassURL == "" {
		recordedClassURL = c.PostForm("recorded_class_url")
	}

	subject := c.PostForm("subject")
	topic := c.PostForm("topic")

	prerequisiteURL := c.PostForm("prerequisiteUrl")
	if prerequisiteURL == "" {
		prerequisiteURL = c.PostForm("prerequisite_url")
	}

	file, err := c.FormFile("file")
	if err != nil && recordedClassURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Either PDF note file or class recording link is required"})
		return
	}

	fileURL := ""
	var dest string
	if err == nil && file != nil {
		// 1. Create target directory
		notesDir := filepath.Join(h.mediaRoot, "notes")
		if err := os.MkdirAll(notesDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": fmt.Sprintf("failed to create notes directory: %v", err)})
			return
		}

		// 2. Save file
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
		dest = filepath.Join(notesDir, filename)
		if err := c.SaveUploadedFile(file, dest); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"detail": fmt.Sprintf("failed to save file: %v", err)})
			return
		}

		fileURL = fmt.Sprintf("%s/media/notes/%s", h.baseURL, filename)
	}

	note := &domain.Note{
		Title:            title,
		Description:      description,
		NoteType:         noteType,
		IsFree:           isFree,
		Price:            price,
		BatchID:          batchID,
		FileURL:          fileURL,
		CourseID:         courseID,
		HasSvgs:          hasSvgs,
		PageCount:        pageCount,
		Category:         category,
		Slug:             slug,
		RecordedClassURL: recordedClassURL,
		Subject:          subject,
		Topic:            topic,
		PrerequisiteURL:  prerequisiteURL,
	}

	if err := h.uc.CreateNote(c.Request.Context(), note); err != nil {
		// Clean up saved file if DB insert fails
		if dest != "" {
			_ = os.Remove(dest)
		}
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
	
	noteType := c.PostForm("noteType")
	if noteType == "" {
		noteType = c.PostForm("note_type")
	}
	if noteType != "" {
		roleVal, exists := c.Get("role")
		if exists {
			role := roleVal.(string)
			if role == "admin" && noteType == "class" {
				c.JSON(http.StatusForbidden, gin.H{"detail": "Admins are not allowed to set note type to class-related. Class notes can only be managed by teachers."})
				return
			}
		}
		updates["noteType"] = noteType
	}
	
	isFreeStr := c.PostForm("isFree")
	if isFreeStr == "" {
		isFreeStr = c.PostForm("is_free")
	}
	if isFreeStr != "" {
		isFree, err := strconv.ParseBool(isFreeStr)
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
	
	batchID := c.PostForm("batchId")
	if batchID == "" {
		batchID = c.PostForm("batch_id")
	}
	if batchID != "" {
		updates["batchId"] = batchID
	}
	
	if val := c.PostForm("category"); val != "" {
		updates["category"] = val
	}
	if val := c.PostForm("slug"); val != "" {
		updates["slug"] = val
	}
	
	recordedClassURL := c.PostForm("recordedClassUrl")
	if recordedClassURL == "" {
		recordedClassURL = c.PostForm("recorded_class_url")
	}
	if recordedClassURL != "" {
		updates["recordedClassUrl"] = recordedClassURL
	}

	if val := c.PostForm("subject"); val != "" {
		updates["subject"] = val
	}
	if val := c.PostForm("topic"); val != "" {
		updates["topic"] = val
	}
	if val := c.PostForm("prerequisiteUrl"); val != "" {
		updates["prerequisiteUrl"] = val
	} else if val := c.PostForm("prerequisite_url"); val != "" {
		updates["prerequisiteUrl"] = val
	}
	
	hasSvgsStr := c.PostForm("hasSvgs")
	if hasSvgsStr == "" {
		hasSvgsStr = c.PostForm("has_svgs")
	}
	if hasSvgsStr != "" {
		hasSvgs, err := strconv.ParseBool(hasSvgsStr)
		if err == nil {
			updates["hasSvgs"] = hasSvgs
		}
	}
	
	pageCountStr := c.PostForm("pageCount")
	if pageCountStr == "" {
		pageCountStr = c.PostForm("page_count")
	}
	if pageCountStr != "" {
		pageCount, err := strconv.Atoi(pageCountStr)
		if err == nil {
			updates["pageCount"] = float64(pageCount)
		}
	}
	
	courseIDStr := c.PostForm("courseId")
	if courseIDStr == "" {
		courseIDStr = c.PostForm("course")
	}
	if courseIDStr != "" {
		if courseIDStr == "null" || courseIDStr == "none" {
			updates["courseId"] = nil
		} else {
			courseID, err := strconv.ParseInt(courseIDStr, 10, 64)
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

func (h *httpHandler) GetPublicNotes(c *gin.Context) {
	filters := map[string]string{
		"noteType": "public",
		"category": c.Query("category"),
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

func (h *httpHandler) GetClassNotes(c *gin.Context) {
	filters := map[string]string{
		"category": c.Query("category"),
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

	notes, err := h.uc.GetClassNotes(c.Request.Context(), userID, role, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

func (h *httpHandler) GetAdminNotes(c *gin.Context) {
	filters := map[string]string{
		"category": c.Query("category"),
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
