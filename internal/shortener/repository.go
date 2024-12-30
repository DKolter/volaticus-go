package shortener

import (
	"database/sql"
	"errors"
	"fmt"
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

func (r *postgresRepository) Create(url *ShortenedURL) error {
	query := `
        INSERT INTO shortened_urls (id, original_url, short_code, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id`

	return r.db.QueryRow(
		query,
		url.ID,
		url.OriginalURL,
		url.ShortCode,
		url.CreatedAt,
	).Scan(&url.ID)
}

func (r *postgresRepository) GetByShortCode(code string) (*ShortenedURL, error) {
	url := new(ShortenedURL)
	err := r.db.Get(url, "SELECT * FROM shortened_urls WHERE short_code = $1", code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("URL not found")
	}
	if err != nil {
		return nil, fmt.Errorf("getting URL by short code: %w", err)
	}
	return url, nil
}

func (r *postgresRepository) IncrementAccessCount(id uuid.UUID) error {
	query := `
        UPDATE shortened_urls 
        SET access_count = access_count + 1,
            last_accessed_at = $1
        WHERE id = $2`

	result, err := r.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("incrementing access count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("URL not found")
	}

	return nil
}
