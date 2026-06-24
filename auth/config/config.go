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
}

func LoadConfig() *Config {
	_ = godotenv.Load() // optional local .env
	return &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		SecretKey:   os.Getenv("SECRET_KEY"),
	}
}
