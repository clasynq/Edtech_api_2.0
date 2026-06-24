package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	SecretKey   string
	BaseURL     string
	MediaRoot   string
}

func LoadConfig() *Config {
	_ = godotenv.Load(".env")
	_ = godotenv.Load()

	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "./media" // default local fallback
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8082" // fallback for direct courses service URL
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
