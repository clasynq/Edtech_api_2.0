package domain

import (
	"context"
	"time"
)

// User represents a basic student profile detail stored in the "users" table.
type User struct {
	ID            int64     `gorm:"primaryKey;column:id" json:"id"`              // Primary key
	FullName      string    `gorm:"column:full_name" json:"fullName"`            // User's full name
	Username      string    `gorm:"column:username" json:"username"`            // User's unique username
	ContactNumber string    `gorm:"column:contact_number" json:"contactNumber"`  // User's contact number
	Email         string    `gorm:"column:email" json:"email"`                  // User's email
	AvatarURL     string    `gorm:"column:avatar_url" json:"avatarUrl"`          // User's avatar URL
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`          // Time created
}

// TableName matches the User struct specifically to the PostgreSQL table "users".
func (User) TableName() string {
	return "users"
}

// Student represents a sub-domain student details struct referencing the primary User.
type Student struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`            // Primary key
	UserID    int64     `gorm:"column:user_id" json:"userId"`              // Foreign key to User
	User      User      `gorm:"foreignKey:UserID" json:"user"`             // Embedded User details
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`        // Student join timestamp
}

// TableName matches the Student struct specifically to the PostgreSQL table "students".
func (Student) TableName() string {
	return "students"
}

// Admin represents an administrative user profile mapping to the "admin" table.
type Admin struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`            // Primary key
	Email     string    `gorm:"column:email;unique;not null" json:"email"` // Admin email
	Password  string    `gorm:"column:password;not null" json:"-"`         // Password hash (hidden in JSON)
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`        // Admin creation timestamp
}

// TableName matches the Admin struct specifically to the PostgreSQL table "admin".
func (Admin) TableName() string {
	return "admin"
}

// Teacher represents educator accounts, specialized subjects, and JSON fields for assigned course lists.
type Teacher struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`                            // Primary key
	Email           string     `gorm:"column:email;unique;not null" json:"email"`                 // Login email
	Password        string     `gorm:"column:password;not null" json:"-"`                         // Password hash (hidden in JSON)
	Name            string     `gorm:"column:name;not null" json:"name"`                           // Teacher's name
	Specialization  string     `gorm:"column:specialization;not null" json:"specialization"`       // Specialized area
	AssignedCourses string     `gorm:"column:Assigned_courses;type:jsonb" json:"assignedCourses"` // Serialized JSON array of course strings
	Tasks           string     `gorm:"column:tasks;type:jsonb" json:"tasks"`                     // Serialized JSON array of tasks strings
	PhotoURL        string     `gorm:"column:photo_url" json:"photoUrl"`                           // Photo URL
	Category        string     `gorm:"column:category" json:"category"`                           // Category classification
	DateOfBirth     *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`           // Birthdate
	CreatedAt       time.Time  `gorm:"column:created_at" json:"createdAt"`                        // Join date
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`                        // Last update timestamp
}

// TableName matches the Teacher struct specifically to the PostgreSQL table "teachers".
func (Teacher) TableName() string {
	return "teachers"
}

// Category represents a course classification category.
type Category struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`            // Primary key
	Name      string    `gorm:"column:name;unique;not null" json:"name"`   // Unique category name (e.g. UPSC, CSE)
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`        // Creation time
}

// TableName matches the Category struct specifically to the PostgreSQL table "categories".
func (Category) TableName() string {
	return "categories"
}

// AdminDashboard holds high level stat counters displayed on the dashboard home.
type AdminDashboard struct {
	ID            int64 `gorm:"primaryKey;column:id"`          // Primary key
	TotalStudents int64 `gorm:"column:total_students"`         // Total registered students count
	TotalTeacher  int64 `gorm:"column:total_teacher"`          // Total active teachers count
	ActiveBatches int64 `gorm:"column:Active_batches"`         // Count of current active course batches
}

// TableName matches the AdminDashboard struct specifically to the PostgreSQL table "admin_Dashboard".
func (AdminDashboard) TableName() string {
	return "admin_Dashboard"
}

// AdminActivity holds logging records for admin audit trails.
type AdminActivity struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`      // Primary key
	AdminID    int64     `gorm:"column:admin_id" json:"adminId"`      // Performing admin ID
	AdminEmail string    `gorm:"-" json:"adminEmail"`                 // Virtual field (resolved at delivery level)
	Action     string    `gorm:"column:action" json:"action"`         // Done action (e.g. Created, Deleted)
	EntityType string    `gorm:"column:entity_type" json:"entityType"` // Targeted category (e.g. Teacher, Course)
	EntityName string    `gorm:"column:entity_name" json:"entityName"` // Target name parameter
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`  // Activity timestamp
}

