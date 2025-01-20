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
	"log"
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
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		h := hmac.New(sha256.New, s.secretKey)
		h.Write(tokenBytes)
		hmacBytes := h.Sum(nil)

		finalBytes := append(tokenBytes, hmacBytes...)
		token = base64.URLEncoding.EncodeToString(finalBytes)

		exists, err = s.repo.TokenExists(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("failed to check token existence: %w", err)
		}

		if !exists {
			break
		}
	}

	if exists {
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
	log.Printf("Created API token: ID=%s, UserID=%s, Name=%s, CreatedAt=%s, IsActive=%v",
		apiToken.ID,
		apiToken.UserID,
		apiToken.Name,
		apiToken.CreatedAt.Format(time.RFC3339),
		apiToken.IsActive,
	)
	err = s.repo.CreateToken(ctx, apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %s", err.Error())
	}

	return apiToken, nil
}

func (s *authService) ValidateAPIToken(ctx context.Context, token string) (*models.APIToken, error) {
	apiToken, err := s.repo.GetAPITokenByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if !apiToken.IsActive || apiToken.RevokedAt != nil {
		return nil, errors.New("token is inactive or revoked")
	}

	if apiToken.ExpiresAt != nil && time.Now().After(*apiToken.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	return apiToken, nil
}

func (s *authService) GetUserAPITokens(ctx context.Context, userID uuid.UUID) ([]*models.APIToken, error) {
	tokens, err := s.repo.ListUserTokens(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *authService) DeleteTokenByUserIdAndToken(ctx context.Context, userID uuid.UUID, token string) error {
	return s.repo.DeleteTokenByUserIdAndToken(ctx, userID, token)
}
