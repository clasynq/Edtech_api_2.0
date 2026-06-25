package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DatabaseURL       string
	RedisURL          string
	SecretKey         string
	MediaRoot         string
	BaseURL           string
}

func LoadConfig() *Config {
	// Attempt to load .env first
	_ = godotenv.Load(".env")
	_ = godotenv.Load()
	return &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		SecretKey:   os.Getenv("SECRET_KEY"),
		MediaRoot:   os.Getenv("MEDIA_ROOT"),
		BaseURL:     os.Getenv("BASE_URL"),
	}
}
