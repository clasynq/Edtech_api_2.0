package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"clasynq/api/test_series/internal/domain"
)

type testSeriesUsecase struct {
	repo domain.TestSeriesRepository
}

func NewTestSeriesUsecase(repo domain.TestSeriesRepository) domain.TestSeriesUsecase {
	return &testSeriesUsecase{repo: repo}
}

func (u *testSeriesUsecase) GetTestSeries(ctx context.Context, userID int64, role string, filters map[string]string) ([]domain.TestSeries, error) {
	return u.repo.GetTestSeries(ctx, filters)
}

func (u *testSeriesUsecase) GetTestSeriesByIDOrSlug(ctx context.Context, userID int64, role string, idOrSlug string) (*domain.TestSeries, bool, error) {
	ts, err := u.repo.GetTestSeriesByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, false, err
	}
	if ts == nil {
		return nil, false, nil
	}

	hasAccess, err := u.HasAccess(ctx, userID, role, ts.ID)
	return ts, hasAccess, err
}

func (u *testSeriesUsecase) CreateTestSeries(ctx context.Context, ts *domain.TestSeries) error {
	if ts.Title == "" {
		return errors.New("title is required")
	}

	if ts.Slug == "" {
		slug, err := u.generateUniqueSlug(ctx, "series")
		if err != nil {
			return err
		}
		ts.Slug = slug
	} else {
		existing, _ := u.repo.GetTestSeriesByIDOrSlug(ctx, ts.Slug)
		if existing != nil {
			return errors.New("slug is already in use")
		}
	}

	ts.CreatedAt = time.Now()
	return u.repo.CreateTestSeries(ctx, ts)
}

func (u *testSeriesUsecase) GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Test, error) {
	return u.repo.GetTestByIDOrSlug(ctx, idOrSlug)
}

func (u *testSeriesUsecase) CreateTest(ctx context.Context, test *domain.Test) error {
	if test.Title == "" {
		return errors.New("test title is required")
	}

	if test.Slug == "" {
		slug, err := u.generateUniqueSlug(ctx, "test")
		if err != nil {
			return err
		}
		test.Slug = slug
	} else {
		existing, _ := u.repo.GetTestByIDOrSlug(ctx, test.Slug)
		if existing != nil {
			return errors.New("slug is already in use")
		}
	}

	test.CreatedAt = time.Now()
	return u.repo.CreateTest(ctx, test)
}

func (u *testSeriesUsecase) AddQuestion(ctx context.Context, q *domain.Question, options []domain.QuestionOption) error {
	if q.QuestionText == nil && q.QuestionImageURL == nil {
		return errors.New("question text or image is required")
	}

	q.CreatedAt = time.Now()
	if err := u.repo.CreateQuestion(ctx, q); err != nil {
		return err
	}

	for i := range options {
		options[i].QuestionID = q.ID
		if err := u.repo.CreateQuestionOption(ctx, &options[i]); err != nil {
			return err
		}
	}

	return nil
}

func (u *testSeriesUsecase) HasAccess(ctx context.Context, userID int64, role string, seriesID int64) (bool, error) {
	if role == "admin" || role == "teacher" {
		return true, nil
	}

	ts, err := u.repo.GetTestSeriesByIDOrSlug(ctx, strconv.FormatInt(seriesID, 10))
	if err != nil {
		return false, err
	}
	if ts == nil {
		return false, errors.New("test series not found")
	}

	if ts.IsFree {
		return true, nil
	}

	if userID <= 0 {
		return false, nil
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	if student == nil {
		return false, nil
	}

	hasAccess, err := u.repo.HasTestSeriesAccess(ctx, student.ID, seriesID)
	if err != nil {
		return false, err
	}
	if hasAccess {
		return true, nil
	}

	if ts.CourseID != nil {
		enrolled, err := u.repo.IsStudentEnrolledInCourse(ctx, student.ID, *ts.CourseID)
		if err != nil {
			return false, err
		}
		if enrolled {
			return true, nil
		}
	}

	return false, nil
}

func (u *testSeriesUsecase) generateUniqueSlug(ctx context.Context, kind string) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 10; i++ {
		b := make([]byte, 22)
		for j := range b {
			num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			b[j] = charset[num.Int64()]
		}
		slug := string(b)
		if kind == "series" {
			existing, _ := u.repo.GetTestSeriesByIDOrSlug(ctx, slug)
			if existing == nil {
				return slug, nil
			}
		} else if kind == "test" {
			existing, _ := u.repo.GetTestByIDOrSlug(ctx, slug)
			if existing == nil {
				return slug, nil
			}
		}
	}
	return "", errors.New("failed to generate unique slug")
}

func (u *testSeriesUsecase) UpdateTest(ctx context.Context, id int64, test *domain.Test) error {
	existing, err := u.repo.GetTestByIDOrSlug(ctx, strconv.FormatInt(id, 10))
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("test not found")
	}

	existing.Title = test.Title
	existing.Description = test.Description
	existing.DurationMinutes = test.DurationMinutes
	existing.TotalMarks = test.TotalMarks
	existing.NegativeMarking = test.NegativeMarking
	existing.Instructions = test.Instructions
	existing.IsPublished = test.IsPublished

	return u.repo.UpdateTest(ctx, existing)
}

func (u *testSeriesUsecase) DeleteTest(ctx context.Context, id int64) error {
	return u.repo.DeleteTest(ctx, id)
}

