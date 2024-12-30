package shortener

import (
	"github.com/google/uuid"
	"time"
)

// ShortenedURL represents a shortened URL in the system
type ShortenedURL struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	OriginalURL    string     `db:"original_url" json:"original_url"`
	ShortCode      string     `db:"short_code" json:"short_code"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	LastAccessedAt *time.Time `db:"last_accessed_at" json:"last_accessed_at,omitempty"`
	AccessCount    int        `db:"access_count" json:"access_count"`
}

// Repository defines methods for URL persistence
type Repository interface {
	Create(url *ShortenedURL) error
	GetByShortCode(code string) (*ShortenedURL, error)
	IncrementAccessCount(id uuid.UUID) error
}

// CreateURLRequest represents the request to create a shortened URL
type CreateURLRequest struct {
	URL string `json:"url" validate:"required,url"`
}

// CreateURLResponse represents the response after creating a shortened URL
type CreateURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	ShortCode   string `json:"short_code"`
}
