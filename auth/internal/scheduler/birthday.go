package scheduler

import (
	"log"
	"time"

	"clasynq/api/auth/config"
	"clasynq/api/auth/internal/domain"
	"clasynq/api/auth/internal/utils"

	"gorm.io/gorm"
)

// StartBirthdayWishScheduler starts the background cron task for birthday emails
func StartBirthdayWishScheduler(db *gorm.DB, cfg *config.Config) {
	log.Println("Starting Birthday Wish Scheduler background worker...")
	go func() {
		// Run first check after a short delay (e.g. 1 minute after server starts)
		time.Sleep(1 * time.Minute)
		RunBirthdayCheck(db, cfg)

		for {
			// Calculate duration until next check time (e.g. daily at 09:00 AM local time)
			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
			if now.After(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}
			duration := time.Until(nextRun)
			log.Printf("[BirthdayScheduler] Next scan scheduled at: %s (in %s)", nextRun.Format("2006-01-02 15:04:05"), duration)

			select {
			case <-time.After(duration):
				RunBirthdayCheck(db, cfg)
			}
		}
	}()
}

// RunBirthdayCheck executes the query and sends birthday emails
func RunBirthdayCheck(db *gorm.DB, cfg *config.Config) {
	log.Println("[BirthdayScheduler] Running birthday wish scan...")
	today := time.Now()

	// 1. Fetch and email students (users table)
	var birthdayUsers []domain.User
	err := db.Where("EXTRACT(MONTH FROM date_of_birth) = ? AND EXTRACT(DAY FROM date_of_birth) = ?", int(today.Month()), today.Day()).Find(&birthdayUsers).Error
	if err != nil {
		log.Printf("[BirthdayScheduler] Error fetching birthday users: %v", err)
	} else {
		log.Printf("[BirthdayScheduler] Found %d students celebrating their birthday today", len(birthdayUsers))
		for _, u := range birthdayUsers {
			log.Printf("[BirthdayScheduler] Sending birthday wish email to student: %s (%s)", u.FullName, u.Email)
			err := utils.SendBirthdayEmail(
				u.Email,
				u.FullName,
				cfg.DefaultFromEmail,
				cfg.EmailHost,
				cfg.EmailPort,
				cfg.EmailHostUser,
				cfg.EmailHostPassword,
			)
			if err != nil {
				log.Printf("[BirthdayScheduler] Failed to send birthday email to %s: %v", u.Email, err)
			}
		}
	}

	// 2. Fetch and email teachers (teachers table)
	var birthdayTeachers []domain.Teacher
	err = db.Where("EXTRACT(MONTH FROM date_of_birth) = ? AND EXTRACT(DAY FROM date_of_birth) = ?", int(today.Month()), today.Day()).Find(&birthdayTeachers).Error
	if err != nil {
		log.Printf("[BirthdayScheduler] Error fetching birthday teachers: %v", err)
	} else {
		log.Printf("[BirthdayScheduler] Found %d teachers celebrating their birthday today", len(birthdayTeachers))
		for _, t := range birthdayTeachers {
			log.Printf("[BirthdayScheduler] Sending birthday wish email to teacher: %s (%s)", t.Name, t.Email)
			err := utils.SendBirthdayEmail(
				t.Email,
				t.Name,
				cfg.DefaultFromEmail,
				cfg.EmailHost,
				cfg.EmailPort,
				cfg.EmailHostUser,
				cfg.EmailHostPassword,
			)
			if err != nil {
				log.Printf("[BirthdayScheduler] Failed to send birthday email to %s: %v", t.Email, err)
			}
		}
	}
	log.Println("[BirthdayScheduler] Birthday wish scan completed.")
}
