package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"volaticus-go/internal/config"
	"volaticus-go/internal/logger"

	"volaticus-go/internal/database"
	"volaticus-go/internal/database/migrate"
	"volaticus-go/internal/server"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Volaticus %s\n", formatVersionInfo())
		return
	}

	// Initialize logger first
	env := os.Getenv("APP_ENV")
	switch env {
	case "local", "development":
		logger.Init("development") // Debug Level
	case "production":
		logger.Init("production") // Info Level
	default:
		logger.Init("development") // Fallback to Debug Level
	}

	log.Info().
		Str("environment", env).
		Str("log_level", zerolog.GlobalLevel().String()).
		Str("version", version).
		Str("commit", commit).
		Str("built", date).
		Msg("Starting Volaticus")

	// Create a base context for the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading configuration")
	}

	// Update logger with correct environment
	logger.Init(cfg.Env)

	// Initialize database with the new implementation
	db, err := database.NewFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database connection")
		}
	}()

	// Run database health check
	if health := db.Health(ctx); health["status"] != "up" {
		log.Fatal().
			Interface("error", health["error"]).
			Msg("Database health check failed")
	}

	// Run migrations
	if err := migrate.RunMigrations(db.DB); err != nil {
		log.Error().Err(err).Msg("Failed to run migrations")
		log.Info().Msg("Attempting to rollback migrations...")

		if rbErr := migrate.RollbackMigrations(db.DB); rbErr != nil {
			log.Fatal().
				Err(rbErr).
				Str("original_error", err.Error()).
				Msg("Failed to rollback migrations after error")
		}

		log.Fatal().Err(err).Msg("Migrations rolled back due to error")
	}

	// Create and initialize server with the new database instance
	srv, err := server.NewServer(cfg, db)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating server")
	}

	// Start HTTP server
	httpServer, err := srv.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Error starting server")
	}

	// Set up graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	go func() {
		<-shutdown
		log.Info().Msg("Shutdown signal received")

		// Create a timeout context for graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
		defer shutdownCancel()

		// Disable keep-alives for new connections
		httpServer.SetKeepAlivesEnabled(false)

		// Shut down the HTTP server
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("HTTP server shutdown error")
		}

		// Cancel the main context
		cancel()
	}()

	// Start the server
	log.Info().
		Str("url", cfg.BaseURL).
		Msg("Server is ready to handle requests")

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("HTTP server error")
	}

	// Wait for context cancellation (shutdown complete)
	<-ctx.Done()
	log.Info().Msg("Server shutdown completed")
}

func formatVersionInfo() string {
	return fmt.Sprintf(`Version: %s
Commit: %s
Built: %s`, version, commit, date)
}
