package config

import (
	"os"
	"reflect"
	"testing"
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
				"PORT":              "8080",
				"SECRET":            "mysecret",
				"APP_ENV":           "development",
				"BASE_URL":          "http://localhost",
				"UPLOAD_DIR":        "./uploads",
				"MAX_UPLOAD_SIZE":   "25MB",
				"UPLOAD_EXPIRES_IN": "24",
			},
			want: &Config{
				Port:            8080,
				Secret:          "mysecret",
				Env:             "development",
				BaseURL:         "http://localhost",
				UploadDirectory: "./uploads",
				MaxUploadSize:   25 * 1024 * 1024,
				UploadExpiresIn: 24,
			},
			wantErr: false,
		},
		{
			name: "Missing PORT",
			envVars: map[string]string{
				"SECRET":            "mysecret",
				"APP_ENV":           "development",
				"BASE_URL":          "http://localhost",
				"UPLOAD_DIR":        "./uploads",
				"MAX_UPLOAD_SIZE":   "25MB",
				"UPLOAD_EXPIRES_IN": "24",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid MAX_UPLOAD_SIZE",
			envVars: map[string]string{
				"PORT":              "8080",
				"SECRET":            "mysecret",
				"APP_ENV":           "development",
				"BASE_URL":          "http://localhost",
				"UPLOAD_DIR":        "./uploads",
				"MAX_UPLOAD_SIZE":   "invalid",
				"UPLOAD_EXPIRES_IN": "24",
			},
			want:    nil,
			wantErr: true,
		},
		{name: "Empty environment variables",
			envVars: map[string]string{
				"PORT":              "",
				"SECRET":            "",
				"APP_ENV":           "",
				"BASE_URL":          "",
				"UPLOAD_DIR":        "",
				"MAX_UPLOAD_SIZE":   "",
				"UPLOAD_EXPIRES_IN": "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Negative PORT",
			envVars: map[string]string{
				"PORT":              "-8080",
				"SECRET":            "mysecret",
				"APP_ENV":           "development",
				"BASE_URL":          "http://localhost",
				"UPLOAD_DIR":        "./uploads",
				"MAX_UPLOAD_SIZE":   "25MB",
				"UPLOAD_EXPIRES_IN": "24",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Zero UPLOAD_EXPIRES_IN",
			envVars: map[string]string{
				"PORT":              "8080",
				"SECRET":            "mysecret",
				"APP_ENV":           "development",
				"BASE_URL":          "http://localhost",
				"UPLOAD_DIR":        "./uploads",
				"MAX_UPLOAD_SIZE":   "25MB",
				"UPLOAD_EXPIRES_IN": "0",
			},
			want: &Config{
				Port:            8080,
				Secret:          "mysecret",
				Env:             "development",
				BaseURL:         "http://localhost",
				UploadDirectory: "./uploads",
				MaxUploadSize:   25 * 1024 * 1024,
				UploadExpiresIn: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				if err != nil {
					return
				}
			}
			got, err := NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfig() got = %v, want %v", got, tt.want)
			}
			for k := range tt.envVars {
				err := os.Unsetenv(k)
				if err != nil {
					return
				}
			}
		})
	}
}

func Test_parseMaxUploadSize(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMaxUploadSize(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseMaxUploadSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseMaxUploadSize() got = %v, want %v", got, tt.want)
			}
		})
	}
}
