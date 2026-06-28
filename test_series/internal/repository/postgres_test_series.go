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

	err := query.Preload("Tests").Order("created_at DESC").Find(&list).Error
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
				if err2 := r.db.WithContext(ctx).Where("user_id = ?", userID).Preload("User").First(&student).Error; err2 == nil {
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
		// 1. Get all questions under this test
		var questionIDs []int64
		if err := tx.Table("questions").Where("test_id = ?", id).Pluck("id", &questionIDs).Error; err != nil {
			return err
		}

		// 2. Get all student attempts under this test
		var attemptIDs []int64
		if err := tx.Table("student_test_attempts").Where("test_id = ?", id).Pluck("id", &attemptIDs).Error; err != nil {
			return err
		}

		// 3. Delete student answers (referenced by attempts and questions)
		if len(attemptIDs) > 0 {
			if err := tx.Exec("DELETE FROM student_answers WHERE attempt_id IN ?", attemptIDs).Error; err != nil {
				return err
			}
			// 4. Delete test results (referenced by attempts)
			if err := tx.Exec("DELETE FROM test_results WHERE attempt_id IN ?", attemptIDs).Error; err != nil {
				return err
			}
			// 5. Delete student test attempts
			if err := tx.Exec("DELETE FROM student_test_attempts WHERE id IN ?", attemptIDs).Error; err != nil {
				return err
			}
		}

		// If there are questions, delete options, answers, and questions
		if len(questionIDs) > 0 {
			// Delete student answers for these questions just in case
			if err := tx.Exec("DELETE FROM student_answers WHERE question_id IN ?", questionIDs).Error; err != nil {
				return err
			}
			// 6. Delete question options
			if err := tx.Exec("DELETE FROM question_options WHERE question_id IN ?", questionIDs).Error; err != nil {
				return err
			}
			// 7. Delete questions
			if err := tx.Exec("DELETE FROM questions WHERE id IN ?", questionIDs).Error; err != nil {
				return err
			}
		}

		// 8. Finally, delete the test itself
		return tx.Delete(&domain.Test{}, id).Error
	})
}

func (r *postgresTestSeriesRepository) DeleteQuestion(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete student answers referencing this question
		if err := tx.Exec("DELETE FROM student_answers WHERE question_id = ?", id).Error; err != nil {
			return err
		}
		// Delete question options referencing this question
		if err := tx.Where("question_id = ?", id).Delete(&domain.QuestionOption{}).Error; err != nil {
			return err
		}
		// Delete the question
		return tx.Delete(&domain.Question{}, id).Error
	})
}

func (r *postgresTestSeriesRepository) UpdateTestSeries(ctx context.Context, id int64, ts *domain.TestSeries) error {
	ts.ID = id
	return r.db.WithContext(ctx).Omit("Tests").Save(ts).Error
}

func (r *postgresTestSeriesRepository) DeleteTestSeries(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Get all tests under this test series
		var testIDs []int64
		if err := tx.Table("tests").Where("test_series_id = ?", id).Pluck("id", &testIDs).Error; err != nil {
			return err
		}

		if len(testIDs) > 0 {
			// 2. Get all questions under these tests
			var questionIDs []int64
			if err := tx.Table("questions").Where("test_id IN ?", testIDs).Pluck("id", &questionIDs).Error; err != nil {
				return err
			}

			// 3. Get all student attempts under these tests
			var attemptIDs []int64
			if err := tx.Table("student_test_attempts").Where("test_id IN ?", testIDs).Pluck("id", &attemptIDs).Error; err != nil {
				return err
			}

			// 4. Delete student answers (referenced by attempts and questions)
			if len(attemptIDs) > 0 {
				if err := tx.Exec("DELETE FROM student_answers WHERE attempt_id IN ?", attemptIDs).Error; err != nil {
					return err
				}
				// 5. Delete test results (referenced by attempts)
				if err := tx.Exec("DELETE FROM test_results WHERE attempt_id IN ?", attemptIDs).Error; err != nil {
					return err
				}
				// 6. Delete student test attempts
				if err := tx.Exec("DELETE FROM student_test_attempts WHERE id IN ?", attemptIDs).Error; err != nil {
					return err
				}
			}

			// If there are questions, delete options, answers, and questions
			if len(questionIDs) > 0 {
				if err := tx.Exec("DELETE FROM student_answers WHERE question_id IN ?", questionIDs).Error; err != nil {
					return err
				}
				// 7. Delete question options
				if err := tx.Exec("DELETE FROM question_options WHERE question_id IN ?", questionIDs).Error; err != nil {
					return err
				}
				// 8. Delete questions
				if err := tx.Exec("DELETE FROM questions WHERE id IN ?", questionIDs).Error; err != nil {
					return err
				}
			}

			// 9. Delete tests
			if err := tx.Exec("DELETE FROM tests WHERE id IN ?", testIDs).Error; err != nil {
				return err
			}
		}

		// 10. Delete test series accesses
		if err := tx.Exec("DELETE FROM test_series_accesses WHERE test_series_id = ?", id).Error; err != nil {
			return err
		}

		// 11. Nullify test_series_id in payment_orders
		if err := tx.Exec("UPDATE payment_orders SET test_series_id = NULL WHERE test_series_id = ?", id).Error; err != nil {
			return err
		}

		// 12. Finally, delete the test series itself
		return tx.Delete(&domain.TestSeries{}, id).Error
	})
}

func (r *postgresTestSeriesRepository) GetQuestionsCountByTestID(ctx context.Context, testID int64) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Question{}).Where("test_id = ?", testID).Count(&count).Error
	return int(count), err
}

func (r *postgresTestSeriesRepository) GetStudentAttemptForTest(ctx context.Context, studentID, testID int64) (*domain.StudentTestAttempt, error) {
	var attempt domain.StudentTestAttempt
	// Check completed attempt first
	if err := r.db.WithContext(ctx).Where("student_id = ? AND test_id = ? AND status = 'completed'", studentID, testID).First(&attempt).Error; err == nil {
		return &attempt, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// Check started/ongoing attempt next
	if err := r.db.WithContext(ctx).Where("student_id = ? AND test_id = ? AND status = 'started'", studentID, testID).First(&attempt).Error; err == nil {
		return &attempt, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, nil
}

