package domain

import (
	"context"
	"time"
)

type User struct {
	ID             int64      `gorm:"primaryKey;column:id" json:"id"`
	FullName       string     `gorm:"column:full_name" json:"fullName"`
	Username       string     `gorm:"column:username" json:"username"`
	ContactNumber  string     `gorm:"column:contact_number" json:"contactNumber"`
	Email          string     `gorm:"column:email" json:"email"`
	AvatarURL      string     `gorm:"column:avatar_url" json:"avatarUrl"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
}

func (User) TableName() string {
	return "users"
}

type Student struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id" json:"userId"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Student) TableName() string {
	return "students"
}

type Admin struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Email     string    `gorm:"column:email;unique;not null" json:"email"`
	Password  string    `gorm:"column:password;not null" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Admin) TableName() string {
	return "admin"
}

type Teacher struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`
	Email           string     `gorm:"column:email;unique;not null" json:"email"`
	Password        string     `gorm:"column:password;not null" json:"-"`
	Name            string     `gorm:"column:name;not null" json:"name"`
	Specialization  string     `gorm:"column:specialization;not null" json:"specialization"`
	AssignedCourses string     `gorm:"column:Assigned_courses;type:jsonb" json:"assignedCourses"` // Stored as JSON string
	Tasks           string     `gorm:"column:tasks;type:jsonb" json:"tasks"`                     // Stored as JSON string
	PhotoURL        string     `gorm:"column:photo_url" json:"photoUrl"`
	Category        string     `gorm:"column:category" json:"category"`
	DateOfBirth     *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Teacher) TableName() string {
	return "teachers"
}

type Category struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Name      string    `gorm:"column:name;unique;not null" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Category) TableName() string {
	return "categories"
}

type AdminDashboard struct {
	ID            int64 `gorm:"primaryKey;column:id"`
	TotalStudents int64 `gorm:"column:total_students"`
	TotalTeacher  int64 `gorm:"column:total_teacher"`
	ActiveBatches int64 `gorm:"column:Active_batches"`
}

func (AdminDashboard) TableName() string {
	return "admin_Dashboard"
}

type AdminActivity struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	AdminID    int64     `gorm:"column:admin_id" json:"adminId"`
	AdminEmail string    `gorm:"-" json:"adminEmail"` // virtual field loaded dynamically
	Action     string    `gorm:"column:action" json:"action"`
	EntityType string    `gorm:"column:entity_type" json:"entityType"`
	EntityName string    `gorm:"column:entity_name" json:"entityName"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AdminActivity) TableName() string {
	return "admin_activities"
}

type Course struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`
	CourseName    string    `gorm:"column:course_name" json:"courseName"`
	BatchID       string    `gorm:"column:batch_id" json:"batchId"`
	Category      string    `gorm:"column:category" json:"category"`
	TeacherID     *int64    `gorm:"column:teacher_id" json:"teacher"`
	FinalPrice    float64   `gorm:"column:final_price" json:"finalPrice"`
	CourseStatus  string    `gorm:"column:course_status" json:"courseStatus"`
	MeetingLink   string    `gorm:"column:meeting_link" json:"meetingLink"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Course) TableName() string {
	return "courses"
}

type Subject struct {
	ID          int64     `gorm:"primaryKey;column:id"`
	SubjectName string    `gorm:"column:subject_name"`
	MeetingLink string    `gorm:"column:meeting_link"`
}

func (Subject) TableName() string {
	return "subjects"
}

type ClassSchedule struct {
	ID               int64     `gorm:"primaryKey;column:id"`
	CourseID         int64     `gorm:"column:course_id"`
	SubjectID        *int64    `gorm:"column:subject_id"`
	BatchID          string    `gorm:"column:batch_id"`
	TeacherID        int64     `gorm:"column:teacher_id"`
	TopicName        string    `gorm:"column:topic_name"`
	ClassDate        time.Time `gorm:"column:class_date;type:date"`
	StartTime        string    `gorm:"column:start_time"` // Stored as "HH:MM:SS"
	EndTime          string    `gorm:"column:end_time"`   // Stored as "HH:MM:SS"
	ClassStatus      string    `gorm:"column:class_status"`
	RescheduleReason *string   `gorm:"column:reschedule_reason"`
	ClassNotesURL    *string   `gorm:"column:class_notes_url"`
	RecordedClassURL *string   `gorm:"column:recorded_class_url"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ClassSchedule) TableName() string {
	return "class_schedules"
}

type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	StudentID int64     `gorm:"column:student_id"`
	CourseID  int64     `gorm:"column:course_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

type Note struct {
	ID       int64   `gorm:"primaryKey;column:id"`
	Title    string  `gorm:"column:title"`
	Price    float64 `gorm:"column:price"`
	Category string  `gorm:"column:category"`
	NoteType string  `gorm:"column:note_type"` // e.g. public, batch
	IsFree   bool    `gorm:"column:is_free"`
}

func (Note) TableName() string {
	return "notes"
}

type NoteAccess struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	NoteID    int64     `gorm:"column:note_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (NoteAccess) TableName() string {
	return "note_accesses"
}

type TestSeries struct {
	ID       int64   `gorm:"primaryKey;column:id"`
	Title    string  `gorm:"column:title"`
	Price    float64 `gorm:"column:price"`
	Category string  `gorm:"column:category"`
	IsFree   bool    `gorm:"column:is_free"`
	CourseID *int64  `gorm:"column:course_id"`
}

func (TestSeries) TableName() string {
	return "test_series"
}

