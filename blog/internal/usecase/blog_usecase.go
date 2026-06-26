package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"clasynq/api/blog/internal/domain"
)

type blogUsecase struct {
	repo domain.BlogRepository
}

func NewBlogUsecase(repo domain.BlogRepository) domain.BlogUsecase {
	return &blogUsecase{repo: repo}
}

// Helper to generate slug from title
func slugify(title string) string {
	slug := strings.ToLower(title)
	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	// Trim trailing/leading hyphens
	slug = strings.Trim(slug, "-")
	// Append random string to ensure unique slug
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().UnixNano()%100000)
	return slug + uniqueSuffix
}

// Log user activity to database
func (u *blogUsecase) logActivity(ctx context.Context, userID int64, activityType, description, targetLink, details string) {
	log := &domain.ActivityLog{
		UserID:       userID,
		ActivityType: activityType,
		Description:  description,
		Timestamp:    time.Now(),
	}
	if targetLink != "" {
		log.TargetLink = &targetLink
	}
	if details != "" {
		log.Details = &details
	}
	_ = u.repo.CreateActivityLog(ctx, log)
}

func (u *blogUsecase) GetFeed(ctx context.Context, userID int64, category string, query string, cursorStr string, tab string, limit int) (map[string]interface{}, error) {
	var cursorTime time.Time
	if cursorStr != "" {
		if t, err := time.Parse(time.RFC3339, cursorStr); err == nil {
			cursorTime = t
		}
	}

	// Pull 5x limit to rank the most relevant items efficiently (smart scoring candidate pool)
	candidateLimit := limit * 5
	candidates, err := u.repo.GetRawFeed(ctx, category, query, cursorTime, candidateLimit)
	if err != nil {
		return nil, err
	}

	// If user is authenticated, load personalization factors
	var followedIDs []int64
	var mutualWeights map[int64]int
	var userInterestSkills []string

	if userID > 0 {
		followedIDs, _ = u.repo.GetFollowedAuthorIDs(ctx, userID)
		mutualWeights, _ = u.repo.GetMutualConnectionWeights(ctx, userID)

		// Fetch user interest skills from database if possible
		// (For now, fallback to default matches since users are in another service)
		userInterestSkills = []string{"react", "django", "python", "postgresql", "css", "go", "golang"}
	}

	followedMap := make(map[int64]bool)
	for _, id := range followedIDs {
		followedMap[id] = true
	}

	// Filter candidates if following tab is selected and user is authenticated
	if tab == "following" && userID > 0 {
		filtered := make([]domain.BlogPost, 0)
		for _, post := range candidates {
			if followedMap[post.AuthorID] {
				filtered = append(filtered, post)
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		return map[string]interface{}{
			"posts":    []interface{}{},
			"next":     nil,
			"hasNext":  false,
		}, nil
	}

	// Structure candidates with metadata and compute personalization score
	type scoredPost struct {
		Post  domain.BlogPost
		Score float64
	}
	scoredList := make([]scoredPost, len(candidates))

	for i, post := range candidates {
		// Annotate metadata
		if userID > 0 {
			post.IsLiked, _ = u.repo.IsLiked(ctx, userID, post.ID)
			post.IsReposted, _ = u.repo.IsReposted(ctx, userID, post.ID)
			post.IsSaved, _ = u.repo.IsSaved(ctx, userID, post.ID)
			post.AuthorIsFollowed, _ = u.repo.IsAuthorFollowed(ctx, userID, post.AuthorID)
		}

		// Personalization Scoring Algorithm:
		var score float64
		if tab == "trending" {
			velocityScore := (float64(post.LikesCount) * 5.0) +
				(float64(post.CommentsCount) * 10.0) +
				(float64(post.RepostsCount) * 8.0) +
				(float64(post.ViewsCount) * 0.5)

			ageHours := time.Since(post.CreatedAt).Hours()
			decay := 1.0 / math.Pow(1.0+0.1*ageHours, 2.0)
			score = (velocityScore + 1.0) * decay
		} else {
			score = post.EngagementScore

			// Boost if author is followed
			if followedMap[post.AuthorID] {
				score += 100.0
			}

			// Boost for mutual connections
			if mutualWeights != nil {
				if count, ok := mutualWeights[post.AuthorID]; ok {
					score += float64(count) * 15.0
				}
			}

			// Boost for skill matches (tags matching interests)
			if len(userInterestSkills) > 0 && post.Tags != "" {
				tagsLower := strings.ToLower(post.Tags)
				for _, skill := range userInterestSkills {
					if strings.Contains(tagsLower, skill) {
						score += 10.0
					}
				}
			}
		}

		scoredList[i] = scoredPost{
			Post:  post,
			Score: score,
		}
	}

	// Sort by Score descending, fallback to CreatedAt
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].Score == scoredList[j].Score {
			return scoredList[i].Post.CreatedAt.After(scoredList[j].Post.CreatedAt)
		}
		return scoredList[i].Score > scoredList[j].Score
	})

	// Truncate to limit
	finalPosts := make([]domain.BlogPost, 0)
	for i := 0; i < len(scoredList) && i < limit; i++ {
		finalPosts = append(finalPosts, scoredList[i].Post)
	}

	hasNext := len(candidates) > limit
	var nextCursor *string
	if len(finalPosts) > 0 && hasNext {
		lastPostTime := finalPosts[len(finalPosts)-1].CreatedAt.Format(time.RFC3339)
		nextCursor = &lastPostTime
	}

	return map[string]interface{}{
		"posts":    finalPosts,
		"next":     nextCursor,
		"hasNext":  hasNext,
	}, nil
}

