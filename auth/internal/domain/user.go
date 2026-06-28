package domain

import (
	"context"
	"time"
)

type User struct {
	ID             int64      `gorm:"primaryKey;column:id" json:"id"`
	FullName       string     `gorm:"column:full_name;type:varchar(255);not null" json:"fullName"`
	Username       string     `gorm:"column:username;type:varchar(30);unique;not null" json:"username"`
	ContactNumber  string     `gorm:"column:contact_number;type:varchar(32);unique;not null" json:"contactNumber"`
	Email          string     `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`
	Password       string     `gorm:"column:password;type:varchar(128);not null" json:"-"`
	AvatarURL      string     `gorm:"column:avatar_url;type:text" json:"avatarUrl"`
	Headline       string     `gorm:"column:headline;type:varchar(255);default:'Learning Path Enthusiast | ClaSynqian'" json:"headline"`
	Bio            string     `gorm:"column:bio;type:text" json:"bio"`
	Skills         string     `gorm:"column:skills;type:text" json:"skills"`
	DateOfBirth    *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`
	Website        string     `gorm:"column:website;type:varchar(500)" json:"website"`
	Github         string     `gorm:"column:github;type:varchar(500)" json:"github"`
	Linkedin       string     `gorm:"column:linkedin;type:varchar(500)" json:"linkedin"`
	Twitter        string     `gorm:"column:twitter;type:varchar(500)" json:"twitter"`
	EmailAlerts    bool       `gorm:"column:email_alerts;type:boolean;default:true" json:"emailAlerts"`
	DirectMessages bool       `gorm:"column:direct_messages;type:boolean;default:true" json:"directMessages"`
	FeedUpdates    bool       `gorm:"column:feed_updates;type:boolean;default:false" json:"feedUpdates"`
	SecurityAlerts bool       `gorm:"column:security_alerts;type:boolean;default:true" json:"securityAlerts"`
	ReferralCode   string     `gorm:"column:referral_code;type:varchar(50);unique" json:"referralCode"`
	CoinsBalance   int        `gorm:"column:coins_balance;type:integer;default:0" json:"coinsBalance"`
	RegistrationIP *string    `gorm:"column:registration_ip;type:inet" json:"registrationIp"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (User) TableName() string {
	return "users"
}

type Student struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id;unique;not null" json:"userId"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Student) TableName() string {
	return "students"
}

type Admin struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Email     string    `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`
	Password  string    `gorm:"column:password;type:varchar(128);not null" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (Admin) TableName() string {
	return "admin"
}

type Teacher struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`
	Email           string     `gorm:"column:email;type:varchar(255);unique;not null" json:"email"`
	Password        string     `gorm:"column:password;type:varchar(128);not null" json:"-"`
	Name            string     `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Specialization  string     `gorm:"column:specialization;type:varchar(255);not null" json:"specialization"`
	AssignedCourses string     `gorm:"column:Assigned_courses;type:jsonb;default:'[]'" json:"assignedCourses"`
	Tasks           string     `gorm:"column:tasks;type:jsonb;default:'[]'" json:"tasks"`
	PhotoURL        string     `gorm:"column:photo_url;type:text" json:"photoUrl"`
	Category        string     `gorm:"column:category;type:varchar(100);default:'CSE(Graduation)'" json:"category"`
	DateOfBirth     *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`
}

func (Teacher) TableName() string {
	return "teachers"
}

type PendingRegistration struct {
	Email         string    `gorm:"primaryKey;column:email;type:varchar(255)" json:"email"`
	FullName      string    `gorm:"column:full_name;type:varchar(255);not null" json:"fullName"`
	Username      string    `gorm:"column:username;type:varchar(30);unique;not null" json:"username"`
	ContactNumber string    `gorm:"column:contact_number;type:varchar(32);not null" json:"contactNumber"`
	PasswordHash  string    `gorm:"column:password_hash;type:varchar(128);not null" json:"-"`
	CodeHash      string    `gorm:"column:code_hash;type:varchar(128);not null" json:"-"`
	CodeExpiresAt time.Time `gorm:"column:code_expires_at;type:timestamp with time zone" json:"codeExpiresAt"`
	Attempts      int       `gorm:"column:attempts;type:smallint;default:0" json:"attempts"`
	ResendCount   int       `gorm:"column:resend_count;type:smallint;default:0" json:"resendCount"`
	LastSentAt    time.Time `gorm:"column:last_sent_at;type:timestamp with time zone" json:"lastSentAt"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`
}

func (PendingRegistration) TableName() string {
	return "pending_registrations"
}

type PasswordResetOTP struct {
	Email         string    `gorm:"primaryKey;column:email;type:varchar(255)" json:"email"`
	CodeHash      string    `gorm:"column:code_hash;type:varchar(128);not null" json:"-"`
	CodeExpiresAt time.Time `gorm:"column:code_expires_at;type:timestamp with time zone" json:"codeExpiresAt"`
	Attempts      int       `gorm:"column:attempts;type:smallint;default:0" json:"attempts"`
	ResendCount   int       `gorm:"column:resend_count;type:smallint;default:0" json:"resendCount"`
	LastSentAt    time.Time `gorm:"column:last_sent_at;type:timestamp with time zone" json:"lastSentAt"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamp with time zone;autoUpdateTime" json:"updatedAt"`
}

func (PasswordResetOTP) TableName() string {
	return "password_reset_otps"
}

type Follow struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	FollowerID int64     `gorm:"column:follower_id;index:follower_followed_idx,unique;index:follower_idx;not null"`
	Follower   User      `gorm:"foreignKey:FollowerID;constraint:OnDelete:CASCADE"`
	FollowedID int64     `gorm:"column:followed_id;index:follower_followed_idx,unique;index:followed_idx;index:followed_created_idx;not null"`
	Followed   User      `gorm:"foreignKey:FollowedID;constraint:OnDelete:CASCADE"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (Follow) TableName() string {
	return "user_follows"
}

type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`
	RecipientID      int64     `gorm:"column:recipient_id;index;not null" json:"recipientId"`
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"`
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`
	Sender           *User     `gorm:"foreignKey:SenderID;constraint:OnDelete:SET NULL" json:"sender,omitempty"`
	SenderName       string    `gorm:"-" json:"senderName"`
	SenderAvatarUrl  string    `gorm:"-" json:"senderAvatarUrl"`
	NotificationType string    `gorm:"column:notification_type;type:varchar(50);default:'follow'" json:"notificationType"`
	Message          string    `gorm:"column:message;type:text" json:"message"`
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

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
	FollowUser(ctx context.Context, followerID, followedID int64) error
	UnfollowUser(ctx context.Context, followerID, followedID int64) error
	GetNotifications(ctx context.Context, userID int64, role string) ([]UserNotification, error)
	MarkNotificationsAsRead(ctx context.Context, userID int64, role string) error
	TokenRefresh(ctx context.Context, refreshToken string) (map[string]string, error)
}
