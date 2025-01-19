package validation

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/url"
	"strings"
	"unicode"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validation functions
	if err := validate.RegisterValidation("username", validateUsername); err != nil {
		panic(fmt.Sprintf("failed to register username validation: %v", err))
	}
	if err := validate.RegisterValidation("password", validatePassword); err != nil {
		panic(fmt.Sprintf("failed to register password validation: %v", err))
	}
	if err := validate.RegisterValidation("url", validateURL); err != nil {
		panic(fmt.Sprintf("failed to register url validation: %v", err))
	}
	if err := validate.RegisterValidation("vanitycode", validateVanityCode); err != nil {
		panic(fmt.Sprintf("failed to register vanitycode validation: %v", err))
	}
}

// Validate validates a struct using tags
func Validate(s interface{}) error {
	return validate.Struct(s)
}

// ValidateUsername validates a username separately
func ValidateUsername(username string) error {
	return validate.Var(username, "required,username")
}

// ValidatePassword validates a password separately
func ValidatePassword(password string) error {
	return validate.Var(password, "required,password")
}

// ValidateURL validates a URL separately
func ValidateURL(urlStr string) error {
	return validate.Var(urlStr, "required,url")
}

// ValidateVanityCode validates a vanity code separately
func ValidateVanityCode(code string) error {
	return validate.Var(code, "vanitycode")
}

// Custom validation functions

func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()

	// Username requirements:
	// - Length between 3 and 50 characters
	// - Only alphanumeric characters, underscores, and hyphens
	// - Must start with a letter
	if len(username) < 3 || len(username) > 50 {
		return false
	}

	if !unicode.IsLetter(rune(username[0])) {
		return false
	}

	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' && char != '-' {
			return false
		}
	}

	return true
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	// Password requirements:
	// - Minimum 8 characters
	// - At least one uppercase letter
	// - At least one lowercase letter
	// - At least one number
	// - At least one special character
	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

func validateURL(fl validator.FieldLevel) bool {
	urlStr := fl.Field().String()

	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// URL requirements:
	// - Must have a scheme (http or https)
	// - Must have a host
	// - No fragments allowed
	return (u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" &&
		u.Fragment == ""
}

func validateVanityCode(fl validator.FieldLevel) bool {
	code := fl.Field().String()

	// Vanity code requirements:
	// - Length between 4 and 30 characters
	// - Only alphanumeric characters, underscores, and hyphens allowed
	if len(code) < 4 || len(code) > 30 {
		return false
	}

	for _, char := range code {
		if !unicode.IsLetter(char) && !unicode.IsNumber(char) && char != '_' && char != '-' {
			return false
		}
	}

	return true
}

// ValidationError represents a validation error
type ValidationError struct {
	Field string
	Error string
}

// FormatError formats a validation error into a human-readable message
func FormatError(err error) []ValidationError {
	var validationErrors []ValidationError

	if err == nil {
		return validationErrors
	}

	// Type assert to validator.ValidationErrors
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			var message string

			switch e.Tag() {
			case "required":
				message = fmt.Sprintf("%s is required", e.Field())
			case "email":
				message = "Invalid email format"
			case "username":
				message = "Username must be 3-50 characters long, start with a letter, and contain only letters, numbers, underscores, or hyphens"
			case "password":
				message = "Password must be at least 8 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character"
			case "url":
				message = "Invalid URL format. Must be a valid http or https URL"
			case "vanitycode":
				message = "Custom URL must be 4-30 characters long and contain only letters, numbers, underscores, or hyphens"
			default:
				message = fmt.Sprintf("Invalid value for %s", e.Field())
			}

			validationErrors = append(validationErrors, ValidationError{
				Field: strings.ToLower(e.Field()),
				Error: message,
			})
		}
	}

	return validationErrors
}
