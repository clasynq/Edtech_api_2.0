package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"clasynq/api/dashboard_profile/config"
	delivery "clasynq/api/dashboard_profile/internal/delivery/http"
	"clasynq/api/dashboard_profile/internal/repository"
	"clasynq/api/dashboard_profile/internal/usecase"

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
		cfg.Port = "8090" // Default port for the dashboard_profile service
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

	// 3. Connect to Redis (optional)
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err == nil {
			rdb = redis.NewClient(opt)
			log.Println("Connected to Redis for session validation")
		} else {
			log.Printf("failed to parse Redis URL: %v", err)
		}
	}

	// 4. Initialize layers
	repo := repository.NewPostgresProfileRepository(db)
	uc := usecase.NewProfileUsecase(repo)
	handler := delivery.NewHttpHandler(uc)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)

	// 5. Initialize router & routes
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong from dashboard_profile service"})
	})

	handler.RegisterRoutes(r, authMiddleware)

	// 6. Start server
	log.Printf("Starting dashboard_profile service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
