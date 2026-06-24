package main

import (
	"log"
	"net/http"
	"strings"

	"clasynq/api/test_series/config"
	delivery "clasynq/api/test_series/internal/delivery/http"
	"clasynq/api/test_series/internal/repository"
	"clasynq/api/test_series/internal/usecase"

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
		cfg.Port = "8085" // default port for test_series service
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
	repo := repository.NewPostgresTestSeriesRepository(db)
	uc := usecase.NewTestSeriesUsecase(repo)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)
	optionalAuthMiddleware := delivery.OptionalAuthMiddleware(cfg.SecretKey)

	// 6. Setup Router and Run
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong from test_series service",
		})
	})

	delivery.RegisterRoutes(r, uc, authMiddleware, optionalAuthMiddleware)

	log.Printf("Starting test_series service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
