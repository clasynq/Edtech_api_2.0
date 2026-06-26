package domain

import (
	"context"
	"encoding/json"
	"time"
)

// User represents the users table
type User struct {
	ID             int64      `gorm:"primaryKey;column:id"`
	FullName       string     `gorm:"column:full_name;type:varchar(255);not null"`
	Username       string     `gorm:"column:username;type:varchar(30);unique;not null"`
	ContactNumber  string     `gorm:"column:contact_number;type:varchar(32);unique;not null"`
	Email          string     `gorm:"column:email;type:varchar(255);unique;not null"`
	Password       string     `gorm:"column:password;type:varchar(128);not null"`
	AvatarURL      string     `gorm:"column:avatar_url;type:text"`
	Headline       string     `gorm:"column:headline;type:varchar(255)"`
	Bio            string     `gorm:"column:bio;type:text"`
	Skills         string     `gorm:"column:skills;type:text"`
	DateOfBirth    *time.Time `gorm:"column:date_of_birth;type:date"`
	Website        string     `gorm:"column:website;type:varchar(500)"`
	Github         string     `gorm:"column:github;type:varchar(500)"`
	Linkedin       string     `gorm:"column:linkedin;type:varchar(500)"`
	Twitter        string     `gorm:"column:twitter;type:varchar(500)"`
	EmailAlerts    bool       `gorm:"column:email_alerts;type:boolean;default:true"`
	DirectMessages bool       `gorm:"column:direct_messages;type:boolean;default:true"`
	FeedUpdates    bool       `gorm:"column:feed_updates;type:boolean;default:false"`
	SecurityAlerts bool       `gorm:"column:security_alerts;type:boolean;default:true"`
	ReferralCode   string     `gorm:"column:referral_code;type:varchar(50);unique"`
	CoinsBalance   int        `gorm:"column:coins_balance;type:integer;default:0"`
	RegistrationIP *string    `gorm:"column:registration_ip;type:inet"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (User) TableName() string {
	return "users"
}

// Student represents the students table
type Student struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (Student) TableName() string {
	return "students"
}

// Course represents the courses table
type Course struct {
	ID                 int64     `gorm:"primaryKey;column:id"`
	CourseName         string    `gorm:"column:course_name;type:varchar(255);not null"`
	BatchID            string    `gorm:"column:batch_id;type:varchar(50);unique;not null"`
	Category           string    `gorm:"column:category;type:varchar(100);not null"`
	Language           string    `gorm:"column:language;type:varchar(50);not null"`
	Description        string    `gorm:"column:description;type:text;not null"`
	TeacherID          *int64    `gorm:"column:teacher_id"`
	OriginalPrice      float64   `gorm:"column:original_price;type:numeric(10,2);not null"`
	FinalPrice         float64   `gorm:"column:final_price;type:numeric(10,2);not null"`
	DiscountPercentage int       `gorm:"column:discount_percentage;type:integer;not null"`
	CourseStatus       string    `gorm:"column:course_status;type:varchar(50);not null"`
	StartDate          time.Time `gorm:"column:start_date;type:date;not null"`
	EndDate            time.Time `gorm:"column:end_date;type:date;not null"`
	AccessDuration     string    `gorm:"column:access_duration;type:varchar(100);not null"`
	BannerURL          string    `gorm:"column:banner_url;type:text"`
	MeetingLink        string    `gorm:"column:meeting_link;type:text"`
	TotalStudents      int       `gorm:"column:total_students;type:integer;default:0"`
	IsFeatured         bool      `gorm:"column:is_featured;type:boolean;default:false"`
	Visibility         string    `gorm:"column:visibility;type:varchar(20);default:'public'"`
	CreatedAt          time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
	Slug               string    `gorm:"column:slug;type:varchar(100)"`
}

func (Course) TableName() string {
	return "courses"
}

// Note represents the notes table
type Note struct {
	ID          int64     `gorm:"primaryKey;column:id"`
	Title       string    `gorm:"column:title;type:varchar(255);not null"`
	Description string    `gorm:"column:description;type:text;not null"`
	NoteType    string    `gorm:"column:note_type;type:varchar(50);not null"`
	IsFree      bool      `gorm:"column:is_free;type:boolean;not null"`
	Price       float64   `gorm:"column:price;type:numeric(10,2);not null"`
	BatchID     string    `gorm:"column:batch_id;type:varchar(50);not null"`
	FileURL     string    `gorm:"column:file_url;type:text;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
	CourseID    *int64    `gorm:"column:course_id"`
	HasSvgs     bool      `gorm:"column:has_svgs;type:boolean;default:false"`
	PageCount   int       `gorm:"column:page_count;type:integer;default:0"`
	Category    string    `gorm:"column:category;type:varchar(100);not null"`
	Slug        string    `gorm:"column:slug;type:varchar(100)"`
	Subject     string    `gorm:"column:subject;type:varchar(255)"`
	Topic       string    `gorm:"column:topic;type:varchar(255)"`
	PrerequisiteURL string `gorm:"column:prerequisite_url;type:text"`
}

