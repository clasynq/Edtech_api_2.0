package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds all environmental configurations required to run the Auth service.
type Config struct {
	Port               string // Port that the HTTP web server will bind to (e.g. 8081).
	DatabaseURL        string // Connection URI for the PostgreSQL database instance.
	RedisURL           string // Connection URI for the Redis cache/session manager.
	SecretKey          string // Symmetric key used for JWT signing and verification.
	EmailHost          string // SMTP server hostname for sending registration/2FA/reset emails.
	EmailPort          string // SMTP server port (usually 587 or 465).
	EmailHostUser      string // Authorized SMTP user name (usually the email itself).
	EmailHostPassword  string // Authorized SMTP password or API token.
	DefaultFromEmail   string // Display sender address shown in email headers.
	TurnstileSecretKey string // Cloudflare Turnstile CAPTCHA secret key for register spam protection.
}

// LoadConfig loads variables from a local or root .env file and environment scope,
// mapping them to a Config structure for clean, structured usage across the codebase.
func LoadConfig() *Config {
	// Attempt to load .env from the root and execution directory paths.
	_ = godotenv.Load(".env")
	_ = godotenv.Load()
	return &Config{
		Port:               os.Getenv("PORT"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           os.Getenv("REDIS_URL"),
		SecretKey:          os.Getenv("SECRET_KEY"),
		EmailHost:          os.Getenv("EMAIL_HOST"),
		EmailPort:          os.Getenv("EMAIL_PORT"),
		EmailHostUser:      os.Getenv("EMAIL_HOST_USER"),
		EmailHostPassword:  os.Getenv("EMAIL_HOST_PASSWORD"),
		DefaultFromEmail:   os.Getenv("DEFAULT_FROM_EMAIL"),
		TurnstileSecretKey: os.Getenv("TURNSTILE_SECRET_KEY"),
	}
}

