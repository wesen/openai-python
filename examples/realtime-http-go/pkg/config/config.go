package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	StaticDir    string
	TemplatesDir string

	// TLS configuration
	TLSCert string
	TLSKey  string

	// OpenAI configuration
	OpenAIAPIKey       string
	OpenAIModelName    string
	OpenAIAPIBase      string
	OpenAIRequestLimit int
	OpenAIUseHTTP      bool

	// Audio configuration
	AudioSampleRate int
	AudioChannels   int
	AudioBitDepth   int
	MinBufferSize   int

	// Logging
	LogLevel string
}

// New creates a new configuration with values from environment variables
// or default values
func New() *Config {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY environment variable must be set")
	}

	return &Config{
		// Server configuration
		Port:         getEnvAsInt("PORT", 8000),
		ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
		StaticDir:    getEnv("STATIC_DIR", "web/static"),
		TemplatesDir: getEnv("TEMPLATES_DIR", "web/templates"),

		// TLS configuration
		TLSCert: getEnv("TLS_CERT", ""),
		TLSKey:  getEnv("TLS_KEY", ""),

		// OpenAI configuration
		OpenAIAPIKey:       apiKey,
		OpenAIModelName:    getEnv("OPENAI_MODEL_NAME", "gpt-4o-realtime-preview"),
		OpenAIAPIBase:      getEnv("OPENAI_API_BASE", "https://api.openai.com/v1"),
		OpenAIRequestLimit: getEnvAsInt("OPENAI_REQUEST_LIMIT", 10),
		OpenAIUseHTTP:      getEnvAsBool("OPENAI_USE_HTTP", false),

		// Audio configuration
		AudioSampleRate: getEnvAsInt("AUDIO_SAMPLE_RATE", 24000),
		AudioChannels:   getEnvAsInt("AUDIO_CHANNELS", 1),
		AudioBitDepth:   getEnvAsInt("AUDIO_BIT_DEPTH", 16),
		MinBufferSize:   getEnvAsInt("MIN_BUFFER_SIZE", 4800), // Default 100ms of audio at 24kHz, 16-bit, mono

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

// UsesTLS returns true if TLS is configured
func (c *Config) UsesTLS() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsDuration gets an environment variable as a duration or returns a default value
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool gets an environment variable as a boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}