func (Note) TableName() string {
	return "notes"
}

// TestSeries represents the test_series table
type TestSeries struct {
	ID          int64      `gorm:"primaryKey;column:id"`
	Title       string     `gorm:"column:title;type:varchar(255);not null"`
	Description string     `gorm:"column:description;type:text;not null"`
	BannerURL   string     `gorm:"column:banner_url;type:text;not null"`
	Category    string     `gorm:"column:category;type:varchar(100);not null"`
	BatchID     *string    `gorm:"column:batch_id;type:varchar(50)"`
	IsPublished bool       `gorm:"column:is_published;type:boolean;default:false"`
	StartDate   *time.Time `gorm:"column:start_date;type:date"`
	EndDate     *time.Time `gorm:"column:end_date;type:date"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
	CourseID    *int64     `gorm:"column:course_id"`
	IsFree      bool       `gorm:"column:is_free;type:boolean;default:false"`
	Price       float64    `gorm:"column:price;type:numeric(10,2);not null"`
	Slug        string     `gorm:"column:slug;type:varchar(100)"`
}

func (TestSeries) TableName() string {
	return "test_series"
}

// PaymentOrder represents the payment_orders table
type PaymentOrder struct {
	ID                    int64      `gorm:"primaryKey;column:id" json:"id"`
	Amount                float64    `gorm:"column:amount;type:numeric(10,2);not null" json:"amount"`
	RazorpayOrderID       string     `gorm:"column:razorpay_order_id;type:varchar(255);not null" json:"razorpayOrderId"`
	RazorpayPaymentID     *string    `gorm:"column:razorpay_payment_id;type:varchar(255)" json:"razorpayPaymentId"`
	Status                string     `gorm:"column:status;type:varchar(50);not null" json:"status"`
	CreatedAt             time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`
	CourseID              *int64     `gorm:"column:course_id" json:"courseId"`
	UserID                int64      `gorm:"column:user_id;not null" json:"userId"`
	NoteID                *int64     `gorm:"column:note_id" json:"noteId"`
	OrderType             string     `gorm:"column:order_type;type:varchar(50);not null" json:"orderType"`
	ReferrerID            *int64     `gorm:"column:referrer_id" json:"referrerId"`
	CoinsRedeemed         int        `gorm:"column:coins_redeemed;type:integer;not null;default:0" json:"coinsRedeemed"`
	TestSeriesID          *int64     `gorm:"column:test_series_id" json:"testSeriesId"`
	AccountAgeAtOrderDays int        `gorm:"column:account_age_at_order_days;type:integer;not null" json:"accountAgeAtOrderDays"`
	DeviceFingerprint     *string    `gorm:"column:device_fingerprint;type:varchar(255)" json:"deviceFingerprint"`
	FailureReason         *string    `gorm:"column:failure_reason;type:text" json:"failureReason"`
	IPAddress             *string    `gorm:"column:ip_address;type:varchar(255)" json:"ipAddress"`
	PaymentMethod         *string    `gorm:"column:payment_method;type:varchar(255)" json:"paymentMethod"`
	Refunded              bool       `gorm:"column:refunded;type:boolean;not null;default:false" json:"refunded"`
	RefundedAmount        *float64   `gorm:"column:refunded_amount;type:numeric(10,2)" json:"refundedAmount"`
	RefundedAt            *time.Time `gorm:"column:refunded_at;type:timestamp with time zone" json:"refundedAt"`
	UserAgent             *string    `gorm:"column:user_agent;type:text" json:"userAgent"`
}

func (PaymentOrder) TableName() string {
	return "payment_orders"
}

// ReferralTransaction represents the referral_transactions table
type ReferralTransaction struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`
	Status          string     `gorm:"column:status;type:varchar(50);not null" json:"status"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	CreditedAt      *time.Time `gorm:"column:credited_at;type:timestamp with time zone" json:"creditedAt"`
	PaymentOrderID  int64      `gorm:"column:payment_order_id;not null" json:"paymentOrderId"`
	ReferredBuyerID int64      `gorm:"column:referred_buyer_id;not null" json:"referredBuyerId"`
	ReferrerID      int64      `gorm:"column:referrer_id;not null" json:"referrerId"`
}

func (ReferralTransaction) TableName() string {
	return "referral_transactions"
}