type TestSeriesAccess struct {
	ID           int64     `gorm:"primaryKey;column:id"`
	TestSeriesID int64     `gorm:"column:test_series_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (TestSeriesAccess) TableName() string {
	return "test_series_accesses"
}

type SiteStatus struct {
	ID                 int64     `gorm:"primaryKey;column:id"`
	ActiveUser         int       `gorm:"column:active_user"`
	LiveClassesPerWeek int       `gorm:"column:live_classes_per_week"`
	LiveBatches        int       `gorm:"column:live_batches"`
	SmartNotes         int       `gorm:"column:smart_notes"`
	Recordings         int       `gorm:"column:recordings"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (SiteStatus) TableName() string {
	return "site_status"
}

type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`
	RecipientID      int64     `gorm:"column:recipient_id;index;not null" json:"recipientId"`
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"`
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`
	NotificationType string    `gorm:"column:notification_type;type:varchar(50)" json:"notificationType"`
	Message          string    `gorm:"column:message;type:text" json:"message"`
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone" json:"createdAt"`
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

// Structs for Sales Analysis Report
type CourseSales struct {
	ID         int64
	CourseName string
	BatchID    string
	Price      float64
	SalesCount int64
}

type NoteSales struct {
	ID         int64
	Title      string
	Price      float64
	SalesCount int64
}

type TestSeriesSales struct {
	ID         int64
	Title      string
	Price      float64
	SalesCount int64
}

type AdminRepository interface {
	GetDashboardStats(ctx context.Context) (*AdminDashboard, error)
	RefreshDashboardStats(ctx context.Context) (*AdminDashboard, error)
	GetAdminByID(ctx context.Context, id int64) (*Admin, error)
	GetAdminByEmail(ctx context.Context, email string) (*Admin, error)
	CreateNotification(ctx context.Context, recipientID int64, recipientRole, notifType, message string) error
	
	// Activities
	GetActivities(ctx context.Context, limit int) ([]AdminActivity, error)
	LogActivity(ctx context.Context, adminID int64, action, entityType, entityName string) error

	// Teachers
	ListTeachers(ctx context.Context, query, category string) ([]Teacher, error)
	GetTeacherByID(ctx context.Context, id int64) (*Teacher, error)
	CreateTeacher(ctx context.Context, teacher *Teacher) error
	UpdateTeacher(ctx context.Context, teacher *Teacher) error
	DeleteTeacher(ctx context.Context, id int64) error
	
	// Students
	ListStudents(ctx context.Context, query, category string) ([]Student, error)

	// Sales Analysis Queries
	GetCoursesSales(ctx context.Context, category string, start, end time.Time) ([]CourseSales, error)
	GetNotesSales(ctx context.Context, category string, start, end time.Time) ([]NoteSales, error)
	GetTestSeriesSales(ctx context.Context, category string, start, end time.Time) ([]TestSeriesSales, error)

	// Categories
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategoryByID(ctx context.Context, id int64) (*Category, error)
	GetCategoryByName(ctx context.Context, name string) (*Category, error)
	CreateCategory(ctx context.Context, category *Category) error
	UpdateCategory(ctx context.Context, category *Category) error
	DeleteCategory(ctx context.Context, id int64) error
	CascadeCategoryUpdate(ctx context.Context, oldName, newName string) error
	CascadeCategoryDelete(ctx context.Context, name string) error

	// Course & schedules updates for teacher assignment
	AssignTeacherToCourses(ctx context.Context, teacherID int64, courseNames []string) error
	UnassignTeacherFromOldCourses(ctx context.Context, teacherID int64, courseNames []string) error
	GetCourseByName(ctx context.Context, name string) (*Course, error)
	GetCourseByBatchID(ctx context.Context, batchID string) (*Course, error)

	// Schedules for teacher tasks
	DeleteClassSchedulesBySignature(ctx context.Context, teacherID int64, batchID, topic string, date time.Time, startTime string) error
	UpsertClassSchedule(ctx context.Context, schedule *ClassSchedule, topic string, subjectObj *Subject) error

	// Platform Stats
	GetSiteStatus(ctx context.Context) (*SiteStatus, error)
	UpdateSiteStatus(ctx context.Context, stats *SiteStatus) error
	GetTotalUsersCount(ctx context.Context) (int64, error)
	GetWeeklyLiveClassesCount(ctx context.Context, start, end time.Time) (int64, error)
	GetActiveBatchesCount(ctx context.Context) (int64, error)
	GetTotalNotesCount(ctx context.Context) (int64, error)
	GetRecordingsCount(ctx context.Context) (int64, error)
}

type AdminUsecase interface {
	GetOverview(ctx context.Context) (map[string]interface{}, error)
	GetActivities(ctx context.Context) ([]map[string]interface{}, error)
	ListTeachers(ctx context.Context, query, category string) (map[string]interface{}, error)
	CreateTeacher(ctx context.Context, teacher *Teacher) (map[string]interface{}, error)
	UpdateTeacher(ctx context.Context, teacherID int64, updates map[string]interface{}) (map[string]interface{}, error)
	DeleteTeacher(ctx context.Context, teacherID int64, complete bool, courseName string, adminID int64) error
	ListStudents(ctx context.Context, query, category string) ([]map[string]interface{}, error)
	GetSalesAnalysis(ctx context.Context, monthStr, category string) (map[string]interface{}, error)
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategory(ctx context.Context, id int64) (*Category, error)
	CreateCategory(ctx context.Context, name string) (*Category, error)
	UpdateCategory(ctx context.Context, id int64, name string) (*Category, error)
	DeleteCategory(ctx context.Context, id int64) error
	GetPlatformStats(ctx context.Context) (map[string]interface{}, error)
	GetPlatformCategories(ctx context.Context) ([]string, error)
}
