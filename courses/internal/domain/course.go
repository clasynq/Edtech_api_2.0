package domain

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateStr handles YYYY-MM-DD mapping for Postgres date and JSON fields
type DateStr time.Time

func (d *DateStr) MarshalJSON() ([]byte, error) {
	t := time.Time(*d)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, t.Format("2006-01-02"))), nil
}

func (d *DateStr) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = DateStr(t)
	return nil
}

func (d *DateStr) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("failed to scan DateStr: expected time.Time, got %T", value)
	}
	*d = DateStr(t)
	return nil
}

func (d DateStr) Value() (driver.Value, error) {
	t := time.Time(d)
	if t.IsZero() {
		return nil, nil
	}
	return t, nil
}

func (d DateStr) ToTime() time.Time {
	return time.Time(d)
}

// TimeStr handles HH:MM:SS mapping for Postgres time and JSON fields
type TimeStr string

func (t *TimeStr) Scan(value interface{}) error {
	if value == nil {
		*t = ""
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = TimeStr(v.Format("15:04:05"))
	case []byte:
		*t = TimeStr(string(v))
	case string:
		*t = TimeStr(v)
	default:
		return fmt.Errorf("failed to scan TimeStr: unexpected type %T", value)
	}
	return nil
}

func (t TimeStr) Value() (driver.Value, error) {
	if t == "" {
		return nil, nil
	}
	return string(t), nil
}

// Subject represents the subjects table
type Subject struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	SubjectName string    `gorm:"column:subject_name;type:varchar(255);unique;not null" json:"subjectName"`
	MeetingLink string    `gorm:"column:meeting_link;type:text" json:"meetingLink"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Subject) TableName() string {
	return "subjects"
}

// Teacher represents the teachers table
type Teacher struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`
	Email          string    `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`
	Name           string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Specialization string    `gorm:"column:specialization;type:varchar(255);not null" json:"specialization"`
	PhotoURL       string    `gorm:"column:photo_url;type:text" json:"photoUrl"`
	Category       string    `gorm:"column:category;type:varchar(100)" json:"category"`
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamp with time zone" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:timestamp with time zone" json:"updatedAt"`
}

func (Teacher) TableName() string {
	return "teachers"
}

// Course represents the courses table
type Course struct {
	ID                 int64           `gorm:"primaryKey;column:id" json:"id"`
	CourseName         string          `gorm:"column:course_name;type:varchar(255);not null" json:"courseName"`
	BatchID            string          `gorm:"column:batch_id;type:varchar(50);unique;not null" json:"batchId"`
	Category           string          `gorm:"column:category;type:varchar(100);not null" json:"category"`
	Language           string          `gorm:"column:language;type:varchar(50);not null" json:"language"`
	Description        string          `gorm:"column:description;type:text;not null" json:"description"`
	TeacherID          *int64          `gorm:"column:teacher_id" json:"teacher"` // FK to primary teacher
	Teacher            *Teacher        `gorm:"foreignKey:TeacherID;references:ID" json:"teacherDetails,omitempty"`
	Teachers           []Teacher       `gorm:"many2many:courses_teachers;joinForeignKey:course_id;joinReferences:teacher_id" json:"teachersDetails"`
	Subjects           []Subject       `gorm:"many2many:courses_subjects;joinForeignKey:course_id;joinReferences:subject_id" json:"subjectsDetails"`
	OriginalPrice      float64         `gorm:"column:original_price;type:numeric(10,2);not null" json:"originalPrice"`
	DiscountPercentage int             `gorm:"column:discount_percentage;type:integer;not null" json:"discountPercentage"`
	FinalPrice         float64         `gorm:"column:final_price;type:numeric(10,2);not null" json:"finalPrice"`
	CourseStatus       string          `gorm:"column:course_status;type:varchar(50);not null" json:"courseStatus"`
	StartDate          DateStr         `gorm:"column:start_date;type:date;not null" json:"startDate"`
	EndDate            DateStr         `gorm:"column:end_date;type:date;not null" json:"endDate"`
	AccessDuration     string          `gorm:"column:access_duration;type:varchar(100);not null" json:"accessDuration"`
	BannerURL          string          `gorm:"column:banner_url;type:text" json:"bannerUrl"`
	MeetingLink        string          `gorm:"column:meeting_link;type:text" json:"meetingLink"`
	TotalStudents      int             `gorm:"column:total_students;type:integer;default:0" json:"totalStudents"`
	IsFeatured         bool            `gorm:"column:is_featured;type:boolean;default:false" json:"isFeatured"`
	Visibility         string          `gorm:"column:visibility;type:varchar(20);default:'public'" json:"visibility"`
	TeacherSubjects    json.RawMessage `gorm:"column:teacher_subjects;type:jsonb" json:"teacherSubjects"`
	Slug               string          `gorm:"column:slug;type:varchar(100);uniqueIndex" json:"slug"`
	CreatedAt          time.Time       `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Course) TableName() string {
	return "courses"
}

