package dashboard

import (
	"context"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"
)

type Repository interface {
	GetDashboardStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error)
	GetRecentURLs(ctx context.Context, userID uuid.UUID, limit int) ([]models.RecentURL, error)
	GetRecentFiles(ctx context.Context, userID uuid.UUID, limit int) ([]models.RecentFile, error)
}

type repository struct {
	*database.Repository
}

func NewRepository(db *database.DB) Repository {
	return &repository{
		Repository: database.NewRepository(db),
	}
}

func (r *repository) GetDashboardStats(ctx context.Context, userID uuid.UUID) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Get URL statistics
		urlQuery := `
            SELECT 
                COUNT(*) as total_urls,
                COALESCE(SUM(access_count), 0) as total_clicks
            FROM shortened_urls 
            WHERE user_id = $1 AND is_active = true`

		if err := tx.GetContext(ctx, stats, urlQuery, userID); err != nil {
			return err
		}

		// Get file statistics
		fileQuery := `
            SELECT 
                COUNT(*) as total_files,
                COALESCE(SUM(file_size), 0) as total_storage
            FROM uploaded_files 
            WHERE user_id = $1`

		return tx.GetContext(ctx, stats, fileQuery, userID)
	})

	return stats, err
}

func (r *repository) GetRecentURLs(ctx context.Context, userID uuid.UUID, limit int) ([]models.RecentURL, error) {
	query := `
        SELECT 
            short_code,
            original_url,
            access_count,
            to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') as created_at
        FROM shortened_urls
        WHERE user_id = $1 AND is_active = true
        ORDER BY created_at DESC
        LIMIT $2`

	var urls []models.RecentURL
	err := r.Select(ctx, &urls, query, userID, limit)
	return urls, err
}

func (r *repository) GetRecentFiles(ctx context.Context, userID uuid.UUID, limit int) ([]models.RecentFile, error) {
	query := `
        SELECT 
            original_name,
            file_size,
            access_count,
            to_char(created_at, 'YYYY-MM-DD HH24:MI:SS') as created_at
        FROM uploaded_files
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2`

	var files []models.RecentFile
	err := r.Select(ctx, &files, query, userID, limit)
	return files, err
}
