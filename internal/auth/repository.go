package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"

	"github.com/google/uuid"
)

// Repository defines methods for token persistence
type Repository interface {
	// CreateToken creates a new API token
	CreateToken(ctx context.Context, token *models.APIToken) error
	// GetAPITokenByToken retrieves a token by its value
	GetAPITokenByToken(ctx context.Context, token string) (*models.APIToken, error)
	// GetAPITokenByID retrieves a token by its ID
	GetAPITokenByID(ctx context.Context, id uuid.UUID) (*models.APIToken, error)
	// ListUserTokens retrieves all tokens for a user
	ListUserTokens(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error)
	// TokenExists checks if a token value already exists
	TokenExists(ctx context.Context, token string) (bool, error)
	// RevokeToken marks a token as revoked
	RevokeToken(ctx context.Context, id uuid.UUID) error
	// UpdateLastUsed updates the last used timestamp
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	// DeleteTokenByUserIdAndToken deletes a token by user ID and token value
	DeleteTokenByUserIdAndToken(ctx context.Context, userID uuid.UUID, token string) error
}

type repository struct {
	*database.Repository
}

// NewRepository creates a new token repository
func NewRepository(db *database.DB) Repository {
	return &repository{
		Repository: database.NewRepository(db),
	}
}

func (r *repository) CreateToken(ctx context.Context, token *models.APIToken) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Check if token exists
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM api_tokens WHERE token = $1)`
		if err := tx.GetContext(ctx, &exists, query, token.Token); err != nil {
			return fmt.Errorf("checking token existence: %w", err)
		}
		if exists {
			return ErrTokenExists
		}

		// Insert token
		insertQuery := `
            INSERT INTO api_tokens (id, user_id, name, token, created_at, is_active)
            VALUES ($1, $2, $3, $4, NOW(), $5) RETURNING id`
		if err := tx.GetContext(ctx, &token.ID, insertQuery, token.ID, token.UserID, token.Name, token.Token, token.IsActive); err != nil {
			return fmt.Errorf("creating token: %w", err)
		}

		return nil
	})
}

func (r *repository) GetAPITokenByToken(ctx context.Context, tokenStr string) (*models.APIToken, error) {
	query := `SELECT * FROM api_tokens WHERE token = $1`

	var token models.APIToken
	if err := r.Get(ctx, &token, query, tokenStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("getting token: %w", err)
	}

	return &token, nil
}

func (r *repository) GetAPITokenByID(ctx context.Context, id uuid.UUID) (*models.APIToken, error) {
	query := `SELECT * FROM api_tokens WHERE id = $1`

	var token models.APIToken
	if err := r.Get(ctx, &token, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("getting token by ID: %w", err)
	}

	return &token, nil
}

func (r *repository) ListUserTokens(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	query := `SELECT * FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`
	var tokens []*models.APIToken
	err := r.Select(ctx, &tokens, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user tokens: %w", err)
	}
	return tokens, nil
}

func (r *repository) TokenExists(ctx context.Context, tokenStr string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM api_tokens WHERE token = $1)`
	if err := r.Get(ctx, &exists, query, tokenStr); err != nil {
		return false, fmt.Errorf("checking token existence: %w", err)
	}
	return exists, nil
}

func (r *repository) RevokeToken(ctx context.Context, id uuid.UUID) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		query := `UPDATE api_tokens SET is_active = false, revoked_at = $1 WHERE id = $2`
		result, err := tx.ExecContext(ctx, query, time.Now(), id)
		if err != nil {
			return fmt.Errorf("revoking token: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("getting rows affected: %w", err)
		}
		if rows == 0 {
			return ErrTokenNotFound
		}
		return nil
	})
}

func (r *repository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		query := `UPDATE api_tokens SET last_used_at = $1 WHERE id = $2`
		result, err := tx.ExecContext(ctx, query, time.Now(), id)
		if err != nil {
			return fmt.Errorf("updating last used: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("getting rows affected: %w", err)
		}
		if rows == 0 {
			return ErrTokenNotFound
		}
		return nil
	})
}

func (r *repository) DeleteTokenByUserIdAndToken(ctx context.Context, userID uuid.UUID, tokenStr string) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		query := `DELETE FROM api_tokens WHERE user_id = $1 AND token = $2`
		result, err := tx.ExecContext(ctx, query, userID, tokenStr)
		if err != nil {
			return fmt.Errorf("deleting token: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("getting rows affected: %w", err)
		}
		if rows == 0 {
			return ErrTokenNotFound
		}
		return nil
	})
}
