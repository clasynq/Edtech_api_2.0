package domain

import (
	"context"
	"time"
)

// User represents the users table reference
type User struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	FullName  string    `gorm:"column:full_name"`
	Username  string    `gorm:"column:username"`
	Email     string    `gorm:"column:email"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (User) TableName() string {
	return "users"
}

// Student represents the students table reference
type Student struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id"`
	User      User      `gorm:"foreignKey:UserID;references:ID"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Student) TableName() string {
	return "students"
}

// Enrollment represents the enrollments table reference
type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	StudentID int64     `gorm:"column:student_id"`
	CourseID  int64     `gorm:"column:course_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

// TestSeries represents the test_series table reference
type TestSeries struct {
	ID          int64      `gorm:"primaryKey;column:id" json:"id"`
	Title       string     `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description string     `gorm:"column:description;type:text;not null" json:"description"`
	BannerURL   string     `gorm:"column:banner_url;type:text;not null" json:"bannerUrl"`
	Category    string     `gorm:"column:category;type:varchar(100);not null" json:"category"`
	BatchID     *string    `gorm:"column:batch_id;type:varchar(50)" json:"batchId"`
	IsPublished bool       `gorm:"column:is_published;type:boolean;not null;default:false" json:"isPublished"`
	StartDate   *time.Time `gorm:"column:start_date;type:date" json:"startDate"`
	EndDate     *time.Time `gorm:"column:end_date;type:date" json:"endDate"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	CourseID    *int64     `gorm:"column:course_id" json:"courseId"`
	IsFree      bool       `gorm:"column:is_free;type:boolean;not null;default:false" json:"isFree"`
	Price       float64    `gorm:"column:price;type:numeric(10,2);not null" json:"price"`
	Slug        string     `gorm:"column:slug;type:varchar(100);uniqueIndex" json:"slug"`
}

func (TestSeries) TableName() string {
	return "test_series"
}

// Test represents the tests table reference
type Test struct {
	ID              int64     `gorm:"primaryKey;column:id" json:"id"`
	Title           string    `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description     string    `gorm:"column:description;type:text;not null" json:"description"`
	DurationMinutes int       `gorm:"column:duration_minutes;type:integer;not null" json:"durationMinutes"`
	TotalMarks      int       `gorm:"column:total_marks;type:integer;not null" json:"totalMarks"`
	NegativeMarking bool      `gorm:"column:negative_marking;type:boolean;not null" json:"negativeMarking"`
	Instructions    *string   `gorm:"column:instructions;type:text" json:"instructions"`
	IsPublished     bool      `gorm:"column:is_published;type:boolean;not null;default:false" json:"isPublished"`
	CreatedAt       time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	TestSeriesID    int64     `gorm:"column:test_series_id;not null" json:"testSeriesId"`
	Slug            string    `gorm:"column:slug;type:varchar(100);uniqueIndex" json:"slug"`
}

func (Test) TableName() string {
	return "tests"
}

// QuestionOption represents the question_options table reference
type QuestionOption struct {
	ID             int64   `gorm:"primaryKey;column:id" json:"id"`
	OptionText     *string `gorm:"column:option_text;type:text" json:"optionText"`
	OptionImageURL *string `gorm:"column:option_image_url;type:text" json:"optionImageUrl"`
	IsCorrect      bool    `gorm:"column:is_correct;type:boolean;not null;default:false" json:"isCorrect,omitempty"` // Omitted/hidden during ongoing exam
	QuestionID     int64   `gorm:"column:question_id;not null" json:"questionId"`
}

func (QuestionOption) TableName() string {
	return "question_options"
}

// Question represents the questions table reference
type Question struct {
	ID                  int64            `gorm:"primaryKey;column:id" json:"id"`
	QuestionType        string           `gorm:"column:question_type;type:varchar(50);not null" json:"questionType"` // e.g. MCQ, MSQ, NAT
	QuestionText        *string          `gorm:"column:question_text;type:text" json:"questionText"`
	QuestionImageURL    *string          `gorm:"column:question_image_url;type:text" json:"questionImageUrl"`
	CorrectAnswer       *string          `gorm:"column:correct_answer;type:text" json:"correctAnswer,omitempty"` // Omitted/hidden during ongoing exam
	Marks               int              `gorm:"column:marks;type:integer;not null" json:"marks"`
	NegativeMarks       float64          `gorm:"column:negative_marks;type:numeric(5,2);not null" json:"negativeMarks"`
	QuestionTimer       *int             `gorm:"column:question_timer;type:integer" json:"questionTimer"`
	ExplanationText     *string          `gorm:"column:explanation_text;type:text" json:"explanationText,omitempty"` // Omitted/hidden during ongoing exam
	ExplanationImageURL *string          `gorm:"column:explanation_image_url;type:text" json:"explanationImageUrl,omitempty"` // Omitted/hidden during ongoing exam
	CreatedAt           time.Time        `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	TestID              int64            `gorm:"column:test_id;not null" json:"testId"`
	Options             []QuestionOption `gorm:"foreignKey:QuestionID" json:"options,omitempty"`
}

func (Question) TableName() string {
	return "questions"
}

// StudentTestAttempt represents the student_test_attempts table
type StudentTestAttempt struct {
	ID           int64      `gorm:"primaryKey;column:id" json:"id"`
	StartedAt    time.Time  `gorm:"column:started_at;type:timestamp with time zone" json:"startedAt"`
	SubmittedAt  *time.Time `gorm:"column:submitted_at;type:timestamp with time zone" json:"submittedAt"`
	Score        float64    `gorm:"column:score;type:numeric(6,2);not null" json:"score"`
	Status       string     `gorm:"column:status;type:varchar(50);not null" json:"status"` // ongoing, submitted
	StudentID    int64      `gorm:"column:student_id;not null" json:"studentId"`
	TestID       int64      `gorm:"column:test_id;not null" json:"testId"`
	Slug         string     `gorm:"column:slug;type:varchar(100);uniqueIndex" json:"slug"`
}

