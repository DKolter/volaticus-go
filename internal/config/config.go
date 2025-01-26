package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Config holds server configuration
type Config struct {
	Port            int           // Port to listen on
	Secret          string        // Secret key for JWT & api tokens
	Env             string        // Environment (dev | prod)
	BaseURL         string        // Base URL for the server
	UploadMaxSize   int64         // Maximum upload size in bytes
	UploadUserQuota int64         // Quota user is allowed to upload in bytes
	UploadExpiresIn time.Duration // Upload expiration time in hours
	Storage         StorageConfig
}

func (c *Config) Log() {
	log.Info().
		Int("port", c.Port).
		Str("env", c.Env).
		Str("base_url", c.BaseURL).
		Int64("upload_max_size", c.UploadMaxSize).
		Int64("upload_user_quota", c.UploadUserQuota).
		Dur("upload_expires_in", c.UploadExpiresIn).
		Msg("server configuration")
}
}

type StorageConfig struct {
	// Provider type ("local" or "gcs")
	Provider string `json:"provider"`

	// Local storage config
	LocalPath string `json:"local_path,omitempty"`

	// GCS config
	ProjectID  string `json:"project_id,omitempty"`
	BucketName string `json:"bucket_name,omitempty"`
}

// NewConfig creates a server configuration from environment variables
func NewConfig() (*Config, error) {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil || port <= 0 {
		log.Error().Err(err).Msg("invalid PORT environment variable")
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	secret := os.Getenv("SECRET")
	if secret == "" {
		log.Error().Msg("SECRET environment variable is required")
		return nil, fmt.Errorf("SECRET is required")
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost"
	}

	uploadMaxSizeStr := os.Getenv("UPLOAD_MAX_SIZE")
	if uploadMaxSizeStr == "" {
		uploadMaxSizeStr = "25MB" // Default value
	}
	uploadMaxSize, err := parseUploadMaxSize(uploadMaxSizeStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid UPLOAD_MAX_SIZE configuration")
		return nil, err
	}

	uploadUserQuotaStr := os.Getenv("UPLOAD_USER_MAX_SIZE")
	if uploadUserQuotaStr == "" {
		uploadUserQuotaStr = "100MB" // Default value
	}
	uploadUserQuota, err := parseUploadMaxSize(uploadUserQuotaStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid UPLOAD_USER_MAX_SIZE configuration")
		return nil, err
	}

	uploadExpiresInStr := os.Getenv("UPLOAD_EXPIRES_IN")
	if uploadExpiresInStr == "" {
		uploadExpiresInStr = "24h"
	} else {
		// Check if the string has a duration suffix
		if !strings.HasSuffix(uploadExpiresInStr, "h") && !strings.HasSuffix(uploadExpiresInStr, "d") {
			uploadExpiresInStr += "h"
		}
	}

	// Parse the duration
	uploadExpiresIn, err := time.ParseDuration(uploadExpiresInStr)
	if err != nil || uploadExpiresIn <= 0 {
		log.Error().Err(err).Msg("invalid UPLOAD_EXPIRES_IN environment variable")
		return nil, fmt.Errorf("invalid UPLOAD_EXPIRES_IN: %w", err)
	}

	// Configure storage
	storageProvider := os.Getenv("STORAGE_PROVIDER")
	if storageProvider == "" {
		storageProvider = "local"
	}

	storageConfig := StorageConfig{
		Provider:   storageProvider,
		LocalPath:  os.Getenv("UPLOAD_DIR"),
		ProjectID:  os.Getenv("GCS_PROJECT_ID"),
		BucketName: os.Getenv("GCS_BUCKET_NAME"),
	}

	// Validate storage configuration
	if err := validateStorageConfig(storageConfig); err != nil {
		return nil, fmt.Errorf("invalid storage configuration: %w", err)
	}

	return &Config{
		Port:            port,
		Secret:          secret,
		Env:             env,
		BaseURL:         baseURL,
		UploadMaxSize:   uploadMaxSize,
		UploadUserQuota: uploadUserQuota,
		UploadExpiresIn: uploadExpiresIn,
		Storage:         storageConfig,
	}, nil
}

// validateStorageConfig ensures the storage configuration is valid
func validateStorageConfig(cfg StorageConfig) error {
	switch cfg.Provider {
	case "local":
		if cfg.LocalPath == "" {
			return fmt.Errorf("UPLOAD_DIR is required for local storage")
		}
	case "gcs":
		if cfg.ProjectID == "" {
			return fmt.Errorf("GCS_PROJECT_ID is required for GCS storage")
		}
		if cfg.BucketName == "" {
			return fmt.Errorf("GCS_BUCKET_NAME is required for GCS storage")
		}
	default:
		return fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
	return nil
}

// parseUploadMaxSize parses the UPLOAD_MAX_SIZE environment variable
// Value is expected to be postfixed with "MB" for megabytes or "GB" for gigabytes, e.g. "100MB"
// If no postfix is provided, the value is assumed to be in megabytes
func parseUploadMaxSize(size string) (int64, error) {
	if strings.HasSuffix(size, "GB") {
		value, err := strconv.ParseInt(strings.TrimSuffix(size, "GB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid UPLOAD_MAX_SIZE: %w", err)
		}
		return value * 1024 * 1024 * 1024, nil
	} else if strings.HasSuffix(size, "MB") {
		value, err := strconv.ParseInt(strings.TrimSuffix(size, "MB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid UPLOAD_MAX_SIZE: %w", err)
		}
		return value * 1024 * 1024, nil
	} else {
		value, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid UPLOAD_MAX_SIZE: %w", err)
		}
		return value * 1024 * 1024, nil
	}
}
