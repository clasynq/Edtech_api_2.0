package usecase

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"clasynq/api/notes/internal/domain"
	"github.com/redis/go-redis/v9"
)

type noteUsecase struct {
	repo    domain.NoteRepository
	baseURL string
	rdb     *redis.Client
}

func NewNoteUsecase(repo domain.NoteRepository, baseURL string, rdb *redis.Client) domain.NoteUsecase {
	return &noteUsecase{
		repo:    repo,
		baseURL: baseURL,
		rdb:     rdb,
	}
}

func serializeFilters(filters map[string]string) string {
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, filters[k]))
	}
	return strings.Join(parts, "&")
}

func (u *noteUsecase) GetNotes(ctx context.Context, userID int64, role string, filters map[string]string) ([]domain.Note, error) {
	serialized := serializeFilters(filters)
	cacheKey := fmt.Sprintf("notes_list:filters:%s", serialized)

	var notes []domain.Note
	cacheHit := false

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			if err := json.Unmarshal([]byte(val), &notes); err == nil {
				cacheHit = true
			}
		}
	}

	if !cacheHit {
		var err error
		notes, err = u.repo.GetNotes(ctx, filters)
		if err != nil {
			return nil, err
		}

		if u.rdb != nil {
			if raw, err := json.Marshal(notes); err == nil {
				_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
			}
		}
	}

	// Determine access for each note and mask URL if no access
	var student *domain.Student
	enrolledCourses := make(map[int64]bool)
	directAccesses := make(map[int64]bool)
	hasAccessListsLoaded := false

	for i := range notes {
		var hasAccess bool
		if role == "admin" || role == "teacher" {
			hasAccess = true
		} else if notes[i].IsFree {
			hasAccess = true
		} else if userID <= 0 {
			hasAccess = false
		} else {
			// Lazy load access lists once if userID > 0 and not admin/teacher
			if !hasAccessListsLoaded {
				if s, err := u.repo.GetStudentByUserID(ctx, userID); err == nil && s != nil {
					student = s
					if courseIDs, err := u.repo.GetEnrolledCourseIDs(ctx, s.ID); err == nil {
						for _, cid := range courseIDs {
							enrolledCourses[cid] = true
						}
					}
					if noteIDs, err := u.repo.GetNoteAccesses(ctx, s.ID); err == nil {
						for _, nid := range noteIDs {
							directAccesses[nid] = true
						}
					}
				}
				hasAccessListsLoaded = true
			}

			if student != nil {
				if directAccesses[notes[i].ID] {
					hasAccess = true
				} else if notes[i].CourseID != nil && enrolledCourses[*notes[i].CourseID] {
					hasAccess = true
				}
			}
		}

		if !hasAccess {
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
	var dbNotes []domain.Note
	var err error

	category := filters["category"]

	if role == "admin" || role == "teacher" {
		filters["noteType"] = "class"
		if role == "teacher" {
			filters["teacherId"] = strconv.FormatInt(userID, 10)
		}
		dbNotes, err = u.GetNotes(ctx, userID, role, filters)
		if err != nil {
			return nil, err
		}

		var schedNotes []domain.Note
		if role == "admin" {
			schedNotes, err = u.repo.GetClassScheduleNotes(ctx, nil, category, 0, true)
		} else {
			schedNotes, err = u.repo.GetClassScheduleNotes(ctx, nil, category, userID, false)
		}
		if err == nil {
			dbNotes = append(dbNotes, schedNotes...)
		}
	} else {
		if userID > 0 {
			student, err := u.repo.GetStudentByUserID(ctx, userID)
			if err == nil && student != nil {
				courseIDs, err := u.repo.GetEnrolledCourseIDs(ctx, student.ID)
				if err == nil && len(courseIDs) > 0 {
					var idStrs []string
					for _, id := range courseIDs {
						idStrs = append(idStrs, strconv.FormatInt(id, 10))
					}
					filters["courseIds"] = strings.Join(idStrs, ",")
					filters["noteType"] = "class"

					dbNotes, err = u.GetNotes(ctx, userID, role, filters)
					if err == nil {
						schedNotes, err := u.repo.GetClassScheduleNotes(ctx, courseIDs, category, 0, false)
						if err == nil {
							dbNotes = append(dbNotes, schedNotes...)
						}
					}
				}
			}
		}
	}

	sort.Slice(dbNotes, func(i, j int) bool {
		return dbNotes[i].CreatedAt.After(dbNotes[j].CreatedAt)
	})

	seenUrls := make(map[string]bool)
	var finalNotes []domain.Note
	for _, n := range dbNotes {
		var cid int64
		if n.CourseID != nil {
			cid = *n.CourseID
		}

		fileURL := strings.TrimSpace(strings.ToLower(n.FileURL))
		if idx := strings.Index(fileURL, "/media/"); idx != -1 {
			fileURL = fileURL[idx:]
		}
		recordedURL := strings.TrimSpace(strings.ToLower(n.RecordedClassURL))
		if idx := strings.Index(recordedURL, "/media/"); idx != -1 {
			recordedURL = recordedURL[idx:]
		}

		if fileURL != "" {
			key := fmt.Sprintf("%d|%s", cid, fileURL)
			if seenUrls[key] {
				continue
			}
			seenUrls[key] = true
		}

		if recordedURL != "" {
			key := fmt.Sprintf("%d|%s", cid, recordedURL)
			if seenUrls[key] {
				continue
			}
			seenUrls[key] = true
		}

		finalNotes = append(finalNotes, n)
	}

	return finalNotes, nil
}

func (u *noteUsecase) GetNoteByIDOrSlug(ctx context.Context, userID int64, role string, idOrSlug string) (*domain.Note, bool, error) {
	cacheKey := fmt.Sprintf("note_detail:%s", idOrSlug)
	var note *domain.Note
	cacheHit := false

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cachedNote domain.Note
			if err := json.Unmarshal([]byte(val), &cachedNote); err == nil {
				note = &cachedNote
				cacheHit = true
			}
		}
	}

	if !cacheHit {
		var err error
		note, err = u.repo.GetNoteByIDOrSlug(ctx, idOrSlug)
		if err != nil {
			return nil, false, err
		}
		if note == nil {
			return nil, false, nil
		}

		if u.rdb != nil {
			if raw, err := json.Marshal(note); err == nil {
				_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
			}
		}
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
	if err := u.repo.CreateNote(ctx, note); err != nil {
		return err
	}
	u.invalidateNotesCache(ctx)
	return nil
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
	u.invalidateNotesCache(ctx)
	return note, nil
}
func (u *noteUsecase) DeleteNote(ctx context.Context, idOrSlug string) error {
	note, err := u.repo.GetNoteByIDOrSlug(ctx, idOrSlug)
	if err == nil && note != nil {
		if err := u.repo.DeleteNote(ctx, note.ID); err != nil {
			return err
		}
		u.invalidateNotesCache(ctx)
		return nil
	}

	// Fallback check for numeric ID (for class_schedules)
	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		if err := u.repo.DeleteNote(ctx, id); err != nil {
			return err
		}
		u.invalidateNotesCache(ctx)
		return nil
	}

	return errors.New("note not found")
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

