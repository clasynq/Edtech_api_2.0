package domain

import (
	"context"
	"time"
)

// User represents a basic user profile details referencing the central "users" table.
type User struct {
	ID           int64  `gorm:"primaryKey;column:id" json:"id"`              // Primary key ID
	FullName     string `gorm:"column:full_name" json:"name"`                // Full name
	Username     string `gorm:"column:username" json:"username"`            // Unique username
	Email        string `gorm:"column:email" json:"email"`                  // Email address
	AvatarURL    string `gorm:"column:avatar_url" json:"avatarUrl"`          // Avatar photo path
	Bio          string `gorm:"column:bio" json:"bio"`                      // Biography summary
	Headline     string `gorm:"column:headline" json:"headline"`            // Headline summary (e.g. educator)
	Skills       string `gorm:"column:skills" json:"skills"`                // User skills
	ReferralCode string `gorm:"column:referral_code" json:"referralCode"`    // Referral code
}

// TableName matches the User struct to the central PostgreSQL table "users".
func (User) TableName() string {
	return "users"
}

// BlogPost represents a published post containing content, categories, tags, and engagement counters.
type BlogPost struct {
	ID              int64      `gorm:"primaryKey;column:id" json:"id"`                                            // Primary key ID
	Title           string     `gorm:"column:title;type:varchar(255);not null;index" json:"title"`                // Post title
	Slug            string     `gorm:"column:slug;type:varchar(255);unique;not null" json:"slug"`                 // Unique URL Slug
	Excerpt         string     `gorm:"column:excerpt;type:text" json:"excerpt"`                                   // Summary paragraph
	Content         string     `gorm:"column:content;type:text;not null" json:"content"`                           // Content HTML/Markdown text
	Category        string     `gorm:"column:category;type:varchar(100);default:'Web Development';index" json:"category"` // Post category classification
	Tags            string     `gorm:"column:tags;type:jsonb" json:"tags"`                                        // JSON array of strings
	AuthorID        int64      `gorm:"column:author_id;not null;index" json:"author_id"`                          // Author ID
	Author          User       `gorm:"foreignKey:AuthorID" json:"author"`                                         // Preloaded Author User details
	BannerURL       *string    `gorm:"column:banner_url;type:varchar(500)" json:"bannerUrl"`                      // Top Banner Image path
	ExploreLink     *string    `gorm:"column:explore_link;type:varchar(500)" json:"exploreLink"`                  // Optional related external link
	LikesCount      int        `gorm:"column:likes_count;default:0" json:"likesCount"`                            // Total likes count
	CommentsCount   int        `gorm:"column:comments_count;default:0" json:"commentsCount"`                      // Total comments count
	ViewsCount      int        `gorm:"column:views_count;default:0" json:"viewsCount"`                            // Total views count
	SharesCount     int        `gorm:"column:shares_count;default:0" json:"sharesCount"`                          // Total shares count
	RepostsCount    int        `gorm:"column:reposts_count;default:0" json:"repostsCount"`                        // Total reposts count
	SavesCount      int        `gorm:"column:saves_count;default:0" json:"savesCount"`                            // Total saves count
	ImageURL        *string    `gorm:"column:image_url;type:varchar(500)" json:"imageUrl"`                        // Optional content image
	VideoURL        *string    `gorm:"column:video_url;type:varchar(500)" json:"videoUrl"`                        // Optional content video
	EngagementScore float64    `gorm:"column:engagement_score;default:0.0" json:"engagementScore"`                // Calculated ranking score
	Featured        bool       `gorm:"column:featured;default:false" json:"featured"`                              // Featured badge toggle
	Trending        bool       `gorm:"column:trending;default:false" json:"trending"`                              // Trending status badge
	Recommended     bool       `gorm:"column:recommended;default:false" json:"recommended"`                        // Recommended status badge
	StaffPick       bool       `gorm:"column:staff_pick;default:false" json:"staffPick"`                          // Staff pick badge toggle
	IsRestricted    bool       `gorm:"column:is_restricted;default:false;index" json:"is_restricted"`             // Paid restricted toggle
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`

	// Annotation helper fields (Virtual, populated at runtime based on active session User)
	IsLiked          bool `gorm:"-" json:"isLiked"`          // Has current User liked this?
	IsReposted       bool `gorm:"-" json:"isReposted"`       // Has current User reposted this?
	IsSaved          bool `gorm:"-" json:"isSaved"`          // Has current User saved this?
	AuthorIsFollowed bool `gorm:"-" json:"authorIsFollowed"` // Is current User following Author?
}

// TableName matches the BlogPost struct to the PostgreSQL table "blog_posts".
func (BlogPost) TableName() string {
	return "blog_posts"
}

// BlogLike records which post likes are associated with which users.
type BlogLike struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_user_post;not null"`       // User ID
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_user_post;not null;index"` // Post ID
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

// TableName matches the BlogLike struct to the PostgreSQL table "blog_likes".
func (BlogLike) TableName() string {
	return "blog_likes"
}

// BlogComment represents replies left under posts (supporting nested threading).
type BlogComment struct {
	ID        int64         `gorm:"primaryKey;column:id" json:"id"`                              // Primary key ID
	PostID    int64         `gorm:"column:post_id;not null;index" json:"postId"`                 // Post ID
	AuthorID  int64         `gorm:"column:author_id;not null;index" json:"authorId"`             // Author ID
	Author    User          `gorm:"foreignKey:AuthorID" json:"author"`                           // Author details
	Content   string        `gorm:"column:content;type:text;not null" json:"content"`             // Text content
	ParentID  *int64        `gorm:"column:parent_id;index" json:"parentId"`                      // Thread parent ID (if nested reply)
	Replies   []BlogComment `gorm:"foreignKey:ParentID" json:"replies,omitempty"`                // Threaded sub-comments
	CreatedAt time.Time     `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime" json:"createdAt"`
}

// TableName matches the BlogComment struct to the PostgreSQL table "blog_comments".
func (BlogComment) TableName() string {
	return "blog_comments"
}

// PostView logs post reading times to detect spam views.
type PostView struct {
	ID               int64     `gorm:"primaryKey;column:id"`
	UserID           *int64    `gorm:"column:user_id;index"`                                       // Optional User ID (if logged-in)
	PostID           int64     `gorm:"column:post_id;index"`                                       // Viewed Post ID
	ReadTimeSeconds  int       `gorm:"column:read_time_seconds;default:0"`                         // Reading duration metric
	ViewedAt         time.Time `gorm:"column:viewed_at;type:timestamp with time zone;autoCreateTime;index"`
	ViewerIdentifier string    `gorm:"column:viewer_identifier;type:varchar(255);default:'';index"` // IP or session UUID fingerprint
}

// TableName matches the PostView struct to the PostgreSQL table "post_views".
func (PostView) TableName() string {
	return "post_views"
}

// Repost records user post reposts.
type Repost struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_repost_user_post;not null"`       // User ID
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_repost_user_post;not null;index"` // Post ID
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

// TableName matches the Repost struct to the PostgreSQL table "reposts".
func (Repost) TableName() string {
	return "reposts"
}

// SavedPost represents saved post bookmarks.
type SavedPost struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"column:user_id;uniqueIndex:idx_saved_user_post;not null"`       // User ID
	PostID    int64     `gorm:"column:post_id;uniqueIndex:idx_saved_user_post;not null;index"` // Post ID
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;autoCreateTime"`
}

