package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"clasynq/api/cbt_exam/internal/domain"
)

type cbtExamUsecase struct {
	repo domain.CbtExamRepository
}

func NewCbtExamUsecase(repo domain.CbtExamRepository) domain.CbtExamUsecase {
	return &cbtExamUsecase{repo: repo}
}

func (u *cbtExamUsecase) StartAttempt(ctx context.Context, userID int64, testIDOrSlug string) (*domain.StudentTestAttempt, []domain.Question, error) {
	test, err := u.repo.GetTestByIDOrSlug(ctx, testIDOrSlug)
	if err != nil {
		return nil, nil, err
	}
	if test == nil {
		return nil, nil, errors.New("test not found")
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if student == nil {
		return nil, nil, errors.New("student profile not found")
	}

	// 1. Verify access to test series containing this test
	hasAccess, err := u.HasAccess(ctx, userID, "student", test.TestSeriesID)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, errors.New("you do not have access to this test series")
	}

	// 2. Check for existing active attempt
	attempt, err := u.repo.GetOngoingAttempt(ctx, student.ID, test.ID)
	if err != nil {
		return nil, nil, err
	}

	// 3. Create new attempt if not found
	if attempt == nil {
		slug, err := u.generateUniqueSlug(ctx, "attempt")
		if err != nil {
			return nil, nil, err
		}
		attempt = &domain.StudentTestAttempt{
			StartedAt: time.Now(),
			Score:     0,
			Status:    "ongoing",
			StudentID: student.ID,
			TestID:    test.ID,
			Slug:      slug,
		}
		if err := u.repo.CreateAttempt(ctx, attempt); err != nil {
			return nil, nil, err
		}
	}

	// 4. Load questions
	questions, err := u.repo.GetQuestionsByTestID(ctx, test.ID)
	if err != nil {
		return nil, nil, err
	}

	// 5. SECURE: Hide correct answers and explanations during ongoing exam
	for i := range questions {
		questions[i].CorrectAnswer = nil
		questions[i].ExplanationText = nil
		questions[i].ExplanationImageURL = nil
		for j := range questions[i].Options {
			questions[i].Options[j].IsCorrect = false
		}
	}

	return attempt, questions, nil
}

func (u *cbtExamUsecase) SubmitAnswer(ctx context.Context, userID int64, attemptSlug string, questionID int64, selectedAnswer string) (*domain.StudentAnswer, error) {
	attempt, err := u.repo.GetAttemptByIDOrSlug(ctx, attemptSlug)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, errors.New("test attempt not found")
	}

	if attempt.Status != "ongoing" {
		return nil, errors.New("this test attempt has already been submitted")
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil || attempt.StudentID != student.ID {
		return nil, errors.New("unauthorized attempt ownership check")
	}

	test, err := u.repo.GetTestByIDOrSlug(ctx, strconv.FormatInt(attempt.TestID, 10))
	if err != nil {
		return nil, err
	}

	// Check time expiration
	timeExpired := time.Now().After(attempt.StartedAt.Add(time.Duration(test.DurationMinutes) * time.Minute))
	if timeExpired {
		_, _ = u.SubmitTest(ctx, userID, attemptSlug)
		return nil, errors.New("test time limit has expired")
	}

	// Load question to grade
	q, err := u.repo.GetQuestionByID(ctx, questionID)
	if err != nil {
		return nil, err
	}
	if q == nil || q.TestID != test.ID {
		return nil, errors.New("question does not belong to this test")
	}

	// Grade correctness
	isCorrect := false
	if q.QuestionType == "NAT" {
		if q.CorrectAnswer != nil {
			isCorrect = (strings.TrimSpace(selectedAnswer) == strings.TrimSpace(*q.CorrectAnswer))
		}
	} else {
		// MCQ
		for _, opt := range q.Options {
			if opt.IsCorrect {
				optIDStr := strconv.FormatInt(opt.ID, 10)
				if selectedAnswer == optIDStr || (opt.OptionText != nil && selectedAnswer == *opt.OptionText) {
					isCorrect = true
					break
				}
			}
		}
	}

	var marksObtained float64 = 0.0
	if isCorrect {
		marksObtained = float64(q.Marks)
	} else {
		if test.NegativeMarking {
			marksObtained = -q.NegativeMarks
		}
	}

	// Save student answer (insert or update)
	ans, err := u.repo.GetStudentAnswerForQuestion(ctx, attempt.ID, questionID)
	if err != nil {
		return nil, err
	}

	if ans == nil {
		ans = &domain.StudentAnswer{
			SelectedAnswer: &selectedAnswer,
			IsCorrect:      isCorrect,
			MarksObtained:  marksObtained,
			QuestionID:     questionID,
			AttemptID:      attempt.ID,
		}
	} else {
		ans.SelectedAnswer = &selectedAnswer
		ans.IsCorrect = isCorrect
		ans.MarksObtained = marksObtained
	}

	if err := u.repo.SaveStudentAnswer(ctx, ans); err != nil {
		return nil, err
	}

	return ans, nil
}