func (u *blogUsecase) GetPostDetail(ctx context.Context, userID int64, slug string, viewerIP string) (map[string]interface{}, error) {
	post, err := u.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	// Annotate personalized values
	if userID > 0 {
		post.IsLiked, _ = u.repo.IsLiked(ctx, userID, post.ID)
		post.IsReposted, _ = u.repo.IsReposted(ctx, userID, post.ID)
		post.IsSaved, _ = u.repo.IsSaved(ctx, userID, post.ID)
		post.AuthorIsFollowed, _ = u.repo.IsAuthorFollowed(ctx, userID, post.AuthorID)
	}

	// Fetch Comments Hierarchy
	comments, err := u.repo.GetCommentsForPost(ctx, post.ID)
	if err != nil {
		comments = []domain.BlogComment{}
	}

	// Increment Views atomically and update Engagement Score (+0.1)
	fields := map[string]interface{}{"views_count": 1}
	_ = u.repo.IncrementPostCounters(ctx, post.ID, fields, 0.1)

	// Record View log
	viewLog := &domain.PostView{
		PostID:           post.ID,
		ViewerIdentifier: viewerIP,
		ViewedAt:         time.Now(),
	}
	if userID > 0 {
		viewLog.UserID = &userID
	}
	_ = u.repo.RecordView(ctx, viewLog)

	// Refresh views count locally to reflect in current payload
	post.ViewsCount++

	return map[string]interface{}{
		"post":     post,
		"comments": comments,
	}, nil
}

func (u *blogUsecase) CreatePost(ctx context.Context, userID int64, title, excerpt, content, category, bannerURL, exploreLink, imageURL, videoURL string) (map[string]interface{}, error) {
	if len(content) > 60000 {
		return nil, errors.New("Content is too long. Maximum limit is 60,000 characters.")
	}

	slug := slugify(title)
	post := &domain.BlogPost{
		Title:        title,
		Slug:         slug,
		Excerpt:      excerpt,
		Content:      content,
		Category:     category,
		AuthorID:     userID,
		IsRestricted: false,
		Tags:         "[]",
	}

	if bannerURL != "" {
		post.BannerURL = &bannerURL
	}
	if exploreLink != "" {
		post.ExploreLink = &exploreLink
	}
	if imageURL != "" {
		post.ImageURL = &imageURL
	}
	if videoURL != "" {
		post.VideoURL = &videoURL
	}

	if err := u.repo.CreatePost(ctx, post); err != nil {
		return nil, err
	}

	// Fetch created post details
	createdPost, _ := u.repo.GetPostByID(ctx, post.ID)

	// Write Activity log
	msg := fmt.Sprintf("Published a new article: %s", title)
	u.logActivity(ctx, userID, "post", msg, "/blog/"+slug, excerpt)

	return map[string]interface{}{
		"post": createdPost,
	}, nil
}

