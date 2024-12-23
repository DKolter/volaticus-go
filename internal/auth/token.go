package auth

import (
	"time"

	"github.com/google/uuid"
)

type APIToken struct {
	ID         uuid.UUID  `db:"id" json:"id"`
	UserID     uuid.UUID  `db:"user_id" json:"user_id"`
	Name       string     `db:"name" json:"name"`
	Token      string     `db:"token" json:"-"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	RevokedAt  *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
	IsActive   bool       `db:"is_active" json:"is_active"`
}

type TokenRepository interface {
	CreateToken(token *APIToken) error
	GetAPITokenByToken(token string) (*APIToken, error)
	GetAPITokenByID(id uuid.UUID) (*APIToken, error)
	ListUserTokens(userID uuid.UUID) ([]*APIToken, error)
	TokenExists(token string) (bool, error)
	RevokeToken(id uuid.UUID) error
	UpdateLastUsed(id uuid.UUID) error
	DeleteTokenByUserIdAndToken(userID uuid.UUID, token string) error
}

type CreateTokenRequest struct {
	Name   string    `json:"name" validate:"required"`
	UserID uuid.UUID `json:"userid" validate:"required"`
}

type TokenResponse struct {
	Token string    `json:"token"`
	Name  string    `json:"name"`
	ID    uuid.UUID `json:"id"`
}
