package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"clasynq/api/auth/config"
	"clasynq/api/auth/internal/domain"
	"clasynq/api/auth/internal/utils"
	"github.com/redis/go-redis/v9"
)

type userUsecase struct {
	repo   domain.UserRepository
	rdb    *redis.Client
	cfg    *config.Config
}

func NewUserUsecase(repo domain.UserRepository, rdb *redis.Client, cfg *config.Config) domain.UserUsecase {
	return &userUsecase{
		repo: repo,
		rdb:  rdb,
		cfg:  cfg,
	}
}

// Generate cryptographically secure numeric code
func generateNumericCode(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(10))
		result[i] = digits[num.Int64()]
	}
	return string(result)
}

// Generate cryptographically secure random salt
func generateSalt(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func (u *userUsecase) Register(ctx context.Context, fullName, username, email, contact, password, remoteIP string) (map[string]interface{}, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.ToLower(strings.TrimSpace(username))
	contact = strings.TrimSpace(contact)

	// Check if email taken
	existingUser, err := u.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return map[string]interface{}{"code": "email_taken", "message": "Email is already in use."}, nil
	}

	// Check if username taken
	existingUsername, err := u.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existingUsername != nil {
		return map[string]interface{}{"code": "username_taken", "message": "Username is already taken."}, nil
	}

	// Check if contact number taken
	existingContact, err := u.repo.GetUserByContact(ctx, contact)
	if err != nil {
		return nil, err
	}
	if existingContact != nil {
		return map[string]interface{}{"code": "contact_number_taken", "message": "Contact number is already in use."}, nil
	}

	// Pre-hash password with Django PBKDF2
	salt := generateSalt(12)
	passwordHash := utils.EncodeDjangoPassword(password, salt, 390000)

	// Generate OTP
	rawCode := generateNumericCode(6)
	codeSalt := generateSalt(12)
	codeHash := utils.EncodeDjangoPassword(rawCode, codeSalt, 390000)

	ttlSeconds := 300 // 5 minutes TTL
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	pending := &domain.PendingRegistration{
		Email:         email,
		FullName:      fullName,
		Username:      username,
		ContactNumber: contact,
		PasswordHash:  passwordHash,
		CodeHash:      codeHash,
		CodeExpiresAt: expiresAt,
		Attempts:      0,
		ResendCount:   0,
		LastSentAt:    time.Now(),
	}

	// Save to DB
	if err := u.repo.SavePendingRegistration(ctx, pending); err != nil {
		return nil, err
	}

	// Send Email
	err = utils.SendOTPEmail(
		email,
		rawCode,
		fullName,
		"register",
		u.cfg.DefaultFromEmail,
		u.cfg.EmailHost,
		u.cfg.EmailPort,
		u.cfg.EmailHostUser,
		u.cfg.EmailHostPassword,
		ttlSeconds,
	)
	if err != nil {
		// Log error but do not fail completely (as Django fails silently/returns email send failed error)
		return map[string]interface{}{"code": "email_send_failed", "message": "We could not send the verification email. Please try again in a moment."}, nil
	}

	return map[string]interface{}{
		"email":              email,
		"full_name":          fullName,
		"expires_in_seconds": ttlSeconds,
	}, nil
}

func (u *userUsecase) VerifyOTP(ctx context.Context, email, code string) (map[string]interface{}, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	pending, err := u.repo.GetPendingRegistration(ctx, email)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return map[string]interface{}{"code": "otp_no_pending_registration", "message": "No pending registration found for this email."}, nil
	}

	if pending.CodeExpiresAt.Before(time.Now()) {
		_ = u.repo.DeletePendingRegistration(ctx, email)
		return map[string]interface{}{"code": "otp_expired", "message": "Verification code has expired. Please request a new one."}, nil
	}

	maxAttempts := 3
	if pending.Attempts >= maxAttempts {
		return map[string]interface{}{"code": "otp_too_many_attempts", "message": "Too many incorrect attempts. Please request a new code."}, nil
	}

	ok, err := utils.VerifyDjangoPassword(code, pending.CodeHash)
	if err != nil || !ok {
		pending.Attempts++
		_ = u.repo.SavePendingRegistration(ctx, pending)
		attemptsRemaining := maxAttempts - pending.Attempts
		if attemptsRemaining < 0 {
			attemptsRemaining = 0
		}
		return map[string]interface{}{
			"code":    "otp_invalid",
			"message": "Verification code is incorrect.",
			"data":    map[string]interface{}{"attempts_remaining": attemptsRemaining},
		}, nil
	}

	// Promoted to Student
	user, err := u.repo.CreateUserFromPending(ctx, pending)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "users_email_key") {
			_ = u.repo.DeletePendingRegistration(ctx, email)
			return map[string]interface{}{"code": "email_taken", "message": "Email is already in use."}, nil
		}
		return nil, err
	}

	// Issue simple_jwt tokens
	tokens, err := utils.GenerateTokenPair(
		ctx, u.rdb, "student", user.ID, "student", "", u.cfg.SecretKey, 18000, 31536000, // Access 300 mins (18000s), Refresh 365 days
	)
	if err != nil {
		return nil, err
	}

	profile, err := u.compileStudentProfile(ctx, user)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"user":          profile,
		"access_token":  tokens["access_token"],
		"refresh_token": tokens["refresh_token"],
	}, nil
}