// WebhookEvent represents the webhook_events table
type WebhookEvent struct {
	ID          int64     `gorm:"primaryKey;column:id" json:"id"`
	EventID     string    `gorm:"column:event_id;type:varchar(255);not null;unique" json:"eventId"`
	ProcessedAt time.Time `gorm:"column:processed_at;type:timestamp with time zone;autoCreateTime" json:"processedAt"`
}

func (WebhookEvent) TableName() string {
	return "webhook_events"
}

// PaymentAuditLog represents the payment_audit_logs table
type PaymentAuditLog struct {
	ID             int64           `gorm:"primaryKey;column:id" json:"id"`
	EventType      string          `gorm:"column:event_type;type:varchar(255);not null" json:"eventType"`
	Payload        json.RawMessage `gorm:"column:payload;type:jsonb;not null" json:"payload"`
	CreatedAt      time.Time       `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	PaymentOrderID *int64          `gorm:"column:payment_order_id" json:"paymentOrderId"`
	UserID         *int64          `gorm:"column:user_id" json:"userId"`
}

func (PaymentAuditLog) TableName() string {
	return "payment_audit_logs"
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

type EnrollmentRepository interface {
	BeginTx(ctx context.Context) (EnrollmentRepository, error)
	CommitTx() error
	RollbackTx() error

	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByReferralCode(ctx context.Context, code string) (*User, error)
	GetStudentByUserID(ctx context.Context, userID int64) (*Student, error)
	UpdateUserCoins(ctx context.Context, userID int64, change int) error

	GetCourseByID(ctx context.Context, id int64) (*Course, error)
	GetNoteByID(ctx context.Context, id int64) (*Note, error)
	GetTestSeriesByID(ctx context.Context, id int64) (*TestSeries, error)

	GetEnrollment(ctx context.Context, studentID, courseID int64) (*Enrollment, error)
	CreateEnrollment(ctx context.Context, enrollment *Enrollment) error
	DeleteEnrollment(ctx context.Context, studentID, courseID int64) error

	GetNoteAccess(ctx context.Context, studentID, noteID int64) (*NoteAccess, error)
	CreateNoteAccess(ctx context.Context, access *NoteAccess) error
	DeleteNoteAccess(ctx context.Context, studentID, noteID int64) error

	GetTestSeriesAccess(ctx context.Context, studentID, testSeriesID int64) (*TestSeriesAccess, error)
	CreateTestSeriesAccess(ctx context.Context, access *TestSeriesAccess) error
	DeleteTestSeriesAccess(ctx context.Context, studentID, testSeriesID int64) error

	GetPaymentOrderByID(ctx context.Context, id int64) (*PaymentOrder, error)
	GetPaymentOrderByRazorpayID(ctx context.Context, razorpayOrderID string) (*PaymentOrder, error)
	GetPaymentOrderByPaymentID(ctx context.Context, paymentID string) (*PaymentOrder, error)
	CreatePaymentOrder(ctx context.Context, order *PaymentOrder) error
	UpdatePaymentOrder(ctx context.Context, order *PaymentOrder) error
	HasUserCompletedOrderForReferrer(ctx context.Context, buyerID, referrerID int64) (bool, error)

	GetReferralTransactionByID(ctx context.Context, id int64) (*ReferralTransaction, error)
	GetReferralTransactionByOrderID(ctx context.Context, orderID int64) (*ReferralTransaction, error)
	CreateReferralTransaction(ctx context.Context, tx *ReferralTransaction) error
	UpdateReferralTransaction(ctx context.Context, tx *ReferralTransaction) error
	GetPendingReferralTransactions(ctx context.Context) ([]ReferralTransaction, error)
	CountCompletedReferralsForReferrer(ctx context.Context, referrerID int64) (int64, error)

	GetWebhookEventByID(ctx context.Context, eventID string) (*WebhookEvent, error)
	CreateWebhookEvent(ctx context.Context, event *WebhookEvent) error

	CreateAuditLog(ctx context.Context, log *PaymentAuditLog) error
	GetMyEnrollments(ctx context.Context, studentID int64, category string) ([]map[string]interface{}, error)
}

type EnrollmentUsecase interface {
	ValidateReferral(ctx context.Context, buyerID int64, buyerIP, referralCode string, courseID int64) (map[string]interface{}, error)
	CreateOrder(ctx context.Context, buyerID int64, buyerIP, userAgent string, req map[string]interface{}) (map[string]interface{}, error)
	VerifyPayment(ctx context.Context, buyerID int64, req map[string]interface{}) (map[string]interface{}, error)
	HandleWebhook(ctx context.Context, rawBody []byte, signature string) error
	RefundOrder(ctx context.Context, orderID int64) error
	ProcessPendingReferrals(ctx context.Context) error
	GetMyEnrollments(ctx context.Context, userID int64, category string) ([]map[string]interface{}, error)
}
