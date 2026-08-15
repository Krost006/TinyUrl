package config

import (
	"errors"
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	base := map[string]string{"DBURL": "postgres://x", "JWT_SECRET": "s"}

	tests := []struct {
		name     string
		env      map[string]string
		wantErr  error
		wantBase string
		wantHost string
	}{
		{"defaults", nil, nil, "http://localhost:8080", "localhost:8080"},
		{"custom port", map[string]string{"PORT": "9000"}, nil, "http://localhost:9000", "localhost:9000"},
		{"explicit base", map[string]string{"BASE_URL": "https://short.ly"}, nil, "https://short.ly", "short.ly"},
		{"trailing slash", map[string]string{"BASE_URL": "https://short.ly/"}, nil, "https://short.ly", "short.ly"},
		{"no scheme", map[string]string{"BASE_URL": "short.ly"}, ErrBadBaseURL, "", ""},
		{"bad scheme", map[string]string{"BASE_URL": "ftp://short.ly"}, ErrBadBaseURL, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range base {
				os.Setenv(k, v)
			}
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			c, err := LoadConfig()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if c.BaseURL != tt.wantBase {
				t.Errorf("BaseURL = %q, want %q", c.BaseURL, tt.wantBase)
			}
			h, _ := c.Host()
			if h != tt.wantHost {
				t.Errorf("Host() = %q, want %q", h, tt.wantHost)
			}
		})
	}
}

func TestLoadConfigRequires(t *testing.T) {
	os.Clearenv()
	if _, err := LoadConfig(); !errors.Is(err, ErrNoDBURL) {
		t.Errorf("got %v, want ErrNoDBURL", err)
	}

	os.Setenv("DBURL", "postgres://x")
	if _, err := LoadConfig(); !errors.Is(err, ErrNoJWTSecret) {
		t.Errorf("got %v, want ErrNoJWTSecret", err)
	}
}
