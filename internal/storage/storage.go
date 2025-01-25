package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type FileInfo struct {
	Name         string
	Size         int64
	ContentType  string
	ModifiedTime time.Time
}

// StorageProvider defines the interface for different storage implementations
type StorageProvider interface {
	// Upload saves a file to storage and returns its unique identifier
	Upload(ctx context.Context, file io.Reader, filename string) (string, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, filename string) error

	// GetURL returns a URL for accessing the file
	GetURL(ctx context.Context, filename string) (string, time.Duration, error)

	// Stream serves the file directly to an http.ResponseWriter
	Stream(ctx context.Context, filename string, w http.ResponseWriter) error

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, filename string) (bool, error)

	ListFiles(ctx context.Context, prefix string) ([]FileInfo, error)

	// Close cleans up any resources
	Close() error
}

// StorageConfig holds configuration for storage providers
type StorageConfig struct {
	// Provider type ("local" or "gcs")
	Provider string `json:"provider"`

	// Local storage config
	LocalPath string `json:"local_path,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`

	// GCS config
	ProjectID  string `json:"project_id,omitempty"`
	BucketName string `json:"bucket_name,omitempty"`
}

// NewStorageProvider creates a storage provider based on configuration
func NewStorageProvider(cfg StorageConfig) (StorageProvider, error) {
	switch cfg.Provider {
	case "local":
		return NewLocalStorage(cfg.LocalPath, cfg.BaseURL)
	case "gcs":
		return NewGCSStorage(cfg.ProjectID, cfg.BucketName)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}