// ClassSchedule represents the class_schedules table
type ClassSchedule struct {
	ID                 int64          `gorm:"primaryKey;column:id" json:"id"`
	CourseID           int64          `gorm:"column:course_id;not null" json:"course"`
	Course             *Course        `gorm:"foreignKey:CourseID;references:ID" json:"-"`
	SubjectID          *int64         `gorm:"column:subject_id" json:"subject"`
	Subject            *Subject       `gorm:"foreignKey:SubjectID;references:ID" json:"-"`
	BatchID            string         `gorm:"column:batch_id;type:varchar(50)" json:"batchId"`
	TeacherID          int64          `gorm:"column:teacher_id;not null" json:"teacher"`
	Teacher            *Teacher       `gorm:"foreignKey:TeacherID;references:ID" json:"-"`
	TopicName          string         `gorm:"column:topic_name;type:varchar(255);not null" json:"topicName"`
	ClassDate          DateStr        `gorm:"column:class_date;type:date;not null" json:"classDate"`
	StartTime          TimeStr        `gorm:"column:start_time;type:time;not null" json:"startTime"`
	EndTime            TimeStr        `gorm:"column:end_time;type:time;not null" json:"endTime"`
	ClassStatus        string         `gorm:"column:class_status;type:varchar(20);default:'pending'" json:"classStatus"`
	RescheduleReason   *string        `gorm:"column:reschedule_reason;type:text" json:"rescheduleReason"`
	ClassNotesURL      *string        `gorm:"column:class_notes_url;type:text" json:"classNotesUrl"`
	RecordedClassURL   *string        `gorm:"column:recorded_class_url;type:text" json:"recordedClassUrl"`
	CreatedAt          time.Time      `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	
	// Read-only virtual/serializer fields
	CourseName            string  `gorm:"-" json:"courseName"`
	CourseBannerURL       string  `gorm:"-" json:"courseBannerUrl"`
	TeacherName           string  `gorm:"-" json:"teacherName"`
	TeacherSpecialization string  `gorm:"-" json:"teacherSpecialization"`
	SubjectName           string  `gorm:"-" json:"subjectName"`
	MeetingLink           string  `gorm:"-" json:"meetingLink"`
}

func (ClassSchedule) TableName() string {
	return "class_schedules"
}

// Enrollment represents the enrollments table (simplified reference)
type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	StudentID int64     `gorm:"column:student_id"`
	CourseID  int64     `gorm:"column:course_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

// Category represents the categories table
type Category struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name;type:varchar(100);unique;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Category) TableName() string {
	return "categories"
}

// Analytics Stats Output Structs
type OverallStats struct {
	TotalClasses       int64   `json:"totalClasses"`
	Completed          int64   `json:"completed"`
	Pending            int64   `json:"pending"`
	Cancelled          int64   `json:"cancelled"`
	Rescheduled        int64   `json:"rescheduled"`
	ProgressPercentage float64 `json:"progressPercentage"`
}

type BatchAnalytics struct {
	ID                 int64    `json:"id"`
	CourseName         string   `json:"courseName"`
	BatchID            string   `json:"batchId"`
	TeacherName        string   `json:"teacherName"`
	CourseBannerURL    string   `json:"courseBannerUrl"`
	TotalStudents      int64    `json:"totalStudents"`
	TotalClasses       int64    `json:"totalClasses"`
	Completed          int64    `json:"completed"`
	Pending            int64    `json:"pending"`
	Cancelled          int64    `json:"cancelled"`
	Rescheduled        int64    `json:"rescheduled"`
	StartDate          *DateStr `json:"startDate"`
	EndDate            *DateStr `json:"endDate"`
	ProgressPercentage float64  `json:"progressPercentage"`
}

