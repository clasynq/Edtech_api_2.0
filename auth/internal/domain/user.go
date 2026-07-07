package domain

import (
	"context"
	"time"
)

// User represents the root core profile entity mapping to the "users" table.
// This is shared across students, teachers, and admins for primary account details.
type User struct {
	ID             int64      `gorm:"primaryKey;column:id" json:"id"`                                                                      // Unique user identifier (primary key)
	FullName       string     `gorm:"column:full_name;type:varchar(255);not null" json:"fullName"`                                         // User's first and last name
	Username       string     `gorm:"column:username;type:varchar(30);unique;not null" json:"username"`                                    // Unique username handles
	ContactNumber  string     `gorm:"column:contact_number;type:varchar(32);unique;not null" json:"contactNumber"`                         // Verified contact number
	Email          string     `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`                                         // Primary email address
	Password       string     `gorm:"column:password;type:varchar(128);not null" json:"-"`                                                 // Django-compatible PBKDF2 hashed password
	AvatarURL      string     `gorm:"column:avatar_url;type:text" json:"avatarUrl"`                                                        // Cloudinary hosted profile image url
	Headline       string     `gorm:"column:headline;type:varchar(255);default:'Learning Path Enthusiast | ClaSynqian'" json:"headline"`   // High-level bio tagline
	Bio            string     `gorm:"column:bio;type:text" json:"bio"`                                                                     // Detailed summary description
	Skills         string     `gorm:"column:skills;type:text" json:"skills"`                                                               // List of learning categories
	DateOfBirth    *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`                                                   // Birthday field (for automation wishes)
	Website        string     `gorm:"column:website;type:varchar(500)" json:"website"`                                                     // Personal homepage
	Github         string     `gorm:"column:github;type:varchar(500)" json:"github"`                                                       // GitHub profile link
	Linkedin       string     `gorm:"column:linkedin;type:varchar(500)" json:"linkedin"`                                                   // LinkedIn profile link
	Twitter        string     `gorm:"column:twitter;type:varchar(500)" json:"twitter"`                                                     // Twitter/X handle link
	EmailAlerts    bool       `gorm:"column:email_alerts;type:boolean;default:true" json:"emailAlerts"`                                    // Email notification preference flag
	DirectMessages bool       `gorm:"column:direct_messages;type:boolean;default:true" json:"directMessages"`                              // Direct messaging allowance toggle
	FeedUpdates    bool       `gorm:"column:feed_updates;type:boolean;default:false" json:"feedUpdates"`                                   // Feed update newsletter toggle
	SecurityAlerts bool       `gorm:"column:security_alerts;type:boolean;default:true" json:"securityAlerts"`                              // Urgent account security alert toggle
	ReferralCode   string     `gorm:"column:referral_code;type:varchar(50);unique" json:"referralCode"`                                    // Unique referral code code for discounts/rewards
	CoinsBalance   int        `gorm:"column:coins_balance;type:integer;default:0" json:"coinsBalance"`                                     // virtual currency ledger balance
	RegistrationIP *string    `gorm:"column:registration_ip;type:inet" json:"registrationIp"`                                              // Track client registration IP
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`                     // Record generation timestamp
}

// TableName matches the struct specifically to the existing PostgreSQL table "users".
func (User) TableName() string {
	return "users"
}

// Student represents sub-domain account properties for pupils, linking directly to a User.
type Student struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`                              // Student record primary key
	UserID    int64     `gorm:"column:user_id;unique;not null" json:"userId"`                // Foreign key linking to "users"
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`   // Embedded user detail object
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"` // Join date
}

// TableName matches the struct specifically to the existing PostgreSQL table "students".
func (Student) TableName() string {
	return "students"
}

