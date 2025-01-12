package shortener

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

// Create stores a new shortened URL
func (r *postgresRepository) Create(url *ShortenedURL) error {
	query := `
        INSERT INTO shortened_urls (
            id, user_id, original_url, short_code, created_at, 
            expires_at, is_vanity, is_active
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`

	return r.db.QueryRow(
		query,
		url.ID,
		url.UserID,
		url.OriginalURL,
		url.ShortCode,
		url.CreatedAt,
		url.ExpiresAt,
		url.IsVanity,
		url.IsActive,
	).Scan(&url.ID)
}

// GetByShortCode retrieves a URL by its short code
func (r *postgresRepository) GetByShortCode(code string) (*ShortenedURL, error) {
	url := new(ShortenedURL)
	err := r.db.Get(url, `
        SELECT * FROM shortened_urls 
        WHERE short_code = $1 
        AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
        AND is_active = true`,
		code,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("URL not found or expired")
	}
	return url, err
}

// GetByUserID retrieves all URLs created by a specific user
func (r *postgresRepository) GetByUserID(userID uuid.UUID) ([]*ShortenedURL, error) {
	var urls []*ShortenedURL
	err := r.db.Select(&urls, `
        SELECT * FROM shortened_urls 
        WHERE user_id = $1 
        AND is_active = true 
        ORDER BY created_at DESC`,
		userID,
	)
	return urls, err
}

// IncrementAccessCount increases the access counter for a URL
func (r *postgresRepository) IncrementAccessCount(id uuid.UUID) error {
	_, err := r.db.Exec(`
        UPDATE shortened_urls 
        SET access_count = access_count + 1,
            last_accessed_at = CURRENT_TIMESTAMP
        WHERE id = $1`,
		id,
	)
	return err
}

// Delete performs a soft delete of a URL
func (r *postgresRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec(`
        UPDATE shortened_urls 
        SET is_active = false 
        WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("URL not found")
	}
	return nil
}

// Update updates a URL's properties
func (r *postgresRepository) Update(url *ShortenedURL) error {
	_, err := r.db.Exec(`
        UPDATE shortened_urls 
        SET expires_at = $1,
            is_active = $2,
            last_accessed_at = CURRENT_TIMESTAMP
        WHERE id = $3`,
		url.ExpiresAt,
		url.IsActive,
		url.ID,
	)
	return err
}

// RecordClick stores analytics data for a click event
func (r *postgresRepository) RecordClick(analytics *ClickAnalytics) error {
	_, err := r.db.Exec(`
        INSERT INTO click_analytics (
            id, url_id, clicked_at, referrer, 
            user_agent, ip_address, country_code,
            city, region
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		analytics.ID,
		analytics.URLID,
		analytics.ClickedAt,
		analytics.Referrer,
		analytics.UserAgent,
		analytics.IPAddress,
		analytics.CountryCode,
		analytics.City,
		analytics.Region,
	)
	return err
}

// GetURLAnalytics retrieves analytics data for a specific URL
func (r *postgresRepository) GetURLAnalytics(urlID uuid.UUID) (*URLAnalytics, error) {
	analytics := &URLAnalytics{}

	// Get the URL details
	url := new(ShortenedURL)
	err := r.db.Get(url, "SELECT * FROM shortened_urls WHERE id = $1", urlID)
	if err != nil {
		return nil, err
	}
	analytics.URL = url

	// Get total clicks
	err = r.db.Get(&analytics.TotalClicks, `
        SELECT COUNT(*) FROM click_analytics WHERE url_id = $1`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get unique clicks (by IP)
	err = r.db.Get(&analytics.UniqueClicks, `
        SELECT COUNT(DISTINCT ip_address) 
        FROM click_analytics 
        WHERE url_id = $1`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get top referrers
	err = r.db.Select(&analytics.TopReferrers, `
        SELECT referrer, COUNT(*) as count
        FROM click_analytics
        WHERE url_id = $1 AND referrer IS NOT NULL AND referrer != ''
        GROUP BY referrer
        ORDER BY count DESC
        LIMIT 10`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get top countries
	err = r.db.Select(&analytics.TopCountries, `
    SELECT 
        country_code,
        COUNT(*) as count
    FROM click_analytics
    WHERE url_id = $1 AND country_code IS NOT NULL
    GROUP BY country_code
    ORDER BY COUNT(*) DESC
    LIMIT 10`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get clicks by day
	err = r.db.Select(&analytics.ClicksByDay, `
        SELECT 
            DATE_TRUNC('day', clicked_at) as date,
            COUNT(*) as count
        FROM click_analytics
        WHERE url_id = $1
        GROUP BY DATE_TRUNC('day', clicked_at)
        ORDER BY date DESC
        LIMIT 30`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	return analytics, nil
}

// GetURLsByExpiration retrieves all URLs that expire before a given time
func (r *postgresRepository) GetURLsByExpiration(before time.Time) ([]*ShortenedURL, error) {
	var urls []*ShortenedURL
	err := r.db.Select(&urls, `
        SELECT * FROM shortened_urls 
        WHERE expires_at IS NOT NULL 
        AND expires_at < $1 
        AND is_active = true`,
		before,
	)
	return urls, err
}
