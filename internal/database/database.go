package database

import (
	"context"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// DB represents a database instance and implements Service
type DB struct {
	*sqlx.DB
}

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	Schema   string
}

// New creates a new database connection
func New(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Schema)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Info().
		Str("host", cfg.Host).
		Str("port", cfg.Port).
		Str("database", cfg.Database).
		Str("schema", cfg.Schema).
		Int("max_open_conns", 25).
		Int("max_idle_conns", 5).
		Dur("conn_max_lifetime", 5*time.Minute).
		Msg("database connection established")

	return &DB{DB: db}, nil
}

// NewFromEnv creates a new database connection using environment variables
func NewFromEnv() (*DB, error) {
	cfg := Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Database: os.Getenv("DB_DATABASE"),
		Username: os.Getenv("DB_USERNAME"),
		Password: os.Getenv("DB_PASSWORD"),
		Schema:   os.Getenv("DB_SCHEMA"),
	}
	return New(cfg)
}

// Health returns database health information
func (db *DB) Health(ctx context.Context) map[string]string {
	stats := make(map[string]string)

	// Check database connectivity
	if err := db.PingContext(ctx); err != nil {
		log.Error().
			Err(err).
			Msg("database health check failed")
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("database ping failed: %v", err)
		return stats
	}

	// Get database stats
	dbStats := db.Stats()
	stats["status"] = "up"
	stats["open_connections"] = fmt.Sprintf("%d", dbStats.OpenConnections)
	stats["in_use"] = fmt.Sprintf("%d", dbStats.InUse)
	stats["idle"] = fmt.Sprintf("%d", dbStats.Idle)

	log.Info().
		Int("open_connections", dbStats.OpenConnections).
		Int("in_use", dbStats.InUse).
		Int("idle", dbStats.Idle).
		Msg("database health check completed")

	return stats
}

// Close closes the database connection
func (db *DB) Close() error {
	if err := db.DB.Close(); err != nil {
		log.Error().
			Err(err).
			Msg("error closing database connection")
		return fmt.Errorf("closing database connection: %w", err)
	}

	log.Info().Msg("database connection closed successfully")
	return nil
}

// WithTx executes a function within a transaction
func (db *DB) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		log.Error().
			Err(err).
			Msg("failed to begin transaction")
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// A panic occurred, rollback and repanic
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Error().
					Err(rbErr).
					Interface("panic", p).
					Msg("failed to rollback transaction after panic")
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Error().
				Err(rbErr).
				Err(err).
				Msg("failed to rollback transaction")
			return fmt.Errorf("rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Error().
			Err(err).
			Msg("failed to commit transaction")
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
