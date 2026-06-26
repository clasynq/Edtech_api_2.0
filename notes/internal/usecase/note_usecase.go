package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"clasynq/api/notes/internal/domain"
)

type noteUsecase struct {
	repo    domain.NoteRepository
	baseURL string
}

func NewNoteUsecase(repo domain.NoteRepository, baseURL string) domain.NoteUsecase {
	return &noteUsecase{
		repo:    repo,
		baseURL: baseURL,
	}
}

func (u *noteUsecase) GetNotes(ctx context.Context, userID int64, role string, filters map[string]string) ([]domain.Note, error) {
	notes, err := u.repo.GetNotes(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Determine access for each note and mask URL if no access
	for i := range notes {
		hasAccess, err := u.checkAccess(ctx, userID, role, &notes[i])
		if err != nil || !hasAccess {
			notes[i].FileURL = "" // mask URL
			notes[i].IsUnlocked = false
		} else {
			notes[i].IsUnlocked = true
		}
		u.populateSVGPageURLs(&notes[i])
	}

	return notes, nil
}

func (u *noteUsecase) GetClassNotes(ctx context.Context, userID int64, role string, filters map[string]string) ([]domain.Note, error) {
	// If role is admin or teacher, they see all class notes
	if role == "admin" || role == "teacher" {
		filters["noteType"] = "class"
		return u.GetNotes(ctx, userID, role, filters)
	}

	// For student, they only see notes from courses they are enrolled in
	if userID <= 0 {
		return []domain.Note{}, nil
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return []domain.Note{}, nil
	}

	courseIDs, err := u.repo.GetEnrolledCourseIDs(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	if len(courseIDs) == 0 {
		return []domain.Note{}, nil
	}

	var idStrs []string
	for _, id := range courseIDs {
		idStrs = append(idStrs, strconv.FormatInt(id, 10))
	}
	filters["courseIds"] = strings.Join(idStrs, ",")
	filters["noteType"] = "class"

	return u.GetNotes(ctx, userID, role, filters)
}

func (u *noteUsecase) GetNoteByIDOrSlug(ctx context.Context, userID int64, role string, idOrSlug string) (*domain.Note, bool, error) {
	note, err := u.repo.GetNoteByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, false, err
	}
	if note == nil {
		return nil, false, nil
	}

	hasAccess, err := u.checkAccess(ctx, userID, role, note)
	if err != nil {
		return nil, false, err
	}

	note.IsUnlocked = hasAccess
	if !hasAccess {
		note.FileURL = "" // mask URL
	}
	u.populateSVGPageURLs(note)

	return note, hasAccess, nil
}

func (u *noteUsecase) CreateNote(ctx context.Context, note *domain.Note) error {
	if note.Title == "" {
		return errors.New("note title is required")
	}

	if note.Slug == "" {
		slug, err := u.generateUniqueSlug(ctx)
		if err != nil {
			return err
		}
		note.Slug = slug
	} else {
		// Verify custom slug uniqueness
		existing, err := u.repo.GetNoteByIDOrSlug(ctx, note.Slug)
		if err != nil {
			return err
		}
		if existing != nil {
			return errors.New("slug is already in use")
		}
	}

	note.CreatedAt = time.Now()
	return u.repo.CreateNote(ctx, note)
}

func (u *noteUsecase) UpdateNote(ctx context.Context, idOrSlug string, updates map[string]interface{}) (*domain.Note, error) {
	note, err := u.repo.GetNoteByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return nil, errors.New("note not found")
	}

	// Apply updates
	if val, ok := updates["title"]; ok {
		note.Title = val.(string)
	}
	if val, ok := updates["description"]; ok {
		note.Description = val.(string)
	}
	if val, ok := updates["noteType"]; ok {
		note.NoteType = val.(string)
	}
	if val, ok := updates["isFree"]; ok {
		note.IsFree = val.(bool)
	}
	if val, ok := updates["price"]; ok {
		note.Price = val.(float64)
	}
	if val, ok := updates["batchId"]; ok {
		note.BatchID = val.(string)
	}
	if val, ok := updates["fileUrl"]; ok {
		note.FileURL = val.(string)
	}
	if val, ok := updates["courseId"]; ok {
		if val == nil {
			note.CourseID = nil
		} else {
			cID := int64(val.(float64))
			note.CourseID = &cID
		}
	}
	if val, ok := updates["hasSvgs"]; ok {
		note.HasSvgs = val.(bool)
	}
	if val, ok := updates["pageCount"]; ok {
		note.PageCount = int(val.(float64))
	}
	if val, ok := updates["category"]; ok {
		note.Category = val.(string)
	}
	if val, ok := updates["slug"]; ok {
		newSlug := val.(string)
		if newSlug != note.Slug && newSlug != "" {
			existing, err := u.repo.GetNoteByIDOrSlug(ctx, newSlug)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				return nil, errors.New("slug is already in use")
			}
		}
	}

	if val, ok := updates["recordedClassUrl"]; ok {
		note.RecordedClassURL = val.(string)
	}
	if val, ok := updates["subject"]; ok {
		note.Subject = val.(string)
	}
	if val, ok := updates["topic"]; ok {
		note.Topic = val.(string)
	}
	if val, ok := updates["prerequisiteUrl"]; ok {
		note.PrerequisiteURL = val.(string)
	}

	if err := u.repo.UpdateNote(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (u *noteUsecase) DeleteNote(ctx context.Context, idOrSlug string) error {
	note, err := u.repo.GetNoteByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return err
	}
	if note == nil {
		return errors.New("note not found")
	}
	return u.repo.DeleteNote(ctx, note.ID)
}

