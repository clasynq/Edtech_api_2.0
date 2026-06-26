package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"clasynq/api/dashboard_profile/internal/domain"
)

type profileUsecase struct {
	repo domain.ProfileRepository
}

func NewProfileUsecase(repo domain.ProfileRepository) domain.ProfileUsecase {
	return &profileUsecase{repo: repo}
}

func (u *profileUsecase) GetMe(ctx context.Context, userID int64, role string) (map[string]interface{}, error) {
	if role == "student" {
		user, err := u.repo.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, errors.New("User not found.")
		}
		return u.compileStudentProfile(ctx, user)
	} else if role == "admin" {
		admin, err := u.repo.GetAdminByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if admin == nil {
			return nil, errors.New("Admin not found.")
		}
		return map[string]interface{}{
			"id":        admin.ID,
			"email":     admin.Email,
			"role":      "admin",
			"fullName":  strings.Title(strings.Replace(strings.Split(admin.Email, "@")[0], ".", " ", -1)),
			"createdAt": admin.CreatedAt,
		}, nil
	} else if role == "teacher" {
		teacher, err := u.repo.GetTeacherByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if teacher == nil {
			return nil, errors.New("Teacher not found.")
		}
		return map[string]interface{}{
			"id":             teacher.ID,
			"email":          teacher.Email,
			"role":           "teacher",
			"fullName":       teacher.Name,
			"specialization": teacher.Specialization,
			"photoUrl":       teacher.PhotoURL,
			"createdAt":      teacher.CreatedAt,
		}, nil
	}

	return nil, errors.New("Unknown user role.")
}

func (u *profileUsecase) UpdateMe(ctx context.Context, userID int64, updates map[string]interface{}) (map[string]interface{}, error) {
	user, err := u.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("User not found.")
	}

	// Update fields selectively
	if val, ok := updates["fullName"].(string); ok {
		user.FullName = val
	}
	if val, ok := updates["avatarUrl"].(string); ok {
		user.AvatarURL = val
	}
	if val, ok := updates["headline"].(string); ok {
		user.Headline = val
	}
	if val, ok := updates["bio"].(string); ok {
		user.Bio = val
	}
	if val, ok := updates["skills"].(string); ok {
		user.Skills = val
	}
	if val, ok := updates["website"].(string); ok {
		user.Website = val
	}
	if val, ok := updates["github"].(string); ok {
		user.Github = val
	}
	if val, ok := updates["linkedin"].(string); ok {
		user.Linkedin = val
	}
	if val, ok := updates["twitter"].(string); ok {
		user.Twitter = val
	}

	dobVal, exists := updates["dateOfBirth"]
	if !exists {
		dobVal, exists = updates["date_of_birth"]
	}
	if exists {
		if valStr, ok := dobVal.(string); ok {
			if valStr == "" {
				user.DateOfBirth = nil
			} else {
				cleanDate := strings.Split(valStr, "T")[0]
				t, err := time.Parse("2006-01-02", cleanDate)
				if err == nil {
					user.DateOfBirth = &t
				}
			}
		}
	}

	if err := u.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return u.compileStudentProfile(ctx, user)
}

