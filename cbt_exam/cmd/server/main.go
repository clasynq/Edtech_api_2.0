package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"clasynq/api/cbt_exam/config"
	delivery "clasynq/api/cbt_exam/internal/delivery/http"
	"clasynq/api/cbt_exam/internal/repository"
	"clasynq/api/cbt_exam/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// main is the server entrypoint for the CBT (Computer Based Test) Exam microservice.
// It initializes DB connections, config, dependency injection layers,
// parses and enforces SSL options, and runs the server.
func main() {
	// 1. Load dotenv variables
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	// 2. Load configuration details
	cfg := config.LoadConfig()
	if cfg.Port == "" {
		cfg.Port = "8087" // default port for cbt_exam service
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

	// 3. Connect to primary PostgreSQL instance
	log.Printf("Connecting to Postgres at: %s", dbURL)
	dbLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: dbLogger,
	})
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

	// 5. Initialize Clean Architecture dependency injection layers
	repo := repository.NewPostgresCbtExamRepository(db)
	uc := usecase.NewCbtExamUsecase(repo, rdb)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)
	optionalAuthMiddleware := delivery.OptionalAuthMiddleware(cfg.SecretKey)

	// 6. Setup Router and Run
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong from cbt_exam service",
		})
	})

	delivery.RegisterRoutes(r, uc, authMiddleware, optionalAuthMiddleware)

	log.Printf("Starting cbt_exam service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

