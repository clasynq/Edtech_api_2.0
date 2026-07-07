package repository

import (
	"context"
	"errors"
	"strconv"

	"clasynq/api/cbt_exam/internal/domain"

	"gorm.io/gorm"
)

// postgresCbtExamRepository implements domain.CbtExamRepository interface.
type postgresCbtExamRepository struct {
	db *gorm.DB
}

// NewPostgresCbtExamRepository initializes postgresCbtExamRepository.
func NewPostgresCbtExamRepository(db *gorm.DB) domain.CbtExamRepository {
	return &postgresCbtExamRepository{db: db}
}

// GetTestByIDOrSlug retrieves a Test by its primary ID or alphanumeric slug.
func (r *postgresCbtExamRepository) GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Test, error) {
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

// GetTestSeriesByIDOrSlug retrieves a TestSeries by its primary ID or unique slug.
func (r *postgresCbtExamRepository) GetTestSeriesByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.TestSeries, error) {
	var ts domain.TestSeries
	query := r.db.WithContext(ctx).Model(&domain.TestSeries{})

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("slug = ?", idOrSlug)
	}

	if err := query.First(&ts).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ts, nil
}

// GetQuestionsByTestID pulls all questions associated with a test, preloading available choices.
func (r *postgresCbtExamRepository) GetQuestionsByTestID(ctx context.Context, testID int64) ([]domain.Question, error) {
	var questions []domain.Question
	err := r.db.WithContext(ctx).Where("test_id = ?", testID).Preload("Options").Find(&questions).Error
	return questions, err
}