func (u *userUsecase) ResendOTP(ctx context.Context, email string) (map[string]interface{}, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	pending, err := u.repo.GetPendingRegistration(ctx, email)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return map[string]interface{}{"code": "otp_no_pending_registration", "message": "No pending registration found for this email."}, nil
	}

	// Check Cooldown (60 seconds)
	cooldownSec := 60
	elapsed := time.Since(pending.LastSentAt).Seconds()
	remaining := cooldownSec - int(elapsed)
	if remaining > 0 {
		return map[string]interface{}{
			"code":    "otp_resend_cooldown",
			"message": "Please wait before requesting another code.",
			"data":    map[string]interface{}{"cooldown_seconds": remaining},
		}, nil
	}

	maxResends := 5
	if pending.ResendCount >= maxResends {
		return map[string]interface{}{"code": "otp_resend_limit", "message": "Resend limit reached. Restart registration."}, nil
	}

	// Update OTP
	rawCode := generateNumericCode(6)
	codeSalt := generateSalt(12)
	pending.CodeHash = utils.EncodeDjangoPassword(rawCode, codeSalt, 390000)
	ttlSeconds := 300
	pending.CodeExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	pending.Attempts = 0
	pending.LastSentAt = time.Now()
	pending.ResendCount++

	if err := u.repo.SavePendingRegistration(ctx, pending); err != nil {
		return nil, err
	}

	// Send email
	err = utils.SendOTPEmail(
		email,
		rawCode,
		pending.FullName,
		"register",
		u.cfg.DefaultFromEmail,
		u.cfg.EmailHost,
		u.cfg.EmailPort,
		u.cfg.EmailHostUser,
		u.cfg.EmailHostPassword,
		ttlSeconds,
	)
	if err != nil {
		return map[string]interface{}{"code": "email_send_failed", "message": "We could not send the verification email. Please try again in a moment."}, nil
	}

	return map[string]interface{}{
		"email":              email,
		"expires_in_seconds": ttlSeconds,
	}, nil
}

func (u *userUsecase) Login(ctx context.Context, emailOrUsername, password, remoteIP string) (map[string]interface{}, error) {
	emailOrUsername = strings.TrimSpace(emailOrUsername)

	// 1. Try resolving as Student (User)
	var studentUser *domain.User
	var err error
	if strings.Contains(emailOrUsername, "@") {
		studentUser, err = u.repo.GetUserByEmail(ctx, emailOrUsername)
	} else {
		studentUser, err = u.repo.GetUserByUsername(ctx, emailOrUsername)
	}
	if err != nil {
		return nil, err
	}

	if studentUser != nil {
		ok, err := utils.VerifyDjangoPassword(password, studentUser.Password)
		if err == nil && ok {
			tokens, err := utils.GenerateTokenPair(
				ctx, u.rdb, "student", studentUser.ID, "student", "", u.cfg.SecretKey, 18000, 31536000,
			)
			if err != nil {
				return nil, err
			}

			profile, err := u.compileStudentProfile(ctx, studentUser)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"user":          profile,
				"access_token":  tokens["access_token"],
				"refresh_token": tokens["refresh_token"],
			}, nil
		}
	}

	// 2. Try resolving as Admin or Teacher
	if strings.Contains(emailOrUsername, "@") {
		// Admin
		adminUser, err := u.repo.GetAdminByEmail(ctx, emailOrUsername)
		if err == nil && adminUser != nil {
			ok, err := utils.VerifyDjangoPassword(password, adminUser.Password)
			if err == nil && ok {
				tokens, err := utils.GenerateTokenPair(
					ctx, u.rdb, "admin", adminUser.ID, "admin", "", u.cfg.SecretKey, 18000, 31536000,
				)
				if err != nil {
					return nil, err
				}

				return map[string]interface{}{
					"user": map[string]interface{}{
						"id":        adminUser.ID,
						"email":     adminUser.Email,
						"role":      "admin",
						"fullName":  strings.Title(strings.Replace(strings.Split(adminUser.Email, "@")[0], ".", " ", -1)),
						"createdAt": adminUser.CreatedAt,
					},
					"access_token":  tokens["access_token"],
					"refresh_token": tokens["refresh_token"],
				}, nil
			}
		}

		// Teacher
		teacherUser, err := u.repo.GetTeacherByEmail(ctx, emailOrUsername)
		if err == nil && teacherUser != nil {
			ok, err := utils.VerifyDjangoPassword(password, teacherUser.Password)
			if err == nil && ok {
				tokens, err := utils.GenerateTokenPair(
					ctx, u.rdb, "teacher", teacherUser.ID, "teacher", "", u.cfg.SecretKey, 18000, 31536000,
				)
				if err != nil {
					return nil, err
				}

				return map[string]interface{}{
					"user": map[string]interface{}{
						"id":             teacherUser.ID,
						"email":          teacherUser.Email,
						"role":           "teacher",
						"fullName":       teacherUser.Name,
						"specialization": teacherUser.Specialization,
						"photoUrl":       teacherUser.PhotoURL,
						"createdAt":      teacherUser.CreatedAt,
					},
					"access_token":  tokens["access_token"],
					"refresh_token": tokens["refresh_token"],
				}, nil
			}
		}
	}

	return nil, errors.New("Authentication failed. Invalid email or password.")
}

