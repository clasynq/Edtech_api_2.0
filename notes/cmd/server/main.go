package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"clasynq/api/notes/config"
	delivery "clasynq/api/notes/internal/delivery/http"
	"clasynq/api/notes/internal/repository"
	"clasynq/api/notes/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	// 2. Load config
	cfg := config.LoadConfig()
	if cfg.Port == "" {
		cfg.Port = "8084" // default port for notes service
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

	// 3.5. Run DB migrations automatically on startup
	log.Println("Running database migrations...")
	migrations := []string{
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS recorded_class_url text",
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS subject varchar(255)",
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS topic varchar(255)",
		"ALTER TABLE notes ADD COLUMN IF NOT EXISTS prerequisite_url text",
		"ALTER TABLE notes ALTER COLUMN has_svgs SET DEFAULT false",
		"ALTER TABLE notes ALTER COLUMN page_count SET DEFAULT 0",
	}
	for _, sqlQuery := range migrations {
		if err := db.Exec(sqlQuery).Error; err != nil {
			log.Printf("Warning: failed to execute migration [%s]: %v", sqlQuery, err)
		}
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
	repo := repository.NewPostgresNoteRepository(db)
	uc := usecase.NewNoteUsecase(repo, cfg.BaseURL, rdb)
	authMiddleware := delivery.AuthMiddleware(cfg.SecretKey, rdb)
	optionalAuthMiddleware := delivery.OptionalAuthMiddleware(cfg.SecretKey)

	// 6. Setup Router and Run
	r := gin.Default()

	// Serve uploaded files statically for local development
	r.Static("/media", cfg.MediaRoot)
	log.Printf("Serving static files from directory %s on /media route", cfg.MediaRoot)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong from notes service",
		})
	})

	delivery.RegisterRoutes(r, uc, cfg.MediaRoot, cfg.BaseURL, authMiddleware, optionalAuthMiddleware)

	log.Printf("Starting notes service on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