// GetQuestionByID retrieves a single question preloaded with choices.
func (r *postgresCbtExamRepository) GetQuestionByID(ctx context.Context, id int64) (*domain.Question, error) {
	var q domain.Question
	if err := r.db.WithContext(ctx).Preload("Options").First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

// GetStudentByUserID resolves a student record, provisioning a profile automatically if missing.
func (r *postgresCbtExamRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
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

// HasTestSeriesAccess verifies if test_series_accesses has an entry for the student and series.
func (r *postgresCbtExamRepository) HasTestSeriesAccess(ctx context.Context, studentID, seriesID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("test_series_accesses").
		Where("student_id = ? AND test_series_id = ?", studentID, seriesID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsStudentEnrolledInCourse checks if the student is registered in enrollments for the course.
func (r *postgresCbtExamRepository) IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("enrollments").
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetOngoingAttempt pulls any unfinished attempt (status = 'ongoing') for a student and test.
func (r *postgresCbtExamRepository) GetOngoingAttempt(ctx context.Context, studentID, testID int64) (*domain.StudentTestAttempt, error) {
	var attempt domain.StudentTestAttempt
	err := r.db.WithContext(ctx).
		Where("student_id = ? AND test_id = ? AND status = 'ongoing'", studentID, testID).
		First(&attempt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &attempt, nil
}

// GetAttemptByIDOrSlug resolves a test attempt by ID or slug.
func (r *postgresCbtExamRepository) GetAttemptByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.StudentTestAttempt, error) {
	var attempt domain.StudentTestAttempt
	query := r.db.WithContext(ctx).Model(&domain.StudentTestAttempt{})

	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("slug = ?", idOrSlug)
	}

	if err := query.First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &attempt, nil
}

// GetLastAttemptForStudentAndTest fetches the most recent attempt.
func (r *postgresCbtExamRepository) GetLastAttemptForStudentAndTest(ctx context.Context, studentID, testID int64) (*domain.StudentTestAttempt, error) {
	var attempt domain.StudentTestAttempt
	err := r.db.WithContext(ctx).
		Where("student_id = ? AND test_id = ?", studentID, testID).
		Order("started_at DESC").
		First(&attempt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &attempt, nil
}

// CreateAttempt stores a new attempt record.
func (r *postgresCbtExamRepository) CreateAttempt(ctx context.Context, attempt *domain.StudentTestAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

// UpdateAttempt updates attempt attributes.
func (r *postgresCbtExamRepository) UpdateAttempt(ctx context.Context, attempt *domain.StudentTestAttempt) error {
	return r.db.WithContext(ctx).Save(attempt).Error
}

// GetStudentAnswersByAttemptID retrieves all answers recorded for an attempt.
func (r *postgresCbtExamRepository) GetStudentAnswersByAttemptID(ctx context.Context, attemptID int64) ([]domain.StudentAnswer, error) {
	var answers []domain.StudentAnswer
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Find(&answers).Error
	return answers, err
}

// GetStudentAnswerForQuestion checks if an answer was already submitted for a specific question.
func (r *postgresCbtExamRepository) GetStudentAnswerForQuestion(ctx context.Context, attemptID, questionID int64) (*domain.StudentAnswer, error) {
	var ans domain.StudentAnswer
	if err := r.db.WithContext(ctx).Where("attempt_id = ? AND question_id = ?", attemptID, questionID).First(&ans).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ans, nil
}

// SaveStudentAnswer inserts or updates a student answer record.
func (r *postgresCbtExamRepository) SaveStudentAnswer(ctx context.Context, ans *domain.StudentAnswer) error {
	if ans.ID > 0 {
		return r.db.WithContext(ctx).Save(ans).Error
	}
	return r.db.WithContext(ctx).Create(ans).Error
}

// GetTestResultByAttemptID retrieves calculated metrics for a completed attempt.
func (r *postgresCbtExamRepository) GetTestResultByAttemptID(ctx context.Context, attemptID int64) (*domain.TestResult, error) {
	var res domain.TestResult
	if err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).First(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &res, nil
}

// CreateTestResult inserts score details.
func (r *postgresCbtExamRepository) CreateTestResult(ctx context.Context, res *domain.TestResult) error {
	return r.db.WithContext(ctx).Create(res).Error
}

// UpdateTestResult modifies score metrics.
func (r *postgresCbtExamRepository) UpdateTestResult(ctx context.Context, res *domain.TestResult) error {
	return r.db.WithContext(ctx).Save(res).Error
}

// GetResultsForTest retrieves all submitted attempts results sorted by score and time.
func (r *postgresCbtExamRepository) GetResultsForTest(ctx context.Context, testID int64) ([]domain.TestResult, error) {
	var results []domain.TestResult
	err := r.db.WithContext(ctx).Table("test_results").
		Joins("JOIN student_test_attempts ON student_test_attempts.id = test_results.attempt_id").
		Where("student_test_attempts.test_id = ? AND student_test_attempts.status = 'submitted'", testID).
		Order("test_results.score DESC, test_results.time_taken_seconds ASC").
		Find(&results).Error
	return results, err
}

// GetLeaderboard returns the top 50 student results ranked for a test.
func (r *postgresCbtExamRepository) GetLeaderboard(ctx context.Context, testID int64) ([]map[string]interface{}, error) {
	rows, err := r.db.WithContext(ctx).Table("test_results").
		Select("users.full_name as student_name, test_results.score, test_results.rank, test_results.time_taken_seconds").
		Joins("JOIN student_test_attempts ON student_test_attempts.id = test_results.attempt_id").
		Joins("JOIN students ON students.id = student_test_attempts.student_id").
		Joins("JOIN users ON users.id = students.user_id").
		Where("student_test_attempts.test_id = ? AND student_test_attempts.status = 'submitted'", testID).
		Order("test_results.rank ASC").
		Limit(50).
		Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaderboard []map[string]interface{}
	for rows.Next() {
		var name string
		var score float64
		var rank int
		var duration int
		if err := rows.Scan(&name, &score, &rank, &duration); err == nil {
			leaderboard = append(leaderboard, map[string]interface{}{
				"studentName":      name,
				"score":            score,
				"rank":             rank,
				"timeTakenSeconds": duration,
			})
		}
	}
	return leaderboard, nil
}

// GetAttemptsMonitoring fetches all student attempts (both ongoing and completed) for real-time overview dashboards.
func (r *postgresCbtExamRepository) GetAttemptsMonitoring(ctx context.Context, testID int64) ([]domain.AttemptMonitorData, error) {
	rows, err := r.db.WithContext(ctx).Table("student_test_attempts").
		Select("student_test_attempts.id, student_test_attempts.student_id, users.full_name as student_name, users.email as student_email, student_test_attempts.started_at, student_test_attempts.submitted_at, student_test_attempts.score, student_test_attempts.status").
		Joins("JOIN students ON students.id = student_test_attempts.student_id").
		Joins("JOIN users ON users.id = students.user_id").
		Where("student_test_attempts.test_id = ?", testID).
		Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.AttemptMonitorData
	for rows.Next() {
		var item domain.AttemptMonitorData
		err := rows.Scan(
			&item.ID,
			&item.StudentID,
			&item.StudentName,
			&item.StudentEmail,
			&item.StartedAt,
			&item.SubmittedAt,
			&item.Score,
			&item.Status,
		)
		if err == nil {
			var res domain.TestResult
			if errRes := r.db.WithContext(ctx).Where("attempt_id = ?", item.ID).First(&res).Error; errRes == nil {
				item.Result = &res
			}
			list = append(list, item)
		}
	}
	return list, nil
}

