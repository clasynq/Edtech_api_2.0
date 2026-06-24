package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"clasynq/api/enrollments/config"
	delivery "clasynq/api/enrollments/internal/delivery/http"
	"clasynq/api/enrollments/internal/repository"
	"clasynq/api/enrollments/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	// 2. Load config
	cfg := config.LoadConfig()
	if cfg.Port == "" {
		cfg.Port = "8083" // default port for enrollments service
	}

	// SSLMode check / disable for local pg if not specified
	dbURL := cfg.DatabaseURL
	if dbURL != "" && !strings.Contains(dbURL, "sslmode=") {
		if strings.Contains(dbURL, "?") {
			dbURL = dbURL + "&sslmode=disable"
		} else {
			dbURL = dbURL + "?sslmode=disable"
		}
	}

	// 3. Connect to Postgres GORM
	log.Printf("Connecting to Postgres at: %s", dbURL)
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// 4. Connect to Redis (optional)
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err == nil {
			rdb = redis.NewClient(opt)
			log.Println("Connected to Redis for session checks")
		} else {
			log.Printf("failed to parse Redis URL: %v", err)
		}
	}

	// 5. Initialize Layers
	repo := repository.NewPostgresEnrollmentRepository(db)
	uc := usecase.NewEnrollmentUsecase(repo, cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)

	// 6. Launch Background Worker for Referral Transactions
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Println("Starting background referral rewards crediting worker...")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("Stopping referral rewards worker...")
				return
			case <-ticker.C:
				if err := uc.ProcessPendingReferrals(ctx); err != nil {
					log.Printf("Referral worker error: %v", err)
				}
			}
		}
	}()

	// 7. Setup Router and Run
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong from enrollments service",
		})
	})

	delivery.RegisterRoutes(r, uc, authMiddleware)

	log.Printf("Starting enrollments service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
