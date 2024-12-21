package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"
	"volaticus-go/internal/user"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
)

type Service struct {
	tokenAuth *jwtauth.JWTAuth
	repo      TokenRepository
	secretKey []byte // used for APIToken generation
}

const TokenExpiry = time.Hour * 24 // 24 hours TODO: get from config

// NewService creates a new auth service
func NewService(secretKey string, repo TokenRepository) *Service {
	tokenAuth := jwtauth.New("HS256", []byte(secretKey), nil)
	return &Service{
		tokenAuth: tokenAuth,
		repo:      repo,
	}
}

// GetAuth returns the JWTAuth instance for middleware
func (s *Service) GetAuth() *jwtauth.JWTAuth {
	return s.tokenAuth
}

// GenerateToken creates a new JWT token for a user
func (s *Service) GenerateToken(user *user.User) (string, error) {
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

func (s *Service) GenerateAPIToken(userID uuid.UUID, name string) (*APIToken, error) {
	var token string
	var exists bool

	// Try generating unique token up to 3 times
	for attempts := 0; attempts < 3; attempts++ {
		// Generate random bytes
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		// Create HMAC of random bytes using secret
		h := hmac.New(sha256.New, s.secretKey)
		h.Write(tokenBytes)
		hmacBytes := h.Sum(nil)

		// Combine random bytes and HMAC
		finalBytes := append(tokenBytes, hmacBytes...)
		token = base64.URLEncoding.EncodeToString(finalBytes)

		// Check if token already exists
		exists, err := s.repo.TokenExists(token)
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

	apiToken := &APIToken{
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
	err := s.repo.CreateToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %s", err.Error())
	}

	return apiToken, nil
}

func (s *Service) ValidateAPIToken(token string) (*APIToken, error) {
	apiToken, err := s.repo.GetAPITokenByToken(token)
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

func (s *Service) GetUserAPITokens(userid uuid.UUID) ([]*APIToken, error) {
	tokens, err := s.repo.ListUserTokens(userid)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}