func (u *blogUsecase) UpdatePost(ctx context.Context, userID int64, slug string, updates map[string]interface{}) (map[string]interface{}, error) {
	post, err := u.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	if post.AuthorID != userID {
		return nil, errors.New("You are not authorized to edit this article.")
	}

	if val, ok := updates["title"].(string); ok {
		post.Title = val
	}
	if val, ok := updates["excerpt"].(string); ok {
		post.Excerpt = val
	}
	if val, ok := updates["content"].(string); ok {
		if len(val) > 60000 {
			return nil, errors.New("Content is too long. Maximum limit is 60,000 characters.")
		}
		post.Content = val
	}
	if val, ok := updates["category"].(string); ok {
		post.Category = val
	}
	if val, ok := updates["bannerUrl"]; ok {
		if val == nil {
			post.BannerURL = nil
		} else if s, ok := val.(string); ok {
			post.BannerURL = &s
		}
	}
	if val, ok := updates["exploreLink"]; ok {
		if val == nil {
			post.ExploreLink = nil
		} else if s, ok := val.(string); ok {
			post.ExploreLink = &s
		}
	}
	if val, ok := updates["imageUrl"]; ok {
		if val == nil {
			post.ImageURL = nil
		} else if s, ok := val.(string); ok {
			post.ImageURL = &s
		}
	}
	if val, ok := updates["videoUrl"]; ok {
		if val == nil {
			post.VideoURL = nil
		} else if s, ok := val.(string); ok {
			post.VideoURL = &s
		}
	}
	if val, ok := updates["tags"]; ok {
		if s, ok := val.(string); ok {
			post.Tags = s
		} else if raw, err := json.Marshal(val); err == nil {
			post.Tags = string(raw)
		}
	}

	if err := u.repo.UpdatePost(ctx, post); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"post": post,
	}, nil
}

func (u *blogUsecase) DeletePost(ctx context.Context, userID int64, slug string) error {
	post, err := u.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if post == nil {
		return errors.New("Article not found.")
	}

	if post.AuthorID != userID {
		return errors.New("You are not authorized to delete this article.")
	}

	if err := u.repo.DeletePost(ctx, post.ID); err != nil {
		return err
	}

	// Write Activity log
	msg := fmt.Sprintf("Deleted the article: %s", post.Title)
	u.logActivity(ctx, userID, "post_delete", msg, "", "")

	return nil
}

func (u *blogUsecase) ToggleLike(ctx context.Context, userID, postID int64) (map[string]interface{}, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	liked, err := u.repo.ToggleLike(ctx, userID, postID)
	if err != nil {
		return nil, err
	}

	var likesDiff int
	var scoreDiff float64
	var msg, actType string

	if liked {
		likesDiff = 1
		scoreDiff = 10.0 // Like weight
		msg = fmt.Sprintf("Liked article: %s", post.Title)
		actType = "like"
	} else {
		likesDiff = -1
		scoreDiff = -10.0
		msg = fmt.Sprintf("Unliked article: %s", post.Title)
		actType = "unlike"
	}

	// Update counters
	fields := map[string]interface{}{"likes_count": likesDiff}
	_ = u.repo.IncrementPostCounters(ctx, postID, fields, scoreDiff)

	// Write Activity log
	u.logActivity(ctx, userID, actType, msg, "/blog/"+post.Slug, "")

	return map[string]interface{}{
		"liked":       liked,
		"likesCount":  post.LikesCount + likesDiff,
	}, nil
}

func (u *blogUsecase) ToggleSave(ctx context.Context, userID, postID int64) (map[string]interface{}, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	saved, err := u.repo.ToggleSave(ctx, userID, postID)
	if err != nil {
		return nil, err
	}

	var savesDiff int
	if saved {
		savesDiff = 1
	} else {
		savesDiff = -1
	}

	fields := map[string]interface{}{"saves_count": savesDiff}
	_ = u.repo.IncrementPostCounters(ctx, postID, fields, 0)

	return map[string]interface{}{
		"saved":      saved,
		"savesCount": post.SavesCount + savesDiff,
	}, nil
}

func (u *blogUsecase) ToggleRepost(ctx context.Context, userID, postID int64) (map[string]interface{}, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	reposted, err := u.repo.ToggleRepost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}

	var repostsDiff int
	if reposted {
		repostsDiff = 1
	} else {
		repostsDiff = -1
	}

	fields := map[string]interface{}{"reposts_count": repostsDiff}
	_ = u.repo.IncrementPostCounters(ctx, postID, fields, 0)

	return map[string]interface{}{
		"reposted":     reposted,
		"repostsCount": post.RepostsCount + repostsDiff,
	}, nil
}