// TableName matches the SavedPost struct to the PostgreSQL table "saved_posts".
func (SavedPost) TableName() string {
	return "saved_posts"
}

// UserNotification holds system alert parameters.
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

// TableName matches the UserNotification struct to the PostgreSQL table "user_notifications".
func (UserNotification) TableName() string {
	return "user_notifications"
}

// ActivityLog represents audit logs representing user actions like creating posts or liking comments.
type ActivityLog struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID       int64     `gorm:"column:user_id;not null;index" json:"userId"`
	ActivityType string    `gorm:"column:activity_type;type:varchar(50);not null" json:"activityType"` // 'post', 'comment', 'like'
	Description  string    `gorm:"column:description;type:text;not null" json:"description"`
	Timestamp    time.Time `gorm:"column:timestamp;type:timestamp with time zone;autoCreateTime;index" json:"timestamp"`
	TargetLink   *string   `gorm:"column:target_link;type:varchar(255)" json:"targetLink"`
	Details      *string   `gorm:"column:details;type:text" json:"details"`
}

// TableName matches the ActivityLog struct to the PostgreSQL table "activity_logs".
func (ActivityLog) TableName() string {
	return "activity_logs"
}

// BlogRepository defines database transaction contracts. Implemented in the repository layer.
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
	GetUserByID(ctx context.Context, id int64) (*User, error)

	// Admin Queries
	GetAdminPosts(ctx context.Context, query string, userSearch string, limit int) ([]BlogPost, error)
	GetDistinctCategories(ctx context.Context) ([]string, error)
}

// BlogUsecase defines core blog orchestrations. Implemented in the usecase layer.
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

