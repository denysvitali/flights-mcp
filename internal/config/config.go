// Package config handles configuration management for the flights-mcp server.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration values for the application.
type Config struct {
	// Server settings
	ServerName    string
	ServerVersion string
	LogLevel      string

	// Scraper settings
	RequestTimeout time.Duration
	MaxRetries     int
	RetryDelay     time.Duration

	// Rate limiting
	RateLimitRequests int
	RateLimitWindow   time.Duration

	// Anti-bot settings
	ProxyURL string

	// Airports
	AirportsFile string
}

// Load loads configuration from environment variables with defaults.
func Load() *Config {
	return &Config{
		// Server settings
		ServerName:    getEnv("SERVER_NAME", "flights-mcp"),
		ServerVersion: getEnv("SERVER_VERSION", "1.0.0"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),

		// Scraper settings
		RequestTimeout: getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
		MaxRetries:     getEnvInt("MAX_RETRIES", 3),
		RetryDelay:     getEnvDuration("RETRY_DELAY", 2*time.Second),

		// Rate limiting
		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 60),
		RateLimitWindow:   getEnvDuration("RATE_LIMIT_WINDOW", 60*time.Second),

		// Anti-bot settings
		ProxyURL: getEnv("PROXY_URL", ""),

		// Airports (empty = use the database embedded in the binary)
		AirportsFile: getEnv("AIRPORTS_FILE", ""),
	}
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt returns the value of an environment variable as an int or a default value.
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// getEnvDuration returns the value of an environment variable as a duration or a default value.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if durVal, err := time.ParseDuration(val); err == nil {
			return durVal
		}
	}
	return defaultVal
}
