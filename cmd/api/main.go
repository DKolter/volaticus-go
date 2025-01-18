package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"volaticus-go/internal/config"

	"volaticus-go/internal/database"
	"volaticus-go/internal/database/migrate"
	"volaticus-go/internal/server"
)

func main() {
	// Create a base context for the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Print configuration if in development
	if cfg.Env == "dev" {
		cfg.String()
	}

	// Initialize database with the new implementation
	db, err := database.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	// Run database health check
	if health := db.Health(ctx); health["status"] != "up" {
		log.Fatalf("Database health check failed: %v", health["error"])
	}

	// Run migrations
	if err := migrate.RunMigrations(db.DB); err != nil {
		log.Printf("Failed to run migrations: %v", err)
		log.Println("Attempting to rollback migrations...")

		if rbErr := migrate.RollbackMigrations(db.DB); rbErr != nil {
			log.Fatalf("Failed to rollback migrations after error: %v. Original error: %v", rbErr, err)
		}

		log.Fatalf("Migrations rolled back due to error: %v", err)
	}

	// Create and initialize server with the new database instance
	srv, err := server.NewServer(cfg, db)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	// Start HTTP server
	httpServer, err := srv.Start()
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	go func() {
		<-shutdown
		log.Println("Shutdown signal received")

		// Create a timeout context for graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
		defer shutdownCancel()

		// Disable keep-alives for new connections
		httpServer.SetKeepAlivesEnabled(false)

		// Shut down the HTTP server
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		// Cancel the main context
		cancel()
	}()

	// Start the server
	log.Printf("Server is ready to handle requests at : %s:%d", cfg.BaseURL, cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP server error: %v", err)
	}

	// Wait for context cancellation (shutdown complete)
	<-ctx.Done()
	log.Println("Server shutdown completed")
}