func (StudentTestAttempt) TableName() string {
	return "student_test_attempts"
}

// StudentAnswer represents the student_answers table
type StudentAnswer struct {
	ID             int64   `gorm:"primaryKey;column:id" json:"id"`
	SelectedAnswer *string `gorm:"column:selected_answer;type:text" json:"selectedAnswer"`
	IsCorrect      bool    `gorm:"column:is_correct;type:boolean;not null" json:"isCorrect"`
	MarksObtained  float64 `gorm:"column:marks_obtained;type:numeric(5,2);not null" json:"marksObtained"`
	QuestionID     int64   `gorm:"column:question_id;not null" json:"questionId"`
	AttemptID      int64   `gorm:"column:attempt_id;not null" json:"attemptId"`
}

func (StudentAnswer) TableName() string {
	return "student_answers"
}

// TestResult represents the test_results table
type TestResult struct {
	ID                  int64     `gorm:"primaryKey;column:id" json:"id"`
	Score               float64   `gorm:"column:score;type:numeric(6,2);not null" json:"score"`
	CorrectAnswersCount int       `gorm:"column:correct_answers_count;type:integer;not null" json:"correctAnswersCount"`
	WrongAnswersCount   int       `gorm:"column:wrong_answers_count;type:integer;not null" json:"wrongAnswersCount"`
	UnattemptedCount    int       `gorm:"column:unattempted_count;type:integer;not null" json:"unattemptedCount"`
	Accuracy            float64   `gorm:"column:accuracy;type:numeric(5,2);not null" json:"accuracy"`
	TimeTakenSeconds    int       `gorm:"column:time_taken_seconds;type:integer;not null" json:"timeTakenSeconds"`
	Rank                *int      `gorm:"column:rank;type:integer" json:"rank"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	AttemptID           int64     `gorm:"column:attempt_id;not null" json:"attemptId"`
}

func (TestResult) TableName() string {
	return "test_results"
}

// TestSeriesAccess represents the test_series_accesses table
type TestSeriesAccess struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	StudentID    int64     `gorm:"column:student_id;not null" json:"studentId"`
	TestSeriesID int64     `gorm:"column:test_series_id;not null" json:"testSeriesId"`
}

func (TestSeriesAccess) TableName() string {
	return "test_series_accesses"
}

type AttemptMonitorData struct {
	ID           int64       `json:"id"`
	StudentID    int64       `json:"studentId"`
	StudentName  string      `json:"studentName"`
	StudentEmail string      `json:"studentEmail"`
	StartedAt    time.Time   `json:"startedAt"`
	SubmittedAt  *time.Time  `json:"submittedAt"`
	Score        float64     `json:"score"`
	Status       string      `json:"status"`
	Result       *TestResult `json:"result"`
}

type CbtExamRepository interface {
	GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*Test, error)
	GetTestSeriesByIDOrSlug(ctx context.Context, idOrSlug string) (*TestSeries, error)
	GetQuestionsByTestID(ctx context.Context, testID int64) ([]Question, error)
	GetQuestionByID(ctx context.Context, id int64) (*Question, error)

	GetStudentByUserID(ctx context.Context, userID int64) (*Student, error)
	HasTestSeriesAccess(ctx context.Context, studentID, seriesID int64) (bool, error)
	IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error)

	GetOngoingAttempt(ctx context.Context, studentID, testID int64) (*StudentTestAttempt, error)
	GetAttemptByIDOrSlug(ctx context.Context, idOrSlug string) (*StudentTestAttempt, error)
	CreateAttempt(ctx context.Context, attempt *StudentTestAttempt) error
	UpdateAttempt(ctx context.Context, attempt *StudentTestAttempt) error

	GetStudentAnswersByAttemptID(ctx context.Context, attemptID int64) ([]StudentAnswer, error)
	GetStudentAnswerForQuestion(ctx context.Context, attemptID, questionID int64) (*StudentAnswer, error)
	SaveStudentAnswer(ctx context.Context, ans *StudentAnswer) error

	GetTestResultByAttemptID(ctx context.Context, attemptID int64) (*TestResult, error)
	CreateTestResult(ctx context.Context, res *TestResult) error
	UpdateTestResult(ctx context.Context, res *TestResult) error
	GetResultsForTest(ctx context.Context, testID int64) ([]TestResult, error)
	GetLeaderboard(ctx context.Context, testID int64) ([]map[string]interface{}, error)
	GetAttemptsMonitoring(ctx context.Context, testID int64) ([]AttemptMonitorData, error)
}

type CbtExamUsecase interface {
	StartAttempt(ctx context.Context, userID int64, testIDOrSlug string) (*StudentTestAttempt, []Question, error)
	SubmitAnswer(ctx context.Context, userID int64, attemptSlug string, questionID int64, selectedAnswer string) (*StudentAnswer, error)
	SubmitTest(ctx context.Context, userID int64, attemptSlug string) (*TestResult, error)
	GetAttemptResult(ctx context.Context, userID int64, attemptSlug string) (map[string]interface{}, error)
	GetLeaderboard(ctx context.Context, testIDOrSlug string) ([]map[string]interface{}, error)
	HasAccess(ctx context.Context, userID int64, role string, seriesID int64) (bool, error)
	GetAttemptsMonitoring(ctx context.Context, userID int64, testIDStr string) (map[string]interface{}, error)
	GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*Test, error)
}
