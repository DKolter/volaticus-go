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
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Print configuration if in development
	if cfg.Env == "dev" {
		cfg.String()
	}

	// Initialize database
	db := database.New()

	// Run migrations
	if err := migrate.RunMigrations(db.DB()); err != nil {
		log.Printf("Failed to run migrations: %v", err)
		log.Println("Attempting to rollback migrations...")

		if rbErr := migrate.RollbackMigrations(db.DB()); rbErr != nil {
			log.Fatalf("Failed to rollback migrations after error: %v. Original error: %v", rbErr, err)
		}

		log.Fatalf("Migrations rolled back due to error: %v", err)
	}

	// Create and initialize server
	srv, err := server.NewServer(cfg, db)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	// Start HTTP server
	httpServer, err := srv.Start()
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		httpServer.SetKeepAlivesEnabled(false)
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
		close(done)
	}()

	// Start the server
	log.Printf("Server is ready to handle requests at :%d", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Could not start server: %v", err)
	}

	<-done
	log.Println("Server stopped")
}
