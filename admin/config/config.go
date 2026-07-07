package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds environmental settings required for running the Admin service.
type Config struct {
	Port             string // Port to bind the server to (e.g. 8088).
	DatabaseURL      string // Connection URL for PostgreSQL.
	RedisURL         string // Redis server connection string.
	SecretKey        string // Signing secret key for administrative JWT tokens.
	MediaRoot        string // Path to the local directory where uploaded assets (like avatars, course files) are saved.
	BaseURL          string // Base domain/URL of the running application (e.g., http://localhost:8088).
	SmtpHost         string // SMTP server host address.
	SmtpPort         string // SMTP server port (usually 587 or 465).
	SmtpUser         string // Authorized SMTP user name.
	SmtpPass         string // SMTP server account password.
	DefaultFromEmail string // Sender email visible in sent mails.
}

// LoadConfig reads variables from .env files and active environment parameters,
// mapping them to the Config struct for clean service injection.
func LoadConfig() *Config {
	// Attempt to load .env from the execution directory and current path.
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

