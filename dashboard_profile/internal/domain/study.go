package domain

import (
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
	if d == nil || t.IsZero() {
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

// Course represents the courses table
type Course struct {
	ID                 int64           `gorm:"primaryKey;column:id" json:"id"`
	CourseName         string          `gorm:"column:course_name;type:varchar(255);not null" json:"courseName"`
	BatchID            string          `gorm:"column:batch_id;type:varchar(50);unique;not null" json:"batchId"`
	Category           string          `gorm:"column:category;type:varchar(100);not null" json:"category"`
	Language           string          `gorm:"column:language;type:varchar(50);not null" json:"language"`
	Description        string          `gorm:"column:description;type:text;not null" json:"description"`
	TeacherID          *int64          `gorm:"column:teacher_id" json:"teacher"`
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
}

func (ClassSchedule) TableName() string {
	return "class_schedules"
}

// Enrollment represents the enrollments table
type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	StudentID int64     `gorm:"column:student_id;not null" json:"studentId"`
	CourseID  int64     `gorm:"column:course_id;not null" json:"courseId"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}
