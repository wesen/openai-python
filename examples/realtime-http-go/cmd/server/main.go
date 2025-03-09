package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openai/realtime-http-go/internal/config"
	"github.com/openai/realtime-http-go/internal/server"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Setup logger
	log.Printf("Starting OpenAI Realtime HTTP server")

	// Create server
	srv := server.NewServer(cfg)

	// Start HTTP server in a goroutine
	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Router,
	}

	go func() {
		log.Printf("HTTP server listening on %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
