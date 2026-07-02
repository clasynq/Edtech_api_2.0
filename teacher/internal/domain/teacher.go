package domain

import (
	"context"
	"encoding/json"
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

type Course struct {
	ID              int64           `gorm:"primaryKey;column:id" json:"id"`
	CourseName      string          `gorm:"column:course_name" json:"courseName"`
	BatchID         string          `gorm:"column:batch_id" json:"batchId"`
	Category        string          `gorm:"column:category" json:"category"`
	TeacherID       *int64          `gorm:"column:teacher_id" json:"teacher"`
	FinalPrice      float64         `gorm:"column:final_price" json:"finalPrice"`
	CourseStatus    string          `gorm:"column:course_status" json:"courseStatus"`
	MeetingLink     string          `gorm:"column:meeting_link" json:"meetingLink"`
	TeacherSubjects json.RawMessage `gorm:"column:teacher_subjects;type:jsonb" json:"teacherSubjects"`
	CreatedAt       time.Time       `gorm:"column:created_at" json:"createdAt"`
	
	// Join relationship for many-to-many courses_teachers
	Teachers      []Teacher `gorm:"many2many:courses_teachers;foreignKey:ID;joinForeignKey:course_id;References:ID;joinReferences:teacher_id" json:"-"`
}

func (Course) TableName() string {
	return "courses"
}

type Subject struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	SubjectName string    `gorm:"column:subject_name" json:"subjectName"`
	MeetingLink string    `gorm:"column:meeting_link" json:"meetingLink"`
}

func (Subject) TableName() string {
	return "subjects"
}

type ClassSchedule struct {
	ID               int64      `gorm:"primaryKey;column:id" json:"id"`
	CourseID         int64      `gorm:"column:course_id" json:"course"`
	SubjectID        *int64     `gorm:"column:subject_id" json:"subject"`
	BatchID          string     `gorm:"column:batch_id" json:"batchId"`
	TeacherID        int64      `gorm:"column:teacher_id" json:"teacher"`
	TopicName        string     `gorm:"column:topic_name" json:"topicName"`
	ClassDate        time.Time  `gorm:"column:class_date;type:date" json:"classDate"`
	StartTime        string     `gorm:"column:start_time" json:"startTime"`
	EndTime          string     `gorm:"column:end_time" json:"endTime"`
	ClassStatus      string     `gorm:"column:class_status" json:"classStatus"`
	RescheduleReason *string    `gorm:"column:reschedule_reason" json:"rescheduleReason"`
	ClassNotesURL    *string    `gorm:"column:class_notes_url" json:"classNotesUrl"`
	RecordedClassURL *string    `gorm:"column:recorded_class_url" json:"recordedClassUrl"`
	Description      *string    `gorm:"column:description" json:"description"`
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	
	// Preloaded relationships
	Course           Course     `gorm:"foreignKey:CourseID" json:"-"`
	Teacher          Teacher    `gorm:"foreignKey:TeacherID" json:"-"`
	Subject          *Subject   `gorm:"foreignKey:SubjectID" json:"-"`
}

func (ClassSchedule) TableName() string {
	return "class_schedules"
}

type Enrollment struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	StudentID int64     `gorm:"column:student_id" json:"studentId"`
	CourseID  int64     `gorm:"column:course_id" json:"courseId"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	
	Student   Student   `gorm:"foreignKey:StudentID" json:"student"`
	Course    Course    `gorm:"foreignKey:CourseID" json:"course"`
}

func (Enrollment) TableName() string {
	return "enrollments"
}

type TeacherActivity struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	TeacherID  int64     `gorm:"column:teacher_id" json:"teacherId"`
	Action     string    `gorm:"column:action" json:"action"`
	EntityType string    `gorm:"column:entity_type" json:"entityType"`
	EntityName string    `gorm:"column:entity_name" json:"entityName"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (TeacherActivity) TableName() string {
	return "teacher_activities"
}