func (u *testSeriesUsecase) CreateQuestion(ctx context.Context, q *domain.Question, options []domain.QuestionOption) error {
	return u.AddQuestion(ctx, q, options)
}

func (u *testSeriesUsecase) DeleteQuestion(ctx context.Context, id int64) error {
	return u.repo.DeleteQuestion(ctx, id)
}

func (u *testSeriesUsecase) UploadQuestions(ctx context.Context, testID int64, data []map[string]interface{}) (int, error) {
	createdCount := 0

	for _, row := range data {
		normRow := make(map[string]interface{})
		for k, v := range row {
			normRow[strings.ToLower(strings.TrimSpace(k))] = v
		}

		questionType := "MCQ"
		if qtVal, ok := normRow["question_type"]; ok && qtVal != nil {
			questionType = strings.ToUpper(strings.TrimSpace(qtVal.(string)))
		}

		var questionText *string
		if qtTextVal, ok := normRow["question_text"]; ok && qtTextVal != nil {
			strVal := qtTextVal.(string)
			questionText = &strVal
		}

		if questionText == nil || *questionText == "" {
			return createdCount, errors.New("question_text is required")
		}

		var correctAnswer *string
		if caVal, ok := normRow["correct_answer"]; ok && caVal != nil {
			strVal := strings.TrimSpace(caVal.(string))
			correctAnswer = &strVal
		}

		marks := 2
		if mVal, ok := normRow["marks"]; ok && mVal != nil {
			switch mv := mVal.(type) {
			case float64:
				marks = int(mv)
			case string:
				if parsed, err := strconv.Atoi(mv); err == nil {
					marks = parsed
				}
			}
		}

		negMarks := 0.0
		if nmVal, ok := normRow["negative_marks"]; ok && nmVal != nil {
			switch nmv := nmVal.(type) {
			case float64:
				negMarks = nmv
			case string:
				if parsed, err := strconv.ParseFloat(nmv, 64); err == nil {
					negMarks = parsed
				}
			}
		}

		var qTimer *int
		if qtVal, ok := normRow["question_timer"]; ok && qtVal != nil {
			switch qtv := qtVal.(type) {
			case float64:
				val := int(qtv)
				qTimer = &val
			case string:
				if parsed, err := strconv.Atoi(qtv); err == nil {
					qTimer = &parsed
				}
			}
		}

		var expText *string
		if etVal, ok := normRow["explanation_text"]; ok && etVal != nil {
			strVal := etVal.(string)
			expText = &strVal
		}

		q := &domain.Question{
			QuestionType:    questionType,
			QuestionText:    questionText,
			CorrectAnswer:   correctAnswer,
			Marks:           marks,
			NegativeMarks:   negMarks,
			QuestionTimer:   qTimer,
			ExplanationText: expText,
			TestID:          testID,
			CreatedAt:       time.Now(),
		}

		if err := u.repo.CreateQuestion(ctx, q); err != nil {
			return createdCount, err
		}

		if questionType != "NAT" {
			if optsVal, ok := normRow["options"]; ok && optsVal != nil {
				if optsList, ok := optsVal.([]interface{}); ok {
					for _, optRaw := range optsList {
						if optMap, ok := optRaw.(map[string]interface{}); ok {
							optTextVal, _ := optMap["option_text"].(string)
							isCorrVal, _ := optMap["is_correct"].(bool)
							if optTextVal != "" {
								opt := &domain.QuestionOption{
									OptionText: &optTextVal,
									IsCorrect:  isCorrVal,
									QuestionID: q.ID,
								}
								_ = u.repo.CreateQuestionOption(ctx, opt)
							}
						}
					}
				}
			} else {
				optionKeys := []string{"option_a", "option_b", "option_c", "option_d"}
				correctList := []string{}
				if correctAnswer != nil {
					for _, item := range strings.Split(*correctAnswer, ",") {
						correctList = append(correctList, strings.TrimSpace(strings.ToUpper(item)))
					}
				}

				for _, key := range optionKeys {
					letter := strings.ToUpper(strings.Split(key, "_")[1])
					optTextVal := ""
					if val, ok := normRow[key]; ok && val != nil {
						optTextVal = strings.TrimSpace(val.(string))
					}
					if optTextVal != "" {
						isCorrect := false
						for _, item := range correctList {
							if item == letter {
								isCorrect = true
								break
							}
						}
						opt := &domain.QuestionOption{
							OptionText: &optTextVal,
							IsCorrect:  isCorrect,
							QuestionID: q.ID,
						}
						_ = u.repo.CreateQuestionOption(ctx, opt)
					} else if key == "option_a" || key == "option_b" {
						return createdCount, errors.New("option_a and option_b are required for MCQ/MSQ")
					}
				}
			}
		}

		createdCount++
	}

	return createdCount, nil
}

func (u *testSeriesUsecase) GetQuestionsByTestID(ctx context.Context, testID int64) ([]domain.Question, error) {
	return u.repo.GetQuestionsByTestID(ctx, testID)
}

func (u *testSeriesUsecase) UpdateTestSeries(ctx context.Context, id int64, ts *domain.TestSeries) error {
	return u.repo.UpdateTestSeries(ctx, id, ts)
}

func (u *testSeriesUsecase) DeleteTestSeries(ctx context.Context, id int64) error {
	return u.repo.DeleteTestSeries(ctx, id)
}