func (u *userUsecase) ForgotPassword(ctx context.Context, email string) (map[string]interface{}, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Verify user exists (either student or worker)
	userExists := false
	var name string

	student, err := u.repo.GetUserByEmail(ctx, email)
	if err == nil && student != nil {
		userExists = true
		name = student.FullName
	} else {
		admin, err := u.repo.GetAdminByEmail(ctx, email)
		if err == nil && admin != nil {
			userExists = true
			name = "Admin"
		} else {
			teacher, err := u.repo.GetTeacherByEmail(ctx, email)
			if err == nil && teacher != nil {
				userExists = true
				name = teacher.Name
			}
		}
	}

	if !userExists {
		return map[string]interface{}{"code": "email_not_found", "message": "No account associated with this email address."}, nil
	}

	// Create Reset OTP
	rawCode := generateNumericCode(6)
	codeSalt := generateSalt(12)
	codeHash := utils.EncodeDjangoPassword(rawCode, codeSalt, 390000)

	ttlSeconds := 300
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	reset := &domain.PasswordResetOTP{
		Email:         email,
		CodeHash:      codeHash,
		CodeExpiresAt: expiresAt,
		Attempts:      0,
		ResendCount:   0,
		LastSentAt:    time.Now(),
	}

	if err := u.repo.SavePasswordResetOTP(ctx, reset); err != nil {
		return nil, err
	}

	err = utils.SendOTPEmail(
		email,
		rawCode,
		name,
		"reset",
		u.cfg.DefaultFromEmail,
		u.cfg.EmailHost,
		u.cfg.EmailPort,
		u.cfg.EmailHostUser,
		u.cfg.EmailHostPassword,
		ttlSeconds,
	)
	if err != nil {
		return map[string]interface{}{"code": "email_send_failed", "message": "We could not send the reset email. Please try again."}, nil
	}

	return map[string]interface{}{
		"email":              email,
		"expires_in_seconds": ttlSeconds,
	}, nil
}

func (u *userUsecase) ResetPassword(ctx context.Context, email, code, newPassword string) (map[string]interface{}, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	reset, err := u.repo.GetPasswordResetOTP(ctx, email)
	if err != nil {
		return nil, err
	}
	if reset == nil {
		return map[string]interface{}{"code": "reset_no_pending_request", "message": "No password reset request found for this email."}, nil
	}

	if reset.CodeExpiresAt.Before(time.Now()) {
		_ = u.repo.DeletePasswordResetOTP(ctx, email)
		return map[string]interface{}{"code": "reset_expired", "message": "Reset code has expired. Please request a new one."}, nil
	}

	maxAttempts := 3
	if reset.Attempts >= maxAttempts {
		return map[string]interface{}{"code": "reset_too_many_attempts", "message": "Too many incorrect attempts. Please request a new code."}, nil
	}

	ok, err := utils.VerifyDjangoPassword(code, reset.CodeHash)
	if err != nil || !ok {
		reset.Attempts++
		_ = u.repo.SavePasswordResetOTP(ctx, reset)
		attemptsRemaining := maxAttempts - reset.Attempts
		if attemptsRemaining < 0 {
			attemptsRemaining = 0
		}
		return map[string]interface{}{
			"code":    "reset_invalid",
			"message": "Reset code is incorrect.",
			"data":    map[string]interface{}{"attempts_remaining": attemptsRemaining},
		}, nil
	}

	// Update Password
	salt := generateSalt(12)
	newPasswordHash := utils.EncodeDjangoPassword(newPassword, salt, 390000)

	// Update password for whatever entity matches
	student, err := u.repo.GetUserByEmail(ctx, email)
	if err == nil && student != nil {
		student.Password = newPasswordHash
		_ = u.repo.UpdateUser(ctx, student)
	} else {
		admin, err := u.repo.GetAdminByEmail(ctx, email)
		if err == nil && admin != nil {
			_ = u.repo.UpdateAdminPassword(ctx, admin.ID, newPasswordHash)
		} else {
			teacher, err := u.repo.GetTeacherByEmail(ctx, email)
			if err == nil && teacher != nil {
				_ = u.repo.UpdateTeacherPassword(ctx, teacher.ID, newPasswordHash)
			}
		}
	}

	_ = u.repo.DeletePasswordResetOTP(ctx, email)

	return map[string]interface{}{
		"message": "Password has been reset successfully.",
	}, nil
}

