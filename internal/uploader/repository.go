package uploader

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"volaticus-go/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDuplicateURLValue = fmt.Errorf("duplicate URL value")
	ErrNoRows            = fmt.Errorf("no rows found")
	ErrTransaction       = fmt.Errorf("transaction error")
	ErrCommit            = fmt.Errorf("commit transaction error")
	ErrRollback          = fmt.Errorf("rollback transaction error")
)

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

type Queries struct {
	*sqlx.Tx
}

func New(tx *sqlx.Tx) *Queries {
	return &Queries{Tx: tx}
}

func (r *postgresRepository) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}

	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w: %v, %v", ErrRollback, err, rbErr)
		}
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrCommit, err)
	}

	return nil
}

func (r *postgresRepository) CreateWithURL(file *models.UploadedFile, urlValue string) error {
	ctx := context.Background() // FIXME: Use context from caller
	return r.execTx(ctx, func(q *Queries) error {
		// Check for duplicate URL value
		var existingFile models.UploadedFile
		err := q.Get(&existingFile, `SELECT * FROM uploaded_files WHERE url_value = $1`, urlValue)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %v", ErrTransaction, err)
		}
		if existingFile.ID != uuid.Nil {
			return fmt.Errorf("%w: %s", ErrDuplicateURLValue, urlValue)
		}

		// Insert uploaded file
		_, err = q.NamedExec(`INSERT INTO uploaded_files (id, original_name, unique_filename, mime_type, file_size, user_id, created_at, last_accessed_at, access_count, expires_at, url_value)
			VALUES (:id, :original_name, :unique_filename, :mime_type, :file_size, :user_id, :created_at, :last_accessed_at, :access_count, :expires_at, :url_value)`, file)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTransaction, err)
		}

		return nil
	})
}

func (r *postgresRepository) GetByUniqueFilename(code string) (*models.UploadedFile, error) {
	var file models.UploadedFile
	err := r.db.Get(&file, `SELECT * FROM uploaded_files WHERE unique_filename = $1`, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return &file, nil
}

func (r *postgresRepository) GetByURLValue(urlValue string) (*models.UploadedFile, error) {
	var file models.UploadedFile
	err := r.db.Get(&file, `SELECT * FROM uploaded_files WHERE url_value = $1`, urlValue)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return &file, nil
}

func (r *postgresRepository) IncrementAccessCount(id uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE uploaded_files SET access_count = access_count + 1, last_accessed_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return nil
}

func (r *postgresRepository) GetExpiredFiles() ([]*models.UploadedFile, error) {
	var files []*models.UploadedFile
	err := r.db.Select(&files, `SELECT * FROM uploaded_files WHERE expires_at < NOW()`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return files, nil
}

func (r *postgresRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM uploaded_files WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return nil
}

func (r *postgresRepository) GetAllFiles() ([]*models.UploadedFile, error) {
	var files []*models.UploadedFile
	err := r.db.Select(&files, `SELECT * FROM uploaded_files`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransaction, err)
	}
	return files, nil
}
