package shortener

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/config"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	alphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength = 8
)

type Service struct {
	repo    Repository
	baseURL string
	geoIP   *GeoIPService
}

func NewService(repo Repository, config *config.Config) *Service {
	return &Service{
		repo:    repo,
		baseURL: config.BaseURL,
		geoIP:   GetGeoIPService(),
	}
}

// CreateShortURL creates a new shortened URL with optional vanity code and expiration
func (s *Service) CreateShortURL(ctx context.Context, userID uuid.UUID, req *models.CreateURLRequest) (*models.CreateURLResponse, error) {
	// Validate URL
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	var shortCode string
	var err error
	isVanity := false

	// Handle vanity code if provided
	if req.VanityCode != "" {
		if err := s.validateVanityCode(ctx, req.VanityCode); err != nil {
			return nil, err
		}
		shortCode = req.VanityCode
		isVanity = true
	} else {
		// Generate random code
		shortCode, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Create ShortenedURL object
	shortenedURL := &models.ShortenedURL{
		ID:          uuid.New(),
		UserID:      userID,
		OriginalURL: req.URL,
		ShortCode:   shortCode,
		CreatedAt:   time.Now(),
		ExpiresAt:   req.ExpiresAt,
		IsVanity:    isVanity,
		IsActive:    true,
	}

	// Save URL in database
	if err := s.repo.Create(ctx, shortenedURL); err != nil {
		return nil, fmt.Errorf("creating shortened URL: %w", err)
	}

	return &models.CreateURLResponse{
		ShortURL:    s.baseURL + "/s/" + shortCode,
		OriginalURL: req.URL,
		ShortCode:   shortCode,
		ExpiresAt:   req.ExpiresAt,
		IsVanity:    isVanity,
	}, nil
}

// GetOriginalURL retrieves the original URL and records analytics
func (s *Service) GetOriginalURL(ctx context.Context, shortCode string, r *models.RequestInfo) (string, error) {
	// Retrieve URL from database
	shortenedURL, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return "", fmt.Errorf("retrieving URL: %w", err)
	}

	// Check if URL is expired
	if shortenedURL.ExpiresAt != nil && time.Now().After(*shortenedURL.ExpiresAt) {
		return "", fmt.Errorf("URL has expired")
	}

	// Get location info from IP
	location := s.geoIP.GetLocation(r.IPAddress)

	// Create a new context with a timeout for the asynchronous operations
	asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Record analytics asynchronously
	go func() {
		defer cancel()
		analytics := &models.ClickAnalytics{
			ID:          uuid.New(),
			URLID:       shortenedURL.ID,
			ClickedAt:   time.Now(),
			Referrer:    r.Referrer,
			UserAgent:   r.UserAgent,
			IPAddress:   r.IPAddress,
			CountryCode: location.CountryCode,
			City:        location.City,
			Region:      location.Region,
		}

		if err := s.repo.RecordClick(asyncCtx, analytics); err != nil {
			log.Error().
				Err(err).
				Str("url_id", shortenedURL.ID.String()).
				Str("short_code", shortCode).
				Str("ip", r.IPAddress).
				Msg("Failed to record click analytics")
		}

		if err := s.repo.IncrementAccessCount(asyncCtx, shortenedURL.ID); err != nil {
			log.Error().
				Err(err).
				Str("url_id", shortenedURL.ID.String()).
				Str("short_code", shortCode).
				Msg("Failed to increment access count")
		}
	}()

	return shortenedURL.OriginalURL, nil
}

// GetUserURLs retrieves all URLs created by a specific user
func (s *Service) GetUserURLs(ctx context.Context, userID uuid.UUID) ([]*models.ShortenedURL, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// GetURLAnalytics retrieves analytics for a specific URL
func (s *Service) GetURLAnalytics(ctx context.Context, urlID uuid.UUID, userID uuid.UUID) (*models.URLAnalytics, error) {
	// First verify the user owns this URL
	urls, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, url := range urls {
		if url.ID == urlID {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("unauthorized access to URL analytics")
	}

	return s.repo.GetURLAnalytics(ctx, urlID)
}

// DeleteURL soft deletes a URL
func (s *Service) DeleteURL(ctx context.Context, urlID uuid.UUID, userID uuid.UUID) error {
	// Verify ownership
	urls, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	found := false
	for _, url := range urls {
		if url.ID == urlID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("unauthorized access to URL")
	}

	return s.repo.Delete(ctx, urlID)
}

// DeleteURLByShortCode deletes a URL by its short code
func (s *Service) DeleteURLByShortCode(ctx context.Context, shortCode string, userID uuid.UUID) error {
	// Retrieve the URL by short code
	shortenedURL, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		return fmt.Errorf("retrieving URL: %w", err)
	}

	// Verify ownership
	if shortenedURL.UserID != userID {
		return fmt.Errorf("unauthorized access to URL")
	}

	// Delete the URL
	if err := s.repo.Delete(ctx, shortenedURL.ID); err != nil {
		return fmt.Errorf("deleting URL: %w", err)
	}

	return nil
}

// UpdateURLExpiration updates the expiration date of a URL
func (s *Service) UpdateURLExpiration(ctx context.Context, urlID uuid.UUID, userID uuid.UUID, expiresAt *time.Time) error {
	// Verify ownership
	urls, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	var targetURL *models.ShortenedURL
	for _, url := range urls {
		if url.ID == urlID {
			targetURL = url
			break
		}
	}

	if targetURL == nil {
		return fmt.Errorf("unauthorized access to URL")
	}

	targetURL.ExpiresAt = expiresAt
	return s.repo.Update(ctx, targetURL)
}

// CleanupExpiredURLs deactivates expired URLs
func (s *Service) CleanupExpiredURLs(ctx context.Context) error {
	urls, err := s.repo.GetURLsByExpiration(ctx, time.Now())
	if err != nil {
		return err
	}

	for _, url := range urls {
		url.IsActive = false
		if err := s.repo.Update(ctx, url); err != nil {
			log.Error().
				Err(err).
				Str("url_id", url.ID.String()).
				Str("short_code", url.ShortCode).
				Time("expires_at", *url.ExpiresAt).
				Msg("Failed to deactivate expired URL")
		}
	}

	return nil
}

// Helper functions

func (s *Service) generateUniqueCode(ctx context.Context) (string, error) {
	for attempts := 0; attempts < 5; attempts++ {
		code, err := s.generateCode(ctx)
		if err != nil {
			continue
		}

		// Check if code already exists
		_, err = s.repo.GetByShortCode(ctx, code)
		if err != nil {
			// If Error "not found", then code is unique
			return code, nil
		}
	}

	return "", fmt.Errorf("could not generate unique code after 5 attempts")
}

func (s *Service) generateCode(ctx context.Context) (string, error) {
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

func (s *Service) validateVanityCode(ctx context.Context, code string) error {
	if len(code) < 4 || len(code) > 30 {
		return fmt.Errorf("vanity code must be between 4 and 30 characters")
	}

	// Check if code contains only allowed characters
	matched, err := regexp.MatchString("^[a-zA-Z0-9-_]+$", code)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("vanity code can only contain letters, numbers, hyphens, and underscores")
	}

	// Check if code already exists
	_, err = s.repo.GetByShortCode(ctx, code)
	if err == nil {
		return fmt.Errorf("vanity code already in use")
	}

	return nil
}