func (u *userUsecase) GetMe(ctx context.Context, userID int64, role string) (map[string]interface{}, error) {
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

func (u *userUsecase) UpdateMe(ctx context.Context, userID int64, updates map[string]interface{}) (map[string]interface{}, error) {
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
	if val, ok := updates["emailAlerts"].(bool); ok {
		user.EmailAlerts = val
	}
	if val, ok := updates["directMessages"].(bool); ok {
		user.DirectMessages = val
	}
	if val, ok := updates["feedUpdates"].(bool); ok {
		user.FeedUpdates = val
	}
	if val, ok := updates["securityAlerts"].(bool); ok {
		user.SecurityAlerts = val
	}

	if err := u.repo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return u.compileStudentProfile(ctx, user)
}

func (u *userUsecase) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := u.repo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("User not found.")
	}

	ok, err := utils.VerifyDjangoPassword(oldPassword, user.Password)
	if err != nil || !ok {
		return errors.New("Old password is incorrect.")
	}

	salt := generateSalt(12)
	newHash := utils.EncodeDjangoPassword(newPassword, salt, 390000)
	user.Password = newHash

	return u.repo.UpdateUser(ctx, user)
}

func (u *userUsecase) SearchUsers(ctx context.Context, query string) ([]map[string]interface{}, error) {
	users, err := u.repo.SearchUsers(ctx, query)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(users))
	for i, usr := range users {
		result[i] = map[string]interface{}{
			"id":        usr.ID,
			"name":      usr.FullName,
			"email":     usr.Email,
			"username":  usr.Username,
			"avatarUrl": usr.AvatarURL,
			"bio":       usr.Bio,
			"headline":  usr.Headline,
		}
	}
	return result, nil
}

func (u *userUsecase) FollowUser(ctx context.Context, followerID, followedID int64) error {
	if followerID == followedID {
		return errors.New("You cannot follow yourself.")
	}

	existing, err := u.repo.GetFollowRelationship(ctx, followerID, followedID)
	if err == nil && existing != nil {
		return errors.New("You are already following this user.")
	}

	err = u.repo.FollowUser(ctx, followerID, followedID)
	if err != nil {
		return err
	}

	// Trigger notifications
	follower, _ := u.repo.GetUserByID(ctx, followerID)
	msg := fmt.Sprintf("%s started following you.", follower.FullName)
	notif := &domain.UserNotification{
		RecipientID:      followedID,
		SenderID:         &followerID,
		NotificationType: "follow",
		Message:          msg,
		IsRead:           false,
	}
	_ = u.repo.CreateNotification(ctx, notif)

	return nil
}

func (u *userUsecase) UnfollowUser(ctx context.Context, followerID, followedID int64) error {
	return u.repo.UnfollowUser(ctx, followerID, followedID)
}

func (u *userUsecase) GetNotifications(ctx context.Context, userID int64, role string) ([]domain.UserNotification, error) {
	return u.repo.GetNotifications(ctx, userID, role)
}

func (u *userUsecase) MarkNotificationsAsRead(ctx context.Context, userID int64, role string) error {
	return u.repo.MarkNotificationsAsRead(ctx, userID, role)
}

// Compile complete StudentProfile serializable response
func (u *userUsecase) compileStudentProfile(ctx context.Context, user *domain.User) (map[string]interface{}, error) {
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

func (u *userUsecase) TokenRefresh(ctx context.Context, refreshToken string) (map[string]string, error) {
	claims, err := utils.VerifyToken(refreshToken, u.cfg.SecretKey)
	if err != nil {
		return nil, errors.New("Token is invalid or expired.")
	}

	if claims.TokenType != "refresh" {
		return nil, errors.New("Token is not a refresh token.")
	}

	// Issue new token pair using the old token's JTI to enforce session limit replacement
	newTokens, err := utils.GenerateTokenPair(
		ctx, u.rdb, claims.SubKind, claims.SubID, claims.Role, claims.ID, u.cfg.SecretKey, 18000, 31536000,
	)
	if err != nil {
		return nil, err
	}

	return newTokens, nil
}

