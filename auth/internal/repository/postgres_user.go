package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"clasynq/api/auth/internal/domain"

	"gorm.io/gorm"
)

type postgresUserRepository struct {
	db *gorm.DB
}

func NewPostgresUserRepository(db *gorm.DB) domain.UserRepository {
	return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("LOWER(username) = ?", strings.ToLower(username)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepository) GetUserByContact(ctx context.Context, contact string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("contact_number = ?", contact).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepository) SearchUsers(ctx context.Context, query string) ([]domain.User, error) {
	var users []domain.User
	q := "%" + strings.ToLower(query) + "%"
	err := r.db.WithContext(ctx).
		Where("LOWER(full_name) LIKE ? OR LOWER(username) LIKE ? OR LOWER(email) LIKE ?", q, q, q).
		Limit(50).
		Find(&users).Error
	return users, err
}

func (r *postgresUserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *postgresUserRepository) IsStudent(ctx context.Context, userID int64) (bool, error) {
	var studentID int64
	err := r.db.WithContext(ctx).Table("students").Where("user_id = ?", userID).Pluck("id", &studentID).Error
	if err != nil {
		return false, err
	}
	if studentID == 0 {
		return false, nil
	}

	var enrollCount int64
	if err := r.db.WithContext(ctx).Table("enrollments").Where("student_id = ?", studentID).Count(&enrollCount).Error; err != nil {
		return false, err
	}

	var noteCount int64
	if err := r.db.WithContext(ctx).Table("note_accesses").Where("student_id = ?", studentID).Count(&noteCount).Error; err != nil {
		return false, err
	}

	var testCount int64
	if err := r.db.WithContext(ctx).Table("test_series_accesses").Where("student_id = ?", studentID).Count(&testCount).Error; err != nil {
		return false, err
	}

	return (enrollCount + noteCount + testCount) > 0, nil
}

func (r *postgresUserRepository) GetStudentProfile(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Preload("User").Where("user_id = ?", userID).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

func (r *postgresUserRepository) GetStudentReferralsCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("payment_orders").
		Where("referrer_id = ? AND status = 'completed'", userID).
		Distinct("user_id").
		Count(&count).Error
	if err != nil {
		return 0, nil
	}
	return count, nil
}

func (r *postgresUserRepository) GetAdminByEmail(ctx context.Context, email string) (*domain.Admin, error) {
	var admin domain.Admin
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

func (r *postgresUserRepository) GetTeacherByEmail(ctx context.Context, email string) (*domain.Teacher, error) {
	var teacher domain.Teacher
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&teacher).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (r *postgresUserRepository) GetTeacherByID(ctx context.Context, id int64) (*domain.Teacher, error) {
	var teacher domain.Teacher
	if err := r.db.WithContext(ctx).First(&teacher, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (r *postgresUserRepository) GetAdminByID(ctx context.Context, id int64) (*domain.Admin, error) {
	var admin domain.Admin
	if err := r.db.WithContext(ctx).First(&admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

func (r *postgresUserRepository) GetPendingRegistration(ctx context.Context, email string) (*domain.PendingRegistration, error) {
	var pending domain.PendingRegistration
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&pending).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pending, nil
}

func (r *postgresUserRepository) SavePendingRegistration(ctx context.Context, pending *domain.PendingRegistration) error {
	pending.Email = strings.ToLower(pending.Email)
	pending.Username = strings.ToLower(pending.Username)
	return r.db.WithContext(ctx).Save(pending).Error
}

func (r *postgresUserRepository) DeletePendingRegistration(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).Delete(&domain.PendingRegistration{}).Error
}

func generateReferralCode(tx *gorm.DB) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for {
		result := make([]byte, 8)
		for i := 0; i < 8; i++ {
			num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			result[i] = charset[num.Int64()]
		}
		code := "CSQ-" + string(result)
		var count int64
		if err := tx.Model(&domain.User{}).Where("referral_code = ?", code).Count(&count).Error; err == nil && count == 0 {
			return code
		}
	}
}

func (r *postgresUserRepository) CreateUserFromPending(ctx context.Context, pending *domain.PendingRegistration) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user = domain.User{
			FullName:      pending.FullName,
			Username:      strings.ToLower(pending.Username),
			ContactNumber: pending.ContactNumber,
			Email:         strings.ToLower(pending.Email),
			Password:      pending.PasswordHash,
			ReferralCode:  generateReferralCode(tx),
		}

		// Save User
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		// Save Student profile
		student := domain.Student{
			UserID: user.ID,
		}
		if err := tx.Create(&student).Error; err != nil {
			return err
		}

		// Delete pending registration
		if err := tx.Where("LOWER(email) = ?", strings.ToLower(pending.Email)).Delete(&domain.PendingRegistration{}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *postgresUserRepository) GetPasswordResetOTP(ctx context.Context, email string) (*domain.PasswordResetOTP, error) {
	var otp domain.PasswordResetOTP
	if err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).First(&otp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &otp, nil
}

func (r *postgresUserRepository) SavePasswordResetOTP(ctx context.Context, reset *domain.PasswordResetOTP) error {
	reset.Email = strings.ToLower(reset.Email)
	return r.db.WithContext(ctx).Save(reset).Error
}

func (r *postgresUserRepository) DeletePasswordResetOTP(ctx context.Context, email string) error {
	return r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(email)).Delete(&domain.PasswordResetOTP{}).Error
}

func (r *postgresUserRepository) GetFollowRelationship(ctx context.Context, followerID, followedID int64) (*domain.Follow, error) {
	var follow domain.Follow
	if err := r.db.WithContext(ctx).Where("follower_id = ? AND followed_id = ?", followerID, followedID).First(&follow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &follow, nil
}

func (r *postgresUserRepository) FollowUser(ctx context.Context, followerID, followedID int64) error {
	follow := domain.Follow{
		FollowerID: followerID,
		FollowedID: followedID,
	}
	return r.db.WithContext(ctx).Create(&follow).Error
}

func (r *postgresUserRepository) UnfollowUser(ctx context.Context, followerID, followedID int64) error {
	return r.db.WithContext(ctx).Where("follower_id = ? AND followed_id = ?", followerID, followedID).Delete(&domain.Follow{}).Error
}

func (r *postgresUserRepository) GetFollowersList(ctx context.Context, userID int64) ([]domain.Follow, error) {
	var follows []domain.Follow
	err := r.db.WithContext(ctx).Preload("Follower").Where("followed_id = ?", userID).Find(&follows).Error
	return follows, err
}

func (r *postgresUserRepository) GetFollowingList(ctx context.Context, userID int64) ([]domain.Follow, error) {
	var follows []domain.Follow
	err := r.db.WithContext(ctx).Preload("Followed").Where("follower_id = ?", userID).Find(&follows).Error
	return follows, err
}
func (r *postgresUserRepository) GetNotifications(ctx context.Context, userID int64, role string) ([]domain.UserNotification, error) {
	var notifications []domain.UserNotification
	var err error
	if role == "student" || role == "user" {
		err = r.db.WithContext(ctx).Preload("Sender").Where("recipient_id = ? AND recipient_role IN ('student', 'user')", userID).Order("created_at desc").Find(&notifications).Error
	} else {
		err = r.db.WithContext(ctx).Preload("Sender").Where("recipient_id = ? AND recipient_role = ?", userID, role).Order("created_at desc").Find(&notifications).Error
	}
	if err != nil {
		return nil, err
	}

	for i := range notifications {
		if notifications[i].SenderID != nil {
			senderID := *notifications[i].SenderID
			if notifications[i].NotificationType == "notice" {
				// Notices are always sent by teachers. Retrieve details from teachers table directly to avoid ID clashes with users.
				var teacher struct {
					Name     string
					PhotoURL string
				}
				if errT := r.db.WithContext(ctx).Table("teachers").Select("name, photo_url").Where("id = ?", senderID).First(&teacher).Error; errT == nil {
					notifications[i].SenderName = teacher.Name
					notifications[i].SenderAvatarUrl = teacher.PhotoURL
				} else {
					notifications[i].SenderName = "Teacher"
				}
			} else if notifications[i].Sender != nil {
				notifications[i].SenderName = notifications[i].Sender.FullName
				notifications[i].SenderAvatarUrl = notifications[i].Sender.AvatarURL
			} else {
				// Try teachers table
				var teacher struct {
					Name     string
					PhotoURL string
				}
				if errT := r.db.WithContext(ctx).Table("teachers").Select("name, photo_url").Where("id = ?", senderID).First(&teacher).Error; errT == nil {
					notifications[i].SenderName = teacher.Name
					notifications[i].SenderAvatarUrl = teacher.PhotoURL
				} else {
					// Try admin table
					var admin struct {
						Email string
					}
					if errA := r.db.WithContext(ctx).Table("admin").Select("email").Where("id = ?", senderID).First(&admin).Error; errA == nil {
						notifications[i].SenderName = "Admin"
						if admin.Email != "" {
							parts := strings.Split(admin.Email, "@")
							notifications[i].SenderName = strings.Title(strings.Replace(parts[0], ".", " ", -1))
						}
					}
				}
			}
		}
	}

	return notifications, nil
}

func (r *postgresUserRepository) MarkNotificationsAsRead(ctx context.Context, userID int64, role string) error {
	if role == "student" || role == "user" {
		return r.db.WithContext(ctx).Model(&domain.UserNotification{}).Where("recipient_id = ? AND recipient_role IN ('student', 'user')", userID).Update("is_read", true).Error
	}
	return r.db.WithContext(ctx).Model(&domain.UserNotification{}).Where("recipient_id = ? AND recipient_role = ?", userID, role).Update("is_read", true).Error
}

func (r *postgresUserRepository) CreateNotification(ctx context.Context, notif *domain.UserNotification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *postgresUserRepository) UpdateAdminPassword(ctx context.Context, id int64, newHash string) error {
	return r.db.WithContext(ctx).Model(&domain.Admin{}).Where("id = ?", id).Update("password", newHash).Error
}

func (r *postgresUserRepository) UpdateTeacherPassword(ctx context.Context, id int64, newHash string) error {
	return r.db.WithContext(ctx).Model(&domain.Teacher{}).Where("id = ?", id).Update("password", newHash).Error
}

