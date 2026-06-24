package repository

import (
	"context"
	"errors"
	"strings"

	"clasynq/api/enrollments/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresEnrollmentRepository struct {
	db *gorm.DB
}

func NewPostgresEnrollmentRepository(db *gorm.DB) domain.EnrollmentRepository {
	return &postgresEnrollmentRepository{db: db}
}

func (r *postgresEnrollmentRepository) BeginTx(ctx context.Context) (domain.EnrollmentRepository, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &postgresEnrollmentRepository{db: tx}, nil
}

func (r *postgresEnrollmentRepository) CommitTx() error {
	return r.db.Commit().Error
}

func (r *postgresEnrollmentRepository) RollbackTx() error {
	return r.db.Rollback().Error
}

func (r *postgresEnrollmentRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresEnrollmentRepository) GetUserByReferralCode(ctx context.Context, code string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("LOWER(referral_code) = ?", strings.ToLower(code)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *postgresEnrollmentRepository) GetStudentByUserID(ctx context.Context, userID int64) (*domain.Student, error) {
	var student domain.Student
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

func (r *postgresEnrollmentRepository) UpdateUserCoins(ctx context.Context, userID int64, change int) error {
	// Lock the user record first within the transaction
	var user domain.User
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		return err
	}

	newBalance := user.CoinsBalance + change
	if newBalance < 0 {
		newBalance = 0
	}

	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("coins_balance", newBalance).Error
}

func (r *postgresEnrollmentRepository) GetCourseByID(ctx context.Context, id int64) (*domain.Course, error) {
	var course domain.Course
	if err := r.db.WithContext(ctx).First(&course, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &course, nil
}

func (r *postgresEnrollmentRepository) GetNoteByID(ctx context.Context, id int64) (*domain.Note, error) {
	var note domain.Note
	if err := r.db.WithContext(ctx).First(&note, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &note, nil
}

func (r *postgresEnrollmentRepository) GetTestSeriesByID(ctx context.Context, id int64) (*domain.TestSeries, error) {
	var ts domain.TestSeries
	if err := r.db.WithContext(ctx).First(&ts, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ts, nil
}

func (r *postgresEnrollmentRepository) GetEnrollment(ctx context.Context, studentID, courseID int64) (*domain.Enrollment, error) {
	var enrollment domain.Enrollment
	if err := r.db.WithContext(ctx).Where("student_id = ? AND course_id = ?", studentID, courseID).First(&enrollment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &enrollment, nil
}

func (r *postgresEnrollmentRepository) CreateEnrollment(ctx context.Context, enrollment *domain.Enrollment) error {
	return r.db.WithContext(ctx).Create(enrollment).Error
}

func (r *postgresEnrollmentRepository) DeleteEnrollment(ctx context.Context, studentID, courseID int64) error {
	return r.db.WithContext(ctx).Where("student_id = ? AND course_id = ?", studentID, courseID).Delete(&domain.Enrollment{}).Error
}

func (r *postgresEnrollmentRepository) GetNoteAccess(ctx context.Context, studentID, noteID int64) (*domain.NoteAccess, error) {
	var access domain.NoteAccess
	if err := r.db.WithContext(ctx).Where("student_id = ? AND note_id = ?", studentID, noteID).First(&access).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &access, nil
}

func (r *postgresEnrollmentRepository) CreateNoteAccess(ctx context.Context, access *domain.NoteAccess) error {
	return r.db.WithContext(ctx).Create(access).Error
}

func (r *postgresEnrollmentRepository) DeleteNoteAccess(ctx context.Context, studentID, noteID int64) error {
	return r.db.WithContext(ctx).Where("student_id = ? AND note_id = ?", studentID, noteID).Delete(&domain.NoteAccess{}).Error
}

func (r *postgresEnrollmentRepository) GetTestSeriesAccess(ctx context.Context, studentID, testSeriesID int64) (*domain.TestSeriesAccess, error) {
	var access domain.TestSeriesAccess
	if err := r.db.WithContext(ctx).Where("student_id = ? AND test_series_id = ?", studentID, testSeriesID).First(&access).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &access, nil
}

func (r *postgresEnrollmentRepository) CreateTestSeriesAccess(ctx context.Context, access *domain.TestSeriesAccess) error {
	return r.db.WithContext(ctx).Create(access).Error
}

func (r *postgresEnrollmentRepository) DeleteTestSeriesAccess(ctx context.Context, studentID, testSeriesID int64) error {
	return r.db.WithContext(ctx).Where("student_id = ? AND test_series_id = ?", studentID, testSeriesID).Delete(&domain.TestSeriesAccess{}).Error
}

func (r *postgresEnrollmentRepository) GetPaymentOrderByID(ctx context.Context, id int64) (*domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	if err := r.db.WithContext(ctx).First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *postgresEnrollmentRepository) GetPaymentOrderByRazorpayID(ctx context.Context, razorpayOrderID string) (*domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	if err := r.db.WithContext(ctx).Where("razorpay_order_id = ?", razorpayOrderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *postgresEnrollmentRepository) GetPaymentOrderByPaymentID(ctx context.Context, paymentID string) (*domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	if err := r.db.WithContext(ctx).Where("razorpay_payment_id = ?", paymentID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *postgresEnrollmentRepository) CreatePaymentOrder(ctx context.Context, order *domain.PaymentOrder) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *postgresEnrollmentRepository) UpdatePaymentOrder(ctx context.Context, order *domain.PaymentOrder) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *postgresEnrollmentRepository) HasUserCompletedOrderForReferrer(ctx context.Context, buyerID, referrerID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.PaymentOrder{}).
		Where("user_id = ? AND referrer_id = ? AND status = 'completed'", buyerID, referrerID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *postgresEnrollmentRepository) GetReferralTransactionByID(ctx context.Context, id int64) (*domain.ReferralTransaction, error) {
	var tx domain.ReferralTransaction
	if err := r.db.WithContext(ctx).First(&tx, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *postgresEnrollmentRepository) GetReferralTransactionByOrderID(ctx context.Context, orderID int64) (*domain.ReferralTransaction, error) {
	var tx domain.ReferralTransaction
	if err := r.db.WithContext(ctx).Where("payment_order_id = ?", orderID).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *postgresEnrollmentRepository) CreateReferralTransaction(ctx context.Context, tx *domain.ReferralTransaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *postgresEnrollmentRepository) UpdateReferralTransaction(ctx context.Context, tx *domain.ReferralTransaction) error {
	return r.db.WithContext(ctx).Save(tx).Error
}

func (r *postgresEnrollmentRepository) GetPendingReferralTransactions(ctx context.Context) ([]domain.ReferralTransaction, error) {
	var txs []domain.ReferralTransaction
	if err := r.db.WithContext(ctx).Where("status = ?", "pending_hold").Find(&txs).Error; err != nil {
		return nil, err
	}
	return txs, nil
}

func (r *postgresEnrollmentRepository) CountCompletedReferralsForReferrer(ctx context.Context, referrerID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("payment_orders").
		Where("referrer_id = ? AND status = 'completed'", referrerID).
		Distinct("user_id").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *postgresEnrollmentRepository) GetWebhookEventByID(ctx context.Context, eventID string) (*domain.WebhookEvent, error) {
	var event domain.WebhookEvent
	if err := r.db.WithContext(ctx).Where("event_id = ?", eventID).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (r *postgresEnrollmentRepository) CreateWebhookEvent(ctx context.Context, event *domain.WebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *postgresEnrollmentRepository) CreateAuditLog(ctx context.Context, log *domain.PaymentAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}