type TeacherAnalytics struct {
	ID                   int64   `json:"id"`
	TeacherName          string  `json:"teacherName"`
	Specialization       string  `json:"specialization"`
	TotalClasses         int64   `json:"totalClasses"`
	Completed            int64   `json:"completed"`
	Cancelled            int64   `json:"cancelled"`
	Rescheduled          int64   `json:"rescheduled"`
	CompletionPercentage float64 `json:"completionPercentage"`
	ActiveBatches        int64   `json:"activeBatches"`
	TotalStudents        int64   `json:"totalStudents"`
}

type CourseAnalytics struct {
	ID                   int64   `json:"id"`
	CourseName           string  `json:"courseName"`
	BatchID              string  `json:"batchId"`
	TotalClasses         int64   `json:"totalClasses"`
	Completed            int64   `json:"completed"`
	Cancelled            int64   `json:"cancelled"`
	Rescheduled          int64   `json:"rescheduled"`
	CompletionPercentage float64 `json:"completionPercentage"`
	TotalStudents        int64   `json:"totalStudents"`
}

type ClassesAnalytics struct {
	OverallStats OverallStats       `json:"overallStats"`
	Batches      []BatchAnalytics   `json:"batches"`
	Teachers     []TeacherAnalytics `json:"teachers"`
	Courses      []CourseAnalytics  `json:"courses"`
}

// CourseRepository defines the data layer contracts
type CourseRepository interface {
	// Course operations
	GetCourses(ctx context.Context, role string, userID int64, isFeatured *bool, search string, category string, limit int) ([]Course, error)
	GetCourseByIDOrSlug(ctx context.Context, idOrSlug string, role string, userID int64) (*Course, error)
	CreateCourse(ctx context.Context, course *Course, teachers []int64, subjects []int64) error
	UpdateCourse(ctx context.Context, course *Course, teachers *[]int64, subjects *[]int64) error
	DeleteCourse(ctx context.Context, id int64) error

	// Teacher operations
	ListTeachers(ctx context.Context) ([]Teacher, error)

	// Subject operations
	ListSubjects(ctx context.Context) ([]Subject, error)
	CreateSubject(ctx context.Context, subject *Subject) error
	UpdateSubjectMeetingLink(ctx context.Context, id int64, link string) error

	// Class Schedule operations
	ListSchedules(ctx context.Context, filters map[string]string) ([]ClassSchedule, error)
	GetScheduleByID(ctx context.Context, id int64) (*ClassSchedule, error)
	CreateSchedule(ctx context.Context, schedule *ClassSchedule) error
	UpdateSchedule(ctx context.Context, schedule *ClassSchedule) error
	DeleteSchedule(ctx context.Context, id int64) error
	GetAnalytics(ctx context.Context, category string) (*ClassesAnalytics, error)

	// Category check
	CategoryExists(ctx context.Context, name string) (bool, error)
}

// CourseUsecase defines the business logic contracts
type CourseUsecase interface {
	GetCourses(ctx context.Context, role string, userID int64, isFeatured *bool, search string, category string, limit int) ([]Course, error)
	GetCourseByIDOrSlug(ctx context.Context, idOrSlug string, role string, userID int64) (*Course, error)
	CreateCourse(ctx context.Context, course *Course, teachers []int64, subjects []int64) error
	UpdateCourse(ctx context.Context, idOrSlug string, updates map[string]interface{}) (*Course, error)
	DeleteCourse(ctx context.Context, idOrSlug string) error

	ListTeachers(ctx context.Context) ([]Teacher, error)

	ListSubjects(ctx context.Context) ([]Subject, error)
	CreateSubject(ctx context.Context, subject *Subject) error
	UpdateSubjectMeetingLink(ctx context.Context, id int64, link string) error

	ListSchedules(ctx context.Context, filters map[string]string) ([]ClassSchedule, error)
	CreateSchedule(ctx context.Context, schedule *ClassSchedule) error
	UpdateSchedule(ctx context.Context, id int64, updates map[string]interface{}) (*ClassSchedule, error)
	DeleteSchedule(ctx context.Context, id int64) error
	GetAnalytics(ctx context.Context, category string) (*ClassesAnalytics, error)
}
