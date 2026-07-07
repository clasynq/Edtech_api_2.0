package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"clasynq/api/blog/internal/domain"

	"gorm.io/gorm"
)

// postgresBlogRepository implements domain.BlogRepository interface for GORM Postgres.
type postgresBlogRepository struct {
	db *gorm.DB
}

// NewPostgresBlogRepository initializes postgresBlogRepository.
func NewPostgresBlogRepository(db *gorm.DB) domain.BlogRepository {
	return &postgresBlogRepository{db: db}
}

// GetRawFeed retrieves non-restricted posts using category filters, text query parameters, and paging cursors.
func (r *postgresBlogRepository) GetRawFeed(ctx context.Context, category string, query string, cursor time.Time, limit int) ([]domain.BlogPost, error) {
	var posts []domain.BlogPost
	db := r.db.WithContext(ctx).Preload("Author").Where("is_restricted = ?", false)

	if category != "" && strings.ToLower(category) != "all" {
		db = db.Where("LOWER(category) = ?", strings.ToLower(category))
	}

	if query != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(query)+"%")
	}

	if !cursor.IsZero() {
		db = db.Where("created_at < ?", cursor)
	}

	err := db.Order("created_at desc").Limit(limit).Find(&posts).Error
	return posts, err
}

// GetPostBySlug retrieves a post by its unique URL slug.
func (r *postgresBlogRepository) GetPostBySlug(ctx context.Context, slug string) (*domain.BlogPost, error) {
	var post domain.BlogPost
	if err := r.db.WithContext(ctx).Preload("Author").Where("slug = ?", slug).First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

// GetPostByID retrieves a post by its primary ID.
func (r *postgresBlogRepository) GetPostByID(ctx context.Context, id int64) (*domain.BlogPost, error) {
	var post domain.BlogPost
	if err := r.db.WithContext(ctx).Preload("Author").First(&post, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

// CreatePost adds a new post record.
func (r *postgresBlogRepository) CreatePost(ctx context.Context, post *domain.BlogPost) error {
	return r.db.WithContext(ctx).Create(post).Error
}

// UpdatePost modifies post attributes.
func (r *postgresBlogRepository) UpdatePost(ctx context.Context, post *domain.BlogPost) error {
	return r.db.WithContext(ctx).Save(post).Error
}

// DeletePost removes a post and cleans up all related records (views, likes, comments, reposts, saves)
// as an atomic GORM database transaction.
func (r *postgresBlogRepository) DeletePost(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Delete associated views
		if err := tx.Where("post_id = ?", id).Delete(&domain.PostView{}).Error; err != nil {
			return err
		}
		// 2. Delete associated likes
		if err := tx.Where("post_id = ?", id).Delete(&domain.BlogLike{}).Error; err != nil {
			return err
		}
		// 3. Delete associated comments
		if err := tx.Where("post_id = ?", id).Delete(&domain.BlogComment{}).Error; err != nil {
			return err
		}
		// 4. Delete associated reposts
		if err := tx.Where("post_id = ?", id).Delete(&domain.Repost{}).Error; err != nil {
			return err
		}
		// 5. Delete associated saves
		if err := tx.Where("post_id = ?", id).Delete(&domain.SavedPost{}).Error; err != nil {
			return err
		}
		// 6. Delete the post itself
		if err := tx.Delete(&domain.BlogPost{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

// IsLiked checks if a user has liked a post.
func (r *postgresBlogRepository) IsLiked(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.BlogLike{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

// IsReposted checks if a user has reposted a post.
func (r *postgresBlogRepository) IsReposted(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Repost{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

// IsSaved checks if a user has saved/bookmarked a post.
func (r *postgresBlogRepository) IsSaved(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.SavedPost{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

// IsAuthorFollowed checks if the monolith's follower table maps followerID to followedID.
func (r *postgresBlogRepository) IsAuthorFollowed(ctx context.Context, followerID, followedID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, followedID).Count(&count).Error
	return count > 0, err
}

// GetFollowedAuthorIDs retrieves author IDs followed by a user.
func (r *postgresBlogRepository) GetFollowedAuthorIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ?", userID).Pluck("followed_id", &ids).Error
	return ids, err
}

// GetMutualConnectionWeights calculates mutual follower connection overlap scores.
// Utilized in recommendation scoring to prioritize feed items.
func (r *postgresBlogRepository) GetMutualConnectionWeights(ctx context.Context, userID int64) (map[int64]int, error) {
	type result struct {
		FollowedID  int64
		MutualCount int
	}
	var list []result

	err := r.db.WithContext(ctx).Raw(`
		SELECT f2.followed_id, COUNT(f2.follower_id) as mutual_count
		FROM user_follows f1
		JOIN user_follows f2 ON f1.followed_id = f2.follower_id
		WHERE f1.follower_id = ? 
		  AND f2.followed_id != ? 
		  AND f2.followed_id NOT IN (SELECT followed_id FROM user_follows WHERE follower_id = ?)
		GROUP BY f2.followed_id
	`, userID, userID, userID).Scan(&list).Error

	if err != nil {
		return nil, err
	}

	weights := make(map[int64]int)
	for _, item := range list {
		weights[item.FollowedID] = item.MutualCount
	}
	return weights, nil
}

// ToggleLike toggles a user's like status on a post. Returns true if liked, false if unliked.
func (r *postgresBlogRepository) ToggleLike(ctx context.Context, userID, postID int64) (bool, error) {
	var like domain.BlogLike
	isCreated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&like)
		if res.Error == nil {
			// Found, delete it (unlike)
			if err := tx.Delete(&like).Error; err != nil {
				return err
			}
			return nil
		} else if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			// Missing, insert it (like)
			like = domain.BlogLike{
				UserID: userID,
				PostID: postID,
			}
			if err := tx.Create(&like).Error; err != nil {
				return err
			}
			isCreated = true
			return nil
		}
		return res.Error
	})

	if err != nil {
		return false, err
	}

	return isCreated, nil
}

// ToggleSave toggles bookmark status. Returns true if bookmarked, false if removed.
func (r *postgresBlogRepository) ToggleSave(ctx context.Context, userID, postID int64) (bool, error) {
	var save domain.SavedPost
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&save)
		if res.Error == nil {
			if err := tx.Delete(&save).Error; err != nil {
				return err
			}
			return nil
		} else if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			save = domain.SavedPost{
				UserID: userID,
				PostID: postID,
			}
			if err := tx.Create(&save).Error; err != nil {
				return err
			}
			return nil
		}
		return res.Error
	})

	if err != nil {
		return false, err
	}
	return save.ID > 0, nil
}

// ToggleRepost toggles a user's repost status on a post. Returns true if reposted, false if removed.
func (r *postgresBlogRepository) ToggleRepost(ctx context.Context, userID, postID int64) (bool, error) {
	var repost domain.Repost
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&repost)
		if res.Error == nil {
			if err := tx.Delete(&repost).Error; err != nil {
				return err
			}
			return nil
		} else if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			repost = domain.Repost{
				UserID: userID,
				PostID: postID,
			}
			if err := tx.Create(&repost).Error; err != nil {
				return err
			}
			return nil
		}
		return res.Error
	})

	if err != nil {
		return false, err
	}
	return repost.ID > 0, nil
}