// Admin represents an administrative user profile mapping to the "admin" table.
type Admin struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`                              // Admin primary key
	Email     string    `gorm:"column:email;type:varchar(255);unique;not null" json:"email"` // Admin contact/login email
	Password  string    `gorm:"column:password;type:varchar(128);not null" json:"-"`         // Password hash (hidden in json marshalling)
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

// TableName matches the struct specifically to the existing PostgreSQL table "admin".
func (Admin) TableName() string {
	return "admin"
}

// Teacher represents educator domains and assigned settings.
type Teacher struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`                                                      // Teacher primary key
	Email           string     `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`                         // Login email credentials
	Password        string     `gorm:"column:password;type:varchar(128);not null" json:"-"`                                 // Secret hashed password
	Name            string     `gorm:"column:name;type:varchar(255);not null" json:"name"`                                   // Professional full name
	Specialization  string     `gorm:"column:specialization;type:varchar(255);not null" json:"specialization"`               // Primary subject matter focus
	AssignedCourses string     `gorm:"column:Assigned_courses;type:jsonb;default:'[]'" json:"assignedCourses"`               // JSON array of course codes
	Tasks           string     `gorm:"column:tasks;type:jsonb;default:'[]'" json:"tasks"`                                   // JSON list of pending tasks
	PhotoURL        string     `gorm:"column:photo_url;type:text" json:"photoUrl"`                                           // Profile photo CDN URL
	Category        string     `gorm:"column:category;type:varchar(100);default:'CSE(Graduation)'" json:"category"`         // Subject tier categorization
	DateOfBirth     *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`                                   // Birthday field
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`     // Record create timestamp
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`     // Last profile update timestamp
}

// TableName matches the struct specifically to the existing PostgreSQL table "teachers".
func (Teacher) TableName() string {
	return "teachers"
}

// PendingRegistration keeps track of signups awaiting email/SMS verification via OTP.
type PendingRegistration struct {
	Email         string    `gorm:"primaryKey;column:email;type:varchar(255)" json:"email"`                                 // Target email address
	FullName      string    `gorm:"column:full_name;type:varchar(255);not null" json:"fullName"`                             // Profile name to be saved on verification
	Username      string    `gorm:"column:username;type:varchar(30);unique;not null" json:"username"`                        // Desired handle
	ContactNumber string    `gorm:"column:contact_number;type:varchar(32);not null" json:"contactNumber"`                    // User phone number
	PasswordHash  string    `gorm:"column:password_hash;type:varchar(128);not null" json:"-"`                                // Staged password hash
	CodeHash      string    `gorm:"column:code_hash;type:varchar(128);not null" json:"-"`                                    // SHA256 hashed 6-digit OTP code
	CodeExpiresAt time.Time `gorm:"column:code_expires_at;type:timestamp with time zone" json:"codeExpiresAt"`                // Expiration window limit
	Attempts      int       `gorm:"column:attempts;type:smallint;default:0" json:"attempts"`                                 // OTP try limits counter
	ResendCount   int       `gorm:"column:resend_count;type:smallint;default:0" json:"resendCount"`                            // Resend request counts tracker
	LastSentAt    time.Time `gorm:"column:last_sent_at;type:timestamp with time zone" json:"lastSentAt"`                         // Rate limit window tracker
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`            // Sign up initiation time
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`            // Last attempt state update
}

// TableName matches the struct specifically to the PostgreSQL table "pending_registrations".
func (PendingRegistration) TableName() string {
	return "pending_registrations"
}

// PasswordResetOTP holds validation records for users requesting password resets.
type PasswordResetOTP struct {
	Email         string    `gorm:"primaryKey;column:email;type:varchar(255)" json:"email"`                                 // Requesting email address
	CodeHash      string    `gorm:"column:code_hash;type:varchar(128);not null" json:"-"`                                    // Reset token OTP hash
	CodeExpiresAt time.Time `gorm:"column:code_expires_at;type:timestamp with time zone" json:"codeExpiresAt"`                // Expiration timestamp
	Attempts      int       `gorm:"column:attempts;type:smallint;default:0" json:"attempts"`                                 // OTP validation try count
	ResendCount   int       `gorm:"column:resend_count;type:smallint;default:0" json:"resendCount"`                            // Resend try count
	LastSentAt    time.Time `gorm:"column:last_sent_at;type:timestamp with time zone" json:"lastSentAt"`                         // Rate-limiting check timestamp
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`            // Record create timestamp
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`            // Last state update
}

// TableName matches the struct specifically to the PostgreSQL table "password_reset_otps".
func (PasswordResetOTP) TableName() string {
	return "password_reset_otps"
}

// Follow maps social follower-following relationships between User profiles.
type Follow struct {
	ID         int64     `gorm:"primaryKey;column:id"`                                                                                           // Follow action ID
	FollowerID int64     `gorm:"column:follower_id;index:follower_followed_idx,unique;index:follower_idx;not null"`                               // User subscribing
	Follower   User      `gorm:"foreignKey:FollowerID;constraint:OnDelete:CASCADE"`                                                              // Follower link
	FollowedID int64     `gorm:"column:followed_id;index:follower_followed_idx,unique;index:followed_idx;index:followed_created_idx;not null"`   // User being followed
	Followed   User      `gorm:"foreignKey:FollowedID;constraint:OnDelete:CASCADE"`                                                              // Followed link
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`                                                 // Time followed
}

// TableName matches the struct specifically to the PostgreSQL table "user_follows".
func (Follow) TableName() string {
	return "user_follows"
}

