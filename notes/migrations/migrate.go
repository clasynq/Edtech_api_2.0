package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Try loading .env from current and parent folders
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("d:/Clasynq_future_update/API_2.0/notes/.env")
	_ = godotenv.Load("d:/Clasynq_future_update/API_2.0/.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is empty")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	// 1. Ensure the schema_migrations table exists
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		log.Fatalf("failed to create schema_migrations table: %v", err)
	}

	// 2. Query already applied migrations
	var applied []string
	err = db.Raw("SELECT version FROM schema_migrations").Scan(&applied).Error
	if err != nil {
		log.Fatalf("failed to query applied migrations: %v", err)
	}

	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	// 3. Find the migrations directory
	migDir := "migrations"
	if _, err := os.Stat(migDir); os.IsNotExist(err) {
		// Check if we are inside the migrations folder itself by looking for README.md or 0001_add_prerequisite_url.sql
		if _, err := os.Stat("README.md"); err == nil {
			migDir = "."
		} else {
			// Fallback to absolute path
			migDir = "d:/Clasynq_future_update/API_2.0/notes/migrations"
		}
	}

	// 4. Read all files in the directory
	files, err := ioutil.ReadDir(migDir)
	if err != nil {
		log.Fatalf("failed to read migrations directory %s: %v", migDir, err)
	}

	var sqlFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			sqlFiles = append(sqlFiles, file.Name())
		}
	}

	// 5. Sort migration files alphabetically (so 0001 runs before 0002 etc.)
	sort.Strings(sqlFiles)

	// 6. Run pending migrations
	runCount := 0
	for _, sqlFile := range sqlFiles {
		if appliedMap[sqlFile] {
			continue // Already applied
		}

		filePath := filepath.Join(migDir, sqlFile)
		query, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("failed to read migration file %s: %v", filePath, err)
		}

		fmt.Printf("Applying migration: %s...\n", sqlFile)

		// Run in a transaction
		tx := db.Begin()
		if err := tx.Exec(string(query)).Error; err != nil {
			tx.Rollback()
			log.Fatalf("failed to execute migration %s: %v", sqlFile, err)
		}

		// Record the migration
		if err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", sqlFile).Error; err != nil {
			tx.Rollback()
			log.Fatalf("failed to record migration %s: %v", sqlFile, err)
		}

		if err := tx.Commit().Error; err != nil {
			log.Fatalf("failed to commit transaction for migration %s: %v", sqlFile, err)
		}

		fmt.Printf("Successfully executed migration: %s\n", sqlFile)
		runCount++
	}

	if runCount == 0 {
		fmt.Println("No new migrations to apply.")
	} else {
		fmt.Printf("Completed %d new migrations successfully.\n", runCount)
	}
}
