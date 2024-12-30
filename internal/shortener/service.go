package shortener

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const (
	// Alphabet for short codes
	alphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength = 8 // Length of short code
)

type Service struct {
	repo    Repository
	baseURL string
}

func NewService(repo Repository, baseURL string) *Service {
	return &Service{
		repo:    repo,
		baseURL: baseURL,
	}
}

func (s *Service) CreateShortURL(longURL string) (*CreateURLResponse, error) {
	// Validating URL
	if _, err := url.ParseRequestURI(longURL); err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// Generate short code
	shortCode, err := s.generateUniqueCode()
	if err != nil {
		return nil, fmt.Errorf("generating short code: %w", err)
	}

	// Create ShortenedURL object
	shortenedURL := &ShortenedURL{
		ID:          uuid.New(),
		OriginalURL: longURL,
		ShortCode:   shortCode,
		CreatedAt:   time.Now(),
	}

	// Save URL in database
	if err := s.repo.Create(shortenedURL); err != nil {
		return nil, fmt.Errorf("creating shortened URL: %w", err)
	}

	// Create response
	return &CreateURLResponse{
		ShortURL:    s.baseURL + "/" + shortCode,
		OriginalURL: longURL,
		ShortCode:   shortCode,
	}, nil
}

func (s *Service) GetOriginalURL(shortCode string) (string, error) {
	// Retrieve URL from database
	shortenedURL, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return "", fmt.Errorf("retrieving URL: %w", err)
	}

	// Increment access count
	if err := s.repo.IncrementAccessCount(shortenedURL.ID); err != nil {
		// Log error but don't return error
		fmt.Printf("Error incrementing access count: %v\n", err)
	}

	return shortenedURL.OriginalURL, nil
}

func (s *Service) generateUniqueCode() (string, error) {
	for attempts := 0; attempts < 5; attempts++ {
		code, err := s.generateCode()
		if err != nil {
			continue
		}

		// Check if code already exists
		_, err = s.repo.GetByShortCode(code)
		if err != nil {
			// If Error "not found", then code is unique
			return code, nil
		}
	}

	return "", fmt.Errorf("could not generate unique code after 5 attempts")
}

func (s *Service) generateCode() (string, error) {
	length := len(alphabet)
	code := make([]byte, codeLength)

	for i := 0; i < codeLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(length)))
		if err != nil {
			return "", err
		}
		code[i] = alphabet[n.Int64()]
	}

	return string(code), nil
}
