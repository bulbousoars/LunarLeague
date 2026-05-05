package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env      string
	LogLevel string

	HTTPBind string
	HTTPPort int

	SecretKey string

	DatabaseURL string
	RedisURL    string

	DataProvider          string
	SportsDataIOAPIKey    string

	SMTP SMTP

	PublicWebURL string
	PublicAPIURL string
}

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// AllowPlaintext disables TLS/STARTTLS (legacy internal relays only). Mailhog and port 1025 are always plaintext.
	AllowPlaintext bool
}

func Load() (*Config, error) {
	c := &Config{
		Env:                getenv("API_ENV", "dev"),
		LogLevel:           getenv("API_LOG_LEVEL", "info"),
		HTTPBind:           getenv("API_BIND", "0.0.0.0"),
		HTTPPort:           getenvInt("API_PORT", 8000),
		SecretKey:          os.Getenv("API_SECRET_KEY"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           getenv("REDIS_URL", "redis://redis:6379/0"),
		DataProvider:       getenv("DATA_PROVIDER", "sleeper"),
		SportsDataIOAPIKey: os.Getenv("SPORTSDATAIO_API_KEY"),
		PublicWebURL:       getenv("PUBLIC_WEB_URL", "http://localhost:3000"),
		PublicAPIURL:       getenv("PUBLIC_API_URL", "http://localhost:8000"),
		SMTP: SMTP{
			Host:           getenv("SMTP_HOST", "mailhog"),
			Port:           getenvInt("SMTP_PORT", 1025),
			Username:       os.Getenv("SMTP_USERNAME"),
			Password:       os.Getenv("SMTP_PASSWORD"),
			From:           getenv("SMTP_FROM", "Lunar League <no-reply@lunarleague.local>"),
			AllowPlaintext: getenvBool("SMTP_ALLOW_PLAINTEXT", false),
		},
	}

	if c.SecretKey == "" {
		return nil, errors.New("API_SECRET_KEY is required")
	}
	if len(c.SecretKey) < 32 {
		return nil, fmt.Errorf("API_SECRET_KEY must be at least 32 chars (got %d)", len(c.SecretKey))
	}
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	switch c.DataProvider {
	case "sleeper":
	case "sportsdataio":
		if c.SportsDataIOAPIKey == "" {
			return nil, errors.New("SPORTSDATAIO_API_KEY is required when DATA_PROVIDER=sportsdataio")
		}
	default:
		return nil, fmt.Errorf("unknown DATA_PROVIDER %q", c.DataProvider)
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}
