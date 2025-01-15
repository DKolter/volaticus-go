package uploader

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateWithURLs(file *UploadedFile, urls []FileURL) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	// Insert file
	if err := insertUploadedFile(tx, file); err != nil {
		tx.Rollback()
		return fmt.Errorf("inserting file: %w", err)
	}

	// Insert URLs
	for _, url := range urls {
		if err := insertFileUrl(tx, &url); err != nil {
			tx.Rollback()
			return fmt.Errorf("inserting url: %w", err)
		}
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByUnixFilename(filename string) (*UploadedFile, error) {
	var file UploadedFile
	query := `SELECT * FROM uploaded_files WHERE unix_filename = $1`
	err := r.db.Get(&file, query, filename)
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}
	return &file, nil
}

func (r *postgresRepository) IncrementAccessCount(id uuid.UUID) error {
	query := `
  UPDATE uploaded_files 
  SET access_count = access_count + 1, 
  last_accessed_at = CURRENT_TIMESTAMP
  WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("incrementing access count: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("file not found")
	}
	return nil
}

func insertUploadedFile(tx *sqlx.Tx, file *UploadedFile) error {
	log.Printf("Inserting file: %v", file)
	query := `
  INSERT INTO uploaded_files (id, original_name, unix_filename, mime_type, file_size, user_id, created_at, access_count, expires_at)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
  `

	_, err := tx.Exec(query,
		file.ID, file.OriginalName, file.UnixFilename, file.MimeType, file.FileSize, file.UserID, file.CreatedAt, file.AccessCount, file.ExpiredAt)
	return err
}

func insertFileUrl(tx *sqlx.Tx, url *FileURL) error {
	query := `
  INSERT INTO file_urls (id, file_id, url_type, url_value, created_at)
  VALUES($1, $2, $3, $4, $5)
  `
	_, err := tx.Exec(query,
		url.ID, url.FileID, url.UrlType, url.UrlValue, url.CreatedAt)
	return err
}

func (r *postgresRepository) GetByURLValue(urlValue string) (*UploadedFile, error) {
	var file UploadedFile
	query := `
  SELECT uf.*
  FROM uploaded_files uf
  JOIN file_urls fu ON uf.id = fu.file_id
  WHERE fu.url_value = $1
  `
	err := r.db.Get(&file, query, urlValue)
	if err != nil {
		return nil, fmt.Errorf("getting file by url value: %w", err)
	}
	return &file, nil
}