// TableName matches the AdminActivity struct specifically to the PostgreSQL table "admin_activities".
func (AdminActivity) TableName() string {
	return "admin_activities"
}

// Course maps course records handled in courses microservice domain database.
type Course struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`              // Primary key
	CourseName   string    `gorm:"column:course_name" json:"courseName"`        // Name
	BatchID      string    `gorm:"column:batch_id" json:"batchId"`              // Unique Batch identifier
	Category     string    `gorm:"column:category" json:"category"`            // Category (e.g. graduation/preparation)
	TeacherID    *int64    `gorm:"column:teacher_id" json:"teacher"`            // Assigned Teacher foreign key
	FinalPrice   float64   `gorm:"column:final_price" json:"finalPrice"`        // Checkout price
	CourseStatus string    `gorm:"column:course_status" json:"courseStatus"`    // Status state
	MeetingLink  string    `gorm:"column:meeting_link" json:"meetingLink"`      // Zoom/Meet URL
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName matches the Course struct specifically to the PostgreSQL table "courses".
func (Course) TableName() string {
	return "courses"
}

// Subject holds class subjects.
type Subject struct {
	ID          int64     `gorm:"primaryKey;column:id"` // Subject ID
	SubjectName string    `gorm:"column:subject_name"`  // Subject label
	MeetingLink string    `gorm:"column:meeting_link"`  // Default link
}

// TableName matches the Subject struct specifically to the PostgreSQL table "subjects".
func (Subject) TableName() string {
	return "subjects"
}

// ClassSchedule represents lecture schedules.
type ClassSchedule struct {
	ID               int64     `gorm:"primaryKey;column:id"`                       // Schedule ID
	CourseID         int64     `gorm:"column:course_id"`                           // Course link
	SubjectID        *int64    `gorm:"column:subject_id"`                          // Subject link
	BatchID          string    `gorm:"column:batch_id"`                            // Batch link
	TeacherID        int64     `gorm:"column:teacher_id"`                          // Assigned instructor ID
	TopicName        string    `gorm:"column:topic_name"`                          // Covered topic
	ClassDate        time.Time `gorm:"column:class_date;type:date"`                // Scheduled date
	StartTime        string    `gorm:"column:start_time"`                          // "HH:MM:SS"
	EndTime          string    `gorm:"column:end_time"`                            // "HH:MM:SS"
	ClassStatus      string    `gorm:"column:class_status"`                        // Status
	RescheduleReason *string   `gorm:"column:reschedule_reason"`                   // Reschedule note
	ClassNotesURL    *string   `gorm:"column:class_notes_url"`                     // Notes URL
	RecordedClassURL *string   `gorm:"column:recorded_class_url"`                  // Recording URL
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`           // Creation log
}

// TableName matches the ClassSchedule struct specifically to the PostgreSQL table "class_schedules".
func (ClassSchedule) TableName() string {
	return "class_schedules"
}

// Enrollment records student course purchases.
type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id"` // Enrollment ID
	StudentID int64     `gorm:"column:student_id"`    // Student reference
	CourseID  int64     `gorm:"column:course_id"`     // Course reference
	CreatedAt time.Time `gorm:"column:created_at"`    // Enrollment timestamp
}

// TableName matches the Enrollment struct specifically to the PostgreSQL table "enrollments".
func (Enrollment) TableName() string {
	return "enrollments"
}

// Note represents smart study PDF notes uploads catalog.
type Note struct {
	ID              int64   `gorm:"primaryKey;column:id"`                   // Note ID
	Title           string  `gorm:"column:title"`                           // Title
	Price           float64 `gorm:"column:price"`                           // Cost
	Category        string  `gorm:"column:category"`                        // Category mapping
	NoteType        string  `gorm:"column:note_type"`                       // "public" or "batch" restricted
	IsFree          bool    `gorm:"column:is_free"`                         // Zero price flag
	Subject         string  `gorm:"column:subject;type:varchar(255)"`       // Note subject
	Topic           string  `gorm:"column:topic;type:varchar(255)"`         // Note topic
	PrerequisiteURL string  `gorm:"column:prerequisite_url;type:text"`      // Cloudinary URL path
}

// TableName matches the Note struct specifically to the PostgreSQL table "notes".
func (Note) TableName() string {
	return "notes"
}

