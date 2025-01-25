package config

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    *Config
		wantErr bool
	}{
		{
			name: "Valid configuration",
			envVars: map[string]string{
				"PORT":                 "8080",
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_MAX_SIZE":      "25MB",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "local",
				"UPLOAD_DIR":           "./uploads",
			},
			want: &Config{
				Port:            8080,
				Secret:          "mysecret",
				Env:             "development",
				BaseURL:         "http://localhost",
				UploadMaxSize:   25 * 1024 * 1024,
				UploadUserQuota: 100 * 1024 * 1024,
				UploadExpiresIn: 24 * time.Hour,
				Storage: StorageConfig{
					Provider:  "local",
					LocalPath: "./uploads",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid GCS configuration",
			envVars: map[string]string{
				"PORT":                 "8080",
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_MAX_SIZE":      "25MB",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "gcs",
				"GCS_PROJECT_ID":       "my-project",
				"GCS_BUCKET_NAME":      "my-bucket",
			},
			want: &Config{
				Port:            8080,
				Secret:          "mysecret",
				Env:             "development",
				BaseURL:         "http://localhost",
				UploadMaxSize:   25 * 1024 * 1024,
				UploadUserQuota: 100 * 1024 * 1024,
				UploadExpiresIn: 24 * time.Hour,
				Storage: StorageConfig{
					Provider:   "gcs",
					ProjectID:  "my-project",
					BucketName: "my-bucket",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing PORT",
			envVars: map[string]string{
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_DIR":           "./uploads",
				"UPLOAD_MAX_SIZE":      "25MB",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "local",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid UPLOAD_MAX_SIZE",
			envVars: map[string]string{
				"PORT":                 "8080",
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_DIR":           "./uploads",
				"UPLOAD_MAX_SIZE":      "invalid",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "local",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty environment variables",
			envVars: map[string]string{
				"PORT":                 "",
				"SECRET":               "",
				"APP_ENV":              "",
				"BASE_URL":             "",
				"UPLOAD_DIR":           "",
				"UPLOAD_MAX_SIZE":      "",
				"UPLOAD_USER_MAX_SIZE": "",
				"UPLOAD_EXPIRES_IN":    "",
				"STORAGE_PROVIDER":     "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid storage provider",
			envVars: map[string]string{
				"PORT":                 "8080",
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_MAX_SIZE":      "25MB",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "invalid",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Missing GCS configuration",
			envVars: map[string]string{
				"PORT":                 "8080",
				"SECRET":               "mysecret",
				"APP_ENV":              "development",
				"BASE_URL":             "http://localhost",
				"UPLOAD_MAX_SIZE":      "25MB",
				"UPLOAD_USER_MAX_SIZE": "100MB",
				"UPLOAD_EXPIRES_IN":    "24",
				"STORAGE_PROVIDER":     "gcs",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment before each test
			os.Clearenv()

			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", k, err)
				}
			}

			got, err := NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_parseUploadMaxSize(t *testing.T) {
	tests := []struct {
		name    string
		size    string
		want    int64
		wantErr bool
	}{
		{
			name:    "Valid MB size",
			size:    "25MB",
			want:    25 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "Valid GB size",
			size:    "1GB",
			want:    1 * 1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "Invalid size",
			size:    "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "No suffix size",
			size:    "25",
			want:    25 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "Zero size",
			size:    "0MB",
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUploadMaxSize(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUploadMaxSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseUploadMaxSize() = %v, want %v", got, tt.want)
			}
		})
	}
}
