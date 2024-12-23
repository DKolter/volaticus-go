package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type postgresTokenRepository struct {
	db *sqlx.DB
}

func NewPostgresTokenRepository(db *sqlx.DB) TokenRepository {
	return &postgresTokenRepository{db: db}
}

func (r *postgresTokenRepository) CreateToken(token *APIToken) error {
	query := `
        INSERT INTO api_tokens (id, user_id, name, token, created_at, is_active)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id`

	return r.db.QueryRow(
		query,
		token.ID,
		token.UserID,
		token.Name,
		token.Token,
		token.CreatedAt,
		token.IsActive,
	).Scan(&token.ID)
}

func (r *postgresTokenRepository) GetAPITokenByToken(token string) (*APIToken, error) {
	var t APIToken
	err := r.db.Get(&t, "SELECT * FROM api_tokens WHERE token = $1", token)
	if err == sql.ErrNoRows {
		return nil, errors.New("token not found")
	}
	return &t, err
}

func (r *postgresTokenRepository) GetAPITokenByID(id uuid.UUID) (*APIToken, error) {
	var t APIToken
	err := r.db.Get(&t, "SELECT * FROM api_tokens WHERE id = $1", id)
	if err == sql.ErrNoRows {
		return nil, errors.New("token not found")
	}
	return &t, err
}

func (r *postgresTokenRepository) ListUserTokens(userID uuid.UUID) ([]*APIToken, error) {
	var tokens []*APIToken
	err := r.db.Select(&tokens, "SELECT * FROM api_tokens WHERE user_id = $1", userID)
	return tokens, err
}

func (r *postgresTokenRepository) RevokeToken(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(
		"UPDATE api_tokens SET is_active = false, revoked_at = $1 WHERE id = $2",
		now,
		id,
	)
	return err
}

func (r *postgresTokenRepository) UpdateLastUsed(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(
		"UPDATE api_tokens SET last_used_at = $1 WHERE id = $2",
		now,
		id,
	)
	return err
}

func (r *postgresTokenRepository) TokenExists(token string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM api_tokens WHERE token = $1)"

	err := r.db.Get(&exists, query, token)
	if err != nil {
		return false, fmt.Errorf("failed to check token existence: %w", err)
	}

	return exists, nil
}

func (r *postgresTokenRepository) DeleteTokenByUserIdAndToken(userID uuid.UUID, token string) error {
	query := `
        DELETE FROM api_tokens
        WHERE user_id = $1 AND token = $2
    `

	result, err := r.db.Exec(query, userID, token)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("Token not found:")
	}

	return nil
}
