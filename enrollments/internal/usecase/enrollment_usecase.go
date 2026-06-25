package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"clasynq/api/enrollments/internal/domain"
)

type enrollmentUsecase struct {
	repo           domain.EnrollmentRepository
	keyID          string
	keySecret      string
	webhookSecret  string
}

func NewEnrollmentUsecase(
	repo domain.EnrollmentRepository,
	keyID string,
	keySecret string,
	webhookSecret string,
) domain.EnrollmentUsecase {
	return &enrollmentUsecase{
		repo:          repo,
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
	}
}

func (u *enrollmentUsecase) ValidateReferral(ctx context.Context, buyerID int64, buyerIP, referralCode string, courseID int64) (map[string]interface{}, error) {
	if referralCode == "" {
		return map[string]interface{}{"valid": false, "message": "Referral code is empty"}, nil
	}

	buyer, err := u.repo.GetUserByID(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if buyer == nil {
		return nil, errors.New("buyer user not found")
	}

	referrer, err := u.repo.GetUserByReferralCode(ctx, referralCode)
	if err != nil {
		return nil, err
	}
	if referrer == nil {
		return map[string]interface{}{"valid": false, "message": "Invalid referral code"}, nil
	}

	// 1. Anti-fraud: Referrer cannot be the buyer
	if referrer.ID == buyerID {
		return map[string]interface{}{"valid": false, "message": "You cannot use your own referral code"}, nil
	}

	// 2. Anti-fraud: Personal info matching
	if strings.ToLower(strings.TrimSpace(buyer.Email)) == strings.ToLower(strings.TrimSpace(referrer.Email)) ||
		strings.ToLower(strings.TrimSpace(buyer.ContactNumber)) == strings.ToLower(strings.TrimSpace(referrer.ContactNumber)) ||
		strings.ToLower(strings.TrimSpace(buyer.FullName)) == strings.ToLower(strings.TrimSpace(referrer.FullName)) {
		return map[string]interface{}{"valid": false, "message": "Anti-fraud validation failed: User details match the referrer"}, nil
	}

	// 3. Anti-fraud: IP address checking
	cleanBuyerIP := strings.Split(buyerIP, ":")[0] // strip port if any
	if referrer.RegistrationIP != nil {
		cleanReferrerIP := strings.Split(*referrer.RegistrationIP, ":")[0]
		if cleanBuyerIP == cleanReferrerIP {
			return map[string]interface{}{"valid": false, "message": "Anti-fraud validation failed: IP address matches the referrer's registration IP"}, nil
		}
		if buyer.RegistrationIP != nil {
			cleanBuyerRegIP := strings.Split(*buyer.RegistrationIP, ":")[0]
			if cleanBuyerRegIP == cleanReferrerIP {
				return map[string]interface{}{"valid": false, "message": "Anti-fraud validation failed: Registration IP matches the referrer"}, nil
			}
		}
	}

	// 4. Anti-fraud: Cannot use the same referral code more than once
	usedBefore, err := u.repo.HasUserCompletedOrderForReferrer(ctx, buyerID, referrer.ID)
	if err != nil {
		return nil, err
	}
	if usedBefore {
		return map[string]interface{}{"valid": false, "message": "You have already used a referral code from this referrer"}, nil
	}

	// 5. Anti-fraud: Referral code cap (10 successful referrals)
	referralsCount, err := u.repo.CountCompletedReferralsForReferrer(ctx, referrer.ID)
	if err != nil {
		return nil, err
	}
	if referralsCount >= 10 {
		return map[string]interface{}{"valid": false, "message": "Referral code has reached its usage limit"}, nil
	}
	if referrer.CoinsBalance >= 10 {
		return map[string]interface{}{"valid": false, "message": "Referrer has reached the coins balance limit"}, nil
	}

	// Calculate discount for the course
	course, err := u.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course == nil {
		return nil, errors.New("course not found")
	}

	discountAmount := math.Round(course.FinalPrice * 0.20)
	finalPrice := course.FinalPrice - discountAmount
	if finalPrice < 0 {
		finalPrice = 0
	}

	return map[string]interface{}{
		"valid":          true,
		"discountAmount": discountAmount,
		"finalPrice":     finalPrice,
		"message":        "Referral code is valid",
		"referrerId":     referrer.ID,
	}, nil
}

func (u *enrollmentUsecase) CreateOrder(ctx context.Context, buyerID int64, buyerIP, userAgent string, req map[string]interface{}) (map[string]interface{}, error) {
	// Parse input parameters
	orderTypeVal, ok := req["orderType"]
	if !ok {
		return nil, errors.New("orderType is required")
	}
	orderType := orderTypeVal.(string)

	var courseID, noteID, testSeriesID int64
	var basePrice float64
	var title string

	// Load appropriate item and set basePrice
	switch orderType {
	case "course":
		cIDVal, ok := req["courseId"]
		if !ok {
			return nil, errors.New("courseId is required for course purchases")
		}
		courseID = int64(cIDVal.(float64))
		course, err := u.repo.GetCourseByID(ctx, courseID)
		if err != nil {
			return nil, err
		}
		if course == nil {
			return nil, errors.New("course not found")
		}
		if course.Visibility != "public" {
			return nil, errors.New("course is not available for public purchase")
		}
		basePrice = course.FinalPrice
		title = course.CourseName

	case "note":
		nIDVal, ok := req["noteId"]
		if !ok {
			return nil, errors.New("noteId is required for note purchases")
		}
		noteID = int64(nIDVal.(float64))
		note, err := u.repo.GetNoteByID(ctx, noteID)
		if err != nil {
			return nil, err
		}
		if note == nil {
			return nil, errors.New("note not found")
		}
		basePrice = note.Price
		title = note.Title

	case "test_series":
		tsIDVal, ok := req["testSeriesId"]
		if !ok {
			return nil, errors.New("testSeriesId is required for test series purchases")
		}
		testSeriesID = int64(tsIDVal.(float64))
		ts, err := u.repo.GetTestSeriesByID(ctx, testSeriesID)
		if err != nil {
			return nil, err
		}
		if ts == nil {
			return nil, errors.New("test series not found")
		}
		basePrice = ts.Price
		title = ts.Title

	default:
		return nil, fmt.Errorf("invalid order type: %s", orderType)
	}

	buyer, err := u.repo.GetUserByID(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if buyer == nil {
		return nil, errors.New("buyer user not found")
	}

	student, err := u.repo.GetStudentByUserID(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return nil, errors.New("student profile not found for this user")
	}

	// Check if already has access
	switch orderType {
	case "course":
		enroll, err := u.repo.GetEnrollment(ctx, student.ID, courseID)
		if err != nil {
			return nil, err
		}
		if enroll != nil {
			return nil, errors.New("you are already enrolled in this course")
		}
	case "note":
		access, err := u.repo.GetNoteAccess(ctx, student.ID, noteID)
		if err != nil {
			return nil, err
		}
		if access != nil {
			return nil, errors.New("you already have access to this note")
		}
	case "test_series":
		access, err := u.repo.GetTestSeriesAccess(ctx, student.ID, testSeriesID)
		if err != nil {
			return nil, err
		}
		if access != nil {
			return nil, errors.New("you already have access to this test series")
		}
	}

	// Check parameters for discounts
	referralCode := ""
	if refVal, ok := req["referralCode"]; ok && refVal != nil {
		referralCode = refVal.(string)
	}

	redeemCoins := false
	if coinsVal, ok := req["redeemCoins"]; ok && coinsVal != nil {
		redeemCoins = coinsVal.(bool)
	}

	deviceFingerprint := ""
	if dfVal, ok := req["deviceFingerprint"]; ok && dfVal != nil {
		deviceFingerprint = dfVal.(string)
	}

	// Coins cannot be redeemed simultaneously with a referral code
	if referralCode != "" && redeemCoins {
		return nil, errors.New("coins cannot be redeemed simultaneously with a referral code")
	}

	finalPrice := basePrice
	var coinsRedeemed int = 0
	var referrerID *int64 = nil

	// Apply discount logic
	if orderType == "course" {
		if referralCode != "" {
			res, err := u.ValidateReferral(ctx, buyerID, buyerIP, referralCode, courseID)
			if err != nil {
				return nil, err
			}
			if valid, ok := res["valid"].(bool); ok && valid {
				finalPrice = res["finalPrice"].(float64)
				refID := res["referrerId"].(int64)
				referrerID = &refID
			} else {
				return nil, fmt.Errorf("referral code validation failed: %s", res["message"].(string))
			}
		} else if redeemCoins && buyer.CoinsBalance > 0 {
			referralsCount, err := u.repo.CountCompletedReferralsForReferrer(ctx, buyerID)
			if err != nil {
				return nil, err
			}

			var maxDiscount float64 = 0
			if referralsCount >= 5 && referralsCount <= 9 {
				if basePrice < 3000 {
					maxDiscount = 500
				} else {
					maxDiscount = 600
				}
			} else if referralsCount == 10 {
				if basePrice < 2500 {
					maxDiscount = 800
				} else {
					maxDiscount = basePrice - 300
					if maxDiscount < 0 {
						maxDiscount = 0
					}
				}
			} else if referralsCount > 10 {
				maxDiscount = basePrice - 100
				if maxDiscount < 0 {
					maxDiscount = 0
				}
			}

			if maxDiscount > 0 {
				maxCoinsAllowed := int(math.Floor(maxDiscount / 120.0))
				if maxCoinsAllowed > buyer.CoinsBalance {
					coinsRedeemed = buyer.CoinsBalance
				} else {
					coinsRedeemed = maxCoinsAllowed
				}
				coinDiscount := float64(coinsRedeemed) * 120.0
				finalPrice = basePrice - coinDiscount
				if finalPrice < 0 {
					finalPrice = 0
				}
			}
		}
	}

	accountAgeDays := int(time.Since(buyer.CreatedAt).Hours() / 24)

	// Create DB order
	order := &domain.PaymentOrder{
		Amount:                finalPrice,
		Status:                "created",
		OrderType:             orderType,
		UserID:                buyerID,
		CoinsRedeemed:         coinsRedeemed,
		AccountAgeAtOrderDays: accountAgeDays,
		DeviceFingerprint:     &deviceFingerprint,
		IPAddress:             &buyerIP,
		UserAgent:             &userAgent,
	}

	if courseID > 0 {
		order.CourseID = &courseID
	}
	if noteID > 0 {
		order.NoteID = &noteID
	}
	if testSeriesID > 0 {
		order.TestSeriesID = &testSeriesID
	}
	if referrerID != nil {
		order.ReferrerID = referrerID
	}

	// 1. Direct free enrollment (finalPrice == 0)
	if finalPrice == 0 {
		txRepo, err := u.repo.BeginTx(ctx)
		if err != nil {
			return nil, err
		}
		defer txRepo.RollbackTx()

		order.RazorpayOrderID = fmt.Sprintf("free_order_%d_%d", buyerID, time.Now().UnixNano())
		order.Status = "completed"

		if err := txRepo.CreatePaymentOrder(ctx, order); err != nil {
			return nil, err
		}

		// Deduct coins if any
		if coinsRedeemed > 0 {
			if err := txRepo.UpdateUserCoins(ctx, buyerID, -coinsRedeemed); err != nil {
				return nil, err
			}
		}

		// Grant access
		if err := u.grantAccess(ctx, txRepo, student.ID, order); err != nil {
			return nil, err
		}

		// Log audit
		payloadBytes, _ := json.Marshal(map[string]interface{}{
			"action":      "direct_free_purchase",
			"orderId":     order.ID,
			"finalPrice":  finalPrice,
			"coinsUsed":   coinsRedeemed,
		})
		audit := &domain.PaymentAuditLog{
			EventType:      "order.completed",
			Payload:        payloadBytes,
			PaymentOrderID: &order.ID,
			UserID:         &buyerID,
		}
		_ = txRepo.CreateAuditLog(ctx, audit)

		if err := txRepo.CommitTx(); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"isFree":         true,
			"freeOrder":      true,
			"paymentOrderId": order.ID,
			"status":         "completed",
			"message":        "Successfully enrolled in free item",
		}, nil
	}

	// 2. Paid Order creation via Razorpay
	// Temporarily save order to get sequential database ID for receipt
	order.RazorpayOrderID = "TEMP_ID"
	if err := u.repo.CreatePaymentOrder(ctx, order); err != nil {
		return nil, err
	}

	receipt := fmt.Sprintf("receipt_order_%d", order.ID)
	rzpOrderID, err := u.createRazorpayOrder(finalPrice, receipt)
	if err != nil {
		// Clean up the temp order if order creation failed
		// Since we didn't use a transaction, we can just delete it or update status to failed
		order.Status = "failed"
		failReason := err.Error()
		order.FailureReason = &failReason
		_ = u.repo.UpdatePaymentOrder(ctx, order)
		return nil, fmt.Errorf("razorpay order creation failed: %w", err)
	}

	order.RazorpayOrderID = rzpOrderID
	if err := u.repo.UpdatePaymentOrder(ctx, order); err != nil {
		return nil, err
	}

	resp := map[string]interface{}{
		"isFree":          false,
		"freeOrder":       false,
		"paymentOrderId":  order.ID,
		"orderId":         rzpOrderID,
		"razorpayOrderId": rzpOrderID,
		"amount":          int(finalPrice * 100), // in paise for checkout
		"currency":        "INR",
		"keyId":           u.keyID,
		"name":            "ClaSynq",
		"description":     fmt.Sprintf("Purchase %s", title),
	}

	if orderType == "course" {
		resp["courseName"] = title
	} else if orderType == "note" {
		resp["noteTitle"] = title
	} else if orderType == "test_series" {
		resp["testSeriesTitle"] = title
	}

	return resp, nil
}

func (u *enrollmentUsecase) createRazorpayOrder(amount float64, receipt string) (string, error) {
	amountInPaise := int(math.Round(amount * 100))
	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  receipt,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(u.keyID + ":" + u.keySecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("status: %d, response: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid razorpay response format: %s", string(respBody))
	}

	return id, nil
}

func (u *enrollmentUsecase) VerifyPayment(ctx context.Context, buyerID int64, req map[string]interface{}) (map[string]interface{}, error) {
	log.Printf("[VerifyPayment] Received request for buyerID=%d: %+v", buyerID, req)
	rzpOrderID, ok1 := req["razorpayOrderId"].(string)
	if !ok1 {
		rzpOrderID, ok1 = req["razorpay_order_id"].(string)
	}
	rzpPaymentID, ok2 := req["razorpayPaymentId"].(string)
	if !ok2 {
		rzpPaymentID, ok2 = req["razorpay_payment_id"].(string)
	}
	rzpSignature, ok3 := req["razorpaySignature"].(string)
	if !ok3 {
		rzpSignature, ok3 = req["razorpay_signature"].(string)
	}

	if !ok1 || !ok2 || !ok3 {
		log.Printf("[VerifyPayment] Missing parameters. ok1=%t, ok2=%t, ok3=%t, keys present: %+v", ok1, ok2, ok3, req)
		return nil, errors.New("missing required payment parameters: razorpayOrderId/razorpay_order_id, razorpayPaymentId/razorpay_payment_id, razorpaySignature/razorpay_signature")
	}

	// Verify Payment Signature
	mac := hmac.New(sha256.New, []byte(u.keySecret))
	mac.Write([]byte(rzpOrderID + "|" + rzpPaymentID))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	log.Printf("[VerifyPayment] Signature check: expected=%s, received=%s", expectedSignature, rzpSignature)
	if subtle.ConstantTimeCompare([]byte(rzpSignature), []byte(expectedSignature)) != 1 {
		log.Printf("[VerifyPayment] Signature verification failed")
		return nil, errors.New("payment signature verification failed")
	}

	// Process transactionally
	txRepo, err := u.repo.BeginTx(ctx)
	if err != nil {
		log.Printf("[VerifyPayment] BeginTx error: %v", err)
		return nil, err
	}
	defer txRepo.RollbackTx()

	order, err := txRepo.GetPaymentOrderByRazorpayID(ctx, rzpOrderID)
	if err != nil {
		log.Printf("[VerifyPayment] GetPaymentOrderByRazorpayID error: %v", err)
		return nil, err
	}
	if order == nil {
		log.Printf("[VerifyPayment] Order not found for razorpayOrderID=%s", rzpOrderID)
		return nil, errors.New("payment order not found")
	}

	if order.Status == "completed" {
		_ = txRepo.CommitTx()
		return map[string]interface{}{"status": "completed", "paymentOrderId": order.ID}, nil
	}

	student, err := txRepo.GetStudentByUserID(ctx, order.UserID)
	if err != nil {
		log.Printf("[VerifyPayment] GetStudentByUserID error: %v", err)
		return nil, err
	}
	if student == nil {
		log.Printf("[VerifyPayment] Student not found for userID=%d", order.UserID)
		return nil, errors.New("student not found for user")
	}

	// Update order status
	order.Status = "completed"
	order.RazorpayPaymentID = &rzpPaymentID
	if err := txRepo.UpdatePaymentOrder(ctx, order); err != nil {
		log.Printf("[VerifyPayment] UpdatePaymentOrder error: %v", err)
		return nil, err
	}

	// Deduct coins if they were used
	if order.CoinsRedeemed > 0 {
		if err := txRepo.UpdateUserCoins(ctx, order.UserID, -order.CoinsRedeemed); err != nil {
			log.Printf("[VerifyPayment] UpdateUserCoins error: %v", err)
			return nil, err
		}
	}

	// Grant item access
	if err := u.grantAccess(ctx, txRepo, student.ID, order); err != nil {
		log.Printf("[VerifyPayment] grantAccess error: %v", err)
		return nil, err
	}

	// Handle referral tracking (set to pending hold)
	if order.ReferrerID != nil {
		refTx := &domain.ReferralTransaction{
			Status:          "pending_hold",
			PaymentOrderID:  order.ID,
			ReferredBuyerID: order.UserID,
			ReferrerID:      *order.ReferrerID,
		}
		if err := txRepo.CreateReferralTransaction(ctx, refTx); err != nil {
			return nil, err
		}
	}

	// Log audit
	payloadBytes, _ := json.Marshal(req)
	audit := &domain.PaymentAuditLog{
		EventType:      "payment.verified",
		Payload:        payloadBytes,
		PaymentOrderID: &order.ID,
		UserID:         &order.UserID,
	}
	_ = txRepo.CreateAuditLog(ctx, audit)

	if err := txRepo.CommitTx(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":         "completed",
		"paymentOrderId": order.ID,
		"message":        "Payment verified and access granted successfully",
	}, nil
}

func (u *enrollmentUsecase) grantAccess(ctx context.Context, txRepo domain.EnrollmentRepository, studentID int64, order *domain.PaymentOrder) error {
	switch order.OrderType {
	case "course":
		if order.CourseID == nil {
			return errors.New("missing course_id on course payment order")
		}
		enrollment := &domain.Enrollment{
			CourseID:  *order.CourseID,
			StudentID: studentID,
		}
		return txRepo.CreateEnrollment(ctx, enrollment)

	case "note":
		if order.NoteID == nil {
			return errors.New("missing note_id on note payment order")
		}
		access := &domain.NoteAccess{
			NoteID:    *order.NoteID,
			StudentID: studentID,
		}
		return txRepo.CreateNoteAccess(ctx, access)

	case "test_series":
		if order.TestSeriesID == nil {
			return errors.New("missing test_series_id on test_series payment order")
		}
		access := &domain.TestSeriesAccess{
			TestSeriesID: *order.TestSeriesID,
			StudentID:    studentID,
		}
		return txRepo.CreateTestSeriesAccess(ctx, access)
	}

	return fmt.Errorf("unknown order type for access grant: %s", order.OrderType)
}

func (u *enrollmentUsecase) revokeAccess(ctx context.Context, txRepo domain.EnrollmentRepository, studentID int64, order *domain.PaymentOrder) error {
	switch order.OrderType {
	case "course":
		if order.CourseID != nil {
			return txRepo.DeleteEnrollment(ctx, studentID, *order.CourseID)
		}
	case "note":
		if order.NoteID != nil {
			return txRepo.DeleteNoteAccess(ctx, studentID, *order.NoteID)
		}
	case "test_series":
		if order.TestSeriesID != nil {
			return txRepo.DeleteTestSeriesAccess(ctx, studentID, *order.TestSeriesID)
		}
	}
	return nil
}

func (u *enrollmentUsecase) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	// 1. Verify webhook signature
	mac := hmac.New(sha256.New, []byte(u.webhookSecret))
	mac.Write(rawBody)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return errors.New("webhook signature verification failed")
	}

	// 2. Parse event payload
	var webhookPayload map[string]interface{}
	if err := json.Unmarshal(rawBody, &webhookPayload); err != nil {
		return err
	}

	var eventID string
	if idVal, ok := webhookPayload["id"]; ok && idVal != nil {
		eventID, _ = idVal.(string)
	}

	eventType, _ := webhookPayload["event"].(string)

	// 3. Idempotency Check (only if eventID is provided)
	if eventID != "" {
		existingEvent, err := u.repo.GetWebhookEventByID(ctx, eventID)
		if err != nil {
			return err
		}
		if existingEvent != nil {
			log.Printf("Webhook event %s already processed, skipping.", eventID)
			return nil
		}

		// Insert event_id immediately to mark as processed
		evtRecord := &domain.WebhookEvent{EventID: eventID, ProcessedAt: time.Now()}
		if err := u.repo.CreateWebhookEvent(ctx, evtRecord); err != nil {
			return err
		}
	}

	// 4. Handle events
	log.Printf("Processing Razorpay Webhook Event: %s (%s)", eventType, eventID)
	switch eventType {
	case "order.paid", "payment.captured":
		var rzpOrderID, rzpPaymentID string
		var paymentMethod string

		payloadObj, ok := webhookPayload["payload"].(map[string]interface{})
		if !ok {
			return nil
		}

		if eventType == "order.paid" {
			orderObj, _ := payloadObj["order"].(map[string]interface{})
			entity, _ := orderObj["entity"].(map[string]interface{})
			rzpOrderID, _ = entity["id"].(string)
			// Get payments associated if any
			// Webhook order.paid typically has the payment in the entity list
		} else {
			paymentObj, _ := payloadObj["payment"].(map[string]interface{})
			entity, _ := paymentObj["entity"].(map[string]interface{})
			rzpOrderID, _ = entity["order_id"].(string)
			rzpPaymentID, _ = entity["id"].(string)
			paymentMethod, _ = entity["method"].(string)
		}

		if rzpOrderID == "" {
			return nil
		}

		txRepo, err := u.repo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer txRepo.RollbackTx()

		order, err := txRepo.GetPaymentOrderByRazorpayID(ctx, rzpOrderID)
		if err != nil {
			return err
		}
		if order == nil {
			return nil // order not from our app, ignore
		}

		if order.Status != "completed" {
			order.Status = "completed"
			if rzpPaymentID != "" {
				order.RazorpayPaymentID = &rzpPaymentID
			}
			if paymentMethod != "" {
				order.PaymentMethod = &paymentMethod
			}

			if err := txRepo.UpdatePaymentOrder(ctx, order); err != nil {
				return err
			}

			// Deduct coins
			if order.CoinsRedeemed > 0 {
				if err := txRepo.UpdateUserCoins(ctx, order.UserID, -order.CoinsRedeemed); err != nil {
					return err
				}
			}

			// Grant access
			student, err := txRepo.GetStudentByUserID(ctx, order.UserID)
			if err != nil {
				return err
			}
			if student != nil {
				_ = u.grantAccess(ctx, txRepo, student.ID, order)
			}

			// Add referral transaction
			if order.ReferrerID != nil {
				refTx := &domain.ReferralTransaction{
					Status:          "pending_hold",
					PaymentOrderID:  order.ID,
					ReferredBuyerID: order.UserID,
					ReferrerID:      *order.ReferrerID,
				}
				_ = txRepo.CreateReferralTransaction(ctx, refTx)
			}
		}

		// Log audit
		audit := &domain.PaymentAuditLog{
			EventType:      eventType,
			Payload:        rawBody,
			PaymentOrderID: &order.ID,
			UserID:         &order.UserID,
		}
		_ = txRepo.CreateAuditLog(ctx, audit)
		_ = txRepo.CommitTx()

	case "payment.failed":
		payloadObj, _ := webhookPayload["payload"].(map[string]interface{})
		paymentObj, _ := payloadObj["payment"].(map[string]interface{})
		entity, _ := paymentObj["entity"].(map[string]interface{})
		rzpOrderID, _ := entity["order_id"].(string)
		errDesc, _ := entity["error_description"].(string)

		if rzpOrderID != "" {
			order, _ := u.repo.GetPaymentOrderByRazorpayID(ctx, rzpOrderID)
			if order != nil && order.Status == "created" {
				order.Status = "failed"
				order.FailureReason = &errDesc
				_ = u.repo.UpdatePaymentOrder(ctx, order)
			}
			audit := &domain.PaymentAuditLog{
				EventType: eventType,
				Payload:   rawBody,
				UserID:    &order.UserID,
			}
			if order != nil {
				audit.PaymentOrderID = &order.ID
			}
			_ = u.repo.CreateAuditLog(ctx, audit)
		}

	case "refund.processed":
		payloadObj, _ := webhookPayload["payload"].(map[string]interface{})
		refundObj, _ := payloadObj["refund"].(map[string]interface{})
		entity, _ := refundObj["entity"].(map[string]interface{})
		rzpPaymentID, _ := entity["payment_id"].(string)
		amountVal, _ := entity["amount"].(float64) // in paise

		if rzpPaymentID != "" {
			txRepo, err := u.repo.BeginTx(ctx)
			if err != nil {
				return err
			}
			defer txRepo.RollbackTx()

			// Lookup order by payment ID
			order, err := txRepo.GetPaymentOrderByPaymentID(ctx, rzpPaymentID)

			if err == nil && order != nil && order.Status != "refunded" {
				order.Status = "refunded"
				order.Refunded = true
				refAmt := amountVal / 100.0
				order.RefundedAmount = &refAmt
				refAt := time.Now()
				order.RefundedAt = &refAt

				_ = txRepo.UpdatePaymentOrder(ctx, order)

				// Revoke student access
				student, _ := txRepo.GetStudentByUserID(ctx, order.UserID)
				if student != nil {
					_ = u.revokeAccess(ctx, txRepo, student.ID, order)
				}

				// Void referral transaction
				refTx, _ := txRepo.GetReferralTransactionByOrderID(ctx, order.ID)
				if refTx != nil {
					if refTx.Status == "credited" {
						// Deduct the rewarded coin from the referrer
						_ = txRepo.UpdateUserCoins(ctx, refTx.ReferrerID, -1)
					}
					refTx.Status = "voided"
					_ = txRepo.UpdateReferralTransaction(ctx, refTx)
				}

				audit := &domain.PaymentAuditLog{
					EventType:      eventType,
					Payload:        rawBody,
					PaymentOrderID: &order.ID,
					UserID:         &order.UserID,
				}
				_ = txRepo.CreateAuditLog(ctx, audit)
				_ = txRepo.CommitTx()
			}
		}
	}

	return nil
}

