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
	_ = godotenv.Load("d:/Clasynq_future_update/API_2.0/.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	rows, err := db.Raw("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'test_series'").Rows()
	if err != nil {
		log.Fatalf("Failed to query columns: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== Columns in 'test_series' ===")
	for rows.Next() {
		var name, dataType string
		rows.Scan(&name, &dataType)
		fmt.Printf("%s: %s\n", name, dataType)
	}
}
