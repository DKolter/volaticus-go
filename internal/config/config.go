package config

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
	"strings"
)

// Config holds server configuration
type Config struct {
	Port            int    // Port to listen on
	Secret          string // Secret key for JWT & api tokens
	Env             string // Environment (dev | prod)
	BaseURL         string // Base URL for the server
	UploadDirectory string // Directory to store uploaded files
	MaxUploadSize   int64  // Maximum upload size in bytes
	UploadExpiresIn int    // Upload expiration time in hours
}

func (c *Config) String() {
	log.Info().
		Int("port", c.Port).
		Str("env", c.Env).
		Str("base_url", c.BaseURL).
		Str("upload_dir", c.UploadDirectory).
		Int64("max_upload_size", c.MaxUploadSize).
		Int("upload_expires_in", c.UploadExpiresIn).
		Msg("server configuration")
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

	uploadDirectory := os.Getenv("UPLOAD_DIR")
	if uploadDirectory == "" {
		uploadDirectory = "./uploads"
	}

	maxUploadSizeStr := os.Getenv("MAX_UPLOAD_SIZE")
	if maxUploadSizeStr == "" {
		maxUploadSizeStr = "25MB" // Default value
	}
	maxUploadSize, err := parseMaxUploadSize(maxUploadSizeStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid MAX_UPLOAD_SIZE configuration")
		return nil, err
	}

	uploadExpiresInStr := os.Getenv("UPLOAD_EXPIRES_IN")
	if uploadExpiresInStr == "" {
		uploadExpiresInStr = "24" // Default value
	}
	uploadExpiresIn, err := strconv.Atoi(uploadExpiresInStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid UPLOAD_EXPIRES_IN environment variable")
		return nil, fmt.Errorf("invalid UPLOAD_EXPIRES_IN: %w", err)
	}

	return &Config{
		Port:            port,
		Secret:          secret,
		Env:             env,
		BaseURL:         baseURL,
		UploadDirectory: uploadDirectory,
		MaxUploadSize:   maxUploadSize,
		UploadExpiresIn: uploadExpiresIn,
	}, nil
}

// parseMaxUploadSize parses the MAX_UPLOAD_SIZE environment variable
// Value is expected to be postfixed with "MB" for megabytes or "GB" for gigabytes, e.g. "100MB"
// If no postfix is provided, the value is assumed to be in megabytes
func parseMaxUploadSize(size string) (int64, error) {
	if strings.HasSuffix(size, "GB") {
		value, err := strconv.ParseInt(strings.TrimSuffix(size, "GB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid MAX_UPLOAD_SIZE: %w", err)
		}
		return value * 1024 * 1024 * 1024, nil
	} else if strings.HasSuffix(size, "MB") {
		value, err := strconv.ParseInt(strings.TrimSuffix(size, "MB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid MAX_UPLOAD_SIZE: %w", err)
		}
		return value * 1024 * 1024, nil
	} else {
		value, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid MAX_UPLOAD_SIZE: %w", err)
		}
		return value * 1024 * 1024, nil
	}
}