func (u *blogUsecase) AddComment(ctx context.Context, userID, postID int64, content string, parentID *int64) (map[string]interface{}, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	comment := &domain.BlogComment{
		PostID:   postID,
		AuthorID: userID,
		Content:  content,
		ParentID: parentID,
	}

	if err := u.repo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	// Update comment counters (+20.0 points weight)
	fields := map[string]interface{}{"comments_count": 1}
	_ = u.repo.IncrementPostCounters(ctx, postID, fields, 20.0)

	// Fetch comment with loaded author details
	rawComments, _ := u.repo.GetCommentsForPost(ctx, postID)
	var createdComment domain.BlogComment
	for _, rc := range rawComments {
		if rc.ID == comment.ID {
			createdComment = rc
			break
		}
	}

	// Log activity
	msg := fmt.Sprintf("Commented on article: %s", post.Title)
	u.logActivity(ctx, userID, "comment", msg, "/blog/"+post.Slug, content)

	return map[string]interface{}{
		"comment": createdComment,
	}, nil
}

func (u *blogUsecase) DeleteComment(ctx context.Context, userID, commentID int64) error {
	// GORM deletion will enforce authorID ownership matching
	return u.repo.DeleteComment(ctx, commentID, userID)
}

func (u *blogUsecase) GetCommentsForPost(ctx context.Context, postID int64) ([]domain.BlogComment, error) {
	comments, err := u.repo.GetCommentsForPost(ctx, postID)
	if err != nil {
		return []domain.BlogComment{}, err
	}
	if comments == nil {
		comments = []domain.BlogComment{}
	}
	return comments, nil
}


func (u *blogUsecase) GetPostIDBySlug(ctx context.Context, slug string) (int64, error) {
	post, err := u.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return 0, err
	}
	if post == nil {
		return 0, errors.New("Article not found.")
	}
	return post.ID, nil
}

func (u *blogUsecase) GetAdminPosts(ctx context.Context, query string, userSearch string, limit int) (map[string]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}
	posts, err := u.repo.GetAdminPosts(ctx, query, userSearch, limit)
	if err != nil {
		return nil, err
	}
	categories, err := u.repo.GetDistinctCategories(ctx)
	if err != nil {
		categories = []string{}
	}

	return map[string]interface{}{
		"posts":      posts,
		"categories": categories,
	}, nil
}

func (u *blogUsecase) UpdatePostAsAdmin(ctx context.Context, id int64, updates map[string]interface{}) (map[string]interface{}, error) {
	post, err := u.repo.GetPostByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, errors.New("Article not found.")
	}

	if val, ok := updates["is_restricted"].(bool); ok {
		post.IsRestricted = val
	}
	if val, ok := updates["title"].(string); ok {
		post.Title = val
	}
	if val, ok := updates["excerpt"].(string); ok {
		post.Excerpt = val
	}
	if val, ok := updates["content"].(string); ok {
		if len(val) > 60000 {
			return nil, errors.New("Content is too long. Maximum limit is 60,000 characters.")
		}
		post.Content = val
	}
	if val, ok := updates["category"].(string); ok {
		post.Category = val
	}
	if val, ok := updates["bannerUrl"]; ok {
		if val == nil {
			post.BannerURL = nil
		} else if s, ok := val.(string); ok {
			post.BannerURL = &s
		}
	}
	if val, ok := updates["exploreLink"]; ok {
		if val == nil {
			post.ExploreLink = nil
		} else if s, ok := val.(string); ok {
			post.ExploreLink = &s
		}
	}
	if val, ok := updates["imageUrl"]; ok {
		if val == nil {
			post.ImageURL = nil
		} else if s, ok := val.(string); ok {
			post.ImageURL = &s
		}
	}
	if val, ok := updates["videoUrl"]; ok {
		if val == nil {
			post.VideoURL = nil
		} else if s, ok := val.(string); ok {
			post.VideoURL = &s
		}
	}
	if val, ok := updates["tags"]; ok {
		if s, ok := val.(string); ok {
			post.Tags = s
		} else if raw, err := json.Marshal(val); err == nil {
			post.Tags = string(raw)
		}
	}

	if err := u.repo.UpdatePost(ctx, post); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"post": post,
	}, nil
}

func (u *blogUsecase) DeletePostAsAdmin(ctx context.Context, id int64) error {
	post, err := u.repo.GetPostByID(ctx, id)
	if err != nil {
		return err
	}
	if post == nil {
		return errors.New("Article not found.")
	}

	return u.repo.DeletePost(ctx, id)
}