func (u *noteUsecase) HasAccess(ctx context.Context, userID int64, role string, noteIDOrSlug string) (bool, error) {
	note, err := u.repo.GetNoteByIDOrSlug(ctx, noteIDOrSlug)
	if err != nil {
		return false, err
	}
	if note == nil {
		return false, errors.New("note not found")
	}
	return u.checkAccess(ctx, userID, role, note)
}

func (u *noteUsecase) checkAccess(ctx context.Context, userID int64, role string, note *domain.Note) (bool, error) {
	// 1. Admins and Teachers always have access
	if role == "admin" || role == "teacher" {
		return true, nil
	}

	// 2. Free notes are accessible by everyone
	if note.IsFree {
		return true, nil
	}

	// 3. User must be logged in for paid notes
	if userID <= 0 {
		return false, nil
	}

	// 4. Find student profile
	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	if student == nil {
		return false, nil
	}

	// 5. Check if they purchased note directly
	hasDirectAccess, err := u.repo.HasNoteAccess(ctx, student.ID, note.ID)
	if err != nil {
		return false, err
	}
	if hasDirectAccess {
		return true, nil
	}

	// 6. Check if note is attached to a course and they are enrolled in it
	if note.CourseID != nil {
		enrolled, err := u.repo.IsStudentEnrolledInCourse(ctx, student.ID, *note.CourseID)
		if err != nil {
			return false, err
		}
		if enrolled {
			return true, nil
		}
	}

	return false, nil
}

func (u *noteUsecase) generateUniqueSlug(ctx context.Context) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 10; i++ { // try up to 10 times to avoid infinite loop
		b := make([]byte, 22)
		for j := range b {
			num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			b[j] = charset[num.Int64()]
		}
		slug := string(b)
		existing, err := u.repo.GetNoteByIDOrSlug(ctx, slug)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return slug, nil
		}
	}
	return "", errors.New("failed to generate unique slug")
}

func (u *noteUsecase) populateSVGPageURLs(note *domain.Note) {
	if !note.IsUnlocked || !note.HasSvgs || note.PageCount <= 0 {
		note.SVGPageURLs = []string{}
		return
	}
	urls := make([]string, note.PageCount)
	for i := 1; i <= note.PageCount; i++ {
		urls[i-1] = fmt.Sprintf("%s/media/notes/note_%d_pages/page_%d.svg", u.baseURL, note.ID, i)
	}
	note.SVGPageURLs = urls
}
