package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"clasynq/api/blog/config"
	delivery "clasynq/api/blog/internal/delivery/http"
	"clasynq/api/blog/internal/repository"
	"clasynq/api/blog/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// 1. Load config
	cfg := config.LoadConfig()
	if cfg.Port == "" {
		cfg.Port = "8086" // Default port for the blog service
	}

	// 2. Connect to Postgres
	log.Printf("Connecting to Postgres at: %s", cfg.DatabaseURL)
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: dbLogger,
	})

	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// 3. Connect to Redis (optional/fail-safe open)
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err == nil {
			rdb = redis.NewClient(opt)
			log.Println("Connected to Redis for active session limiting")
		} else {
			log.Printf("failed to parse Redis URL: %v", err)
		}
	}

	// 4. Initialize layers
	repo := repository.NewPostgresBlogRepository(db)
	uc := usecase.NewBlogUsecase(repo)
	handler := delivery.NewHttpHandler(uc, cfg.MediaRoot, cfg.BaseURL)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)
	optionalAuthMiddleware := delivery.OptionalAuthMiddleware(cfg.SecretKey)

	// 5. Initialize router & routes
	r := gin.Default()

	// Serve uploaded media statically for development/testing
	r.Static("/media", cfg.MediaRoot)
	log.Printf("Serving static files from directory %s on /media route", cfg.MediaRoot)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong from blog service"})
	})

	handler.RegisterRoutes(r, authMiddleware, optionalAuthMiddleware)

	// 6. Start server
	log.Printf("Starting blog service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
