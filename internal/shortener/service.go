package shortener

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"regexp"
	"time"
	"volaticus-go/internal/common/models"

	"github.com/google/uuid"
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

func NewService(repo Repository, baseURL string) *Service {
	return &Service{
		repo:    repo,
		baseURL: baseURL,
		geoIP:   GetGeoIPService(),
	}
}

// CreateShortURL creates a new shortened URL with optional vanity code and expiration
func (s *Service) CreateShortURL(userID uuid.UUID, req *models.CreateURLRequest) (*models.CreateURLResponse, error) {
	// Validate URL
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	var shortCode string
	var err error
	isVanity := false

	// Handle vanity code if provided
	if req.VanityCode != "" {
		if err := s.validateVanityCode(req.VanityCode); err != nil {
			return nil, err
		}
		shortCode = req.VanityCode
		isVanity = true
	} else {
		// Generate random code
		shortCode, err = s.generateUniqueCode()
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
	if err := s.repo.Create(shortenedURL); err != nil {
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
func (s *Service) GetOriginalURL(shortCode string, r *models.RequestInfo) (string, error) {
	// Retrieve URL from database
	shortenedURL, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return "", fmt.Errorf("retrieving URL: %w", err)
	}

	// Check if URL is expired
	if shortenedURL.ExpiresAt != nil && time.Now().After(*shortenedURL.ExpiresAt) {
		return "", fmt.Errorf("URL has expired")
	}

	// Get location info from IP
	location := s.geoIP.GetLocation(r.IPAddress)

	// Record analytics asynchronously
	go func() {
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

		if err := s.repo.RecordClick(analytics); err != nil {
			// Log error but don't fail the request
			log.Printf("Error recording click: %v\n", err)
		}

		if err := s.repo.IncrementAccessCount(shortenedURL.ID); err != nil {
			log.Printf("Error incrementing access count: %v\n", err)
		}
	}()

	return shortenedURL.OriginalURL, nil
}

// GetUserURLs retrieves all URLs created by a specific user
func (s *Service) GetUserURLs(userID uuid.UUID) ([]*models.ShortenedURL, error) {
	return s.repo.GetByUserID(userID)
}

// GetURLAnalytics retrieves analytics for a specific URL
func (s *Service) GetURLAnalytics(urlID uuid.UUID, userID uuid.UUID) (*models.URLAnalytics, error) {
	// First verify the user owns this URL
	urls, err := s.repo.GetByUserID(userID)
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

	return s.repo.GetURLAnalytics(urlID)
}

// DeleteURL soft deletes a URL
func (s *Service) DeleteURL(urlID uuid.UUID, userID uuid.UUID) error {
	// Verify ownership
	urls, err := s.repo.GetByUserID(userID)
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

	return s.repo.Delete(urlID)
}

// DeleteURLByShortCode deletes a URL by its short code
func (s *Service) DeleteURLByShortCode(shortCode string, userID uuid.UUID) error {
	// Retrieve the URL by short code
	shortenedURL, err := s.repo.GetByShortCode(shortCode)
	if err != nil {
		return fmt.Errorf("retrieving URL: %w", err)
	}

	// Verify ownership
	if shortenedURL.UserID != userID {
		return fmt.Errorf("unauthorized access to URL")
	}

	// Delete the URL
	if err := s.repo.Delete(shortenedURL.ID); err != nil {
		return fmt.Errorf("deleting URL: %w", err)
	}

	return nil
}

// UpdateURLExpiration updates the expiration date of a URL
func (s *Service) UpdateURLExpiration(urlID uuid.UUID, userID uuid.UUID, expiresAt *time.Time) error {
	// Verify ownership
	urls, err := s.repo.GetByUserID(userID)
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
	return s.repo.Update(targetURL)
}

// CleanupExpiredURLs deactivates expired URLs
func (s *Service) CleanupExpiredURLs() error {
	urls, err := s.repo.GetURLsByExpiration(time.Now())
	if err != nil {
		return err
	}

	for _, url := range urls {
		url.IsActive = false
		if err := s.repo.Update(url); err != nil {
			log.Printf("Error deactivating URL %s: %v\n", url.ShortCode, err)
		}
	}

	return nil
}

// Helper functions

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

func (s *Service) validateVanityCode(code string) error {
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
	_, err = s.repo.GetByShortCode(code)
	if err == nil {
		return fmt.Errorf("vanity code already in use")
	}

	return nil
}
