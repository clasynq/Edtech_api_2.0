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
	EmailHost         string
	EmailPort         string
	EmailHostUser     string
	EmailHostPassword string
	DefaultFromEmail  string
	TurnstileSecretKey string
}

func LoadConfig() *Config {
	// Attempt to load root .env first
	_ = godotenv.Load(".env")
	_ = godotenv.Load()
	return &Config{
		Port:              os.Getenv("PORT"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisURL:          os.Getenv("REDIS_URL"),
		SecretKey:         os.Getenv("SECRET_KEY"),
		EmailHost:         os.Getenv("EMAIL_HOST"),
		EmailPort:         os.Getenv("EMAIL_PORT"),
		EmailHostUser:     os.Getenv("EMAIL_HOST_USER"),
		EmailHostPassword: os.Getenv("EMAIL_HOST_PASSWORD"),
		DefaultFromEmail:  os.Getenv("DEFAULT_FROM_EMAIL"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
	}
}