func (u *enrollmentUsecase) RefundOrder(ctx context.Context, orderID int64) error {
	txRepo, err := u.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer txRepo.RollbackTx()

	order, err := txRepo.GetPaymentOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return errors.New("payment order not found")
	}

	if order.Status == "refunded" {
		return errors.New("order is already refunded")
	}

	if order.Status != "completed" {
		return errors.New("cannot refund an incomplete order")
	}

	if order.RazorpayPaymentID == nil || *order.RazorpayPaymentID == "" {
		return errors.New("cannot refund order without a valid razorpay payment ID")
	}

	// Call Razorpay API to process refund
	refundPayload := map[string]interface{}{
		"amount": int(order.Amount * 100), // full refund in paise
	}
	bodyBytes, _ := json.Marshal(refundPayload)

	endpoint := fmt.Sprintf("https://api.razorpay.com/v1/payments/%s/refund", *order.RazorpayPaymentID)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(u.keyID + ":" + u.keySecret))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to process refund via Razorpay: (status %d) %s", resp.StatusCode, string(respBody))
	}

	// Update order status
	order.Status = "refunded"
	order.Refunded = true
	refAmt := order.Amount
	order.RefundedAmount = &refAmt
	refAt := time.Now()
	order.RefundedAt = &refAt
	if err := txRepo.UpdatePaymentOrder(ctx, order); err != nil {
		return err
	}

	// Revoke Access
	student, err := txRepo.GetStudentByUserID(ctx, order.UserID)
	if err != nil {
		return err
	}
	if student != nil {
		_ = u.revokeAccess(ctx, txRepo, student.ID, order)
	}

	// Void Referral
	refTx, err := txRepo.GetReferralTransactionByOrderID(ctx, order.ID)
	if err == nil && refTx != nil {
		if refTx.Status == "credited" {
			// Deduct coin from referrer
			_ = txRepo.UpdateUserCoins(ctx, refTx.ReferrerID, -1)
		}
		refTx.Status = "voided"
		_ = txRepo.UpdateReferralTransaction(ctx, refTx)
	}

	// Audit Log
	audit := &domain.PaymentAuditLog{
		EventType:      "refund.initiated",
		Payload:        respBody,
		PaymentOrderID: &order.ID,
		UserID:         &order.UserID,
	}
	_ = txRepo.CreateAuditLog(ctx, audit)

	return txRepo.CommitTx()
}

