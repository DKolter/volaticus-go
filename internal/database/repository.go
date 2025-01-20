package database

import "fmt"

import (
	"context"
	"database/sql"
	"github.com/jmoiron/sqlx"
)

// Repository provides common database operations
type Repository struct {
	db *DB
}

// NewRepository creates a new repository instance
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// QueryRow executes a query that expects a single row result
func (r *Repository) QueryRow(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return r.db.QueryRowxContext(ctx, query, args...)
}

// Query executes a query that returns multiple rows
func (r *Repository) Query(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return r.db.QueryxContext(ctx, query, args...)
}

// Exec executes a query without returning any rows
func (r *Repository) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// Get selects a single row into a destination struct
func (r *Repository) Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return r.db.GetContext(ctx, dest, query, args...)
}

// Select selects multiple rows into a slice destination
func (r *Repository) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return r.db.SelectContext(ctx, dest, query, args...)
}

// WithTx executes operations within a transaction
func (r *Repository) WithTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				panic(fmt.Sprintf("panic during transaction: %v, rollback failed: %v", p, rbErr))
			}
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

// Error wraps repository errors with context
func (r *Repository) Error(op string, err error) error {
	return fmt.Errorf("repository %s: %w", op, err)
}