func (u *blogUsecase) GetUserActivities(ctx context.Context, userID int64, limit int) (map[string]interface{}, error) {
	logs, err := u.repo.GetActivityLogs(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	activities := make([]map[string]interface{}, len(logs))
	for i, log := range logs {
		targetLink := ""
		if log.TargetLink != nil {
			targetLink = *log.TargetLink
		}
		details := ""
		if log.Details != nil {
			details = *log.Details
		}

		activities[i] = map[string]interface{}{
			"id":          fmt.Sprintf("log_%d", log.ID),
			"type":        log.ActivityType,
			"description": log.Description,
			"timestamp":   log.Timestamp.Format(time.RFC3339),
			"targetLink":  targetLink,
			"details":     details,
		}
	}

	return map[string]interface{}{
		"activities": activities,
	}, nil
}

func (u *blogUsecase) TrackPostView(ctx context.Context, postID int64, viewerIdentifier string, userID int64) (int, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return 0, err
	}
	if post == nil {
		return 0, errors.New("Article not found.")
	}

	cooldownTime := time.Now().Add(-10 * time.Minute)
	latestView, err := u.repo.GetLatestPostView(ctx, postID, viewerIdentifier)
	if err != nil {
		return 0, err
	}

	isCooldown := latestView != nil && latestView.ViewedAt.After(cooldownTime)

	if !isCooldown {
		// Increment views_count and add 0.1 to engagement score
		fields := map[string]interface{}{"views_count": 1}
		_ = u.repo.IncrementPostCounters(ctx, postID, fields, 0.1)

		newView := &domain.PostView{
			PostID:           postID,
			ViewerIdentifier: viewerIdentifier,
			ReadTimeSeconds:  0,
			ViewedAt:         time.Now(),
		}
		if userID > 0 {
			newView.UserID = &userID
		}
		_ = u.repo.RecordView(ctx, newView)
		post.ViewsCount++

		// Milestone Notification for every 100 views
		if post.ViewsCount > 0 && post.ViewsCount % 100 == 0 {
			authorRole, errR := u.repo.GetUserRole(ctx, post.AuthorID)
			if errR == nil {
				msg := fmt.Sprintf("Your article \"%s\" has reached %d views!", post.Title, post.ViewsCount)
				notif := &domain.UserNotification{
					RecipientID:      post.AuthorID,
					RecipientRole:    authorRole,
					NotificationType: "milestone",
					Message:          msg,
					IsRead:           false,
				}
				_ = u.repo.CreateNotification(ctx, notif)
			}
		}
	} else {
		latestView.ViewedAt = time.Now()
		if userID > 0 && latestView.UserID == nil {
			latestView.UserID = &userID
		}
		_ = u.repo.UpdatePostView(ctx, latestView)
	}

	return post.ViewsCount, nil
}

func (u *blogUsecase) TrackPostEngagement(ctx context.Context, postID int64, readTimeSeconds int, viewerIdentifier string, userID int64) (float64, error) {
	post, err := u.repo.GetPostByID(ctx, postID)
	if err != nil {
		return 0, err
	}
	if post == nil {
		return 0, errors.New("Article not found.")
	}

	latestView, err := u.repo.GetLatestPostView(ctx, postID, viewerIdentifier)
	if err != nil {
		return 0, err
	}

	if latestView != nil {
		latestView.ReadTimeSeconds += readTimeSeconds
		if userID > 0 && latestView.UserID == nil {
			latestView.UserID = &userID
		}
		_ = u.repo.UpdatePostView(ctx, latestView)
	} else {
		newView := &domain.PostView{
			PostID:           postID,
			ViewerIdentifier: viewerIdentifier,
			ReadTimeSeconds:  readTimeSeconds,
			ViewedAt:         time.Now(),
		}
		if userID > 0 {
			newView.UserID = &userID
		}
		_ = u.repo.RecordView(ctx, newView)
	}

	if readTimeSeconds > 0 {
		// Increment engagement score by readTimeSeconds * 0.1
		scoreDiff := float64(readTimeSeconds) * 0.1
		_ = u.repo.IncrementPostCounters(ctx, postID, nil, scoreDiff)
		post.EngagementScore += scoreDiff
	}

	return post.EngagementScore, nil
}

func (u *blogUsecase) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID == followedID {
		return false, errors.New("You cannot follow yourself.")
	}

	isFollowing, err := u.repo.ToggleFollowUser(ctx, followerID, followedID)
	if err != nil {
		return false, err
	}

	if isFollowing {
		msg := "Started following user."
		u.logActivity(ctx, followerID, "follow", msg, fmt.Sprintf("/user/%d", followedID), "")
	}

	return isFollowing, nil
}