func (u *cbtExamUsecase) SubmitTest(ctx context.Context, userID int64, attemptSlug string) (*domain.TestResult, error) {
	attempt, err := u.repo.GetAttemptByIDOrSlug(ctx, attemptSlug)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, errors.New("test attempt not found")
	}

	if attempt.Status != "ongoing" {
		// Return existing results if already submitted
		res, err := u.repo.GetTestResultByAttemptID(ctx, attempt.ID)
		if err != nil {
			return nil, err
		}
		if res != nil {
			return res, nil
		}
		return nil, errors.New("attempt already submitted, but no result record found")
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil || attempt.StudentID != student.ID {
		return nil, errors.New("unauthorized attempt check")
	}

	test, err := u.repo.GetTestByIDOrSlug(ctx, strconv.FormatInt(attempt.TestID, 10))
	if err != nil {
		return nil, err
	}

	questions, err := u.repo.GetQuestionsByTestID(ctx, test.ID)
	if err != nil {
		return nil, err
	}

	answers, err := u.repo.GetStudentAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	// Map answers by question ID
	ansMap := make(map[int64]domain.StudentAnswer)
	for _, ans := range answers {
		ansMap[ans.QuestionID] = ans
	}

	var score float64 = 0.0
	correctCount := 0
	wrongCount := 0
	unattemptedCount := 0

	for _, q := range questions {
		if ans, ok := ansMap[q.ID]; ok && ans.SelectedAnswer != nil && *ans.SelectedAnswer != "" {
			score += ans.MarksObtained
			if ans.IsCorrect {
				correctCount++
			} else {
				wrongCount++
			}
		} else {
			unattemptedCount++
		}
	}

	submittedAt := time.Now()
	attempt.SubmittedAt = &submittedAt
	attempt.Status = "submitted"
	attempt.Score = score
	if err := u.repo.UpdateAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	timeTaken := int(submittedAt.Sub(attempt.StartedAt).Seconds())
	if timeTaken > test.DurationMinutes*60 {
		timeTaken = test.DurationMinutes * 60
	}

	accuracy := 0.0
	attemptedCount := correctCount + wrongCount
	if attemptedCount > 0 {
		accuracy = (float64(correctCount) / float64(attemptedCount)) * 100.0
	}

	res := &domain.TestResult{
		Score:               score,
		CorrectAnswersCount: correctCount,
		WrongAnswersCount:   wrongCount,
		UnattemptedCount:    unattemptedCount,
		Accuracy:            accuracy,
		TimeTakenSeconds:    timeTaken,
		CreatedAt:           time.Now(),
		AttemptID:           attempt.ID,
	}

	if err := u.repo.CreateTestResult(ctx, res); err != nil {
		return nil, err
	}

	// Recalculate ranks for the test
	go func() {
		bgCtx := context.Background()
		results, err := u.repo.GetResultsForTest(bgCtx, test.ID)
		if err == nil {
			for i := range results {
				rankNum := i + 1
				results[i].Rank = &rankNum
				_ = u.repo.UpdateTestResult(bgCtx, &results[i])
			}
		}
	}()

	return res, nil
}

func (u *cbtExamUsecase) GetAttemptResult(ctx context.Context, userID int64, attemptSlug string) (map[string]interface{}, error) {
	attempt, err := u.repo.GetAttemptByIDOrSlug(ctx, attemptSlug)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, errors.New("attempt not found")
	}

	if attempt.Status != "submitted" {
		return nil, errors.New("this test attempt has not been submitted yet")
	}

	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil || attempt.StudentID != student.ID {
		return nil, errors.New("unauthorized attempt access")
	}

	result, err := u.repo.GetTestResultByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	questions, err := u.repo.GetQuestionsByTestID(ctx, attempt.TestID)
	if err != nil {
		return nil, err
	}

	answers, err := u.repo.GetStudentAnswersByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"attempt":   attempt,
		"result":    result,
		"questions": questions,
		"answers":   answers,
	}, nil
}