func (u *enrollmentUsecase) ProcessPendingReferrals(ctx context.Context) error {
	txs, err := u.repo.GetPendingReferralTransactions(ctx)
	if err != nil {
		return err
	}

	for _, tx := range txs {
		// Verify purchase is completed
		order, err := u.repo.GetPaymentOrderByID(ctx, tx.PaymentOrderID)
		if err != nil {
			log.Printf("Error fetching payment order %d: %v", tx.PaymentOrderID, err)
			continue
		}

		if order == nil {
			continue
		}

		if order.Status == "completed" {
			txRepo, err := u.repo.BeginTx(ctx)
			if err != nil {
				log.Printf("Error starting tx to credit referral %d: %v", tx.ID, err)
				continue
			}

			// Verify referrer has < 10 coins and total completed referrals < 10
			referrer, err := txRepo.GetUserByID(ctx, tx.ReferrerID)
			if err == nil && referrer != nil {
				completedCount, _ := txRepo.CountCompletedReferralsForReferrer(ctx, tx.ReferrerID)

				if referrer.CoinsBalance < 10 && completedCount <= 10 {
					// Credit referral reward (1 coin)
					if err := txRepo.UpdateUserCoins(ctx, tx.ReferrerID, 1); err == nil {
						tx.Status = "credited"
						now := time.Now()
						tx.CreditedAt = &now
						_ = txRepo.UpdateReferralTransaction(ctx, &tx)
						_ = txRepo.CommitTx()
						log.Printf("Successfully credited 1 coin to user %d for referral %d", tx.ReferrerID, tx.ID)
						continue
					}
				} else {
					// Capped out, void it
					tx.Status = "voided"
					_ = txRepo.UpdateReferralTransaction(ctx, &tx)
					_ = txRepo.CommitTx()
					log.Printf("Voided referral transaction %d (limit reached)", tx.ID)
					continue
				}
			}
			txRepo.RollbackTx()

		} else if order.Status == "failed" || order.Status == "refunded" {
			txRepo, err := u.repo.BeginTx(ctx)
			if err == nil {
				tx.Status = "voided"
				_ = txRepo.UpdateReferralTransaction(ctx, &tx)
				_ = txRepo.CommitTx()
				log.Printf("Voided referral transaction %d because order is %s", tx.ID, order.Status)
			}
		}
	}

	return nil
}

func (u *enrollmentUsecase) GetMyEnrollments(ctx context.Context, userID int64, category string) ([]map[string]interface{}, error) {
	student, err := u.repo.GetStudentByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return []map[string]interface{}{}, nil
	}
	return u.repo.GetMyEnrollments(ctx, student.ID, category)
}