// NoteAccess records purchases or manual grants of notes to students.
type NoteAccess struct {
	ID        int64     `gorm:"primaryKey;column:id"` // ID
	NoteID    int64     `gorm:"column:note_id"`       // Linked note ID
	CreatedAt time.Time `gorm:"column:created_at"`    // Grant time
}

// TableName matches the NoteAccess struct specifically to the PostgreSQL table "note_accesses".
func (NoteAccess) TableName() string {
	return "note_accesses"
}

// TestSeries holds parameters for test series products.
type TestSeries struct {
	ID       int64   `gorm:"primaryKey;column:id"` // Test series ID
	Title    string  `gorm:"column:title"`         // Title
	Price    float64 `gorm:"column:price"`         // Checkout price
	Category string  `gorm:"column:category"`      // Category
	IsFree   bool    `gorm:"column:is_free"`       // Free toggle flag
	CourseID *int64  `gorm:"column:course_id"`     // Optional Course mapping
}

// TableName matches the TestSeries struct specifically to the PostgreSQL table "test_series".
func (TestSeries) TableName() string {
	return "test_series"
}

// TestSeriesAccess records purchases of test series by students.
type TestSeriesAccess struct {
	ID           int64     `gorm:"primaryKey;column:id"`  // ID
	TestSeriesID int64     `gorm:"column:test_series_id"` // Test Series ID
	CreatedAt    time.Time `gorm:"column:created_at"`     // Buy date
}

// TableName matches the TestSeriesAccess struct specifically to the PostgreSQL table "test_series_accesses".
func (TestSeriesAccess) TableName() string {
	return "test_series_accesses"
}

// SiteStatus represents general landing page metrics statistics counters.
type SiteStatus struct {
	ID                 int64     `gorm:"primaryKey;column:id"`
	ActiveUser         int       `gorm:"column:active_user"`            // Active counter
	LiveClassesPerWeek int       `gorm:"column:live_classes_per_week"`  // Weekly classes count
	LiveBatches        int       `gorm:"column:live_batches"`           // Batches count
	SmartNotes         int       `gorm:"column:smart_notes"`            // Notes count
	Recordings         int       `gorm:"column:recordings"`             // Video count
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

// TableName matches the SiteStatus struct specifically to the PostgreSQL table "site_status".
func (SiteStatus) TableName() string {
	return "site_status"
}

// UserNotification holds system event alert configurations.
type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`                     // ID
	RecipientID      int64     `gorm:"column:recipient_id;index;not null" json:"recipientId"` // Target ID
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"` // Recipient category
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`             // Action producer ID
	NotificationType string    `gorm:"column:notification_type;type:varchar(50)" json:"notificationType"` // Type
	Message          string    `gorm:"column:message;type:text" json:"message"`            // Message body
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"` // Read flag
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone" json:"createdAt"` // Alert time
}

// TableName matches the UserNotification struct specifically to the PostgreSQL table "user_notifications".
func (UserNotification) TableName() string {
	return "user_notifications"
}

// CourseSales represents compiled metrics for course purchase analyses.
type CourseSales struct {
	ID         int64
	CourseName string
	BatchID    string
	Price      float64
	SalesCount int64
	Revenue    float64
}

// NoteSales represents compiled metrics for note purchases.
type NoteSales struct {
	ID         int64
	Title      string
	Price      float64
	SalesCount int64
	Revenue    float64
}

// TestSeriesSales represents compiled metrics for test series purchases.
type TestSeriesSales struct {
	ID         int64
	Title      string
	Price      float64
	SalesCount int64
	Revenue    float64
}

// AdminRepository defines database transaction contracts. Implemented in the repository layer.
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
	GetStudentEnrollmentInfo(ctx context.Context, studentIDs []int64) (map[int64][]string, map[int64][]string, map[int64][]string, map[int64][]string, error)

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
	UpsertClassSchedule(ctx context.Context, schedule *ClassSchedule, topic string, subjectName string) error
	GetClassSchedulesByTeacher(ctx context.Context, teacherID int64) ([]*ClassSchedule, error)

	// Platform Stats
	GetSiteStatus(ctx context.Context) (*SiteStatus, error)
	UpdateSiteStatus(ctx context.Context, stats *SiteStatus) error
	GetTotalUsersCount(ctx context.Context) (int64, error)
	GetWeeklyLiveClassesCount(ctx context.Context, start, end time.Time) (int64, error)
	GetActiveBatchesCount(ctx context.Context) (int64, error)
	GetTotalNotesCount(ctx context.Context) (int64, error)
	GetRecordingsCount(ctx context.Context) (int64, error)

	// Careers Repository Methods
	ListActiveJobPositions(ctx context.Context) ([]JobPosition, error)
	ListJobPositions(ctx context.Context) ([]JobPosition, error)
	GetJobPositionByID(ctx context.Context, id int64) (*JobPosition, error)
	CreateJobPosition(ctx context.Context, jp *JobPosition) error
	UpdateJobPosition(ctx context.Context, id int64, updates map[string]interface{}) (*JobPosition, error)
	DeleteJobPosition(ctx context.Context, id int64) error
	ListJobApplications(ctx context.Context) ([]JobApplication, error)
	GetJobApplicationByID(ctx context.Context, id int64) (*JobApplication, error)
	CreateJobApplication(ctx context.Context, app *JobApplication) error
	UpdateJobApplication(ctx context.Context, app *JobApplication) error
}

