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
	SmtpHost          string
	SmtpPort          string
	SmtpUser          string
	SmtpPass          string
	DefaultFromEmail  string
}

func LoadConfig() *Config {
	// Attempt to load .env first
	_ = godotenv.Load(".env")
	_ = godotenv.Load()
	return &Config{
		Port:             os.Getenv("PORT"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisURL:         os.Getenv("REDIS_URL"),
		SecretKey:        os.Getenv("SECRET_KEY"),
		MediaRoot:        os.Getenv("MEDIA_ROOT"),
		BaseURL:          os.Getenv("BASE_URL"),
		SmtpHost:         os.Getenv("EMAIL_HOST"),
		SmtpPort:         os.Getenv("EMAIL_PORT"),
		SmtpUser:         os.Getenv("EMAIL_HOST_USER"),
		SmtpPass:         os.Getenv("EMAIL_HOST_PASSWORD"),
		DefaultFromEmail: os.Getenv("DEFAULT_FROM_EMAIL"),
	}
}