type TeacherRepository interface {
	GetTeacherByID(ctx context.Context, id int64) (*Teacher, error)
	GetCoursesByTeacher(ctx context.Context, teacherID int64, category string) ([]Course, error)
	GetEnrollmentsByCourses(ctx context.Context, courseIDs []int64) ([]Enrollment, error)
	GetClassSchedulesByTeacher(ctx context.Context, teacherID int64, category string) ([]ClassSchedule, error)
	GetTeacherActivities(ctx context.Context, teacherID int64, limit int) ([]TeacherActivity, error)
	LogTeacherActivity(ctx context.Context, teacherID int64, action, entityType, entityName string) error
	
	// Student Assignment
	GetCourseByID(ctx context.Context, courseID int64) (*Course, error)
	GetUserByID(ctx context.Context, userID int64) (*User, error)
	GetOrCreateStudentProfile(ctx context.Context, user *User) (*Student, error)
	GetOrCreateEnrollment(ctx context.Context, studentID, courseID int64) (*Enrollment, bool, error)
	
	// Class Scheduling
	CreateClassSchedule(ctx context.Context, schedule *ClassSchedule) error
	GetClassScheduleByID(ctx context.Context, id int64) (*ClassSchedule, error)
	UpdateClassSchedule(ctx context.Context, schedule *ClassSchedule) error
	DeleteClassSchedule(ctx context.Context, id int64) error
	
	// Subjects for meeting link resolution
	GetSubjectsForCourse(ctx context.Context, courseID int64) ([]Subject, error)
	GetSubjectByID(ctx context.Context, subjectID int64) (*Subject, error)
	
	// All Users (to return in list of allStudents in overview)
	GetAllUsers(ctx context.Context) ([]User, error)

	// Note uploads
	CreateNote(ctx context.Context, note *Note) error
	GetCourseByBatchID(ctx context.Context, batchID string) (*Course, error)

	// Notifications
	CreateNotification(ctx context.Context, notif *UserNotification) error
}

type TeacherUsecase interface {
	GetOverview(ctx context.Context, teacherID int64, category string) (map[string]interface{}, error)
	AssignStudent(ctx context.Context, teacherID, studentUserID, courseID int64) (map[string]interface{}, error)
	GetBatches(ctx context.Context, teacherID int64, category string) ([]map[string]interface{}, error)
	GetClasses(ctx context.Context, teacherID int64, category string) ([]map[string]interface{}, error)
	ScheduleClass(ctx context.Context, teacherID int64, scheduleData map[string]interface{}) (map[string]interface{}, error)
	UpdateClass(ctx context.Context, teacherID, classID int64, updates map[string]interface{}) (map[string]interface{}, error)
	DeleteClass(ctx context.Context, teacherID, classID int64) error
	UploadNote(ctx context.Context, teacherID int64, batchID, title, fileURL, recordedClassURL, subject, topic, prerequisiteURL, description string) (map[string]interface{}, error)
	GetCategories(ctx context.Context, teacherID int64) ([]string, error)
	SendNotice(ctx context.Context, teacherID int64, batchID, message string) (map[string]interface{}, error)
}

type Note struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	Title       string    `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description string    `gorm:"column:description;type:text;not null" json:"description"`
	NoteType    string    `gorm:"column:note_type;type:varchar(50);not null" json:"noteType"`
	IsFree      bool      `gorm:"column:is_free;type:boolean;not null" json:"isFree"`
	Price       float64   `gorm:"column:price;type:numeric(10,2);not null" json:"price"`
	BatchID     string    `gorm:"column:batch_id;type:varchar(50);not null" json:"batchId"`
	FileURL          string    `gorm:"column:file_url;type:text;not null" json:"fileUrl"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	CourseID         *int64    `gorm:"column:course_id" json:"courseId"`
	HasSvgs          bool      `gorm:"column:has_svgs;type:boolean;default:false" json:"hasSvgs"`
	PageCount        int       `gorm:"column:page_count;type:integer;default:0" json:"pageCount"`
	Category         string    `gorm:"column:category;type:varchar(100);not null" json:"category"`
	RecordedClassURL string    `gorm:"column:recorded_class_url;type:text" json:"recordedClassUrl"`
	Subject          string    `gorm:"column:subject;type:varchar(255)" json:"subject"`
	Topic            string    `gorm:"column:topic;type:varchar(255)" json:"topic"`
	PrerequisiteURL  string    `gorm:"column:prerequisite_url;type:text" json:"prerequisiteUrl"`
}

func (Note) TableName() string {
	return "notes"
}

type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	RecipientID      int64     `gorm:"column:recipient_id;not null" json:"recipientId"`
	SenderID         *int64    `gorm:"column:sender_id" json:"senderId"`
	IsRead           bool      `gorm:"column:is_read;default:false" json:"isRead"`
	NotificationType string    `gorm:"column:notification_type;type:varchar(50)" json:"notificationType"`
	Message          string    `gorm:"column:message;type:text" json:"message"`
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50)" json:"recipientRole"`
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

