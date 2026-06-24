package repository

import (
	"context"
	"errors"
	"strconv"

	"clasynq/api/cbt_exam/internal/domain"

	"gorm.io/gorm"
)

type postgresCbtExamRepository struct {
	db *gorm.DB
}

func NewPostgresCbtExamRepository(db *gorm.DB) domain.CbtExamRepository {
	return &postgresCbtExamRepository{db: db}
}

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

func (r *postgresCbtExamRepository) GetQuestionsByTestID(ctx context.Context, testID int64) ([]domain.Question, error) {
	var questions []domain.Question
	err := r.db.WithContext(ctx).Where("test_id = ?", testID).Preload("Options").Find(&questions).Error
	return questions, err
}

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

func (r *postgresCbtExamRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Preload("User").First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

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

func (r *postgresCbtExamRepository) CreateAttempt(ctx context.Context, attempt *domain.StudentTestAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *postgresCbtExamRepository) UpdateAttempt(ctx context.Context, attempt *domain.StudentTestAttempt) error {
	return r.db.WithContext(ctx).Save(attempt).Error
}

func (r *postgresCbtExamRepository) GetStudentAnswersByAttemptID(ctx context.Context, attemptID int64) ([]domain.StudentAnswer, error) {
	var answers []domain.StudentAnswer
	err := r.db.WithContext(ctx).Where("attempt_id = ?", attemptID).Find(&answers).Error
	return answers, err
}

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

func (r *postgresCbtExamRepository) SaveStudentAnswer(ctx context.Context, ans *domain.StudentAnswer) error {
	if ans.ID > 0 {
		return r.db.WithContext(ctx).Save(ans).Error
	}
	return r.db.WithContext(ctx).Create(ans).Error
}

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

func (r *postgresCbtExamRepository) CreateTestResult(ctx context.Context, res *domain.TestResult) error {
	return r.db.WithContext(ctx).Create(res).Error
}

func (r *postgresCbtExamRepository) UpdateTestResult(ctx context.Context, res *domain.TestResult) error {
	return r.db.WithContext(ctx).Save(res).Error
}

func (r *postgresCbtExamRepository) GetResultsForTest(ctx context.Context, testID int64) ([]domain.TestResult, error) {
	var results []domain.TestResult
	err := r.db.WithContext(ctx).Table("test_results").
		Joins("JOIN student_test_attempts ON student_test_attempts.id = test_results.attempt_id").
		Where("student_test_attempts.test_id = ? AND student_test_attempts.status = 'submitted'", testID).
		Order("test_results.score DESC, test_results.time_taken_seconds ASC").
		Find(&results).Error
	return results, err
}

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
