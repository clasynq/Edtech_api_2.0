package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"clasynq/api/test_series/internal/repository"
	"clasynq/api/test_series/internal/usecase"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load("d:/Clasynq_future_update/API_2.0/test_series/.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	repo := repository.NewPostgresTestSeriesRepository(db)
	uc := usecase.NewTestSeriesUsecase(repo)

	ctx := context.Background()

	// Check userID = 1, role = "student"
	list, err := uc.GetTestSeries(ctx, 1, "student", map[string]string{})
	if err != nil {
		log.Fatalf("GetTestSeries failed: %v", err)
	}

	fmt.Println("=== GetTestSeries for User 1 (student) ===")
	for _, ts := range list {
		fmt.Printf("ID: %d, Title: %s, IsFree: %t, Price: %f, HasAccess: %t, IsPurchased: %t, IsPurchasedSnake: %t\n",
			ts.ID, ts.Title, ts.IsFree, ts.Price, ts.HasAccess, ts.IsPurchased, ts.IsPurchasedSnake)
	}

	// Check userID = 2, role = "student"
	list2, err := uc.GetTestSeries(ctx, 2, "student", map[string]string{})
	if err != nil {
		log.Fatalf("GetTestSeries failed: %v", err)
	}

	fmt.Println("\n=== GetTestSeries for User 2 (student) ===")
	for _, ts := range list2 {
		fmt.Printf("ID: %d, Title: %s, IsFree: %t, Price: %f, HasAccess: %t, IsPurchased: %t, IsPurchasedSnake: %t\n",
			ts.ID, ts.Title, ts.IsFree, ts.Price, ts.HasAccess, ts.IsPurchased, ts.IsPurchasedSnake)
	}
}
