package server

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds server configuration
type Config struct {
	Port   int
	Secret string
	Env    string
}

// NewConfig creates a server configuration from environment variables
func NewConfig() (*Config, error) {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	secret := os.Getenv("SECRET")
	if secret == "" {
		return nil, fmt.Errorf("SECRET is required")
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	return &Config{
		Port:   port,
		Secret: secret,
		Env:    env,
	}, nil
}
