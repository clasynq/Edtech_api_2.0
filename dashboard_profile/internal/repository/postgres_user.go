package repository

import (
	"context"
	"errors"
	"time"

	"clasynq/api/dashboard_profile/internal/domain"

	"gorm.io/gorm"
)

type postgresProfileRepository struct {
	db *gorm.DB
}

func NewPostgresProfileRepository(db *gorm.DB) domain.ProfileRepository {
	return &postgresProfileRepository{db: db}
}

func (r *postgresProfileRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresProfileRepository) GetStudentReferralsCount(ctx context.Context, userID int64) (int64, error) {
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

func (r *postgresProfileRepository) GetFollowersList(ctx context.Context, userID int64) ([]domain.Follow, error) {
	var follows []domain.Follow
	err := r.db.WithContext(ctx).Preload("Follower").Where("followed_id = ?", userID).Find(&follows).Error
	return follows, err
}

func (r *postgresProfileRepository) GetFollowingList(ctx context.Context, userID int64) ([]domain.Follow, error) {
	var follows []domain.Follow
	err := r.db.WithContext(ctx).Preload("Followed").Where("follower_id = ?", userID).Find(&follows).Error
	return follows, err
}

func (r *postgresProfileRepository) GetFollowRelationship(ctx context.Context, followerID, followedID int64) (*domain.Follow, error) {
	var follow domain.Follow
	if err := r.db.WithContext(ctx).Where("follower_id = ? AND followed_id = ?", followerID, followedID).First(&follow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &follow, nil
}

func (r *postgresProfileRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *postgresProfileRepository) GetTeacherByID(ctx context.Context, id int64) (*domain.Teacher, error) {
	var teacher domain.Teacher
	if err := r.db.WithContext(ctx).First(&teacher, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (r *postgresProfileRepository) GetAdminByID(ctx context.Context, id int64) (*domain.Admin, error) {
	var admin domain.Admin
	if err := r.db.WithContext(ctx).First(&admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &admin, nil
}

func (r *postgresProfileRepository) ToggleFollowUser(ctx context.Context, followerID, followedID int64) (bool, error) {
	var follow domain.Follow
	err := r.db.WithContext(ctx).Where("follower_id = ? AND followed_id = ?", followerID, followedID).First(&follow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newFollow := domain.Follow{
				FollowerID: followerID,
				FollowedID: followedID,
			}
			if err := r.db.WithContext(ctx).Create(&newFollow).Error; err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}
	if err := r.db.WithContext(ctx).Delete(&follow).Error; err != nil {
		return false, err
	}
	return false, nil
}

func (r *postgresProfileRepository) GetMutualConnections(ctx context.Context, userID int64) ([]domain.MutualConnection, error) {
	type result struct {
		FollowedID  int64
		MutualCount int
	}
	var list []result

	// Query mutual connections
	err := r.db.WithContext(ctx).Raw(`
		SELECT f2.followed_id, COUNT(f2.follower_id) as mutual_count
		FROM user_follows f1
		JOIN user_follows f2 ON f1.followed_id = f2.follower_id
		WHERE f1.follower_id = ? 
		  AND f2.followed_id != ? 
		  AND f2.followed_id NOT IN (SELECT followed_id FROM user_follows WHERE follower_id = ?)
		GROUP BY f2.followed_id
		ORDER BY mutual_count DESC
		LIMIT 10
	`, userID, userID, userID).Scan(&list).Error
	if err != nil {
		return nil, err
	}

	var candidateIDs []int64
	mutualCountsMap := make(map[int64]int)
	for _, item := range list {
		candidateIDs = append(candidateIDs, item.FollowedID)
		mutualCountsMap[item.FollowedID] = item.MutualCount
	}

	// Complement with other recent/popular users if < 5 suggestions
	if len(candidateIDs) < 5 {
		var popularIDs []int64
		limitVal := 10 - len(candidateIDs)

		// Query popular/recent users not followed by user
		subQuery := r.db.WithContext(ctx).Table("user_follows").Select("followed_id").Where("follower_id = ?", userID)

		q := r.db.WithContext(ctx).Table("users").Select("id").
			Where("id != ?", userID).
			Where("id NOT IN (?)", subQuery)

		if len(candidateIDs) > 0 {
			q = q.Where("id NOT IN (?)", candidateIDs)
		}

		err = q.Order("created_at desc").Limit(limitVal).Pluck("id", &popularIDs).Error
		if err == nil {
			for _, pid := range popularIDs {
				candidateIDs = append(candidateIDs, pid)
				mutualCountsMap[pid] = 0
			}
		}
	}

	if len(candidateIDs) == 0 {
		return []domain.MutualConnection{}, nil
	}

	var users []domain.User
	if err := r.db.WithContext(ctx).Where("id IN (?)", candidateIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	res := make([]domain.MutualConnection, 0, len(users))
	for _, u := range users {
		res = append(res, domain.MutualConnection{
			User:        u,
			MutualCount: mutualCountsMap[u.ID],
		})
	}

	return res, nil
}

func (r *postgresProfileRepository) CreateActivityLog(ctx context.Context, log *domain.ActivityLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *postgresProfileRepository) CreateNotification(ctx context.Context, notif *domain.UserNotification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *postgresProfileRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

func (r *postgresProfileRepository) GetEnrollmentsByStudentID(ctx context.Context, studentID int64) ([]domain.Enrollment, error) {
	var enrollments []domain.Enrollment
	err := r.db.WithContext(ctx).Where("student_id = ?", studentID).Find(&enrollments).Error
	return enrollments, err
}

func (r *postgresProfileRepository) GetCoursesByIDs(ctx context.Context, courseIDs []int64, category string) ([]domain.Course, error) {
	var courses []domain.Course
	q := r.db.WithContext(ctx).Preload("Teachers").Preload("Subjects").Where("id IN (?)", courseIDs)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	err := q.Find(&courses).Error
	return courses, err
}

func (r *postgresProfileRepository) GetClassSchedulesByCourseIDsAndDateRange(ctx context.Context, courseIDs []int64, startDate, endDate time.Time) ([]domain.ClassSchedule, error) {
	var schedules []domain.ClassSchedule
	err := r.db.WithContext(ctx).
		Preload("Course").
		Preload("Course.Subjects").
		Preload("Teacher").
		Preload("Subject").
		Where("course_id IN (?) AND class_date >= ? AND class_date <= ?", courseIDs, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Order("class_date ASC, start_time ASC").
		Find(&schedules).Error
	return schedules, err
}

func (r *postgresProfileRepository) GetCompletedClassSchedulesByCourseIDs(ctx context.Context, courseIDs []int64) ([]domain.ClassSchedule, error) {
	var schedules []domain.ClassSchedule
	err := r.db.WithContext(ctx).
		Preload("Course").
		Preload("Course.Subjects").
		Preload("Teacher").
		Preload("Subject").
		Where("course_id IN (?) AND class_status = 'completed'", courseIDs).
		Order("class_date DESC, start_time DESC").
		Find(&schedules).Error
	return schedules, err
}
