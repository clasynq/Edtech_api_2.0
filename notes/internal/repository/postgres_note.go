package repository

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"clasynq/api/notes/internal/domain"

	"gorm.io/gorm"
)

type postgresNoteRepository struct {
	db *gorm.DB
}

func NewPostgresNoteRepository(db *gorm.DB) domain.NoteRepository {
	return &postgresNoteRepository{db: db}
}

func (r *postgresNoteRepository) GetNotes(ctx context.Context, filters map[string]string) ([]domain.Note, error) {
	var notes []domain.Note
	query := r.db.WithContext(ctx).Model(&domain.Note{}).
		Select("notes.*, courses.course_name AS course_name").
		Joins("LEFT JOIN courses ON notes.course_id = courses.id")

	if category, ok := filters["category"]; ok && category != "" {
		query = query.Where("notes.category = ?", category)
	}

	if courseIDStr, ok := filters["courseId"]; ok && courseIDStr != "" {
		if courseID, err := strconv.ParseInt(courseIDStr, 10, 64); err == nil {
			query = query.Where("notes.course_id = ?", courseID)
		}
	}

	if batchID, ok := filters["batchId"]; ok && batchID != "" {
		query = query.Where("notes.batch_id = ?", batchID)
	}

	if isFreeStr, ok := filters["isFree"]; ok && isFreeStr != "" {
		if isFree, err := strconv.ParseBool(isFreeStr); err == nil {
			query = query.Where("notes.is_free = ?", isFree)
		}
	}

	if noteType, ok := filters["noteType"]; ok && noteType != "" {
		query = query.Where("notes.note_type = ?", noteType)
	}

	if courseIDsStr, ok := filters["courseIds"]; ok && courseIDsStr != "" {
		parts := strings.Split(courseIDsStr, ",")
		var ids []int64
		for _, p := range parts {
			if id, err := strconv.ParseInt(p, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			query = query.Where("notes.course_id IN ?", ids)
		} else {
			query = query.Where("1 = 0")
		}
	}

	if search, ok := filters["search"]; ok && search != "" {
		searchParam := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(notes.title) LIKE ? OR LOWER(notes.description) LIKE ?", searchParam, searchParam)
	}

	err := query.Order("notes.created_at DESC").Find(&notes).Error
	if err != nil {
		return nil, err
	}

	// Union with class_schedules materials if noteType is 'class'
	if noteType, ok := filters["noteType"]; ok && noteType == "class" {
		var scheduleNotes []domain.Note
		scheduleQuery := r.db.WithContext(ctx).Table("class_schedules cs").
			Select("cs.id, cs.topic_name AS title, cs.description AS description, 'class' AS note_type, true AS is_free, 0.0 AS price, cs.batch_id, cs.class_notes_url AS file_url, cs.created_at, cs.course_id, cs.recorded_class_url, s.subject_name AS subject, cs.topic_name AS topic, '' AS prerequisite_url, c.course_name AS course_name").
			Joins("LEFT JOIN subjects s ON cs.subject_id = s.id").
			Joins("LEFT JOIN courses c ON cs.course_id = c.id")

		if courseIDsStr, ok := filters["courseIds"]; ok && courseIDsStr != "" {
			parts := strings.Split(courseIDsStr, ",")
			var ids []int64
			for _, p := range parts {
				if id, err := strconv.ParseInt(p, 10, 64); err == nil {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 {
				scheduleQuery = scheduleQuery.Where("cs.course_id IN ?", ids)
			} else {
				scheduleQuery = scheduleQuery.Where("1 = 0")
			}
		} else if courseIDStr, ok := filters["courseId"]; ok && courseIDStr != "" {
			if courseID, err := strconv.ParseInt(courseIDStr, 10, 64); err == nil {
				scheduleQuery = scheduleQuery.Where("cs.course_id = ?", courseID)
			}
		}

		if category, ok := filters["category"]; ok && category != "" {
			scheduleQuery = scheduleQuery.Where("c.category = ?", category)
		}

		if search, ok := filters["search"]; ok && search != "" {
			searchParam := "%" + strings.ToLower(search) + "%"
			scheduleQuery = scheduleQuery.Where("LOWER(cs.topic_name) LIKE ? OR LOWER(s.subject_name) LIKE ?", searchParam, searchParam)
		}

		if teacherIDStr, ok := filters["teacherId"]; ok && teacherIDStr != "" {
			if teacherID, err := strconv.ParseInt(teacherIDStr, 10, 64); err == nil {
				scheduleQuery = scheduleQuery.Where("cs.teacher_id = ?", teacherID)
			}
		}

		// Only fetch schedules with uploaded materials
		scheduleQuery = scheduleQuery.Where("cs.class_notes_url <> '' OR cs.recorded_class_url <> ''")

		if err := scheduleQuery.Find(&scheduleNotes).Error; err == nil {
			notes = append(notes, scheduleNotes...)
		}

		// Sort combined slice by created_at DESC
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].CreatedAt.After(notes[j].CreatedAt)
		})
	}

	return notes, nil
}

