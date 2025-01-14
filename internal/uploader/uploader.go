package uploader

import (
	"time"

	"github.com/google/uuid"
)

type UploadedFile struct {
	ID uuid.UUID `db:"id" json:"id"`

	OriginalName string `db:"original_name" json:"original_name"`
	UnixFilename uint64 `db:"unix_filename" json:"unix_filename"`
	MimeType     string `db:"mime_type" json:"mime_type"`
	FileSize     uint64 `db:"file_size" json:"file_size"`

	UserID         uuid.UUID  `db:"user_id" json:"user_id"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	LastAccessedAt *time.Time `db:"last_accessed_at" json:"last_accessed_at,omitempty"`
	AccessCount    int        `db:"access_count" json:"access_count"`
	ExpiredAt      time.Time  `db:"expired_at" json:"expired_at"`
}

type FileURL struct {
	ID        uuid.UUID `db:"id" json:"id"`
	FileID    uuid.UUID `db:"file_id" json:"file_id"`
	UrlType   string    `db:"url_type" json:"url_type"`
	UrlValue  string    `db:"url_value" json:"url_value"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Repository interface {
	CreateWithURLs(file *UploadedFile, urls []FileURL) error
	GetByUnixFilename(code string) (*UploadedFile, error)
	IncrementAccessCount(id uuid.UUID) error
}

type CreateFileURLRequest struct {
  URL string `json:"url" validate:"required,url"`
}

type CreateFileURLResponse struct {

}

type CreateFileRequest struct {
	File string `json:"file" validate:"required,file"`
}

type CreateFileResponse struct {
	FileUrl     string `json:"short_url"`
	OriginalName string `json:"original_name"`
	UnixFilename string `json:"unix_filename"`
}
