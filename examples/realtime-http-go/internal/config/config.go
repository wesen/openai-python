package config

import (
	"os"
)

// Config holds the application configuration
type Config struct {
	// ListenAddr is the address and port the server listens on
	ListenAddr string

	// OpenAIAPIKey is the API key for OpenAI
	OpenAIAPIKey string

	// LogLevel is the logging level
	LogLevel string

	// OpenAIModel is the OpenAI model to use for the realtime API
	OpenAIModel string
}

// LoadConfig loads the application configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		ListenAddr:   getEnv("LISTEN_ADDR", "0.0.0.0:8000"),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		OpenAIModel:  getEnv("OPENAI_MODEL", "gpt-4o-realtime-preview"),
	}

	return cfg
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