func (u *profileUsecase) compileStudentProfile(ctx context.Context, user *domain.User) (map[string]interface{}, error) {
	followers, _ := u.repo.GetFollowersList(ctx, user.ID)
	following, _ := u.repo.GetFollowingList(ctx, user.ID)
	referralsCount, _ := u.repo.GetStudentReferralsCount(ctx, user.ID)

	followersList := make([]map[string]interface{}, len(followers))
	for i, f := range followers {
		isFollowed, _ := u.repo.GetFollowRelationship(ctx, user.ID, f.FollowerID)
		followersList[i] = map[string]interface{}{
			"id":         f.Follower.ID,
			"name":       f.Follower.FullName,
			"email":      f.Follower.Email,
			"username":   f.Follower.Username,
			"avatarUrl":  f.Follower.AvatarURL,
			"avatar_url": f.Follower.AvatarURL,
			"bio":        f.Follower.Bio,
			"headline":   f.Follower.Headline,
			"skills":     f.Follower.Skills,
			"website":    f.Follower.Website,
			"github":     f.Follower.Github,
			"linkedin":   f.Follower.Linkedin,
			"twitter":    f.Follower.Twitter,
			"isFollowed": isFollowed != nil,
		}
	}

	followingList := make([]map[string]interface{}, len(following))
	for i, f := range following {
		followingList[i] = map[string]interface{}{
			"id":         f.Followed.ID,
			"name":       f.Followed.FullName,
			"email":      f.Followed.Email,
			"username":   f.Followed.Username,
			"avatarUrl":  f.Followed.AvatarURL,
			"avatar_url": f.Followed.AvatarURL,
			"bio":        f.Followed.Bio,
			"headline":   f.Followed.Headline,
			"skills":     f.Followed.Skills,
			"website":    f.Followed.Website,
			"github":     f.Followed.Github,
			"linkedin":   f.Followed.Linkedin,
			"twitter":    f.Followed.Twitter,
			"isFollowed": true,
		}
	}

	return map[string]interface{}{
		"id":              user.ID,
		"fullName":        user.FullName,
		"username":        user.Username,
		"contactNumber":   user.ContactNumber,
		"email":           user.Email,
		"role":            "student",
		"createdAt":       user.CreatedAt,
		"followersCount":  len(followers),
		"followingCount":  len(following),
		"followersList":   followersList,
		"followingList":   followingList,
		"avatarUrl":       user.AvatarURL,
		"avatar_url":      user.AvatarURL,
		"headline":        user.Headline,
		"bio":             user.Bio,
		"skills":          user.Skills,
		"website":         user.Website,
		"github":          user.Github,
		"linkedin":        user.Linkedin,
		"twitter":         user.Twitter,
		"emailAlerts":     user.EmailAlerts,
		"directMessages":  user.DirectMessages,
		"feedUpdates":     user.FeedUpdates,
		"securityAlerts":  user.SecurityAlerts,
		"coinsBalance":    user.CoinsBalance,
		"referralCode":    user.ReferralCode,
		"dateOfBirth":     user.DateOfBirth,
		"referralsCount":  referralsCount,
	}, nil
}

func (u *profileUsecase) GetMutualConnections(ctx context.Context, userID int64) (map[string]interface{}, error) {
	mutuals, err := u.repo.GetMutualConnections(ctx, userID)
	if err != nil {
		return nil, err
	}

	suggestions := make([]map[string]interface{}, len(mutuals))
	for i, m := range mutuals {
		suggestions[i] = map[string]interface{}{
			"id":          m.User.ID,
			"name":        m.User.FullName,
			"username":    m.User.Username,
			"email":       m.User.Email,
			"avatarUrl":   m.User.AvatarURL,
			"avatar_url":  m.User.AvatarURL,
			"headline":    m.User.Headline,
			"bio":         m.User.Bio,
			"mutualCount": m.MutualCount,
			"isFollowed":  false,
			"is_followed":  false,
		}
	}

	return map[string]interface{}{
		"suggestions": suggestions,
	}, nil
}

func (u *profileUsecase) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID == followedID {
		return false, errors.New("You cannot follow yourself.")
	}

	isFollowing, err := u.repo.ToggleFollowUser(ctx, followerID, followedID)
	if err != nil {
		return false, err
	}

	if isFollowing {
		desc := "Started following user."
		link := fmt.Sprintf("/user/%d", followedID)
		actLog := domain.ActivityLog{
			UserID:       followerID,
			ActivityType: "follow",
			Description:  desc,
			TargetLink:   &link,
		}
		_ = u.repo.CreateActivityLog(ctx, &actLog)

		// Create follow notification
		var followerName string
		followerUser, errU := u.repo.GetUserByID(ctx, followerID)
		if errU == nil && followerUser != nil {
			followerName = followerUser.FullName
		} else {
			teacher, errT := u.repo.GetTeacherByID(ctx, followerID)
			if errT == nil && teacher != nil {
				followerName = teacher.Name
			} else {
				admin, errA := u.repo.GetAdminByID(ctx, followerID)
				if errA == nil && admin != nil {
					followerName = "Admin"
					if admin.Email != "" {
						parts := strings.Split(admin.Email, "@")
						followerName = strings.Title(strings.Replace(parts[0], ".", " ", -1))
					}
				}
			}
		}

		if followerName != "" {
			followedRole := "student"
			teacher, errT := u.repo.GetTeacherByID(ctx, followedID)
			if errT == nil && teacher != nil {
				followedRole = "teacher"
			} else {
				admin, errA := u.repo.GetAdminByID(ctx, followedID)
				if errA == nil && admin != nil {
					followedRole = "admin"
				}
			}

			msg := fmt.Sprintf("%s started following you.", followerName)
			notif := domain.UserNotification{
				RecipientID:      followedID,
				RecipientRole:    followedRole,
				SenderID:         &followerID,
				NotificationType: "follow",
				Message:          msg,
				IsRead:           false,
			}
			_ = u.repo.CreateNotification(ctx, &notif)
		}
	}

	return isFollowing, nil
}