// AdminUsecase defines core administrative orchestrations. Implemented in the usecase layer.
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

	// Careers Usecase Methods
	ListActiveJobPositions(ctx context.Context) ([]JobPosition, error)
	CreateJobApplication(ctx context.Context, app *JobApplication) error
	ListJobApplications(ctx context.Context) ([]JobApplication, error)
	GetAdminPositions(ctx context.Context) ([]JobPosition, error)
	CreateJobPosition(ctx context.Context, jp *JobPosition) error
	UpdateJobPosition(ctx context.Context, id int64, updates map[string]interface{}) (*JobPosition, error)
	DeleteJobPosition(ctx context.Context, id int64) error
	SendCandidateNotification(ctx context.Context, id int64, emailType, meetingLink, interviewDatetime string, joiningLetterName string, joiningLetterData []byte) error
}

// JobPosition holds careers position details.
type JobPosition struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`              // Job ID
	Title          string    `gorm:"column:title" json:"title"`                  // Job Title
	Department     string    `gorm:"column:department" json:"department"`        // Department
	Location       string    `gorm:"column:location" json:"location"`            // Job Location
	EmploymentType string    `gorm:"column:employment_type" json:"employmentType"` // Type (e.g. full-time)
	Description    string    `gorm:"column:description" json:"description"`      // Details
	Requirements   string    `gorm:"column:requirements" json:"requirements"`    // Requirements
	IsActive       bool      `gorm:"column:is_active;default:true" json:"isActive"` // Active toggle
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// TableName matches the JobPosition struct specifically to the PostgreSQL table "job_positions".
func (JobPosition) TableName() string {
	return "job_positions"
}

// JobApplication holds submitted candidate job applications parameters.
type JobApplication struct {
	ID                int64      `gorm:"primaryKey;column:id" json:"id"`              // Application ID
	PositionID        *int64     `gorm:"column:position_id" json:"position"`          // Job Position ID
	FullName          string     `gorm:"column:full_name" json:"fullName"`            // Candidate Name
	Email             string     `gorm:"column:email" json:"email"`                  // Candidate Email
	Phone             string     `gorm:"column:phone" json:"phone"`                  // Candidate Phone
	Qualification     string     `gorm:"column:qualification" json:"qualification"`  // Qualification
	Branch            string     `gorm:"column:branch" json:"branch"`                  // Branch
	PursuitStatus     string     `gorm:"column:pursuit_status;default:'passout'" json:"pursuitStatus"` // Current status
	PassingYear       int        `gorm:"column:passing_year" json:"passingYear"`      // Passing Year
	CGPA              float64    `gorm:"column:cgpa" json:"cgpa"`                    // Grade CGPA
	ApplyForRole      string     `gorm:"column:apply_for_role" json:"applyForRole"`  // Applying role
	Specialization    string     `gorm:"column:specialization" json:"specialization"` // Specialty
	ExperienceYears   float64    `gorm:"column:experience_years;default:0.0" json:"experienceYears"` // Years exp
	ResumeURL         string     `gorm:"column:resume_url" json:"resumeUrl"`          // PDF Resume path
	PhotoURL          string     `gorm:"column:photo_url" json:"photoUrl"`            // Avatar photo path
	Status            string     `gorm:"column:status;default:'applied'" json:"status"` // Application status
	MeetingLink       string     `gorm:"column:meeting_link;default:''" json:"meetingLink"` // 2FA/interview link
	InterviewDateTime *time.Time `gorm:"column:interview_datetime" json:"interviewDatetime"` // Schedule time
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

// TableName matches the JobApplication struct specifically to the PostgreSQL table "job_applications".
func (JobApplication) TableName() string {
	return "job_applications"
}

