package domain

import (
	"context"
	"time"
)

// Note represents the notes table
type Note struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	Title       string    `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description string    `gorm:"column:description;type:text;not null" json:"description"`
	NoteType    string    `gorm:"column:note_type;type:varchar(50);not null" json:"noteType"`
	IsFree      bool      `gorm:"column:is_free;type:boolean;not null" json:"isFree"`
	Price       float64   `gorm:"column:price;type:numeric(10,2);not null" json:"price"`
	BatchID     string    `gorm:"column:batch_id;type:varchar(50);not null" json:"batchId"`
	FileURL     string    `gorm:"column:file_url;type:text;not null" json:"fileUrl"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	CourseID    *int64    `gorm:"column:course_id" json:"courseId"`
	HasSvgs     bool      `gorm:"column:has_svgs;type:boolean;default:false" json:"hasSvgs"`
	PageCount   int       `gorm:"column:page_count;type:integer;default:0" json:"pageCount"`
	Category    string    `gorm:"column:category;type:varchar(100);not null" json:"category"`
	Slug        string    `gorm:"column:slug;type:varchar(100)" json:"slug"`

	// Virtual fields populated dynamically
	CourseName  string    `gorm:"column:course_name;<-:false" json:"courseName"`
	IsUnlocked  bool      `gorm:"-" json:"isUnlocked"`
	SVGPageURLs []string  `gorm:"-" json:"svgPageUrls"`
}

func (Note) TableName() string {
	return "notes"
}

// NoteAccess represents the note_accesses table
type NoteAccess struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	NoteID    int64     `gorm:"column:note_id;not null" json:"noteId"`
	StudentID int64     `gorm:"column:student_id;not null" json:"studentId"`
}

func (NoteAccess) TableName() string {
	return "note_accesses"
}

// Student represents the students table
type Student struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"userId"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Student) TableName() string {
	return "students"
}

// Enrollment represents the enrollments table
type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	CourseID  int64     `gorm:"column:course_id;not null" json:"courseId"`
	StudentID int64     `gorm:"column:student_id;not null" json:"studentId"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

type NoteRepository interface {
	GetNotes(ctx context.Context, filters map[string]string) ([]Note, error)
	GetNoteByIDOrSlug(ctx context.Context, idOrSlug string) (*Note, error)
	CreateNote(ctx context.Context, note *Note) error
	UpdateNote(ctx context.Context, note *Note) error
	DeleteNote(ctx context.Context, id int64) error

	GetStudentByUserID(ctx context.Context, userID int64) (*Student, error)
	GetEnrolledCourseIDs(ctx context.Context, studentID int64) ([]int64, error)
	HasNoteAccess(ctx context.Context, studentID, noteID int64) (bool, error)
	IsStudentEnrolledInCourse(ctx context.Context, studentID, courseID int64) (bool, error)
}

type NoteUsecase interface {
	GetNotes(ctx context.Context, userID int64, role string, filters map[string]string) ([]Note, error)
	GetClassNotes(ctx context.Context, userID int64, role string, filters map[string]string) ([]Note, error)
	GetNoteByIDOrSlug(ctx context.Context, userID int64, role string, idOrSlug string) (*Note, bool, error)
	CreateNote(ctx context.Context, note *Note) error
	UpdateNote(ctx context.Context, idOrSlug string, updates map[string]interface{}) (*Note, error)
	DeleteNote(ctx context.Context, idOrSlug string) error
	HasAccess(ctx context.Context, userID int64, role string, noteIDOrSlug string) (bool, error)
}
