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
	MediaRoot   string
	BaseURL     string
}

func LoadConfig() *Config {
	_ = godotenv.Load() // optional local .env
	
	mediaRoot := os.Getenv("MEDIA_ROOT")
	if mediaRoot == "" {
		mediaRoot = "./media"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000" // default base url / gateway
	}

	return &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		SecretKey:   os.Getenv("SECRET_KEY"),
		MediaRoot:   mediaRoot,
		BaseURL:     baseURL,
	}
}