// UserNotification holds system event alert configurations.
type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`                                              // Notification ID
	RecipientID      int64     `gorm:"column:recipient_id;index;not null" json:"recipientId"`                       // ID of target recipient
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"` // Recipient login category (e.g. student, teacher)
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`                                      // Optional action producer ID
	Sender           *User     `gorm:"foreignKey:SenderID;constraint:OnDelete:SET NULL" json:"sender,omitempty"`    // Optional sender metadata
	SenderName       string    `gorm:"-" json:"senderName"`                                                         // Dynamic UI helper field (excluded in GORM)
	SenderAvatarUrl  string    `gorm:"-" json:"senderAvatarUrl"`                                                    // Dynamic UI helper field (excluded in GORM)
	NotificationType string    `gorm:"column:notification_type;type:varchar(50);default:'follow'" json:"notificationType"` // Notification category trigger
	Message          string    `gorm:"column:message;type:text" json:"message"`                                     // Body text message
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"`                     // Read receipt status flag
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"` // Log time
}

// TableName matches the struct specifically to the PostgreSQL table "user_notifications".
func (UserNotification) TableName() string {
	return "user_notifications"
}

// UserRepository defines database operation contracts. Implemented in the repository layer.
type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByContact(ctx context.Context, contact string) (*User, error)
	SearchUsers(ctx context.Context, query string) ([]User, error)
	UpdateUser(ctx context.Context, user *User) error

	IsStudent(ctx context.Context, userID int64) (bool, error)
	GetStudentProfile(ctx context.Context, userID int64) (*Student, error)
	GetStudentReferralsCount(ctx context.Context, userID int64) (int64, error)

	GetAdminByEmail(ctx context.Context, email string) (*Admin, error)
	GetTeacherByEmail(ctx context.Context, email string) (*Teacher, error)
	GetTeacherByID(ctx context.Context, id int64) (*Teacher, error)
	GetAdminByID(ctx context.Context, id int64) (*Admin, error)

	GetPendingRegistration(ctx context.Context, email string) (*PendingRegistration, error)
	SavePendingRegistration(ctx context.Context, pending *PendingRegistration) error
	DeletePendingRegistration(ctx context.Context, email string) error
	CreateUserFromPending(ctx context.Context, pending *PendingRegistration) (*User, error)

	GetPasswordResetOTP(ctx context.Context, email string) (*PasswordResetOTP, error)
	SavePasswordResetOTP(ctx context.Context, reset *PasswordResetOTP) error
	DeletePasswordResetOTP(ctx context.Context, email string) error

	GetFollowRelationship(ctx context.Context, followerID, followedID int64) (*Follow, error)
	FollowUser(ctx context.Context, followerID, followedID int64) error
	UnfollowUser(ctx context.Context, followerID, followedID int64) error
	GetFollowersList(ctx context.Context, userID int64) ([]Follow, error)
	GetFollowingList(ctx context.Context, userID int64) ([]Follow, error)

	GetNotifications(ctx context.Context, userID int64, role string) ([]UserNotification, error)
	MarkNotificationsAsRead(ctx context.Context, userID int64, role string) error
	CreateNotification(ctx context.Context, notif *UserNotification) error
	UpdateAdminPassword(ctx context.Context, id int64, newHash string) error
	UpdateTeacherPassword(ctx context.Context, id int64, newHash string) error
}

// UserUsecase defines business logic workflow contracts. Implemented in the usecase layer.
type UserUsecase interface {
	Register(ctx context.Context, fullName, username, email, contact, password, remoteIP string) (map[string]interface{}, error)
	VerifyOTP(ctx context.Context, email, code string) (map[string]interface{}, error)
	ResendOTP(ctx context.Context, email string) (map[string]interface{}, error)
	Login(ctx context.Context, emailOrUsername, password, remoteIP, role string) (map[string]interface{}, error)
	VerifyLogin2FA(ctx context.Context, email, code, role string) (map[string]interface{}, error)
	ForgotPassword(ctx context.Context, email string) (map[string]interface{}, error)
	ResetPassword(ctx context.Context, email, code, newPassword string) (map[string]interface{}, error)
	GetMe(ctx context.Context, userID int64, role string) (map[string]interface{}, error)
	UpdateMe(ctx context.Context, userID int64, updates map[string]interface{}) (map[string]interface{}, error)
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	SearchUsers(ctx context.Context, query string) ([]map[string]interface{}, error)
	ToggleFollowUser(ctx context.Context, followerID, followedID int64) error
	FollowUser(ctx context.Context, followerID, followedID int64) error
	UnfollowUser(ctx context.Context, followerID, followedID int64) error
	GetNotifications(ctx context.Context, userID int64, role string) ([]UserNotification, error)
	MarkNotificationsAsRead(ctx context.Context, userID int64, role string) error
	TokenRefresh(ctx context.Context, refreshToken string) (map[string]string, error)
}
