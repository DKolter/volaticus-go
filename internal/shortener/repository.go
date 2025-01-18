package shortener

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jmoiron/sqlx"
	"time"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"

	"github.com/google/uuid"
)

// Repository defines methods for URL persistence
type Repository interface {
	Create(ctx context.Context, url *models.ShortenedURL) error
	GetByShortCode(ctx context.Context, code string) (*models.ShortenedURL, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.ShortenedURL, error)
	IncrementAccessCount(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, url *models.ShortenedURL) error

	// Analytics methods
	RecordClick(ctx context.Context, analytics *models.ClickAnalytics) error
	GetURLAnalytics(ctx context.Context, urlID uuid.UUID) (*models.URLAnalytics, error)
	GetURLsByExpiration(ctx context.Context, before time.Time) ([]*models.ShortenedURL, error)
}

type repository struct {
	*database.Repository
}

// NewRepository creates a new shortener repository
func NewRepository(db *database.DB) Repository {
	return &repository{
		Repository: database.NewRepository(db),
	}
}

// Create stores a new shortened URL
func (r *repository) Create(ctx context.Context, url *models.ShortenedURL) error {
	query := `
        INSERT INTO shortened_urls (
            id, user_id, original_url, short_code, created_at,
            expires_at, is_vanity, is_active
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id`

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.QueryRowContext(ctx, query,
			url.ID,
			url.UserID,
			url.OriginalURL,
			url.ShortCode,
			url.CreatedAt,
			url.ExpiresAt,
			url.IsVanity,
			url.IsActive,
		).Scan(&url.ID)
	})
}

// GetByShortCode retrieves a URL by its short code
func (r *repository) GetByShortCode(ctx context.Context, code string) (*models.ShortenedURL, error) {
	url := new(models.ShortenedURL)
	err := r.Get(ctx, url, `
        SELECT * FROM shortened_urls
        WHERE short_code = $1
        AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
        AND is_active = true`,
		code,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("URL not found or expired")
	}
	return url, err
}

// GetByUserID retrieves all URLs created by a specific user
func (r *repository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*models.ShortenedURL, error) {
	var urls []*models.ShortenedURL
	err := r.Select(ctx, &urls, `
        SELECT * FROM shortened_urls
        WHERE user_id = $1
        AND is_active = true
        ORDER BY created_at DESC`,
		userID,
	)
	return urls, err
}

// IncrementAccessCount increases the access counter for a URL
func (r *repository) IncrementAccessCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.Exec(ctx, `
        UPDATE shortened_urls
        SET access_count = access_count + 1,
            last_accessed_at = CURRENT_TIMESTAMP
        WHERE id = $1`,
		id,
	)
	return err
}

// Delete performs a soft delete of a URL
func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.Exec(ctx, `
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
func (r *repository) Update(ctx context.Context, url *models.ShortenedURL) error {
	_, err := r.Exec(ctx, `
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
func (r *repository) RecordClick(ctx context.Context, analytics *models.ClickAnalytics) error {
	query := `
        INSERT INTO click_analytics (
            id, url_id, clicked_at, referrer,
            user_agent, ip_address, country_code,
            city, region
        ) VALUES (:id, :url_id, :clicked_at, :referrer, :user_agent, :ip_address, :country_code, :city, :region)`

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.NamedExecContext(ctx, query, analytics)
		return err
	})
}

// GetURLAnalytics retrieves analytics data for a specific URL
func (r *repository) GetURLAnalytics(ctx context.Context, urlID uuid.UUID) (*models.URLAnalytics, error) {
	analytics := &models.URLAnalytics{}

	// Get the URL details
	url := new(models.ShortenedURL)
	err := r.Get(ctx, url, "SELECT * FROM shortened_urls WHERE id = $1", urlID)
	if err != nil {
		return nil, err
	}
	analytics.URL = url

	// Get total clicks
	err = r.Get(ctx, &analytics.TotalClicks, `
        SELECT COUNT(*) FROM click_analytics WHERE url_id = $1`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get unique clicks (by IP)
	err = r.Get(ctx, &analytics.UniqueClicks, `
        SELECT COUNT(DISTINCT ip_address)
        FROM click_analytics
        WHERE url_id = $1`,
		urlID,
	)
	if err != nil {
		return nil, err
	}

	// Get top referrers
	err = r.Select(ctx, &analytics.TopReferrers, `
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
	err = r.Select(ctx, &analytics.TopCountries, `
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
	err = r.Select(ctx, &analytics.ClicksByDay, `
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
func (r *repository) GetURLsByExpiration(ctx context.Context, before time.Time) ([]*models.ShortenedURL, error) {
	var urls []*models.ShortenedURL
	err := r.Select(ctx, &urls, `
        SELECT * FROM shortened_urls
        WHERE expires_at IS NOT NULL
        AND expires_at < $1
        AND is_active = true`,
		before,
	)
	return urls, err
}
