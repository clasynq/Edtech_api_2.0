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

// TestSeries represents the test_series table
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
	HasAccess   bool       `json:"hasAccess" gorm:"-"`

	// Preloaded list of tests
	Tests []Test `gorm:"foreignKey:TestSeriesID" json:"tests,omitempty"`
}

func (TestSeries) TableName() string {
	return "test_series"
}

// Test represents the tests table
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

// QuestionOption represents the question_options table
type QuestionOption struct {
	ID             int64   `gorm:"primaryKey;column:id" json:"id"`
	OptionText     *string `gorm:"column:option_text;type:text" json:"optionText"`
	OptionImageURL *string `gorm:"column:option_image_url;type:text" json:"optionImageUrl"`
	IsCorrect      bool    `gorm:"column:is_correct;type:boolean;not null;default:false" json:"isCorrect,omitempty"` // Hashed/Omitted during ongoing exam
	QuestionID     int64   `gorm:"column:question_id;not null" json:"questionId"`
}

func (QuestionOption) TableName() string {
	return "question_options"
}

// Question represents the questions table
type Question struct {
	ID                  int64            `gorm:"primaryKey;column:id" json:"id"`
	QuestionType        string           `gorm:"column:question_type;type:varchar(50);not null" json:"questionType"` // e.g. MCQ, MSQ, NAT
	QuestionText        *string          `gorm:"column:question_text;type:text" json:"questionText"`
	QuestionImageURL    *string          `gorm:"column:question_image_url;type:text" json:"questionImageUrl"`
	CorrectAnswer       *string          `gorm:"column:correct_answer;type:text" json:"correctAnswer,omitempty"` // Hashed/Omitted during ongoing exam
	Marks               int              `gorm:"column:marks;type:integer;not null" json:"marks"`
	NegativeMarks       float64          `gorm:"column:negative_marks;type:numeric(5,2);not null" json:"negativeMarks"`
	QuestionTimer       *int             `gorm:"column:question_timer;type:integer" json:"questionTimer"`
	ExplanationText     *string          `gorm:"column:explanation_text;type:text" json:"explanationText,omitempty"` // Hashed/Omitted during ongoing exam
	ExplanationImageURL *string          `gorm:"column:explanation_image_url;type:text" json:"explanationImageUrl,omitempty"` // Hashed/Omitted during ongoing exam
	CreatedAt           time.Time        `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	TestID              int64            `gorm:"column:test_id;not null" json:"testId"`
	Options             []QuestionOption `gorm:"foreignKey:QuestionID" json:"options,omitempty"`
}

func (Question) TableName() string {
	return "questions"
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

type TestSeriesRepository interface {
	GetTestSeries(ctx context.Context, filters map[string]string) ([]TestSeries, error)
	GetTestSeriesByIDOrSlug(ctx context.Context, idOrSlug string) (*TestSeries, error)
	CreateTestSeries(ctx context.Context, ts *TestSeries) error
	UpdateTestSeries(ctx context.Context, id int64, ts *TestSeries) error
	DeleteTestSeries(ctx context.Context, id int64) error

	GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*Test, error)
	CreateTest(ctx context.Context, test *Test) error
	UpdateTest(ctx context.Context, test *Test) error
	DeleteTest(ctx context.Context, id int64) error
	GetTestsBySeriesID(ctx context.Context, seriesID int64) ([]Test, error)

	GetQuestionsByTestID(ctx context.Context, testID int64) ([]Question, error)
	GetQuestionByID(ctx context.Context, id int64) (*Question, error)
	CreateQuestion(ctx context.Context, question *Question) error
	CreateQuestionOption(ctx context.Context, option *QuestionOption) error
	DeleteQuestion(ctx context.Context, id int64) error

	GetStudentByUserID(ctx context.Context, userID int64) (*Student, error)
	HasTestSeriesAccess(ctx context.Context, studentID, seriesID int64) (bool, error)
	IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error)
}

type TestSeriesUsecase interface {
	GetTestSeries(ctx context.Context, userID int64, role string, filters map[string]string) ([]TestSeries, error)
	GetTestSeriesByIDOrSlug(ctx context.Context, userID int64, role string, idOrSlug string) (*TestSeries, bool, error)
	CreateTestSeries(ctx context.Context, ts *TestSeries) error
	UpdateTestSeries(ctx context.Context, id int64, ts *TestSeries) error
	DeleteTestSeries(ctx context.Context, id int64) error

	GetTestByIDOrSlug(ctx context.Context, idOrSlug string) (*Test, error)
	CreateTest(ctx context.Context, test *Test) error
	UpdateTest(ctx context.Context, id int64, test *Test) error
	DeleteTest(ctx context.Context, id int64) error
	AddQuestion(ctx context.Context, q *Question, options []QuestionOption) error
	CreateQuestion(ctx context.Context, q *Question, options []QuestionOption) error
	DeleteQuestion(ctx context.Context, id int64) error
	GetQuestionsByTestID(ctx context.Context, testID int64) ([]Question, error)
	UploadQuestions(ctx context.Context, testID int64, data []map[string]interface{}) (int, error)
	HasAccess(ctx context.Context, userID int64, role string, seriesID int64) (bool, error)
}
