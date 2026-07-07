package config

import (
	"os"
	"github.com/joho/godotenv"
)

// Config holds environmental settings required for running the CBT exam service.
type Config struct {
	Port        string // Port to bind the server to (e.g. 8087).
	DatabaseURL string // PostgreSQL database connection string.
	RedisURL    string // Redis server connection string.
	SecretKey   string // HMAC secret key for verifying JWTs.
}

// LoadConfig loads settings from environmental variables and .env configuration.
func LoadConfig() *Config {
	_ = godotenv.Load() // optional local .env
	return &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		SecretKey:   os.Getenv("SECRET_KEY"),
	}
}