func (u *cbtExamUsecase) GetLeaderboard(ctx context.Context, testIDOrSlug string) ([]map[string]interface{}, error) {
	test, err := u.repo.GetTestByIDOrSlug(ctx, testIDOrSlug)
	if err != nil {
		return nil, err
	}
	if test == nil {
		return nil, errors.New("test not found")
	}

	return u.repo.GetLeaderboard(ctx, test.ID)
}

func (u *cbtExamUsecase) HasAccess(ctx context.Context, userID int64, role string, seriesID int64) (bool, error) {
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

func (u *cbtExamUsecase) generateUniqueSlug(ctx context.Context, kind string) (string, error) {
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
		if kind == "attempt" {
			existing, _ := u.repo.GetAttemptByIDOrSlug(ctx, slug)
			if existing == nil {
				return slug, nil
			}
		}
	}
	return "", errors.New("failed to generate unique slug")
}

func (u *cbtExamUsecase) GetAttemptsMonitoring(ctx context.Context, userID int64, testIDStr string) (map[string]interface{}, error) {
	test, err := u.repo.GetTestByIDOrSlug(ctx, testIDStr)
	if err != nil {
		return nil, err
	}
	if test == nil {
		return nil, errors.New("test not found")
	}

	attempts, err := u.repo.GetAttemptsMonitoring(ctx, test.ID)
	if err != nil {
		return nil, err
	}

	totalAttempts := len(attempts)
	var completedCount int = 0
	var sumScore float64 = 0.0
	var highestScore float64 = 0.0

	type completedInfo struct {
		index       int
		score       float64
		submittedAt time.Time
	}
	var completedList []completedInfo

	for idx, att := range attempts {
		if att.Status == "completed" || att.Status == "submitted" {
			completedCount++
			sumScore += att.Score
			if att.Score > highestScore {
				highestScore = att.Score
			}

			subTime := att.StartedAt
			if att.SubmittedAt != nil {
				subTime = *att.SubmittedAt
			}
			completedList = append(completedList, completedInfo{
				index:       idx,
				score:       att.Score,
				submittedAt: subTime,
			})
		}
	}

	sort.Slice(completedList, func(i, j int) bool {
		if completedList[i].score != completedList[j].score {
			return completedList[i].score > completedList[j].score
		}
		return completedList[i].submittedAt.Before(completedList[j].submittedAt)
	})

	rankMap := make(map[int64]int)
	for rk, info := range completedList {
		rankMap[attempts[info.index].ID] = rk + 1
	}

	avgScore := 0.0
	if completedCount > 0 {
		avgScore = sumScore / float64(completedCount)
	}

	studentAttemptsData := []map[string]interface{}{}
	for _, att := range attempts {
		var resultData interface{} = nil
		if att.Result != nil {
			var rkVal interface{} = nil
			if rk, exists := rankMap[att.ID]; exists {
				rkVal = rk
			}
			resultData = map[string]interface{}{
				"score":            att.Result.Score,
				"correct":          att.Result.CorrectAnswersCount,
				"wrong":            att.Result.WrongAnswersCount,
				"unattempted":      att.Result.UnattemptedCount,
				"accuracy":          att.Result.Accuracy,
				"timeTakenSeconds": att.Result.TimeTakenSeconds,
				"rank":             rkVal,
			}
		}

		studentAttemptsData = append(studentAttemptsData, map[string]interface{}{
			"id":           att.ID,
			"studentId":    att.StudentID,
			"studentName":  att.StudentName,
			"studentEmail": att.StudentEmail,
			"startedAt":    att.StartedAt.Format(time.RFC3339),
			"submittedAt":  att.SubmittedAt,
			"score":        att.Score,
			"status":        att.Status,
			"result":        resultData,
		})
	}

	return map[string]interface{}{
		"testTitle":  test.Title,
		"totalMarks": test.TotalMarks,
		"stats": map[string]interface{}{
			"totalAttempts":     totalAttempts,
			"completedAttempts": completedCount,
			"averageScore":      math.Round(avgScore*100) / 100,
			"highestScore":      math.Round(highestScore*100) / 100,
		},
		"attempts": studentAttemptsData,
	}, nil
}

func (u *cbtExamUsecase) GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*domain.Test, error) {
	return u.repo.GetTestByIDOrSlug(ctx, idOrSlug)
}