// CreateComment saves a comment on a post.
func (r *postgresBlogRepository) CreateComment(ctx context.Context, comment *domain.BlogComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

// DeleteComment removes a comment by ID if the author matches.
func (r *postgresBlogRepository) DeleteComment(ctx context.Context, id int64, authorID int64) error {
	return r.db.WithContext(ctx).Where("id = ? AND author_id = ?", id, authorID).Delete(&domain.BlogComment{}).Error
}

// GetCommentsForPost returns comments on a post preloaded with author profiles.
func (r *postgresBlogRepository) GetCommentsForPost(ctx context.Context, postID int64) ([]domain.BlogComment, error) {
	var rawComments []domain.BlogComment
	err := r.db.WithContext(ctx).Preload("Author").Where("post_id = ?", postID).Order("created_at asc").Find(&rawComments).Error
	if err != nil {
		return nil, err
	}
	return rawComments, nil
}

// IncrementPostCounters performs GORM expression updates to increment counters and recalculate scores.
func (r *postgresBlogRepository) IncrementPostCounters(ctx context.Context, postID int64, updates map[string]interface{}, scoreDiff float64) error {
	tx := r.db.WithContext(ctx).Model(&domain.BlogPost{}).Where("id = ?", postID)

	gormUpdates := make(map[string]interface{})
	for key, val := range updates {
		gormUpdates[key] = gorm.Expr(key+" + ?", val)
	}

	if scoreDiff != 0 {
		gormUpdates["engagement_score"] = gorm.Expr("engagement_score + ?", scoreDiff)
	}

	return tx.UpdateColumns(gormUpdates).Error
}

// CreateActivityLog saves user activity logs.
func (r *postgresBlogRepository) CreateActivityLog(ctx context.Context, log *domain.ActivityLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetActivityLogs retrieves recent activity logs for a user.
func (r *postgresBlogRepository) GetActivityLogs(ctx context.Context, userID int64, limit int) ([]domain.ActivityLog, error) {
	var logs []domain.ActivityLog
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("timestamp desc").Limit(limit).Find(&logs).Error
	return logs, err
}

// RecordView inserts a new view entry.
func (r *postgresBlogRepository) RecordView(ctx context.Context, view *domain.PostView) error {
	return r.db.WithContext(ctx).Create(view).Error
}

// GetAdminPosts retrieves posts preloaded with authors, matching search terms.
func (r *postgresBlogRepository) GetAdminPosts(ctx context.Context, query string, userSearch string, limit int) ([]domain.BlogPost, error) {
	var posts []domain.BlogPost
	db := r.db.WithContext(ctx).Preload("Author")

	if query != "" {
		db = db.Where("LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?",
			"%"+strings.ToLower(query)+"%",
			"%"+strings.ToLower(query)+"%",
			"%"+strings.ToLower(query)+"%")
	}

	if userSearch != "" {
		var authorID int64
		if id, err := strconv.ParseInt(userSearch, 10, 64); err == nil {
			authorID = id
		}

		if authorID > 0 {
			db = db.Where("author_id = ? OR author_id IN (SELECT id FROM users WHERE LOWER(username) LIKE ? OR LOWER(full_name) LIKE ?)",
				authorID, "%"+strings.ToLower(userSearch)+"%", "%"+strings.ToLower(userSearch)+"%")
		} else {
			db = db.Where("author_id IN (SELECT id FROM users WHERE LOWER(username) LIKE ? OR LOWER(full_name) LIKE ?)",
				"%"+strings.ToLower(userSearch)+"%", "%"+strings.ToLower(userSearch)+"%")
		}
	}

	if limit > 0 {
		db = db.Limit(limit)
	}

	err := db.Order("created_at desc").Find(&posts).Error
	return posts, err
}

// GetDistinctCategories returns unique categories listed across active posts.
func (r *postgresBlogRepository) GetDistinctCategories(ctx context.Context) ([]string, error) {
	var categories []string
	err := r.db.WithContext(ctx).Model(&domain.BlogPost{}).Distinct("category").Pluck("category", &categories).Error
	return categories, err
}

// GetLatestPostView retrieves the last logged view record for a post and viewer identifier.
func (r *postgresBlogRepository) GetLatestPostView(ctx context.Context, postID int64, viewerIdentifier string) (*domain.PostView, error) {
	var view domain.PostView
	err := r.db.WithContext(ctx).Where("post_id = ? AND viewer_identifier = ?", postID, viewerIdentifier).Order("viewed_at desc").First(&view).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &view, nil
}

// UpdatePostView updates a view record.
func (r *postgresBlogRepository) UpdatePostView(ctx context.Context, view *domain.PostView) error {
	return r.db.WithContext(ctx).Save(view).Error
}

// ToggleFollowUser adds or removes follow associations under user_follows table.
func (r *postgresBlogRepository) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, followedID).Count(&count).Error
	if err != nil {
		return false, err
	}

	if count > 0 {
		err = r.db.WithContext(ctx).Exec("DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?", followerID, followedID).Error
		return false, err
	}

	newFollow := map[string]interface{}{
		"follower_id": followerID,
		"followed_id": followedID,
		"created_at":  time.Now(),
	}
	err = r.db.WithContext(ctx).Table("user_follows").Create(&newFollow).Error
	return true, err
}

// CreateNotification logs a notification, omitting nested structures to avoid DB errors.
func (r *postgresBlogRepository) CreateNotification(ctx context.Context, notif *domain.UserNotification) error {
	return r.db.WithContext(ctx).Omit("Sender").Create(notif).Error
}

// GetUserRole queries monolith tables to resolve whether user belongs to teachers or admin roles.
func (r *postgresBlogRepository) GetUserRole(ctx context.Context, userID int64) (string, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("teachers").Where("id = ?", userID).Count(&count).Error; err == nil && count > 0 {
		return "teacher", nil
	}
	if err := r.db.WithContext(ctx).Table("admin").Where("id = ?", userID).Count(&count).Error; err == nil && count > 0 {
		return "admin", nil
	}
	return "student", nil
}

// GetUserByID retrieves a user profile by ID.
func (r *postgresBlogRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}


