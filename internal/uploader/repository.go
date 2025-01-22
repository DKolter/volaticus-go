package uploader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateWithURL(ctx context.Context, file *models.UploadedFile, urlValue string) error
	GetAllFiles(ctx context.Context) ([]*models.UploadedFile, error)
	GetByUniqueFilename(ctx context.Context, code string) (*models.UploadedFile, error)
	GetByURLValue(ctx context.Context, urlValue string) (*models.UploadedFile, error)
	IncrementAccessCount(ctx context.Context, id uuid.UUID) error
	GetExpiredFiles(ctx context.Context) ([]*models.UploadedFile, error)
	GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.UploadedFile, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.UploadedFile, error)
	GetUserFilesCount(ctx context.Context, userID uuid.UUID) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUniqueName(ctx context.Context, file string) error
	GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error)
}

type repository struct {
	*database.Repository
}

// NewRepository creates a new uploader repository
func NewRepository(db *database.DB) Repository {
	return &repository{
		Repository: database.NewRepository(db),
	}
}

func (r *repository) CreateWithURL(ctx context.Context, file *models.UploadedFile, urlValue string) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Check for duplicate URL value
		var existingFile models.UploadedFile
		err := tx.GetContext(ctx, &existingFile, `SELECT * FROM uploaded_files WHERE url_value = $1`, urlValue)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %v", ErrTransaction, err)
		}
		if existingFile.ID != uuid.Nil {
			return fmt.Errorf("%w: %s", ErrDuplicateURLValue, urlValue)
		}

		// Insert uploaded file
		_, err = tx.NamedExecContext(ctx, `INSERT INTO uploaded_files (id, original_name, unique_filename, mime_type, file_size, user_id, created_at, last_accessed_at, access_count, expires_at, url_value)
			VALUES (:id, :original_name, :unique_filename, :mime_type, :file_size, :user_id, :created_at, :last_accessed_at, :access_count, :expires_at, :url_value)`, file)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTransaction, err)
		}

		return nil
	})
}

func (r *repository) GetByUniqueFilename(ctx context.Context, code string) (*models.UploadedFile, error) {
	var file models.UploadedFile
	err := r.Get(ctx, &file, `SELECT * FROM uploaded_files WHERE unique_filename = $1`, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return &file, nil
}

func (r *repository) GetByURLValue(ctx context.Context, urlValue string) (*models.UploadedFile, error) {
	var file models.UploadedFile
	err := r.Get(ctx, &file, `SELECT * FROM uploaded_files WHERE url_value = $1`, urlValue)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return &file, nil
}

func (r *repository) IncrementAccessCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.Exec(ctx, `UPDATE uploaded_files SET access_count = access_count + 1, last_accessed_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return nil
}

func (r *repository) GetExpiredFiles(ctx context.Context) ([]*models.UploadedFile, error) {
	var files []*models.UploadedFile
	err := r.Select(ctx, &files, `SELECT * FROM uploaded_files WHERE expires_at < NOW()`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return files, nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.Exec(ctx, `DELETE FROM uploaded_files WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return nil
}

func (r *repository) GetAllFiles(ctx context.Context) ([]*models.UploadedFile, error) {
	var files []*models.UploadedFile
	err := r.Select(ctx, &files, `SELECT * FROM uploaded_files`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return files, nil
}

func (r *repository) GetUserFiles(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.UploadedFile, error) {
	var files []*models.UploadedFile
	query := `
        SELECT * FROM uploaded_files
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3`
	err := r.Select(ctx, &files, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("getting user files: %w", err)
	}
	return files, nil
}

func (r *repository) GetUserFilesCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM uploaded_files WHERE user_id = $1`
	err := r.Get(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("getting user files count: %w", err)
	}
	return count, nil
}

func (r *repository) DeleteFile(ctx context.Context, fileID, userID uuid.UUID) error {
	// First check if the file belongs to the user
	var exists bool
	err := r.Get(ctx, &exists, `
        SELECT EXISTS(
            SELECT 1 FROM uploaded_files
            WHERE id = $1 AND user_id = $2
        )`, fileID, userID)
	if err != nil {
		return fmt.Errorf("checking file ownership: %w", err)
	}
	if !exists {
		return ErrUnauthorized
	}

	// Delete the file
	result, err := r.Exec(ctx, `DELETE FROM uploaded_files WHERE id = $1`, fileID)
	if err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if rows == 0 {
		return ErrNoRows
	}

	return nil
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*models.UploadedFile, error) {
	var file models.UploadedFile
	err := r.Get(ctx, &file, `SELECT * FROM uploaded_files WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return &file, nil
}

func (r *repository) DeleteByUniqueName(ctx context.Context, file string) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM uploaded_files WHERE unique_filename = $1`, file)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTransaction, err)
		}
		return nil
	})
}

func (r *repository) GetFileStats(ctx context.Context, userID uuid.UUID) (*models.FileStats, error) {
	// Get total files and size
	var stats models.FileStats
	err := r.Get(ctx, &stats, `
        SELECT
            COUNT(*) as total_files,
            COALESCE(SUM(file_size), 0) as total_size
        FROM uploaded_files
        WHERE user_id = $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("getting file stats: %w", err)
	}

	// Get popular types (top 5)
	err = r.Select(ctx, &stats.PopularTypes, `
        SELECT mime_type
        FROM uploaded_files
        WHERE user_id = $1
        GROUP BY mime_type
        ORDER BY COUNT(*) DESC
        LIMIT 5`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("getting popular types: %w", err)
	}

	// Get total views
	err = r.Get(ctx, &stats.TotalViews, `
		SELECT COALESCE(SUM(access_count), 0) as total_views
		FROM uploaded_files
		WHERE user_id = $1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("getting total views: %w", err)
	}

	return &stats, nil
}