func (r *postgresNoteRepository) GetNoteByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Note, error) {
	var note domain.Note
	query := r.db.WithContext(ctx).Model(&domain.Note{}).
		Select("notes.*, courses.course_name AS course_name").
		Joins("LEFT JOIN courses ON notes.course_id = courses.id")

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("notes.id = ?", id)
	} else {
		query = query.Where("notes.slug = ?", idOrSlug)
	}

	if err := query.First(&note).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &note, nil
}

func (r *postgresNoteRepository) CreateNote(ctx context.Context, note *domain.Note) error {
	return r.db.WithContext(ctx).Create(note).Error
}

func (r *postgresNoteRepository) UpdateNote(ctx context.Context, note *domain.Note) error {
	return r.db.WithContext(ctx).Save(note).Error
}

func (r *postgresNoteRepository) DeleteNote(ctx context.Context, id int64) error {
	// Check if note exists in notes table
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Note{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// 1. Delete referencing note_accesses records
			if err := tx.Exec("DELETE FROM note_accesses WHERE note_id = ?", id).Error; err != nil {
				return err
			}
			// 2. Nullify referencing payment_orders records
			if err := tx.Exec("UPDATE payment_orders SET note_id = NULL WHERE note_id = ?", id).Error; err != nil {
				return err
			}
			// 3. Delete the note record
			if err := tx.Delete(&domain.Note{}, id).Error; err != nil {
				return err
			}
			return nil
		})
	}

	// Not found in notes table, check if it's a class schedule from class_schedules
	var csCount int64
	if err := r.db.WithContext(ctx).Table("class_schedules").Where("id = ?", id).Count(&csCount).Error; err != nil {
		return err
	}
	if csCount > 0 {
		// Clear/Nullify the notes and recording URLs on this schedule
		return r.db.WithContext(ctx).Table("class_schedules").Where("id = ?", id).Updates(map[string]interface{}{
			"class_notes_url":    "",
			"recorded_class_url": "",
		}).Error
	}

	return errors.New("note not found")
}

func (r *postgresNoteRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Check if the user exists in the users table first to prevent foreign key constraint violation
			var count int64
			if err := r.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
				return nil, err
			}
			if count == 0 {
				return nil, nil
			}

			// Create the student profile on the fly
			student = domain.Student{
				UserID: userID,
			}
			if err := r.db.WithContext(ctx).Create(&student).Error; err != nil {
				// Handle potential race conditions by trying to fetch one more time
				if err2 := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err2 == nil {
					return &student, nil
				}
				return nil, err
			}
			return &student, nil
		}
		return nil, err
	}
	return &student, nil
}

func (r *postgresNoteRepository) HasNoteAccess(ctx context.Context, studentID, noteID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("note_accesses").
		Where("student_id = ? AND note_id = ?", studentID, noteID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresNoteRepository) IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresNoteRepository) GetEnrolledCourseIDs(ctx context.Context, studentID int64) ([]int64, error) {
	var courseIDs []int64
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("student_id = ?", studentID).
		Pluck("course_id", &courseIDs).Error
	return courseIDs, err
}

func (r *postgresNoteRepository) GetNoteAccesses(ctx context.Context, studentID int64) ([]int64, error) {
	var noteIDs []int64
	err := r.db.WithContext(ctx).Table("note_accesses").
		Where("student_id = ?", studentID).
		Pluck("note_id", &noteIDs).Error
	return noteIDs, err
}
