package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"time"
	"volaticus-go/internal/common/models"
)

type Service interface {
	GetAuth() *jwtauth.JWTAuth
	GenerateToken(user *models.User) (string, error)
	GenerateAPIToken(ctx context.Context, userID uuid.UUID, name string) (*models.APIToken, error)
	ValidateAPIToken(ctx context.Context, token string) (*models.APIToken, error)
	DeleteTokenByUserIdAndToken(ctx context.Context, userID uuid.UUID, token string) error
	GetUserAPITokens(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error)
}
type authService struct {
	tokenAuth *jwtauth.JWTAuth
	repo      Repository
	secretKey []byte
}

const TokenExpiry = time.Hour * 24 // 24 hours TODO: implement refresh tokens

// NewService creates a new auth service
func NewService(secretKey string, repo Repository) Service {
	tokenAuth := jwtauth.New("HS256", []byte(secretKey), nil)
	return &authService{
		tokenAuth: tokenAuth,
		repo:      repo,
		secretKey: []byte(secretKey),
	}
}

// GetAuth returns the JWTAuth instance for middleware
func (s *authService) GetAuth() *jwtauth.JWTAuth {
	return s.tokenAuth
}

// GenerateToken creates a new JWT token for a user
func (s *authService) GenerateToken(user *models.User) (string, error) {
	claims := map[string]interface{}{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"exp":      time.Now().Add(TokenExpiry).Unix(),
	}

	_, tokenString, err := s.tokenAuth.Encode(claims)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("Failed to generate JWT token")
		return "", err
	}

	return tokenString, nil
}

// LoginRequest represents the data needed for login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response from a successful login
type LoginResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

func (s *authService) GenerateAPIToken(ctx context.Context, userID uuid.UUID, name string) (*models.APIToken, error) {
	var token string
	var exists bool
	var err error

	for attempts := 0; attempts < 3; attempts++ {
		tokenBytes := make([]byte, 32)
		if _, err = rand.Read(tokenBytes); err != nil {
			log.Error().
				Err(err).
				Str("user_id", userID.String()).
				Msg("Failed to generate random bytes for API token")
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		h := hmac.New(sha256.New, s.secretKey)
		h.Write(tokenBytes)
		hmacBytes := h.Sum(nil)

		finalBytes := append(tokenBytes, hmacBytes...)
		token = base64.URLEncoding.EncodeToString(finalBytes)

		exists, err = s.repo.TokenExists(ctx, token)
		if err != nil {
			log.Error().
				Err(err).
				Str("user_id", userID.String()).
				Int("attempt", attempts+1).
				Msg("Failed to check token existence")
			return nil, fmt.Errorf("failed to check token existence: %w", err)
		}

		if !exists {
			break
		}

		log.Warn().
			Str("user_id", userID.String()).
			Int("attempt", attempts+1).
			Msg("Token collision occurred, retrying")
	}

	if exists {
		log.Error().
			Str("user_id", userID.String()).
			Msg("Failed to generate unique token after 3 attempts")
		return nil, errors.New("failed to generate unique token after 3 attempts")
	}

	apiToken := &models.APIToken{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Token:     token,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	log.Info().
		Str("token_id", apiToken.ID.String()).
		Str("user_id", apiToken.UserID.String()).
		Str("name", apiToken.Name).
		Time("created_at", apiToken.CreatedAt).
		Bool("is_active", apiToken.IsActive).
		Msg("Created new API token")

	err = s.repo.CreateToken(ctx, apiToken)
	if err != nil {
		log.Error().
			Err(err).
			Str("token_id", apiToken.ID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to save API token to database")
		return nil, fmt.Errorf("failed to create token: %s", err.Error())
	}

	return apiToken, nil
}

func (s *authService) ValidateAPIToken(ctx context.Context, token string) (*models.APIToken, error) {
	apiToken, err := s.repo.GetAPITokenByToken(ctx, token)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to retrieve API token")
		return nil, err
	}

	if !apiToken.IsActive || apiToken.RevokedAt != nil {
		log.Warn().
			Str("token_id", apiToken.ID.String()).
			Str("user_id", apiToken.UserID.String()).
			Bool("is_active", apiToken.IsActive).
			Time("revoked_at", *apiToken.RevokedAt).
			Msg("Attempt to use inactive or revoked token")
		return nil, errors.New("token is inactive or revoked")
	}

	if apiToken.ExpiresAt != nil && time.Now().After(*apiToken.ExpiresAt) {
		log.Warn().
			Str("token_id", apiToken.ID.String()).
			Str("user_id", apiToken.UserID.String()).
			Time("expired_at", *apiToken.ExpiresAt).
			Msg("Attempt to use expired token")
		return nil, errors.New("token has expired")
	}

	err = s.repo.UpdateLastUsed(ctx, apiToken.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("token_id", apiToken.ID.String()).
			Msg("Failed to update last used timestamp")
	}

	return apiToken, nil
}

func (s *authService) GetUserAPITokens(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	tokens, err := s.repo.ListUserTokens(ctx, userID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to retrieve user API tokens")
		return nil, err
	}
	return tokens, nil
}

func (s *authService) DeleteTokenByUserIdAndToken(ctx context.Context, userID uuid.UUID, token string) error {
	err := s.repo.DeleteTokenByUserIdAndToken(ctx, userID, token)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to delete API token")
		return err
	}

	log.Info().
		Str("user_id", userID.String()).
		Msg("Successfully deleted API token")

	return nil
}
