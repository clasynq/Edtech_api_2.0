package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load .env
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:suro1234@localhost:6432/Clasynq"
	}
	fmt.Printf("Connecting to database: %s\n", dbURL)

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	// 1. Inspect teachers
	type Teacher struct {
		ID              int64
		Name            string
		AssignedCourses string `gorm:"column:Assigned_courses"`
	}
	var teachers []Teacher
	db.Table("teachers").Find(&teachers)
	fmt.Println("\n--- Teachers ---")
	for _, t := range teachers {
		fmt.Printf("ID: %d | Name: %s | Assigned_courses (JSON): %s\n", t.ID, t.Name, t.AssignedCourses)
	}

	// 2. Inspect courses
	type Course struct {
		ID         int64
		CourseName string `gorm:"column:course_name"`
		TeacherID  *int64 `gorm:"column:teacher_id"`
	}
	var courses []Course
	db.Table("courses").Find(&courses)
	fmt.Println("\n--- Courses ---")
	for _, c := range courses {
		tID := int64(0)
		if c.TeacherID != nil {
			tID = *c.TeacherID
		}
		fmt.Printf("ID: %d | Name: %s | TeacherID (Primary): %d\n", c.ID, c.CourseName, tID)
	}

	// 3. Inspect courses_teachers
	type Link struct {
		CourseID  int64 `gorm:"column:course_id"`
		TeacherID int64 `gorm:"column:teacher_id"`
	}
	var links []Link
	db.Table("courses_teachers").Find(&links)
	fmt.Println("\n--- courses_teachers Link Table ---")
	for _, l := range links {
		fmt.Printf("CourseID: %d | TeacherID: %d\n", l.CourseID, l.TeacherID)
	}
}
