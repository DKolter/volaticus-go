package validation

import (
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestValidatorInit ensures all custom validations are registered
func TestValidatorInit(t *testing.T) {
	// This will panic if registration fails
	validate := validator.New()

	// These should not panic
	assert.NotPanics(t, func() {
		err := validate.RegisterValidation("username", validateUsername)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		err := validate.RegisterValidation("password", validatePassword)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		err := validate.RegisterValidation("url", validateURL)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		err := validate.RegisterValidation("vanitycode", validateVanityCode)
		assert.NoError(t, err)
	})
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"Valid username", "john_doe123", false},
		{"Too short", "jo", true},
		{"Too long", string(make([]byte, 51)), true},
		{"Invalid start char", "1john", true},
		{"Invalid chars", "john@doe", true},
		{"Valid with hyphen", "john-doe", false},
		{"Valid complex", "John_Doe-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"Valid password", "TestPass1!", false},
		{"Too short", "Test1!", true},
		{"No uppercase", "testpass1!", true},
		{"No lowercase", "TESTPASS1!", true},
		{"No number", "TestPass!", true},
		{"No special", "TestPass1", true},
		{"Valid complex", "Test1Pass!@#", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"Valid http URL", "http://example.com", false},
		{"Valid https URL", "https://example.com/path", false},
		{"Invalid scheme", "ftp://example.com", true},
		{"No scheme", "example.com", true},
		{"Invalid URL", "not-a-url", true},
		{"Valid with query", "https://example.com/path?q=test", false},
		{"Invalid with fragment", "https://example.com#fragment", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVanityCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"Valid code", "test-code", false},
		{"Too short", "abc", true},
		{"Too long", string(make([]byte, 31)), true},
		{"Invalid chars", "test@code", true},
		{"Valid with numbers", "test-123", false},
		{"Valid with underscore", "test_code", false},
		{"Valid complex", "Test-Code_123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVanityCode(tt.code)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	type TestStruct struct {
		Username string `validate:"required,username"`
		Password string `validate:"required,password"`
		URL      string `validate:"required,url"`
	}

	test := TestStruct{
		Username: "1invalid",
		Password: "weak",
		URL:      "invalid-url",
	}

	err := Validate(&test)
	assert.Error(t, err)

	errors := FormatError(err)
	assert.NotEmpty(t, errors)

	// Check that we have validation errors for all fields
	fields := make(map[string]bool)
	for _, e := range errors {
		fields[e.Field] = true
		assert.NotEmpty(t, e.Error)
	}

	assert.True(t, fields["username"])
	assert.True(t, fields["password"])
	assert.True(t, fields["url"])
}
