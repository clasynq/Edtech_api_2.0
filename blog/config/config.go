package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config represents environmental settings required for running the Blog service.
type Config struct {
	Port        string // Port to bind the server to (e.g. 8086).
	DatabaseURL string // Connection URL for PostgreSQL.
	RedisURL    string // Redis server connection string.
	SecretKey   string // Signing secret key for JWT verification.
	BaseURL     string // Base domain/URL of the running application.
	MediaRoot   string // Path to the local directory where uploaded files are saved.
}

// LoadConfig loads parameters from .env files and active environment parameters,
// mapping them to the Config struct.
func LoadConfig() *Config {
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "./media"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8086" // fallback for direct blog service port
	}

	return &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		SecretKey:   os.Getenv("SECRET_KEY"),
		BaseURL:     baseURL,
		MediaRoot:   mediaRoot,
	}
}

