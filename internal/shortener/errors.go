package shortener

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"net/http"
)

// APIError represents a standardized error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Common error codes
const (
	ErrCodeInvalidInput  = "INVALID_INPUT"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeAlreadyExists = "ALREADY_EXISTS"
	ErrCodeInternalError = "INTERNAL_ERROR"
	ErrCodeExpired       = "EXPIRED"
)

// Error responses
var (
	ErrInvalidURL = &APIError{
		Code:    ErrCodeInvalidInput,
		Message: "Invalid URL format",
	}
	ErrURLNotFound = &APIError{
		Code:    ErrCodeNotFound,
		Message: "URL not found or expired",
	}
	ErrUnauthorized = &APIError{
		Code:    ErrCodeUnauthorized,
		Message: "Unauthorized access",
	}
	ErrVanityCodeTaken = &APIError{
		Code:    ErrCodeAlreadyExists,
		Message: "Custom URL code already in use",
	}
	ErrInvalidVanityCode = &APIError{
		Code:    ErrCodeInvalidInput,
		Message: "Invalid custom URL format",
	}
	ErrURLExpired = &APIError{
		Code:    ErrCodeExpired,
		Message: "URL has expired",
	}
)

// HandleError sends a standardized error response
func HandleError(w http.ResponseWriter, err *APIError, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(err); err != nil {
		log.Error().
			Err(err).
			Interface("api_error", err).
			Msg("failed to encode error response")
	}
}

// LogError logs an error and returns an appropriate API error
func LogError(err error, context string) *APIError {
	log.Error().
		Err(err).
		Str("context", context).
		Msg("internal error occurred")
	return &APIError{
		Code:    ErrCodeInternalError,
		Message: "An internal error occurred",
		Details: context,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return err.Error() == "URL not found" || err.Error() == "URL not found or expired"
}

// IsUnauthorized checks if an error is an unauthorized error
func IsUnauthorized(err error) bool {
	return err.Error() == "unauthorized access" || err.Error() == "unauthorized access to URL"
}
