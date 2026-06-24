package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"clasynq/api/test_series/internal/domain"

	"gorm.io/gorm"
)

type postgresTestSeriesRepository struct {
	db *gorm.DB
}

func NewPostgresTestSeriesRepository(db *gorm.DB) domain.TestSeriesRepository {
	return &postgresTestSeriesRepository{db: db}
}

func (r *postgresTestSeriesRepository) GetTestSeries(ctx context.Context, filters map[string]string) ([]domain.TestSeries, error) {
	var list []domain.TestSeries
	query := r.db.WithContext(ctx).Model(&domain.TestSeries{})

	if category, ok := filters["category"]; ok && category != "" {
		query = query.Where("category = ?", category)
	}

	if courseIDStr, ok := filters["courseId"]; ok && courseIDStr != "" {
		if courseID, err := strconv.ParseInt(courseIDStr, 10, 64); err == nil {
			query = query.Where("course_id = ?", courseID)
		}
	}

	if isPublishedStr, ok := filters["isPublished"]; ok && isPublishedStr != "" {
		if isPublished, err := strconv.ParseBool(isPublishedStr); err == nil {
			query = query.Where("is_published = ?", isPublished)
		}
	}

	if search, ok := filters["search"]; ok && search != "" {
		searchParam := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", searchParam, searchParam)
	}

	err := query.Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *postgresTestSeriesRepository) GetTestSeriesByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.TestSeries, error) {
	var ts domain.TestSeries
	query := r.db.WithContext(ctx).Model(&domain.TestSeries{})

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("slug = ?", idOrSlug)
	}

	if err := query.Preload("Tests").First(&ts).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ts, nil
}

func (r *postgresTestSeriesRepository) CreateTestSeries(ctx context.Context, ts *domain.TestSeries) error {
	return r.db.WithContext(ctx).Create(ts).Error
}

func (r *postgresTestSeriesRepository) GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Test, error) {
	var test domain.Test
	query := r.db.WithContext(ctx).Model(&domain.Test{})

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("slug = ?", idOrSlug)
	}

	if err := query.First(&test).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &test, nil
}

func (r *postgresTestSeriesRepository) CreateTest(ctx context.Context, test *domain.Test) error {
	return r.db.WithContext(ctx).Create(test).Error
}

func (r *postgresTestSeriesRepository) GetTestsBySeriesID(ctx context.Context, seriesID int64) ([]domain.Test, error) {
	var tests []domain.Test
	err := r.db.WithContext(ctx).Where("test_series_id = ?", seriesID).Find(&tests).Error
	return tests, err
}

func (r *postgresTestSeriesRepository) GetQuestionsByTestID(ctx context.Context, testID int64) ([]domain.Question, error) {
	var questions []domain.Question
	err := r.db.WithContext(ctx).Where("test_id = ?", testID).Preload("Options").Find(&questions).Error
	return questions, err
}

func (r *postgresTestSeriesRepository) GetQuestionByID(ctx context.Context, id int64) (*domain.Question, error) {
	var q domain.Question
	if err := r.db.WithContext(ctx).Preload("Options").First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *postgresTestSeriesRepository) CreateQuestion(ctx context.Context, question *domain.Question) error {
	return r.db.WithContext(ctx).Create(question).Error
}

func (r *postgresTestSeriesRepository) CreateQuestionOption(ctx context.Context, option *domain.QuestionOption) error {
	return r.db.WithContext(ctx).Create(option).Error
}

func (r *postgresTestSeriesRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Preload("User").First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

func (r *postgresTestSeriesRepository) HasTestSeriesAccess(ctx context.Context, studentID, seriesID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("test_series_accesses").
		Where("student_id = ? AND test_series_id = ?", studentID, seriesID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresTestSeriesRepository) IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresTestSeriesRepository) UpdateTest(ctx context.Context, test *domain.Test) error {
	return r.db.WithContext(ctx).Save(test).Error
}

func (r *postgresTestSeriesRepository) DeleteTest(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var questions []domain.Question
		if err := tx.Where("test_id = ?", id).Find(&questions).Error; err != nil {
			return err
		}
		for _, q := range questions {
			if err := tx.Where("question_id = ?", q.ID).Delete(&domain.QuestionOption{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("test_id = ?", id).Delete(&domain.Question{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Test{}, id).Error
	})
}

func (r *postgresTestSeriesRepository) DeleteQuestion(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("question_id = ?", id).Delete(&domain.QuestionOption{}).Error; err != nil {
			return err
		}
		return tx.Delete(&domain.Question{}, id).Error
	})
}
