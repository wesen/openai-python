package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/openai/realtime-http-go/internal/handler"
	"github.com/openai/realtime-http-go/pkg/audio"
	"github.com/openai/realtime-http-go/pkg/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		// Just log, don't fail if .env doesn't exist
		fmt.Println("No .env file found")
	}

	// Setup logger
	setupLogger()
	log.Info().Msg("Starting OpenAI Realtime HTTP server")

	// Check dependencies
	checkDependencies()

	// Load configuration
	cfg := config.New()

	// Initialize server
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Create a context that will be canceled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up routes and handler
	h := handler.NewHandler(cfg)
	h.RegisterRoutes(mux)

	// Start server in a goroutine so it doesn't block the shutdown handling
	go func() {
		var err error
		if cfg.UsesTLS() {
			log.Info().Int("port", cfg.Port).Msg("HTTPS server started")
			err = server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			log.Info().Int("port", cfg.Port).Msg("HTTP server started")
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Create a deadline for server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown handler
	if err := h.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Handler shutdown error")
	}

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited properly")
}

// setupLogger configures the global logger
func setupLogger() {
	// Determine log level from environment
	levelStr := getEnv("LOG_LEVEL", "info")
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		level = zerolog.InfoLevel
	}

	// Configure zerolog
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

// checkDependencies checks if all required dependencies are installed
func checkDependencies() {
	// Check if FFmpeg is installed
	if !audio.CheckFFmpegInstalled() {
		log.Warn().Msg("FFmpeg not found in PATH. Audio functionality will be limited.")
		log.Warn().Msg("Please install FFmpeg for full functionality: https://ffmpeg.org/download.html")
	} else {
		log.Info().Msg("FFmpeg found, audio processing enabled")
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}