package usecase

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"clasynq/api/cbt_exam/internal/domain"
	"github.com/redis/go-redis/v9"
)

type cbtExamUsecase struct {
	repo domain.CbtExamRepository
	rdb  *redis.Client
}

func NewCbtExamUsecase(repo domain.CbtExamRepository, rdb *redis.Client) domain.CbtExamUsecase {
	return &cbtExamUsecase{
		repo: repo,
		rdb:  rdb,
	}
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

	// 2. Check for existing attempt
	attempt, err := u.repo.GetLastAttemptForStudentAndTest(ctx, student.ID, test.ID)
	if err != nil {
		return nil, nil, err
	}

	if attempt != nil {
		if attempt.Status == "submitted" {
			return nil, nil, errors.New("You have already completed this test.")
		}

		if attempt.Status == "ongoing" {
			// Check if time has expired
			elapsedSeconds := time.Since(attempt.StartedAt).Seconds()
			durationSeconds := float64(test.DurationMinutes * 60)
			if elapsedSeconds >= durationSeconds {
				// Auto-submit the attempt
				_, _ = u.SubmitTest(ctx, userID, attempt.Slug)
				return nil, nil, errors.New("Time limit expired. Test submitted automatically.")
			}
			// Otherwise allow resume/entry (within remaining seconds)
		}
	} else {
		// 3. Create new attempt if not found
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
	var questions []domain.Question
	cacheKey := fmt.Sprintf("cbt_questions:test_id:%d", test.ID)
	cacheHit := false

	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			if err := json.Unmarshal([]byte(val), &questions); err == nil {
				cacheHit = true
			}
		}
	}

	if !cacheHit {
		var err error
		questions, err = u.repo.GetQuestionsByTestID(ctx, test.ID)
		if err != nil {
			return nil, nil, err
		}
		if u.rdb != nil {
			if raw, err := json.Marshal(questions); err == nil {
				_ = u.rdb.Set(ctx, cacheKey, string(raw), 10*time.Minute).Err()
			}
		}
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

		if u.rdb != nil {
			u.rdb.Del(bgCtx, fmt.Sprintf("cbt_leaderboard:%d", test.ID))
			u.rdb.Del(bgCtx, fmt.Sprintf("cbt_leaderboard:%s", test.Slug))
			u.rdb.Del(bgCtx, fmt.Sprintf("cbt_leaderboard:id:%d", test.ID))
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

	cacheKey := fmt.Sprintf("cbt_attempt_result:%s", attemptSlug)
	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cached map[string]interface{}
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		}
	}

	result, err := u.repo.GetTestResultByAttemptID(ctx, attempt.ID)
	if err != nil {
		return nil, err
	}

	test, err := u.repo.GetTestByIDOrSlug(ctx, strconv.FormatInt(attempt.TestID, 10))
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

	// Calculate total participants and rank
	allResults, err := u.repo.GetResultsForTest(ctx, attempt.TestID)
	totalParticipants := len(allResults)
	rank := 1
	if result != nil {
		if result.Rank != nil && *result.Rank > 0 {
			rank = *result.Rank
		} else {
			// fallback calculate rank dynamically
			for _, r := range allResults {
				if r.Score > result.Score {
					rank++
				} else if r.Score == result.Score && r.TimeTakenSeconds < result.TimeTakenSeconds {
					rank++
				}
			}
		}
	}

	// Map answers by question ID
	answersMap := make(map[int64]domain.StudentAnswer)
	for _, ans := range answers {
		answersMap[ans.QuestionID] = ans
	}

	reviews := make([]map[string]interface{}, 0, len(questions))
	for _, q := range questions {
		ans, hasAnswer := answersMap[q.ID]
		selectedAnswer := ""
		if hasAnswer && ans.SelectedAnswer != nil {
			selectedAnswer = *ans.SelectedAnswer
		}
		isCorrect := false
		if hasAnswer {
			isCorrect = ans.IsCorrect
		}
		marksObtained := 0.0
		if hasAnswer {
			marksObtained = ans.MarksObtained
		}

		optionsList := make([]map[string]interface{}, 0, len(q.Options))
		for _, opt := range q.Options {
			optionsList = append(optionsList, map[string]interface{}{
				"id":               opt.ID,
				"optionText":       opt.OptionText,
				"option_text":      opt.OptionText,
				"optionImageUrl":   opt.OptionImageURL,
				"option_image_url": opt.OptionImageURL,
				"isCorrect":        opt.IsCorrect,
				"is_correct":       opt.IsCorrect,
				"questionId":       opt.QuestionID,
				"question_id":      opt.QuestionID,
			})
		}

		reviews = append(reviews, map[string]interface{}{
			"id":                  q.ID,
			"questionType":        q.QuestionType,
			"questionText":        q.QuestionText,
			"questionImageUrl":    q.QuestionImageURL,
			"correctAnswer":       q.CorrectAnswer,
			"marks":               q.Marks,
			"negativeMarks":       q.NegativeMarks,
			"explanationText":     q.ExplanationText,
			"explanationImageUrl": q.ExplanationImageURL,
			"options":             optionsList,
			"selectedAnswer":      selectedAnswer,
			"isCorrect":           isCorrect,
			"marksObtained":       marksObtained,
		})
	}

	var resultMap map[string]interface{}
	if result != nil {
		resultMap = map[string]interface{}{
			"id":                    result.ID,
			"attempt":               result.AttemptID,
			"attemptId":             result.AttemptID,
			"score":                 result.Score,
			"correctAnswersCount":   result.CorrectAnswersCount,
			"correct_answers_count": result.CorrectAnswersCount,
			"wrongAnswersCount":     result.WrongAnswersCount,
			"wrong_answers_count":   result.WrongAnswersCount,
			"unattemptedCount":      result.UnattemptedCount,
			"unattempted_count":     result.UnattemptedCount,
			"accuracy":              result.Accuracy,
			"timeTakenSeconds":      result.TimeTakenSeconds,
			"time_taken_seconds":    result.TimeTakenSeconds,
			"rank":                  rank,
			"created_at":            result.CreatedAt,
			"createdAt":             result.CreatedAt,
		}
	}

	testTitle := ""
	totalMarks := 0
	if test != nil {
		testTitle = test.Title
		totalMarks = test.TotalMarks
	}

	payload := map[string]interface{}{
		"testTitle":         testTitle,
		"totalMarks":        totalMarks,
		"result":            resultMap,
		"reviews":           reviews,
		"totalParticipants": totalParticipants,
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(payload); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 30*time.Minute).Err()
		}
	}

	return payload, nil
}

func (u *cbtExamUsecase) GetLeaderboard(ctx context.Context, testIDOrSlug string) ([]map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("cbt_leaderboard:%s", testIDOrSlug)
	if u.rdb != nil {
		if val, err := u.rdb.Get(ctx, cacheKey).Result(); err == nil {
			var cached []map[string]interface{}
			if err := json.Unmarshal([]byte(val), &cached); err == nil {
				return cached, nil
			}
		}
	}

	test, err := u.repo.GetTestByIDOrSlug(ctx, testIDOrSlug)
	if err != nil {
		return nil, err
	}
	if test == nil {
		return nil, errors.New("test not found")
	}

	leaderboard, err := u.repo.GetLeaderboard(ctx, test.ID)
	if err != nil {
		return nil, err
	}

	if u.rdb != nil {
		if raw, err := json.Marshal(leaderboard); err == nil {
			_ = u.rdb.Set(ctx, cacheKey, string(raw), 5*time.Minute).Err()
			cacheKeyID := fmt.Sprintf("cbt_leaderboard:id:%d", test.ID)
			_ = u.rdb.Set(ctx, cacheKeyID, string(raw), 5*time.Minute).Err()
		}
	}

	return leaderboard, nil
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
