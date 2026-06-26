package repository

import (
	"context"
	"errors"
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
	return notes, err
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

func (r *postgresNoteRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
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
