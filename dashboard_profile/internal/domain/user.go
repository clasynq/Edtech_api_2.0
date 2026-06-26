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
	UserID    int64     `gorm:"column:user_id;uniqueIndex;not null" json:"userId"`
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

type ActivityLog struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null;index" json:"userId"`
	ActivityType string    `gorm:"column:activity_type;type:varchar(50);not null" json:"activityType"`
	Description  string    `gorm:"column:description;type:text;not null" json:"description"`
	Timestamp    time.Time `gorm:"column:timestamp;type:timestamp with time zone;autoCreateTime;index" json:"timestamp"`
	TargetLink   *string   `gorm:"column:target_link;type:varchar(255)" json:"targetLink"`
	Details      *string   `gorm:"column:details;type:text" json:"details"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}

type MutualConnection struct {
	User        User
	MutualCount int
}

type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`
	RecipientID      int64     `gorm:"column:recipient_id;not null;index" json:"recipientId"`
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"`
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`
	Sender           User      `gorm:"foreignKey:SenderID;constraint:OnDelete:SET NULL" json:"sender"`
	NotificationType string    `gorm:"column:notification_type;type:varchar(50);default:'follow'" json:"notificationType"`
	Message          string    `gorm:"column:message;type:text;not null" json:"message"`
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

type ProfileRepository interface {
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetStudentReferralsCount(ctx context.Context, userID int64) (int64, error)
	GetFollowersList(ctx context.Context, userID int64) ([]Follow, error)
	GetFollowingList(ctx context.Context, userID int64) ([]Follow, error)
	GetFollowRelationship(ctx context.Context, followerID, followedID int64) (*Follow, error)
	UpdateUser(ctx context.Context, user *User) error
	GetTeacherByID(ctx context.Context, id int64) (*Teacher, error)
	GetAdminByID(ctx context.Context, id int64) (*Admin, error)
	ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error)
	GetMutualConnections(ctx context.Context, userID int64) ([]MutualConnection, error)
	CreateActivityLog(ctx context.Context, log *ActivityLog) error
	CreateNotification(ctx context.Context, notif *UserNotification) error

	// Study & Dashboard methods
	GetStudentByUserID(ctx context.Context, userID int64) (*Student, error)
	GetEnrollmentsByStudentID(ctx context.Context, studentID int64) ([]Enrollment, error)
	GetCoursesByIDs(ctx context.Context, courseIDs []int64, category string) ([]Course, error)
	GetClassSchedulesByCourseIDsAndDateRange(ctx context.Context, courseIDs []int64, startDate, endDate time.Time) ([]ClassSchedule, error)
	GetCompletedClassSchedulesByCourseIDs(ctx context.Context, courseIDs []int64) ([]ClassSchedule, error)
}

type ProfileUsecase interface {
	GetMe(ctx context.Context, userID int64, role string) (map[string]interface{}, error)
	UpdateMe(ctx context.Context, userID int64, updates map[string]interface{}) (map[string]interface{}, error)
	GetMutualConnections(ctx context.Context, userID int64) (map[string]interface{}, error)
	ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error)
	GetStudyDashboard(ctx context.Context, userID int64, category string) (map[string]interface{}, error)
	GetHistory(ctx context.Context, userID int64, category string) (map[string]interface{}, error)
}
