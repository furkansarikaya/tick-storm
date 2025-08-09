package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/server"
)

func main() {
	// Load configuration
	config := server.DefaultConfig()
	server.LoadConfigFromEnv(config)

	// Create server
	srv := server.NewServer(config)

	// Start server
	log.Printf("Starting Tick-Storm TCP server on %s", config.ListenAddr)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Get actual listen address (useful when using port 0)
	log.Printf("Server listening on %s", srv.ListenAddr())

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	// Print final stats
	stats := srv.GetStats()
	fmt.Println("\nFinal server statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}

	log.Println("Server stopped")
}
