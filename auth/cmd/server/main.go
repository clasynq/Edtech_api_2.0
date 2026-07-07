package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"clasynq/api/auth/config"
	delivery "clasynq/api/auth/internal/delivery/http"
	"clasynq/api/auth/internal/repository"
	"clasynq/api/auth/internal/scheduler"
	"clasynq/api/auth/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// main is the application entrypoint for the Auth & Accounts Microservice.
// It loads configuration, connects to the relational database (PostgreSQL) and the cache (Redis),
// builds the Clean Architecture dependency layers, starts background workers, and spins up the HTTP server.
func main() {
	// 1. Load configuration from environment variables or .env files.
	cfg := config.LoadConfig()
	if cfg.Port == "" {
		cfg.Port = "8081" // Default port for the auth microservice
	}

	log.Printf("Connecting to Postgres at: %s", cfg.DatabaseURL)
	
	// 2. Setup database logger and connect to PostgreSQL database using GORM ORM.
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

	// 3. Connect to Redis (used for tracking and limiting active JWT user sessions/OTP verification rate limits).
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

	// 4. Initialize Clean Architecture dependency injection layers.
	// - Repository Layer: Directly interacts with PostgreSQL/GORM.
	// - Usecase Layer: Core business logic (Registration, Login, OTP, etc.).
	// - Delivery/HTTP Layer: Router handoffs, JWT authentication middleware, request handlers.
	repo := repository.NewPostgresUserRepository(db)
	uc := usecase.NewUserUsecase(repo, rdb, cfg)
	handler := delivery.NewHttpHandler(uc, cfg.SecretKey, cfg.TurnstileSecretKey, rdb)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)

	// Start Birthday Wish background scheduler to run periodic check jobs.
	scheduler.StartBirthdayWishScheduler(db, cfg)

	// 5. Initialize the Gin HTTP router, add health check ping endpoint, and register HTTP routes.
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong from auth service"})
	})

	handler.RegisterRoutes(r, authMiddleware)

	// 6. Start the Gin HTTP server on the configured port.
	log.Printf("Starting auth service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

