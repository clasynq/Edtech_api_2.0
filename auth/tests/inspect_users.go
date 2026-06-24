package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"clasynq/api/auth/config"
	"clasynq/api/auth/internal/domain"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Load config (try parent directories first as it is in the tests folder)
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")

	cfg := config.LoadConfig()
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set in environment or env files.")
	}

	fmt.Printf("Connecting to Postgres database...\n")
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 2. Query Students/Users
	var users []domain.User
	if err := db.Limit(50).Order("created_at desc").Find(&users).Error; err != nil {
		log.Fatalf("Failed to fetch users: %v", err)
	}

	// 3. Query Admins
	var admins []domain.Admin
	if err := db.Limit(20).Find(&admins).Error; err != nil {
		log.Printf("Warning: Failed to fetch admins: %v", err)
	}

	// 4. Query Teachers
	var teachers []domain.Teacher
	if err := db.Limit(20).Find(&teachers).Error; err != nil {
		log.Printf("Warning: Failed to fetch teachers: %v", err)
	}

	// 5. Structure Output
	output := map[string]interface{}{
		"users_count": len(users),
		"users":       users,
		"admins":      admins,
		"teachers":    teachers,
	}

	// Render JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Failed to serialize output: %v", err)
	}

	fmt.Println("\n--- Database User Inspection Results ---")
	fmt.Println(string(jsonData))
	fmt.Println("----------------------------------------")
}
