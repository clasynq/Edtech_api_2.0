package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"clasynq/api/admin/config"
	delivery "clasynq/api/admin/internal/delivery/http"
	"clasynq/api/admin/internal/repository"
	"clasynq/api/admin/internal/usecase"

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
		cfg.Port = "8088" // default port for admin service
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

	// 3. Connect to Redis (optional/fail-safe)
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err == nil {
			rdb = redis.NewClient(opt)
			log.Println("Connected to Redis for caching")
		} else {
			log.Printf("failed to parse Redis URL: %v", err)
		}
	}

	// 4. Initialize Layers
	repo := repository.NewPostgresAdminRepository(db)
	uc := usecase.NewAdminUsecase(repo, rdb, cfg.SmtpHost, cfg.SmtpPort, cfg.SmtpUser, cfg.SmtpPass, cfg.DefaultFromEmail)
	handler := delivery.NewHttpHandler(uc, cfg.SecretKey, cfg.MediaRoot, cfg.BaseURL)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey)

	// 5. Initialize Router
	r := gin.Default()

	// Serve uploaded files statically for local development
	r.Static("/media", cfg.MediaRoot)
	log.Printf("Serving static files from directory %s on /media route", cfg.MediaRoot)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong from admin service"})
	})

	handler.RegisterRoutes(r, authMiddleware)

	// 6. Start server
	log.Printf("Starting admin service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
