package repository

import (
	"context"
	"errors"
	"strings"
	"time"

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
			// Check if the user exists in the users table first to prevent foreign key constraint violation
			var count int64
			if err := r.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; err != nil {
				return nil, err
			}
			if count == 0 {
				return nil, nil
			}

			// Create the student profile on the fly
			student = domain.Student{
				UserID: userID,
			}
			if err := r.db.WithContext(ctx).Create(&student).Error; err != nil {
				// Handle potential race conditions by trying to fetch one more time
				if err2 := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&student).Error; err2 == nil {
					return &student, nil
				}
				return nil, err
			}
			return &student, nil
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
	var enrollments []domain.Enrollment
	if err := r.db.WithContext(ctx).Where("student_id = ? AND course_id = ?", studentID, courseID).Limit(1).Find(&enrollments).Error; err != nil {
		return nil, err
	}
	if len(enrollments) == 0 {
		return nil, nil
	}
	return &enrollments[0], nil
}

func (r *postgresEnrollmentRepository) CreateEnrollment(ctx context.Context, enrollment *domain.Enrollment) error {
	return r.db.WithContext(ctx).Create(enrollment).Error
}

func (r *postgresEnrollmentRepository) DeleteEnrollment(ctx context.Context, studentID, courseID int64) error {
	return r.db.WithContext(ctx).Where("student_id = ? AND course_id = ?", studentID, courseID).Delete(&domain.Enrollment{}).Error
}

func (r *postgresEnrollmentRepository) GetNoteAccess(ctx context.Context, studentID, noteID int64) (*domain.NoteAccess, error) {
	var accesses []domain.NoteAccess
	if err := r.db.WithContext(ctx).Where("student_id = ? AND note_id = ?", studentID, noteID).Limit(1).Find(&accesses).Error; err != nil {
		return nil, err
	}
	if len(accesses) == 0 {
		return nil, nil
	}
	return &accesses[0], nil
}

func (r *postgresEnrollmentRepository) CreateNoteAccess(ctx context.Context, access *domain.NoteAccess) error {
	return r.db.WithContext(ctx).Create(access).Error
}

func (r *postgresEnrollmentRepository) DeleteNoteAccess(ctx context.Context, studentID, noteID int64) error {
	return r.db.WithContext(ctx).Where("student_id = ? AND note_id = ?", studentID, noteID).Delete(&domain.NoteAccess{}).Error
}

func (r *postgresEnrollmentRepository) GetTestSeriesAccess(ctx context.Context, studentID, testSeriesID int64) (*domain.TestSeriesAccess, error) {
	var accesses []domain.TestSeriesAccess
	if err := r.db.WithContext(ctx).Where("student_id = ? AND test_series_id = ?", studentID, testSeriesID).Limit(1).Find(&accesses).Error; err != nil {
		return nil, err
	}
	if len(accesses) == 0 {
		return nil, nil
	}
	return &accesses[0], nil
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

func (r *postgresEnrollmentRepository) GetMyEnrollments(ctx context.Context, studentID int64, category string) ([]map[string]interface{}, error) {
	type Result struct {
		EnrollmentID   int64     `gorm:"column:enrollment_id"`
		CreatedAt      time.Time `gorm:"column:created_at"`
		CourseID       int64     `gorm:"column:course_id"`
		CourseName     string    `gorm:"column:course_name"`
		BatchID        string    `gorm:"column:batch_id"`
		Category       string    `gorm:"column:category"`
		BannerURL      string    `gorm:"column:banner_url"`
		FinalPrice     float64   `gorm:"column:final_price"`
		TeacherID      *int64    `gorm:"column:teacher_id"`
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("enrollments e").
		Select("e.id AS enrollment_id, e.created_at, c.id AS course_id, c.course_name, c.batch_id, c.category, c.banner_url, c.final_price, c.teacher_id").
		Joins("JOIN courses c ON e.course_id = c.id").
		Where("e.student_id = ?", studentID)

	if category != "" {
		query = query.Where("c.category = ?", category)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	// Pre-fetch teacher details to avoid N+1 query problem
	var courseIDs []int64
	var directTeacherIDs []int64
	for _, res := range results {
		courseIDs = append(courseIDs, res.CourseID)
		if res.TeacherID != nil {
			directTeacherIDs = append(directTeacherIDs, *res.TeacherID)
		}
	}

	type CourseTeacher struct {
		CourseID int64  `gorm:"column:course_id"`
		Name     string `gorm:"column:name"`
	}
	var ctResults []CourseTeacher
	if len(courseIDs) > 0 {
		_ = r.db.WithContext(ctx).Table("teachers t").
			Select("ct.course_id, t.name").
			Joins("JOIN courses_teachers ct ON t.id = ct.teacher_id").
			Where("ct.course_id IN ?", courseIDs).
			Find(&ctResults).Error
	}

	type DirectTeacher struct {
		ID   int64  `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	var dtResults []DirectTeacher
	if len(directTeacherIDs) > 0 {
		_ = r.db.WithContext(ctx).Table("teachers").
			Select("id, name").
			Where("id IN ?", directTeacherIDs).
			Find(&dtResults).Error
	}

	// Build lookup maps
	m2mTeachers := make(map[int64][]string)
	for _, ct := range ctResults {
		m2mTeachers[ct.CourseID] = append(m2mTeachers[ct.CourseID], ct.Name)
	}

	directTeachers := make(map[int64]string)
	for _, dt := range dtResults {
		directTeachers[dt.ID] = dt.Name
	}

	data := make([]map[string]interface{}, 0)
	for _, res := range results {
		mentorName := "Instructor"

		// 1. Check direct teacher_id first
		if res.TeacherID != nil {
			if name, ok := directTeachers[*res.TeacherID]; ok && name != "" {
				mentorName = name
			}
		}

		// 2. Check ManyToMany course_teachers
		if names, ok := m2mTeachers[res.CourseID]; ok && len(names) > 0 {
			mentorName = strings.Join(names, ", ")
		}

		enrolledOn := "Recently"
		if !res.CreatedAt.IsZero() {
			enrolledOn = res.CreatedAt.Format("January 02, 2006")
		}

		item := map[string]interface{}{
			"id":               res.EnrollmentID,
			"slug":             res.BatchID,
			"title":            res.CourseName,
			"category":         res.Category,
			"level":            "All levels",
			"mentor":           mentorName,
			"enrolledOn":       enrolledOn,
			"lastAccessed":     "Just now",
			"progressPercent":  0,
			"modulesCompleted": 0,
			"modulesTotal":     12,
			"nextLesson":       "Introduction Session",
			"batchId":          res.BatchID,
			"bannerUrl":        res.BannerURL,
			"finalPrice":       res.FinalPrice,
		}
		data = append(data, item)
	}

	return data, nil
}
