package uploader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"
)

type Repository interface {
	CreateWithURL(ctx context.Context, file *models.UploadedFile, urlValue string) error
	GetAllFiles(ctx context.Context) ([]*models.UploadedFile, error)
	GetByUniqueFilename(ctx context.Context, code string) (*models.UploadedFile, error)
	GetByURLValue(ctx context.Context, urlValue string) (*models.UploadedFile, error)
	IncrementAccessCount(ctx context.Context, id uuid.UUID) error
	GetExpiredFiles(ctx context.Context) ([]*models.UploadedFile, error)
	Delete(ctx context.Context, id uuid.UUID) error
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
