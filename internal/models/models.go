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
	ExpiredAt      time.Time  `db:"expires_at" json:"expires_at"`                       // Timestamp when the file will expire
	URLValue       string     `db:"url_value" json:"url_value"`                         // URL value associated with the uploaded file
}

type CreateFileResponse struct {
	FileUrl      string `json:"file_url"`
	OriginalName string `json:"original_name"`
	UnixFilename string `json:"unix_filename"`
}
