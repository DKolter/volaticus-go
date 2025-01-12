package shortener

import (
	"github.com/google/uuid"
	"time"
)

// ShortenedURL represents a shortened URL in the system
type ShortenedURL struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	UserID         uuid.UUID  `db:"user_id" json:"user_id"`
	OriginalURL    string     `db:"original_url" json:"original_url"`
	ShortCode      string     `db:"short_code" json:"short_code"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	ExpiresAt      *time.Time `db:"expires_at" json:"expires_at,omitempty"`
	LastAccessedAt *time.Time `db:"last_accessed_at" json:"last_accessed_at,omitempty"`
	AccessCount    int        `db:"access_count" json:"access_count"`
	IsVanity       bool       `db:"is_vanity" json:"is_vanity"`
	IsActive       bool       `db:"is_active" json:"is_active"`
}

// ClickAnalytics represents a single click event
type ClickAnalytics struct {
	ID          uuid.UUID `db:"id" json:"id"`
	URLID       uuid.UUID `db:"url_id" json:"url_id"`
	ClickedAt   time.Time `db:"clicked_at" json:"clicked_at"`
	Referrer    string    `db:"referrer" json:"referrer"`
	UserAgent   string    `db:"user_agent" json:"user_agent"`
	IPAddress   string    `db:"ip_address" json:"ip_address"`
	CountryCode string    `db:"country_code" json:"country_code"`
	City        string    `db:"city" json:"city"`
	Region      string    `db:"region" json:"region"`
}

// CreateURLRequest represents the request to create a shortened URL
type CreateURLRequest struct {
	URL        string     `json:"url" validate:"required,url"`
	VanityCode string     `json:"vanity_code,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// CreateURLResponse represents the response after creating a shortened URL
type CreateURLResponse struct {
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ShortCode   string     `json:"short_code"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsVanity    bool       `json:"is_vanity"`
}

// URLAnalytics represents analytics for a shortened URL
type URLAnalytics struct {
	URL          *ShortenedURL   `json:"url"`
	TotalClicks  int             `json:"total_clicks"`
	UniqueClicks int             `json:"unique_clicks"`
	TopReferrers []ReferrerStats `json:"top_referrers"`
	TopCountries []CountryStats  `json:"top_countries"`
	ClicksByDay  []ClicksByDay   `json:"clicks_by_day"`
}

// ReferrerStats represents statistics for referrers
type ReferrerStats struct {
	Referrer string `json:"referrer" db:"referrer"`
	Count    int    `json:"count" db:"count"`
}

// CountryStats represents statistics by country
type CountryStats struct {
	CountryCode string `json:"country_code" db:"country_code"`
	Count       int    `json:"count" db:"count"`
}

// ClicksByDay represents clicks grouped by day
type ClicksByDay struct {
	Date  time.Time `json:"date" db:"date"`
	Count int       `json:"count" db:"count"`
}

// Repository defines methods for URL persistence
type Repository interface {
	Create(url *ShortenedURL) error
	GetByShortCode(code string) (*ShortenedURL, error)
	GetByUserID(userID uuid.UUID) ([]*ShortenedURL, error)
	IncrementAccessCount(id uuid.UUID) error
	Delete(id uuid.UUID) error
	Update(url *ShortenedURL) error

	// Analytics methods
	RecordClick(analytics *ClickAnalytics) error
	GetURLAnalytics(urlID uuid.UUID) (*URLAnalytics, error)
	GetURLsByExpiration(before time.Time) ([]*ShortenedURL, error)
}

// RequestInfo contains information about the incoming request for analytics
type RequestInfo struct {
	Referrer    string
	UserAgent   string
	IPAddress   string
	CountryCode string
	City        string
	Region      string
}
