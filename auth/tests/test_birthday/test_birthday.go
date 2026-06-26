package main

import (
	"fmt"
	"time"

	"clasynq/api/auth/config"
	"clasynq/api/auth/internal/domain"
	"clasynq/api/auth/internal/scheduler"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load config from .env or auth/.env
	_ = godotenv.Load("d:/Clasynq_future_update/API_2.0/auth/.env")
	cfg := config.LoadConfig()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	fmt.Println("Connected to database successfully.")

	// Fetch a student (User model, users table)
	var student domain.User
	err = db.Order("id asc").First(&student).Error
	if err != nil {
		fmt.Printf("Warning: no student found in users table: %v\n", err)
	}

	// Fetch a teacher (Teacher model, teachers table)
	var teacher domain.Teacher
	err = db.Order("id asc").First(&teacher).Error
	if err != nil {
		fmt.Printf("Warning: no teacher found in teachers table: %v\n", err)
	}

	// Capture original DOBs to restore them later
	originalStudentDOB := student.DateOfBirth
	originalTeacherDOB := teacher.DateOfBirth

	// Set DOB to today (varying years to be realistic, e.g., 2000 for student, 1990 for teacher)
	today := time.Now()
	studentDOBToday := time.Date(2000, today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	teacherDOBToday := time.Date(1990, today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	// Update in DB temporarily
	if student.ID != 0 {
		fmt.Printf("Updating student %s (ID=%d) DOB to today: %s\n", student.FullName, student.ID, studentDOBToday.Format("2006-01-02"))
		err = db.Model(&domain.User{}).Where("id = ?", student.ID).Update("date_of_birth", studentDOBToday).Error
		if err != nil {
			panic(fmt.Sprintf("Failed to update student DOB: %v", err))
		}
	}

	if teacher.ID != 0 {
		fmt.Printf("Updating teacher %s (ID=%d) DOB to today: %s\n", teacher.Name, teacher.ID, teacherDOBToday.Format("2006-01-02"))
		err = db.Model(&domain.Teacher{}).Where("id = ?", teacher.ID).Update("date_of_birth", teacherDOBToday).Error
		if err != nil {
			panic(fmt.Sprintf("Failed to update teacher DOB: %v", err))
		}
	}

	// Defer restoration of original DOBs
	defer func() {
		fmt.Println("Restoring original DOBs in database...")
		if student.ID != 0 {
			db.Model(&domain.User{}).Where("id = ?", student.ID).Update("date_of_birth", originalStudentDOB)
		}
		if teacher.ID != 0 {
			db.Model(&domain.Teacher{}).Where("id = ?", teacher.ID).Update("date_of_birth", originalTeacherDOB)
		}
		fmt.Println("Database records restored.")
	}()

	// Execute RunBirthdayCheck manually
	fmt.Println("Executing RunBirthdayCheck manually...")
	scheduler.RunBirthdayCheck(db, cfg)
	fmt.Println("RunBirthdayCheck execution completed.")
}