func (u *noteUsecase) invalidateCache(ctx context.Context, patterns ...string) {
	if u.rdb == nil {
		return
	}
	for _, pattern := range patterns {
		iter := u.rdb.Scan(ctx, 0, pattern, 0).Iterator()
		for iter.Next(ctx) {
			u.rdb.Del(ctx, iter.Val())
		}
	}
}

func (u *noteUsecase) invalidateNotesCache(ctx context.Context) {
	u.invalidateCache(ctx, "notes_list*", "class_notes_list*", "note_detail*")
}

func (u *noteUsecase) CreateImportantNote(ctx context.Context, note *domain.ImportantNote) error {
	if note.Title == "" || note.BatchID == "" || note.FileURL == "" {
		return errors.New("title, batchId, and fileUrl are required")
	}
	return u.repo.CreateImportantNote(ctx, note)
}

func (u *noteUsecase) DeleteImportantNote(ctx context.Context, id int64) error {
	return u.repo.DeleteImportantNote(ctx, id)
}

func (u *noteUsecase) GetImportantNotes(ctx context.Context, userID int64, role string, batchID string) ([]domain.ImportantNote, error) {
	if role == "student" {
		student, err := u.repo.GetStudentByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if student == nil {
			return nil, errors.New("student profile not found")
		}
		// Resolve enrolled batches for the student
		batches, err := u.repo.GetBatchesByStudentID(ctx, student.ID)
		if err != nil {
			return nil, err
		}
		if len(batches) == 0 {
			return []domain.ImportantNote{}, nil
		}
		return u.repo.GetImportantNotes(ctx, batches)
	}

	// For admin/teacher
	return u.repo.GetImportantNotesAdmin(ctx, batchID)
}

func (u *noteUsecase) GetImportantNoteByID(ctx context.Context, userID int64, role string, id int64) (*domain.ImportantNote, error) {
	note, err := u.repo.GetImportantNoteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return nil, errors.New("note not found")
	}

	// For students, check batch authorization
	if role == "student" {
		student, err := u.repo.GetStudentByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if student == nil {
			return nil, errors.New("student profile not found")
		}
		batches, err := u.repo.GetBatchesByStudentID(ctx, student.ID)
		if err != nil {
			return nil, err
		}
		hasAccess := false
		for _, b := range batches {
			if b == note.BatchID {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			return nil, errors.New("unauthorized batch access to this note")
		}
	}

	return note, nil
}
