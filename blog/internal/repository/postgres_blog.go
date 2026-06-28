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

type postgresBlogRepository struct {
	db *gorm.DB
}

func NewPostgresBlogRepository(db *gorm.DB) domain.BlogRepository {
	return &postgresBlogRepository{db: db}
}

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

func (r *postgresBlogRepository) CreatePost(ctx context.Context, post *domain.BlogPost) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *postgresBlogRepository) UpdatePost(ctx context.Context, post *domain.BlogPost) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *postgresBlogRepository) DeletePost(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Delete from post_views
		if err := tx.Where("post_id = ?", id).Delete(&domain.PostView{}).Error; err != nil {
			return err
		}
		// 2. Delete from blog_likes
		if err := tx.Where("post_id = ?", id).Delete(&domain.BlogLike{}).Error; err != nil {
			return err
		}
		// 3. Delete from blog_comments
		if err := tx.Where("post_id = ?", id).Delete(&domain.BlogComment{}).Error; err != nil {
			return err
		}
		// 4. Delete from reposts
		if err := tx.Where("post_id = ?", id).Delete(&domain.Repost{}).Error; err != nil {
			return err
		}
		// 5. Delete from saved_posts
		if err := tx.Where("post_id = ?", id).Delete(&domain.SavedPost{}).Error; err != nil {
			return err
		}
		// 6. Finally delete the post itself
		if err := tx.Delete(&domain.BlogPost{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *postgresBlogRepository) IsLiked(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.BlogLike{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

func (r *postgresBlogRepository) IsReposted(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Repost{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

func (r *postgresBlogRepository) IsSaved(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.SavedPost{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	return count > 0, err
}

func (r *postgresBlogRepository) IsAuthorFollowed(ctx context.Context, followerID, followedID int64) (bool, error) {
	var count int64
	// Table user_follows has columns follower_id and followed_id
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, followedID).Count(&count).Error
	return count > 0, err
}

func (r *postgresBlogRepository) GetFollowedAuthorIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ?", userID).Pluck("followed_id", &ids).Error
	return ids, err
}

func (r *postgresBlogRepository) GetMutualConnectionWeights(ctx context.Context, userID int64) (map[int64]int, error) {
	type result struct {
		FollowedID  int64
		MutualCount int
	}
	var list []result

	// Monolith SQL:
	// SELECT f2.followed_id, COUNT(f2.follower_id) as mutual_count
	// FROM user_follows f1
	// JOIN user_follows f2 ON f1.followed_id = f2.follower_id
	// WHERE f1.follower_id = ? AND f2.followed_id != ? AND f2.followed_id NOT IN (select followed_id from user_follows where follower_id = ?)
	// GROUP BY f2.followed_id
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

func (r *postgresBlogRepository) ToggleLike(ctx context.Context, userID, postID int64) (bool, error) {
	var like domain.BlogLike
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&like)
		if res.Error == nil {
			// Exist, so delete it (unlike)
			if err := tx.Delete(&like).Error; err != nil {
				return err
			}
			return nil // Return nil transaction, like represents liked state (it was unliked)
		} else if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			// Not found, so create it (like)
			like = domain.BlogLike{
				UserID: userID,
				PostID: postID,
			}
			if err := tx.Create(&like).Error; err != nil {
				return err
			}
			return nil
		}
		return res.Error
	})

	if err != nil {
		return false, err
	}

	// If like.ID > 0, it means we just created it (liked)
	return like.ID > 0, nil
}

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

func (r *postgresBlogRepository) CreateComment(ctx context.Context, comment *domain.BlogComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *postgresBlogRepository) DeleteComment(ctx context.Context, id int64, authorID int64) error {
	return r.db.WithContext(ctx).Where("id = ? AND author_id = ?", id, authorID).Delete(&domain.BlogComment{}).Error
}

func (r *postgresBlogRepository) GetCommentsForPost(ctx context.Context, postID int64) ([]domain.BlogComment, error) {
	var rawComments []domain.BlogComment
	// Fetch comments and pre-fill author details
	err := r.db.WithContext(ctx).Preload("Author").Where("post_id = ?", postID).Order("created_at asc").Find(&rawComments).Error
	if err != nil {
		return nil, err
	}
	return rawComments, nil
}

func (r *postgresBlogRepository) IncrementPostCounters(ctx context.Context, postID int64, updates map[string]interface{}, scoreDiff float64) error {
	tx := r.db.WithContext(ctx).Model(&domain.BlogPost{}).Where("id = ?", postID)

	// Build GORM updates
	gormUpdates := make(map[string]interface{})
	for key, val := range updates {
		gormUpdates[key] = gorm.Expr(key+" + ?", val)
	}

	if scoreDiff != 0 {
		gormUpdates["engagement_score"] = gorm.Expr("engagement_score + ?", scoreDiff)
	}

	return tx.UpdateColumns(gormUpdates).Error
}

func (r *postgresBlogRepository) CreateActivityLog(ctx context.Context, log *domain.ActivityLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *postgresBlogRepository) GetActivityLogs(ctx context.Context, userID int64, limit int) ([]domain.ActivityLog, error) {
	var logs []domain.ActivityLog
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("timestamp desc").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *postgresBlogRepository) RecordView(ctx context.Context, view *domain.PostView) error {
	return r.db.WithContext(ctx).Create(view).Error
}

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

func (r *postgresBlogRepository) GetDistinctCategories(ctx context.Context) ([]string, error) {
	var categories []string
	err := r.db.WithContext(ctx).Model(&domain.BlogPost{}).Distinct("category").Pluck("category", &categories).Error
	return categories, err
}

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

func (r *postgresBlogRepository) UpdatePostView(ctx context.Context, view *domain.PostView) error {
	return r.db.WithContext(ctx).Save(view).Error
}

func (r *postgresBlogRepository) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ? AND followed_id = ?", followerID, followedID).Count(&count).Error
	if err != nil {
		return false, err
	}

	if count > 0 {
		// Delete
		err = r.db.WithContext(ctx).Exec("DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?", followerID, followedID).Error
		return false, err
	}

	// Create
	newFollow := map[string]interface{}{
		"follower_id": followerID,
		"followed_id": followedID,
		"created_at":  time.Now(),
	}
	err = r.db.WithContext(ctx).Table("user_follows").Create(&newFollow).Error
	return true, err
}

func (r *postgresBlogRepository) CreateNotification(ctx context.Context, notif *domain.UserNotification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *postgresBlogRepository) GetUserRole(ctx context.Context, userID int64) (string, error) {
	var count int64
	// Check teachers table
	if err := r.db.WithContext(ctx).Table("teachers").Where("id = ?", userID).Count(&count).Error; err == nil && count > 0 {
		return "teacher", nil
	}
	// Check admin table
	if err := r.db.WithContext(ctx).Table("admin").Where("id = ?", userID).Count(&count).Error; err == nil && count > 0 {
		return "admin", nil
	}
	// Default to student
	return "student", nil
}

