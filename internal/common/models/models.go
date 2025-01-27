package models

import (
	"time"

	"github.com/google/uuid"
)

// Uploader

// UploadedFile represents a file that has been uploaded to the system.
type UploadedFile struct {
	ID uuid.UUID `db:"id" json:"id"` // Unique identifier for the uploaded file

	OriginalName   string `db:"original_name" json:"original_name"`     // Original name of the uploaded file
	UniqueFilename string `db:"unique_filename" json:"unique_filename"` // Unique filename generated to avoid conflicts, includes extension if any
	MimeType       string `db:"mime_type" json:"mime_type"`             // MIME type of the uploaded file
	FileSize       uint64 `db:"file_size" json:"file_size"`             // Size of the uploaded file in bytes

	UserID         uuid.UUID  `db:"user_id" json:"user_id"`                             // ID of the user who uploaded the file, can be NIL
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`                       // Timestamp when the file was uploaded
	LastAccessedAt *time.Time `db:"last_accessed_at" json:"last_accessed_at,omitempty"` // Timestamp when the file was last accessed
	AccessCount    int        `db:"access_count" json:"access_count"`                   // Number of times the file has been accessed
	ExpiresAt      time.Time  `db:"expires_at" json:"expires_at"`                       // Timestamp when the file will expire
	URLValue       string     `db:"url_value" json:"url_value"`                         // URL value associated with the uploaded file
}

type CreateFileResponse struct {
	FileUrl      string `json:"file_url"`
	OriginalName string `json:"original_name"`
	UnixFilename string `json:"unix_filename"`
}

// APIToken represents an API token used for authenticating API requests.
type APIToken struct {
	ID         uuid.UUID  `db:"id" json:"id"`                               // Unique identifier for the API token
	UserID     uuid.UUID  `db:"user_id" json:"user_id"`                     // ID of the user associated with the API token
	Name       string     `db:"name" json:"name"`                           // Name of the API token
	Token      string     `db:"token" json:"-"`                             // The actual token value (not serialized to JSON)
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`               // Timestamp when the API token was created
	LastUsedAt *time.Time `db:"last_used_at" json:"last_used_at,omitempty"` // Timestamp when the API token was last used
	ExpiresAt  *time.Time `db:"expires_at" json:"expires_at,omitempty"`     // Timestamp when the API token will expire
	RevokedAt  *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`     // Timestamp when the API token was revoked
	IsActive   bool       `db:"is_active" json:"is_active"`                 // Indicates whether the API token is active
}

// User represents a user in the system
type User struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

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

// RequestInfo contains information about the incoming request for analytics
type RequestInfo struct {
	Referrer    string
	UserAgent   string
	IPAddress   string
	CountryCode string
	City        string
	Region      string
}

// CreateURLRequest represents the request to create a shortened URL
type CreateURLRequest struct {
	URL        string     `json:"url" validate:"required,url"`
	VanityCode string     `json:"vanity_code,omitempty" validate:"omitempty,vanitycode"`
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

// FileStats represents statistics about uploaded files
type FileStats struct {
	TotalFiles   int      `db:"total_files"`   // Total number of files uploaded
	TotalSize    int64    `db:"total_size"`    // Total size of all files in bytes
	TotalViews   int64    `db:"total_views"`   // Total number of views
	StorageQuota int64    `db:"storage_quota"` // User's storage quota in bytes
	PopularTypes []string `db:"popular_types"` // Most common file types
}

// DashboardStats represents the statistics shown on the dashboard
type DashboardStats struct {
	TotalURLs    int64        `json:"total_urls" db:"total_urls"`
	TotalClicks  int64        `json:"total_clicks" db:"total_clicks"`
	TotalFiles   int64        `json:"total_files" db:"total_files"`
	TotalStorage int64        `json:"total_storage" db:"total_storage"`
	RecentURLs   []RecentURL  `json:"recent_urls"`
	RecentFiles  []RecentFile `json:"recent_files"`
}

// RecentURL represents a recently created shortened URL
type RecentURL struct {
	ShortCode   string `json:"short_code" db:"short_code"`
	OriginalURL string `json:"original_url" db:"original_url"`
	AccessCount int    `json:"access_count" db:"access_count"`
	CreatedAt   string `json:"created_at" db:"created_at"`
}

// RecentFile represents a recently uploaded file
type RecentFile struct {
	FileName    string `json:"file_name" db:"original_name"`
	FileSize    int64  `json:"file_size" db:"file_size"`
	AccessCount int    `json:"access_count" db:"access_count"`
	CreatedAt   string `json:"created_at" db:"created_at"`
}
