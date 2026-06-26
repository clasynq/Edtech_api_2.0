package domain

import (
	"context"
	"time"
)

type User struct {
	ID            int64  `gorm:"primaryKey;column:id" json:"id"`
	FullName      string `gorm:"column:full_name" json:"name"`
	Username      string `gorm:"column:username" json:"username"`
	Email         string `gorm:"column:email" json:"email"`
	AvatarURL     string `gorm:"column:avatar_url" json:"avatarUrl"`
	Bio           string `gorm:"column:bio" json:"bio"`
	Headline      string `gorm:"column:headline" json:"headline"`
	Skills        string `gorm:"column:skills" json:"skills"`
	ReferralCode  string `gorm:"column:referral_code" json:"referralCode"`
}

func (User) TableName() string {
	return "users"
}

type BlogPost struct {
	ID                 int64      `gorm:"primaryKey;column:id" json:"id"`
	Title              string     `gorm:"column:title;type:varchar(255);not null;index" json:"title"`
	Slug               string     `gorm:"column:slug;type:varchar(255);unique;not null" json:"slug"`
	Excerpt            string     `gorm:"column:excerpt;type:text" json:"excerpt"`
	Content            string     `gorm:"column:content;type:text;not null" json:"content"`
	Category           string     `gorm:"column:category;type:varchar(100);default:'Web Development';index" json:"category"`
	Tags               string     `gorm:"column:tags;type:jsonb" json:"tags"` // JSON array string
	AuthorID           int64      `gorm:"column:author_id;not null;index" json:"author_id"`
	Author             User       `gorm:"foreignKey:AuthorID" json:"author"`
	BannerURL          *string    `gorm:"column:banner_url;type:varchar(500)" json:"bannerUrl"`
	ExploreLink        *string    `gorm:"column:explore_link;type:varchar(500)" json:"exploreLink"`
	LikesCount         int        `gorm:"column:likes_count;default:0" json:"likesCount"`
	CommentsCount      int        `gorm:"column:comments_count;default:0" json:"commentsCount"`
	ViewsCount         int        `gorm:"column:views_count;default:0" json:"viewsCount"`
	SharesCount        int        `gorm:"column:shares_count;default:0" json:"sharesCount"`
	RepostsCount       int        `gorm:"column:reposts_count;default:0" json:"repostsCount"`
	SavesCount         int        `gorm:"column:saves_count;default:0" json:"savesCount"`
	ImageURL           *string    `gorm:"column:image_url;type:varchar(500)" json:"imageUrl"`
	VideoURL           *string    `gorm:"column:video_url;type:varchar(500)" json:"videoUrl"`
	EngagementScore    float64    `gorm:"column:engagement_score;default:0.0" json:"engagementScore"`
	Featured           bool       `gorm:"column:featured;default:false" json:"featured"`
	Trending           bool       `gorm:"column:trending;default:false" json:"trending"`
	Recommended        bool       `gorm:"column:recommended;default:false" json:"recommended"`
	StaffPick          bool       `gorm:"column:staff_pick;default:false" json:"staffPick"`
	IsRestricted       bool       `gorm:"column:is_restricted;default:false;index" json:"is_restricted"`
	CreatedAt          time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`

	// Annotation helper fields (not persisted in DB)
	IsLiked          bool `gorm:"-" json:"isLiked"`
	IsReposted       bool `gorm:"-" json:"isReposted"`
	IsSaved          bool `gorm:"-" json:"isSaved"`
	AuthorIsFollowed bool `gorm:"-" json:"authorIsFollowed"`
}

func (BlogPost) TableName() string {
	return "blog_posts"
}

type BlogLike struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_user_post;not null"`
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_user_post;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (BlogLike) TableName() string {
	return "blog_likes"
}

type BlogComment struct {
	ID        int64          `gorm:"primaryKey;column:id" json:"id"`
	PostID    int64          `gorm:"column:post_id;not null;index" json:"postId"`
	AuthorID  int64          `gorm:"column:author_id;not null;index" json:"authorId"`
	Author    User           `gorm:"foreignKey:AuthorID" json:"author"`
	Content   string         `gorm:"column:content;type:text;not null" json:"content"`
	ParentID  *int64         `gorm:"column:parent_id;index" json:"parentId"`
	Replies   []BlogComment  `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (BlogComment) TableName() string {
	return "blog_comments"
}

type PostView struct {
	ID               int64     `gorm:"primaryKey;column:id"`
	UserID           *int64    `gorm:"column:user_id;index"`
	PostID           int64     `gorm:"column:post_id;index"`
	ReadTimeSeconds  int       `gorm:"column:read_time_seconds;default:0"`
	ViewedAt         time.Time `gorm:"column:viewed_at;type:timestamp with time zone;autoCreateTime;index"`
	ViewerIdentifier string    `gorm:"column:viewer_identifier;type:varchar(255);default:'';index"`
}

func (PostView) TableName() string {
	return "post_views"
}

type Repost struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_repost_user_post;not null"`
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_repost_user_post;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (Repost) TableName() string {
	return "reposts"
}

type SavedPost struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_saved_user_post;not null"`
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_saved_user_post;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

func (SavedPost) TableName() string {
	return "saved_posts"
}

type UserNotification struct {
	ID               int64     `gorm:"primaryKey;column:id" json:"id"`
	RecipientID      int64     `gorm:"column:recipient_id;not null;index" json:"recipientId"`
	RecipientRole    string    `gorm:"column:recipient_role;type:varchar(50);default:'student'" json:"recipientRole"`
	SenderID         *int64    `gorm:"column:sender_id;index" json:"senderId"`
	Sender           User      `gorm:"foreignKey:SenderID;constraint:OnDelete:SET NULL" json:"sender"`
	NotificationType string    `gorm:"column:notification_type;type:varchar(50);default:'system'" json:"notificationType"`
	Message          string    `gorm:"column:message;type:text;not null" json:"message"`
	IsRead           bool      `gorm:"column:is_read;type:boolean;default:false" json:"isRead"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

func (UserNotification) TableName() string {
	return "user_notifications"
}

type ActivityLog struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null;index" json:"userId"`
	ActivityType string    `gorm:"column:activity_type;type:varchar(50);not null" json:"activityType"` // 'post', 'comment', 'like', 'post_delete'
	Description  string    `gorm:"column:description;type:text;not null" json:"description"`
	Timestamp    time.Time `gorm:"column:timestamp;type:timestamp with time zone;autoCreateTime;index" json:"timestamp"`
	TargetLink   *string   `gorm:"column:target_link;type:varchar(255)" json:"targetLink"`
	Details      *string   `gorm:"column:details;type:text" json:"details"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}

type BlogRepository interface {
	GetRawFeed(ctx context.Context, category string, query string, cursor time.Time, limit int) ([]BlogPost, error)
	GetPostBySlug(ctx context.Context, slug string) (*BlogPost, error)
	GetPostByID(ctx context.Context, id int64) (*BlogPost, error)
	CreatePost(ctx context.Context, post *BlogPost) error
	UpdatePost(ctx context.Context, post *BlogPost) error
	DeletePost(ctx context.Context, id int64) error

	// Annotation Helpers
	IsLiked(ctx context.Context, userID, postID int64) (bool, error)
	IsReposted(ctx context.Context, userID, postID int64) (bool, error)
	IsSaved(ctx context.Context, userID, postID int64) (bool, error)
	IsAuthorFollowed(ctx context.Context, followerID, followedID int64) (bool, error)

	// User connection details (for feed algorithm)
	GetFollowedAuthorIDs(ctx context.Context, userID int64) ([]int64, error)
	GetMutualConnectionWeights(ctx context.Context, userID int64) (map[int64]int, error)

	// Interactions
	ToggleLike(ctx context.Context, userID, postID int64) (bool, error)
	ToggleSave(ctx context.Context, userID, postID int64) (bool, error)
	ToggleRepost(ctx context.Context, userID, postID int64) (bool, error)

	// Comments
	CreateComment(ctx context.Context, comment *BlogComment) error
	DeleteComment(ctx context.Context, id int64, authorID int64) error
	GetCommentsForPost(ctx context.Context, postID int64) ([]BlogComment, error)

	// Counters and logs
	IncrementPostCounters(ctx context.Context, postID int64, updates map[string]interface{}, scoreDiff float64) error
	CreateActivityLog(ctx context.Context, log *ActivityLog) error
	GetActivityLogs(ctx context.Context, userID int64, limit int) ([]ActivityLog, error)
	RecordView(ctx context.Context, view *PostView) error
	GetLatestPostView(ctx context.Context, postID int64, viewerIdentifier string) (*PostView, error)
	UpdatePostView(ctx context.Context, view *PostView) error
	ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error)
	CreateNotification(ctx context.Context, notif *UserNotification) error
	GetUserRole(ctx context.Context, userID int64) (string, error)

	// Admin Queries
	GetAdminPosts(ctx context.Context, query string, userSearch string, limit int) ([]BlogPost, error)
	GetDistinctCategories(ctx context.Context) ([]string, error)
}

type BlogUsecase interface {
	GetFeed(ctx context.Context, userID int64, category string, query string, cursorStr string, tab string, limit int) (map[string]interface{}, error)
	GetPostDetail(ctx context.Context, userID int64, slug string, viewerIP string) (map[string]interface{}, error)
	CreatePost(ctx context.Context, userID int64, title, excerpt, content, category, bannerURL, exploreLink, imageURL, videoURL string) (map[string]interface{}, error)
	UpdatePost(ctx context.Context, userID int64, slug string, updates map[string]interface{}) (map[string]interface{}, error)
	DeletePost(ctx context.Context, userID int64, slug string) error
	ToggleLike(ctx context.Context, userID, postID int64) (map[string]interface{}, error)
	ToggleSave(ctx context.Context, userID, postID int64) (map[string]interface{}, error)
	ToggleRepost(ctx context.Context, userID, postID int64) (map[string]interface{}, error)
	AddComment(ctx context.Context, userID, postID int64, content string, parentID *int64) (map[string]interface{}, error)
	DeleteComment(ctx context.Context, userID, commentID int64) error
	GetCommentsForPost(ctx context.Context, postID int64) ([]BlogComment, error)
	GetPostIDBySlug(ctx context.Context, slug string) (int64, error)
	GetUserActivities(ctx context.Context, userID int64, limit int) (map[string]interface{}, error)
	TrackPostView(ctx context.Context, postID int64, viewerIdentifier string, userID int64) (int, error)
	TrackPostEngagement(ctx context.Context, postID int64, readTimeSeconds int, viewerIdentifier string, userID int64) (float64, error)
	ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error)

	// Admin Operations
	GetAdminPosts(ctx context.Context, query string, userSearch string, limit int) (map[string]interface{}, error)
	UpdatePostAsAdmin(ctx context.Context, id int64, updates map[string]interface{}) (map[string]interface{}, error)
	DeletePostAsAdmin(ctx context.Context, id int64) error
}
